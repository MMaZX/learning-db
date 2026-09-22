package query

import (
	"context"
	"fmt"
	"strconv"

	"github.com/xwb1989/sqlparser"

	"github.com/fulanito/db-intelligence-mcp/internal/security"
)

// offsetWarningThreshold es a partir de cuántas filas de OFFSET se advierte:
// MySQL igual tiene que escanear y descartar esas filas antes de llegar al
// LIMIT, así que un OFFSET grande es costoso aunque el LIMIT sea pequeño.
const offsetWarningThreshold = 1000

// ExplainResult resume qué hace una consulta sin modificarla ni devolver sus
// datos: tablas, joins, filtros, agregaciones y el plan EXPLAIN de MySQL.
type ExplainResult struct {
	Tables       []string         `json:"tables"`
	Joins        []string         `json:"joins,omitempty"`
	HasWhere     bool             `json:"has_where"`
	Aggregations []string         `json:"aggregations,omitempty"`
	HasGroupBy   bool             `json:"has_group_by"`
	HasOrderBy   bool             `json:"has_order_by"`
	Warnings     []string         `json:"warnings,omitempty"`
	ExplainPlan  []map[string]any `json:"explain_plan,omitempty"`
	SQL          string           `json:"sql"`
}

// Explain valida el SQL (misma capa de seguridad que Execute, nunca se
// ejecuta la consulta real) y ejecuta EXPLAIN para obtener el plan de MySQL,
// además de un resumen estructural extraído del AST.
func (e *Engine) Explain(ctx context.Context, rawSQL string) (*ExplainResult, error) {
	stmt, err := security.Validate(rawSQL)
	if err != nil {
		return nil, err
	}
	normalizedSQL := security.String(stmt)

	res := &ExplainResult{SQL: normalizedSQL}
	analyzeSelect(stmt, res)

	timeoutCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	rows, err := e.pool.QueryContext(timeoutCtx, "EXPLAIN "+normalizedSQL)
	if err != nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf("no se pudo obtener EXPLAIN: %v", err))
		return res, nil
	}
	defer rows.Close()

	planResult, err := scanRows(rows, 100, 50, 1<<20)
	if err == nil {
		res.ExplainPlan = planResult.Rows
	}

	return res, nil
}

func analyzeSelect(stmt sqlparser.SelectStatement, res *ExplainResult) {
	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		res.Warnings = append(res.Warnings, "es una consulta UNION; el análisis estructural detallado solo cubre SELECT simples")
		return
	}

	for _, te := range sel.From {
		walkTableExpr(te, res)
	}
	res.HasWhere = sel.Where != nil
	res.HasGroupBy = len(sel.GroupBy) > 0
	res.HasOrderBy = len(sel.OrderBy) > 0

	for _, se := range sel.SelectExprs {
		if aliased, ok := se.(*sqlparser.AliasedExpr); ok {
			if fn, ok := aliased.Expr.(*sqlparser.FuncExpr); ok {
				name := fn.Name.Lowered()
				switch name {
				case "count", "sum", "avg", "min", "max", "group_concat":
					res.Aggregations = append(res.Aggregations, name)
				}
			}
		}
	}

	if sel.Where == nil && sel.Limit == nil {
		res.Warnings = append(res.Warnings, "la consulta no tiene WHERE ni LIMIT explícito: puede escanear tablas completas")
	}

	if hasStarExpr(sel.SelectExprs) {
		res.Warnings = append(res.Warnings, "usa SELECT *: especifica las columnas necesarias para reducir bytes transferidos y no romper si cambia el schema")
	}

	if hasFunctionWrappedColumn(sel.Where) {
		res.Warnings = append(res.Warnings, "el WHERE aplica una función sobre una columna (ej. UPPER(columna) = ...): eso impide usar un índice normal sobre esa columna; compara contra el valor ya normalizado o crea un índice funcional/generado")
	}

	if n, ok := offsetValue(sel.Limit); ok && n > offsetWarningThreshold {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"OFFSET %d es costoso: MySQL escanea y descarta esas %d filas antes de aplicar el LIMIT; prefiere paginación por cursor (WHERE id > ultimo_id / WHERE fecha > ultima_fecha) en vez de OFFSET grande", n, n))
	}
}

// hasStarExpr detecta "SELECT *" o "SELECT tabla.*".
func hasStarExpr(exprs sqlparser.SelectExprs) bool {
	for _, se := range exprs {
		if _, ok := se.(*sqlparser.StarExpr); ok {
			return true
		}
	}
	return false
}

// hasFunctionWrappedColumn detecta una función SQL llamada directamente
// sobre una columna en el WHERE (ej. UPPER(email) = '...', YEAR(fecha) = 2024):
// ese patrón impide que MySQL use un índice normal sobre esa columna, porque
// el índice guarda el valor crudo, no el resultado de la función.
func hasFunctionWrappedColumn(where *sqlparser.Where) bool {
	if where == nil {
		return false
	}
	found := false
	_ = sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		if found {
			return false, nil
		}
		fn, ok := node.(*sqlparser.FuncExpr)
		if !ok {
			return true, nil
		}
		for _, arg := range fn.Exprs {
			aliased, ok := arg.(*sqlparser.AliasedExpr)
			if !ok {
				continue
			}
			if _, ok := aliased.Expr.(*sqlparser.ColName); ok {
				found = true
				return false, nil
			}
		}
		return true, nil
	}, where.Expr)
	return found
}

// offsetValue extrae el valor numérico de un OFFSET literal (ej. "LIMIT 20
// OFFSET 5000"). Si el offset no es un entero literal (parámetro, expresión),
// no se puede evaluar estáticamente y se devuelve ok=false.
func offsetValue(limit *sqlparser.Limit) (int, bool) {
	if limit == nil || limit.Offset == nil {
		return 0, false
	}
	val, ok := limit.Offset.(*sqlparser.SQLVal)
	if !ok || val.Type != sqlparser.IntVal {
		return 0, false
	}
	n, err := strconv.Atoi(string(val.Val))
	if err != nil {
		return 0, false
	}
	return n, true
}

func walkTableExpr(te sqlparser.TableExpr, res *ExplainResult) {
	switch t := te.(type) {
	case *sqlparser.AliasedTableExpr:
		if tableName, ok := t.Expr.(sqlparser.TableName); ok {
			res.Tables = append(res.Tables, tableName.Name.String())
		}
	case *sqlparser.JoinTableExpr:
		walkTableExpr(t.LeftExpr, res)
		walkTableExpr(t.RightExpr, res)
		res.Joins = append(res.Joins, string(t.Join))
	case *sqlparser.ParenTableExpr:
		for _, inner := range t.Exprs {
			walkTableExpr(inner, res)
		}
	}
}
