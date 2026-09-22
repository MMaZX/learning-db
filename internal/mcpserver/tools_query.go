package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func registerQueryTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("consultar_base_datos",
			mcp.WithDescription("Ejecuta una consulta SQL de solo lectura (SELECT/UNION) contra la base de datos. Se valida, se limita automáticamente el número de filas y el tiempo de ejecución, y queda registrada en el historial. Cualquier sentencia de escritura o múltiples sentencias es rechazada."),
			mcp.WithString("sql", mcp.Required(), mcp.Description("Sentencia SQL SELECT a ejecutar")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			SQL string `json:"sql"`
		}) (*mcp.CallToolResult, error) {
			result, err := deps.Engine.Execute(ctx, args.SQL)

			entry := store.QueryHistoryEntry{Tool: "consultar_base_datos", SQL: args.SQL}
			if err != nil {
				entry.Error = err.Error()
				_, _ = deps.Store.RecordQuery(ctx, entry)
				recordAudit(ctx, deps, "consultar_base_datos", "consulta_rechazada_o_fallida", mustJSON(map[string]string{"sql": args.SQL}), err.Error())
				return toolError(err)
			}

			entry.DurationMS = result.DurationMS
			entry.RowCount = result.RowCount
			entry.Truncated = result.Truncated
			_, _ = deps.Store.RecordQuery(ctx, entry)
			recordAudit(ctx, deps, "consultar_base_datos", "consulta_ejecutada",
				mustJSON(map[string]any{"sql": result.SQLExecuted, "filas": result.RowCount, "truncado": result.Truncated}), "")

			var b strings.Builder
			if result.RowCount > 0 {
				b.WriteString(formatRowsTSV(result.Columns, result.Rows))
			}
			fmt.Fprintf(&b, "total_filas: %d\ntruncado: %t\nduracion_ms: %d\nsql_ejecutado: %s",
				result.RowCount, result.Truncated, result.DurationMS, result.SQLExecuted)
			return mcp.NewToolResultText(b.String()), nil
		}),
	)

	s.AddTool(
		mcp.NewTool("explicar_consulta",
			mcp.WithDescription("Explica qué hace una consulta SQL sin ejecutarla ni modificarla: tablas usadas, joins, filtros, agregaciones y el plan EXPLAIN de MySQL. Además señala en 'advertencias' patrones que suelen degradar el rendimiento (SELECT *, funciones envolviendo una columna en el WHERE, OFFSET grande, ausencia de WHERE/LIMIT). Útil para revisar una consulta antes de ejecutarla con consultar_base_datos."),
			mcp.WithString("sql", mcp.Required(), mcp.Description("Sentencia SQL SELECT a analizar")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			SQL string `json:"sql"`
		}) (*mcp.CallToolResult, error) {
			result, err := deps.Engine.Explain(ctx, args.SQL)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(map[string]any{
				"sql":            result.SQL,
				"tablas":         result.Tables,
				"joins":          result.Joins,
				"tiene_where":    result.HasWhere,
				"agregaciones":   result.Aggregations,
				"tiene_group_by": result.HasGroupBy,
				"tiene_order_by": result.HasOrderBy,
				"advertencias":   result.Warnings,
				"plan_explain":   result.ExplainPlan,
			})
		}),
	)
}
