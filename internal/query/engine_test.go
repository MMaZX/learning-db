package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// El motor solo usa database/sql, así que un SQLite en memoria sirve para
// ejercitar Execute sin MySQL.
func TestExecute_ReturnsTablesButNotInJSON(t *testing.T) {
	pool, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { pool.Close() })
	for _, stmt := range []string{
		`CREATE TABLE pedido (id INTEGER, cliente_id INTEGER)`,
		`CREATE TABLE cliente (id INTEGER, nombre TEXT)`,
		`INSERT INTO pedido VALUES (1, 1)`,
		`INSERT INTO cliente VALUES (1, 'Ana')`,
	} {
		if _, err := pool.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	e := NewEngine(pool, Config{Timeout: time.Second, MaxRows: 10, MaxColumns: 10, MaxResultSizeBytes: 1 << 20, MaxConcurrent: 1, SlowQueryThreshold: time.Hour},
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	res, err := e.Execute(context.Background(),
		"SELECT p.id, c.nombre FROM pedido p JOIN cliente c ON c.id = p.cliente_id WHERE p.id IN (SELECT id FROM pedido)")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Tables, ",") != "cliente,pedido" {
		t.Fatalf("tables=%v", res.Tables)
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), "tables") || strings.Contains(string(b), "Tables") {
		t.Fatalf("Tables no debe serializarse: %s", b)
	}
}
