package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	productionapp "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	productiondomain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestProductionLifecycleIntegration homologates the application service and
// repository against a dedicated, migrated PostgreSQL database. It never uses
// development orders as fixtures.
func TestProductionLifecycleIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") || strings.EqualFold(config.ConnConfig.Database, "loja_3d") {
		t.Fatalf("unsafe test database %q", config.ConnConfig.Database)
	}
	if development := strings.TrimSpace(os.Getenv("DATABASE_URL")); development != "" && development == dsn {
		t.Fatal("TEST_DATABASE_URL must differ from DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var materialID, colorID, categoryID, productID, variantID, userID, orderID, adminID string
	mustScan := func(query string, args []any, destination ...any) {
		t.Helper()
		if scanErr := pool.QueryRow(ctx, query, args...).Scan(destination...); scanErr != nil {
			t.Fatal(scanErr)
		}
	}
	mustScan(`INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id`, []any{"life-mat-" + suffix, "PLA Homologação " + suffix}, &materialID)
	mustScan(`INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#123456') RETURNING id`, []any{"life-color-" + suffix, "Cor Homologação " + suffix}, &colorID)
	mustScan(`INSERT INTO categories(name,slug) VALUES($1,$2) RETURNING id`, []any{"Categoria Homologação " + suffix, "life-category-" + suffix}, &categoryID)
	// Keep production fixtures outside public catalog queries while preserving
	// the real product/variant relationships required by order snapshots.
	mustScan(`INSERT INTO products(category_id,slug,name,description,status) VALUES($1,$2,$3,'Fixture isolada','draft') RETURNING id`, []any{categoryID, "life-product-" + suffix, "Produto Homologação " + suffix}, &productID)
	mustScan(`INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity) VALUES($1,$2,$3,$4,1000,20) RETURNING id`, []any{productID, materialID, colorID, "LIFE-SKU-" + suffix}, &variantID)
	mustScan(`INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Cliente Homologação',now()) RETURNING id`, []any{"life-" + suffix + "@example.test"}, &userID)
	mustScan(`INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,subtotal_cents,total_cents,payment_readiness) VALUES($1,$2,'confirmed','Cliente Homologação',$3,4000,4000,'ready') RETURNING id`, []any{"PED-LIFE-" + suffix, userID, "life-" + suffix + "@example.test"}, &orderID)
	for index := 1; index <= 4; index++ {
		if _, err = pool.Exec(ctx, `INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents) VALUES($1,'catalog',$2,$3,$4,'Snapshot',$5,'PLA','Azul',1,1000)`, orderID, productID, variantID, fmt.Sprintf("Produto %d", index), fmt.Sprintf("%s-%d", "LIFE-SKU-"+suffix, index)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,paid_amount_cents,refunded_amount_cents,currency,paid_at) VALUES($1,'pix','paid',4000,4000,0,'BRL',now())`, orderID); err != nil {
		t.Fatal(err)
	}
	mustScan(`INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Gerente Homologação',repeat('x',60),'production_manager') RETURNING id`, []any{"manager-life-" + suffix + "@example.test"}, &adminID)

	manager := admindomain.Session{UserID: adminID, Role: "production_manager"}
	operator := admindomain.Session{UserID: adminID, Role: "production_operator"}
	service := productionapp.New(NewProductionRepository(pool))
	orderNumber := "PED-LIFE-" + suffix
	created, err := service.CreateProductionJobsForPaidOrder(ctx, manager, orderNumber)
	if err != nil || created != 4 {
		t.Fatalf("create jobs: count=%d err=%v", created, err)
	}
	created, err = service.CreateProductionJobsForPaidOrder(ctx, manager, orderNumber)
	if err != nil || created != 0 {
		t.Fatalf("idempotent creation: count=%d err=%v", created, err)
	}
	page, err := service.ListProductionQueue(ctx, manager, "awaiting_production", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	jobs := make([]productionapp.JobView, 0, 4)
	for _, job := range page.Items {
		if job.OrderNumber == orderNumber {
			jobs = append(jobs, job)
		}
	}
	if len(jobs) != 4 {
		t.Fatalf("fixture jobs=%d, want 4", len(jobs))
	}

	printer, err := service.CreatePrinter(ctx, manager, productionapp.PrinterInput{Code: "LIFE-PRINTER-" + suffix, Name: "A1 Homologação", Manufacturer: "Bambu Lab", Model: "A1", BuildWidthMM: 256, BuildDepthMM: 256, BuildHeightMM: 256, NozzleMicrons: 400})
	if err != nil {
		t.Fatal(err)
	}
	printer, err = service.ChangePrinterStatus(ctx, manager, printer.ID, "maintenance")
	if err != nil || printer.Status != "maintenance" {
		t.Fatalf("printer maintenance: %+v %v", printer, err)
	}
	if _, err = service.RegisterPrinterMaintenance(ctx, manager, printer.ID, productionapp.MaintenanceInput{EventType: "preventive", Description: "Homologação preventiva"}); err != nil {
		t.Fatal(err)
	}
	printer, err = service.ChangePrinterStatus(ctx, manager, printer.ID, "available")
	if err != nil {
		t.Fatal(err)
	}
	spool, err := service.RegisterMaterialSpool(ctx, manager, productionapp.SpoolInput{Code: "LIFE-SPOOL-" + suffix, MaterialID: materialID, ColorID: colorID, Manufacturer: "Homologação", ProductLine: "Test", LotNumber: suffix, InitialWeightGrams: 1000, CurrentWeightGrams: 1000, MinimumStockGrams: 100, CostPerKGCents: 12000})
	if err != nil {
		t.Fatal(err)
	}
	spool, err = service.AdjustSpoolBalance(ctx, manager, spool.ID, -10, "Calibração de balança", spool.Version)
	if err != nil || spool.CurrentWeightGrams != 990 {
		t.Fatalf("spool adjustment: %+v %v", spool, err)
	}
	if _, err = service.AdjustSpoolBalance(ctx, manager, spool.ID, -2000, "Saldo inválido", spool.Version); err == nil {
		t.Fatal("insufficient spool adjustment accepted")
	}

	// Happy path: paid order -> queue -> preparation -> printing -> quality -> completion.
	job, err := service.QueueProductionJob(ctx, manager, jobs[0].ID, "Fila homologada", jobs[0].Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.QueueProductionJob(ctx, manager, jobs[0].ID, "Versão obsoleta", jobs[0].Version); err == nil {
		t.Fatal("stale job version accepted")
	}
	job, err = service.StartPreparation(ctx, operator, job.ID, printer.ID, job.Version)
	if err != nil || len(job.Runs) != 1 {
		t.Fatalf("preparation: %+v %v", job, err)
	}
	run := job.Runs[0]
	run, err = service.StartProductionRun(ctx, operator, run.ID, run.Version)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RecordMaterialConsumption(ctx, operator, run.ID, spool.ID, 100, 10); err != nil {
		t.Fatal(err)
	}
	if err = service.RecordMaterialConsumption(ctx, operator, run.ID, spool.ID, 2000, 0); err == nil {
		t.Fatal("insufficient production consumption accepted")
	}
	run, err = service.FinishProductionRun(ctx, operator, run.ID, run.Version)
	if err != nil {
		t.Fatal(err)
	}
	functional := true
	inspection := productionapp.InspectionInput{Result: productiondomain.InspectionConditional, DimensionalAccuracyOK: true, VisualQualityOK: true, FunctionalTestOK: &functional, Notes: "Inspeção homologada"}
	if _, err = service.CreateQualityInspection(ctx, operator, run.ID, inspection); err != nil {
		t.Fatal(err)
	}
	run, err = service.ApproveProductionRun(ctx, manager, run.ID, inspection, run.Version)
	if err != nil || run.Status != string(productiondomain.RunCompleted) {
		t.Fatalf("approve run: %+v %v", run, err)
	}
	job, err = service.GetProductionJob(ctx, manager, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	job, err = service.CompleteProductionJob(ctx, manager, job.ID, job.Version)
	if err != nil || job.Status != string(productiondomain.JobCompleted) {
		t.Fatalf("complete job: %+v %v", job, err)
	}
	var snapshot, totalCost int64
	if err = pool.QueryRow(ctx, `SELECT cost_per_kg_snapshot_cents,total_cost_cents FROM production_run_materials WHERE production_run_id=$1`, run.ID).Scan(&snapshot, &totalCost); err != nil || snapshot != 12000 || totalCost != 1320 {
		t.Fatalf("cost snapshot=%d total=%d err=%v", snapshot, totalCost, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE production_events SET description='tampered' WHERE production_job_id=$1`, job.ID); err == nil {
		t.Fatal("append-only event update accepted")
	}

	// Print failure and reprint.
	failedJob, err := service.QueueProductionJob(ctx, manager, jobs[1].ID, "Fila para falha", jobs[1].Version)
	if err != nil {
		t.Fatal(err)
	}
	failedJob, err = service.StartPreparation(ctx, operator, failedJob.ID, printer.ID, failedJob.Version)
	if err != nil {
		t.Fatal(err)
	}
	failedRun, err := service.StartProductionRun(ctx, operator, failedJob.Runs[0].ID, failedJob.Runs[0].Version)
	if err != nil {
		t.Fatal(err)
	}
	failedRun, err = service.FailProductionRun(ctx, operator, failedRun.ID, "Falha controlada", failedRun.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateReprint(ctx, manager, failedRun.ID, printer.ID); err != nil {
		t.Fatal(err)
	}

	// Quality rejection permits a reprint and returns the job to queue.
	rejectedJob, err := service.QueueProductionJob(ctx, manager, jobs[2].ID, "Fila para reprovação", jobs[2].Version)
	if err != nil {
		t.Fatal(err)
	}
	rejectedJob, err = service.StartPreparation(ctx, operator, rejectedJob.ID, printer.ID, rejectedJob.Version)
	if err != nil {
		t.Fatal(err)
	}
	rejectedRun, err := service.StartProductionRun(ctx, operator, rejectedJob.Runs[0].ID, rejectedJob.Runs[0].Version)
	if err != nil {
		t.Fatal(err)
	}
	rejectedRun, err = service.FinishProductionRun(ctx, operator, rejectedRun.ID, rejectedRun.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RejectProductionRun(ctx, manager, rejectedRun.ID, productionapp.InspectionInput{Notes: "Reprovado na homologação"}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateReprint(ctx, manager, rejectedRun.ID, printer.ID); err != nil {
		t.Fatal(err)
	}

	// Cancellation and invalid transition.
	if _, err = service.CompleteProductionJob(ctx, manager, jobs[3].ID, jobs[3].Version); err == nil {
		t.Fatal("invalid completion accepted")
	}
	cancelled, err := service.CancelProductionJob(ctx, manager, jobs[3].ID, "Cancelamento homologado", jobs[3].Version)
	if err != nil || cancelled.Status != string(productiondomain.JobCancelled) {
		t.Fatalf("cancel job: %+v %v", cancelled, err)
	}

	quarantined, err := service.QuarantineMaterialSpool(ctx, manager, spool.ID, "Quarentena homologada", spool.Version+1)
	if err != nil || quarantined.Status != "quarantined" {
		t.Fatalf("quarantine: %+v %v", quarantined, err)
	}
	if _, err = service.AdjustSpoolBalance(ctx, manager, spool.ID, 1, "Versão antiga", spool.Version); !errors.Is(err, productionapp.ErrConflict) {
		t.Fatalf("expected stale spool conflict, got %v", err)
	}
}
