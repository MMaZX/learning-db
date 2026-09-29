package store

import (
	"context"
	"fmt"
	"testing"
)

func proposeK(t *testing.T, s *Store, k Knowledge) int64 {
	t.Helper()
	k.Version = 1
	id, err := s.ProposeKnowledge(context.Background(), k)
	if err != nil {
		t.Fatalf("ProposeKnowledge: %v", err)
	}
	return id
}

// seedFiller agrega filas validadas sin relación con nada: bm25 de FTS5
// depende del tamaño del corpus (IDF), así que los tests necesitan un
// corpus realista para que los términos compartidos pesen.
func seedFiller(t *testing.T, s *Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		seedValidatedKnowledge(t, s, context.Background(), Knowledge{
			Subject: fmt.Sprintf("relleno%d", i), Context: fmt.Sprintf("zona%d", i),
			Claim: fmt.Sprintf("dato%d vive en tabla%d", i, i),
		})
	}
}

func relationsByType(rels []KnowledgeRelation) map[string][]int64 {
	out := map[string][]int64{}
	for _, r := range rels {
		out[r.Relation] = append(out[r.Relation], r.ToID)
	}
	return out
}

func hasRelation(rels []KnowledgeRelation, relation string, to int64) bool {
	for _, r := range rels {
		if r.Relation == relation && r.ToID == to {
			return true
		}
	}
	return false
}

