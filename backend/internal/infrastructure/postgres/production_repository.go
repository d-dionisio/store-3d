package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type productionDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Begin(context.Context) (pgx.Tx, error)
}

type ProductionRepository struct{ pool productionDB }

func NewProductionRepository(pool *pgxpool.Pool) *ProductionRepository {
	return &ProductionRepository{pool: pool}
}

func productionError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505", "40001", "P0001":
			return fmt.Errorf("%w: %s", app.ErrConflict, pg.Message)
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: %s", app.ErrInvalid, pg.Message)
		}
	}
	return err
}

func (r *ProductionRepository) CreateJobsForPaidOrder(ctx context.Context, reference string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT create_production_jobs_for_paid_order(id) FROM orders WHERE order_number=$1`, reference).Scan(&count)
	return count, productionError(err)
}

func (r *ProductionRepository) ListJobs(ctx context.Context, status string, page, limit int) (app.Page, error) {
	result := app.Page{Page: page, Limit: limit, Items: []app.JobView{}}
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM production_jobs WHERE ($1='' OR status=$1)`, status).Scan(&result.Total)
	if err != nil {
		return result, productionError(err)
	}
	rows, err := r.pool.Query(ctx, `SELECT pj.id,pj.job_number,o.order_number,oi.product_name,pj.status,pj.quantity,pj.priority,pj.version,pj.due_at FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id WHERE ($1='' OR pj.status=$1) ORDER BY pj.priority DESC,pj.due_at NULLS LAST,pj.created_at LIMIT $2 OFFSET $3`, status, limit, (page-1)*limit)
	if err != nil {
		return result, productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.JobView
		if err = rows.Scan(&v.ID, &v.Number, &v.OrderNumber, &v.ProductName, &v.Status, &v.Quantity, &v.Priority, &v.Version, &v.DueAt); err != nil {
			return result, err
		}
		result.Items = append(result.Items, v)
	}
	return result, rows.Err()
}

