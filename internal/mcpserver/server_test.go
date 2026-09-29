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

// TestRecordarContextoDescribedAsSingleRecallEntrypoint verifica el contrato
// de T2: recordar_contexto se presenta como la única/primera tool de
// recuerdo, y buscar_conocimiento / obtener_conocimiento_validado quedan
// explícitamente degradadas a auditoría amplia / verificación puntual (no
// recuerdo de sesión), sin que ninguna otra tool deje de existir.
func TestRecordarContextoDescribedAsSingleRecallEntrypoint(t *testing.T) {
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

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
	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		t.Fatal(err)
	}

	tools, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}

	descriptions := map[string]string{}
	for _, tool := range tools.Tools {
		descriptions[tool.Name] = tool.Description
	}

	recordar, ok := descriptions["recordar_contexto"]
	if !ok {
		t.Fatal("recordar_contexto no aparece en tools/list")
	}
	if !strings.Contains(recordar, "ÚNICA y PRIMERA") {
		t.Fatalf("recordar_contexto ya no se describe como única/primera tool de recuerdo: %q", recordar)
	}
	if !strings.Contains(recordar, "no necesitas una segunda llamada") {
		t.Fatalf("recordar_contexto no aclara que no hace falta una segunda llamada para recordar: %q", recordar)
	}

	buscar, ok := descriptions["buscar_conocimiento"]
	if !ok {
		t.Fatal("buscar_conocimiento no aparece en tools/list")
	}
	if !strings.Contains(buscar, "AMPLIA") || !strings.Contains(buscar, "No es la tool de recuerdo de sesión") {
		t.Fatalf("buscar_conocimiento no quedó degradada a auditoría amplia: %q", buscar)
	}

	validado, ok := descriptions["obtener_conocimiento_validado"]
	if !ok {
		t.Fatal("obtener_conocimiento_validado no aparece en tools/list")
	}
	if !strings.Contains(validado, "PUNTUAL, no recuerdo de sesión") {
		t.Fatalf("obtener_conocimiento_validado no quedó degradada a verificación puntual: %q", validado)
	}

	// T2 no quita ni renombra tools existentes: solo aclara prioridad.
	const expectedToolCount = 20
	if len(tools.Tools) != expectedToolCount {
		t.Fatalf("se esperaban %d tools registradas, hay %d: %v", expectedToolCount, len(tools.Tools), toolNames(tools.Tools))
	}
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return names
}

// TestRecordarContextoIncludesSourceAndAliasWithoutSecondCall verifica que la
// respuesta de recordar_contexto ya trae, por cada item validado, su fuente
// (tablas/columnas/relación) y el alias resuelto, para que no haga falta una
// segunda llamada a obtener_conocimiento_validado solo para leer la fuente.
func TestRecordarContextoIncludesSourceAndAliasWithoutSecondCall(t *testing.T) {
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	id, err := metadata.ProposeKnowledge(ctx, store.Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim:      "ventas.codVendedor identifica al vendedor",
		SourceJSON: `{"columnas":["ventas.codVendedor"]}`,
		Version:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := metadata.UpsertEntityAlias(ctx, "ventas", "codVendedor", "vendedor", "admin"); err != nil {
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
	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		t.Fatal(err)
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = "recordar_contexto"
	request.Params.Arguments = map[string]any{
		"tarea":    "¿quién es el vendedor de esta venta?",
		"contexto": "ventas",
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
	if !strings.Contains(textContent.Text, `"fuente"`) {
		t.Fatalf("la respuesta no trae 'fuente', obligaría a una segunda llamada: %s", textContent.Text)
	}
	if !strings.Contains(textContent.Text, `"fuente_alias"`) || !strings.Contains(textContent.Text, `"vendedor"`) {
		t.Fatalf("la respuesta no trae el alias resuelto de la fuente: %s", textContent.Text)
	}
}

// TestRecordarContextoAcceptance cubre el criterio de aceptación global de la
// feature: con 2 conceptos validados sembrados, una tarea parafraseada (otras
// palabras, acentos y mayúsculas) acierta en 1 llamada con `fuente`; un
// concepto desconocido devuelve memoria_encontrada=false con la instrucción de
// preguntar al usuario + aprender_del_usuario; y tool_usage muestra que cada
// consulta de recuerdo costó exactamente 1 llamada (recordar_contexto).
func TestRecordarContextoAcceptance(t *testing.T) {
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	seed := []store.Knowledge{
		{Subject: "vendedor", Context: "ventas", Claim: "ventas.codVendedor identifica al vendedor",
			SourceJSON: `{"columnas":["ventas.codVendedor"]}`, Version: 1},
		{Subject: "movimientos de stock", Context: "almacen", Claim: "movimientostock registra cada entrada y salida de stock",
			SourceJSON: `{"tablas":["movimientostock"]}`, Version: 1},
	}
	for _, k := range seed {
		id, err := metadata.ProposeKnowledge(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := metadata.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
			t.Fatal(err)
		}
	}

	mcpServer := New(&Deps{Store: metadata, Knowledge: knowledge.NewStore(t.TempDir())})
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
	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		t.Fatal(err)
	}

	recall := func(tarea string) string {
		t.Helper()
		request := mcp.CallToolRequest{}
		request.Params.Name = "recordar_contexto"
		request.Params.Arguments = map[string]any{"tarea": tarea}
		result, err := mcpClient.CallTool(ctx, request)
		if err != nil || result.IsError || len(result.Content) == 0 {
			t.Fatalf("recordar_contexto falló: %v %+v", err, result)
		}
		text, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatalf("respuesta inesperada: %T", result.Content[0])
		}
		return text.Text
	}

	hits := []struct{ tarea, concepto string }{
		{"Necesito saber QUIÉN fue el VENDEDOR de la venta 123", `"concepto":"vendedor"`},
		{"muéstrame los Movimientos de STOCK del almacén de ayer", `"concepto":"movimientos de stock"`},
	}
	for _, h := range hits {
		out := recall(h.tarea)
		if !strings.Contains(out, `"memoria_encontrada":true`) || !strings.Contains(out, h.concepto) ||
			!strings.Contains(out, `"fuente"`) {
			t.Fatalf("tarea parafraseada %q sin acierto con fuente: %s", h.tarea, out)
		}
	}

	unknown := recall("cuál es el margen de utilidad neto por sucursal")
	if !strings.Contains(unknown, `"memoria_encontrada":false`) ||
		!strings.Contains(unknown, "aprender_del_usuario") ||
		!strings.Contains(unknown, "pregúntale al usuario") {
		t.Fatalf("concepto desconocido no instruye preguntar + aprender_del_usuario: %s", unknown)
	}

	stats, err := metadata.ToolUsageStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var recallCalls, otherCalls int64
	for _, u := range stats {
		if u.Tool == "recordar_contexto" {
			recallCalls = u.CallCount
		} else {
			otherCalls += u.CallCount
		}
	}
	if recallCalls != 3 || otherCalls != 0 {
		t.Fatalf("3 consultas deben costar 3 llamadas a recordar_contexto y 0 follow-ups: recordar=%d otras=%d", recallCalls, otherCalls)
	}
}
