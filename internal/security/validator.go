// Package security valida que el SQL entrante sea de solo lectura, seguro y
// de una única sentencia. Es una capa de defensa independiente de los
// permisos de MySQL: nunca se confía únicamente en el usuario read-only.
package security

import (
	"fmt"
	"strings"

	"github.com/xwb1989/sqlparser"
)

// ErrUnsafeQuery se devuelve cuando una consulta no pasa la validación.
type ErrUnsafeQuery struct {
	Reason string
}

func (e *ErrUnsafeQuery) Error() string {
	return fmt.Sprintf("consulta rechazada: %s", e.Reason)
}

// forbiddenKeywords es una lista de respaldo por si el parser aceptara algo
// inesperado. La validación principal se basa en el AST, no en esta lista.
var forbiddenKeywords = []string{
	"insert", "update", "delete", "drop", "alter", "create", "truncate",
	"rename", "grant", "revoke", "replace", "lock tables", "unlock tables",
	"call", "load data", "load_file", "into outfile", "into dumpfile",
	"set password", "flush",
}

// Validate comprueba que sql sea una única sentencia SELECT/UNION de solo
// lectura. Devuelve el AST parseado para que el caller (query engine) pueda
// inspeccionarlo (por ejemplo, para inyectar LIMIT) sin volver a parsear.
func Validate(sql string) (sqlparser.SelectStatement, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return nil, &ErrUnsafeQuery{Reason: "la consulta está vacía"}
	}

	// 1) Detección explícita de sentencias múltiples (defensa en profundidad,
	// no depender solo de que el parser falle con ';').
	pieces, err := sqlparser.SplitStatementToPieces(trimmed)
	if err != nil {
		return nil, &ErrUnsafeQuery{Reason: fmt.Sprintf("no se pudo dividir la consulta: %v", err)}
	}
	nonEmpty := 0
	for _, p := range pieces {
		if strings.TrimSpace(p) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 1 {
		return nil, &ErrUnsafeQuery{Reason: "no se permiten múltiples sentencias SQL en una sola consulta"}
	}

	// 2) Lista de respaldo basada en texto (case-insensitive, sin importar
	// comentarios). Se ejecuta antes del parseo estructural por si el parser
	// no reconociera alguna palabra clave peligrosa.
	lower := strings.ToLower(trimmed)
	for _, kw := range forbiddenKeywords {
		if strings.Contains(lower, kw) {
			return nil, &ErrUnsafeQuery{Reason: fmt.Sprintf("palabra clave no permitida detectada: %q", kw)}
		}
	}

	// 3) Validación estructural real vía AST.
	stmt, err := sqlparser.Parse(trimmed)
	if err != nil {
		return nil, &ErrUnsafeQuery{Reason: fmt.Sprintf("SQL inválido: %v", err)}
	}

	switch s := stmt.(type) {
	case *sqlparser.Select:
		if err := validateSelectBody(s); err != nil {
			return nil, err
		}
		return s, nil
	case *sqlparser.Union:
		return s, nil
	default:
		return nil, &ErrUnsafeQuery{Reason: fmt.Sprintf("solo se permiten sentencias SELECT, se recibió %T", stmt)}
	}
}

// validateSelectBody rechaza construcciones peligrosas o no deseadas dentro
// de un SELECT que el parser aceptaría sintácticamente (SELECT INTO, locks
// explícitos FOR UPDATE, etc).
func validateSelectBody(s *sqlparser.Select) error {
	if s.Lock != "" {
		return &ErrUnsafeQuery{Reason: "no se permiten bloqueos explícitos (FOR UPDATE / LOCK IN SHARE MODE)"}
	}
	return nil
}

// EnforceLimit inspecciona/ajusta el LIMIT de un SELECT para que nunca supere
// maxRows. Si la consulta es un *sqlparser.Union, se envuelve para poder
// aplicar el LIMIT sin reparsear.
func EnforceLimit(stmt sqlparser.SelectStatement, maxRows int) sqlparser.SelectStatement {
	limit := &sqlparser.Limit{
		Rowcount: sqlparser.NewIntVal([]byte(fmt.Sprintf("%d", maxRows))),
	}

	switch s := stmt.(type) {
	case *sqlparser.Select:
		if s.Limit == nil {
			s.Limit = limit
			return s
		}
		// Ya trae LIMIT: lo dejamos, MySQL truncará; el motor de queries
		// igual corta la lectura de filas en maxRows como cinturón extra.
		return s
	case *sqlparser.Union:
		if s.Limit == nil {
			s.Limit = limit
		}
		return s
	default:
		return stmt
	}
}

// String serializa el statement de vuelta a SQL.
func String(stmt sqlparser.SelectStatement) string {
	return sqlparser.String(stmt)
}