func TestDetectRelations_SameTopicValidatedAndSupersedesCandidate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	validated := seedValidatedKnowledge(t, s, ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "viene de pedido.usuario_id"})

	fresh := proposeK(t, s, Knowledge{Subject: "Vendedor", Context: "ventas", Claim: "viene de pedido.vendedor_id"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRelation(rels, RelationSameTopic, validated) || !hasRelation(rels, RelationSupersedesCandidate, validated) {
		t.Fatalf("esperaba same_topic y supersedes_candidate hacia %d, got %+v", validated, rels)
	}

	// Si la propuesta ya apunta con supersedes_id, no es "candidata".
	sup := validated
	explicit := proposeK(t, s, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "otra version", SupersedesID: &sup})
	rels, err = s.DetectKnowledgeRelations(ctx, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRelation(rels, RelationSameTopic, validated) {
		t.Errorf("esperaba same_topic, got %+v", rels)
	}
	if hasRelation(rels, RelationSupersedesCandidate, validated) {
		t.Errorf("supersedes_id ya apunta a la fila: no debe haber supersedes_candidate, got %+v", rels)
	}
}

func TestDetectRelations_SameTopicProposedAndRejected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	proposed := proposeK(t, s, Knowledge{Subject: "cliente", Context: "crm", Claim: "tabla clientes"})
	rejected := proposeK(t, s, Knowledge{Subject: "cliente", Context: "crm", Claim: "tabla personas"})
	if _, err := s.RejectKnowledge(ctx, rejected, "admin", "no es esa tabla"); err != nil {
		t.Fatal(err)
	}

	fresh := proposeK(t, s, Knowledge{Subject: "cliente", Context: "crm", Claim: "tabla customers"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRelation(rels, RelationSameTopic, proposed) {
		t.Errorf("proposed misma clave -> same_topic, got %+v", rels)
	}
	if hasRelation(rels, RelationSupersedesCandidate, proposed) {
		t.Errorf("proposed no es candidata a reemplazo, got %+v", rels)
	}
	if !hasRelation(rels, RelationPreviouslyRejected, rejected) {
		t.Errorf("rejected misma clave -> previously_rejected, got %+v", rels)
	}
	if hasRelation(rels, RelationSameTopic, rejected) {
		t.Errorf("rejected no debe generar same_topic, got %+v", rels)
	}
}

func TestDetectRelations_IgnoresDeprecatedAndSelf(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	old := seedValidatedKnowledge(t, s, ctx, Knowledge{Subject: "margen", Context: "finanzas", Claim: "precio menos costo"})
	// Aprobar una versión que reemplaza a la anterior la depreca.
	sup := old
	newer := proposeK(t, s, Knowledge{Subject: "margen", Context: "finanzas", Claim: "precio menos costo menos descuento", SupersedesID: &sup})
	newer2 := newer
	if _, err := s.ApproveKnowledge(ctx, newer2, "admin", "ok"); err != nil {
		t.Fatal(err)
	}
	if k, _ := s.GetKnowledge(ctx, old); k.Status != KnowledgeStatusDeprecated {
		t.Fatalf("precondición: %d debería estar deprecated, es %s", old, k.Status)
	}

	fresh := proposeK(t, s, Knowledge{Subject: "margen", Context: "finanzas", Claim: "precio menos costo menos descuento"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rels {
		if r.ToID == old {
			t.Errorf("fila deprecated no debe relacionarse: %+v", r)
		}
		if r.ToID == fresh {
			t.Errorf("la fila no debe relacionarse consigo misma: %+v", r)
		}
	}
	if !hasRelation(rels, RelationSameTopic, newer) {
		t.Errorf("esperaba same_topic con la validada vigente, got %+v", rels)
	}
}

func TestDetectRelations_PossibleConflictByFTS(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedFiller(t, s, 12)
	validated := seedValidatedKnowledge(t, s, ctx, Knowledge{Subject: "facturacion mensual", Context: "finanzas", Claim: "la facturacion mensual sale de la tabla invoices"})
	proposed := proposeK(t, s, Knowledge{Subject: "facturacion mensual", Context: "contabilidad", Claim: "la facturacion mensual sale de billing_docs"})

	fresh := proposeK(t, s, Knowledge{Subject: "facturacion mensual", Context: "ventas", Claim: "la facturacion mensual sale de la tabla facturas"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []int64{validated, proposed} {
		if !hasRelation(rels, RelationPossibleConflict, other) {
			t.Errorf("esperaba possible_conflict con %d, got %+v", other, rels)
		}
	}
	for _, r := range rels {
		if r.Relation == RelationPossibleConflict && (r.Score == nil || *r.Score < relationScoreFloor) {
			t.Errorf("score debe existir y superar el umbral: %+v", r)
		}
	}
}

func TestDetectRelations_PreviouslyRejectedByFTS(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedFiller(t, s, 12)
	rejected := proposeK(t, s, Knowledge{Subject: "descuento maximo", Context: "ventas", Claim: "el descuento maximo permitido es 30 por ciento"})
	if _, err := s.RejectKnowledge(ctx, rejected, "admin", "es 20"); err != nil {
		t.Fatal(err)
	}

	fresh := proposeK(t, s, Knowledge{Subject: "descuento maximo", Context: "comercial", Claim: "el descuento maximo permitido es 30 por ciento"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if !hasRelation(rels, RelationPreviouslyRejected, rejected) {
		t.Fatalf("esperaba previously_rejected por FTS, got %+v", rels)
	}
	if hasRelation(rels, RelationPossibleConflict, rejected) {
		t.Errorf("rejected no debe ser possible_conflict, got %+v", rels)
	}
}

func TestDetectRelations_ScoreFloorExcludesWeakMatches(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// "informacion" aparece en todas las filas: su IDF es ~0 y una
	// coincidencia solo por ese término no debe superar el umbral.
	for i := 0; i < 30; i++ {
		seedValidatedKnowledge(t, s, ctx, Knowledge{
			Subject: fmt.Sprintf("concepto%d", i), Context: fmt.Sprintf("area%d", i),
			Claim: fmt.Sprintf("dato%d se guarda en tabla%d con informacion general", i, i),
		})
	}
	fresh := proposeK(t, s, Knowledge{Subject: "otra cosa", Context: "zzz", Claim: "informacion"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 0 {
		t.Errorf("un término común suelto no debe generar relaciones, got %+v", rels)
	}
}

func TestDetectRelations_CapsFTSRelations(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedFiller(t, s, 20)
	for i := 0; i < maxFTSRelations+4; i++ {
		seedValidatedKnowledge(t, s, ctx, Knowledge{
			Subject: "inventario valorizado", Context: fmt.Sprintf("bodega%d", i),
			Claim: "el inventario valorizado se calcula con costo promedio",
		})
	}
	fresh := proposeK(t, s, Knowledge{Subject: "inventario valorizado", Context: "central", Claim: "el inventario valorizado se calcula con costo promedio"})
	rels, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(relationsByType(rels)[RelationPossibleConflict]); got != maxFTSRelations {
		t.Errorf("possible_conflict = %d, want tope %d", got, maxFTSRelations)
	}
}

func TestDetectRelations_Idempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedValidatedKnowledge(t, s, ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "viene de pedido"})
	fresh := proposeK(t, s, Knowledge{Subject: "vendedor", Context: "ventas", Claim: "viene de otra parte"})

	first, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.DetectKnowledgeRelations(ctx, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("redetectar debe devolver lo mismo: %d vs %d", len(first), len(second))
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge_relations WHERE from_id = ?`, fresh).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != len(first) {
		t.Errorf("filas persistidas = %d, want %d (sin duplicados)", count, len(first))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].ID == 0 {
			t.Errorf("ids deben ser estables: %+v vs %+v", first[i], second[i])
		}
	}
}

func TestRelationsFor_BatchWithLiveStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	related := proposeK(t, s, Knowledge{Subject: "cliente", Context: "crm", Claim: "tabla clientes"})
	a := proposeK(t, s, Knowledge{Subject: "cliente", Context: "crm", Claim: "tabla customers"})
	b := proposeK(t, s, Knowledge{Subject: "unico", Context: "solo", Claim: "sin relaciones"})
	if _, err := s.DetectKnowledgeRelations(ctx, a); err != nil {
		t.Fatal(err)
	}

	got, err := s.RelationsFor(ctx, []int64{a, b, a})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[a]) != 1 || got[a][0].ID != related || got[a][0].Status != KnowledgeStatusProposed {
		t.Fatalf("esperaba 1 relación proposed hacia %d, got %+v", related, got[a])
	}
	if _, ok := got[b]; ok {
		t.Errorf("id sin relaciones no debe aparecer: %+v", got[b])
	}

	// El estado se calcula en vivo: al rechazar la fila relacionada cambia.
	if _, err := s.RejectKnowledge(ctx, related, "admin", "duplicada"); err != nil {
		t.Fatal(err)
	}
	got, err = s.RelationsFor(ctx, []int64{a})
	if err != nil {
		t.Fatal(err)
	}
	r := got[a][0]
	if r.Status != KnowledgeStatusRejected || r.DecidedBy != "admin" || r.DecisionNote != "duplicada" || r.DecidedAt == nil {
		t.Errorf("estado en vivo incorrecto: %+v", r)
	}

	empty, err := s.RelationsFor(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Errorf("sin ids -> mapa vacío, got %+v err=%v", empty, err)
	}
}

func TestMigration0007_AppliesOnExistingDB(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	id := proposeK(t, s, Knowledge{Subject: "x", Context: "y", Claim: "z"})
	s.Close()

	// Reabrir sobre la base existente: las migraciones ya aplicadas se saltan.
	s, err = Open(ctx, dir)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer s.Close()
	if _, err := s.DetectKnowledgeRelations(ctx, id); err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = '0007_knowledge_relations.sql'`).Scan(&applied); err != nil || applied != 1 {
		t.Errorf("migración 0007 aplicada = %d err=%v", applied, err)
	}
}
