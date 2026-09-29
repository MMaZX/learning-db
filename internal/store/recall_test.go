package store

import (
	"context"
	"testing"
)

// seedValidatedKnowledge crea y aprueba conocimiento en un solo paso, tal
// como lo dejaría un aprobar_conocimiento real, para no repetir el
// propose+approve en cada test de recall.
func seedValidatedKnowledge(t *testing.T, s *Store, ctx context.Context, k Knowledge) int64 {
	t.Helper()
	k.Version = 1
	if k.SourceJSON == "" {
		// Datos reales siempre traen source_json (aprender_del_usuario lo
		// rellena con tablas/columnas/relación); se fija aquí para que las
		// semillas se parezcan a datos reales.
		k.SourceJSON = `{"tables":["seed_table"]}`
	}
	id, err := s.ProposeKnowledge(ctx, k)
	if err != nil {
		t.Fatalf("ProposeKnowledge: %v", err)
	}
	if _, err := s.ApproveKnowledge(ctx, id, "admin", "seed de test"); err != nil {
		t.Fatalf("ApproveKnowledge: %v", err)
	}
	return id
}

func TestRecallValidatedKnowledge_FTSHitsKnownQueries(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim: "El vendedor de una venta es pedido.usuario_id -> users.id",
	})
	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "movimientos de stock", Context: "inventario",
		Claim: "Los movimientos de stock se registran en stock_movements",
	})

	found, err := s.RecallValidatedKnowledge(ctx, "vendedor en ventas", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("esperaba al menos un resultado para 'vendedor en ventas'")
	}
	if found[0].Subject != "vendedor" {
		t.Errorf("subject = %q, want vendedor (mejor match primero)", found[0].Subject)
	}

	found, err = s.RecallValidatedKnowledge(ctx, "movimientos de stock", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].Subject != "movimientos de stock" {
		t.Fatalf("esperaba encontrar 'movimientos de stock', got %+v", found)
	}
}

func TestRecallValidatedKnowledge_AccentInsensitive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "gestión de turnos", Context: "rrhh",
		Claim: "La gestión de turnos vive en shifts",
	})

	found, err := s.RecallValidatedKnowledge(ctx, "gestion de turnos", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].Subject != "gestión de turnos" {
		t.Fatalf("query sin tilde debería encontrar 'gestión de turnos', got %+v", found)
	}
}

func TestRecallValidatedKnowledge_ExcludesNonValidated(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Propuesto pero NUNCA aprobado: no debe aparecer en recall aunque
	// coincida perfectamente con la query.
	if _, err := s.ProposeKnowledge(ctx, Knowledge{
		Subject: "comision", Context: "ventas",
		Claim:   "La comisión del vendedor es un 5% sobre la venta",
		Version: 1,
	}); err != nil {
		t.Fatal(err)
	}

	found, err := s.RecallValidatedKnowledge(ctx, "comision del vendedor", "ventas", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range found {
		if k.Subject == "comision" {
			t.Fatalf("conocimiento propuesto (no validado) no debería aparecer en recall: %+v", k)
		}
	}
}

func TestRecallValidatedKnowledge_UnknownQueryReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim: "pedido.usuario_id -> users.id",
	})

	found, err := s.RecallValidatedKnowledge(ctx, "algo totalmente desconocido xyzqwerty", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("query sin relación no debería devolver resultados, got %+v", found)
	}
}

func TestRecallValidatedKnowledge_FTSSpecialCharsDoNotError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim: "pedido.usuario_id -> users.id",
	})

	// Comillas, *, -, paréntesis y el operador NEAR son sintaxis especial de
	// FTS5: la sanitización (normalizar y luego citar cada token) debe
	// evitar que produzcan un error de sintaxis MATCH.
	queries := []string{
		`vendedor" OR 1=1--`,
		`vendedor*`,
		`vendedor NEAR/2 ventas`,
		`(vendedor OR ventas)`,
		`"vendedor`,
		`---***`,
	}
	for _, q := range queries {
		if _, err := s.RecallValidatedKnowledge(ctx, q, "", 20); err != nil {
			t.Errorf("query %q no debería producir error, got: %v", q, err)
		}
	}
}

