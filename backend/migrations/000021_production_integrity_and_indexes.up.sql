DROP TRIGGER IF EXISTS production_jobs_payment_trigger ON production_jobs;
DROP FUNCTION IF EXISTS ensure_production_payment_confirmed();

CREATE FUNCTION enforce_production_job_transition()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    related_order RECORD;
    related_payment RECORD;
    actor_id UUID;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.job_number IS NULL OR length(btrim(NEW.job_number)) = 0 THEN
            NEW.job_number := 'PJ-' || upper(substr(replace(NEW.id::text, '-', ''), 1, 16));
        END IF;
        IF NEW.status <> 'awaiting_production' THEN
            RAISE EXCEPTION 'new production jobs must start as awaiting_production';
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.status IS NOT DISTINCT FROM OLD.status THEN
        RETURN NEW;
    END IF;

    BEGIN
        actor_id := nullif(current_setting('app.production_admin_id', true), '')::uuid;
    EXCEPTION WHEN invalid_text_representation THEN
        actor_id := NULL;
    END;
    IF actor_id IS NULL OR NOT EXISTS (
        SELECT 1 FROM admin_users
        WHERE id = actor_id AND status = 'active'
          AND role IN ('admin', 'production_operator', 'production_manager')
    ) THEN
        RAISE EXCEPTION 'an active production administrator is required for transitions';
    END IF;

    IF NOT (CASE OLD.status
        WHEN 'awaiting_production' THEN NEW.status IN ('queued', 'cancelled')
        WHEN 'queued' THEN NEW.status IN ('preparing', 'failed', 'cancelled')
        WHEN 'preparing' THEN NEW.status IN ('queued', 'printing', 'failed', 'cancelled')
        WHEN 'printing' THEN NEW.status IN ('post_processing', 'quality_check', 'failed', 'cancelled')
        WHEN 'post_processing' THEN NEW.status IN ('quality_check', 'failed', 'cancelled')
        WHEN 'quality_check' THEN NEW.status IN ('queued', 'completed', 'failed', 'cancelled')
        ELSE FALSE
    END) THEN
        RAISE EXCEPTION 'invalid production job transition: % -> %', OLD.status, NEW.status;
    END IF;

    IF NEW.status = 'queued' THEN
        SELECT o.id, o.status, o.currency, o.total_cents
          INTO related_order
          FROM order_items oi JOIN orders o ON o.id = oi.order_id
         WHERE oi.id = NEW.order_item_id
         FOR UPDATE OF o;

        IF related_order.id IS NULL OR related_order.status NOT IN ('confirmed', 'in_production') THEN
            RAISE EXCEPTION 'production queue requires a financially confirmed order';
        END IF;

        SELECT p.status, p.amount_cents, p.paid_amount_cents, p.refunded_amount_cents, p.currency
          INTO related_payment
          FROM payments p
         WHERE p.order_id = related_order.id AND p.status = 'paid'
         FOR UPDATE;

        IF related_payment.status IS NULL
           OR related_payment.amount_cents <> related_order.total_cents
           OR related_payment.paid_amount_cents <> related_order.total_cents
           OR related_payment.refunded_amount_cents <> 0
           OR related_payment.currency <> related_order.currency THEN
            RAISE EXCEPTION 'production queue requires full non-refunded settlement in order currency';
        END IF;
    END IF;

    IF NEW.status = 'completed' AND NOT EXISTS (
        SELECT 1
          FROM production_runs pr
         WHERE pr.production_job_id = NEW.id
           AND pr.status = 'completed'
           AND EXISTS (
               SELECT 1 FROM quality_inspections qi
                WHERE qi.production_run_id = pr.id AND qi.result = 'approved'
           )
    ) THEN
        RAISE EXCEPTION 'production job completion requires a completed approved run';
    END IF;

    NEW.version := OLD.version + 1;
    NEW.updated_at := now();
    IF NEW.status = 'printing' AND NEW.started_at IS NULL THEN NEW.started_at := now(); END IF;
    IF NEW.status = 'completed' AND NEW.completed_at IS NULL THEN NEW.completed_at := now(); END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION append_production_job_transition_event()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    actor_id UUID := nullif(current_setting('app.production_admin_id', true), '')::uuid;
    event_description TEXT := nullif(current_setting('app.production_description', true), '');
