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
	// La CLI de usuarios solo necesita el data path: se despacha antes de
	// validar la configuración completa para no exigir credenciales de MySQL.
	cliCfg := &config.Config{DataPath: config.DataPathFromEnv()}
	if handled, err := handleCLI(cliCfg, os.Args[1:]); handled {
		return err
	}

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

		PassiveCapture: cfg.PassiveCapture,
	})

	if cfg.MCPTransport == "http" {
		if cfg.MCPAuthToken == "" {
			users, err := metaStore.ListUsers(ctx)
			if err != nil {
				return fmt.Errorf("listando usuarios: %w", err)
			}
			if len(users) == 0 {
				return errors.New("transporte http sin MCP_AUTH_TOKEN requiere al menos un usuario (ejecuta 'user create <name>')")
			}
		}
		return serveHTTP(ctx, cfg, mcpServer, metaStore, logger)
	}

	logger.Info("MCP listo, sirviendo por stdio", "server", cfg.ServerName)
	return server.ServeStdio(mcpServer)
}

func handleCLI(cfg *config.Config, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "create-user":
		if len(args) < 2 {
			return true, fmt.Errorf("uso: %s create-user <username>", os.Args[0])
		}
		return true, createUserCmd(cfg, args[1])

	case "user":
		if len(args) < 2 {
			return true, fmt.Errorf("uso: %s user <create|list> [args]", os.Args[0])
		}
		switch args[1] {
		case "create":
			if len(args) < 3 {
				return true, fmt.Errorf("uso: %s user create <username>", os.Args[0])
			}
			return true, createUserCmd(cfg, args[2])
		case "list":
			return true, listUsersCmd(cfg)
		default:
			return true, fmt.Errorf("subcomando desconocido: user %s", args[1])
		}

	case "list-users":
		return true, listUsersCmd(cfg)

	case "help", "-help", "--help", "-h":
		fmt.Printf(`Uso:
  %s                     Arranca el servidor MCP
  %s user create <name>  Crea un nuevo usuario con token personal (A2)
  %s user list           Lista los usuarios registrados
`, os.Args[0], os.Args[0], os.Args[0])
		return true, nil

	default:
		return false, nil
	}
}

func createUserCmd(cfg *config.Config, username string) error {
	ctx := context.Background()
	metaStore, err := store.Open(ctx, cfg.DataPath)
	if err != nil {
		return fmt.Errorf("abriendo metadata store: %w", err)
	}
	defer metaStore.Close()

	token, err := store.GenerateToken()
	if err != nil {
		return fmt.Errorf("generando token: %w", err)
	}
	hash := store.HashToken(token)

	u, err := metaStore.CreateUser(ctx, username, hash)
	if err != nil {
		return fmt.Errorf("creando usuario: %w", err)
	}

	fmt.Printf("Usuario creado correctamente:\n")
	fmt.Printf("  ID:       %d\n", u.ID)
	fmt.Printf("  Usuario:  %s\n", u.Username)
	fmt.Printf("  Token:    %s\n\n", token)
	fmt.Println("IMPORTANTE: Guarde este token ahora. Por seguridad, no se volverá a mostrar.")
	return nil
}

func listUsersCmd(cfg *config.Config) error {
	ctx := context.Background()
	metaStore, err := store.Open(ctx, cfg.DataPath)
	if err != nil {
		return fmt.Errorf("abriendo metadata store: %w", err)
	}
	defer metaStore.Close()

	users, err := metaStore.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("listando usuarios: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No hay usuarios registrados.")
		return nil
	}

	fmt.Printf("%-5s %-20s %-8s %s\n", "ID", "USUARIO", "ACTIVO", "CREADO")
	for _, u := range users {
		fmt.Printf("%-5d %-20s %-8v %s\n", u.ID, u.Username, u.IsActive, u.CreatedAt.Format(time.RFC3339))
	}
	return nil
}

// serveHTTP sirve el MCP por Streamable HTTP, permitiendo múltiples sesiones
// concurrentes (una por cliente, gestionadas internamente por mcp-go vía
// SessionID). Requiere autenticación por token (personal o estático de transición).
func serveHTTP(ctx context.Context, cfg *config.Config, mcpServer *server.MCPServer, metaStore *store.Store, logger *slog.Logger) error {
	streamableSrv := server.NewStreamableHTTPServer(
		mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStreamableHTTPLogger(logger),
		server.WithStateful(true),
		server.WithSessionIdleTTL(30*time.Minute),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.WithUserAuth(metaStore, cfg.MCPAuthToken, streamableSrv))

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
