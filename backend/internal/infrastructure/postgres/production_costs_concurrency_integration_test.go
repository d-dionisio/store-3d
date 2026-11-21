package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This integration test uses two independent PostgreSQL pools and explicitly
// holds row locks until both repository calls are observed waiting on them.
// The schema is created by TestSupport, migrated using the official migration
// set, and dropped by its exact owned name after the test.
func TestProductionCostsConcurrencyPostgreSQL(t *testing.T) {
	f := newConversionFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	observer := f.pool
	var err error
	var serverVersion string
	if err = observer.QueryRow(ctx, `SELECT current_setting('server_version')`).Scan(&serverVersion); err != nil {
		t.Fatal("read isolated PostgreSQL server version:", err)
	}
	suffix, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique printer fixture identifier:", err)
	}
	adminID := ""
	if err := observer.QueryRow(ctx, `SELECT created_by::text FROM quote_proposals WHERE id=$1`, f.proposalID).Scan(&adminID); err != nil {
		t.Fatal("load isolated fixture administrator:", err)
	}
	if _, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID); err != nil {
		t.Fatal("convert isolated quote into a production order fixture:", err)
	}
	var orderID, jobID, printerID string
	var total int64
	if err := observer.QueryRow(ctx, `SELECT id::text,total_cents FROM orders WHERE quote_id=$1`, f.quoteID).Scan(&orderID, &total); err != nil {
		t.Fatal("load converted fixture order:", err)
	}
	if _, err := observer.Exec(ctx, `UPDATE orders SET status='confirmed' WHERE id=$1`, orderID); err != nil {
		t.Fatal("confirm isolated fixture order:", err)
	}
	if _, err := observer.Exec(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,paid_amount_cents,currency,paid_at) VALUES($1,'pix','paid',$2,$2,'BRL',now())`, orderID, total); err != nil {
		t.Fatal("settle isolated fixture order:", err)
	}
	if _, err := observer.Exec(ctx, `SELECT create_production_jobs_for_paid_order($1)`, orderID); err != nil {
		t.Fatal("create production job through official paid-order path:", err)
	}
	if err := observer.QueryRow(ctx, `SELECT id::text FROM production_jobs WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id=$1)`, orderID).Scan(&jobID); err != nil {
		t.Fatal("load isolated production job:", err)
	}
	if err := observer.QueryRow(ctx, `INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'Cost race printer','Fixture','No hardware',100,100,100) RETURNING id::text`, "race-"+suffix).Scan(&printerID); err != nil {
		t.Fatal("insert isolated printer fixture:", err)
	}
	var failedRunID, runID string
	if err := observer.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,failure_reason) VALUES($1,$2,1,1,'failed',1,now()-interval '2 minutes','fixture reprint source') RETURNING id::text`, jobID, printerID).Scan(&failedRunID); err != nil {
		t.Fatal("insert isolated failed reprint source:", err)
	}
	if err := observer.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,completed_at) VALUES($1,$2,$3,2,1,'completed',1,now()-interval '1 minute',now()) RETURNING id::text`, jobID, failedRunID, printerID).Scan(&runID); err != nil {
		t.Fatal("insert isolated completed run fixture:", err)
	}

	newPool := func(appName string) *pgxpool.Pool {
		p, poolErr := f.schema.OpenPoolWithApplicationName(ctx, appName)
		if poolErr != nil {
			t.Fatalf("open independent pool %s: %v", appName, poolErr)
		}
		return p
	}
	poolA, poolB := newPool("cost-race-a"), newPool("cost-race-b")
	repoA, repoB := NewProductionCostRepository(poolA), NewProductionCostRepository(poolB)
	var pidA, pidB int
	if err = poolA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidA); err != nil {
		t.Fatal(err)
	}
	if err = poolB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidB); err != nil {
		t.Fatal(err)
	}
	if pidA == pidB {
		t.Fatalf("race participants unexpectedly share PostgreSQL backend pid=%d", pidA)
	}
	t.Logf("independent PostgreSQL backends for profile race: pidA=%d pidB=%d server=%s", pidA, pidB, serverVersion)

	base := app.CostProfileInput{ElectricityRateCentsPerKWh: 110, PrinterPowerMilliwatts: ptrInt64(100000), MachineAcquisitionCents: 100000, MachineUsefulLifeSeconds: 36000000, MaintenanceRateCentsPerHour: 20, LaborRateCentsPerHour: 3000}
	initialProfile, err := repoA.SaveCostProfile(ctx, printerID, base, adminID, app.CostAuditContext{})
	if err != nil || initialProfile.Version != 1 {
		t.Fatalf("create baseline profile before concurrent update: version=%d err=%v", initialProfile.Version, err)
	}
	base.ExpectedVersion = initialProfile.Version
	base.ElectricityRateCentsPerKWh = 125
	lock, err := observer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var lockedID string
	if err = lock.QueryRow(ctx, `SELECT id::text FROM printers WHERE id=$1 FOR UPDATE`, printerID).Scan(&lockedID); err != nil {
		_ = lock.Rollback(ctx)
		t.Fatal("lock fixture printer:", err)
	}
	profileStart := make(chan struct{})
	profileResults := make(chan struct {
		view app.CostProfileView
		err  error
	}, 2)
	for _, repo := range []*ProductionCostRepository{repoA, repoB} {
		go func(repo *ProductionCostRepository) {
			<-profileStart
			v, saveErr := repo.SaveCostProfile(ctx, printerID, base, adminID, app.CostAuditContext{})
			profileResults <- struct {
				view app.CostProfileView
				err  error
			}{v, saveErr}
		}(repo)
	}
	close(profileStart)
	waitForLockWaiters(t, ctx, observer, "cost-race-a", "cost-race-b")
	if err = lock.Commit(ctx); err != nil {
		t.Fatal("release profile race lock:", err)
	}
	accepted, conflicted := 0, 0
	for range 2 {
		result := <-profileResults
		if result.err == nil && result.view.Version == 2 {
			accepted++
		} else if errors.Is(result.err, app.ErrConflict) {
			conflicted++
		} else {
			t.Fatalf("unexpected concurrent profile result: version=%d err=%v", result.view.Version, result.err)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("profile optimistic concurrency: accepted=%d conflicts=%d", accepted, conflicted)
	}
	var profileRows int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&profileRows); err != nil || profileRows != 2 {
		t.Fatalf("profile race left rows=%d err=%v, want baseline plus one winning version", profileRows, err)
	}
	t.Log("profile race: exactly one expected-version=1 update committed as version 2; one conflicted")

	profile, err := repoA.GetCurrentCostProfile(ctx, printerID)
	if err != nil {
		t.Fatal("load winning profile:", err)
	}
	var runVersion int64
	if err = observer.QueryRow(ctx, `SELECT version FROM production_runs WHERE id=$1`, runID).Scan(&runVersion); err != nil {
		t.Fatal(err)
	}
	costCtx := app.CostContext{
		Profile: domain.CostProfile{PrinterID: printerID, Version: profile.Version, ElectricityRateCentsPerKWh: profile.ElectricityRateCentsPerKWh, PrinterPowerMilliwatts: profile.PrinterPowerMilliwatts, MachineAcquisitionCents: profile.MachineAcquisitionCents, MachineUsefulLifeSeconds: profile.MachineUsefulLifeSeconds, MaintenanceRateCentsPerHour: profile.MaintenanceRateCentsPerHour, LaborRateCentsPerHour: profile.LaborRateCentsPerHour, PreparationSeconds: profile.PreparationSeconds, FinishingSeconds: profile.FinishingSeconds},
		Input:   domain.CostInput{EnergyMilliWh: 1000, EnergyRateCentsKWh: profile.ElectricityRateCentsPerKWh, MachineAcquisitionCents: profile.MachineAcquisitionCents, MachineUsefulLifeSeconds: profile.MachineUsefulLifeSeconds, MaintenanceRateCentsHour: profile.MaintenanceRateCentsPerHour, LaborRateCentsHour: profile.LaborRateCentsPerHour, RunSeconds: 60},
		RunID:   runID, PrinterID: printerID, SnapshotType: "actual", EnergySource: "configured_power", ProfileID: profile.ID, RunVersion: runVersion, Quantity: 1, MaterialBreakdown: []byte("[]"),
	}
	breakdown, err := domain.CalculateManufacturingCost(costCtx.Input, costCtx.Quantity)
	if err != nil {
		t.Fatal("calculate deterministic concurrent test snapshot:", err)
	}
	lock, err = observer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = lock.QueryRow(ctx, `SELECT id::text FROM production_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&lockedID); err != nil {
		_ = lock.Rollback(ctx)
		t.Fatal("lock fixture run:", err)
	}
	snapshotStart := make(chan struct{})
	snapshotResults := make(chan struct {
		view app.CostSnapshotView
		err  error
	}, 2)
	for _, repo := range []*ProductionCostRepository{repoA, repoB} {
		go func(repo *ProductionCostRepository) {
			<-snapshotStart
			v, saveErr := repo.SaveRunCostSnapshot(ctx, costCtx, breakdown, adminID, app.CostAuditContext{})
			snapshotResults <- struct {
				view app.CostSnapshotView
				err  error
			}{v, saveErr}
		}(repo)
	}
	close(snapshotStart)
	waitForLockWaiters(t, ctx, observer, "cost-race-a", "cost-race-b")
	if err = lock.Commit(ctx); err != nil {
		t.Fatal("release snapshot race lock:", err)
	}
	var snapshotID string
	for range 2 {
		result := <-snapshotResults
		if result.err != nil {
			t.Fatalf("concurrent snapshot operation failed: %v", result.err)
		}
		if snapshotID == "" {
			snapshotID = result.view.ID
		} else if result.view.ID != snapshotID {
			t.Fatalf("concurrent snapshot generated different IDs: %s and %s", snapshotID, result.view.ID)
		}
	}
	var snapshotRows int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1 AND snapshot_type='actual'`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
		t.Fatalf("concurrent snapshots left rows=%d err=%v, want one", snapshotRows, err)
	}
	t.Logf("snapshot race: both independent callers returned the same snapshot id %s; persisted rows=1", snapshotID)

	var duplicateErr error
	_, duplicateErr = poolA.Exec(ctx, `INSERT INTO production_run_cost_snapshots SELECT gen_random_uuid(),production_run_id,snapshot_type,printer_id,cost_profile_id,cost_profile_version,energy_source,energy_milliwh,run_seconds,preparation_seconds,finishing_seconds,consumed_grams,wasted_grams,material_consumed_cost_cents,material_waste_cost_cents,material_total_cost_cents,electricity_rate_cents_per_kwh,energy_cost_cents,depreciation_cost_cents,maintenance_cost_cents,preparation_cost_cents,finishing_cost_cents,labor_cost_cents,total_cost_cents,unit_cost_cents,quantity,revenue_snapshot_cents,contribution_margin_cents,contribution_margin_basis_points,profit_cents,profit_margin_basis_points,material_breakdown,created_by_admin_user_id,created_at FROM production_run_cost_snapshots WHERE id=$1`, snapshotID)
	var pgErr *pgconn.PgError
	if !errors.As(duplicateErr, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("unique constraint did not reject second snapshot for same run/type: %v", duplicateErr)
	}
	if _, err = repoA.SaveCostProfile(ctx, printerID, app.CostProfileInput{ExpectedVersion: 1, ElectricityRateCentsPerKWh: 222, PrinterPowerMilliwatts: ptrInt64(100000), MachineUsefulLifeSeconds: 1000}, adminID, app.CostAuditContext{}); !errors.Is(err, app.ErrConflict) {
		t.Fatalf("stale profile version did not conflict: %v", err)
	}
	var afterStale int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&afterStale); err != nil || afterStale != 2 {
		t.Fatalf("stale update left partial profile row count=%d err=%v", afterStale, err)
	}
	var rollbackRunID string
	if err = observer.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,completed_at) VALUES($1,$2,$3,3,1,'completed',1,now()-interval '1 minute',now()) RETURNING id::text`, jobID, failedRunID, printerID).Scan(&rollbackRunID); err != nil {
		t.Fatal("insert isolated rollback snapshot run:", err)
	}
	badCtx := costCtx
	badPrinterID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique invalid printer reference:", err)
	}
	badCtx.RunID = rollbackRunID
	badCtx.RunVersion = 1
	badCtx.PrinterID = badPrinterID
	if _, err = repoA.SaveRunCostSnapshot(ctx, badCtx, breakdown, adminID, app.CostAuditContext{}); err == nil {
		t.Fatal("snapshot with mismatched run printer should fail")
	}
	var rollbackRows int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1`, rollbackRunID).Scan(&rollbackRows); err != nil || rollbackRows != 0 {
		t.Fatalf("failed snapshot transaction left partial rows=%d err=%v", rollbackRows, err)
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
		t.Fatalf("rollback did not preserve the successful historical snapshot: rows=%d err=%v", snapshotRows, err)
	}
	if _, err = observer.Exec(ctx, `UPDATE production_run_cost_snapshots SET total_cost_cents=total_cost_cents+1 WHERE id=$1`, snapshotID); err == nil {
		t.Fatal("append-only snapshot trigger allowed mutation")
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM production_run_cost_snapshots WHERE production_run_id=$1`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
		t.Fatalf("snapshot integrity after failure/mutation attempt: rows=%d err=%v", snapshotRows, err)
	}
	t.Log("rollback and integrity: stale profile, invalid snapshot, duplicate unique key and immutable-history checks passed")
}

func waitForLockWaiters(t *testing.T, ctx context.Context, observer *pgxpool.Pool, appA, appB string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var waiting, distinct int
		err := observer.QueryRow(ctx, `SELECT count(*),count(DISTINCT pid) FROM pg_stat_activity WHERE datname=current_database() AND application_name = ANY($1) AND wait_event_type='Lock'`, []string{appA, appB}).Scan(&waiting, &distinct)
		if err != nil {
			t.Fatal("observe independent PostgreSQL lock waiters:", err)
		}
		if waiting == 2 && distinct == 2 {
			t.Logf("confirmed concurrent independent backend lock waits: %s and %s", appA, appB)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("both independent repository calls did not reach PostgreSQL row locks before timeout")
}
