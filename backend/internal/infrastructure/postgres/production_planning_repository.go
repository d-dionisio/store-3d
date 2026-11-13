package postgres

import (
	"context"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
)

func (r *ProductionRepository) ProductionDashboard(ctx context.Context, since time.Time) (app.Dashboard, error) {
	var out app.Dashboard
	out.PeriodDays = 30
	err := r.pool.QueryRow(ctx, `
WITH jobs AS (
 SELECT count(*) FILTER (WHERE status='awaiting_production') awaiting,
        count(*) FILTER (WHERE status IN ('queued','preparing')) queued,
        count(*) FILTER (WHERE status='quality_check') quality,
        count(*) FILTER (WHERE due_at < now() AND status NOT IN ('completed','cancelled')) overdue,
        count(*) FILTER (WHERE status='completed' AND completed_at >= $1) completed
 FROM production_jobs
), runs AS (
 SELECT count(*) FILTER (WHERE status IN ('printing','post_processing')) printing FROM production_runs
), printers AS (
 SELECT count(*) FILTER (WHERE status='available') available,
        count(*) FILTER (WHERE status='busy') busy,
        count(*) FILTER (WHERE status='maintenance') maintenance FROM printers
), materials AS (
 SELECT count(*) FILTER (WHERE usable <= minimum) low FROM (
  SELECT material_id,color_id,sum(current_weight_grams) usable,sum(minimum_stock_grams) minimum
  FROM material_spools WHERE status IN ('sealed','available','in_use') GROUP BY material_id,color_id
 ) grouped_materials
)
SELECT jobs.awaiting,jobs.queued,runs.printing,jobs.quality,jobs.overdue,jobs.completed,
       printers.available,printers.busy,printers.maintenance,materials.low
FROM jobs,runs,printers,materials`, since).Scan(
		&out.AwaitingJobs, &out.QueuedJobs, &out.PrintingRuns, &out.QualityJobs, &out.OverdueJobs,
		&out.CompletedJobs, &out.AvailablePrinters, &out.BusyPrinters, &out.MaintenancePrinters, &out.LowStockMaterials)
	return out, productionError(err)
}