func scanJob(row pgx.Row) (app.JobView, error) {
	var v app.JobView
	err := row.Scan(&v.ID, &v.Number, &v.OrderNumber, &v.ProductName, &v.Status, &v.Quantity, &v.Priority, &v.Version, &v.DueAt)
	return v, productionError(err)
}
func (r *ProductionRepository) GetJob(ctx context.Context, id string) (app.JobView, error) {
	v, err := scanJob(r.pool.QueryRow(ctx, `SELECT pj.id,pj.job_number,o.order_number,oi.product_name,pj.status,pj.quantity,pj.priority,pj.version,pj.due_at FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id WHERE pj.id=$1`, id))
	if err != nil {
		return v, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,production_job_id,coalesce(printer_id::text,''),status,attempt_number,quantity,version,material_cost_cents,energy_cost_cents,other_cost_cents FROM production_runs WHERE production_job_id=$1 ORDER BY attempt_number`, id)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var run app.RunView
		if err = rows.Scan(&run.ID, &run.JobID, &run.PrinterID, &run.Status, &run.Attempt, &run.Quantity, &run.Version, &run.MaterialCostCents, &run.EnergyCostCents, &run.OtherCostCents); err != nil {
			rows.Close()
			return v, err
		}
		v.Runs = append(v.Runs, run)
	}
	rows.Close()
	events, err := r.pool.Query(ctx, `SELECT id,production_job_id,coalesce(production_run_id::text,''),coalesce(admin_user_id::text,''),coalesce(previous_status,''),coalesce(new_status,''),description,occurred_at FROM production_events WHERE production_job_id=$1 ORDER BY occurred_at`, id)
	if err != nil {
		return v, err
	}
	for events.Next() {
		var e domain.ProductionEvent
		if err = events.Scan(&e.ID, &e.JobID, &e.RunID, &e.AdminUserID, &e.PreviousStatus, &e.NewStatus, &e.Description, &e.OccurredAt); err != nil {
			events.Close()
			return v, err
		}
		v.Events = append(v.Events, e)
	}
	events.Close()
	return v, events.Err()
}

func (r *ProductionRepository) TransitionJob(ctx context.Context, id string, to domain.JobStatus, actor, description string, version int64) (app.JobView, error) {
	_, err := r.pool.Exec(ctx, `SELECT transition_production_job($1,$2,$3,$4,$5)`, id, string(to), actor, description, version)
	if err != nil {
		return app.JobView{}, productionError(err)
	}
	return r.GetJob(ctx, id)
}

func scanRun(row pgx.Row) (app.RunView, error) {
	var v app.RunView
	err := row.Scan(&v.ID, &v.JobID, &v.PrinterID, &v.Status, &v.Attempt, &v.Quantity, &v.Version, &v.MaterialCostCents, &v.EnergyCostCents, &v.OtherCostCents)
	return v, productionError(err)
}

const runSelect = `SELECT id,production_job_id,coalesce(printer_id::text,''),status,attempt_number,quantity,version,material_cost_cents,energy_cost_cents,other_cost_cents FROM production_runs WHERE id=$1`

func (r *ProductionRepository) StartPreparation(ctx context.Context, jobID, printerID, actor string, jobVersion int64) (app.JobView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.JobView{}, err
	}
	defer tx.Rollback(ctx)
	var runID string
	var runVersion int64
	err = tx.QueryRow(ctx, `SELECT id,version FROM production_runs WHERE production_job_id=$1 ORDER BY attempt_number DESC LIMIT 1 FOR UPDATE`, jobID).Scan(&runID, &runVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,printer_id,attempt_number,quantity,created_by_admin_user_id) SELECT id,$2,1,quantity,$3 FROM production_jobs WHERE id=$1 RETURNING id,version`, jobID, printerID, actor).Scan(&runID, &runVersion)
	} else if err == nil {
		_, err = tx.Exec(ctx, `UPDATE production_runs SET printer_id=$2,updated_at=now() WHERE id=$1 AND status='queued'`, runID, printerID)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT transition_production_job($1,'preparing',$2,'Preparação iniciada.',$3)`, jobID, actor, jobVersion)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT transition_production_run($1,'preparing',$2,'Preparação iniciada.',$3)`, runID, actor, runVersion)
	}
	if err != nil {
		return app.JobView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.JobView{}, err
	}
	return r.GetJob(ctx, jobID)
}

