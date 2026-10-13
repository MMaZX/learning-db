package mcpserver

import (
	"context"
	"testing"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func TestAuthorAttributionFromContext(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)

	userAlice := &store.User{ID: 1, Username: "alice", Role: store.RoleUser}
	userBob := &store.User{ID: 2, Username: "bob", Role: store.RoleUser}
	userCharlie := &store.User{ID: 3, Username: "charlie", Role: store.RoleAdmin}

	aliceCtx := ContextWithUser(ctx, userAlice)
	bobCtx := ContextWithUser(ctx, userBob)
	charlieCtx := ContextWithUser(ctx, userCharlie)

	// 1. proponer_conocimiento con aliceCtx: es una propuesta basada en evidencia, no una enseñanza directa: taught_by queda vacío
	propRes := callJSON(t, aliceCtx, c, "proponer_conocimiento", map[string]any{
		"concepto": "descuento", "contexto": "ventas", "afirmacion": "descuento en porcentaje",
	})
	propID := int64(propRes["id"].(float64))
	k, err := s.GetKnowledge(ctx, propID)
	if err != nil || k == nil {
		t.Fatalf("error recuperando propuesta %d: %v", propID, err)
	}
	if k.TaughtBy != "" {
		t.Fatalf("taught_by esperado vacío en proponer_conocimiento, obtenido: %q", k.TaughtBy)
	}

	// 2. aprobar_conocimiento con bobCtx: decided_by debe ser "bob", prevaleciendo sobre aprobado_por="falso_admin"
	callJSON(t, bobCtx, c, "aprobar_conocimiento", map[string]any{
		"id": propID, "aprobado_por": "falso_admin", "nota": "aprobado por bob",
	})
	k, err = s.GetKnowledge(ctx, propID)
	if err != nil || k == nil {
		t.Fatalf("error recuperando conocimiento %d: %v", propID, err)
	}
	if k.DecidedBy != "bob" {
		t.Fatalf("decided_by en aprobación esperado 'bob', obtenido: %q", k.DecidedBy)
	}

	// 3. revocar_conocimiento con charlieCtx: decided_by debe ser "charlie", prevaleciendo sobre revocado_por="falso"
	callJSON(t, charlieCtx, c, "revocar_conocimiento", map[string]any{
		"id": propID, "revocado_por": "falso", "motivo": "ya no aplica",
	})
	k, err = s.GetKnowledge(ctx, propID)
	if err != nil || k == nil {
		t.Fatalf("error recuperando conocimiento %d: %v", propID, err)
	}
	if k.DecidedBy != "charlie" {
		t.Fatalf("decided_by en revocación esperado 'charlie', obtenido: %q", k.DecidedBy)
	}

	// 4. rechazar_conocimiento con bobCtx
	propRes2 := callJSON(t, aliceCtx, c, "proponer_conocimiento", map[string]any{
		"concepto": "comision", "contexto": "ventas", "afirmacion": "comision del 5%",
	})
	propID2 := int64(propRes2["id"].(float64))
	callJSON(t, bobCtx, c, "rechazar_conocimiento", map[string]any{
		"id": propID2, "rechazado_por": "falso_rechazo", "nota": "rechazado por bob",
	})
	k2, err := s.GetKnowledge(ctx, propID2)
	if err != nil || k2 == nil {
		t.Fatalf("error recuperando conocimiento %d: %v", propID2, err)
	}
	if k2.DecidedBy != "bob" {
		t.Fatalf("decided_by en rechazo esperado 'bob', obtenido: %q", k2.DecidedBy)
	}

	// 5. Fallback sin usuario en contexto (A4: compatibilidad)
	propRes3 := callJSON(t, ctx, c, "proponer_conocimiento", map[string]any{
		"concepto": "impuesto", "contexto": "ventas", "afirmacion": "igv 18%",
	})
	propID3 := int64(propRes3["id"].(float64))
	callJSON(t, ctx, c, "aprobar_conocimiento", map[string]any{
		"id": propID3, "aprobado_por": "operador_manual",
	})
	k3, err := s.GetKnowledge(ctx, propID3)
	if err != nil || k3 == nil {
		t.Fatalf("error recuperando conocimiento %d: %v", propID3, err)
	}
	if k3.DecidedBy != "operador_manual" {
		t.Fatalf("decided_by en fallback esperado 'operador_manual', obtenido: %q", k3.DecidedBy)
	}
}

// aprender_del_usuario y actualizar_alias usan resolveAuthor para taught_by y
// los alias: el usuario autenticado prevalece sobre ensenado_por (S2, A4).
func TestResolveAuthorServerValuePrevails(t *testing.T) {
	ctx := ContextWithUser(context.Background(), &store.User{ID: 1, Username: "alice", Role: store.RoleUser})
	if got := resolveAuthor(ctx, "falso"); got != "alice" {
		t.Fatalf("el usuario autenticado debe prevalecer, obtenido: %q", got)
	}
	if got := resolveAuthor(context.Background(), "operador_manual"); got != "operador_manual" {
		t.Fatalf("sin usuario debe usar el valor del parámetro, obtenido: %q", got)
	}
}