func TestRecallValidatedKnowledge_TriggersReflectUpdatesAndDeletes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id := seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "encargado", Context: "logistica",
		Claim: "El encargado de turno es warehouse.manager_id",
	})

	found, err := s.RecallValidatedKnowledge(ctx, "encargado de turno logistica", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("esperaba encontrar 'encargado' antes de modificarlo")
	}

	// Actualiza subject/claim/context directamente (trigger knowledge_au) a
	// un concepto sin relación con la búsqueda anterior. Cambia también
	// context: si quedara "logistica" el fallback en Go seguiría
	// matcheando por ese campo y el test no aislaría el efecto del trigger.
	if _, err := s.db.ExecContext(ctx, `UPDATE knowledge SET subject = ?, claim = ?, context = ? WHERE id = ?`,
		"proveedor", "El proveedor principal es supplier.id", "compras", id); err != nil {
		t.Fatalf("actualizando knowledge directamente: %v", err)
	}

	found, err = s.RecallValidatedKnowledge(ctx, "encargado de turno logistica", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range found {
		if k.ID == id {
			t.Fatalf("el índice FTS debería reflejar el UPDATE (trigger knowledge_au), got %+v", k)
		}
	}

	found, err = s.RecallValidatedKnowledge(ctx, "proveedor principal", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 || found[0].ID != id {
		t.Fatalf("tras el UPDATE debería encontrarse por 'proveedor principal', got %+v", found)
	}

	// Borra la fila (trigger knowledge_ad) y verifica que desaparece sin
	// que el recall falle.
	if _, err := s.db.ExecContext(ctx, `DELETE FROM knowledge WHERE id = ?`, id); err != nil {
		t.Fatalf("borrando knowledge directamente: %v", err)
	}

	found, err = s.RecallValidatedKnowledge(ctx, "proveedor principal", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatalf("tras el DELETE no debería encontrarse más, got %+v", found)
	}
}

func TestRecallValidatedKnowledge_FallsBackToGoScoringOnFTSError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedValidatedKnowledge(t, s, ctx, Knowledge{
		Subject: "vendedor", Context: "ventas",
		Claim: "pedido.usuario_id -> users.id",
	})

	// Rompe el índice FTS a propósito: MATCH sobre una tabla inexistente
	// falla, así que RecallValidatedKnowledge debe caer al scoring en Go
	// (LIKE/contains) y seguir devolviendo el resultado correcto.
	if _, err := s.db.ExecContext(ctx, `DROP TABLE knowledge_fts`); err != nil {
		t.Fatalf("dropeando knowledge_fts: %v", err)
	}

	found, err := s.RecallValidatedKnowledge(ctx, "vendedor en ventas", "", 20)
	if err != nil {
		t.Fatalf("el fallback en Go no debería fallar aunque FTS esté roto: %v", err)
	}
	if len(found) == 0 || found[0].Subject != "vendedor" {
		t.Fatalf("esperaba que el fallback en Go encontrara 'vendedor', got %+v", found)
	}
}

func TestRecallContains_EmptyMatchesNothing(t *testing.T) {
	if recallContains("", "ventas") || recallContains("ventas", "") || recallContains("", "") {
		t.Error("un campo o término vacío no debe coincidir")
	}
	if !recallContains("gestion de ventas", "ventas") || !recallContains("ven", "ventas") {
		t.Error("la coincidencia mutua no vacía debe seguir funcionando")
	}
}

func TestScoreRecallCandidate_EmptySourceJSONGivesNoScore(t *testing.T) {
	k := Knowledge{Subject: "zzz", Claim: "yyy", SourceJSON: ""}
	if got := scoreRecallCandidate(k, "vendedor ventas", "", []string{"vendedor", "ventas"}); got != 0 {
		t.Errorf("score = %d, quería 0 (SourceJSON vacío no debe puntuar)", got)
	}
}