BEGIN
    INSERT INTO production_events (
        production_job_id, event_type, previous_status, new_status,
        admin_user_id, description
    ) VALUES (
        NEW.id, 'status_transition', OLD.status, NEW.status, actor_id,
        coalesce(event_description, 'Transição de estado da produção.')
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER production_jobs_transition_guard_trigger
BEFORE INSERT OR UPDATE OF status ON production_jobs
FOR EACH ROW EXECUTE FUNCTION enforce_production_job_transition();

CREATE TRIGGER production_jobs_transition_event_trigger
AFTER UPDATE OF status ON production_jobs
FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION append_production_job_transition_event();

CREATE FUNCTION transition_production_job(
    target_job_id UUID,
    target_status TEXT,
    actor_admin_id UUID,
    transition_description TEXT,
    expected_version BIGINT
) RETURNS production_jobs LANGUAGE plpgsql AS $$
DECLARE changed production_jobs;
BEGIN
    IF length(btrim(coalesce(transition_description, ''))) = 0 THEN
        RAISE EXCEPTION 'transition description is required';
    END IF;
    PERFORM set_config('app.production_admin_id', actor_admin_id::text, true);
    PERFORM set_config('app.production_description', transition_description, true);
    UPDATE production_jobs
       SET status = target_status
     WHERE id = target_job_id AND version = expected_version
     RETURNING * INTO changed;
    IF changed.id IS NULL THEN
        RAISE EXCEPTION 'production job not found or version conflict';
    END IF;
    RETURN changed;
END;
$$;

CREATE FUNCTION enforce_production_run_integrity()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    job_quantity INTEGER;
    parent_run RECORD;
    latest_inspection TEXT;
    actor_id UUID;
BEGIN
    SELECT quantity INTO job_quantity FROM production_jobs WHERE id = NEW.production_job_id;
    IF NEW.quantity > job_quantity THEN
        RAISE EXCEPTION 'run quantity cannot exceed job quantity';
    END IF;

    IF TG_OP = 'INSERT' THEN
        IF NEW.attempt_number = 1 AND NEW.reprint_of_run_id IS NOT NULL THEN
            RAISE EXCEPTION 'first run cannot be a reprint';
        ELSIF NEW.attempt_number > 1 THEN
            IF NEW.reprint_of_run_id IS NULL THEN RAISE EXCEPTION 'reprint source is required'; END IF;
            SELECT id, production_job_id, attempt_number, status INTO parent_run
              FROM production_runs WHERE id = NEW.reprint_of_run_id FOR UPDATE;
            IF parent_run.id IS NULL OR parent_run.production_job_id <> NEW.production_job_id
               OR parent_run.attempt_number >= NEW.attempt_number THEN
                RAISE EXCEPTION 'invalid reprint source';
            END IF;
            SELECT result INTO latest_inspection FROM quality_inspections
             WHERE production_run_id = parent_run.id ORDER BY inspected_at DESC, created_at DESC LIMIT 1;
            IF parent_run.status <> 'failed' AND latest_inspection IS DISTINCT FROM 'rejected' THEN
                RAISE EXCEPTION 'reprint requires a failed or quality-rejected run';
            END IF;
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.status IS NOT DISTINCT FROM OLD.status THEN RETURN NEW; END IF;
    actor_id := nullif(current_setting('app.production_admin_id', true), '')::uuid;
    IF actor_id IS NULL OR NOT EXISTS (
        SELECT 1 FROM admin_users
        WHERE id = actor_id AND status = 'active'
          AND role IN ('admin', 'production_operator', 'production_manager')
    ) THEN RAISE EXCEPTION 'an active production administrator is required for run transitions'; END IF;
    IF NOT (CASE OLD.status
        WHEN 'queued' THEN NEW.status IN ('preparing', 'cancelled')
        WHEN 'preparing' THEN NEW.status IN ('printing', 'failed', 'cancelled')
        WHEN 'printing' THEN NEW.status IN ('post_processing', 'quality_check', 'failed', 'cancelled')
        WHEN 'post_processing' THEN NEW.status IN ('quality_check', 'failed', 'cancelled')
        WHEN 'quality_check' THEN NEW.status IN ('completed', 'failed', 'cancelled')
        ELSE FALSE
    END) THEN RAISE EXCEPTION 'invalid production run transition: % -> %', OLD.status, NEW.status; END IF;
    IF NEW.status IN ('preparing', 'printing', 'post_processing', 'quality_check', 'completed')
       AND NEW.printer_id IS NULL THEN RAISE EXCEPTION 'run requires a printer'; END IF;
    IF NEW.status = 'completed' AND NOT EXISTS (
        SELECT 1 FROM quality_inspections WHERE production_run_id = NEW.id AND result = 'approved'
    ) THEN RAISE EXCEPTION 'run completion requires approved quality inspection'; END IF;
    NEW.version := OLD.version + 1;
    NEW.updated_at := now();
    IF NEW.status = 'printing' AND NEW.started_at IS NULL THEN NEW.started_at := now(); END IF;
    IF NEW.status = 'completed' AND NEW.completed_at IS NULL THEN NEW.completed_at := now(); END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION append_production_run_transition_event()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO production_events (
        production_job_id, production_run_id, event_type, previous_status,
        new_status, admin_user_id, description
    ) VALUES (
        NEW.production_job_id, NEW.id, 'status_transition', OLD.status, NEW.status,
        nullif(current_setting('app.production_admin_id', true), '')::uuid,
        coalesce(nullif(current_setting('app.production_description', true), ''), 'Transição de execução da produção.')
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER production_runs_integrity_trigger
BEFORE INSERT OR UPDATE OF status ON production_runs
FOR EACH ROW EXECUTE FUNCTION enforce_production_run_integrity();

CREATE TRIGGER production_runs_transition_event_trigger
AFTER UPDATE OF status ON production_runs
FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status)
EXECUTE FUNCTION append_production_run_transition_event();

CREATE FUNCTION transition_production_run(
    target_run_id UUID,
    target_status TEXT,
    actor_admin_id UUID,
    transition_description TEXT,
    expected_version BIGINT
) RETURNS production_runs LANGUAGE plpgsql AS $$
DECLARE changed production_runs;
BEGIN
    IF length(btrim(coalesce(transition_description, ''))) = 0 THEN RAISE EXCEPTION 'transition description is required'; END IF;
    PERFORM set_config('app.production_admin_id', actor_admin_id::text, true);
    PERFORM set_config('app.production_description', transition_description, true);
    UPDATE production_runs SET status = target_status
     WHERE id = target_run_id AND version = expected_version RETURNING * INTO changed;
    IF changed.id IS NULL THEN RAISE EXCEPTION 'production run not found or version conflict'; END IF;
    RETURN changed;
END;
$$;

CREATE FUNCTION enforce_quality_inspection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE run_job UUID;
BEGIN
    SELECT production_job_id INTO run_job FROM production_runs
     WHERE id = NEW.production_run_id AND status = 'quality_check' FOR UPDATE;
    IF run_job IS NULL THEN RAISE EXCEPTION 'quality inspection requires a run in quality_check'; END IF;
    INSERT INTO production_events (
        production_job_id, production_run_id, event_type, admin_user_id, description,
        metadata, occurred_at
    ) VALUES (
        run_job, NEW.production_run_id, 'quality_inspected', NEW.inspected_by_admin_user_id,
        'Inspeção de qualidade registrada.', jsonb_build_object('result', NEW.result), NEW.inspected_at
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER quality_inspections_event_trigger
AFTER INSERT ON quality_inspections
FOR EACH ROW EXECUTE FUNCTION enforce_quality_inspection();

CREATE FUNCTION apply_production_material_usage()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    spool RECORD;
    run_job UUID;
    debit_grams INTEGER := NEW.consumed_grams + NEW.wasted_grams;
BEGIN
    SELECT id, current_weight_grams, cost_per_kg_cents, status INTO spool
      FROM material_spools WHERE id = NEW.material_spool_id FOR UPDATE;
    IF spool.id IS NULL OR spool.status IN ('quarantined', 'discarded', 'depleted') THEN
        RAISE EXCEPTION 'material spool is not consumable';
    END IF;
    IF debit_grams > spool.current_weight_grams THEN RAISE EXCEPTION 'insufficient material spool balance'; END IF;
    NEW.cost_per_kg_snapshot_cents := spool.cost_per_kg_cents;
    NEW.total_cost_cents := ((debit_grams::bigint * spool.cost_per_kg_cents) + 500) / 1000;
    RETURN NEW;
END;
$$;

CREATE FUNCTION debit_production_material_usage()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE run_job UUID;
BEGIN
    UPDATE material_spools
       SET current_weight_grams = current_weight_grams - (NEW.consumed_grams + NEW.wasted_grams),
           status = CASE WHEN current_weight_grams - (NEW.consumed_grams + NEW.wasted_grams) = 0 THEN 'depleted' ELSE 'in_use' END,
           exhausted_at = CASE WHEN current_weight_grams - (NEW.consumed_grams + NEW.wasted_grams) = 0 THEN now() ELSE exhausted_at END,
           version = version + 1, updated_at = now()
     WHERE id = NEW.material_spool_id;
    UPDATE production_runs SET material_cost_cents = material_cost_cents + NEW.total_cost_cents,
           updated_at = now() WHERE id = NEW.production_run_id
     RETURNING production_job_id INTO run_job;
    INSERT INTO production_events (
        production_job_id, production_run_id, event_type, admin_user_id, description, metadata
    ) VALUES (
        run_job, NEW.production_run_id, 'material_consumed', NEW.recorded_by_admin_user_id,
        'Consumo de filamento registrado.',
        jsonb_build_object('spool_id', NEW.material_spool_id, 'consumed_grams', NEW.consumed_grams,
                           'wasted_grams', NEW.wasted_grams, 'cost_cents', NEW.total_cost_cents)
    );
    RETURN NEW;
END;
$$;

CREATE FUNCTION prevent_production_material_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'production material usage is immutable'; END;
$$;

CREATE TRIGGER production_run_materials_cost_trigger
BEFORE INSERT ON production_run_materials
FOR EACH ROW EXECUTE FUNCTION apply_production_material_usage();
CREATE TRIGGER production_run_materials_debit_trigger
AFTER INSERT ON production_run_materials
FOR EACH ROW EXECUTE FUNCTION debit_production_material_usage();
CREATE TRIGGER production_run_materials_immutable_trigger
BEFORE UPDATE OR DELETE ON production_run_materials
FOR EACH ROW EXECUTE FUNCTION prevent_production_material_mutation();

CREATE UNIQUE INDEX quality_inspections_one_approved_run_idx
    ON quality_inspections(production_run_id) WHERE result = 'approved';
CREATE INDEX printers_status_idx ON printers(status, name);
CREATE INDEX printer_maintenance_due_idx ON printer_maintenance_events(status, scheduled_at);
CREATE INDEX material_spools_lookup_idx ON material_spools(material_id, color_id, status);
CREATE INDEX material_spools_low_stock_idx ON material_spools(current_weight_grams)
    WHERE status IN ('available', 'in_use');
CREATE INDEX production_jobs_queue_idx ON production_jobs(priority DESC, due_at, created_at)
    WHERE status IN ('awaiting_production', 'queued', 'preparing');
CREATE INDEX production_jobs_parent_idx ON production_jobs(parent_job_id) WHERE parent_job_id IS NOT NULL;
CREATE INDEX production_runs_job_created_idx ON production_runs(production_job_id, attempt_number DESC);
CREATE INDEX production_runs_printer_status_idx ON production_runs(printer_id, status, scheduled_at);
CREATE INDEX production_events_job_occurred_idx ON production_events(production_job_id, occurred_at DESC);
CREATE INDEX production_events_run_occurred_idx ON production_events(production_run_id, occurred_at DESC)
    WHERE production_run_id IS NOT NULL;
CREATE INDEX quality_inspections_run_idx ON quality_inspections(production_run_id, inspected_at DESC);

