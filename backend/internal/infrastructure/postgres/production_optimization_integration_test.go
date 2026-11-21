package postgres

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProductionOptimizationReadOnlyIntegration(t *testing.T) {
	pool, ctx := newProductionIntegrationPool(t, "optimization")
	defer pool.Close()
	repository := NewProductionRepository(pool)
	before := map[string]int64{}
	for _, table := range []string{"production_jobs", "production_runs", "material_spools"} {
		var count int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		before[table] = count
	}
	const workers = 4
	results := make([]int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			snapshot, snapshotErr := repository.OptimizationSnapshot(ctx, time.Now().UTC())
			if snapshotErr != nil {
				t.Errorf("snapshot: %v", snapshotErr)
				return
			}
			results[index] = len(snapshot.Jobs)*100000 + len(snapshot.Printers)
		}(i)
	}
	wg.Wait()
	for i := 1; i < workers; i++ {
		if results[i] != results[0] {
			t.Fatalf("non deterministic counts: %v", results)
		}
	}
	for table, count := range before {
		var after int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&after); err != nil || after != count {
			t.Fatalf("read-only invariant %s before=%d after=%d err=%v", table, count, after, err)
		}
	}
	rows, err := pool.Query(ctx, `EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id LEFT JOIN product_variants pv ON pv.id=oi.product_variant_id WHERE pj.status IN ('awaiting_production','queued','preparing') ORDER BY pj.priority DESC,pj.due_at NULLS LAST,pj.created_at,pj.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	plan := []string{}
	for rows.Next() {
		var line string
		if err = rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, line)
	}
	if len(plan) == 0 {
		t.Fatal("empty explain plan")
	}
	t.Log(strings.Join(plan, "\n"))
}
