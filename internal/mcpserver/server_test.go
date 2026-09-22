package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func TestServerAdvertisesAndServesMemoryRecall(t *testing.T) {
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	id, err := metadata.ProposeKnowledge(ctx, store.Knowledge{
		Subject: "almacén", Context: "movimientos de stock",
		Claim: "movimientostock.codAlmacen identifica el almacén", Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
		t.Fatal(err)
	}

	mcpServer := New(&Deps{
		Store:     metadata,
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

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	initialized, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(initialized.Instructions, "recordar_contexto") {
		t.Fatalf("las instrucciones de inicio no ordenan recuperar memoria: %q", initialized.Instructions)
	}

	tools, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	foundTool := false
	for _, tool := range tools.Tools {
		if tool.Name == "recordar_contexto" {
			foundTool = true
			break
		}
	}
	if !foundTool {
		t.Fatal("recordar_contexto no aparece en tools/list")
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = "recordar_contexto"
	request.Params.Arguments = map[string]any{
		"tarea": "¿Cuál fue el último movimiento de almacen?",
	}
	result, err := mcpClient.CallTool(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("recordar_contexto falló: %+v", result)
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("respuesta inesperada: %T", result.Content[0])
	}
	if !strings.Contains(textContent.Text, `"memoria_encontrada":true`) ||
		!strings.Contains(textContent.Text, `"concepto":"almacén"`) {
		t.Fatalf("la tool no devolvió el recuerdo esperado: %s", textContent.Text)
	}
}
