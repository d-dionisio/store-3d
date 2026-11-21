package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

func TestProductionHistoricalFallbackHierarchy(t *testing.T) {
	pool, ctx := newProductionIntegrationPool(t, "planning_history")
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	suffix := fmt.Sprintf("history-%d", time.Now().UnixNano())
	var category, material, color, color2, user, order, admin, printerA, printerX string
	mustID := func(query string, args []any, target *string) {
		t.Helper()
		if scanErr := tx.QueryRow(ctx, query, args...).Scan(target); scanErr != nil {
			t.Fatal(scanErr)
		}
	}
	mustID(`INSERT INTO categories(name,slug) VALUES($1,$2) RETURNING id`, []any{"History fixture", suffix}, &category)
	mustID(`INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id`, []any{suffix, "History material " + suffix}, &material)
	mustID(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#112233') RETURNING id`, []any{suffix, "History color " + suffix}, &color)
	mustID(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#445566') RETURNING id`, []any{suffix + "-2", "History color 2 " + suffix}, &color2)
	mustID(`INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'History fixture',now()) RETURNING id`, []any{suffix + "@example.test"}, &user)
	mustID(`INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,subtotal_cents,total_cents,payment_readiness) VALUES($1,$2,'confirmed','History fixture',$3,100000,100000,'ready') RETURNING id`, []any{"PED-" + suffix, user, suffix + "@example.test"}, &order)
	mustID(`INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'History manager',repeat('x',60),'production_manager') RETURNING id`, []any{suffix + "-admin@example.test"}, &admin)
	if _, err = tx.Exec(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,paid_amount_cents,currency,paid_at) VALUES($1,'pix','paid',100000,100000,'BRL',now())`, order); err != nil {
		t.Fatal("insert paid planning fixture:", err)
	}
	mustID(`INSERT INTO printers(code,name,manufacturer,model,status,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'History A1','Bambu Lab','A1','available',256,256,256) RETURNING id`, []any{suffix + "-a1"}, &printerA)
	mustID(`INSERT INTO printers(code,name,manufacturer,model,status,build_width_mm,build_depth_mm,build_height_mm) VALUES($1,'History X1','Bambu Lab','X1','available',256,256,256) RETURNING id`, []any{suffix + "-x1"}, &printerX)
	type productSet struct{ product, primary, secondary string }
	products := map[string]productSet{}
	for _, key := range []string{"a", "b", "c", "d"} {
		var product, primary, secondary string
		mustID(`INSERT INTO products(category_id,slug,name,description,status) VALUES($1,$2,$3,'History fixture','draft') RETURNING id`, []any{category, suffix + "-" + key, "History " + key}, &product)
		mustID(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity) VALUES($1,$2,$3,$4,100,100) RETURNING id`, []any{product, material, color, "SKU-" + suffix + "-" + key}, &primary)
		mustID(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity) VALUES($1,$2,$3,$4,100,100) RETURNING id`, []any{product, material, color2, "SKU-" + suffix + "-" + key + "-2"}, &secondary)
		products[key] = productSet{product, primary, secondary}
	}
	now := time.Now().UTC()
	sequence := 0
	addRun := func(label string, set productSet, variant, printer, status string, actual int, completedAt time.Time) string {
		t.Helper()
		sequence++
		var item, job, run string
		mustID(`INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents) VALUES($1,'catalog',$2,$3,$4,'History',$5,'History material','History color',1,100) RETURNING id`, []any{order, set.product, variant, label, fmt.Sprintf("SKU-H-%s-%d", suffix, sequence)}, &item)
		mustID(`INSERT INTO production_jobs(order_item_id,material_id,color_id,quantity,job_number) VALUES($1,$2,$3,1,$4) RETURNING id`, []any{item, material, color, fmt.Sprintf("H-%s-%03d", suffix, sequence)}, &job)
		started := any(nil)
		completed := any(nil)
		actualValue := any(nil)
		if status == "completed" {
			started = completedAt.Add(-time.Duration(actual) * time.Minute)
			completed = completedAt
			actualValue = actual
		} else if status == "printing" {
			started = now.Add(-time.Minute)
		}
		mustID(`INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity,status,actual_minutes,started_at,completed_at) VALUES($1,$2,1,1,$3,$4,$5,$6) RETURNING id`, []any{job, printer, status, actualValue, started, completed}, &run)
		if status == "queued" {
			if _, transitionErr := tx.Exec(ctx, `SELECT transition_production_job($1,'queued',$2,'Planning fixture queued',1)`, job, admin); transitionErr != nil {
				t.Fatalf("queue paid target job through production transition: %v", transitionErr)
			}
		}
		return job
	}
	addCurrent := func(key string, set productSet, printer string) string {
		t.Helper()
		return addRun("target-"+key, set, set.primary, printer, "queued", 0, time.Time{})
	}
	within := now.Add(-24 * time.Hour)
	for _, minutes := range []int{10, 20, 100} {
		addRun("a-specific", products["a"], products["a"].primary, printerA, "completed", minutes, within)
	}
	addRun("a-less-specific", products["a"], products["a"].secondary, printerA, "completed", 500, within)
	targetA := addCurrent("a", products["a"], printerA)
	for _, minutes := range []int{30, 50} {
		addRun("b-product-model", products["b"], products["b"].secondary, printerA, "completed", minutes, within)
	}
	addRun("b-product-only", products["b"], products["b"].secondary, printerX, "completed", 900, within)
	targetB := addCurrent("b", products["b"], printerA)
	for _, minutes := range []int{60, 80, 100, 120, 140, 160, 180, 200, 220, 240} {
		addRun("c-product", products["c"], products["c"].primary, printerA, "completed", minutes, within)
	}
	addRun("c-outside-window", products["c"], products["c"].primary, printerA, "completed", 999, now.Add(-91*24*time.Hour))
	addRun("c-invalid-zero", products["c"], products["c"].primary, printerA, "completed", 0, within)
	addRun("c-incomplete", products["c"], products["c"].primary, printerA, "printing", 0, time.Time{})
	targetC := addCurrent("c", products["c"], printerX)
	targetD := addCurrent("d", products["d"], printerA)
	repository := &ProductionRepository{pool: tx}
	service := app.New(repository)
	service.SetPlanningRepository(repository)
	page, err := service.ListProductionPlanning(ctx, admindomain.Session{Role: "production_manager"}, app.PlanningFilter{Page: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]app.PlanningJob{}
	for _, job := range page.Items {
		byID[job.ID] = job
	}
	assert := func(id, source, confidence string, median, samples int) {
		t.Helper()
		job, ok := byID[id]
		if !ok {
			t.Fatalf("target %s missing", id)
		}
		if job.Forecast.Source != source || string(job.Forecast.Confidence) != confidence || job.Forecast.Samples != samples {
			t.Fatalf("forecast %s = %+v", id, job.Forecast)
		}
		if median == 0 {
			if job.Forecast.EstimatedMinutes != nil {
				t.Fatalf("expected unknown: %+v", job.Forecast)
			}
		} else if job.Forecast.EstimatedMinutes == nil || *job.Forecast.EstimatedMinutes != median {
			t.Fatalf("median %s = %+v", id, job.Forecast)
		}
	}
	assert(targetA, "historical_median", "medium", 20, 3)
	assert(targetB, "historical_median", "low", 40, 2)
	assert(targetC, "historical_median", "high", 150, 10)
	assert(targetD, "unknown", "unknown", 0, 0)
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var residual int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM production_jobs WHERE job_number LIKE $1`, "H-"+suffix+"-%").Scan(&residual); err != nil || residual != 0 {
		t.Fatalf("history fixture cleanup=%d err=%v", residual, err)
	}
}
