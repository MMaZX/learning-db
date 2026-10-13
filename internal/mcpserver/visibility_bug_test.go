package mcpserver

import (
	"testing"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

// TestVisibilityBug_User3Teaches_User2Recalls reproduce el reporte del usuario (S3 / T4):
// "cuando la persona 3 por ejemplo le enseña algo y la persona 2 lo quiere leer
// al parecer no trae el conocimiento de la persona 3 sino que solo le lista el
// conocimiento que le enseño la persona 2 unicamente".
//
// Este test comprueba la hipótesis planteada en S3:
// Lo enseñado por el usuario 3 queda en estado 'proposed' y recordar_contexto
// filtra únicamente por 'validated', por lo que el usuario 2 no recibe el
// conocimiento enseñado por el usuario 3 a menos que sea previamente aprobado.
func TestVisibilityBug_User3Teaches_User2Recalls(t *testing.T) {
	ctx, s, c := newProgressiveClient(t)

	user2 := &store.User{ID: 2, Username: "persona 2", Role: store.RoleUser}
	user3 := &store.User{ID: 3, Username: "persona 3", Role: store.RoleUser}

	ctxUser2 := ContextWithUser(ctx, user2)
	ctxUser3 := ContextWithUser(ctx, user3)

	// 1. Persona 2 tiene conocimiento validado previo
	prop2 := callJSON(t, ctxUser2, c, "proponer_conocimiento", map[string]any{
		"concepto": "descuento", "contexto": "ventas", "afirmacion": "descuento del 10% en ventas",
	})
	id2 := int64(prop2["id"].(float64))
	callJSON(t, ctxUser2, c, "aprobar_conocimiento", map[string]any{
		"id": id2, "aprobado_por": "persona 2",
	})

	// 2. Persona 3 enseña un nuevo concepto ("vendedor")
	prop3 := callJSON(t, ctxUser3, c, "proponer_conocimiento", map[string]any{
		"concepto": "vendedor", "contexto": "ventas", "afirmacion": "vendedor sale de pedido.usuario_id",
	})
	id3 := int64(prop3["id"].(float64))

	// Verificamos que id3 quedó como 'proposed'
	k3, err := s.GetKnowledge(ctx, id3)
	if err != nil || k3 == nil {
		t.Fatalf("error recuperando conocimiento 3: %v", err)
	}
	if k3.Status != store.KnowledgeStatusProposed {
		t.Fatalf("se esperaba status='proposed', obtenido: %s", k3.Status)
	}

	// recall devuelve los ids de conocimiento validado que recibe persona 2.
	recall := func(t *testing.T) map[int64]bool {
		t.Helper()
		res := callJSON(t, ctxUser2, c, "recordar_contexto", map[string]any{
			"tarea": "quién es el vendedor de una venta", "contexto": "ventas",
		})
		ids := map[int64]bool{}
		list, _ := res["conocimiento_validado"].([]any)
		for _, item := range list {
			ids[int64(item.(map[string]any)["id"].(float64))] = true
		}
		return ids
	}

	t.Run("reproduccion_sin_aprobar_queda_oculto", func(t *testing.T) {
		ids := recall(t)
		if !ids[id2] {
			t.Fatalf("persona 2 debería ver su propio conocimiento validado (id=%d), obtenido: %v", id2, ids)
		}
		if ids[id3] {
			t.Fatalf("persona 2 no debería ver el conocimiento de persona 3 (id=%d) mientras siga en status='proposed'", id3)
		}
	})

	t.Run("una_vez_aprobado_es_visible", func(t *testing.T) {
		callJSON(t, ctxUser2, c, "aprobar_conocimiento", map[string]any{
			"id": id3, "aprobado_por": "persona 2",
		})
		ids := recall(t)
		if !ids[id3] {
			t.Fatalf("persona 2 debería ver el conocimiento de persona 3 (id=%d) una vez aprobado, obtenido: %v", id3, ids)
		}
	})
}
