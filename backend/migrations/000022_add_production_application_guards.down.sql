DROP INDEX IF EXISTS material_spool_adjustments_spool_created_idx;
DROP FUNCTION IF EXISTS create_production_jobs_for_paid_order(UUID);
DROP TRIGGER IF EXISTS material_spool_adjustments_append_only_trigger ON material_spool_adjustments;
DROP FUNCTION IF EXISTS prevent_material_spool_adjustment_mutation();
DROP TABLE IF EXISTS material_spool_adjustments;
DROP INDEX IF EXISTS production_jobs_initial_order_item_key;
UPDATE production_jobs pj SET parent_job_id = NULL, updated_at = now()
  FROM production_job_legacy_links legacy WHERE legacy.job_id = pj.id;
DROP TABLE IF EXISTS production_job_legacy_links;

