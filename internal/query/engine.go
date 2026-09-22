// Package query implementa el Query Engine: valida, limita, ejecuta y
// audita cada SQL de solo lectura antes de tocar la base de datos.
package query

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/fulanito/db-intelligence-mcp/internal/security"
)

// Result es la forma estructurada que se devuelve a los tools MCP.
type Result struct {
	Columns     []string         `json:"columns"`
	Rows        []map[string]any `json:"rows"`
	RowCount    int              `json:"row_count"`
	Truncated   bool             `json:"truncated"`
	DurationMS  int64            `json:"duration_ms"`
	SQLExecuted string           `json:"sql_executed"`
}

type Config struct {
	Timeout            time.Duration
	MaxRows            int
	MaxColumns         int
	MaxResultSizeBytes int64
	MaxConcurrent      int
	SlowQueryThreshold time.Duration
}

type Engine struct {
	pool   *sql.DB
	cfg    Config
	logger *slog.Logger
	sem    chan struct{}
}

func NewEngine(pool *sql.DB, cfg Config, logger *slog.Logger) *Engine {
	return &Engine{
		pool:   pool,
		cfg:    cfg,
		logger: logger,
		sem:    make(chan struct{}, cfg.MaxConcurrent),
	}
}

// Execute valida y ejecuta un SQL de solo lectura, aplicando todos los
// límites configurados. rawSQL nunca se ejecuta directamente: siempre pasa
// primero por security.Validate.
func (e *Engine) Execute(ctx context.Context, rawSQL string) (*Result, error) {
	stmt, err := security.Validate(rawSQL)
	if err != nil {
		return nil, err
	}
	limited := security.EnforceLimit(stmt, e.cfg.MaxRows)
	finalSQL := security.String(limited)

	// Límite de consultas concurrentes.
	select {
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	start := time.Now()
	rows, err := e.pool.QueryContext(timeoutCtx, finalSQL)
	if err != nil {
		return nil, fmt.Errorf("ejecutando consulta: %w", err)
	}
	defer rows.Close()

	result, err := scanRows(rows, e.cfg.MaxRows, e.cfg.MaxColumns, e.cfg.MaxResultSizeBytes)
	duration := time.Since(start)
	result.DurationMS = duration.Milliseconds()
	result.SQLExecuted = finalSQL
	if err != nil {
		return nil, err
	}

	if duration >= e.cfg.SlowQueryThreshold {
		e.logger.Warn("consulta lenta", "duration_ms", duration.Milliseconds(), "sql", finalSQL)
	}

	return result, nil
}

func scanRows(rows *sql.Rows, maxRows, maxColumns int, maxBytes int64) (*Result, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("leyendo columnas: %w", err)
	}
	if len(cols) > maxColumns {
		return nil, fmt.Errorf("la consulta devuelve %d columnas, el máximo permitido es %d", len(cols), maxColumns)
	}

	result := &Result{Columns: cols}
	var approxBytes int64

	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}

		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("leyendo fila: %w", err)
		}

		rowMap := make(map[string]any, len(cols))
		for i, col := range cols {
			v := normalizeValue(values[i])
			rowMap[col] = v
			approxBytes += estimateSize(v)
		}

		if approxBytes > maxBytes {
			result.Truncated = true
			break
		}

		result.Rows = append(result.Rows, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterando resultados: %w", err)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

func normalizeValue(v any) any {
	switch b := v.(type) {
	case []byte:
		return string(b)
	default:
		return v
	}
}

func estimateSize(v any) int64 {
	switch t := v.(type) {
	case string:
		return int64(len(t))
	default:
		return 8
	}
}
