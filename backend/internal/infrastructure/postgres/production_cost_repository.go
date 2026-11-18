package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductionCostRepository struct{ pool productionDB }

func NewProductionCostRepository(pool *pgxpool.Pool) *ProductionCostRepository {
	return &ProductionCostRepository{pool: pool}
}

func costRepositoryError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505", "40001", "P0001":
			return fmt.Errorf("%w: cost configuration or snapshot conflict", app.ErrConflict)
		case "23503", "23514", "22P02":
			return fmt.Errorf("%w: invalid cost reference or value", app.ErrInvalid)
		}
	}
	return err
}

const costProfileSelect = `SELECT p.id::text,p.printer_id::text,p.version,p.electricity_rate_cents_per_kwh,p.printer_power_milliwatts,coalesce(p.power_measurement_note,''),p.machine_acquisition_cents,p.machine_useful_life_seconds,p.maintenance_rate_cents_per_hour,p.labor_rate_cents_per_hour,p.preparation_seconds,p.finishing_seconds,coalesce(p.notes,''),p.created_at FROM production_cost_profiles p `

func scanCostProfile(row pgx.Row) (app.CostProfileView, error) {
	var v app.CostProfileView
	err := row.Scan(&v.ID, &v.PrinterID, &v.Version, &v.ElectricityRateCentsPerKWh, &v.PrinterPowerMilliwatts, &v.PowerMeasurementNote, &v.MachineAcquisitionCents, &v.MachineUsefulLifeSeconds, &v.MaintenanceRateCentsPerHour, &v.LaborRateCentsPerHour, &v.PreparationSeconds, &v.FinishingSeconds, &v.Notes, &v.CreatedAt)
	return v, costRepositoryError(err)
}

func (r *ProductionCostRepository) SaveCostProfile(ctx context.Context, printerID string, in app.CostProfileInput, actor string) (app.CostProfileView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CostProfileView{}, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT true FROM printers WHERE id=$1 FOR UPDATE`, printerID).Scan(&exists); err != nil {
		return app.CostProfileView{}, costRepositoryError(err)
	}
	var current int64
	err = tx.QueryRow(ctx, `SELECT p.version FROM production_cost_profile_current c JOIN production_cost_profiles p ON p.id=c.profile_id WHERE c.printer_id=$1 FOR UPDATE OF c`, printerID).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		current = 0
	} else if err != nil {
		return app.CostProfileView{}, costRepositoryError(err)
	}
	if current != in.ExpectedVersion {
		return app.CostProfileView{}, app.ErrConflict
	}
	version := current + 1
	var profileID string
	err = tx.QueryRow(ctx, `INSERT INTO production_cost_profiles(printer_id,version,electricity_rate_cents_per_kwh,printer_power_milliwatts,power_measurement_note,machine_acquisition_cents,machine_useful_life_seconds,maintenance_rate_cents_per_hour,labor_rate_cents_per_hour,preparation_seconds,finishing_seconds,notes,created_by_admin_user_id) VALUES($1,$2,$3,$4,nullif($5,''),$6,$7,$8,$9,$10,$11,nullif($12,''),$13) RETURNING id::text`, printerID, version, in.ElectricityRateCentsPerKWh, in.PrinterPowerMilliwatts, in.PowerMeasurementNote, in.MachineAcquisitionCents, in.MachineUsefulLifeSeconds, in.MaintenanceRateCentsPerHour, in.LaborRateCentsPerHour, in.PreparationSeconds, in.FinishingSeconds, in.Notes, actor).Scan(&profileID)
	if err != nil {
		return app.CostProfileView{}, costRepositoryError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO production_cost_profile_current(printer_id,profile_id,updated_at) VALUES($1,$2,now()) ON CONFLICT(printer_id) DO UPDATE SET profile_id=excluded.profile_id,updated_at=now()`, printerID, profileID)
	if err != nil {
		return app.CostProfileView{}, costRepositoryError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.CostProfileView{}, costRepositoryError(err)
	}
	return r.GetCurrentCostProfile(ctx, printerID)
}

