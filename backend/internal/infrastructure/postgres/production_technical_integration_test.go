package postgres

import (
	"context"
	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProductionTechnicalIntegrityIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.ToLower(cfg.ConnConfig.Database), "_test") || strings.EqualFold(cfg.ConnConfig.Database, "loja_3d") {
		t.Fatal("technical integration requires *_test")
	}
	if dev := strings.TrimSpace(os.Getenv("DATABASE_URL")); dev != "" && dev == dsn {
		t.Fatal("test database must differ")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	suffix := fmt.Sprint(time.Now().UnixNano())
	var admin, material, color, printer, printer2, spool, spool2, spec, installation string
	must := func(q string, args []any, target *string) {
		t.Helper()
		if err := tx.QueryRow(ctx, q, args...).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO admin_users(email,full_name,password_hash,role)VALUES($1,'Technical Test',repeat('x',60),'production_manager')RETURNING id`, []any{"tech-" + suffix + "@example.test"}, &admin)
	must(`INSERT INTO materials(code,name)VALUES($1,$2)RETURNING id`, []any{"TECH-" + suffix, "Technical " + suffix}, &material)
	must(`INSERT INTO colors(slug,name,hex_code)VALUES($1,$2,'#123456')RETURNING id`, []any{"tech-" + suffix, "Technical " + suffix}, &color)
	must(`INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm)VALUES($1,'Tech A','Test','Model',256,256,256)RETURNING id`, []any{"TECH-P-" + suffix}, &printer)
	must(`INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm)VALUES($1,'Tech B','Test','Model',256,256,256)RETURNING id`, []any{"TECH-P2-" + suffix}, &printer2)
	must(`INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents)VALUES($1,$2,$3,'Test','available',1000,1000,1000)RETURNING id`, []any{"TECH-S-" + suffix, material, color}, &spool)
	must(`INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents)VALUES($1,$2,$3,'Test','available',1000,1000,1000)RETURNING id`, []any{"TECH-S2-" + suffix, material, color}, &spool2)
	must(`INSERT INTO printer_technical_specs(printer_id,version,status,technology,max_width_mm,max_depth_mm,max_height_mm,max_hotend_temp_c,max_bed_temp_c,enclosed_chamber,source_reference,source_verified_at,created_by_admin_user_id)VALUES($1,1,'verified','fdm',256,256,256,300,110,false,'test fixture',now(),$2)RETURNING id`, []any{printer, admin}, &spec)
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO printer_supported_materials VALUES($1,$2)`, []any{spec, material}},
		{`INSERT INTO printer_supported_nozzles VALUES($1,400,'hardened_steel')`, []any{spec}},
		{`INSERT INTO printer_operational_configs(printer_id,status,installed_nozzle_microns,installed_nozzle_type,material_slot_count,updated_by_admin_user_id)VALUES($1,'configured',400,'hardened_steel',1,$2)`, []any{printer, admin}},
		{`INSERT INTO material_technical_specs(material_id,print_temp_min_c,print_temp_max_c,print_temp_recommended_c,bed_temp_min_c,bed_temp_max_c,bed_temp_recommended_c,status,source_reference,source_verified_at,updated_by_admin_user_id)VALUES($1,190,230,210,40,70,60,'verified','test fixture',now(),$2)`, []any{material, admin}},
	} {
		if _, err = tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO spool_installations(printer_id,material_spool_id,slot_number,installed_by_admin_user_id)VALUES($1,$2,1,$3)RETURNING id`, []any{printer, spool, admin}, &installation)
	if _, err = tx.Exec(ctx, `SAVEPOINT duplicate_spool`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO spool_installations(printer_id,material_spool_id,slot_number,installed_by_admin_user_id)VALUES($1,$2,1,$3)`, printer2, spool, admin); err == nil {
		t.Fatal("same spool installed twice")
	}
	if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT duplicate_spool`); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if _, err = tx.Exec(ctx, `SAVEPOINT duplicate_slot`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO spool_installations(printer_id,material_spool_id,slot_number,installed_by_admin_user_id)VALUES($1,$2,1,$3)`, printer, spool2, admin); err == nil {
		t.Fatal("same slot used twice")
	}
	if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT duplicate_slot`); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	repository := &ProductionRepository{pool: tx}
	size := 100
	result, err := repository.TechnicalCompatibility(ctx, app.TechnicalCompatibilityRequest{PrinterID: printer, MaterialID: material, WidthMM: &size, DepthMM: &size, HeightMM: &size})
	if err != nil || result.Status != domain.CompatibilityCompatible {
		t.Fatalf("compatibility=%+v err=%v", result, err)
	}
	if _, err = tx.Exec(ctx, `UPDATE spool_installations SET removed_at=now(),removed_by_admin_user_id=$2,removal_reason='test removal' WHERE id=$1`, installation, admin); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE spool_installations SET removal_reason='retroactive edit' WHERE id=$1`, installation); err == nil {
		t.Fatal("removed history was mutable")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var residual int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM printers WHERE code LIKE 'TECH-P-%' OR code LIKE 'TECH-P2-%'`).Scan(&residual); err != nil || residual != 0 {
		t.Fatalf("rollback residual=%d err=%v", residual, err)
	}
}
