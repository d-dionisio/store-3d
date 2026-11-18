-- Cost profiles are append-only. The separate current pointer can change while
-- every previously used configuration remains addressable by its version.
CREATE TABLE production_cost_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL CHECK (version > 0),
    electricity_rate_cents_per_kwh BIGINT NOT NULL CHECK (electricity_rate_cents_per_kwh >= 0),
    printer_power_milliwatts BIGINT CHECK (printer_power_milliwatts > 0),
    power_measurement_note TEXT,
    machine_acquisition_cents BIGINT NOT NULL CHECK (machine_acquisition_cents >= 0),
    machine_useful_life_seconds BIGINT NOT NULL CHECK (machine_useful_life_seconds > 0),
    maintenance_rate_cents_per_hour BIGINT NOT NULL CHECK (maintenance_rate_cents_per_hour >= 0),
    labor_rate_cents_per_hour BIGINT NOT NULL CHECK (labor_rate_cents_per_hour >= 0),
    preparation_seconds BIGINT NOT NULL CHECK (preparation_seconds >= 0),
    finishing_seconds BIGINT NOT NULL CHECK (finishing_seconds >= 0),
    notes TEXT,
    created_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (printer_id, version),
    UNIQUE (id, printer_id),
    UNIQUE (id, printer_id, version),
    CHECK (printer_power_milliwatts IS NOT NULL OR nullif(btrim(power_measurement_note), '') IS NOT NULL)
);

CREATE TABLE production_cost_profile_current (
    printer_id UUID PRIMARY KEY REFERENCES printers(id) ON DELETE RESTRICT,
    profile_id UUID NOT NULL UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (profile_id, printer_id) REFERENCES production_cost_profiles(id, printer_id) ON DELETE RESTRICT
);

CREATE TABLE production_run_cost_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    production_run_id UUID NOT NULL REFERENCES production_runs(id) ON DELETE RESTRICT,
    snapshot_type TEXT NOT NULL CHECK (snapshot_type IN ('estimate', 'actual')),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
    cost_profile_id UUID NOT NULL,
    cost_profile_version BIGINT NOT NULL CHECK (cost_profile_version > 0),
    energy_source TEXT NOT NULL CHECK (energy_source IN ('measured', 'configured_power')),
    energy_milliwh BIGINT NOT NULL CHECK (energy_milliwh >= 0),
    run_seconds BIGINT NOT NULL CHECK (run_seconds >= 0),
    preparation_seconds BIGINT NOT NULL CHECK (preparation_seconds >= 0),
    finishing_seconds BIGINT NOT NULL CHECK (finishing_seconds >= 0),
    consumed_grams BIGINT NOT NULL CHECK (consumed_grams >= 0),
    wasted_grams BIGINT NOT NULL CHECK (wasted_grams >= 0),
    material_consumed_cost_cents BIGINT NOT NULL CHECK (material_consumed_cost_cents >= 0),
    material_waste_cost_cents BIGINT NOT NULL CHECK (material_waste_cost_cents >= 0),
    material_total_cost_cents BIGINT NOT NULL CHECK (material_total_cost_cents >= 0),
    electricity_rate_cents_per_kwh BIGINT NOT NULL CHECK (electricity_rate_cents_per_kwh >= 0),
    energy_cost_cents BIGINT NOT NULL CHECK (energy_cost_cents >= 0),
    depreciation_cost_cents BIGINT NOT NULL CHECK (depreciation_cost_cents >= 0),
    maintenance_cost_cents BIGINT NOT NULL CHECK (maintenance_cost_cents >= 0),
    preparation_cost_cents BIGINT NOT NULL CHECK (preparation_cost_cents >= 0),
    finishing_cost_cents BIGINT NOT NULL CHECK (finishing_cost_cents >= 0),
    labor_cost_cents BIGINT NOT NULL CHECK (labor_cost_cents >= 0),
    total_cost_cents BIGINT NOT NULL CHECK (total_cost_cents >= 0),
    unit_cost_cents BIGINT NOT NULL CHECK (unit_cost_cents >= 0),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    revenue_snapshot_cents BIGINT NOT NULL CHECK (revenue_snapshot_cents >= 0),
    contribution_margin_cents BIGINT NOT NULL,
    contribution_margin_basis_points BIGINT,
    profit_cents BIGINT NOT NULL,
    profit_margin_basis_points BIGINT,
    material_breakdown JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(material_breakdown) = 'array'),
    created_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (production_run_id, snapshot_type),
    FOREIGN KEY (cost_profile_id, printer_id, cost_profile_version) REFERENCES production_cost_profiles(id, printer_id, version) ON DELETE RESTRICT,
    CHECK (material_total_cost_cents = material_consumed_cost_cents + material_waste_cost_cents),
    CHECK (labor_cost_cents = preparation_cost_cents + finishing_cost_cents),
    CHECK (total_cost_cents = material_total_cost_cents + energy_cost_cents + depreciation_cost_cents + maintenance_cost_cents + labor_cost_cents),
    CHECK (unit_cost_cents = round(total_cost_cents::numeric / quantity)::bigint),
    CHECK (profit_cents = revenue_snapshot_cents - total_cost_cents),
    CHECK (contribution_margin_cents = revenue_snapshot_cents - material_total_cost_cents - energy_cost_cents - maintenance_cost_cents - labor_cost_cents),
    CHECK ((revenue_snapshot_cents = 0 AND contribution_margin_basis_points IS NULL AND profit_margin_basis_points IS NULL) OR
           (revenue_snapshot_cents > 0 AND contribution_margin_basis_points = round(contribution_margin_cents::numeric * 10000 / revenue_snapshot_cents)::bigint AND
            profit_margin_basis_points = round(profit_cents::numeric * 10000 / revenue_snapshot_cents)::bigint))
);

