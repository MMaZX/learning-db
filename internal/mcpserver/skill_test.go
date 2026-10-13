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

func TestUpdateSkill_ReturnsEmbeddedContentAndVersion(t *testing.T) {
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

	req := mcp.CallToolRequest{}
	req.Params.Name = "update_skill"

	res, err := mcpClient.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("llamando a update_skill: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("resultado con error: %+v", res)
	}

	textContent, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("contenido inesperado: %T", res.Content[0])
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("error parseando JSON de update_skill: %v, texto: %s", err, textContent.Text)
	}

	if data["skill"] != "jarvis" {
		t.Fatalf("se esperaba skill='jarvis', obtenido: %v", data["skill"])
	}
	if data["version"] != version {
		t.Fatalf("se esperaba version=%s, obtenido: %v", version, data["version"])
	}

	contenido, ok := data["contenido"].(string)
	if !ok || len(contenido) == 0 {
		t.Fatalf("se esperaba contenido no vacío de la skill, obtenido: %v", data["contenido"])
	}

	if !strings.Contains(contenido, "Jarvis") || !strings.Contains(contenido, "database_multi_final") {
		t.Fatalf("el contenido devuelto no contiene las marcas de la skill jarvis: %s", contenido)
	}

	for _, placeholder := range []string{"{{peticion}}", "{{version}}"} {
		if strings.Contains(contenido, placeholder) {
			t.Fatalf("el contenido devuelto conserva el marcador sin resolver %s", placeholder)
		}
	}
	if strings.Contains(contenido, "Petición del usuario:") {
		t.Fatalf("el contenido devuelto conserva la línea de petición del usuario")
	}
}
