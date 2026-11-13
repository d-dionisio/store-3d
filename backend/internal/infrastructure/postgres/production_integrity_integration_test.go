package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This suite is intentionally opt-in because it exercises real PostgreSQL
// triggers and concurrent row locks. Point TEST_DATABASE_URL at an isolated,
// fully migrated database.
func TestProductionIntegrityIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	// This test deliberately exercises immutable audit records and concurrent
	// row locks. Never let it write its full commercial graph to a development
	// database; provision and dispose a dedicated *_test database instead.
	if !strings.HasSuffix(strings.ToLower(poolConfig.ConnConfig.Database), "_test") {
		t.Fatalf("TEST_DATABASE_URL must point to a dedicated database whose name ends in _test; got %q", poolConfig.ConnConfig.Database)
	}
	if developmentURL := strings.TrimSpace(os.Getenv("DATABASE_URL")); developmentURL != "" && developmentURL == strings.TrimSpace(url) {
		t.Fatal("TEST_DATABASE_URL must not equal DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var materialID, colorID, categoryID, productID, variantID, userID string
	var orderID, itemID, adminID, printerID, spoolID string
	mustScan := func(query string, args []any, destination ...any) {
		t.Helper()
		if err := pool.QueryRow(ctx, query, args...).Scan(destination...); err != nil {
			t.Fatal(err)
		}
	}
	mustScan(`INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id`, []any{"mat-" + suffix, "Material " + suffix}, &materialID)
	mustScan(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#112233') RETURNING id`, []any{"cor-" + suffix, "Cor " + suffix}, &colorID)
	mustScan(`INSERT INTO categories(name,slug) VALUES($1,$2) RETURNING id`, []any{"Categoria " + suffix, "categoria-" + suffix}, &categoryID)
	// Production fixtures must never enter the commercial catalog queries.
	mustScan(`INSERT INTO products(category_id,slug,name,description,status) VALUES($1,$2,$3,'Teste','draft') RETURNING id`, []any{categoryID, "produto-" + suffix, "Produto " + suffix}, &productID)
	mustScan(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity) VALUES($1,$2,$3,$4,1000,10) RETURNING id`, []any{productID, materialID, colorID, "SKU-" + suffix}, &variantID)
	mustScan(`INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Cliente Produção',now()) RETURNING id`, []any{"production-" + suffix + "@example.com"}, &userID)
	mustScan(`INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,subtotal_cents,total_cents,payment_readiness) VALUES($1,$2,'confirmed','Cliente Produção',$3,1000,1000,'ready') RETURNING id`, []any{"PED-PROD-" + suffix, userID, "production-" + suffix + "@example.com"}, &orderID)
	mustScan(`INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents) VALUES($1,'catalog',$2,$3,'Produto','Snapshot',$4,'PLA','Preto',1,1000) RETURNING id`, []any{orderID, productID, variantID, "SKU-" + suffix}, &itemID)
	mustScan(`INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Gerente Produção',repeat('x',60),'production_manager') RETURNING id`, []any{"manager-" + suffix + "@example.com"}, &adminID)
	mustScan(`INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'A1 Teste','Bambu Lab','A1',256,256,256) RETURNING id`, []any{"printer-" + suffix}, &printerID)
	mustScan(`INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents) VALUES($1,$2,$3,'Teste','available',100,100,10000) RETURNING id`, []any{"spool-" + suffix, materialID, colorID}, &spoolID)

	var job1, job2, run1, run2 string
	mustScan(`INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity) VALUES($1,$2,$3,1) RETURNING id`, []any{itemID, materialID, colorID}, &job1)
	if _, err = pool.Exec(ctx, `SELECT transition_production_job($1,'queued',$2,'Fila sem pagamento',1)`, job1, adminID); err == nil {
		t.Fatal("queue accepted without paid payment")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,paid_amount_cents,refunded_amount_cents,currency,paid_at) VALUES($1,'pix','paid',1000,1000,0,'BRL',now())`, orderID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `SELECT transition_production_job($1,'queued',$2,'Pagamento integral confirmado',1)`, job1, adminID); err != nil {
		t.Fatal(err)
	}
	mustScan(`INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity) VALUES($1,$2,1,1) RETURNING id`, []any{job1, printerID}, &run1)

	mustScan(`INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity,parent_job_id) VALUES($1,$2,$3,1,$4) RETURNING id`, []any{itemID, materialID, colorID, job1}, &job2)
	mustScan(`INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity) VALUES($1,$2,1,1) RETURNING id`, []any{job2, printerID}, &run2)

	var successes atomic.Int32
	var wg sync.WaitGroup
	for _, runID := range []string{run1, run2} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, execErr := pool.Exec(ctx, `INSERT INTO production_run_materials(production_run_id,material_spool_id,consumed_grams,wasted_grams,cost_per_kg_snapshot_cents,total_cost_cents,recorded_by_admin_user_id) VALUES($1,$2,80,0,0,0,$3)`, id, spoolID, adminID)
			if execErr == nil {
				successes.Add(1)
			}
		}(runID)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent consumption successes=%d, want 1", successes.Load())
	}
	var balance int
	if err = pool.QueryRow(ctx, `SELECT current_weight_grams FROM material_spools WHERE id=$1`, spoolID).Scan(&balance); err != nil || balance != 20 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}

	if _, err = pool.Exec(ctx, `SELECT transition_production_run($1,'preparing',$2,'Preparação',1)`, run1, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `SELECT transition_production_run($1,'failed',$2,'Falha de impressão',2)`, run1, adminID); err != nil {
		t.Fatal(err)
	}
	var reprint string
	mustScan(`INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity) VALUES($1,$2,$3,2,1) RETURNING id`, []any{job1, run1, printerID}, &reprint)
	if reprint == "" {
		t.Fatal("reprint was not created")
	}
	if _, err = pool.Exec(ctx, `UPDATE production_events SET description='tampered' WHERE production_job_id=$1`, job1); err == nil {
		t.Fatal("append-only event accepted an update")
	}
}