CREATE FUNCTION prevent_production_cost_history_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'production cost history is append-only'; END;
$$;
CREATE TRIGGER production_cost_profiles_append_only
BEFORE UPDATE OR DELETE ON production_cost_profiles
FOR EACH ROW EXECUTE FUNCTION prevent_production_cost_history_mutation();
CREATE TRIGGER production_run_cost_snapshots_append_only
BEFORE UPDATE OR DELETE ON production_run_cost_snapshots
FOR EACH ROW EXECUTE FUNCTION prevent_production_cost_history_mutation();

CREATE FUNCTION validate_production_cost_snapshot()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE target_run production_runs%ROWTYPE;
        target_profile production_cost_profiles%ROWTYPE;
BEGIN
    SELECT * INTO target_run FROM production_runs WHERE id = NEW.production_run_id FOR UPDATE;
    SELECT * INTO target_profile FROM production_cost_profiles WHERE id = NEW.cost_profile_id;
    IF target_run.id IS NULL OR target_run.printer_id IS DISTINCT FROM NEW.printer_id THEN
        RAISE EXCEPTION 'cost snapshot printer does not match production run';
    END IF;
    IF target_profile.id IS NULL OR target_profile.printer_id IS DISTINCT FROM NEW.printer_id OR
       target_profile.version <> NEW.cost_profile_version OR
       target_profile.electricity_rate_cents_per_kwh <> NEW.electricity_rate_cents_per_kwh OR
       (NEW.energy_source = 'configured_power' AND target_profile.printer_power_milliwatts IS NULL) THEN
        RAISE EXCEPTION 'cost snapshot does not match its immutable profile';
    END IF;
    IF NEW.snapshot_type = 'actual' AND (
        target_run.status NOT IN ('completed','failed','cancelled') OR
        (target_run.started_at IS NULL AND target_run.actual_minutes IS NULL) OR
        (target_run.status = 'completed' AND target_run.completed_at IS NULL)
    ) THEN
        RAISE EXCEPTION 'actual cost snapshot requires a terminal run with recorded execution';
    END IF;
    IF NEW.snapshot_type = 'estimate' AND target_run.status IN ('completed','failed','cancelled') THEN
        RAISE EXCEPTION 'estimate snapshot is not valid for a closed production run';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER production_run_cost_snapshot_validate
BEFORE INSERT ON production_run_cost_snapshots
FOR EACH ROW EXECUTE FUNCTION validate_production_cost_snapshot();