func (r *ProductionCostRepository) GetCurrentCostProfile(ctx context.Context, printerID string) (app.CostProfileView, error) {
	return scanCostProfile(r.pool.QueryRow(ctx, costProfileSelect+` JOIN production_cost_profile_current c ON c.profile_id=p.id WHERE p.printer_id=$1`, printerID))
}
func (r *ProductionCostRepository) ListCostProfileHistory(ctx context.Context, printerID string) ([]app.CostProfileView, error) {
	rows, err := r.pool.Query(ctx, costProfileSelect+` WHERE p.printer_id=$1 ORDER BY p.version DESC`, printerID)
	if err != nil {
		return nil, costRepositoryError(err)
	}
	defer rows.Close()
	items := []app.CostProfileView{}
	for rows.Next() {
		v, e := scanCostProfile(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *ProductionCostRepository) LoadRunCostContext(ctx context.Context, runID, kind string, measuredEnergy *int64) (app.CostContext, error) {
	var c app.CostContext
	var status string
	var estimatedMinutes, actualMinutes, estimatedWeight, actualWeight *int64
	var started, completed *time.Time
	var acquisition, life, maint, labor, rate, prep, finish int64
	var power *int64
	var profileNote *string
	err := r.pool.QueryRow(ctx, `SELECT pr.id::text,coalesce(pr.printer_id::text,''),pr.version,pr.status,pr.quantity,pr.estimated_minutes,pr.actual_minutes,pr.estimated_weight_grams,pr.actual_weight_grams,pr.started_at,pr.completed_at,CASE WHEN $2='estimate' OR (pr.status='completed' AND NOT EXISTS (SELECT 1 FROM production_runs earlier WHERE earlier.production_job_id=pr.production_job_id AND earlier.status='completed' AND (earlier.completed_at,earlier.id)<(pr.completed_at,pr.id))) THEN CASE WHEN oi.item_type='custom' THEN round(oi.approved_total_cents::numeric*pr.quantity/oi.quantity)::bigint ELSE oi.unit_price_cents*pr.quantity END ELSE 0 END,coalesce(c.profile_id::text,''),coalesce(p.version,0),coalesce(p.electricity_rate_cents_per_kwh,0),p.printer_power_milliwatts,p.power_measurement_note,coalesce(p.machine_acquisition_cents,0),coalesce(p.machine_useful_life_seconds,0),coalesce(p.maintenance_rate_cents_per_hour,0),coalesce(p.labor_rate_cents_per_hour,0),coalesce(p.preparation_seconds,0),coalesce(p.finishing_seconds,0) FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id LEFT JOIN production_cost_profile_current c ON c.printer_id=pr.printer_id LEFT JOIN production_cost_profiles p ON p.id=c.profile_id WHERE pr.id=$1`, runID, kind).Scan(&c.RunID, &c.PrinterID, &c.RunVersion, &status, &c.Quantity, &estimatedMinutes, &actualMinutes, &estimatedWeight, &actualWeight, &started, &completed, &c.Input.RevenueCents, &c.ProfileID, &c.Profile.Version, &rate, &power, &profileNote, &acquisition, &life, &maint, &labor, &prep, &finish)
	if err != nil {
		return c, costRepositoryError(err)
	}
	if c.PrinterID == "" || c.ProfileID == "" {
		return c, app.ErrCostDataUnavailable
	}
	c.Profile.PrinterID, c.Profile.ElectricityRateCentsPerKWh, c.Profile.PrinterPowerMilliwatts = c.PrinterID, rate, power
	c.Profile.MachineAcquisitionCents, c.Profile.MachineUsefulLifeSeconds, c.Profile.MaintenanceRateCentsPerHour = acquisition, life, maint
	c.Profile.LaborRateCentsPerHour, c.Profile.PreparationSeconds, c.Profile.FinishingSeconds = labor, prep, finish
	if profileNote != nil {
		c.Profile.PowerMeasurementNote = *profileNote
	}
	c.SnapshotType = kind
	c.EnergySource = "configured_power"
	var duration int64
	if kind == "estimate" {
		if status == "completed" || status == "failed" || status == "cancelled" || estimatedMinutes == nil || estimatedWeight == nil || *estimatedWeight <= 0 || power == nil {
			return c, app.ErrCostDataUnavailable
		}
		duration = *estimatedMinutes * 60
		var materialID, colorID string
		if err = r.pool.QueryRow(ctx, `SELECT material_id::text,color_id::text FROM production_jobs WHERE id=(SELECT production_job_id FROM production_runs WHERE id=$1)`, runID).Scan(&materialID, &colorID); err != nil {
			return c, costRepositoryError(err)
		}
		var costPerKg int64
		var estimateSpoolID, estimateSpoolCode string
		err = r.pool.QueryRow(ctx, `SELECT id::text,spool_code,cost_per_kg_cents FROM material_spools WHERE material_id=$1 AND color_id=$2 AND status NOT IN ('discarded','quarantined') ORDER BY created_at DESC,id DESC LIMIT 1`, materialID, colorID).Scan(&estimateSpoolID, &estimateSpoolCode, &costPerKg)
		if err != nil {
			return c, app.ErrCostDataUnavailable
		}
		c.Input.Materials = []domain.MaterialCostInput{{ConsumedGrams: *estimatedWeight, CostPerKGCents: costPerKg}}
		c.ConsumedGrams = *estimatedWeight
		c.MaterialBreakdown, _ = json.Marshal([]map[string]any{{"spool_id": estimateSpoolID, "spool_code": estimateSpoolCode, "estimated_grams": *estimatedWeight, "cost_per_kg_snapshot_cents": costPerKg, "price_source": "latest matching spool at estimate time"}})
	} else if kind == "actual" {
		if (status != "completed" && status != "failed" && status != "cancelled") || (started == nil && actualMinutes == nil) || (status == "completed" && completed == nil) {
			return c, app.ErrCostDataUnavailable
		}
		switch {
		case started != nil && completed != nil:
			duration = int64(completed.Sub(*started).Seconds())
		case actualMinutes != nil:
			duration = *actualMinutes * 60
		default:
			return c, app.ErrCostDataUnavailable
		}
		rows, e := r.pool.Query(ctx, `SELECT rm.consumed_grams,rm.wasted_grams,rm.cost_per_kg_snapshot_cents,rm.total_cost_cents,rm.material_spool_id::text,ms.spool_code FROM production_run_materials rm JOIN material_spools ms ON ms.id=rm.material_spool_id WHERE rm.production_run_id=$1 ORDER BY rm.created_at,rm.id`, runID)
		if e != nil {
			return c, costRepositoryError(e)
		}
		materials := []map[string]any{}
		for rows.Next() {
			var item domain.MaterialCostInput
			var total int64
			var spool, spoolCode string
			if e = rows.Scan(&item.ConsumedGrams, &item.WastedGrams, &item.CostPerKGCents, &total, &spool, &spoolCode); e != nil {
				rows.Close()
				return c, e
			}
			c.Input.Materials = append(c.Input.Materials, item)
			c.ConsumedGrams += item.ConsumedGrams
			c.WastedGrams += item.WastedGrams
			materials = append(materials, map[string]any{"spool_id": spool, "spool_code": spoolCode, "consumed_grams": item.ConsumedGrams, "wasted_grams": item.WastedGrams, "cost_per_kg_snapshot_cents": item.CostPerKGCents, "total_cost_snapshot_cents": total, "price_source": "production_run_materials immutable snapshot"})
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return c, e
		}
		if len(c.Input.Materials) == 0 {
			return c, app.ErrCostDataUnavailable
		}
		c.MaterialBreakdown, _ = json.Marshal(materials)
	} else {
		return c, app.ErrInvalid
	}
	if measuredEnergy != nil {
		c.Input.EnergyMilliWh = *measuredEnergy
		c.EnergySource = "measured"
	} else {
		if power == nil {
			return c, app.ErrCostDataUnavailable
		}
		energy, e := domain.EnergyMilliWhFromPower(*power, duration)
		if e != nil {
			return c, app.ErrInvalid
		}
		c.Input.EnergyMilliWh = energy
	}
	c.Input.EnergyRateCentsKWh = rate
	c.Input.MachineAcquisitionCents = acquisition
	c.Input.MachineUsefulLifeSeconds = life
	c.Input.MaintenanceRateCentsHour = maint
	c.Input.LaborRateCentsHour = labor
	c.Input.RunSeconds = duration
	c.Input.PreparationSeconds = prep
	c.Input.FinishingSeconds = finish
	if kind == "estimate" {
		c.Input.RunSeconds = duration
	}
	return c, nil
}

func (r *ProductionCostRepository) SaveRunCostSnapshot(ctx context.Context, c app.CostContext, b domain.CostBreakdown, actor string) (app.CostSnapshotView, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.CostSnapshotView{}, err
	}
	defer tx.Rollback(ctx)
	var version int64
	var status string
	if err = tx.QueryRow(ctx, `SELECT version,status FROM production_runs WHERE id=$1 FOR UPDATE`, c.RunID).Scan(&version, &status); err != nil {
		return app.CostSnapshotView{}, costRepositoryError(err)
	}
	if version != c.RunVersion {
		return app.CostSnapshotView{}, app.ErrConflict
	}
	var existingID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM production_run_cost_snapshots WHERE production_run_id=$1 AND snapshot_type=$2`, c.RunID, c.SnapshotType).Scan(&existingID)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return app.CostSnapshotView{}, costRepositoryError(err)
		}
		items, listErr := r.ListRunCostSnapshots(ctx, c.RunID)
		if listErr != nil {
			return app.CostSnapshotView{}, listErr
		}
		for _, item := range items {
			if item.ID == existingID {
				return item, nil
			}
		}
		return app.CostSnapshotView{}, app.ErrNotFound
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return app.CostSnapshotView{}, costRepositoryError(err)
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO production_run_cost_snapshots(production_run_id,snapshot_type,printer_id,cost_profile_id,cost_profile_version,energy_source,energy_milliwh,run_seconds,preparation_seconds,finishing_seconds,consumed_grams,wasted_grams,material_consumed_cost_cents,material_waste_cost_cents,material_total_cost_cents,electricity_rate_cents_per_kwh,energy_cost_cents,depreciation_cost_cents,maintenance_cost_cents,preparation_cost_cents,finishing_cost_cents,labor_cost_cents,total_cost_cents,unit_cost_cents,quantity,revenue_snapshot_cents,contribution_margin_cents,contribution_margin_basis_points,profit_cents,profit_margin_basis_points,material_breakdown,created_by_admin_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32) RETURNING id::text`, c.RunID, c.SnapshotType, c.PrinterID, c.ProfileID, c.Profile.Version, c.EnergySource, c.Input.EnergyMilliWh, c.Input.RunSeconds, c.Input.PreparationSeconds, c.Input.FinishingSeconds, c.ConsumedGrams, c.WastedGrams, b.MaterialConsumedCents, b.MaterialWasteCents, b.MaterialTotalCents, c.Input.EnergyRateCentsKWh, b.EnergyCents, b.DepreciationCents, b.MaintenanceCents, b.PreparationCents, b.FinishingCents, b.LaborTotalCents, b.TotalCostCents, b.UnitCostCents, c.Quantity, b.RevenueCents, b.ContributionMarginCents, b.ContributionMarginBPS, b.ProfitCents, b.ProfitMarginBPS, nonEmptyJSON(c.MaterialBreakdown), actor).Scan(&id)
	if err != nil {
		return app.CostSnapshotView{}, costRepositoryError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return app.CostSnapshotView{}, costRepositoryError(err)
	}
	items, e := r.ListRunCostSnapshots(ctx, c.RunID)
	if e != nil {
		return app.CostSnapshotView{}, e
	}
	for _, v := range items {
		if v.ID == id {
			return v, nil
		}
	}
	return app.CostSnapshotView{}, app.ErrNotFound
}
func nonEmptyJSON(v []byte) []byte {
	if len(v) == 0 {
		return []byte("[]")
	}
	return v
}

