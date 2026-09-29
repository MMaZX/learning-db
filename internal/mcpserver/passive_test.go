package mcpserver

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	_ "modernc.org/sqlite"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/query"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

// newQueryClient arma el servidor con un Engine sobre SQLite en memoria (el
// motor solo usa database/sql), así se ejercita consultar_base_datos completo
// sin MySQL.
func newQueryClient(t *testing.T, passive bool) (context.Context, *store.Store, *client.Client) {
	t.Helper()
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	pool, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	for _, s := range []string{
		`CREATE TABLE pedido (id INTEGER, cliente_id INTEGER, email TEXT)`,
		`CREATE TABLE cliente (id INTEGER, nombre TEXT)`,
		`INSERT INTO pedido VALUES (1, 1, 'secreto@correo.com')`,
		`INSERT INTO cliente VALUES (1, 'Ana')`,
	} {
		if _, err := pool.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := query.NewEngine(pool, query.Config{Timeout: time.Second, MaxRows: 10, MaxColumns: 10,
		MaxResultSizeBytes: 1 << 20, MaxConcurrent: 2, SlowQueryThreshold: time.Hour}, logger)

	c, err := client.NewInProcessClient(New(&Deps{
		Engine: engine, Store: metadata, Knowledge: knowledge.NewStore(t.TempDir()),
		Logger: logger, PassiveCapture: passive,
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	if _, err := c.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	return ctx, metadata, c
}

func runQuery(t *testing.T, ctx context.Context, c *client.Client, sqlText string) string {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = "consultar_base_datos"
	req.Params.Arguments = map[string]any{"sql": sqlText}
	res, err := c.CallTool(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("la consulta falló: %+v", res.Content)
	}
	return res.Content[0].(mcp.TextContent).Text
}

const passiveSQL = "SELECT p.email, c.nombre FROM pedido p JOIN cliente c ON c.id = p.cliente_id WHERE p.email = 'secreto@correo.com'"

func passiveObservations(t *testing.T, ctx context.Context, s *store.Store) []store.Observation {
	t.Helper()
	obs, err := s.SearchObservations(ctx, "consulta:", 50)
	if err != nil {
		t.Fatal(err)
	}
	return obs
}

func TestPassiveCapture_FlagOffWritesNothing(t *testing.T) {
	ctx, s, c := newQueryClient(t, false)
	runQuery(t, ctx, c, passiveSQL)
	if obs := passiveObservations(t, ctx, s); len(obs) != 0 {
		t.Fatalf("con el flag apagado no debe haber observaciones: %+v", obs)
	}
}

func TestPassiveCapture_FlagOnRecordsTablesOnlyAndResponseUnchanged(t *testing.T) {
	ctx, s, c := newQueryClient(t, true)
	out := runQuery(t, ctx, c, passiveSQL)
	runQuery(t, ctx, c, passiveSQL) // repetida: no genera otra observación

	// Forma de la respuesta idéntica: TSV + pie de metadatos, sin campos nuevos.
	if !strings.Contains(out, "total_filas: 1\ntruncado: false\nduracion_ms: ") || !strings.Contains(out, "sql_ejecutado: ") || !strings.Contains(out, "Ana") {
		t.Fatalf("respuesta inesperada: %s", out)
	}
	if strings.Contains(out, "tablas") || strings.Contains(out, "passive") {
		t.Fatalf("la respuesta no debe exponer la captura: %s", out)
	}

	obs := passiveObservations(t, ctx, s)
	if len(obs) != 1 {
		t.Fatalf("observaciones=%d, quería 1", len(obs))
	}
	o := obs[0]
	if o.Subject != "consulta:cliente,pedido" || o.Agent != store.PassiveAgent || o.Kind != "observation" {
		t.Fatalf("observación inesperada: %+v", o)
	}
	all := o.Subject + o.Statement + o.Context + o.EvidenceJSON + o.RelatedQueriesJSON
	for _, leak := range []string{"secreto", "correo", "SELECT", "select", "Ana", "WHERE"} {
		if strings.Contains(all, leak) {
			t.Fatalf("la observación filtra %q: %+v", leak, o)
		}
	}
}

func TestPassiveCapture_FailureDoesNotFailQuery(t *testing.T) {
	ctx, s, c := newQueryClient(t, true)
	// Con el store cerrado la captura falla; la consulta debe seguir respondiendo.
	_ = s.Close()
	out := runQuery(t, ctx, c, "SELECT nombre FROM cliente")
	if !strings.Contains(out, "Ana") {
		t.Fatalf("respuesta inesperada: %s", out)
	}
}
