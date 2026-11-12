DROP TRIGGER IF EXISTS production_events_append_only_trigger ON production_events;
DROP FUNCTION IF EXISTS prevent_production_event_mutation();
DROP TABLE IF EXISTS quality_inspections;
DROP TABLE IF EXISTS production_events;
DROP TABLE IF EXISTS production_run_materials;
DROP TABLE IF EXISTS production_runs;

ALTER TABLE production_jobs DROP CONSTRAINT production_jobs_status_check;
UPDATE production_jobs SET status = CASE
    WHEN status = 'awaiting_production' THEN 'pending'
    WHEN status = 'preparing' THEN 'queued'
    ELSE status
END;
ALTER TABLE production_jobs
    ALTER COLUMN status SET DEFAULT 'pending',
    ADD CONSTRAINT production_jobs_status_check CHECK (status IN (
        'pending', 'queued', 'printing', 'post_processing', 'quality_check',
        'completed', 'failed', 'cancelled'
    )),
    DROP CONSTRAINT production_jobs_parent_not_self_check,
    DROP CONSTRAINT production_jobs_job_number_key,
    DROP COLUMN version,
    DROP COLUMN due_at,
    DROP COLUMN priority,
    DROP COLUMN parent_job_id,
    DROP COLUMN job_number;

