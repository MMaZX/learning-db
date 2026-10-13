package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func newPreferencesTestServer(t *testing.T) (*store.Store, *client.Client) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	mcpServer := New(&Deps{
		Store:     st,
		Knowledge: knowledge.NewStore(t.TempDir()),
	})

	mcpClient, err := client.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mcpClient.Close() })

	if err := mcpClient.Start(ctx); err != nil {
		t.Fatal(err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0"}
	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		t.Fatal(err)
	}

	return st, mcpClient
}

func TestRecordandoConfiguracion_SinPreferencias(t *testing.T) {
	st, mcpClient := newPreferencesTestServer(t)
	ctx := context.Background()

	// Crear usuario autenticado
	u, err := st.CreateUser(ctx, "carlos", store.HashToken("token-carlos"))
	if err != nil {
		t.Fatalf("creando usuario: %v", err)
	}
	userCtx := ContextWithUser(ctx, u)

	req := mcp.CallToolRequest{}
	req.Params.Name = "recordando_configuracion"

	res, err := mcpClient.CallTool(userCtx, req)
	if err != nil {
		t.Fatalf("llamando a recordando_configuracion: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("resultado con error inesperado: %+v", res)
	}

	textContent, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("contenido inesperado: %T", res.Content[0])
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("parseando respuesta JSON: %v, texto: %s", err, textContent.Text)
	}

	if data["migrado"] != false {
		t.Fatalf("se esperaba migrado=false, obtenido: %v", data["migrado"])
	}
	if data["usuario"] != "carlos" {
		t.Fatalf("se esperaba usuario=carlos, obtenido: %v", data["usuario"])
	}
	if !strings.Contains(textContent.Text, "migrar_preferencias") {
		t.Fatalf("se esperaba sugerencia de migrar_preferencias en el texto: %s", textContent.Text)
	}
}

func TestRecordandoConfiguracion_ConPreferencias(t *testing.T) {
	st, mcpClient := newPreferencesTestServer(t)
	ctx := context.Background()

	u, err := st.CreateUser(ctx, "elena", store.HashToken("token-elena"))
	if err != nil {
		t.Fatalf("creando usuario: %v", err)
	}
	userCtx := ContextWithUser(ctx, u)

	prefContent := `{"estilo_respuesta":"conciso","idioma":"es","modo_jarvis":true}`
	if _, err := st.SaveUserPreferences(ctx, u.ID, prefContent); err != nil {
		t.Fatalf("guardando preferencias: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "recordando_configuracion"

	res, err := mcpClient.CallTool(userCtx, req)
	if err != nil {
		t.Fatalf("llamando a recordando_configuracion: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("resultado con error inesperado: %+v", res)
	}

	textContent, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("contenido inesperado: %T", res.Content[0])
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("parseando respuesta JSON: %v, texto: %s", err, textContent.Text)
	}

	if data["tiene_preferencias"] != true {
		t.Fatalf("se esperaba tiene_preferencias=true, obtenido: %v", data["tiene_preferencias"])
	}
	if data["migrado"] != false {
		t.Fatalf("guardadas sin migración: se esperaba migrado=false, obtenido: %v", data["migrado"])
	}
	if data["usuario"] != "elena" {
		t.Fatalf("se esperaba usuario=elena, obtenido: %v", data["usuario"])
	}

	prefsMap, ok := data["preferencias"].(map[string]any)
	if !ok {
		t.Fatalf("se esperaba mapa de preferencias, obtenido: %T (%+v)", data["preferencias"], data["preferencias"])
	}
	if prefsMap["estilo_respuesta"] != "conciso" || prefsMap["idioma"] != "es" {
		t.Fatalf("preferencias recuperadas no coinciden: %+v", prefsMap)
	}
}
