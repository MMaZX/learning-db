package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func newProgressiveClient(t *testing.T) (context.Context, *store.Store, *client.Client) {
	t.Helper()
	ctx := context.Background()
	metadata, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })
	c, err := client.NewInProcessClient(New(&Deps{Store: metadata, Knowledge: knowledge.NewStore(t.TempDir())}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	if _, err := c.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	return ctx, metadata, c
}

func callJSON(t *testing.T, ctx context.Context, c *client.Client, tool string, args map[string]any) map[string]any {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = tool
	req.Params.Arguments = args
	res, err := c.CallTool(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("%s falló: %+v", tool, res)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func seedValidated(t *testing.T, ctx context.Context, s *store.Store, subject, ctxName, claim, source string) int64 {
	t.Helper()
	id, err := s.ProposeKnowledge(ctx, store.Knowledge{Subject: subject, Context: ctxName, Claim: claim, SourceJSON: source, Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
		t.Fatal(err)
	}
	return id
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestRecordarContexto_DefaultShapeUnchanged(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	seedValidated(t, ctx, s, "vendedor", "ventas", "ventas.codVendedor identifica al vendedor", `{"columnas":["ventas.codVendedor"]}`)

	for _, args := range []map[string]any{
		{"tarea": "quién es el vendedor", "contexto": "ventas"},
		{"tarea": "quién es el vendedor", "contexto": "ventas", "detalle": "completo"},
	} {
		out := callJSON(t, ctx, c, "recordar_contexto", args)
		if len(out) != 3 {
			t.Fatalf("claves top-level inesperadas: %v", keysOf(out))
		}
		for _, k := range []string{"memoria_encontrada", "conocimiento_validado", "instruccion"} {
			if _, ok := out[k]; !ok {
				t.Fatalf("falta %q en %v", k, keysOf(out))
			}
		}
		item := out["conocimiento_validado"].([]any)[0].(map[string]any)
		for _, k := range []string{"id", "concepto", "afirmacion", "estado", "version", "creado_en", "contexto", "fuente"} {
			if _, ok := item[k]; !ok {
				t.Fatalf("falta %q en item completo: %v", k, keysOf(item))
			}
		}
		if _, ok := item["afirmacion_preview"]; ok {
			t.Fatal("el modo completo no debe traer afirmacion_preview")
		}
	}
}

func TestRecordarContexto_CompactShape(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	long := strings.Repeat("palabra ", 40) + "final"
	seedValidated(t, ctx, s, "vendedor", "ventas", long,
		`{"columnas":["ventas.codVendedor","vendedores.nombre"],"relacion":{"origen":"ventas.codVendedor","destino":"vendedores.id"}}`)
	if _, err := s.UpsertEntityAlias(ctx, "vendedores", "", "empleados de venta", "admin"); err != nil {
		t.Fatal(err)
	}

	out := callJSON(t, ctx, c, "recordar_contexto", map[string]any{"tarea": "vendedor de una venta", "contexto": "ventas", "detalle": "compacto"})
	if out["memoria_encontrada"] != true || out["siguiente_paso"] == nil || out["instruccion"] == nil {
		t.Fatalf("respuesta compacta incompleta: %v", out)
	}
	if !strings.Contains(out["siguiente_paso"].(string), "obtener_conocimiento_por_id") {
		t.Fatalf("siguiente_paso no apunta a obtener_conocimiento_por_id: %v", out["siguiente_paso"])
	}
	item := out["conocimiento_validado"].([]any)[0].(map[string]any)
	if _, ok := item["afirmacion"]; ok {
		t.Fatal("el compacto no debe traer la afirmación completa")
	}
	if item["truncado"] != true {
		t.Fatalf("truncado debería ser true: %v", item)
	}
	preview := item["afirmacion_preview"].(string)
	if n := utf8.RuneCountInString(preview); n > previewMaxRunes+1 {
		t.Fatalf("preview demasiado largo (%d runas)", n)
	}
	tablas := item["tablas"].([]any)
	// Orden alfabético y sin duplicados.
	if len(tablas) != 2 || tablas[0] != "vendedores" || tablas[1] != "ventas" {
		t.Fatalf("tablas inesperadas: %v", tablas)
	}
	if item["tablas_alias"].(map[string]any)["vendedores"] != "empleados de venta" {
		t.Fatalf("tablas_alias inesperado: %v", item["tablas_alias"])
	}
}

func TestPreviewAfirmacion(t *testing.T) {
	if p, tr := previewAfirmacion("  corto  "); p != "corto" || tr {
		t.Fatalf("corto: %q %v", p, tr)
	}
	// Corte a mitad de palabra: retrocede al último espacio.
	s := strings.Repeat("a", 150) + " " + strings.Repeat("b", 30)
	p, tr := previewAfirmacion(s)
	if !tr || p != strings.Repeat("a", 150)+"…" {
		t.Fatalf("no cortó en límite de palabra: %q", p)
	}
	// Runas multibyte: nunca corta a mitad de runa.
	m := strings.Repeat("ñ", 300)
	p, tr = previewAfirmacion(m)
	if !tr || !utf8.ValidString(p) || utf8.RuneCountInString(p) != previewMaxRunes+1 {
		t.Fatalf("multibyte: valid=%v runas=%d", utf8.ValidString(p), utf8.RuneCountInString(p))
	}
	// Exactamente en el límite: no se trunca.
	if _, tr := previewAfirmacion(strings.Repeat("x", previewMaxRunes)); tr {
		t.Fatal("no debería truncar en el límite exacto")
	}
	// Palabra completa justo en el límite seguida de espacio.
	e := strings.Repeat("a", previewMaxRunes-1) + " " + "resto"
	if p, tr := previewAfirmacion(e); !tr || p != strings.Repeat("a", previewMaxRunes-1)+"…" {
		t.Fatalf("límite con espacio: %q", p)
	}
}

func TestObtenerConocimientoPorID_OnlyValidated(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	valid := seedValidated(t, ctx, s, "vendedor", "ventas", "v", `{"columnas":["ventas.codVendedor"]}`)
	proposed, err := s.ProposeKnowledge(ctx, store.Knowledge{Subject: "cliente", Context: "ventas", Claim: "p", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := s.ProposeKnowledge(ctx, store.Knowledge{Subject: "margen", Context: "ventas", Claim: "r", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RejectKnowledge(ctx, rejected, "admin", "no"); err != nil {
		t.Fatal(err)
	}

	out := callJSON(t, ctx, c, "obtener_conocimiento_por_id", map[string]any{"ids": []any{valid, proposed, rejected, 9999}})
	items := out["conocimiento_validado"].([]any)
	if len(items) != 1 || int64(items[0].(map[string]any)["id"].(float64)) != valid {
		t.Fatalf("solo debe volver el validado: %v", items)
	}
	if _, ok := items[0].(map[string]any)["afirmacion"]; !ok {
		t.Fatal("el item debe ser el completo")
	}
	if len(out["no_encontrados"].([]any)) != 3 {
		t.Fatalf("no_encontrados inesperado: %v", out["no_encontrados"])
	}

	// Cap y vacío se rechazan.
	for _, ids := range [][]any{{}, make([]any, maxIDsPorConsulta+1)} {
		for i := range ids {
			ids[i] = i + 1
		}
		req := mcp.CallToolRequest{}
		req.Params.Name = "obtener_conocimiento_por_id"
		req.Params.Arguments = map[string]any{"ids": ids}
		res, err := c.CallTool(ctx, req)
		if err != nil || !res.IsError {
			t.Fatalf("debía rechazar %d ids: %v %+v", len(ids), err, res)
		}
	}
}
