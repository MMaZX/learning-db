// Package mcpserver expone las tools MCP que conectan a los asistentes de IA
// con las capas internas (schema, query engine, knowledge, learning). Es la
// única capa que habla el protocolo MCP; todo lo demás es agnóstico de él.
package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/query"
	"github.com/fulanito/db-intelligence-mcp/internal/schema"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

const version = "0.2.0"

const serverInstructions = `Este servidor tiene memoria persistente de negocio entre sesiones.
Al recibir una petición sobre datos o procesos de negocio, llama primero a recordar_contexto con la petición completa del usuario, antes de explorar el schema o escribir SQL.
Si recordar_contexto devuelve conocimiento validado, reutiliza directamente sus tablas, columnas, relaciones y fórmulas; no repitas el descubrimiento ni vuelvas a pedir ese contexto al usuario.
Explora únicamente lo que la memoria no cubra. Las propuestas y observaciones no son hechos confiables hasta que un humano las apruebe.`

// Deps son las dependencias compartidas por todos los handlers de tools.
type Deps struct {
	Inspector *schema.Inspector
	Engine    *query.Engine
	Knowledge *knowledge.Store
	Store     *store.Store
	Logger    *slog.Logger
	MaxRows   int
}

// New construye el servidor MCP con todas las tools de la v1 registradas.
func New(deps *Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"db-intelligence",
		version,
		server.WithToolCapabilities(true),
		server.WithLogging(),
		server.WithInstructions(serverInstructions),
		server.WithToolHandlerMiddleware(toolUsageMiddleware(deps)),
	)

	registerSchemaTools(s, deps)
	registerQueryTools(s, deps)
	registerKnowledgeTools(s, deps)
	registerLearningTools(s, deps)
	registerTelemetryTools(s, deps)

	seedToolUsage(s, deps)

	return s
}

// toolUsageMiddleware envuelve TODAS las tools registradas (a diferencia de
// recordAudit, que solo cubre las que cada handler llama explícitamente) y
// cuenta cada llamada en tool_usage. Es la única forma de responder "cuántas
// veces se usa cada herramienta" con confianza: no depende de que ningún
// handler nuevo recuerde instrumentarse.
func toolUsageMiddleware(deps *Deps) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := next(ctx, req)
			isError := err != nil || (result != nil && result.IsError)
			if recErr := deps.Store.RecordToolUsage(ctx, req.Params.Name, isError); recErr != nil && deps.Logger != nil {
				deps.Logger.Warn("no se pudo registrar telemetría de uso de tool", "tool", req.Params.Name, "error", recErr)
			}
			return result, err
		}
	}
}

// seedToolUsage crea una fila en 0 para cada tool que quedó registrada, para
// que obtener_estadisticas_uso siempre liste el catálogo completo -- también
// las tools que nunca se han llamado -- en vez de solo las que ya generaron
// tráfico. Falla en silencio (solo log) porque no debe impedir arrancar el
// servidor: en el peor caso, las tools sin uso aún no aparecen listadas
// hasta su primera llamada real.
func seedToolUsage(s *server.MCPServer, deps *Deps) {
	names := make([]string, 0, len(s.ListTools()))
	for name := range s.ListTools() {
		names = append(names, name)
	}
	if err := deps.Store.SeedToolUsage(context.Background(), names); err != nil && deps.Logger != nil {
		deps.Logger.Warn("no se pudo sembrar tool_usage", "error", err)
	}
}

// toJSONResult serializa cualquier resultado de tool como JSON compacto (sin
// indentar): el consumidor es el modelo, no un parser estricto, así que los
// espacios y saltos de línea de MarshalIndent son puro desperdicio de tokens.
func toJSONResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("error interno serializando la respuesta: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func toolError(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}
