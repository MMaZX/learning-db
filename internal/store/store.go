// Package store persiste la metadata propia del MCP (historial de queries,
// auditoría, observaciones y conocimiento) en SQLite. Está deliberadamente
// separado de la base de datos de producción: el MCP nunca escribe ahí.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"

	"github.com/fulanito/db-intelligence-mcp/migrations"
)

type Store struct {
	db *sql.DB
}

// Open abre (o crea) el archivo SQLite en dataPath/mcp.db y aplica las
// migraciones pendientes.
func Open(ctx context.Context, dataPath string) (*Store, error) {
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return nil, fmt.Errorf("creando data path %q: %w", dataPath, err)
	}
	dbPath := filepath.Join(dataPath, "mcp.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("abriendo sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite + WAL: un único writer evita "database is locked"

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("conectando a sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`); err != nil {
		return fmt.Errorf("creando tabla schema_migrations: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("leyendo migraciones embebidas: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&applied); err != nil {
			return fmt.Errorf("comprobando migración %s: %w", name, err)
		}
		if applied > 0 {
			continue
		}
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("leyendo migración %s: %w", name, err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("aplicando migración %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (?)`, name); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
