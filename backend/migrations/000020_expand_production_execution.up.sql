ALTER TABLE production_jobs DROP CONSTRAINT production_jobs_status_check;
ALTER TABLE production_jobs
    ADD COLUMN job_number TEXT,
    ADD COLUMN parent_job_id UUID REFERENCES production_jobs(id) ON DELETE RESTRICT,
    ADD COLUMN priority SMALLINT NOT NULL DEFAULT 50 CHECK (priority BETWEEN 0 AND 100),
    ADD COLUMN due_at TIMESTAMPTZ,
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);

UPDATE production_jobs
SET job_number = 'PJ-' || upper(substr(replace(id::text, '-', ''), 1, 16)),
    status = CASE status WHEN 'pending' THEN 'awaiting_production' ELSE status END;

ALTER TABLE production_jobs
    ALTER COLUMN status SET DEFAULT 'awaiting_production',
    ALTER COLUMN job_number SET NOT NULL,
    ADD CONSTRAINT production_jobs_job_number_key UNIQUE (job_number),
    ADD CONSTRAINT production_jobs_parent_not_self_check CHECK (parent_job_id IS NULL OR parent_job_id <> id),
    ADD CONSTRAINT production_jobs_status_check CHECK (status IN (
        'awaiting_production', 'queued', 'preparing', 'printing', 'post_processing',
        'quality_check', 'completed', 'failed', 'cancelled'
    ));

CREATE TABLE production_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_job_id UUID NOT NULL REFERENCES production_jobs(id) ON DELETE RESTRICT,
    reprint_of_run_id UUID REFERENCES production_runs(id) ON DELETE RESTRICT,
    printer_id UUID REFERENCES printers(id) ON DELETE RESTRICT,
    printer_reference_snapshot TEXT,
    attempt_number INTEGER NOT NULL CHECK (attempt_number > 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN (
        'queued', 'preparing', 'printing', 'post_processing', 'quality_check',
        'completed', 'failed', 'cancelled'
    )),
    estimated_minutes INTEGER CHECK (estimated_minutes > 0),
    actual_minutes INTEGER CHECK (actual_minutes >= 0),
    estimated_weight_grams INTEGER CHECK (estimated_weight_grams > 0),
    actual_weight_grams INTEGER CHECK (actual_weight_grams >= 0),
    material_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (material_cost_cents >= 0),
    energy_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (energy_cost_cents >= 0),
    other_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (other_cost_cents >= 0),
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_by_admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT production_runs_job_attempt_key UNIQUE (production_job_id, attempt_number),
    CONSTRAINT production_runs_id_job_key UNIQUE (id, production_job_id),
    CHECK (reprint_of_run_id IS NULL OR reprint_of_run_id <> id),
    CHECK (status NOT IN ('printing', 'post_processing', 'quality_check', 'completed') OR started_at IS NOT NULL),
    CHECK (status <> 'completed' OR completed_at IS NOT NULL),
    CHECK (status <> 'failed' OR length(btrim(failure_reason)) > 0)
);

CREATE TABLE production_run_materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_run_id UUID NOT NULL REFERENCES production_runs(id) ON DELETE RESTRICT,
    material_spool_id UUID NOT NULL REFERENCES material_spools(id) ON DELETE RESTRICT,
    reserved_grams INTEGER NOT NULL DEFAULT 0 CHECK (reserved_grams >= 0),
    consumed_grams INTEGER NOT NULL DEFAULT 0 CHECK (consumed_grams >= 0),
    wasted_grams INTEGER NOT NULL DEFAULT 0 CHECK (wasted_grams >= 0),
    cost_per_kg_snapshot_cents BIGINT NOT NULL CHECK (cost_per_kg_snapshot_cents >= 0),
    total_cost_cents BIGINT NOT NULL CHECK (total_cost_cents >= 0),
    recorded_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT production_run_materials_run_spool_key UNIQUE (production_run_id, material_spool_id),
    CHECK (reserved_grams + consumed_grams + wasted_grams > 0)
);

CREATE TABLE production_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_job_id UUID NOT NULL REFERENCES production_jobs(id) ON DELETE RESTRICT,
    production_run_id UUID,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'migration', 'status_transition', 'material_consumed', 'quality_inspected',
        'reprint_created', 'note'
    )),
    previous_status TEXT,
    new_status TEXT,
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    description TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT production_events_run_job_fk FOREIGN KEY (production_run_id, production_job_id)
        REFERENCES production_runs(id, production_job_id) ON DELETE RESTRICT,
    CHECK (length(btrim(description)) > 0),
    CHECK ((event_type = 'status_transition') = (previous_status IS NOT NULL AND new_status IS NOT NULL))
);

CREATE TABLE quality_inspections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_run_id UUID NOT NULL REFERENCES production_runs(id) ON DELETE RESTRICT,
    result TEXT NOT NULL CHECK (result IN ('approved', 'rejected', 'conditional')),
    dimensional_accuracy_ok BOOLEAN NOT NULL,
    visual_quality_ok BOOLEAN NOT NULL,
    functional_test_ok BOOLEAN,
    notes TEXT,
    inspected_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    inspected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (result <> 'approved' OR (dimensional_accuracy_ok AND visual_quality_ok AND functional_test_ok IS NOT FALSE)),
    CHECK (result <> 'rejected' OR length(btrim(coalesce(notes, ''))) > 0)
);

INSERT INTO production_runs (
    production_job_id, printer_reference_snapshot, attempt_number, quantity, status,
    estimated_minutes, actual_minutes, estimated_weight_grams, actual_weight_grams,
    scheduled_at, started_at, completed_at, failure_reason, created_at, updated_at
)
SELECT id, printer_reference, 1, quantity,
       CASE status WHEN 'awaiting_production' THEN 'queued' ELSE status END,
       estimated_minutes, actual_minutes, material_estimated_grams, material_actual_grams,
       scheduled_at,
       CASE WHEN status = 'completed' THEN coalesce(started_at, created_at) ELSE started_at END,
       completed_at, failure_reason, created_at, updated_at
FROM production_jobs;

INSERT INTO production_events (
    production_job_id, production_run_id, event_type, description, metadata, occurred_at, created_at
)
SELECT pj.id, pr.id, 'migration',
       'Registro legado migrado para o modelo de execuções de produção.',
       jsonb_build_object('legacy_status', CASE pj.status WHEN 'awaiting_production' THEN 'pending' ELSE pj.status END),
       pj.created_at, now()
FROM production_jobs pj
JOIN production_runs pr ON pr.production_job_id = pj.id AND pr.attempt_number = 1;

CREATE FUNCTION prevent_production_event_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'production events are append-only';
END;
$$;

CREATE TRIGGER production_events_append_only_trigger
BEFORE UPDATE OR DELETE ON production_events
FOR EACH ROW EXECUTE FUNCTION prevent_production_event_mutation();

