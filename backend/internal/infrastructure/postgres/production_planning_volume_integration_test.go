package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionPlanningVolumeAndGlobalMetrics(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") || strings.EqualFold(config.ConnConfig.Database, "loja_3d") {
		t.Fatal("volume test requires a dedicated *_test database")
	}
	if development := strings.TrimSpace(os.Getenv("DATABASE_URL")); development != "" && development == dsn {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// Fixtures exist only inside this transaction. Replica mode permits representative
	// historical states without executing commercial transition/payment triggers.
	if _, err = tx.Exec(ctx, `SET LOCAL session_replication_role = replica`); err != nil {
		t.Fatalf("isolated fixture mode: %v", err)
	}
	suffix := fmt.Sprintf("planning-volume-%d", time.Now().UnixNano())
	var categoryID, materialID, colorID, materialEnoughID, colorEnoughID, productID, variantID, userID, orderID string
	mustID := func(query string, args []any, target *string) {
		t.Helper()
		if err := tx.QueryRow(ctx, query, args...).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	mustID(`INSERT INTO categories(name,slug) VALUES($1,$2) RETURNING id`, []any{"Planning volume", suffix}, &categoryID)
	mustID(`INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id`, []any{suffix, "Material " + suffix}, &materialID)
	mustID(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#654321') RETURNING id`, []any{suffix, "Color " + suffix}, &colorID)
	mustID(`INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id`, []any{suffix + "-enough", "Material enough " + suffix}, &materialEnoughID)
	mustID(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#123456') RETURNING id`, []any{suffix + "-enough", "Color enough " + suffix}, &colorEnoughID)
	mustID(`INSERT INTO products(category_id,slug,name,description,status) VALUES($1,$2,$3,'Volume fixture','draft') RETURNING id`, []any{categoryID, suffix, "Product " + suffix}, &productID)
	mustID(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity,name) VALUES($1,$2,$3,$4,100,1000,'Volume') RETURNING id`, []any{productID, materialID, colorID, "SKU-" + suffix}, &variantID)
	mustID(`INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Planning fixture',now()) RETURNING id`, []any{suffix + "@example.test"}, &userID)
	mustID(`INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,subtotal_cents,total_cents,payment_readiness) VALUES($1,$2,'confirmed','Planning fixture',$3,15000,15000,'ready') RETURNING id`, []any{"PED-" + suffix, userID, suffix + "@example.test"}, &orderID)

	printerIDs := make([]string, 3)
	printerStates := []string{"available", "busy", "maintenance"}
	for i, status := range printerStates {
		mustID(`INSERT INTO printers(code,name,manufacturer,model,status,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,$2,'Bambu Lab','A1',$3,256,256,256) RETURNING id`, []any{fmt.Sprintf("%s-p%d", suffix, i), fmt.Sprintf("Fixture printer %d", i), status}, &printerIDs[i])
	}
	if _, err = tx.Exec(ctx, `INSERT INTO printer_maintenance_events(printer_id,event_type,status,description,started_at) VALUES($1,'preventive','in_progress','Volume maintenance',now())`, printerIDs[2]); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,minimum_stock_grams,cost_per_kg_cents) VALUES
	 ($1,$5,$6,'Fixture','available',1000,80,100,10000),($2,$5,$6,'Fixture','in_use',1000,70,100,10000),($3,$5,$6,'Fixture','quarantined',1000,900,100,10000),
	 ($4,$7,$8,'Fixture','available',1000,900,100,10000)`, suffix+"-low", suffix+"-use", suffix+"-quarantine", suffix+"-enough", materialID, colorID, materialEnoughID, colorEnoughID); err != nil {
		t.Fatal(err)
	}

	statuses := []string{"awaiting_production", "queued", "preparing", "printing", "quality_check", "completed", "failed", "cancelled"}
	now := time.Now().UTC()
	jobIDs := make([]string, 0, 152)
	runIDs := map[string]string{}
	for i := 0; i < 152; i++ {
		var itemID, jobID string
		mustID(`INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents) VALUES($1,'catalog',$2,$3,$4,'Volume snapshot',$5,'Fixture material','Fixture color',1,100) RETURNING id`, []any{orderID, productID, variantID, fmt.Sprintf("Fixture product %03d", i), fmt.Sprintf("SKU-%s-%03d", suffix, i)}, &itemID)
		status := statuses[i%len(statuses)]
		var due any
		switch {
		case i%5 == 0:
			due = now.Add(-24 * time.Hour)
		case i%5 == 1:
			due = now.Add(4 * time.Hour)
		case i%5 == 2:
			due = now.Add(72 * time.Hour)
		default:
			due = nil
		}
		completedAt := any(nil)
		jobStartedAt := any(nil)
		jobFailure := any(nil)
		if status == "completed" {
			completedAt = now.Add(-time.Duration(i%20) * 24 * time.Hour)
		}
		if status == "printing" || status == "quality_check" {
			jobStartedAt = now.Add(-30 * time.Minute)
		}
		if status == "failed" {
			jobFailure = "Fixture failure"
		}
		mustID(`INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity,status,job_number,priority,due_at,started_at,completed_at,failure_reason) VALUES($1,$2,$3,1,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, []any{itemID, materialID, colorID, status, fmt.Sprintf("PV-%s-%03d", suffix, i), i % 101, due, jobStartedAt, completedAt, jobFailure}, &jobID)
		jobIDs = append(jobIDs, jobID)
		if status != "awaiting_production" {
			runStatus := status
			if runStatus == "quality_check" {
				runStatus = "quality_check"
			}
			if runStatus == "completed" {
				runStatus = "completed"
			}
			if runStatus == "cancelled" {
				runStatus = "cancelled"
			}
			started := any(nil)
			completed := any(nil)
			actual := any(nil)
			failure := any(nil)
			printer := any(nil)
			if status == "preparing" || status == "printing" || status == "quality_check" || status == "completed" {
				printer = printerIDs[1]
			}
			if status == "printing" || status == "quality_check" || status == "completed" {
				started = now.Add(-30 * time.Minute)
			}
			if status == "completed" {
				completed = now.Add(-10 * time.Minute)
				actual = 20 + (i % 11)
			}
			if status == "failed" {
				failure = "Fixture failure"
			}
			estimated := any(60 + i%30)
			weight := any(50 + i%20)
			if i%7 == 0 {
				estimated = nil
			}
			if i%9 == 0 {
				weight = nil
			}
			var runID string
			mustID(`INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity,status,estimated_minutes,actual_minutes,estimated_weight_grams,started_at,completed_at,failure_reason) VALUES($1,$2,1,1,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, []any{jobID, printer, runStatus, estimated, actual, weight, started, completed, failure}, &runID)
			runIDs[jobID] = runID
		}
	}
	// Explicit reprints exercise latest-attempt selection without changing global job totals.
	for i := 6; i < 24; i += 8 {
		parent := runIDs[jobIDs[i]]
		if parent != "" {
			if _, err = tx.Exec(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,estimated_minutes,estimated_weight_grams) VALUES($1,$2,$3,2,1,'queued',75,60)`, jobIDs[i], parent, printerIDs[0]); err != nil {
				t.Fatal(err)
			}
		}
	}

	repository := &ProductionRepository{pool: tx}
	dashboard, err := repository.ProductionDashboard(ctx, now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	var expected app.Dashboard
	expected.PeriodDays = 30
	if err = tx.QueryRow(ctx, `SELECT
	 count(*) FILTER(WHERE status='awaiting_production'),count(*) FILTER(WHERE status IN('queued','preparing')),
	 (SELECT count(*) FROM production_runs WHERE status IN('printing','post_processing')),
	 count(*) FILTER(WHERE status='quality_check'),count(*) FILTER(WHERE due_at<now() AND status NOT IN('completed','cancelled')),
	 count(*) FILTER(WHERE status='completed' AND completed_at >= $1),
	 (SELECT count(*) FROM printers WHERE status='available'),(SELECT count(*) FROM printers WHERE status='busy'),
	 (SELECT count(*) FROM printers WHERE status='maintenance'),
	 (SELECT count(*) FROM (SELECT material_id,color_id,sum(current_weight_grams) usable,sum(minimum_stock_grams) minimum FROM material_spools WHERE status IN('sealed','available','in_use') GROUP BY material_id,color_id) s WHERE usable<=minimum)
	 FROM production_jobs`, now.Add(-30*24*time.Hour)).Scan(&expected.AwaitingJobs, &expected.QueuedJobs, &expected.PrintingRuns, &expected.QualityJobs, &expected.OverdueJobs, &expected.CompletedJobs, &expected.AvailablePrinters, &expected.BusyPrinters, &expected.MaintenancePrinters, &expected.LowStockMaterials); err != nil {
		t.Fatal(err)
	}
	if dashboard != expected {
		t.Fatalf("dashboard API=%+v SQL=%+v", dashboard, expected)
	}
	var fixtureJobs int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_jobs WHERE job_number LIKE $1`, "PV-"+suffix+"-%").Scan(&fixtureJobs); err != nil || fixtureJobs != 152 {
		t.Fatalf("fixtures=%d err=%v", fixtureJobs, err)
	}
	inputs, total, err := repository.PlanningInputs(ctx, app.PlanningFilter{Page: 1, Limit: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if total <= 100 || len(inputs) <= 100 {
		t.Fatalf("planning total=%d rows=%d", total, len(inputs))
	}
	_, firstTotal, err := repository.PlanningInputs(ctx, app.PlanningFilter{Page: 1, Limit: 100})
	if err != nil || firstTotal <= 100 {
		t.Fatalf("first page total=%d err=%v", firstTotal, err)
	}
	secondPage, secondTotal, err := repository.PlanningInputs(ctx, app.PlanningFilter{Page: 2, Limit: 100})
	if err != nil || secondTotal != firstTotal || len(secondPage) == 0 {
		t.Fatalf("second page rows=%d total=%d err=%v", len(secondPage), secondTotal, err)
	}
	service := app.New(repository)
	service.SetPlanningRepository(repository)
	planning, err := service.ListProductionPlanning(ctx, admindomain.Session{Role: "production_manager"}, app.PlanningFilter{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	riskTotal := 0
	for _, count := range planning.RiskCounts {
		riskTotal += count
	}
	if int64(riskTotal) != planning.Total || planning.RiskCounts["overdue"] == 0 || planning.RiskCounts["blocked"] == 0 || planning.RiskCounts["attention"] == 0 {
		t.Fatalf("risk counts=%v total=%d", planning.RiskCounts, planning.Total)
	}
	capacities, err := repository.PrinterCapacities(ctx, now)
	if err != nil || len(capacities) < 3 {
		t.Fatalf("capacities=%d err=%v", len(capacities), err)
	}
	materials, err := repository.MaterialPlanning(ctx)
	if err != nil || len(materials) == 0 {
		t.Fatalf("materials=%d err=%v", len(materials), err)
	}
	explainQueries := map[string]string{
		"dashboard": `SELECT count(*) FILTER(WHERE status='awaiting_production'),count(*) FILTER(WHERE due_at<now() AND status NOT IN('completed','cancelled')) FROM production_jobs`,
		"planning":  `SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id LEFT JOIN production_runs pr ON pr.production_job_id=pj.id ORDER BY pj.priority DESC,pj.due_at NULLS LAST,pj.created_at LIMIT 20`,
		"capacity":  `SELECT p.id,count(pr.id) FROM printers p LEFT JOIN production_runs pr ON pr.printer_id=p.id AND pr.status IN('preparing','printing','post_processing','quality_check') GROUP BY p.id`,
		"materials": `SELECT material_id,color_id,sum(current_weight_grams),sum(minimum_stock_grams) FROM material_spools WHERE status IN('sealed','available','in_use') GROUP BY material_id,color_id`,
	}
	for name, query := range explainQueries {
		rows, explainErr := tx.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+query)
		if explainErr != nil {
			t.Fatalf("explain %s: %v", name, explainErr)
		}
		var plan []string
		for rows.Next() {
			var line string
			if scanErr := rows.Scan(&line); scanErr != nil {
				t.Fatal(scanErr)
			}
			plan = append(plan, line)
		}
		rows.Close()
		t.Logf("EXPLAIN %s\n%s", name, strings.Join(plan, "\n"))
	}
	t.Logf("fixtures=152 dashboard=%+v planning_rows=%d", dashboard, len(inputs))
	if err = tx.Rollback(ctx); err != nil {
		t.Fatalf("fixture rollback: %v", err)
	}
	var remaining int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM production_jobs WHERE job_number LIKE 'PV-planning-volume-%'`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("fixture cleanup remaining=%d err=%v", remaining, err)
	}
}
