package mcpserver

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// renderJarvisSkill devuelve la plantilla como especificación reutilizable:
// sustituye la versión y descarta la línea de la petición, que solo aplica al
// prompt MCP `jarvis`.
func renderJarvisSkill() string {
	lines := strings.Split(jarvisPromptTemplate, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if strings.Contains(l, "{{peticion}}") {
			continue
		}
		kept = append(kept, l)
	}
	out := strings.TrimRight(strings.Join(kept, "\n"), "\n") + "\n"
	return strings.ReplaceAll(out, "{{version}}", version)
}

func registerSkillTools(s *server.MCPServer, _ *Deps) {
	s.AddTool(
		mcp.NewTool("update_skill",
			mcp.WithDescription("Entrega el contenido actualizado de la skill de Jarvis (instrucciones de identidad, comportamiento y uso de herramientas para bases de datos) junto con el identificador de versión del servidor. Diseñada para que clientes sin terminal o en entornos móviles puedan leer y actualizar la skill directamente en la conversación."),
		),
		mcp.NewTypedToolHandler(func(_ context.Context, _ mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, error) {
			rendered := renderJarvisSkill()
			return toJSONResult(map[string]any{
				"skill":         "jarvis",
				"version":       version,
				"contenido":     rendered,
				"instrucciones": "Actualiza la skill o prompt de la conversación con este contenido para guiar el comportamiento del asistente de base de datos.",
			})
		}),
	)
}
