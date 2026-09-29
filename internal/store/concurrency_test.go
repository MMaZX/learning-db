package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Simula dos procesos sobre el mismo archivo SQLite (dos Store con su propio
// pool) aprobando y proponiendo en paralelo: con BEGIN IMMEDIATE + busy_timeout
// no debe aparecer SQLITE_BUSY y solo una versión por topic_key queda validada.
func TestTwoStores_ConcurrentWritesNoBusy(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	s2, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	const n = 20
	ids := make([]int64, n)
	for i := range ids {
		id, err := s1.ProposeKnowledge(ctx, Knowledge{Subject: "vendedor", Context: "ventas", Claim: fmt.Sprintf("c%d", i), Version: 1})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}

	var wg sync.WaitGroup
	errs := make(chan error, 4*n)
	for i, id := range ids {
		st := s1
		if i%2 == 1 {
			st = s2
		}
		wg.Add(2)
		go func(st *Store, id int64) {
			defer wg.Done()
			// Colisiones de topic_key son esperadas; SQLITE_BUSY no.
			if _, err := st.ApproveKnowledge(ctx, id, "admin", ""); err != nil && !strings.Contains(err.Error(), "vendedor@ventas") {
				errs <- err
			}
		}(st, id)
		go func(st *Store, i int) {
			defer wg.Done()
			other := s2
			if st == s2 {
				other = s1
			}
			if _, err := other.ProposeKnowledge(ctx, Knowledge{Subject: fmt.Sprintf("otro%d", i), Claim: "x", Version: 1}); err != nil {
				errs <- err
			}
		}(st, i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if strings.Contains(strings.ToLower(err.Error()), "locked") || strings.Contains(err.Error(), "BUSY") {
			t.Errorf("error de bloqueo entre procesos: %v", err)
		} else {
			t.Errorf("error inesperado: %v", err)
		}
	}

	var validated int
	if err := s1.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM knowledge WHERE topic_key = 'vendedor@ventas' AND status = 'validated'`).Scan(&validated); err != nil {
		t.Fatal(err)
	}
	if validated != 1 {
		t.Errorf("validadas para vendedor@ventas = %d, quería 1", validated)
	}
}
