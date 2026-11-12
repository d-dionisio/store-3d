DROP INDEX IF EXISTS quality_inspections_run_idx;
DROP INDEX IF EXISTS production_events_run_occurred_idx;
DROP INDEX IF EXISTS production_events_job_occurred_idx;
DROP INDEX IF EXISTS production_runs_printer_status_idx;
DROP INDEX IF EXISTS production_runs_job_created_idx;
DROP INDEX IF EXISTS production_jobs_parent_idx;
DROP INDEX IF EXISTS production_jobs_queue_idx;
DROP INDEX IF EXISTS material_spools_low_stock_idx;
DROP INDEX IF EXISTS material_spools_lookup_idx;
DROP INDEX IF EXISTS printer_maintenance_due_idx;
DROP INDEX IF EXISTS printers_status_idx;
DROP INDEX IF EXISTS quality_inspections_one_approved_run_idx;

DROP TRIGGER IF EXISTS production_run_materials_immutable_trigger ON production_run_materials;
DROP TRIGGER IF EXISTS production_run_materials_debit_trigger ON production_run_materials;
DROP TRIGGER IF EXISTS production_run_materials_cost_trigger ON production_run_materials;
DROP FUNCTION IF EXISTS prevent_production_material_mutation();
DROP FUNCTION IF EXISTS debit_production_material_usage();
DROP FUNCTION IF EXISTS apply_production_material_usage();
DROP TRIGGER IF EXISTS quality_inspections_event_trigger ON quality_inspections;
DROP FUNCTION IF EXISTS enforce_quality_inspection();
DROP FUNCTION IF EXISTS transition_production_run(UUID, TEXT, UUID, TEXT, BIGINT);
DROP TRIGGER IF EXISTS production_runs_transition_event_trigger ON production_runs;
DROP TRIGGER IF EXISTS production_runs_integrity_trigger ON production_runs;
DROP FUNCTION IF EXISTS append_production_run_transition_event();
DROP FUNCTION IF EXISTS enforce_production_run_integrity();
DROP FUNCTION IF EXISTS transition_production_job(UUID, TEXT, UUID, TEXT, BIGINT);
DROP TRIGGER IF EXISTS production_jobs_transition_event_trigger ON production_jobs;
DROP TRIGGER IF EXISTS production_jobs_transition_guard_trigger ON production_jobs;
DROP FUNCTION IF EXISTS append_production_job_transition_event();
DROP FUNCTION IF EXISTS enforce_production_job_transition();

CREATE OR REPLACE FUNCTION ensure_production_payment_confirmed()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE related_order_id UUID;
BEGIN
    SELECT order_id INTO related_order_id FROM order_items WHERE id = NEW.order_item_id;
    IF related_order_id IS NULL THEN RAISE EXCEPTION 'order item % does not exist', NEW.order_item_id; END IF;
    IF NOT EXISTS (
        SELECT 1 FROM payments WHERE order_id = related_order_id
          AND status IN ('paid', 'partially_refunded') AND paid_amount_cents > refunded_amount_cents
    ) THEN RAISE EXCEPTION 'production requires a confirmed payment for order %', related_order_id; END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_jobs_payment_trigger
BEFORE INSERT OR UPDATE OF order_item_id ON production_jobs
FOR EACH ROW EXECUTE FUNCTION ensure_production_payment_confirmed();

