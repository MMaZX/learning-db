// Package database gestiona la conexión de solo lectura hacia MySQL/MariaDB.
package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// DB envuelve el pool de conexiones hacia la base de datos de producción.
// Es siempre de solo lectura: nunca se usa para persistir metadata del MCP.
type DB struct {
	pool *sql.DB
}

// readOnlyConnector envuelve el connector real del driver mysql para forzar
// "SET SESSION TRANSACTION READ ONLY" en CADA conexión física nueva que abre
// el pool, no solo en la primera. Es una capa de defensa adicional a los
// permisos del usuario MySQL y al validador SQL de internal/security.
type readOnlyConnector struct {
	driver.Connector
}

func (c *readOnlyConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	execer, ok := conn.(driver.ExecerContext)
	if !ok {
		return conn, nil
	}
	if _, err := execer.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY", nil); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("no se pudo forzar sesión READ ONLY en la conexión: %w", err)
	}
	return conn, nil
}

// Open abre el pool y verifica conectividad.
func Open(ctx context.Context, dsn string) (*DB, error) {
	mysqlCfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("DSN inválido: %w", err)
	}

	baseConnector, err := mysql.NewConnector(mysqlCfg)
	if err != nil {
		return nil, fmt.Errorf("creando connector mysql: %w", err)
	}

	pool := sql.OpenDB(&readOnlyConnector{Connector: baseConnector})

	pool.SetMaxOpenConns(15)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)
	pool.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.PingContext(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Pool expone el *sql.DB subyacente para los paquetes que lo necesiten
// (schema inspector, query engine).
func (d *DB) Pool() *sql.DB {
	return d.pool
}

func (d *DB) Close() error {
	return d.pool.Close()
}

// DatabaseName obtiene el nombre de la base de datos activa.
func (d *DB) DatabaseName(ctx context.Context) (string, error) {
	var name string
	if err := d.pool.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}
