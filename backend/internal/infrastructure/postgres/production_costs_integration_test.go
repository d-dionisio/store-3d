package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test never creates a database or applies migrations. It only uses the
// explicitly isolated loja_3d_test database and rolls every fixture back.
func TestProductionCostsPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(testDatabaseURL())
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL; credentials are hidden")
	}
	if strings.EqualFold(config.ConnConfig.Database, "loja_3d") || config.ConnConfig.Database != "loja_3d_test" {
		t.Fatal("refusing any database except loja_3d_test")
	}
	host := strings.ToLower(config.ConnConfig.Host)
	if host != "localhost" && host != "127.0.0.1" && host != "::1" || config.ConnConfig.Port != 5432 {
		t.Fatal("test database must use loopback:5432")
	}
	if dev := strings.TrimSpace(testDevelopmentDatabaseURL()); dev != "" && dev == dsn {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("connect to isolated test database")
	}
	defer pool.Close()
	var currentDB string
	if err = pool.QueryRow(ctx, `SELECT current_database()`).Scan(&currentDB); err != nil {
		t.Fatal(err)
	}
	if currentDB != "loja_3d_test" {
		t.Fatalf("refusing connected database %q", currentDB)
	}
	var migration int
	var dirty bool
	if err = pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&migration, &dirty); err != nil {
		t.Fatal("could not verify migration state")
	}
	if migration != 28 || dirty {
		t.Fatalf("expected migration 28 clean; got version=%d dirty=%v", migration, dirty)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	suffix := fmt.Sprintf("cost-test-%d", time.Now().UnixNano())
	var adminID, printerID string
	if err = tx.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Cost test','012345678901234567890123456789012345678901234567890123456789','production_manager') RETURNING id::text`, suffix+"@example.invalid").Scan(&adminID); err != nil {
		t.Fatal("create transaction-local admin fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'Cost test printer','Fixture','No hardware',100,100,100) RETURNING id::text`, suffix).Scan(&printerID); err != nil {
		t.Fatal("create transaction-local printer fixture:", err)
	}
	repo := &ProductionCostRepository{pool: tx}
	base := app.CostProfileInput{ElectricityRateCentsPerKWh: 110, PrinterPowerMilliwatts: ptrInt64(150000), MachineAcquisitionCents: 100000, MachineUsefulLifeSeconds: 36000000, MaintenanceRateCentsPerHour: 20, LaborRateCentsPerHour: 3000, PreparationSeconds: 600, FinishingSeconds: 300}
	first, err := repo.SaveCostProfile(ctx, printerID, base, adminID)
	if err != nil {
		t.Fatal("save initial profile:", err)
	}
	if first.Version != 1 {
		t.Fatalf("first profile version=%d", first.Version)
	}
	base.ExpectedVersion = 1
	base.ElectricityRateCentsPerKWh = 125
	second, err := repo.SaveCostProfile(ctx, printerID, base, adminID)
	if err != nil {
		t.Fatal("save next profile version:", err)
	}
	if second.Version != 2 {
		t.Fatalf("second profile version=%d", second.Version)
	}
	base.ElectricityRateCentsPerKWh = 999
	if _, err = repo.SaveCostProfile(ctx, printerID, base, adminID); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale optimistic version should conflict, got %v", err)
	}
	history, err := repo.ListCostProfileHistory(ctx, printerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Version != 2 || history[1].ElectricityRateCentsPerKWh != 110 {
		t.Fatalf("profile history did not preserve prior configuration: %+v", history)
	}

	// Build a fully isolated commercial-to-production chain in this transaction.
	// All rows, including the immutable cost history, disappear on rollback below.
	var materialID, colorID, quoteID, proposalID, orderID, itemID, jobID, spoolID string
	if err = tx.QueryRow(ctx, `INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id::text`, suffix, "Fixture material "+suffix).Scan(&materialID); err != nil {
		t.Fatal("create material fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#123456') RETURNING id::text`, suffix, "Fixture color "+suffix).Scan(&colorID); err != nil {
		t.Fatal("create color fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,quoted_amount_cents,currency,estimated_lead_time_days,quoted_at,valid_until,approved_at) VALUES($1,'Cost Fixture','cost-fixture@example.invalid','approved',$2,$3,1,10000,'BRL',5,now(),now()+interval '5 days',now()) RETURNING id::text`, "Q-"+suffix, materialID, colorID).Scan(&quoteID); err != nil {
		t.Fatal("create approved quote fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at) VALUES($1,1,'sent',10000,5,now()+interval '5 days',$2,$2,now()) RETURNING id::text`, quoteID, adminID).Scan(&proposalID); err != nil {
		t.Fatal("create proposal snapshot fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO orders(order_number,quote_id,quote_proposal_id,quote_proposal_version,production_lead_time_days,status,customer_name,customer_email,currency,subtotal_cents,shipping_cents,discount_cents,total_cents) VALUES($1,$2,$3,1,5,'confirmed','Cost Fixture','cost-fixture@example.invalid','BRL',10000,0,0,10000) RETURNING id::text`, "O-"+suffix, quoteID, proposalID).Scan(&orderID); err != nil {
		t.Fatal("create confirmed order fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO order_items(order_id,item_type,product_name,description_snapshot,material_name,color_name,quantity,unit_price_cents,approved_total_cents) VALUES($1,'custom','Fixture item','Fixture description','Fixture material','Fixture color',1,10000,10000) RETURNING id::text`, orderID).Scan(&itemID); err != nil {
		t.Fatal("create order item fixture:", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,paid_amount_cents,currency,paid_at) VALUES($1,'pix','paid',10000,10000,'BRL',now())`, orderID); err != nil {
		t.Fatal("create paid payment fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity) VALUES($1,$2,$3,1) RETURNING id::text`, itemID, materialID, colorID).Scan(&jobID); err != nil {
		t.Fatal("create production job fixture:", err)
	}
	var failedRunID, actualRunID, estimateRunID string
	if err = tx.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,failure_reason) VALUES($1,$2,1,1,'failed',10,now()-interval '10 minutes','fixture reprint source') RETURNING id::text`, jobID, printerID).Scan(&failedRunID); err != nil {
		t.Fatal("create failed source run fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,estimated_minutes,actual_minutes,estimated_weight_grams,actual_weight_grams,started_at,completed_at) VALUES($1,$2,$3,2,1,'completed',60,60,100,110,now()-interval '1 hour',now()) RETURNING id::text`, jobID, failedRunID, printerID).Scan(&actualRunID); err != nil {
		t.Fatal("create reprint fixture:", err)
	}
	if err = tx.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,estimated_minutes,estimated_weight_grams) VALUES($1,$2,$3,3,1,'queued',60,100) RETURNING id::text`, jobID, failedRunID, printerID).Scan(&estimateRunID); err != nil {
		t.Fatal("create estimate run fixture:", err)
	}
	duplicateRunTx, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, duplicateRunErr := duplicateRunTx.Exec(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status) VALUES($1,$2,$3,2,1,'queued')`, jobID, failedRunID, printerID)
	_ = duplicateRunTx.Rollback(ctx)
	if duplicateRunErr == nil {
		t.Fatal("duplicate production attempt number was accepted")
	}
	if err = tx.QueryRow(ctx, `INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents) VALUES($1,$2,$3,'Fixture','in_use',1000,1000,20000) RETURNING id::text`, "S-"+suffix, materialID, colorID).Scan(&spoolID); err != nil {
		t.Fatal("create spool fixture:", err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO production_run_materials(production_run_id,material_spool_id,consumed_grams,wasted_grams,cost_per_kg_snapshot_cents,total_cost_cents,recorded_by_admin_user_id) VALUES($1,$2,100,10,0,0,$3)`, actualRunID, spoolID, adminID); err != nil {
		t.Fatal("record transactional material usage:", err)
	}
	var spoolBalance, capturedPrice, capturedMaterialCost int64
	if err = tx.QueryRow(ctx, `SELECT current_weight_grams FROM material_spools WHERE id=$1`, spoolID).Scan(&spoolBalance); err != nil || spoolBalance != 890 {
		t.Fatalf("material debit balance=%d err=%v", spoolBalance, err)
	}
	if err = tx.QueryRow(ctx, `SELECT cost_per_kg_snapshot_cents,total_cost_cents FROM production_run_materials WHERE production_run_id=$1`, actualRunID).Scan(&capturedPrice, &capturedMaterialCost); err != nil || capturedPrice != 20000 || capturedMaterialCost != 2200 {
		t.Fatalf("material price/cost snapshot=%d/%d err=%v", capturedPrice, capturedMaterialCost, err)
	}

	measuredEnergy := int64(500000)
	costContext, err := repo.LoadRunCostContext(ctx, actualRunID, "actual", &measuredEnergy)
	if err != nil {
		t.Fatal("load actual cost context:", err)
	}
	costContext.Input.PreparationSeconds = 1800
	costContext.Input.FinishingSeconds = 900
	actualBreakdown, err := domain.CalculateManufacturingCost(costContext.Input, costContext.Quantity)
	if err != nil {
		t.Fatal("calculate actual costs:", err)
	}
	if actualBreakdown.MaterialConsumedCents != 2000 || actualBreakdown.MaterialWasteCents != 200 || actualBreakdown.EnergyCents != 63 || actualBreakdown.DepreciationCents != 10 || actualBreakdown.MaintenanceCents != 20 || actualBreakdown.PreparationCents != 1500 || actualBreakdown.FinishingCents != 750 || actualBreakdown.LaborTotalCents != 2250 || actualBreakdown.TotalCostCents != 4543 || actualBreakdown.UnitCostCents != 4543 || actualBreakdown.ContributionMarginCents != 5467 || actualBreakdown.ProfitCents != 5457 {
		t.Fatalf("unexpected PostgreSQL-backed financial calculation: %+v", actualBreakdown)
	}
	badBreakdown := actualBreakdown
	badBreakdown.TotalCostCents++ // violates the database's component-sum CHECK.
	if _, err = repo.SaveRunCostSnapshot(ctx, costContext, badBreakdown, adminID); err == nil {
		t.Fatal("expected database rejection of inconsistent snapshot totals")
	}
	var snapshotCount int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1`, actualRunID).Scan(&snapshotCount); err != nil || snapshotCount != 0 {
		t.Fatalf("failed snapshot left partial row count=%d err=%v", snapshotCount, err)
	}
	actualSnapshot, err := repo.SaveRunCostSnapshot(ctx, costContext, actualBreakdown, adminID)
	if err != nil {
		t.Fatal("save actual cost snapshot:", err)
	}
	duplicateSnapshot, err := repo.SaveRunCostSnapshot(ctx, costContext, actualBreakdown, adminID)
	if err != nil || duplicateSnapshot.ID != actualSnapshot.ID {
		t.Fatalf("duplicate actual snapshot was not idempotent: first=%s second=%s err=%v", actualSnapshot.ID, duplicateSnapshot.ID, err)
	}

	// Price and operating configuration changes must affect future estimates,
	// never the actual snapshot just recorded against profile version 2.
	if _, err = tx.Exec(ctx, `UPDATE material_spools SET cost_per_kg_cents=90000,version=version+1,updated_at=now() WHERE id=$1`, spoolID); err != nil {
		t.Fatal("update fixture spool price:", err)
	}
	base.ExpectedVersion = 2
	base.ElectricityRateCentsPerKWh = 777
	base.PrinterPowerMilliwatts = ptrInt64(300000)
	base.MachineAcquisitionCents = 200000
	base.MaintenanceRateCentsPerHour = 400
	base.LaborRateCentsPerHour = 6000
	base.FinishingSeconds = 300
	third, err := repo.SaveCostProfile(ctx, printerID, base, adminID)
	if err != nil || third.Version != 3 {
		t.Fatalf("save revised profile version=%d err=%v", third.Version, err)
	}
	oldSnapshots, err := repo.ListRunCostSnapshots(ctx, actualRunID)
	if err != nil || len(oldSnapshots) != 1 {
		t.Fatalf("load historical snapshot: count=%d err=%v", len(oldSnapshots), err)
	}
	var materialBreakdown []struct {
		Price int64 `json:"cost_per_kg_snapshot_cents"`
	}
	if err = json.Unmarshal(oldSnapshots[0].MaterialBreakdown, &materialBreakdown); err != nil || len(materialBreakdown) != 1 || materialBreakdown[0].Price != 20000 || oldSnapshots[0].CostProfileVersion != 2 || oldSnapshots[0].ElectricityRateCentsPerKWh != 125 || oldSnapshots[0].MachineAcquisitionCents != 100000 || oldSnapshots[0].MachineUsefulLifeSeconds != 36000000 || oldSnapshots[0].MaintenanceRateCentsPerHour != 20 || oldSnapshots[0].LaborRateCentsPerHour != 3000 || oldSnapshots[0].TotalCostCents != 4543 {
		t.Fatalf("historical snapshot changed after rate/config updates: %+v material=%+v err=%v", oldSnapshots, materialBreakdown, err)
	}
	snapshotMutationTx, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, snapshotMutationErr := snapshotMutationTx.Exec(ctx, `UPDATE production_run_cost_snapshots SET total_cost_cents=1 WHERE id=$1`, actualSnapshot.ID)
	_ = snapshotMutationTx.Rollback(ctx)
	if snapshotMutationErr == nil {
		t.Fatal("immutable historical cost snapshot accepted an update")
	}

	estimateContext, err := repo.LoadRunCostContext(ctx, estimateRunID, "estimate", nil)
	if err != nil {
		t.Fatal("load estimate context from latest price/profile:", err)
	}
	estimate, err := domain.CalculateManufacturingCost(estimateContext.Input, estimateContext.Quantity)
	if err != nil || estimate.MaterialTotalCents != 9000 || estimate.EnergyCents != 233 || estimate.DepreciationCents != 20 || estimate.MaintenanceCents != 400 || estimate.LaborTotalCents != 1500 || estimate.TotalCostCents != 11153 {
		t.Fatalf("estimate did not use current profile and spool price: %+v err=%v", estimate, err)
	}
	badContext := estimateContext
	badContext.PrinterID = "00000000-0000-4000-8000-000000000000" // FK failure occurs inside the snapshot transaction.
	if _, err = repo.SaveRunCostSnapshot(ctx, badContext, estimate, adminID); err == nil {
		t.Fatal("expected invalid printer FK to abort snapshot registration")
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1`, estimateRunID).Scan(&snapshotCount); err != nil || snapshotCount != 0 {
		t.Fatalf("failed snapshot transaction left partial row count=%d err=%v", snapshotCount, err)
	}
	if _, err = repo.SaveRunCostSnapshot(ctx, estimateContext, estimate, adminID); err != nil {
		t.Fatal("save valid estimate snapshot after failed transaction:", err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_runs WHERE production_job_id=$1`, jobID).Scan(&snapshotCount); err != nil || snapshotCount != 3 {
		t.Fatalf("reprint fixture run count=%d err=%v", snapshotCount, err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id IN ($1,$2)`, actualRunID, estimateRunID).Scan(&snapshotCount); err != nil || snapshotCount != 2 {
		t.Fatalf("fixture snapshot count=%d err=%v", snapshotCount, err)
	}

	inner, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, mutationErr := inner.Exec(ctx, `UPDATE production_cost_profiles SET electricity_rate_cents_per_kwh=1 WHERE printer_id=$1`, printerID)
	if mutationErr == nil {
		_ = inner.Rollback(ctx)
		t.Fatal("expected append-only profile trigger to reject update")
	}
	_ = inner.Rollback(ctx)
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("profile rows=%d err=%v", count, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM production_cost_profiles WHERE printer_id=$1)+(SELECT count(*) FROM printers WHERE id=$1::uuid)`, printerID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("fixture survived rollback: %d", count)
	}
	if err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM admin_users WHERE id=$1) +
		(SELECT count(*) FROM printers WHERE id=$2) +
		(SELECT count(*) FROM materials WHERE id=$3) +
		(SELECT count(*) FROM colors WHERE id=$4) +
		(SELECT count(*) FROM quotes WHERE id=$5) +
		(SELECT count(*) FROM quote_proposals WHERE id=$6) +
		(SELECT count(*) FROM orders WHERE id=$7) +
		(SELECT count(*) FROM order_items WHERE id=$8) +
		(SELECT count(*) FROM payments WHERE order_id=$7) +
		(SELECT count(*) FROM production_jobs WHERE id=$9) +
		(SELECT count(*) FROM production_runs WHERE id IN ($10,$11,$12)) +
		(SELECT count(*) FROM production_run_materials WHERE production_run_id=$11) +
		(SELECT count(*) FROM material_spools WHERE id=$13) +
		(SELECT count(*) FROM production_cost_profiles WHERE printer_id=$2) +
		(SELECT count(*) FROM production_cost_profile_current WHERE printer_id=$2) +
		(SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id IN ($11,$12))`, adminID, printerID, materialID, colorID, quoteID, proposalID, orderID, itemID, jobID, failedRunID, actualRunID, estimateRunID, spoolID).Scan(&count); err != nil {
		t.Fatal("verify all transaction-local fixture cleanup:", err)
	}
	if count != 0 {
		t.Fatalf("transaction fixtures survived rollback across tables: %d", count)
	}
}

func ptrInt64(v int64) *int64 { return &v }