func (r *ProductionRepository) TransitionRun(ctx context.Context, id string, to domain.RunStatus, actor, description string, version int64) (app.RunView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.RunView{}, err
	}
	defer tx.Rollback(ctx)
	var jobID, jobStatus string
	var jobVersion int64
	err = tx.QueryRow(ctx, `SELECT pj.id,pj.status,pj.version FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id WHERE pr.id=$1 FOR UPDATE OF pr,pj`, id).Scan(&jobID, &jobStatus, &jobVersion)
	if err == nil && to == domain.RunFailed {
		_, err = tx.Exec(ctx, `UPDATE production_runs SET failure_reason=$2 WHERE id=$1`, id, description)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT transition_production_run($1,$2,$3,$4,$5)`, id, string(to), actor, description, version)
	}
	jobTarget := ""
	switch to {
	case domain.RunPrinting:
		jobTarget = "printing"
	case domain.RunQualityCheck:
		jobTarget = "quality_check"
	case domain.RunFailed:
		jobTarget = "failed"
	}
	if err == nil && jobTarget != "" && jobStatus != jobTarget {
		if to == domain.RunFailed {
			_, err = tx.Exec(ctx, `UPDATE production_jobs SET failure_reason=$2 WHERE id=$1`, jobID, description)
		}
	}
	if err == nil && jobTarget != "" && jobStatus != jobTarget {
		_, err = tx.Exec(ctx, `SELECT transition_production_job($1,$2,$3,$4,$5)`, jobID, jobTarget, actor, description, jobVersion)
	}
	if err != nil {
		return app.RunView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.RunView{}, err
	}
	return scanRun(r.pool.QueryRow(ctx, runSelect, id))
}

func (r *ProductionRepository) CreateReprint(ctx context.Context, sourceID, printerID, actor string) (app.RunView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.RunView{}, err
	}
	defer tx.Rollback(ctx)
	var jobID string
	var attempt, quantity int
	var jobStatus string
	var jobVersion int64
	err = tx.QueryRow(ctx, `SELECT pr.production_job_id,pr.attempt_number,pr.quantity,pj.status,pj.version FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id WHERE pr.id=$1 FOR UPDATE OF pr,pj`, sourceID).Scan(&jobID, &attempt, &quantity, &jobStatus, &jobVersion)
	var id string
	if err == nil {
		err = tx.QueryRow(ctx, `INSERT INTO production_runs(production_job_id,reprint_of_run_id,printer_id,attempt_number,quantity,created_by_admin_user_id) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, jobID, sourceID, printerID, attempt+1, quantity, actor).Scan(&id)
	}
	if err == nil && jobStatus == "quality_check" {
		_, err = tx.Exec(ctx, `SELECT transition_production_job($1,'queued',$2,'Reimpressão criada.',$3)`, jobID, actor, jobVersion)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO production_events(production_job_id,production_run_id,event_type,admin_user_id,description,metadata) VALUES($1,$2,'reprint_created',$3,'Reimpressão criada.',jsonb_build_object('source_run_id',$4::text))`, jobID, id, actor, sourceID)
	}
	if err != nil {
		return app.RunView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.RunView{}, err
	}
	return scanRun(r.pool.QueryRow(ctx, runSelect, id))
}

func scanPrinter(row pgx.Row) (app.PrinterView, error) {
	var v app.PrinterView
	err := row.Scan(&v.ID, &v.Code, &v.Name, &v.Manufacturer, &v.Model, &v.Status, &v.BuildWidthMM, &v.BuildDepthMM, &v.BuildHeightMM, &v.NozzleMicrons, &v.UpdatedAt)
	return v, productionError(err)
}

const printerReturning = `id,code,name,manufacturer,model,status,build_width_mm,build_depth_mm,build_height_mm,nozzle_diameter_microns,updated_at`

func (r *ProductionRepository) CreatePrinter(ctx context.Context, in app.PrinterInput) (app.PrinterView, error) {
	return scanPrinter(r.pool.QueryRow(ctx, `INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm,nozzle_diameter_microns) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING `+printerReturning, in.Code, in.Name, in.Manufacturer, in.Model, in.BuildWidthMM, in.BuildDepthMM, in.BuildHeightMM, in.NozzleMicrons))
}
func (r *ProductionRepository) UpdatePrinter(ctx context.Context, id string, in app.PrinterInput) (app.PrinterView, error) {
	return scanPrinter(r.pool.QueryRow(ctx, `UPDATE printers SET code=$2,name=$3,manufacturer=$4,model=$5,build_width_mm=$6,build_depth_mm=$7,build_height_mm=$8,nozzle_diameter_microns=$9,updated_at=now() WHERE id=$1 RETURNING `+printerReturning, id, in.Code, in.Name, in.Manufacturer, in.Model, in.BuildWidthMM, in.BuildDepthMM, in.BuildHeightMM, in.NozzleMicrons))
}
func (r *ProductionRepository) ListPrinters(ctx context.Context) ([]app.PrinterView, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+printerReturning+` FROM printers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.PrinterView{}
	for rows.Next() {
		v, e := scanPrinter(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) ChangePrinterStatus(ctx context.Context, id, status string) (app.PrinterView, error) {
	retired := any(nil)
	if status == "retired" {
		retired = time.Now()
	}
	return scanPrinter(r.pool.QueryRow(ctx, `UPDATE printers SET status=$2,retired_at=$3,updated_at=now() WHERE id=$1 RETURNING `+printerReturning, id, status, retired))
}
func (r *ProductionRepository) RegisterMaintenance(ctx context.Context, printer, actor string, in app.MaintenanceInput) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, `SELECT id FROM printers WHERE id=$1 FOR UPDATE`, printer).Scan(&printer); err != nil {
		return "", productionError(err)
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO printer_maintenance_events(printer_id,event_type,description,performed_by_admin_user_id,scheduled_at,cost_cents) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, printer, in.EventType, in.Description, actor, in.ScheduledAt, in.CostCents).Scan(&id)
	if err != nil {
		return "", productionError(err)
	}
	return id, tx.Commit(ctx)
}

