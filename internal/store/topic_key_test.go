package store

import (
	"context"
	"strings"
	"testing"
)

func TestTopicKey(t *testing.T) {
	cases := []struct{ subject, context, want string }{
		{"vendedor", "ventas", "vendedor@ventas"},
		{"  Vendedor ", "VENTAS", "vendedor@ventas"},
		{"Gestión de Cobranza", "Cuentas  por   Cobrar", "gestion-de-cobranza@cuentas-por-cobrar"},
		{"Año Fiscal", "Contabilidad", "ano-fiscal@contabilidad"},
		{"cortesia", "", "cortesia@"},
		{"a@b", "c", "a-b@c"},
	}
	for _, c := range cases {
		if got := TopicKey(c.subject, c.context); got != c.want {
			t.Errorf("TopicKey(%q,%q) = %q, quería %q", c.subject, c.context, got, c.want)
		}
	}
}

func TestProposeKnowledge_SetsTopicKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := s.ProposeKnowledge(ctx, Knowledge{Subject: "Vendedor", Context: "Ventas", Claim: "x", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	k, _ := s.GetKnowledge(ctx, id)
	if k.TopicKey != "vendedor@ventas" {
		t.Errorf("topic_key = %q", k.TopicKey)
	}
}

func TestApproveKnowledge_SecondValidatedSameConceptFails(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a, _ := s.ProposeKnowledge(ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "a", Version: 1})
	b, _ := s.ProposeKnowledge(ctx, Knowledge{Subject: "Vendedor", Context: "ventas", Claim: "b", Version: 1})
	if _, err := s.ApproveKnowledge(ctx, a, "admin", ""); err != nil {
		t.Fatal(err)
	}
	_, err := s.ApproveKnowledge(ctx, b, "admin", "")
	if err == nil || !strings.Contains(err.Error(), "vendedor@ventas") {
		t.Fatalf("esperaba error de colisión de topic_key, obtuve %v", err)
	}
	kb, _ := s.GetKnowledge(ctx, b)
	if kb.Status != KnowledgeStatusProposed {
		t.Errorf("la propuesta en colisión debe seguir proposed, está %s", kb.Status)
	}
}

func TestApproveKnowledge_SupersedingVersionDeprecatesPrevious(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	v1, _ := s.ProposeKnowledge(ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "v1", Version: 1})
	if _, err := s.ApproveKnowledge(ctx, v1, "admin", ""); err != nil {
		t.Fatal(err)
	}
	v2, _ := s.ProposeKnowledge(ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "v2", Version: 2, SupersedesID: &v1})
	if _, err := s.ApproveKnowledge(ctx, v2, "admin", ""); err != nil {
		t.Fatalf("aprobar la versión que reemplaza debe funcionar: %v", err)
	}
	old, _ := s.GetKnowledge(ctx, v1)
	if old.Status != KnowledgeStatusDeprecated || old.Claim != "v1" {
		t.Errorf("v1 debe quedar deprecated e intacta, status=%s claim=%q", old.Status, old.Claim)
	}
	found, err := s.FindValidatedKnowledge(ctx, "Vendedor", "ventas")
	if err != nil || len(found) != 1 || found[0].ID != v2 {
		t.Fatalf("FindValidatedKnowledge debe devolver solo v2: %v %v", found, err)
	}
	if found[0].SupersedesID == nil || *found[0].SupersedesID != v1 {
		t.Error("el historial SupersedesID debe conservarse")
	}
}

func TestRejectedManyTimesSameTopicKey(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		id, _ := s.ProposeKnowledge(ctx, Knowledge{Subject: "x", Context: "y", Claim: "c", Version: 1})
		if _, err := s.RejectKnowledge(ctx, id, "admin", ""); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnsureTopicKeys_BackfillsAndHandlesLegacyDuplicates(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// Simula una base previa: sin índice y sin topic_key.
	if _, err := s.db.ExecContext(ctx, `DROP INDEX idx_knowledge_topic_key_validated`); err != nil {
		t.Fatal(err)
	}
	ins := func(subject, context_, status string, version int) int64 {
		res, err := s.db.ExecContext(ctx, `INSERT INTO knowledge (subject, context, claim, status, version) VALUES (?, ?, 'c', ?, ?)`,
			subject, context_, status, version)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	v1 := ins("Gestión", "Ventas", "validated", 1)
	v2 := ins("gestion", "ventas", "validated", 2)
	rej := ins("gestion", "ventas", "rejected", 1)

	for i := 0; i < 2; i++ { // idempotente
		if err := s.ensureTopicKeys(ctx); err != nil {
			t.Fatalf("ensureTopicKeys #%d: %v", i, err)
		}
	}
	get := func(id int64) *Knowledge { k, _ := s.GetKnowledge(ctx, id); return k }
	if get(v2).TopicKey != "gestion@ventas" {
		t.Errorf("la versión mayor debe conservar la clave, tiene %q", get(v2).TopicKey)
	}
	if get(v1).TopicKey != "" || get(v1).Status != KnowledgeStatusValidated {
		t.Errorf("el duplicado antiguo queda validated sin clave: %+v", get(v1))
	}
	if get(rej).TopicKey != "gestion@ventas" {
		t.Errorf("las filas no validadas reciben clave, tiene %q", get(rej).TopicKey)
	}
	found, err := s.FindValidatedKnowledge(ctx, "Gestión", "Ventas")
	if err != nil || len(found) != 2 || found[0].ID != v2 {
		t.Fatalf("lookup debe ver ambas validadas, v2 primero: %v %v", found, err)
	}
}
