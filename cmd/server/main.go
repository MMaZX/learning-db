// Command server arranca el MCP de inteligencia de base de datos: conecta a
// MySQL/MariaDB en modo solo lectura, carga el schema, el conocimiento de
// negocio y la metadata de aprendizaje, y sirve las tools MCP por stdio.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/config"
	"github.com/fulanito/db-intelligence-mcp/internal/database"
	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/mcpserver"
	"github.com/fulanito/db-intelligence-mcp/internal/query"
	"github.com/fulanito/db-intelligence-mcp/internal/schema"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cargando configuración: %w", err)
	}

	// El log va a stderr: stdout está reservado para el transporte MCP
	// (stdio), nunca debe mezclarse con logging.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	logger.Info("conectando a la base de datos", "host", cfg.DBHost, "port", cfg.DBPort, "database", cfg.DBDatabase, "user", cfg.DBUsername)
	db, err := database.Open(ctx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("conectando a mysql: %w", err)
	}
	defer db.Close()

	inspector := schema.NewInspector(db.Pool(), cfg.ExcludedTables, cfg.ExcludedColumns)
	if _, err := inspector.Refresh(ctx); err != nil {
		return fmt.Errorf("descubriendo schema inicial: %w", err)
	}
	logger.Info("schema descubierto")

	knowledgeStore := knowledge.NewStore(cfg.KnowledgePath)
	if err := knowledgeStore.Reload(); err != nil {
		return fmt.Errorf("cargando conocimiento de negocio: %w", err)
	}

	metaStore, err := store.Open(ctx, cfg.DataPath)
	if err != nil {
		return fmt.Errorf("abriendo metadata store: %w", err)
	}
	defer metaStore.Close()

	engine := query.NewEngine(db.Pool(), query.Config{
		Timeout:            cfg.QueryTimeout,
		MaxRows:            cfg.MaxRows,
		MaxColumns:         cfg.MaxColumns,
		MaxResultSizeBytes: cfg.MaxResultSizeBytes,
		MaxConcurrent:      cfg.MaxConcurrentQueries,
		SlowQueryThreshold: cfg.SlowQueryThreshold,
	}, logger)

	mcpServer := mcpserver.New(&mcpserver.Deps{
		Inspector: inspector,
		Engine:    engine,
		Knowledge: knowledgeStore,
		Store:     metaStore,
		Logger:    logger,
		MaxRows:   cfg.MaxRows,
	})

	if cfg.MCPTransport == "http" {
		return serveHTTP(ctx, cfg, mcpServer, logger)
	}

	logger.Info("MCP listo, sirviendo por stdio", "server", cfg.ServerName)
	return server.ServeStdio(mcpServer)
}

// serveHTTP sirve el MCP por Streamable HTTP, permitiendo múltiples sesiones
// concurrentes (una por cliente, gestionadas internamente por mcp-go vía
// SessionID). Requiere MCP_AUTH_TOKEN: sin él, config.Load ya rechaza el
// arranque en modo http, así que aquí siempre hay token que exigir.
func serveHTTP(ctx context.Context, cfg *config.Config, mcpServer *server.MCPServer, logger *slog.Logger) error {
	streamableSrv := server.NewStreamableHTTPServer(
		mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStreamableHTTPLogger(logger),
		server.WithStateful(true),
		server.WithSessionIdleTTL(30*time.Minute),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.WithBearerAuth(cfg.MCPAuthToken, streamableSrv))

	httpServer := &http.Server{
		Addr:    cfg.MCPHTTPAddr,
		Handler: mux,
	}

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("MCP listo, sirviendo por HTTP", "addr", cfg.MCPHTTPAddr, "endpoint", "/mcp")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-sigCtx.Done():
		logger.Info("apagando MCP HTTP")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		streamableSrv.CloseSessions(shutdownCtx)
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("apagando servidor HTTP: %w", err)
		}
		return nil
	}
}

func logLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