func (r *ProductionCostRepository) ListRunCostSnapshots(ctx context.Context, runID string) ([]app.CostSnapshotView, error) {
	rows, err := r.pool.Query(ctx, `SELECT s.id::text,s.production_run_id::text,s.snapshot_type,s.printer_id::text,s.cost_profile_id::text,s.cost_profile_version,s.energy_source,s.energy_milliwh,s.run_seconds,s.preparation_seconds,s.finishing_seconds,s.electricity_rate_cents_per_kwh,p.machine_acquisition_cents,p.machine_useful_life_seconds,p.maintenance_rate_cents_per_hour,p.labor_rate_cents_per_hour,s.consumed_grams,s.wasted_grams,s.material_breakdown,s.material_consumed_cost_cents,s.material_waste_cost_cents,s.material_total_cost_cents,s.energy_cost_cents,s.depreciation_cost_cents,s.maintenance_cost_cents,s.preparation_cost_cents,s.finishing_cost_cents,s.labor_cost_cents,s.total_cost_cents,s.unit_cost_cents,s.revenue_snapshot_cents,s.contribution_margin_cents,s.contribution_margin_basis_points,s.profit_cents,s.profit_margin_basis_points FROM production_run_cost_snapshots s JOIN production_cost_profiles p ON p.id=s.cost_profile_id WHERE s.production_run_id=$1 ORDER BY s.created_at DESC`, runID)
	if err != nil {
		return nil, costRepositoryError(err)
	}
	defer rows.Close()
	items := []app.CostSnapshotView{}
	for rows.Next() {
		var v app.CostSnapshotView
		if err = rows.Scan(&v.ID, &v.RunID, &v.SnapshotType, &v.PrinterID, &v.CostProfileID, &v.CostProfileVersion, &v.EnergySource, &v.EnergyMilliWh, &v.RunSeconds, &v.PreparationSeconds, &v.FinishingSeconds, &v.ElectricityRateCentsPerKWh, &v.MachineAcquisitionCents, &v.MachineUsefulLifeSeconds, &v.MaintenanceRateCentsPerHour, &v.LaborRateCentsPerHour, &v.ConsumedGrams, &v.WastedGrams, &v.MaterialBreakdown, &v.MaterialConsumedCents, &v.MaterialWasteCents, &v.MaterialTotalCents, &v.EnergyCents, &v.DepreciationCents, &v.MaintenanceCents, &v.PreparationCents, &v.FinishingCents, &v.LaborTotalCents, &v.TotalCostCents, &v.UnitCostCents, &v.RevenueCents, &v.ContributionMarginCents, &v.ContributionMarginBPS, &v.ProfitCents, &v.ProfitMarginBPS); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *ProductionCostRepository) CostProfitability(ctx context.Context, from, to string) ([]app.ProductCostProfitability, error) {
	rows, err := r.pool.Query(ctx, `SELECT coalesce(oi.product_id::text,''),oi.product_name,count(DISTINCT s.production_run_id),sum(s.quantity),sum(s.revenue_snapshot_cents),sum(s.total_cost_cents),sum(s.contribution_margin_cents),sum(s.profit_cents),CASE WHEN sum(s.revenue_snapshot_cents)>0 THEN round(sum(s.contribution_margin_cents)::numeric*10000/sum(s.revenue_snapshot_cents))::bigint END,CASE WHEN sum(s.revenue_snapshot_cents)>0 THEN round(sum(s.profit_cents)::numeric*10000/sum(s.revenue_snapshot_cents))::bigint END FROM production_run_cost_snapshots s JOIN production_runs pr ON pr.id=s.production_run_id JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id WHERE s.snapshot_type='actual' AND ($1='' OR s.created_at >= $1::date) AND ($2='' OR s.created_at < ($2::date + interval '1 day')) GROUP BY oi.product_id,oi.product_name ORDER BY sum(s.profit_cents) DESC`, from, to)
	if err != nil {
		return nil, costRepositoryError(err)
	}
	defer rows.Close()
	items := []app.ProductCostProfitability{}
	for rows.Next() {
		var v app.ProductCostProfitability
		if err = rows.Scan(&v.ProductID, &v.ProductName, &v.Runs, &v.Units, &v.RevenueCents, &v.ManufacturingCostCents, &v.ContributionMarginCents, &v.ProfitCents, &v.ContributionMarginBPS, &v.ProfitMarginBPS); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *ProductionCostRepository) CostProfitabilityByPrinter(ctx context.Context, from, to string) ([]app.PrinterCostProfitability, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id::text,p.name,count(*),sum(s.revenue_snapshot_cents),sum(s.total_cost_cents),sum(s.contribution_margin_cents),sum(s.profit_cents),CASE WHEN sum(s.revenue_snapshot_cents)>0 THEN round(sum(s.contribution_margin_cents)::numeric*10000/sum(s.revenue_snapshot_cents))::bigint END,CASE WHEN sum(s.revenue_snapshot_cents)>0 THEN round(sum(s.profit_cents)::numeric*10000/sum(s.revenue_snapshot_cents))::bigint END FROM production_run_cost_snapshots s JOIN printers p ON p.id=s.printer_id WHERE s.snapshot_type='actual' AND ($1='' OR s.created_at >= $1::date) AND ($2='' OR s.created_at < ($2::date + interval '1 day')) GROUP BY p.id,p.name ORDER BY sum(s.profit_cents) DESC`, from, to)
	if err != nil {
		return nil, costRepositoryError(err)
	}
	defer rows.Close()
	items := []app.PrinterCostProfitability{}
	for rows.Next() {
		var v app.PrinterCostProfitability
		if err = rows.Scan(&v.PrinterID, &v.PrinterName, &v.Runs, &v.RevenueCents, &v.ManufacturingCostCents, &v.ContributionMarginCents, &v.ProfitCents, &v.ContributionMarginBPS, &v.ProfitMarginBPS); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *ProductionCostRepository) CostEstimateComparison(ctx context.Context, from, to string) ([]app.CostEstimateComparison, error) {
	rows, err := r.pool.Query(ctx, `SELECT actual.production_run_id::text,estimate.total_cost_cents,actual.total_cost_cents,actual.total_cost_cents-estimate.total_cost_cents,CASE WHEN estimate.total_cost_cents=0 THEN NULL ELSE round((actual.total_cost_cents-estimate.total_cost_cents)::numeric*10000/estimate.total_cost_cents)::bigint END FROM production_run_cost_snapshots actual JOIN production_run_cost_snapshots estimate ON estimate.production_run_id=actual.production_run_id AND estimate.snapshot_type='estimate' WHERE actual.snapshot_type='actual' AND ($1='' OR actual.created_at >= $1::date) AND ($2='' OR actual.created_at < ($2::date + interval '1 day')) ORDER BY actual.created_at DESC`, from, to)
	if err != nil {
		return nil, costRepositoryError(err)
	}
	defer rows.Close()
	items := []app.CostEstimateComparison{}
	for rows.Next() {
		var v app.CostEstimateComparison
		if err = rows.Scan(&v.RunID, &v.EstimatedCostCents, &v.ActualCostCents, &v.DifferenceCents, &v.DifferenceBasisPoints); err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
