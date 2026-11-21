package postgres

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	"github.com/jackc/pgx/v5"
)

func TestProductionAnalyticsPostgreSQL(t *testing.T) {
	_, pool, ctx := newProductionIntegrationSchema(t, "analytics")
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	suffix := fmt.Sprintf("analytics-%d", time.Now().UnixNano())
	var category, material, color, user, order, admin, printer, product, variant, item, spool, secondSpool string
	id := func(query string, args []any, target *string) {
		t.Helper()
		if err := tx.QueryRow(ctx, query, args...).Scan(target); err != nil {
			t.Fatalf("fixture insert: %v", err)
		}
	}
	id(`INSERT INTO categories(name,slug)VALUES('Analytics',$1)RETURNING id`, []any{suffix}, &category)
	id(`INSERT INTO materials(code,name)VALUES($1,$2)RETURNING id`, []any{suffix, "PLA " + suffix}, &material)
	id(`INSERT INTO colors(slug,name,hex_code)VALUES($1,$2,'#7655aa')RETURNING id`, []any{suffix, "Violeta " + suffix}, &color)
	id(`INSERT INTO users(email,full_name,email_verified_at)VALUES($1,'Analytics',now())RETURNING id`, []any{suffix + "@example.test"}, &user)
	id(`INSERT INTO admin_users(email,full_name,password_hash,role)VALUES($1,'Analytics manager',repeat('x',60),'production_manager')RETURNING id`, []any{"admin-" + suffix + "@example.test"}, &admin)
	id(`INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,subtotal_cents,total_cents,payment_readiness)VALUES($1,$2,'confirmed','Analytics',$3,100,100,'ready')RETURNING id`, []any{"PED-" + suffix, user, suffix + "@example.test"}, &order)
	id(`INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm)VALUES($1,'A1 Analytics','Bambu Lab','A1',256,256,256)RETURNING id`, []any{suffix}, &printer)
	id(`INSERT INTO products(category_id,slug,name,description,status)VALUES($1,$2,'Produto Analytics','Fixture','draft')RETURNING id`, []any{category, "product-" + suffix}, &product)
	id(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity)VALUES($1,$2,$3,$4,100,1)RETURNING id`, []any{product, material, color, "SKU-" + suffix}, &variant)
	id(`INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents)VALUES($1,'catalog',$2,$3,'Produto Analytics','Fixture',$4,'PLA Analytics','Violeta',1,100)RETURNING id`, []any{order, product, variant, "SKU-" + suffix}, &item)
	id(`INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents)VALUES($1,$2,$3,'Fixture','available',1000,1000,8000)RETURNING id`, []any{"SP-" + suffix, material, color}, &spool)
	id(`INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents)VALUES($1,$2,$3,'Fixture','available',1000,900,20000)RETURNING id`, []any{"SP2-" + suffix, material, color}, &secondSpool)

	base := time.Now().UTC().Truncate(time.Microsecond)
	intArg := func(value *int) any {
		if value == nil {
			return nil
		}
		return *value
	}
	intValue := func(value int) *int { return &value }
	type runFixture struct {
		status     string
		estimated  *int
		actual     *int
		legacyCost int64
		reprint    bool
		failure    any
	}
	runFixtures := []runFixture{
		{status: "completed", estimated: intValue(90), actual: intValue(100)},
		{status: "completed", estimated: intValue(180), actual: intValue(200), reprint: true},
		{status: "failed", estimated: intValue(60), actual: intValue(50), failure: "fixture failure"},
		{status: "completed", estimated: intValue(210), actual: intValue(300), legacyCost: 700}, // legged run, no material snapshot
		{status: "completed", estimated: intValue(10), actual: intValue(0)},                     // zero duration is excluded from accuracy
		{status: "completed", estimated: nil, actual: intValue(75)},                             // missing estimate is excluded
		{status: "printing", estimated: intValue(60), actual: nil},                              // incomplete run is excluded
	}
	jobIDs := make([]string, len(runFixtures))
	for index := range runFixtures {
		if index == 2 { // The failed run and its reprint must share one production job.
			jobIDs[index] = jobIDs[1]
			continue
		}
		itemID := item
		if index != 0 {
			id(`INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents)VALUES($1,'catalog',$2,$3,'Produto Analytics','Fixture',$4,'PLA Analytics','Violeta',1,100)RETURNING id`, []any{order, product, variant, fmt.Sprintf("SKU-%s-%d", suffix, index)}, &itemID)
		}
		id(`INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity,job_number)VALUES($1,$2,$3,1,$4)RETURNING id`, []any{itemID, material, color, fmt.Sprintf("PJ-%s-%d", suffix, index)}, &jobIDs[index])
	}

	runIDs := make([]string, len(runFixtures))
	insertRun := func(index int) {
		t.Helper()
		fixture := runFixtures[index]
		var runID string
		var reprint any
		if fixture.reprint {
			reprint = runIDs[2]
		}
		completed := any(base.Add(-time.Duration(index) * time.Minute))
		if fixture.status == "printing" {
			completed = nil
		}
		insertStatus := fixture.status
		if index == 0 || index == 2 {
			insertStatus = "quality_check"
		}
		actualMinutes := 0
		if fixture.actual != nil {
			actualMinutes = *fixture.actual
		}
		started := base.Add(-time.Duration(index)*time.Minute - time.Duration(actualMinutes)*time.Minute)
		if fixture.status == "printing" {
			started = base.Add(-time.Duration(index) * time.Minute)
		}
		id(`INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,estimated_minutes,actual_minutes,material_cost_cents,started_at,completed_at,failure_reason)
			VALUES($1,$2,$3,$4,1,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
			[]any{jobIDs[index], reprint, printer, map[bool]int{true: 2, false: 1}[fixture.reprint], insertStatus, intArg(fixture.estimated), intArg(fixture.actual), fixture.legacyCost, started, completed, fixture.failure}, &runID)
		runIDs[index] = runID
	}
	// Seed the failed first attempt and record its rejection before inserting the
	// dependent reprint; all production integrity triggers stay enabled.
	insertRun(2)
	if _, err = tx.Exec(ctx, `INSERT INTO quality_inspections(production_run_id,result,dimensional_accuracy_ok,visual_quality_ok,functional_test_ok,notes,inspected_by_admin_user_id)VALUES($1,'rejected',false,true,false,'Fixture reprovada',$2)`, runIDs[2], admin); err != nil {
		t.Fatal("insert rejected quality fixture:", err)
	}
	if _, err = tx.Exec(ctx, `SELECT transition_production_run($1,'failed',$2,'Analytics fixture failed',1)`, runIDs[2], admin); err != nil {
		t.Fatal("transition rejected source run to failed:", err)
	}
	insertRun(0)
	if _, err = tx.Exec(ctx, `INSERT INTO quality_inspections(production_run_id,result,dimensional_accuracy_ok,visual_quality_ok,functional_test_ok,notes,inspected_by_admin_user_id)VALUES($1,'approved',true,true,true,NULL,$2)`, runIDs[0], admin); err != nil {
		t.Fatal("insert approved quality fixture:", err)
	}
	if _, err = tx.Exec(ctx, `SELECT transition_production_run($1,'completed',$2,'Analytics fixture completed',1)`, runIDs[0], admin); err != nil {
		t.Fatal("transition approved run to completed:", err)
	}
	insertRun(1)
	for index := 3; index < len(runFixtures); index++ {
		insertRun(index)
	}

	consume := func(runID, spoolID string, grams, wasted int, unitCost, totalCost int64) {
		t.Helper()
		_, err := tx.Exec(ctx, `INSERT INTO production_run_materials(production_run_id,material_spool_id,consumed_grams,wasted_grams,cost_per_kg_snapshot_cents,total_cost_cents,recorded_by_admin_user_id)VALUES($1,$2,$3,$4,$5,$6,$7)`, runID, spoolID, grams, wasted, unitCost, totalCost, admin)
		if err != nil {
			t.Fatalf("insert material snapshot: %v", err)
		}
	}
	consume(runIDs[0], spool, 100, 5, 8000, 840)
	consume(runIDs[0], secondSpool, 50, 2, 20000, 1040)
	consume(runIDs[1], spool, 200, 5, 8000, 1640)
	consume(runIDs[2], spool, 300, 5, 8000, 2440)
	// Change current spool prices after recording the immutable run snapshots.
	if _, err = tx.Exec(ctx, `UPDATE material_spools SET cost_per_kg_cents=999999 WHERE id IN($1,$2)`, spool, secondSpool); err != nil {
		t.Fatal("change current spool prices:", err)
	}

	repo := &ProductionRepository{pool: tx}
	allPage, err := repo.ListHistory(ctx, app.AnalyticsFilter{ProductID: product, Page: 1, Limit: 3})
	if err != nil || allPage.Total != 7 || len(allPage.Items) != 3 {
		t.Fatalf("page 1=%+v err=%v", allPage, err)
	}
	if allPage.Items[0].ID != runIDs[0] || allPage.Items[1].ID != runIDs[1] || allPage.Items[2].ID != runIDs[2] {
		t.Fatalf("unexpected descending history order: got IDs %s, %s, %s", allPage.Items[0].ID, allPage.Items[1].ID, allPage.Items[2].ID)
	}
	secondPage, err := repo.ListHistory(ctx, app.AnalyticsFilter{ProductID: product, Page: 2, Limit: 3})
	if err != nil || secondPage.Total != 7 || len(secondPage.Items) != 3 || secondPage.Items[0].ID != runIDs[3] {
		t.Fatalf("page 2=%+v err=%v", secondPage, err)
	}
	from, to := base.Add(-90*time.Second), base.Add(time.Second)
	combined, err := repo.ListHistory(ctx, app.AnalyticsFilter{From: &from, To: &to, PrinterID: printer, ProductID: product, MaterialID: material, ProductQuery: "Produto Analytics", MaterialQuery: "PLA", Status: "completed", Quality: "approved", Page: 1, Limit: 20})
	if err != nil || combined.Total != 1 || len(combined.Items) != 1 || combined.Items[0].ID != runIDs[0] {
		t.Fatalf("combined filter=%+v err=%v", combined, err)
	}
	datePage, err := repo.ListHistory(ctx, app.AnalyticsFilter{From: &from, To: &to, Page: 1, Limit: 20})
	if err != nil || datePage.Total != 2 {
		t.Fatalf("date filter total=%d err=%v", datePage.Total, err)
	}
	failed, err := repo.ListHistory(ctx, app.AnalyticsFilter{Status: "failed", Page: 1, Limit: 20})
	if err != nil || failed.Total != 1 || failed.Items[0].ID != runIDs[2] || failed.Items[0].FailureReason == "" {
		t.Fatalf("failed filter=%+v err=%v", failed, err)
	}
	rejected, err := repo.ListHistory(ctx, app.AnalyticsFilter{Quality: "rejected", Page: 1, Limit: 20})
	if err != nil || rejected.Total != 1 || rejected.Items[0].ID != runIDs[2] {
		t.Fatalf("quality filter=%+v err=%v", rejected, err)
	}
	detail, err := repo.GetHistory(ctx, runIDs[0])
	if err != nil || len(detail.Materials) != 2 || len(detail.Inspections) != 1 || len(detail.Events) != 4 {
		t.Fatalf("detail materials=%d inspections=%d events=%d err=%v", len(detail.Materials), len(detail.Inspections), len(detail.Events), err)
	}
	if detail.Run.ConsumedGrams != 150 || detail.Run.WastedGrams != 7 || detail.Run.MaterialCostCents != 1880 {
		t.Fatalf("run snapshot was duplicated or recalculated: %+v", detail.Run)
	}

	analytics, err := repo.Productivity(ctx, app.AnalyticsFilter{ProductID: product, Page: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	summary := analytics.Summary
	if summary.TotalRuns != 7 || summary.CompletedRuns != 5 || summary.FailedRuns != 1 || summary.Reprints != 1 || summary.RecordedMinutes != 725 || summary.ConsumedGrams != 650 || summary.WastedGrams != 17 || summary.MaterialCostCents != 6660 {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.SuccessRate == nil || *summary.SuccessRate != 83.33 || summary.FailureRate == nil || *summary.FailureRate != 16.67 {
		t.Fatalf("rates success=%v failure=%v", summary.SuccessRate, summary.FailureRate)
	}
	if len(analytics.TimeAccuracy) != 1 || analytics.TimeAccuracy[0].Samples != 3 || *analytics.TimeAccuracy[0].MedianEstimatedMinutes != 180 || *analytics.TimeAccuracy[0].MedianActualMinutes != 200 || *analytics.TimeAccuracy[0].MedianDifferenceMinutes != 20 || math.Abs(*analytics.TimeAccuracy[0].MedianDifferencePercent-11.11111111111111) > 0.001 {
		if len(analytics.TimeAccuracy) == 1 {
			got := analytics.TimeAccuracy[0]
			t.Fatalf("odd median or invalid-duration filtering mismatch: samples=%d estimate=%v actual=%v difference=%v percent=%v", got.Samples, *got.MedianEstimatedMinutes, *got.MedianActualMinutes, *got.MedianDifferenceMinutes, *got.MedianDifferencePercent)
		}
		t.Fatalf("odd median or invalid-duration filtering mismatch: %+v", analytics.TimeAccuracy)
	}
	if len(analytics.Printers) != 1 || analytics.Printers[0].TotalRuns != 7 || analytics.Printers[0].MaterialCostCents != summary.MaterialCostCents {
		t.Fatalf("printer aggregation duplicated or mismatched: %+v", analytics.Printers)
	}
	if len(analytics.Products) != 1 || analytics.Products[0].TotalRuns != 7 || analytics.Products[0].MaterialCostCents != summary.MaterialCostCents {
		t.Fatalf("product aggregation duplicated or mismatched: %+v", analytics.Products)
	}
	if len(analytics.Materials) != 1 || analytics.Materials[0].ConsumedGrams != 650 || analytics.Materials[0].WastedGrams != 17 || analytics.Materials[0].MaterialCostCents != summary.MaterialCostCents {
		t.Fatalf("material aggregation duplicated or mismatched: %+v", analytics.Materials)
	}
	if summary.UtilizationPercent != nil {
		t.Fatal("utilization must remain unknown without availability history")
	}
	evenMedian, err := repo.Productivity(ctx, app.AnalyticsFilter{From: &from, To: &to, ProductID: product, Page: 1, Limit: 100})
	if err != nil || len(evenMedian.TimeAccuracy) != 1 || evenMedian.TimeAccuracy[0].Samples != 2 || *evenMedian.TimeAccuracy[0].MedianEstimatedMinutes != 135 || *evenMedian.TimeAccuracy[0].MedianActualMinutes != 150 || *evenMedian.TimeAccuracy[0].MedianDifferenceMinutes != 15 {
		t.Fatalf("even median mismatch: analytics=%+v err=%v", evenMedian.TimeAccuracy, err)
	}
	legacyDetail, err := repo.GetHistory(ctx, runIDs[3])
	if err != nil || len(legacyDetail.Materials) != 0 || legacyDetail.Run.MaterialCostCents != 700 {
		t.Fatalf("legacy run without line snapshots should use only its preserved cost: detail=%+v err=%v", legacyDetail, err)
	}

	assertIndependentAnalyticsSQL(t, ctx, tx, product, printer, analytics)
	logAnalyticsExplain(t, ctx, tx, product, runIDs[0])
}

func assertIndependentAnalyticsSQL(t *testing.T, ctx context.Context, tx pgx.Tx, productID, printerID string, actual app.ProductivityAnalytics) {
	t.Helper()
	const perRun = `WITH per_run AS (
		SELECT pr.id,pr.status,pr.reprint_of_run_id,pr.actual_minutes,pr.estimated_minutes,
		       oi.product_id,pr.printer_id,pj.material_id,pr.material_cost_cents,
		       count(rm.id)::bigint material_rows,coalesce(sum(rm.consumed_grams),0)::bigint consumed,
		       coalesce(sum(rm.wasted_grams),0)::bigint wasted,coalesce(sum(rm.total_cost_cents),0)::bigint line_cost
		  FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id
		  JOIN order_items oi ON oi.id=pj.order_item_id
		  LEFT JOIN production_run_materials rm ON rm.production_run_id=pr.id
		 WHERE oi.product_id=$1
		 GROUP BY pr.id,oi.product_id,pj.material_id
	)`
	var total, completed, failed, reprints, minutes, consumed, wasted, cost int64
	if err := tx.QueryRow(ctx, perRun+` SELECT count(*),count(*) FILTER(WHERE status='completed'),count(*) FILTER(WHERE status='failed'),count(*) FILTER(WHERE reprint_of_run_id IS NOT NULL),coalesce(sum(actual_minutes) FILTER(WHERE actual_minutes>0 AND status IN('completed','failed')),0),coalesce(sum(consumed),0),coalesce(sum(wasted),0),coalesce(sum(CASE WHEN material_rows>0 THEN line_cost ELSE material_cost_cents END),0) FROM per_run`, productID).Scan(&total, &completed, &failed, &reprints, &minutes, &consumed, &wasted, &cost); err != nil {
		t.Fatal("independent summary SQL:", err)
	}
	s := actual.Summary
	if total != s.TotalRuns || completed != s.CompletedRuns || failed != s.FailedRuns || reprints != s.Reprints || minutes != s.RecordedMinutes || consumed != s.ConsumedGrams || wasted != s.WastedGrams || cost != s.MaterialCostCents {
		t.Fatalf("API/independent SQL summary mismatch API=%+v SQL=(%d,%d,%d,%d,%d,%d,%d,%d)", s, total, completed, failed, reprints, minutes, consumed, wasted, cost)
	}
	denominator := completed + failed
	var successRate, failureRate float64
	if denominator > 0 {
		successRate = float64(completed) * 100 / float64(denominator)
		failureRate = float64(failed) * 100 / float64(denominator)
	}
	if s.SuccessRate == nil || s.FailureRate == nil || *s.SuccessRate != 83.33 || *s.FailureRate != 16.67 || successRate < 83.333 || successRate > 83.334 || failureRate < 16.666 || failureRate > 16.667 {
		t.Fatalf("API rates do not match independent SQL counts: success=%v failure=%v SQL=(%.4f,%.4f)", s.SuccessRate, s.FailureRate, successRate, failureRate)
	}

	var samples int64
	var estimatedMedian, actualMedian, differenceMedian, percentMedian float64
	if err := tx.QueryRow(ctx, perRun+` SELECT count(*),percentile_cont(.5) WITHIN GROUP(ORDER BY estimated_minutes),percentile_cont(.5) WITHIN GROUP(ORDER BY actual_minutes),percentile_cont(.5) WITHIN GROUP(ORDER BY actual_minutes-estimated_minutes),percentile_cont(.5) WITHIN GROUP(ORDER BY ((actual_minutes-estimated_minutes)::numeric/estimated_minutes)*100) FROM per_run WHERE status='completed' AND estimated_minutes>0 AND actual_minutes>0`, productID).Scan(&samples, &estimatedMedian, &actualMedian, &differenceMedian, &percentMedian); err != nil {
		t.Fatal("independent time-accuracy SQL:", err)
	}
	accuracy := actual.TimeAccuracy[0]
	if samples != accuracy.Samples || estimatedMedian != *accuracy.MedianEstimatedMinutes || actualMedian != *accuracy.MedianActualMinutes || differenceMedian != *accuracy.MedianDifferenceMinutes || percentMedian < *accuracy.MedianDifferencePercent-0.001 || percentMedian > *accuracy.MedianDifferencePercent+0.001 {
		t.Fatalf("API/independent SQL time mismatch API=%+v SQL=(%d,%.2f,%.2f,%.2f,%.4f)", accuracy, samples, estimatedMedian, actualMedian, differenceMedian, percentMedian)
	}

	var printerRuns, printerCompleted, printerFailed, printerReprints, printerMinutes, printerConsumed, printerWasted, printerCost int64
	var printerAverage, printerMedian float64
	if err := tx.QueryRow(ctx, perRun+` SELECT count(*),count(*) FILTER(WHERE status='completed'),count(*) FILTER(WHERE status='failed'),count(*) FILTER(WHERE reprint_of_run_id IS NOT NULL),coalesce(sum(actual_minutes) FILTER(WHERE actual_minutes>0 AND status IN('completed','failed')),0),coalesce(sum(consumed),0),coalesce(sum(wasted),0),coalesce(sum(CASE WHEN material_rows>0 THEN line_cost ELSE material_cost_cents END),0),avg(actual_minutes) FILTER(WHERE actual_minutes>0 AND status IN('completed','failed')),percentile_cont(.5) WITHIN GROUP(ORDER BY actual_minutes) FILTER(WHERE actual_minutes>0 AND status IN('completed','failed')) FROM per_run WHERE printer_id=$2`, productID, printerID).Scan(&printerRuns, &printerCompleted, &printerFailed, &printerReprints, &printerMinutes, &printerConsumed, &printerWasted, &printerCost, &printerAverage, &printerMedian); err != nil {
		t.Fatal("independent printer SQL:", err)
	}
	if len(actual.Printers) != 1 {
		t.Fatalf("expected one printer aggregate, got %+v", actual.Printers)
	}
	p := actual.Printers[0]
	if printerRuns != p.TotalRuns || printerCompleted != p.CompletedRuns || printerFailed != p.FailedRuns || printerReprints != p.Reprints || printerMinutes != p.RecordedMinutes || printerConsumed != p.ConsumedGrams || printerWasted != p.WastedGrams || printerCost != p.MaterialCostCents || p.AverageMinutes == nil || p.MedianMinutes == nil || printerAverage != *p.AverageMinutes || printerMedian != *p.MedianMinutes || p.SuccessRate == nil || p.FailureRate == nil || *p.SuccessRate != 83.33 || *p.FailureRate != 16.67 {
		t.Fatalf("API/independent SQL printer mismatch SQL=(runs %d completed %d failed %d reprints %d minutes %d consumed %d wasted %d cost %d average %.2f median %.2f) API=%+v", printerRuns, printerCompleted, printerFailed, printerReprints, printerMinutes, printerConsumed, printerWasted, printerCost, printerAverage, printerMedian, p)
	}
	var productRuns, productCompleted, productFailed, productMinutes, productConsumed, productCost int64
	var productMedian float64
	if err := tx.QueryRow(ctx, perRun+` SELECT count(*),count(*) FILTER(WHERE status='completed'),count(*) FILTER(WHERE status='failed'),coalesce(sum(actual_minutes) FILTER(WHERE actual_minutes>0 AND status IN('completed','failed')),0),coalesce(sum(consumed),0),coalesce(sum(CASE WHEN material_rows>0 THEN line_cost ELSE material_cost_cents END),0),percentile_cont(.5) WITHIN GROUP(ORDER BY actual_minutes) FILTER(WHERE actual_minutes>0 AND status='completed') FROM per_run`, productID).Scan(&productRuns, &productCompleted, &productFailed, &productMinutes, &productConsumed, &productCost, &productMedian); err != nil {
		t.Fatal("independent product SQL:", err)
	}
	if len(actual.Products) != 1 {
		t.Fatalf("expected one product aggregate, got %+v", actual.Products)
	}
	product := actual.Products[0]
	if productRuns != product.TotalRuns || productCompleted != product.CompletedRuns || productFailed != product.FailedRuns || productMinutes != product.RecordedMinutes || productConsumed != product.ConsumedGrams || productCost != product.MaterialCostCents || product.MedianMinutes == nil || productMedian != *product.MedianMinutes {
		t.Fatalf("API/independent SQL product mismatch SQL=(runs %d completed %d failed %d minutes %d consumed %d cost %d median %.2f) API=%+v", productRuns, productCompleted, productFailed, productMinutes, productConsumed, productCost, productMedian, product)
	}
	var materialConsumed, materialWasted, materialCost int64
	if err := tx.QueryRow(ctx, perRun+` SELECT coalesce(sum(consumed),0),coalesce(sum(wasted),0),coalesce(sum(CASE WHEN material_rows>0 THEN line_cost ELSE material_cost_cents END),0) FROM per_run`, productID).Scan(&materialConsumed, &materialWasted, &materialCost); err != nil {
		t.Fatal("independent material SQL:", err)
	}
	if len(actual.Materials) != 1 || materialConsumed != actual.Materials[0].ConsumedGrams || materialWasted != actual.Materials[0].WastedGrams || materialCost != actual.Materials[0].MaterialCostCents {
		t.Fatalf("API/independent SQL material mismatch SQL=(%d,%d,%d) API=%+v", materialConsumed, materialWasted, materialCost, actual.Materials)
	}
}

func logAnalyticsExplain(t *testing.T, ctx context.Context, tx pgx.Tx, productID, runID string) {
	t.Helper()
	const from = ` FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id JOIN materials m ON m.id=pj.material_id JOIN colors c ON c.id=pj.color_id LEFT JOIN printers p ON p.id=pr.printer_id LEFT JOIN LATERAL(SELECT result,inspected_at FROM quality_inspections WHERE production_run_id=pr.id ORDER BY inspected_at DESC LIMIT 1) qi ON true LEFT JOIN LATERAL(SELECT count(*)::bigint material_rows,coalesce(sum(consumed_grams),0)::bigint consumed,coalesce(sum(wasted_grams),0)::bigint wasted,coalesce(sum(total_cost_cents),0)::bigint cost FROM production_run_materials WHERE production_run_id=pr.id) usage ON true `
	const cost = `CASE WHEN usage.material_rows>0 THEN usage.cost ELSE coalesce(pr.material_cost_cents,0) END`
	queries := []struct {
		name string
		sql  string
		args []any
	}{
		{"history-paged", `SELECT pr.id` + from + ` WHERE oi.product_id=$1 ORDER BY coalesce(pr.completed_at,pr.started_at,pr.created_at) DESC,pr.id LIMIT 3 OFFSET 0`, []any{productID}},
		{"history-detail", `SELECT pr.id` + from + ` WHERE pr.id=$1`, []any{runID}},
		{"productivity-general", `SELECT count(*),count(*) FILTER(WHERE pr.status='completed'),count(*) FILTER(WHERE pr.status='failed'),coalesce(sum(` + cost + `),0)` + from + ` WHERE oi.product_id=$1`, []any{productID}},
		{"productivity-printer", `SELECT pr.printer_id,count(*),coalesce(sum(` + cost + `),0)` + from + ` WHERE oi.product_id=$1 GROUP BY pr.printer_id`, []any{productID}},
		{"material-consumption-cost", `SELECT pj.material_id,coalesce(sum(usage.consumed),0),coalesce(sum(usage.wasted),0),coalesce(sum(` + cost + `),0)` + from + ` WHERE oi.product_id=$1 GROUP BY pj.material_id`, []any{productID}},
		{"historical-cost", `SELECT pr.id,` + cost + from + ` WHERE oi.product_id=$1`, []any{productID}},
		{"time-accuracy", `SELECT oi.product_id,percentile_cont(.5) WITHIN GROUP(ORDER BY pr.estimated_minutes),percentile_cont(.5) WITHIN GROUP(ORDER BY pr.actual_minutes)` + from + ` WHERE oi.product_id=$1 AND pr.status='completed' AND pr.estimated_minutes>0 AND pr.actual_minutes>0 GROUP BY oi.product_id`, []any{productID}},
	}
	for _, query := range queries {
		rows, err := tx.Query(ctx, `EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) `+query.sql, query.args...)
		if err != nil {
			t.Fatalf("EXPLAIN %s: %v", query.name, err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				rows.Close()
				t.Fatalf("scan EXPLAIN %s: %v", query.name, err)
			}
			plan = append(plan, line)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("read EXPLAIN %s: %v", query.name, err)
		}
		rows.Close()
		planning, execution, buffers := "not reported", "not reported", "not reported"
		seqScans, indexes := map[string]struct{}{}, map[string]struct{}{}
		for _, line := range plan {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "Planning Time:"):
				planning = trimmed
			case strings.HasPrefix(trimmed, "Execution Time:"):
				execution = trimmed
			case strings.HasPrefix(trimmed, "Buffers:") && buffers == "not reported":
				buffers = trimmed
			}
			if index := strings.Index(trimmed, "Seq Scan on "); index >= 0 {
				after := trimmed[index+len("Seq Scan on "):]
				seqScans[strings.Fields(after)[0]] = struct{}{}
			}
			if strings.Contains(trimmed, "Index Scan using ") || strings.Contains(trimmed, "Index Only Scan using ") || strings.Contains(trimmed, "Bitmap Index Scan using ") {
				if parts := strings.SplitN(trimmed, " using ", 2); len(parts) == 2 {
					indexes[strings.Fields(parts[1])[0]] = struct{}{}
				}
			}
		}
		root := "empty plan"
		if len(plan) > 0 {
			root = strings.TrimSpace(strings.SplitN(plan[0], " (cost=", 2)[0])
		}
		seqList, indexList := make([]string, 0, len(seqScans)), make([]string, 0, len(indexes))
		for name := range seqScans {
			seqList = append(seqList, name)
		}
		for name := range indexes {
			indexList = append(indexList, name)
		}
		sort.Strings(seqList)
		sort.Strings(indexList)
		t.Logf("EXPLAIN %s root=%s buffers=%s planning=%s execution=%s seq_scans=%v indexes=%v", query.name, root, buffers, planning, execution, seqList, indexList)
	}
}
