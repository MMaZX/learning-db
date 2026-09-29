package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newPassiveStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	s := newTestStore(t)
	clock := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	return s, &clock
}

func countRows(t *testing.T, s *Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCapturePassiveQuery_FirstCreatesObservationTablesOnly(t *testing.T) {
	s, _ := newPassiveStore(t)
	ctx := context.Background()

	created, err := s.CapturePassiveQuery(ctx, []string{"Pedido", "cliente", "pedido"})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	obs, err := s.SearchObservations(ctx, "consulta:", 10)
	if err != nil || len(obs) != 1 {
		t.Fatalf("obs=%v err=%v", obs, err)
	}
	o := obs[0]
	if o.Kind != "observation" || o.Agent != PassiveAgent || o.Subject != "consulta:cliente,pedido" {
		t.Fatalf("observación inesperada: %+v", o)
	}
	if o.EvidenceJSON != `{"tablas":["cliente","pedido"]}` {
		t.Fatalf("evidencia inesperada: %s", o.EvidenceJSON)
	}
	if strings.Contains(strings.ToLower(o.Statement+o.EvidenceJSON), "select") {
		t.Fatal("no debe guardarse SQL")
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM knowledge`); n != 0 {
		t.Fatalf("la captura pasiva nunca crea conocimiento, hay %d", n)
	}
}

func TestCapturePassiveQuery_DedupeWithinWindow(t *testing.T) {
	s, clock := newPassiveStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := s.CapturePassiveQuery(ctx, []string{"pedido"}); err != nil {
			t.Fatal(err)
		}
		*clock = clock.Add(time.Minute)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM observations`); n != 1 {
		t.Fatalf("observaciones=%d, quería 1", n)
	}
	if h := countRows(t, s, `SELECT hits FROM passive_capture WHERE table_set='pedido'`); h != 5 {
		t.Fatalf("hits=%d, quería 5", h)
	}
}

func TestCapturePassiveQuery_NewObservationAfterWindow(t *testing.T) {
	s, clock := newPassiveStore(t)
	ctx := context.Background()
	_, _ = s.CapturePassiveQuery(ctx, []string{"pedido"})
	*clock = clock.Add(passiveWindow - time.Minute)
	if created, _ := s.CapturePassiveQuery(ctx, []string{"pedido"}); created {
		t.Fatal("dentro de la ventana no debe crear")
	}
	*clock = clock.Add(2 * time.Minute)
	created, err := s.CapturePassiveQuery(ctx, []string{"pedido"})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM observations`); n != 2 {
		t.Fatalf("observaciones=%d, quería 2", n)
	}
}

func TestCapturePassiveQuery_HourlyCap(t *testing.T) {
	s, clock := newPassiveStore(t)
	ctx := context.Background()
	for i := 0; i < passiveMaxPerHour+5; i++ {
		if _, err := s.CapturePassiveQuery(ctx, []string{"t" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM observations`); n != passiveMaxPerHour {
		t.Fatalf("observaciones=%d, tope=%d", n, passiveMaxPerHour)
	}
	// Los conjuntos que no entraron siguen contando hits y quedan pendientes.
	if n := countRows(t, s, `SELECT COUNT(*) FROM passive_capture WHERE last_observed IS NULL`); n != 5 {
		t.Fatalf("pendientes=%d, quería 5", n)
	}
	// Pasada la hora, un conjunto pendiente sí genera su observación.
	*clock = clock.Add(61 * time.Minute)
	created, err := s.CapturePassiveQuery(ctx, []string{"t" + string(rune('a'+passiveMaxPerHour))})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestCapturePassiveQuery_EmptyTablesNoop(t *testing.T) {
	s, _ := newPassiveStore(t)
	if created, err := s.CapturePassiveQuery(context.Background(), []string{"", "  "}); created || err != nil {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if n := countRows(t, s, `SELECT COUNT(*) FROM passive_capture`); n != 0 {
		t.Fatalf("filas=%d", n)
	}
}
