ALTER TABLE admin_users DROP CONSTRAINT admin_users_role_check;
ALTER TABLE admin_users ADD CONSTRAINT admin_users_role_check
    CHECK (role IN ('quote_viewer', 'admin', 'production_operator', 'production_manager'));

CREATE TABLE printers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    manufacturer TEXT NOT NULL,
    model TEXT NOT NULL,
    serial_number TEXT UNIQUE,
    technology TEXT NOT NULL DEFAULT 'fdm' CHECK (technology IN ('fdm')),
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN (
        'available', 'busy', 'maintenance', 'offline', 'retired'
    )),
    build_width_mm INTEGER NOT NULL CHECK (build_width_mm > 0),
    build_depth_mm INTEGER NOT NULL CHECK (build_depth_mm > 0),
    build_height_mm INTEGER NOT NULL CHECK (build_height_mm > 0),
    nozzle_diameter_microns INTEGER NOT NULL DEFAULT 400 CHECK (nozzle_diameter_microns > 0),
    notes TEXT,
    retired_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(code)) > 0),
    CHECK (length(btrim(name)) > 0),
    CHECK (length(btrim(manufacturer)) > 0),
    CHECK (length(btrim(model)) > 0),
    CHECK ((status = 'retired') = (retired_at IS NOT NULL))
);

CREATE TABLE printer_maintenance_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'preventive', 'corrective', 'calibration', 'inspection', 'part_replacement', 'other'
    )),
    status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN (
        'scheduled', 'in_progress', 'completed', 'cancelled'
    )),
    description TEXT NOT NULL,
    performed_by_admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    next_due_at TIMESTAMPTZ,
    cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (cost_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(description)) > 0),
    CHECK (status <> 'in_progress' OR started_at IS NOT NULL),
    CHECK (status <> 'completed' OR completed_at IS NOT NULL)
);

CREATE TABLE material_spools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    spool_code TEXT NOT NULL UNIQUE,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    color_id UUID NOT NULL REFERENCES colors(id) ON DELETE RESTRICT,
    manufacturer TEXT NOT NULL,
    product_line TEXT,
    lot_number TEXT,
    status TEXT NOT NULL DEFAULT 'sealed' CHECK (status IN (
        'sealed', 'available', 'in_use', 'depleted', 'quarantined', 'discarded'
    )),
    initial_weight_grams INTEGER NOT NULL CHECK (initial_weight_grams > 0),
    current_weight_grams INTEGER NOT NULL CHECK (current_weight_grams >= 0),
    minimum_stock_grams INTEGER NOT NULL DEFAULT 0 CHECK (minimum_stock_grams >= 0),
    cost_per_kg_cents BIGINT NOT NULL CHECK (cost_per_kg_cents >= 0),
    opened_at TIMESTAMPTZ,
    exhausted_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(spool_code)) > 0),
    CHECK (length(btrim(manufacturer)) > 0),
    CHECK (current_weight_grams <= initial_weight_grams),
    CHECK ((status = 'depleted') = (current_weight_grams = 0)),
    CHECK (exhausted_at IS NULL OR current_weight_grams = 0)
);

