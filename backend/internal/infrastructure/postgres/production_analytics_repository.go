package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
	"github.com/jackc/pgx/v5"
)

const analyticsFrom = ` FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id JOIN materials m ON m.id=pj.material_id JOIN colors c ON c.id=pj.color_id LEFT JOIN printers p ON p.id=pr.printer_id LEFT JOIN LATERAL(SELECT result,inspected_at FROM quality_inspections WHERE production_run_id=pr.id ORDER BY inspected_at DESC LIMIT 1) qi ON true LEFT JOIN LATERAL(SELECT count(*)::bigint material_rows,coalesce(sum(consumed_grams),0)::bigint consumed,coalesce(sum(wasted_grams),0)::bigint wasted,coalesce(sum(total_cost_cents),0)::bigint cost FROM production_run_materials WHERE production_run_id=pr.id) usage ON true `

const materialCostSnapshot = `CASE WHEN usage.material_rows>0 THEN usage.cost ELSE coalesce(pr.material_cost_cents,0) END`

func analyticsWhere(filter app.AnalyticsFilter) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}
	add := func(value any, clause string) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.From != nil {
		add(*filter.From, "coalesce(pr.completed_at,pr.started_at,pr.created_at)>=$%d")
	}
	if filter.To != nil {
		add(*filter.To, "coalesce(pr.completed_at,pr.started_at,pr.created_at)<$%d")
	}
	if filter.PrinterID != "" {
		add(filter.PrinterID, "pr.printer_id=$%d")
	}
	if filter.ProductID != "" {
		add(filter.ProductID, "oi.product_id=$%d")
	}
	if strings.TrimSpace(filter.ProductQuery) != "" {
		add("%"+strings.TrimSpace(filter.ProductQuery)+"%", "oi.product_name ILIKE $%d")
	}
	if filter.MaterialID != "" {
		add(filter.MaterialID, "pj.material_id=$%d")
	}
	if strings.TrimSpace(filter.MaterialQuery) != "" {
		add("%"+strings.TrimSpace(filter.MaterialQuery)+"%", "m.name ILIKE $%d")
	}
	if filter.Status != "" {
		add(filter.Status, "pr.status=$%d")
	}
	if filter.Quality != "" {
		add(filter.Quality, "qi.result=$%d")
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

const historySelect = `SELECT pr.id,pj.id,pj.job_number,o.order_number,coalesce(oi.product_id::text,''),oi.product_name,coalesce(oi.product_variant_id::text,''),coalesce(nullif(concat_ws(' · ',oi.size_name,oi.finish_name,oi.dimensions_snapshot),''),oi.sku_snapshot,''),coalesce(pr.printer_id::text,''),coalesce(p.name,pr.printer_reference_snapshot,''),pj.material_id,m.name,c.name,pr.status,coalesce(qi.result,''),coalesce(pr.failure_reason,''),pr.attempt_number,pr.quantity,pr.estimated_minutes,pr.actual_minutes,CASE WHEN pr.estimated_minutes>0 AND pr.actual_minutes>0 THEN pr.actual_minutes-pr.estimated_minutes END,CASE WHEN pr.estimated_minutes>0 AND pr.actual_minutes>0 THEN round(((pr.actual_minutes-pr.estimated_minutes)::numeric/pr.estimated_minutes)*100,2)::float8 END,usage.consumed,usage.wasted,` + materialCostSnapshot + `,pr.started_at,pr.completed_at,pr.reprint_of_run_id IS NOT NULL,CASE WHEN pr.actual_minutes>0 THEN 'manual_run' ELSE 'unknown' END`

func scanHistory(row pgx.Row) (app.HistoryItem, error) {
	var value app.HistoryItem
	err := row.Scan(&value.ID, &value.JobID, &value.JobNumber, &value.OrderNumber, &value.ProductID, &value.ProductName, &value.VariantID, &value.VariantName, &value.PrinterID, &value.PrinterName, &value.MaterialID, &value.MaterialName, &value.ColorName, &value.Status, &value.QualityResult, &value.FailureReason, &value.Attempt, &value.Quantity, &value.EstimatedMinutes, &value.ActualMinutes, &value.DifferenceMinutes, &value.DifferencePercent, &value.ConsumedGrams, &value.WastedGrams, &value.MaterialCostCents, &value.StartedAt, &value.CompletedAt, &value.IsReprint, &value.DurationSource)
	return value, productionError(err)
}

func (r *ProductionRepository) ListHistory(ctx context.Context, filter app.AnalyticsFilter) (app.HistoryPage, error) {
	where, args := analyticsWhere(filter)
	result := app.HistoryPage{Items: []app.HistoryItem{}, Page: filter.Page, Limit: filter.Limit}
	if err := r.pool.QueryRow(ctx, "SELECT count(*)"+analyticsFrom+where, args...).Scan(&result.Total); err != nil {
		return result, productionError(err)
	}
	args = append(args, filter.Limit, (filter.Page-1)*filter.Limit)
	query := historySelect + analyticsFrom + where + fmt.Sprintf(" ORDER BY coalesce(pr.completed_at,pr.started_at,pr.created_at) DESC,pr.id LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return result, productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		value, scanErr := scanHistory(rows)
		if scanErr != nil {
			return result, scanErr
		}
		result.Items = append(result.Items, value)
	}
	return result, rows.Err()
}

func (r *ProductionRepository) GetHistory(ctx context.Context, id string) (app.HistoryDetail, error) {
	value, err := scanHistory(r.pool.QueryRow(ctx, historySelect+analyticsFrom+` WHERE pr.id=$1`, id))
	if err != nil {
		return app.HistoryDetail{}, err
	}
	result := app.HistoryDetail{Run: value, Materials: []app.RunMaterialView{}, Inspections: []app.InspectionView{}, Events: []app.AnalyticsEventView{}, ObservedCandidates: []app.ObservedRunCandidate{}}
	rows, err := r.pool.Query(ctx, `SELECT ms.id,ms.spool_code,m.name,c.name,rm.consumed_grams,rm.wasted_grams,rm.cost_per_kg_snapshot_cents,rm.total_cost_cents FROM production_run_materials rm JOIN material_spools ms ON ms.id=rm.material_spool_id JOIN materials m ON m.id=ms.material_id JOIN colors c ON c.id=ms.color_id WHERE rm.production_run_id=$1 ORDER BY rm.created_at`, id)
	if err != nil {
		return result, productionError(err)
	}
	for rows.Next() {
		var item app.RunMaterialView
		if err = rows.Scan(&item.SpoolID, &item.SpoolCode, &item.MaterialName, &item.ColorName, &item.ConsumedGrams, &item.WastedGrams, &item.CostPerKGCents, &item.TotalCostCents); err != nil {
			rows.Close()
			return result, err
		}
		result.Materials = append(result.Materials, item)
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, `SELECT id,result,coalesce(notes,''),dimensional_accuracy_ok,visual_quality_ok,functional_test_ok,inspected_at FROM quality_inspections WHERE production_run_id=$1 ORDER BY inspected_at`, id)
	if err != nil {
		return result, productionError(err)
	}
	for rows.Next() {
		var item app.InspectionView
		if err = rows.Scan(&item.ID, &item.Result, &item.Notes, &item.DimensionalAccuracyOK, &item.VisualQualityOK, &item.FunctionalTestOK, &item.InspectedAt); err != nil {
			rows.Close()
			return result, err
		}
		result.Inspections = append(result.Inspections, item)
	}
	rows.Close()
	rows, err = r.pool.Query(ctx, `SELECT id,event_type,coalesce(previous_status,''),coalesce(new_status,''),description,occurred_at FROM production_events WHERE production_run_id=$1 ORDER BY occurred_at`, id)
	if err != nil {
		return result, productionError(err)
	}
	for rows.Next() {
		var item app.AnalyticsEventView
		if err = rows.Scan(&item.ID, &item.Type, &item.PreviousStatus, &item.NewStatus, &item.Description, &item.OccurredAt); err != nil {
			rows.Close()
			return result, err
		}
		result.Events = append(result.Events, item)
	}
	rows.Close()
	if value.PrinterID != "" && value.StartedAt != nil {
		to := time.Now().UTC()
		if value.CompletedAt != nil {
			to = *value.CompletedAt
		}
		rows, err = r.pool.Query(ctx, `SELECT id,coalesce(job_name,''),last_state,first_observed_at,last_observed_at,max_progress_percent::float8 FROM printer_observed_jobs WHERE printer_id=$1 AND last_observed_at>=$2 AND first_observed_at<=$3 ORDER BY first_observed_at`, value.PrinterID, *value.StartedAt, to)
		if err != nil {
			return result, productionError(err)
		}
		for rows.Next() {
			var item app.ObservedRunCandidate
			item.Association = "time_window_candidate"
			if err = rows.Scan(&item.ID, &item.JobName, &item.LastState, &item.FirstObservedAt, &item.LastObservedAt, &item.MaxProgressPercent); err != nil {
				rows.Close()
				return result, err
			}
			result.ObservedCandidates = append(result.ObservedCandidates, item)
		}
		rows.Close()
	}
	return result, nil
}

func (r *ProductionRepository) Productivity(ctx context.Context, filter app.AnalyticsFilter) (app.ProductivityAnalytics, error) {
	where, args := analyticsWhere(filter)
	result := app.ProductivityAnalytics{Timeline: []app.PeriodPoint{}, Printers: []app.PrinterProductivity{}, Products: []app.ProductProductivity{}, Materials: []app.MaterialProductivity{}, TimeAccuracy: []app.TimeAccuracyPoint{}, GeneratedAt: time.Now().UTC(), Limitations: []string{"Utilização permanece indisponível sem histórico confiável de disponibilidade da impressora.", "Duração usa somente production_runs.actual_minutes; telemetria observada não é misturada."}}
	err := r.pool.QueryRow(ctx, `SELECT count(*),count(*)FILTER(WHERE pr.status='completed'),count(*)FILTER(WHERE pr.status='failed'),count(*)FILTER(WHERE pr.reprint_of_run_id IS NOT NULL),coalesce(sum(pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed')),0),coalesce(sum(usage.consumed),0),coalesce(sum(usage.wasted),0),coalesce(sum(`+materialCostSnapshot+`),0)`+analyticsFrom+where, args...).Scan(&result.Summary.TotalRuns, &result.Summary.CompletedRuns, &result.Summary.FailedRuns, &result.Summary.Reprints, &result.Summary.RecordedMinutes, &result.Summary.ConsumedGrams, &result.Summary.WastedGrams, &result.Summary.MaterialCostCents)
	if err != nil {
		return result, productionError(err)
	}
	result.Summary.SuccessRate = domain.Percentage(result.Summary.CompletedRuns, result.Summary.CompletedRuns+result.Summary.FailedRuns)
	result.Summary.FailureRate = domain.Percentage(result.Summary.FailedRuns, result.Summary.CompletedRuns+result.Summary.FailedRuns)
	result.Summary.UtilizationLimitation = "availability_history_unavailable"
	if err = r.analyticsTimeline(ctx, &result, where, args); err != nil {
		return result, err
	}
	if err = r.analyticsPrinters(ctx, &result, where, args); err != nil {
		return result, err
	}
	if err = r.analyticsProducts(ctx, &result, where, args); err != nil {
		return result, err
	}
	if err = r.analyticsMaterials(ctx, &result, where, args); err != nil {
		return result, err
	}
	if err = r.analyticsTime(ctx, &result, where, args); err != nil {
		return result, err
	}
	return result, nil
}

func (r *ProductionRepository) analyticsTimeline(ctx context.Context, out *app.ProductivityAnalytics, where string, args []any) error {
	rows, err := r.pool.Query(ctx, `SELECT to_char(date_trunc('day',coalesce(pr.completed_at,pr.started_at,pr.created_at)),'YYYY-MM-DD'),count(*),count(*)FILTER(WHERE pr.status='completed'),count(*)FILTER(WHERE pr.status='failed'),coalesce(sum(pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed')),0),coalesce(sum(usage.consumed),0),coalesce(sum(usage.wasted),0),coalesce(sum(`+materialCostSnapshot+`),0)`+analyticsFrom+where+` GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.PeriodPoint
		if err = rows.Scan(&v.Period, &v.Runs, &v.Completed, &v.Failed, &v.RecordedMinutes, &v.ConsumedGrams, &v.WastedGrams, &v.MaterialCostCents); err != nil {
			return err
		}
		out.Timeline = append(out.Timeline, v)
	}
	return rows.Err()
}
func (r *ProductionRepository) analyticsPrinters(ctx context.Context, out *app.ProductivityAnalytics, where string, args []any) error {
	rows, err := r.pool.Query(ctx, `SELECT coalesce(pr.printer_id::text,''),coalesce(p.name,pr.printer_reference_snapshot,'Não informada'),count(*),count(*)FILTER(WHERE pr.status='completed'),count(*)FILTER(WHERE pr.status='failed'),count(*)FILTER(WHERE pr.reprint_of_run_id IS NOT NULL),coalesce(sum(pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed')),0),coalesce(sum(usage.consumed),0),coalesce(sum(usage.wasted),0),coalesce(sum(`+materialCostSnapshot+`),0),avg(pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed'))::float8,percentile_cont(.5)WITHIN GROUP(ORDER BY pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed'))::float8`+analyticsFrom+where+` GROUP BY pr.printer_id,p.name,pr.printer_reference_snapshot ORDER BY count(*) DESC`, args...)
	if err != nil {
		return productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.PrinterProductivity
		if err = rows.Scan(&v.PrinterID, &v.PrinterName, &v.TotalRuns, &v.CompletedRuns, &v.FailedRuns, &v.Reprints, &v.RecordedMinutes, &v.ConsumedGrams, &v.WastedGrams, &v.MaterialCostCents, &v.AverageMinutes, &v.MedianMinutes); err != nil {
			return err
		}
		v.SuccessRate = domain.Percentage(v.CompletedRuns, v.CompletedRuns+v.FailedRuns)
		v.FailureRate = domain.Percentage(v.FailedRuns, v.CompletedRuns+v.FailedRuns)
		v.UtilizationLimitation = "availability_history_unavailable"
		out.Printers = append(out.Printers, v)
	}
	return rows.Err()
}
func (r *ProductionRepository) analyticsProducts(ctx context.Context, out *app.ProductivityAnalytics, where string, args []any) error {
	rows, err := r.pool.Query(ctx, `SELECT coalesce(oi.product_id::text,''),oi.product_name,coalesce(oi.product_variant_id::text,''),coalesce(nullif(concat_ws(' · ',oi.size_name,oi.finish_name,oi.dimensions_snapshot),''),oi.sku_snapshot,''),count(*),count(*)FILTER(WHERE pr.status='completed'),count(*)FILTER(WHERE pr.status='failed'),coalesce(sum(pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status IN('completed','failed')),0),coalesce(sum(usage.consumed),0),coalesce(sum(`+materialCostSnapshot+`),0),percentile_cont(.5)WITHIN GROUP(ORDER BY pr.actual_minutes)FILTER(WHERE pr.actual_minutes>0 AND pr.status='completed')::float8`+analyticsFrom+where+` GROUP BY oi.product_id,oi.product_name,oi.product_variant_id,oi.sku_snapshot,oi.size_name,oi.finish_name,oi.dimensions_snapshot ORDER BY count(*) DESC LIMIT 100`, args...)
	if err != nil {
		return productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.ProductProductivity
		if err = rows.Scan(&v.ProductID, &v.ProductName, &v.VariantID, &v.VariantName, &v.TotalRuns, &v.CompletedRuns, &v.FailedRuns, &v.RecordedMinutes, &v.ConsumedGrams, &v.MaterialCostCents, &v.MedianMinutes); err != nil {
			return err
		}
		out.Products = append(out.Products, v)
	}
	return rows.Err()
}
func (r *ProductionRepository) analyticsMaterials(ctx context.Context, out *app.ProductivityAnalytics, where string, args []any) error {
	rows, err := r.pool.Query(ctx, `SELECT pj.material_id,m.name,coalesce(sum(usage.consumed),0),coalesce(sum(usage.wasted),0),coalesce(sum(`+materialCostSnapshot+`),0)`+analyticsFrom+where+` GROUP BY pj.material_id,m.name ORDER BY 3 DESC`, args...)
	if err != nil {
		return productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.MaterialProductivity
		if err = rows.Scan(&v.MaterialID, &v.MaterialName, &v.ConsumedGrams, &v.WastedGrams, &v.MaterialCostCents); err != nil {
			return err
		}
		out.Materials = append(out.Materials, v)
	}
	return rows.Err()
}
func (r *ProductionRepository) analyticsTime(ctx context.Context, out *app.ProductivityAnalytics, where string, args []any) error {
	where += ` AND pr.status='completed' AND pr.estimated_minutes>0 AND pr.actual_minutes>0`
	rows, err := r.pool.Query(ctx, `SELECT coalesce(oi.product_id::text,''),oi.product_name,coalesce(oi.product_variant_id::text,''),coalesce(nullif(concat_ws(' · ',oi.size_name,oi.finish_name,oi.dimensions_snapshot),''),oi.sku_snapshot,''),count(*),percentile_cont(.5)WITHIN GROUP(ORDER BY pr.estimated_minutes)::float8,percentile_cont(.5)WITHIN GROUP(ORDER BY pr.actual_minutes)::float8,percentile_cont(.5)WITHIN GROUP(ORDER BY pr.actual_minutes-pr.estimated_minutes)::float8,percentile_cont(.5)WITHIN GROUP(ORDER BY ((pr.actual_minutes-pr.estimated_minutes)::numeric/pr.estimated_minutes)*100)::float8`+analyticsFrom+where+` GROUP BY oi.product_id,oi.product_name,oi.product_variant_id,oi.sku_snapshot,oi.size_name,oi.finish_name,oi.dimensions_snapshot ORDER BY count(*) DESC LIMIT 100`, args...)
	if err != nil {
		return productionError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v app.TimeAccuracyPoint
		if err = rows.Scan(&v.ProductID, &v.ProductName, &v.VariantID, &v.VariantName, &v.Samples, &v.MedianEstimatedMinutes, &v.MedianActualMinutes, &v.MedianDifferenceMinutes, &v.MedianDifferencePercent); err != nil {
			return err
		}
		out.TimeAccuracy = append(out.TimeAccuracy, v)
	}
	return rows.Err()
}