func (r *ProductionRepository) PlanningInputs(ctx context.Context, filter app.PlanningFilter) ([]app.PlanningInput, int64, error) {
	var total int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM production_jobs pj
WHERE (($1='' AND pj.status NOT IN ('completed','cancelled')) OR pj.status=$1) AND ($2::uuid IS NULL OR EXISTS (
 SELECT 1 FROM production_runs pr WHERE pr.production_job_id=pj.id AND pr.printer_id=$2::uuid))
AND ($3::timestamptz IS NULL OR pj.due_at >= $3) AND ($4::timestamptz IS NULL OR pj.due_at <= $4)`,
		filter.Status, nullableUUID(filter.PrinterID), filter.DueFrom, filter.DueTo).Scan(&total)
	if err != nil {
		return nil, 0, productionError(err)
	}
	rows, err := r.pool.Query(ctx, `
WITH stock AS (
 SELECT material_id,color_id,sum(current_weight_grams)::bigint usable,
        sum(minimum_stock_grams)::bigint minimum
 FROM material_spools WHERE status IN ('sealed','available','in_use') GROUP BY material_id,color_id
)
SELECT pj.id,pj.job_number,o.order_number,oi.product_name,coalesce(pv.name,oi.sku_snapshot,''),pj.status,
       coalesce(latest.printer_id::text,''),coalesce(p.name,''),coalesce(p.status,''),
       pj.material_id,m.name,pj.color_id,c.name,pj.quantity,pj.priority,pj.due_at,latest.started_at,
       latest.estimated_minutes,h.median_minutes,h.samples,latest.estimated_weight_grams,
       CASE WHEN latest.estimated_weight_grams IS NULL THEN false
		ELSE latest.estimated_weight_grams > greatest(coalesce(stock.usable,0)-coalesce(stock.minimum,0),0) END
FROM production_jobs pj
JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id
LEFT JOIN product_variants pv ON pv.id=oi.product_variant_id
JOIN materials m ON m.id=pj.material_id JOIN colors c ON c.id=pj.color_id
LEFT JOIN LATERAL (SELECT pr.* FROM production_runs pr WHERE pr.production_job_id=pj.id ORDER BY pr.attempt_number DESC LIMIT 1) latest ON true
LEFT JOIN printers p ON p.id=latest.printer_id
LEFT JOIN stock ON stock.material_id=pj.material_id AND stock.color_id=pj.color_id
LEFT JOIN LATERAL (
 WITH candidates AS (
  SELECT coalesce(prh.actual_minutes,extract(epoch FROM (prh.completed_at-prh.started_at))/60)::int minutes,
   CASE WHEN oih.product_variant_id=oi.product_variant_id AND p.model IS NOT NULL AND ph.model=p.model THEN 1
        WHEN oih.product_id=oi.product_id AND p.model IS NOT NULL AND ph.model=p.model THEN 2 ELSE 3 END tier
  FROM production_runs prh JOIN production_jobs pjh ON pjh.id=prh.production_job_id
  JOIN order_items oih ON oih.id=pjh.order_item_id LEFT JOIN printers ph ON ph.id=prh.printer_id
  WHERE prh.status='completed' AND prh.completed_at >= now()-interval '90 days'
    AND coalesce(prh.actual_minutes,extract(epoch FROM (prh.completed_at-prh.started_at))/60)::int > 0
    AND oih.product_id=oi.product_id
 )
 SELECT percentile_disc(.5) WITHIN GROUP (ORDER BY minutes)::int median_minutes,count(*)::int samples
 FROM candidates WHERE tier=(SELECT min(tier) FROM candidates)
) h ON true
WHERE (($1='' AND pj.status NOT IN ('completed','cancelled')) OR pj.status=$1) AND ($2::uuid IS NULL OR latest.printer_id=$2::uuid)
 AND ($3::timestamptz IS NULL OR pj.due_at >= $3) AND ($4::timestamptz IS NULL OR pj.due_at <= $4)
ORDER BY pj.priority DESC,pj.due_at NULLS LAST,pj.created_at
LIMIT $5 OFFSET $6`, filter.Status, nullableUUID(filter.PrinterID), filter.DueFrom, filter.DueTo, filter.Limit, (filter.Page-1)*filter.Limit)
	if err != nil {
		return nil, 0, productionError(err)
	}
	defer rows.Close()
	result := []app.PlanningInput{}
	for rows.Next() {
		var v app.PlanningInput
		if err := rows.Scan(&v.ID, &v.Number, &v.OrderNumber, &v.ProductName, &v.VariantName, &v.Status, &v.PrinterID, &v.PrinterName, &v.PrinterStatus, &v.MaterialID, &v.MaterialName, &v.ColorID, &v.ColorName, &v.Quantity, &v.Priority, &v.DueAt, &v.StartedAt, &v.ExplicitEstimatedMinutes, &v.HistoricalMedianMinutes, &v.HistoricalSamples, &v.EstimatedWeightGrams, &v.MaterialInsufficient); err != nil {
			return nil, 0, err
		}
		result = append(result, v)
	}
	return result, total, rows.Err()
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *ProductionRepository) PrinterCapacities(ctx context.Context, now time.Time) ([]app.PrinterCapacityView, error) {
	rows, err := r.pool.Query(ctx, `SELECT p.id,p.code,p.name,p.manufacturer,p.model,p.status,
	 coalesce(work.assigned_jobs,0),coalesce(work.active_runs,0),coalesce(work.known_minutes,0),
	 coalesce(maintenance.events,0),maintenance.last_at
FROM printers p
LEFT JOIN LATERAL (
 SELECT count(DISTINCT production_job_id) FILTER (WHERE status NOT IN ('completed','failed','cancelled'))::int assigned_jobs,
        count(*) FILTER (WHERE status IN ('preparing','printing','post_processing','quality_check'))::int active_runs,
        coalesce(sum(estimated_minutes) FILTER (WHERE status NOT IN ('completed','failed','cancelled')),0)::int known_minutes
 FROM production_runs WHERE printer_id=p.id
) work ON true
LEFT JOIN LATERAL (
 SELECT count(*)::int events,max(completed_at) last_at FROM printer_maintenance_events WHERE printer_id=p.id
) maintenance ON true
ORDER BY p.name`)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	result := []app.PrinterCapacityView{}
	for rows.Next() {
		var v app.PrinterCapacityView
		if err := rows.Scan(&v.ID, &v.Code, &v.Name, &v.Manufacturer, &v.Model, &v.Status, &v.AssignedJobs, &v.ActiveRuns, &v.KnownMinutes, &v.MaintenanceEvents, &v.LastMaintenanceAt); err != nil {
			return nil, err
		}
		v.Available = v.Status == "available"
		if v.KnownMinutes > 0 {
			next := now.Add(time.Duration(v.KnownMinutes) * time.Minute)
			v.NextAvailableAt = &next
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

func (r *ProductionRepository) MaterialPlanning(ctx context.Context) ([]app.MaterialPlanningView, error) {
	rows, err := r.pool.Query(ctx, `WITH supply AS (
 SELECT material_id,color_id,count(*)::int spools,sum(current_weight_grams)::bigint usable,sum(minimum_stock_grams)::bigint minimum
 FROM material_spools WHERE status IN ('sealed','available','in_use') GROUP BY material_id,color_id
), demand AS (
	SELECT pj.material_id,pj.color_id,sum(coalesce(pr.estimated_weight_grams,0))::bigint required,
 count(*) FILTER (WHERE pr.estimated_weight_grams IS NULL)::int unknown
 FROM production_jobs pj LEFT JOIN LATERAL (SELECT estimated_weight_grams FROM production_runs r WHERE r.production_job_id=pj.id ORDER BY attempt_number DESC LIMIT 1) pr ON true
 WHERE pj.status NOT IN ('completed','cancelled') GROUP BY pj.material_id,pj.color_id
), keys AS (SELECT material_id,color_id FROM supply UNION SELECT material_id,color_id FROM demand)
SELECT k.material_id,m.name,k.color_id,c.name,coalesce(s.usable,0),coalesce(s.minimum,0),coalesce(d.required,0),coalesce(d.unknown,0),coalesce(s.spools,0),
 coalesce(d.required,0)>greatest(coalesce(s.usable,0)-coalesce(s.minimum,0),0)
FROM keys k JOIN materials m ON m.id=k.material_id JOIN colors c ON c.id=k.color_id
LEFT JOIN supply s USING(material_id,color_id) LEFT JOIN demand d USING(material_id,color_id) ORDER BY m.name,c.name`)
	if err != nil {
		return nil, productionError(err)
	}
	defer rows.Close()
	result := []app.MaterialPlanningView{}
	for rows.Next() {
		var v app.MaterialPlanningView
		if err := rows.Scan(&v.MaterialID, &v.MaterialName, &v.ColorID, &v.ColorName, &v.UsableGrams, &v.MinimumStockGrams, &v.KnownRequiredGrams, &v.UnknownDemandJobs, &v.SpoolCount, &v.Insufficient); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