func scanSpool(row pgx.Row) (app.SpoolView, error) {
	var v app.SpoolView
	err := row.Scan(&v.ID, &v.Code, &v.MaterialID, &v.ColorID, &v.Manufacturer, &v.Status, &v.InitialWeightGrams, &v.CurrentWeightGrams, &v.CostPerKGCents, &v.Version, &v.UpdatedAt)
	return v, productionError(err)
}

const spoolReturning = `id,spool_code,material_id,color_id,manufacturer,status,initial_weight_grams,current_weight_grams,cost_per_kg_cents,version,updated_at`

func (r *ProductionRepository) RegisterSpool(ctx context.Context, in app.SpoolInput) (app.SpoolView, error) {
	status := "available"
	if in.CurrentWeightGrams == 0 {
		status = "depleted"
	}
	return scanSpool(r.pool.QueryRow(ctx, `INSERT INTO material_spools(spool_code,material_id,color_id,manufacturer,product_line,lot_number,status,initial_weight_grams,current_weight_grams,minimum_stock_grams,cost_per_kg_cents) VALUES($1,$2,$3,$4,nullif($5,''),nullif($6,''),$7,$8,$9,$10,$11) RETURNING `+spoolReturning, in.Code, in.MaterialID, in.ColorID, in.Manufacturer, in.ProductLine, in.LotNumber, status, in.InitialWeightGrams, in.CurrentWeightGrams, in.MinimumStockGrams, in.CostPerKGCents))
}
func (r *ProductionRepository) UpdateSpool(ctx context.Context, id string, in app.SpoolInput, version int64) (app.SpoolView, error) {
	v, err := scanSpool(r.pool.QueryRow(ctx, `UPDATE material_spools SET manufacturer=$2,product_line=nullif($3,''),lot_number=nullif($4,''),minimum_stock_grams=$5,cost_per_kg_cents=$6,version=version+1,updated_at=now() WHERE id=$1 AND version=$7 RETURNING `+spoolReturning, id, in.Manufacturer, in.ProductLine, in.LotNumber, in.MinimumStockGrams, in.CostPerKGCents, version))
	if errors.Is(err, app.ErrNotFound) {
		return v, app.ErrConflict
	}
	return v, err
}
func (r *ProductionRepository) ListSpools(ctx context.Context, status string) ([]app.SpoolView, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+spoolReturning+` FROM material_spools WHERE ($1='' OR status=$1) ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.SpoolView{}
	for rows.Next() {
		v, e := scanSpool(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ProductionRepository) GetSpool(ctx context.Context, id string) (app.SpoolView, error) {
	return scanSpool(r.pool.QueryRow(ctx, `SELECT `+spoolReturning+` FROM material_spools WHERE id=$1`, id))
}
func (r *ProductionRepository) AdjustSpool(ctx context.Context, id string, delta int, reason, actor string, version int64) (app.SpoolView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.SpoolView{}, err
	}
	defer tx.Rollback(ctx)
	var old int
	var currentVersion int64
	err = tx.QueryRow(ctx, `SELECT current_weight_grams,version FROM material_spools WHERE id=$1 FOR UPDATE`, id).Scan(&old, &currentVersion)
	next := old + delta
	if err == nil && (version != currentVersion || next < 0) {
		err = app.ErrConflict
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO material_spool_adjustments(material_spool_id,delta_grams,previous_balance_grams,new_balance_grams,reason,admin_user_id) VALUES($1,$2,$3,$4,$5,$6)`, id, delta, old, next, reason, actor)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE material_spools SET current_weight_grams=$2,status=CASE WHEN $2=0 THEN 'depleted' ELSE 'available' END,exhausted_at=CASE WHEN $2=0 THEN now() ELSE NULL END,version=version+1,updated_at=now() WHERE id=$1`, id, next)
	}
	if err != nil {
		return app.SpoolView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.SpoolView{}, err
	}
	return r.GetSpool(ctx, id)
}
func (r *ProductionRepository) ConsumeMaterial(ctx context.Context, run, spool string, consumed, wasted int, actor string) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO production_run_materials(production_run_id,material_spool_id,consumed_grams,wasted_grams,cost_per_kg_snapshot_cents,total_cost_cents,recorded_by_admin_user_id) VALUES($1,$2,$3,$4,0,0,$5)`, run, spool, consumed, wasted, actor)
	return productionError(err)
}
func (r *ProductionRepository) QuarantineSpool(ctx context.Context, id, reason string, version int64) (app.SpoolView, error) {
	if strings.TrimSpace(reason) == "" {
		return app.SpoolView{}, app.ErrInvalid
	}
	v, err := scanSpool(r.pool.QueryRow(ctx, `UPDATE material_spools SET status='quarantined',version=version+1,updated_at=now() WHERE id=$1 AND version=$2 RETURNING `+spoolReturning, id, version))
	if errors.Is(err, app.ErrNotFound) {
		return v, app.ErrConflict
	}
	return v, err
}

