package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fulanito/db-intelligence-mcp/migrations"
)

// seedStoreBeforeMigration deja en dataPath un mcp.db con las migraciones
// anteriores a `stopBefore` aplicadas y datos que referencian knowledge(id)
// (supersedes_id y knowledge_relations), como en un store de producción.
func seedStoreBeforeMigration(t *testing.T, dataPath, stopBefore string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dataPath, "mcp.db")+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") && e.Name() < stopBefore {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			t.Fatalf("aplicando %s: %v", name, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			t.Fatalf("registrando %s: %v", name, err)
		}
	}

	for _, q := range []string{
		`INSERT INTO knowledge(id, subject, claim, status) VALUES (1, 'a', 'primero', 'validated')`,
		`INSERT INTO knowledge(id, subject, claim, status, supersedes_id) VALUES (2, 'a', 'segundo', 'proposed', 1)`,
		`INSERT INTO knowledge_relations(from_id, to_id, relation) VALUES (2, 1, 'same_topic')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
}

func TestMigrate_RebuildConFilasReferenciadasPreservaDatosYRelaciones(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	seedStoreBeforeMigration(t, dir, "0009_")

	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open sobre store con filas referenciadas: %v", err)
	}
	defer s.Close()

	var n, rel, sup int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("knowledge = %d (err %v), want 2", n, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_relations`).Scan(&rel); err != nil || rel != 1 {
		t.Fatalf("knowledge_relations = %d (err %v), want 1", rel, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT supersedes_id FROM knowledge WHERE id = 2`).Scan(&sup); err != nil || sup != 1 {
		t.Fatalf("supersedes_id = %d (err %v), want 1", sup, err)
	}
	// 'revoked' ya es un status válido tras 0009.
	if _, err := s.db.ExecContext(ctx, `UPDATE knowledge SET status = 'revoked' WHERE id = 1`); err != nil {
		t.Fatalf("status revoked rechazado tras 0009: %v", err)
	}
	// Las foreign keys vuelven a estar activas en la conexión.
	var fk int
	if err := s.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (err %v), want 1", fk, err)
	}
}

func TestMigrate_ViolacionDeFKPreexistenteAbortaSinAplicar(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	seedStoreBeforeMigration(t, dir, "0009_")

	// Corrompemos el store: una relación apunta a un id inexistente.
	raw, err := sql.Open("sqlite", filepath.Join(dir, "mcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO knowledge_relations(from_id, to_id, relation) VALUES (99, 1, 'same_topic')`); err != nil {
		t.Fatalf("seed huérfano: %v", err)
	}
	raw.Close()

	if _, err := Open(ctx, dir); err == nil {
		t.Fatal("Open debía fallar ante una violación de FK preexistente")
	}

	raw, err = sql.Open("sqlite", filepath.Join(dir, "mcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var applied int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version LIKE '0009_%'`).Scan(&applied); err != nil || applied != 0 {
		t.Fatalf("0009 registrada pese al fallo: %d (err %v)", applied, err)
	}
}
