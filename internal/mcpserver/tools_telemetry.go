package mcpserver

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTelemetryTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("obtener_estadisticas_uso",
			mcp.WithDescription("Devuelve cuántas veces se ha llamado a cada tool de este servidor MCP (incluidas las que nunca se han usado, con contador 0), ordenadas de más a menos usada. Útil para saber qué tools valen la pena mantener/documentar y cuáles nadie está usando en la práctica."),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, error) {
			stats, err := deps.Store.ToolUsageStats(ctx)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(map[string]any{
				"tools": toolUsageAJSON(stats),
			})
		}),
	)
}
