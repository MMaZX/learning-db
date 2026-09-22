package store

import (
	"context"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestObservation_IsNotKnowledge(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	obsID, err := s.RecordObservation(ctx, Observation{
		Subject: "pedido.estado", Kind: "observation",
		Statement: "estado=10 aparece junto con precio=0 en 40 filas",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Una observación nunca debe aparecer como conocimiento pendiente o
	// validado: son tablas y conceptos distintos por diseño.
	pending, err := s.PendingKnowledge(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("una observation no debería crear conocimiento pendiente automáticamente, got %d", len(pending))
	}

	obs, err := s.GetObservations(ctx, []int64{obsID})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].Kind != "observation" {
		t.Errorf("esperaba recuperar la observation registrada")
	}
}

func TestKnowledgeLifecycle_ProposedNeverAutoValidates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.ProposeKnowledge(ctx, Knowledge{
		Subject: "cortesia", Claim: "pedido.estado = 10 representa una cortesía",
		Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	k, err := s.GetKnowledge(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if k.Status != KnowledgeStatusProposed {
		t.Fatalf("status = %q, want %q (nunca 'validated' automáticamente)", k.Status, KnowledgeStatusProposed)
	}

	pending, err := s.PendingKnowledge(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("esperaba 1 propuesta pendiente, got %d", len(pending))
	}

	approved, err := s.ApproveKnowledge(ctx, id, "admin", "confirmado con evidencia")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != KnowledgeStatusValidated {
		t.Errorf("status tras aprobar = %q, want validated", approved.Status)
	}
	if approved.DecidedBy != "admin" {
		t.Errorf("decided_by = %q, want admin", approved.DecidedBy)
	}

	// Ya no debe aparecer como pendiente.
	pending, err = s.PendingKnowledge(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("tras aprobar no debería quedar pendiente, got %d", len(pending))
	}
}

func TestKnowledgeRejection_KeepsHistory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.ProposeKnowledge(ctx, Knowledge{Subject: "x", Claim: "y", Version: 1})
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := s.RejectKnowledge(ctx, id, "admin", "sin evidencia suficiente")
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != KnowledgeStatusRejected {
		t.Errorf("status = %q, want rejected", rejected.Status)
	}

	// El registro se conserva (no se borra), solo cambia de estado.
	again, err := s.GetKnowledge(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if again == nil || again.Claim != "y" {
		t.Error("el registro rechazado debería seguir existiendo íntegro")
	}
}

func TestKnowledgeVersioning_SupersedesPreviousWithoutOverwriting(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	v1ID, err := s.ProposeKnowledge(ctx, Knowledge{Subject: "cortesia", Claim: "v1", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveKnowledge(ctx, v1ID, "admin", ""); err != nil {
		t.Fatal(err)
	}

	v2ID, err := s.ProposeKnowledge(ctx, Knowledge{Subject: "cortesia", Claim: "v2, más precisa", Version: 2, SupersedesID: &v1ID})
	if err != nil {
		t.Fatal(err)
	}

	v1, err := s.GetKnowledge(ctx, v1ID)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Claim != "v1" {
		t.Error("v1 no debería haberse sobrescrito al proponer v2")
	}

	v2, err := s.GetKnowledge(ctx, v2ID)
	if err != nil {
		t.Fatal(err)
	}
	if v2.SupersedesID == nil || *v2.SupersedesID != v1ID {
		t.Error("v2 debería referenciar a v1 vía SupersedesID")
	}
	if v2.Status != KnowledgeStatusProposed {
		t.Error("v2 debe nacer como proposed aunque v1 ya esté validado")
	}
}

func TestFindValidatedKnowledge_IsScopedByContext(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ventasID, err := s.ProposeKnowledge(ctx, Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim: "pedido.usuario_id -> users.id", Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveKnowledge(ctx, ventasID, "admin", ""); err != nil {
		t.Fatal(err)
	}

	// Sin conocimiento validado para "vendedor" en el contexto "compras": no
	// debe extrapolarse desde "ventas".
	found, err := s.FindValidatedKnowledge(ctx, "vendedor", "compras")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("no debería reutilizar conocimiento de 'ventas' para el contexto 'compras', got %+v", found)
	}

	found, err = s.FindValidatedKnowledge(ctx, "vendedor", "ventas")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("esperaba 1 resultado para vendedor/ventas, got %d", len(found))
	}
}

func TestApproveKnowledge_FailsIfNotProposed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.ProposeKnowledge(ctx, Knowledge{Subject: "x", Claim: "y", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
		t.Fatal(err)
	}
	// Segunda aprobación sobre algo que ya no está 'proposed' debe fallar.
	if _, err := s.ApproveKnowledge(ctx, id, "admin", ""); err == nil {
		t.Error("esperaba error al aprobar dos veces la misma propuesta")
	}
}

func TestRecallValidatedKnowledge_FromNaturalLanguageTask(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	stockID, err := s.ProposeKnowledge(ctx, Knowledge{
		Subject: "almacén", Context: "movimientos de stock",
		Claim: "movimientostock.codAlmacen identifica el almacén del movimiento", Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApproveKnowledge(ctx, stockID, "admin", ""); err != nil {
		t.Fatal(err)
	}

	proposedID, err := s.ProposeKnowledge(ctx, Knowledge{
		Subject: "estado interno", Context: "movimientos de stock",
		Claim: "hipótesis todavía no confirmada", Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = proposedID

	found, err := s.RecallValidatedKnowledge(ctx, "¿Cuál fue el último movimiento de almacen?", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("esperaba recuperar 1 recuerdo validado, got %d: %+v", len(found), found)
	}
	if found[0].ID != stockID {
		t.Fatalf("recuperó id=%d, want id=%d", found[0].ID, stockID)
	}
	if found[0].Status != KnowledgeStatusValidated {
		t.Fatalf("status=%q, want validated", found[0].Status)
	}
}

func TestRecallValidatedKnowledge_PrioritizesContext(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, k := range []Knowledge{
		{Subject: "producto", Context: "ventas", Claim: "producto vendido", Version: 1},
		{Subject: "producto", Context: "movimientos de stock", Claim: "producto del movimiento", Version: 1},
	} {
		id, err := s.ProposeKnowledge(ctx, k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.ApproveKnowledge(ctx, id, "admin", ""); err != nil {
			t.Fatal(err)
		}
	}

	found, err := s.RecallValidatedKnowledge(ctx, "muéstrame el producto", "ventas", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Context != "ventas" {
		t.Fatalf("esperaba priorizar contexto ventas, got %+v", found)
	}
}
