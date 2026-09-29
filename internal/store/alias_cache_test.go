package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAliasCache_HitSkipsQuery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.UpsertEntityAlias(ctx, "ped", "", "Pedidos", "t"); err != nil {
		t.Fatal(err)
	}
	first, err := s.AllEntityAliases(ctx)
	if err != nil || len(first) != 1 {
		t.Fatalf("got %v, %v", first, err)
	}
	// Insert behind the store's back: a cache hit must not see it.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO entity_aliases (table_name, alias) VALUES ('otra', 'Otra')`); err != nil {
		t.Fatal(err)
	}
	second, _ := s.AllEntityAliases(ctx)
	if len(second) != 1 {
		t.Errorf("expected cached result (1 alias), got %d", len(second))
	}
}

func TestAliasCache_InvalidatedByUpsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.UpsertEntityAlias(ctx, "ped", "", "Pedidos", "t")
	s.AllEntityAliases(ctx) // warm the cache

	if _, err := s.UpsertEntityAlias(ctx, "ped", "", "Ordenes", "t"); err != nil {
		t.Fatal(err)
	}
	list, _ := s.AllEntityAliases(ctx)
	if len(list) != 1 || list[0].Alias != "Ordenes" {
		t.Errorf("corrected alias not visible immediately: %+v", list)
	}
}

func TestAliasCache_ReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.UpsertEntityAlias(ctx, "ped", "", "Pedidos", "t")
	a, _ := s.AllEntityAliases(ctx)
	a[0].Alias = "mutado"
	b, _ := s.AllEntityAliases(ctx)
	if b[0].Alias != "Pedidos" {
		t.Errorf("caller mutation leaked into cache: %q", b[0].Alias)
	}
}

func TestAliasCache_TTLExpires(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now()
	s.aliases.now = func() time.Time { return now }
	s.UpsertEntityAlias(ctx, "ped", "", "Pedidos", "t")
	s.AllEntityAliases(ctx)
	s.db.ExecContext(ctx, `INSERT INTO entity_aliases (table_name, alias) VALUES ('otra', 'Otra')`)

	now = now.Add(aliasCacheTTL + time.Second)
	list, _ := s.AllEntityAliases(ctx)
	if len(list) != 2 {
		t.Errorf("expected reload after TTL, got %d aliases", len(list))
	}
}

func TestAliasCache_Concurrent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := s.AllEntityAliases(ctx); err != nil {
					t.Error(err)
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if _, err := s.UpsertEntityAlias(ctx, "ped", "", "A", "t"); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