func (r *ProductionRepository) Inspect(ctx context.Context, run, actor string, in app.InspectionInput) (string, error) {
	q := domain.QualityInspection{ID: "new", RunID: run, InspectorAdminID: actor, Result: in.Result, DimensionalAccuracyOK: in.DimensionalAccuracyOK, VisualQualityOK: in.VisualQualityOK, FunctionalTestOK: in.FunctionalTestOK, Notes: in.Notes, InspectedAt: time.Now()}
	if err := q.Validate(); err != nil {
		return "", app.ErrInvalid
	}
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO quality_inspections(production_run_id,result,dimensional_accuracy_ok,visual_quality_ok,functional_test_ok,notes,inspected_by_admin_user_id) VALUES($1,$2,$3,$4,$5,nullif($6,''),$7) RETURNING id`, run, string(in.Result), in.DimensionalAccuracyOK, in.VisualQualityOK, in.FunctionalTestOK, in.Notes, actor).Scan(&id)
	return id, productionError(err)
}
func (r *ProductionRepository) ApproveRun(ctx context.Context, run, actor, description string, in app.InspectionInput, version int64) (app.RunView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.RunView{}, err
	}
	defer tx.Rollback(ctx)
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO quality_inspections(production_run_id,result,dimensional_accuracy_ok,visual_quality_ok,functional_test_ok,notes,inspected_by_admin_user_id) VALUES($1,'approved',$2,$3,$4,nullif($5,''),$6) RETURNING id`, run, in.DimensionalAccuracyOK, in.VisualQualityOK, in.FunctionalTestOK, in.Notes, actor).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT transition_production_run($1,'completed',$2,$3,$4)`, run, actor, description, version)
	}
	if err != nil {
		return app.RunView{}, productionError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.RunView{}, err
	}
	return scanRun(r.pool.QueryRow(ctx, runSelect, run))
}
func (r *ProductionRepository) RejectRun(ctx context.Context, run, actor string, in app.InspectionInput) (string, error) {
	in.Result = domain.InspectionRejected
	return r.Inspect(ctx, run, actor, in)
}
