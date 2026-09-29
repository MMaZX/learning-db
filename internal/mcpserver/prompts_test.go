package mcpserver

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func newPromptTestClient(t *testing.T) *client.Client {
	t.Helper()
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	mcpClient, err := client.NewInProcessClient(New(&Deps{
		Store:     metadata,
		Knowledge: knowledge.NewStore(t.TempDir()),
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mcpClient.Close() })
	if err := mcpClient.Start(ctx); err != nil {
		t.Fatal(err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	if _, err := mcpClient.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	return mcpClient
}

func getJarvisPrompt(t *testing.T, c *client.Client, args map[string]string) *mcp.GetPromptResult {
	t.Helper()
	req := mcp.GetPromptRequest{}
	req.Params.Name = "jarvis"
	req.Params.Arguments = args
	res, err := c.GetPrompt(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 1 || res.Messages[0].Role != mcp.RoleUser {
		t.Fatalf("se esperaba un único mensaje de rol user: %+v", res.Messages)
	}
	return res
}

func promptText(t *testing.T, res *mcp.GetPromptResult) string {
	t.Helper()
	text, ok := res.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatalf("contenido no es texto: %T", res.Messages[0].Content)
	}
	return text.Text
}

func TestJarvisPromptListedWithPeticionArgument(t *testing.T) {
	c := newPromptTestClient(t)
	ctx := context.Background()

	prompts, err := c.ListPrompts(ctx, mcp.ListPromptsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts.Prompts) != 1 || prompts.Prompts[0].Name != "jarvis" {
		t.Fatalf("prompts inesperados: %+v", prompts.Prompts)
	}
	p := prompts.Prompts[0]
	if p.Description == "" || len(p.Arguments) != 1 || p.Arguments[0].Name != "peticion" || !p.Arguments[0].Required {
		t.Fatalf("definición del prompt inesperada: %+v", p)
	}

	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 21 {
		t.Fatalf("los prompts no deben alterar las tools: %d", len(tools.Tools))
	}
}

func TestJarvisPromptSubstitutesPeticion(t *testing.T) {
	c := newPromptTestClient(t)
	peticion := "cuál fue la última venta"
	text := promptText(t, getJarvisPrompt(t, c, map[string]string{"peticion": peticion}))

	if !strings.Contains(text, "Petición del usuario: "+peticion) {
		t.Fatalf("no incluye la petición sustituida")
	}
	for _, bad := range []string{"$ARGUMENTS", "{{peticion}}", "{{version}}", "Auto-actualización", "mcp__jarvis__"} {
		if strings.Contains(text, bad) {
			t.Fatalf("el prompt no debe contener %q", bad)
		}
	}
	if !strings.Contains(text, "v"+version) {
		t.Fatalf("el prompt no menciona la versión del servidor %s", version)
	}
}

func TestJarvisPromptWithoutPeticionAsksUser(t *testing.T) {
	c := newPromptTestClient(t)
	text := promptText(t, getJarvisPrompt(t, c, nil))

	if !strings.Contains(text, "pregúntale qué necesita") || strings.Contains(text, "{{peticion}}") {
		t.Fatalf("sin petición debe indicar preguntar al usuario")
	}
}

// El prompt embebido deriva de .claude/commands/jarvis.md: si el flujo clave
// cambia en uno y no en el otro, este test lo delata.
func TestJarvisPromptKeepsCommandFlowMarkers(t *testing.T) {
	command, err := os.ReadFile("../../.claude/commands/jarvis.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"recordar_contexto", "obtener_conocimiento_validado", "aprender_del_usuario",
		"obtener_conocimiento_por_id", "posibles_conflictos", "previously_rejected",
		"consultar_base_datos", "obtener_conocimiento_negocio", `detalle: "compacto"`,
	} {
		if !strings.Contains(string(command), marker) {
			t.Errorf("el comando perdió el marcador %q", marker)
		}
		if !strings.Contains(jarvisPromptTemplate, marker) {
			t.Errorf("el prompt embebido no contiene el marcador %q del comando", marker)
		}
	}
}

// Tope de coste de contexto: el template no puede llegar a 100 líneas.
func TestJarvisPromptTemplateUnder100Lines(t *testing.T) {
	if n := strings.Count(jarvisPromptTemplate, "\n") + 1; n >= 100 {
		t.Fatalf("el template tiene %d líneas; el máximo es 99", n)
	}
}
