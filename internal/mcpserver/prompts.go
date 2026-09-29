package mcpserver

import (
	"context"
	_ "embed"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// jarvisPromptName es el nombre del prompt MCP. El cliente lo expone con el
// prefijo del nombre con que registró el servidor (p. ej. en Claude Code,
// /mcp__<nombre-del-servidor>__jarvis).
const jarvisPromptName = "jarvis"

// jarvisPromptTemplate deriva de .claude/commands/jarvis.md (v1.3.0) sin la
// sección de auto-actualización (el prompt se versiona con el binario). DEBE
// mantenerse sincronizado con ese comando; TestJarvisPromptKeepsCommandFlowMarkers
// detecta la deriva más obvia. Marcadores: {{version}} y {{peticion}}.
//
//go:embed prompts/jarvis.md
var jarvisPromptTemplate string

// peticionVacia es lo que se sustituye cuando el cliente no envía `peticion`.
const peticionVacia = "(el usuario no indicó ninguna petición todavía: " +
	"salúdalo con el estilo de Jarvis y pregúntale qué necesita)"

// renderJarvisPrompt sustituye los marcadores del template.
func renderJarvisPrompt(peticion string) string {
	peticion = strings.TrimSpace(peticion)
	if peticion == "" {
		peticion = peticionVacia
	}
	// La petición se sustituye al final para que un {{version}} escrito por el
	// usuario no sea interpretado como marcador.
	out := strings.ReplaceAll(jarvisPromptTemplate, "{{version}}", version)
	return strings.ReplaceAll(out, "{{peticion}}", peticion)
}

// registerPrompts registra los prompts MCP del servidor.
func registerPrompts(s *server.MCPServer) {
	prompt := mcp.NewPrompt(jarvisPromptName,
		mcp.WithPromptDescription("Invoca a Jarvis (MCP de inteligencia sobre database_multi_final) para explorar el schema, consultar datos o enseñar/validar conocimiento de negocio"),
		mcp.WithArgument("peticion",
			mcp.ArgumentDescription("Tu pregunta o petición para Jarvis"),
			mcp.RequiredArgument(),
		),
	)
	s.AddPrompt(prompt, func(_ context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		text := renderJarvisPrompt(req.Params.Arguments["peticion"])
		return mcp.NewGetPromptResult(
			"Jarvis (db-intelligence v"+version+")",
			[]mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))},
		), nil
	})
}
