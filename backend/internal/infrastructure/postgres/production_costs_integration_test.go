package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
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
		t.Fatalf("stale concurrent version should conflict, got %v", err)
	}
	history, err := repo.ListCostProfileHistory(ctx, printerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Version != 2 || history[1].ElectricityRateCentsPerKWh != 110 {
		t.Fatalf("profile history did not preserve prior configuration: %+v", history)
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
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM production_cost_profiles WHERE printer_id=$1`, printerID).Scan(&count); err != nil || count != 2 {
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
}

func ptrInt64(v int64) *int64 { return &v }
