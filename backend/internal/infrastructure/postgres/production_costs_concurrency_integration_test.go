package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This integration test uses two independent PostgreSQL pools and explicitly
// holds row locks until both repository calls are observed waiting on them.
// Its disposable schema is cloned from the migration-28 tables and dropped by
// its exact generated name after the test; it never touches commercial rows.
func TestProductionCostsConcurrencyPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(testDatabaseURL())
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL; credentials hidden")
	}
	if cfg.ConnConfig.Database != "loja_3d_test" || strings.EqualFold(cfg.ConnConfig.Database, "loja_3d") {
		t.Fatal("refusing any database except loja_3d_test")
	}
	host := strings.ToLower(cfg.ConnConfig.Host)
	if (host != "localhost" && host != "127.0.0.1" && host != "::1") || cfg.ConnConfig.Port != 5432 {
		t.Fatal("test database must use loopback:5432")
	}
	dev := strings.TrimSpace(testDevelopmentDatabaseURL())
	devCfg, err := pgxpool.ParseConfig(dev)
	if err != nil || dev == dsn || devCfg.ConnConfig.Database != "loja_3d" {
		t.Fatal("DATABASE_URL and TEST_DATABASE_URL must be distinct and target loja_3d and loja_3d_test respectively")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	adminCfg := cfg.Copy()
	adminCfg.ConnConfig.RuntimeParams["application_name"] = "cost-concurrency-setup"
	adminCfg.MaxConns, adminCfg.MinConns = 4, 0
	observer, err := pgxpool.NewWithConfig(ctx, adminCfg)
	if err != nil {
		t.Fatal("connect isolated PostgreSQL test database")
	}
	t.Cleanup(observer.Close)
	var dbName, serverVersion string
	if err = observer.QueryRow(ctx, `SELECT current_database(), current_setting('server_version')`).Scan(&dbName, &serverVersion); err != nil {
		t.Fatal("verify isolated database connection")
	}
	if dbName != "loja_3d_test" || !strings.HasPrefix(serverVersion, "17.") {
		t.Fatalf("database guard failed: database=%q server=%q", dbName, serverVersion)
	}
	var migration int
	var dirty bool
	if err = observer.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&migration, &dirty); err != nil || migration != 28 || dirty {
		t.Fatalf("expected migration 28 clean before fixture setup; version=%d dirty=%v err=%v", migration, dirty, err)
	}

	schema := fmt.Sprintf("cost_concurrency_%d", time.Now().UnixNano())
	if _, err = observer.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("create uniquely named disposable test schema:", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := observer.Exec(cleanupCtx, `DROP SCHEMA `+schema+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop exact disposable schema %s: %v", schema, cleanupErr)
			return
		}
		var exists bool
		if cleanupErr := observer.QueryRow(cleanupCtx, `SELECT to_regnamespace($1) IS NOT NULL`, schema).Scan(&exists); cleanupErr != nil || exists {
			t.Errorf("disposable schema residue exists=%v err=%v", exists, cleanupErr)
		}
	})

	for _, table := range []string{"printers", "admin_users", "admin_audit_events", "production_runs", "production_cost_profiles", "production_cost_profile_current", "production_run_cost_snapshots"} {
		if _, err = observer.Exec(ctx, `CREATE TABLE `+schema+`.`+table+` (LIKE public.`+table+` INCLUDING ALL)`); err != nil {
			t.Fatalf("clone %s table into disposable test schema: %v", table, err)
		}
	}
	for _, trigger := range []string{
		`CREATE TRIGGER production_cost_profiles_append_only BEFORE UPDATE OR DELETE ON ` + schema + `.production_cost_profiles FOR EACH ROW EXECUTE FUNCTION public.prevent_production_cost_history_mutation()`,
		`CREATE TRIGGER production_run_cost_snapshots_append_only BEFORE UPDATE OR DELETE ON ` + schema + `.production_run_cost_snapshots FOR EACH ROW EXECUTE FUNCTION public.prevent_production_cost_history_mutation()`,
		`CREATE TRIGGER production_run_cost_snapshot_validate BEFORE INSERT ON ` + schema + `.production_run_cost_snapshots FOR EACH ROW EXECUTE FUNCTION public.validate_production_cost_snapshot()`,
	} {
		if _, err = observer.Exec(ctx, trigger); err != nil {
			t.Fatal("install migration-defined cost triggers in disposable schema:", err)
		}
	}

	var adminID, printerID, runID string
	suffix := fmt.Sprintf("cost-race-%d", time.Now().UnixNano())
	if err = observer.QueryRow(ctx, `INSERT INTO `+schema+`.admin_users(email,full_name,password_hash,role) VALUES($1,'Cost race fixture','012345678901234567890123456789012345678901234567890123456789','production_manager') RETURNING id::text`, suffix+"@example.invalid").Scan(&adminID); err != nil {
		t.Fatal("insert isolated admin fixture:", err)
	}
	if err = observer.QueryRow(ctx, `INSERT INTO `+schema+`.printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'Cost race printer','Fixture','No hardware',100,100,100) RETURNING id::text`, suffix).Scan(&printerID); err != nil {
		t.Fatal("insert isolated printer fixture:", err)
	}
	if err = observer.QueryRow(ctx, `INSERT INTO `+schema+`.production_runs(production_job_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,completed_at) VALUES(gen_random_uuid(),$1,1,1,'completed',1,now()-interval '1 minute',now()) RETURNING id::text`, printerID).Scan(&runID); err != nil {
		t.Fatal("insert isolated completed run fixture:", err)
	}

	newPool := func(appName string) *pgxpool.Pool {
		poolCfg := cfg.Copy()
		poolCfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
		poolCfg.ConnConfig.RuntimeParams["application_name"] = appName
		poolCfg.MaxConns, poolCfg.MinConns = 3, 0
		p, poolErr := pgxpool.NewWithConfig(ctx, poolCfg)
		if poolErr != nil {
			t.Fatalf("open independent pool %s: %v", appName, poolErr)
		}
		t.Cleanup(p.Close)
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
	if err = lock.QueryRow(ctx, `SELECT id::text FROM `+schema+`.printers WHERE id=$1 FOR UPDATE`, printerID).Scan(&lockedID); err != nil {
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
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&profileRows); err != nil || profileRows != 2 {
		t.Fatalf("profile race left rows=%d err=%v, want baseline plus one winning version", profileRows, err)
	}
	t.Log("profile race: exactly one expected-version=1 update committed as version 2; one conflicted")

	profile, err := repoA.GetCurrentCostProfile(ctx, printerID)
	if err != nil {
		t.Fatal("load winning profile:", err)
	}
	var runVersion int64
	if err = observer.QueryRow(ctx, `SELECT version FROM `+schema+`.production_runs WHERE id=$1`, runID).Scan(&runVersion); err != nil {
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
	if err = lock.QueryRow(ctx, `SELECT id::text FROM `+schema+`.production_runs WHERE id=$1 FOR UPDATE`, runID).Scan(&lockedID); err != nil {
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
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_run_cost_snapshots WHERE production_run_id=$1 AND snapshot_type='actual'`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
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
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&afterStale); err != nil || afterStale != 2 {
		t.Fatalf("stale update left partial profile row count=%d err=%v", afterStale, err)
	}
	var rollbackRunID string
	if err = observer.QueryRow(ctx, `INSERT INTO `+schema+`.production_runs(production_job_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,completed_at) VALUES(gen_random_uuid(),$1,1,1,'completed',1,now()-interval '1 minute',now()) RETURNING id::text`, printerID).Scan(&rollbackRunID); err != nil {
		t.Fatal("insert isolated rollback run fixture:", err)
	}
	badCtx := costCtx
	badCtx.RunID = rollbackRunID
	badCtx.RunVersion = 1
	badCtx.PrinterID = "00000000-0000-4000-8000-000000000000"
	if _, err = repoA.SaveRunCostSnapshot(ctx, badCtx, breakdown, adminID, app.CostAuditContext{}); err == nil {
		t.Fatal("snapshot with mismatched run printer should fail")
	}
	var rollbackRows int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_run_cost_snapshots WHERE production_run_id=$1`, rollbackRunID).Scan(&rollbackRows); err != nil || rollbackRows != 0 {
		t.Fatalf("failed snapshot transaction left partial rows=%d err=%v", rollbackRows, err)
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_run_cost_snapshots WHERE production_run_id=$1`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
		t.Fatalf("rollback did not preserve the successful historical snapshot: rows=%d err=%v", snapshotRows, err)
	}
	if _, err = observer.Exec(ctx, `UPDATE `+schema+`.production_run_cost_snapshots SET total_cost_cents=total_cost_cents+1 WHERE id=$1`, snapshotID); err == nil {
		t.Fatal("append-only snapshot trigger allowed mutation")
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.production_run_cost_snapshots WHERE production_run_id=$1`, runID).Scan(&snapshotRows); err != nil || snapshotRows != 1 {
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
