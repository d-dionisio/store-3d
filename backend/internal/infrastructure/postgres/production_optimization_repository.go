package postgres

import (
	"context"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

// OptimizationSnapshot is read-only and deliberately bounded to active jobs
// and a 90-day historical window.
func (r *ProductionRepository) OptimizationSnapshot(ctx context.Context, now time.Time) (app.OptimizationSnapshot, error) {
	result := app.OptimizationSnapshot{Jobs: []domain.OptimizationJob{}, Printers: []domain.OptimizationPrinter{}}
	rows, err := r.pool.Query(ctx, `
SELECT pj.id,pj.job_number,oi.product_name,pj.material_id,m.name,pj.color_id,c.name,
       pj.priority,pj.quantity,pj.due_at,pj.created_at,pv.width_mm,pv.depth_mm,pv.height_mm,
       coalesce(latest.estimated_minutes,h.median_minutes),
       CASE WHEN latest.estimated_minutes IS NOT NULL THEN 0 ELSE coalesce(h.samples,0) END,
       mts.print_temp_recommended_c,mts.bed_temp_recommended_c,mts.requires_enclosure,
       coalesce(mts.required_surface,''),coalesce(mts.status='verified',false)
FROM production_jobs pj
JOIN order_items oi ON oi.id=pj.order_item_id
LEFT JOIN product_variants pv ON pv.id=oi.product_variant_id
JOIN materials m ON m.id=pj.material_id JOIN colors c ON c.id=pj.color_id
LEFT JOIN material_technical_specs mts ON mts.material_id=pj.material_id
LEFT JOIN LATERAL (SELECT estimated_minutes FROM production_runs WHERE production_job_id=pj.id ORDER BY attempt_number DESC LIMIT 1) latest ON true
LEFT JOIN LATERAL (
 WITH candidates AS (
  SELECT coalesce(prh.actual_minutes,extract(epoch FROM (prh.completed_at-prh.started_at))/60)::int minutes,
         CASE WHEN oih.product_variant_id=oi.product_variant_id THEN 1 ELSE 2 END tier
  FROM production_runs prh JOIN production_jobs pjh ON pjh.id=prh.production_job_id
  JOIN order_items oih ON oih.id=pjh.order_item_id
  WHERE prh.status='completed' AND prh.completed_at >= $1::timestamptz-interval '90 days'
    AND coalesce(prh.actual_minutes,extract(epoch FROM (prh.completed_at-prh.started_at))/60)::int>0
    AND oih.product_id=oi.product_id
 ) SELECT round(percentile_cont(.5) WITHIN GROUP(ORDER BY minutes))::int median_minutes,count(*)::int samples
   FROM candidates WHERE tier=(SELECT min(tier) FROM candidates)
) h ON true
WHERE pj.status IN ('awaiting_production','queued','preparing')
ORDER BY pj.priority DESC,pj.due_at NULLS LAST,pj.created_at,pj.id`, now)
	if err != nil {
		return result, productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var job domain.OptimizationJob
		var samples int
		if err = rows.Scan(&job.ID, &job.Number, &job.ProductName, &job.MaterialID, &job.MaterialName, &job.ColorID, &job.ColorName, &job.Priority, &job.Quantity, &job.DueAt, &job.CreatedAt, &job.WidthMM, &job.DepthMM, &job.HeightMM, &job.EstimatedMinutes, &samples, &job.PrintTempC, &job.BedTempC, &job.RequiresEnclosure, &job.RequiredSurface, &job.MaterialSpecsVerified); err != nil {
			return result, err
		}
		job.Confidence = domain.Confidence(samples)
		result.Jobs = append(result.Jobs, job)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	printerRows, err := r.pool.Query(ctx, `
SELECT p.id,p.name,p.model,CASE WHEN telemetry.last_seen_at>=now()-interval '45 seconds' AND telemetry.connection_state='online' AND telemetry.printer_state IN ('printing','heating','paused') THEN 'busy' WHEN telemetry.last_seen_at>=now()-interval '45 seconds' AND telemetry.connection_state='online' AND telemetry.printer_state IN ('idle','finished') THEN 'available' WHEN telemetry.last_seen_at>=now()-interval '45 seconds' AND telemetry.connection_state IN ('offline','error') THEN 'offline' ELSE p.status END,
       least(p.build_width_mm,coalesce(spec.max_width_mm,p.build_width_mm)),least(p.build_depth_mm,coalesce(spec.max_depth_mm,p.build_depth_mm)),least(p.build_height_mm,coalesce(spec.max_height_mm,p.build_height_mm)),
       coalesce(load.known_minutes,0),coalesce(installed.material_id::text,active.material_id::text,''),coalesce(installed.color_id::text,active.color_id::text,''),
       spec.max_hotend_temp_c,spec.max_bed_temp_c,config.installed_nozzle_microns,spec.enclosed_chamber,
       coalesce(config.installed_nozzle_type,''),coalesce(config.installed_surface,''),coalesce(spec.status='verified',false),coalesce(config.status='configured',false),
       coalesce(materials.ids,ARRAY[]::text[]),coalesce(nozzles.diameters,ARRAY[]::integer[])
FROM printers p
LEFT JOIN printer_current_telemetry telemetry ON telemetry.printer_id=p.id
LEFT JOIN printer_technical_specs spec ON spec.printer_id=p.id AND spec.status IN ('draft','verified')
LEFT JOIN printer_operational_configs config ON config.printer_id=p.id
LEFT JOIN LATERAL (SELECT array_agg(material_id::text ORDER BY material_id) ids FROM printer_supported_materials WHERE printer_spec_id=spec.id) materials ON true
LEFT JOIN LATERAL (SELECT array_agg(DISTINCT diameter_microns ORDER BY diameter_microns) diameters FROM printer_supported_nozzles WHERE printer_spec_id=spec.id) nozzles ON true
LEFT JOIN LATERAL (
 SELECT coalesce(sum(coalesce(pr.estimated_minutes,0)),0)::int known_minutes
 FROM production_runs pr WHERE pr.printer_id=p.id AND pr.status NOT IN ('completed','failed','cancelled')
) load ON true
LEFT JOIN LATERAL (
 SELECT pj.material_id,pj.color_id FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id
 WHERE pr.printer_id=p.id AND pr.status IN ('preparing','printing','post_processing','quality_check')
 ORDER BY pr.updated_at DESC LIMIT 1
) active ON true
LEFT JOIN LATERAL (
 SELECT ms.material_id,ms.color_id FROM spool_installations si JOIN material_spools ms ON ms.id=si.material_spool_id
 WHERE si.printer_id=p.id AND si.removed_at IS NULL ORDER BY si.slot_number LIMIT 1
) installed ON true
ORDER BY p.code,p.id`)
	if err != nil {
		return result, productionError(err)
	}
	defer printerRows.Close()
	for printerRows.Next() {
		var printer domain.OptimizationPrinter
		var materialIDs []string
		var nozzles []int32
		if err = printerRows.Scan(&printer.ID, &printer.Name, &printer.Model, &printer.Status, &printer.BuildWidthMM, &printer.BuildDepthMM, &printer.BuildHeightMM, &printer.KnownMinutes, &printer.CurrentMaterialID, &printer.CurrentColorID, &printer.MaxHotendTempC, &printer.MaxBedTempC, &printer.InstalledNozzleMicrons, &printer.EnclosedChamber, &printer.InstalledNozzleType, &printer.InstalledSurface, &printer.TechnicalSpecsVerified, &printer.OperationalConfigReady, &materialIDs, &nozzles); err != nil {
			return result, err
		}
		printer.SupportedMaterialIDs = map[string]bool{}
		for _, id := range materialIDs {
			printer.SupportedMaterialIDs[id] = true
		}
		printer.SupportedNozzles = map[int]bool{}
		for _, diameter := range nozzles {
			printer.SupportedNozzles[int(diameter)] = true
		}
		result.Printers = append(result.Printers, printer)
	}
	return result, printerRows.Err()
}
