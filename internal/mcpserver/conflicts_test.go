package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func callRaw(t *testing.T, ctx context.Context, c *client.Client, tool string, args map[string]any) any {
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
	var out any
	if err := json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func conflictos(t *testing.T, m map[string]any) []any {
	t.Helper()
	v, ok := m["posibles_conflictos"]
	if !ok || v == nil {
		t.Fatalf("posibles_conflictos debe ser un arreglo, got %v en %v", v, keysOf(m))
	}
	return v.([]any)
}

func findConflict(list []any, relacion string) map[string]any {
	for _, it := range list {
		m := it.(map[string]any)
		if m["relacion"] == relacion {
			return m
		}
	}
	return nil
}

func TestProponer_SurfacesConflictWithValidated(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	validated := seedValidated(t, ctx, s, "vendedor", "ventas", "viene de pedido.usuario_id", `{}`)

	out := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{
		"concepto": "Vendedor", "contexto": "ventas", "afirmacion": "viene de pedido.vendedor_id",
	})
	list := conflictos(t, out)
	item := findConflict(list, "same_topic")
	if item == nil || int64(item["id"].(float64)) != validated {
		t.Fatalf("esperaba same_topic hacia %d, got %v", validated, list)
	}
	if item["estado"] != "validado" || item["concepto"] != "vendedor" || item["afirmacion_resumen"] == "" {
		t.Errorf("item incompleto: %v", item)
	}
	if _, ok := item["puntaje"]; ok {
		t.Errorf("same_topic no lleva puntaje: %v", item)
	}
	if _, ok := item["motivo_rechazo"]; ok {
		t.Errorf("solo previously_rejected lleva motivo: %v", item)
	}
	if findConflict(list, "supersedes_candidate") == nil {
		t.Errorf("esperaba supersedes_candidate: %v", list)
	}
}

func TestProponer_SurfacesPreviouslyRejected(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	rejected, err := s.ProposeKnowledge(ctx, store.Knowledge{Subject: "margen", Context: "reportes", Claim: "margen es precio menos costo", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RejectKnowledge(ctx, rejected, "ana", "el margen incluye descuentos"); err != nil {
		t.Fatal(err)
	}

	out := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{
		"concepto": "margen", "contexto": "reportes", "afirmacion": "margen = precio - costo",
	})
	item := findConflict(conflictos(t, out), "previously_rejected")
	if item == nil || int64(item["id"].(float64)) != rejected {
		t.Fatalf("esperaba previously_rejected hacia %d, got %v", rejected, out["posibles_conflictos"])
	}
	if item["estado"] != "rechazado" || item["motivo_rechazo"] != "el margen incluye descuentos" || item["rechazado_por"] != "ana" {
		t.Errorf("motivo/por incorrectos: %v", item)
	}
	if f, _ := item["fecha"].(string); f == "" {
		t.Errorf("falta fecha: %v", item)
	}
}

func TestProponer_NoConflictsIsEmptyArrayNotNull(t *testing.T) {
	ctx, _, c := newProgressiveClient(t)
	out := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{
		"concepto": "unico", "contexto": "solo", "afirmacion": "sin parecidos",
	})
	if list := conflictos(t, out); len(list) != 0 {
		t.Fatalf("esperaba [], got %v", list)
	}
}

func TestProponer_DetectionFailureDoesNotFailPropose(t *testing.T) {
	ctx, _, c := newProgressiveClient(t)
	orig := detectarRelaciones
	detectarRelaciones = func(context.Context, *store.Store, int64) ([]store.KnowledgeRelation, error) {
		return nil, errors.New("fallo simulado")
	}
	t.Cleanup(func() { detectarRelaciones = orig })

	out := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{
		"concepto": "x", "contexto": "y", "afirmacion": "z",
	})
	if out["estado"] != "propuesto" || out["id"] == nil {
		t.Fatalf("la propuesta debe crearse: %v", out)
	}
	if list := conflictos(t, out); len(list) != 0 {
		t.Fatalf("esperaba [] ante fallo, got %v", list)
	}
}

func TestPendiente_ItemsIncludeConflictsPerItem(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	validated := seedValidated(t, ctx, s, "vendedor", "ventas", "viene de pedido.usuario_id", `{}`)
	callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{"concepto": "vendedor", "contexto": "ventas", "afirmacion": "otra fuente"})
	callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{"concepto": "unico", "contexto": "solo", "afirmacion": "nada parecido"})

	raw := callRaw(t, ctx, c, "obtener_conocimiento_pendiente", map[string]any{})
	items, ok := raw.([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("la respuesta debe seguir siendo un arreglo de 2, got %v", raw)
	}
	var withConflict, without int
	for _, it := range items {
		m := it.(map[string]any)
		list := conflictos(t, m)
		if len(list) == 0 {
			without++
			continue
		}
		withConflict++
		if item := findConflict(list, "same_topic"); item == nil || int64(item["id"].(float64)) != validated {
			t.Errorf("conflicto inesperado: %v", list)
		}
	}
	if withConflict != 1 || without != 1 {
		t.Fatalf("esperaba 1 con conflicto y 1 sin, got %d/%d", withConflict, without)
	}
}

func TestAprobar_ResponseIncludesConflictsWithLiveStatus(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)
	validated := seedValidated(t, ctx, s, "vendedor", "ventas", "viene de pedido.usuario_id", `{}`)
	prop := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{"concepto": "vendedor", "contexto": "ventas", "afirmacion": "otra fuente", "reemplaza_a_id": validated})

	out := callJSON(t, ctx, c, "aprobar_conocimiento", map[string]any{"id": prop["id"], "aprobado_por": "admin"})
	item := findConflict(conflictos(t, out), "same_topic")
	if item == nil || int64(item["id"].(float64)) != validated {
		t.Fatalf("esperaba same_topic hacia %d, got %v", validated, out["posibles_conflictos"])
	}
	// Al aprobar, la versión anterior pasa a obsoleta: el estado es el vivo.
	if item["estado"] != "obsoleto" {
		t.Errorf("estado debería ser el vivo (obsoleto), got %v", item["estado"])
	}
}
