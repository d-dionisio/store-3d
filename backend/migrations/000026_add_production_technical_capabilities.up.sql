CREATE TABLE printer_technical_specs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
    version BIGINT NOT NULL CHECK (version > 0),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','verified','superseded')),
    technology TEXT NOT NULL CHECK (technology IN ('fdm')),
    max_width_mm INTEGER CHECK (max_width_mm > 0),
    max_depth_mm INTEGER CHECK (max_depth_mm > 0),
    max_height_mm INTEGER CHECK (max_height_mm > 0),
    max_hotend_temp_c INTEGER CHECK (max_hotend_temp_c > 0),
    max_bed_temp_c INTEGER CHECK (max_bed_temp_c >= 0),
    enclosed_chamber BOOLEAN,
    technical_notes TEXT,
    source_reference TEXT,
    source_verified_at TIMESTAMPTZ,
    created_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (printer_id, version),
    CHECK ((status <> 'verified') OR (source_reference IS NOT NULL AND source_verified_at IS NOT NULL)),
    CHECK ((max_width_mm IS NULL) = (max_depth_mm IS NULL) AND (max_width_mm IS NULL) = (max_height_mm IS NULL))
);
CREATE UNIQUE INDEX printer_technical_specs_current_idx ON printer_technical_specs(printer_id) WHERE status IN ('draft','verified');

CREATE TABLE printer_supported_nozzles (
    printer_spec_id UUID NOT NULL REFERENCES printer_technical_specs(id) ON DELETE CASCADE,
    diameter_microns INTEGER NOT NULL CHECK (diameter_microns > 0),
    nozzle_type TEXT NOT NULL CHECK (length(btrim(nozzle_type)) > 0),
    PRIMARY KEY (printer_spec_id, diameter_microns, nozzle_type)
);
CREATE TABLE printer_supported_materials (
    printer_spec_id UUID NOT NULL REFERENCES printer_technical_specs(id) ON DELETE CASCADE,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    caveat TEXT,
    PRIMARY KEY (printer_spec_id, material_id)
);
CREATE TABLE printer_build_surfaces (
    printer_spec_id UUID NOT NULL REFERENCES printer_technical_specs(id) ON DELETE CASCADE,
    surface TEXT NOT NULL CHECK (length(btrim(surface)) > 0),
    PRIMARY KEY (printer_spec_id, surface)
);
CREATE TABLE printer_features (
    printer_spec_id UUID NOT NULL REFERENCES printer_technical_specs(id) ON DELETE CASCADE,
    feature TEXT NOT NULL CHECK (length(btrim(feature)) > 0),
    PRIMARY KEY (printer_spec_id, feature)
);

CREATE TABLE printer_operational_configs (
    printer_id UUID PRIMARY KEY REFERENCES printers(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','configured')),
    installed_nozzle_microns INTEGER CHECK (installed_nozzle_microns > 0),
    installed_nozzle_type TEXT,
    installed_surface TEXT,
    material_slot_count INTEGER CHECK (material_slot_count > 0),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status <> 'configured' OR (installed_nozzle_microns IS NOT NULL AND installed_nozzle_type IS NOT NULL AND material_slot_count IS NOT NULL))
);

CREATE TABLE material_technical_specs (
    material_id UUID PRIMARY KEY REFERENCES materials(id) ON DELETE RESTRICT,
    manufacturer TEXT,
    commercial_name TEXT,
    polymer_type TEXT,
    print_temp_min_c INTEGER CHECK (print_temp_min_c > 0),
    print_temp_max_c INTEGER CHECK (print_temp_max_c > 0),
    print_temp_recommended_c INTEGER CHECK (print_temp_recommended_c > 0),
    bed_temp_min_c INTEGER CHECK (bed_temp_min_c >= 0),
    bed_temp_max_c INTEGER CHECK (bed_temp_max_c >= 0),
    bed_temp_recommended_c INTEGER CHECK (bed_temp_recommended_c >= 0),
    recommended_speed_mm_s INTEGER CHECK (recommended_speed_mm_s > 0),
    requires_enclosure BOOLEAN,
    requires_drying BOOLEAN,
    moisture_sensitivity TEXT CHECK (moisture_sensitivity IN ('low','medium','high')),
    required_surface TEXT,
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','verified')),
    source_reference TEXT,
    source_verified_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    updated_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (print_temp_min_c IS NULL OR print_temp_max_c IS NULL OR print_temp_min_c <= print_temp_max_c),
    CHECK (print_temp_recommended_c IS NULL OR (print_temp_min_c IS NOT NULL AND print_temp_max_c IS NOT NULL AND print_temp_recommended_c BETWEEN print_temp_min_c AND print_temp_max_c)),
    CHECK (bed_temp_min_c IS NULL OR bed_temp_max_c IS NULL OR bed_temp_min_c <= bed_temp_max_c),
    CHECK (bed_temp_recommended_c IS NULL OR (bed_temp_min_c IS NOT NULL AND bed_temp_max_c IS NOT NULL AND bed_temp_recommended_c BETWEEN bed_temp_min_c AND bed_temp_max_c)),
    CHECK ((status <> 'verified') OR (source_reference IS NOT NULL AND source_verified_at IS NOT NULL))
);

CREATE TABLE spool_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE RESTRICT,
    material_spool_id UUID NOT NULL REFERENCES material_spools(id) ON DELETE RESTRICT,
    slot_number INTEGER NOT NULL CHECK (slot_number > 0),
    installed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    removed_at TIMESTAMPTZ,
    installed_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    removed_by_admin_user_id UUID REFERENCES admin_users(id) ON DELETE RESTRICT,
    removal_reason TEXT,
    CHECK (removed_at IS NULL OR removed_at >= installed_at),
    CHECK ((removed_at IS NULL) = (removed_by_admin_user_id IS NULL)),
    CHECK (removed_at IS NULL OR length(btrim(removal_reason)) >= 3)
);
CREATE UNIQUE INDEX spool_installations_active_spool_idx ON spool_installations(material_spool_id) WHERE removed_at IS NULL;
CREATE UNIQUE INDEX spool_installations_active_slot_idx ON spool_installations(printer_id,slot_number) WHERE removed_at IS NULL;
CREATE INDEX spool_installations_printer_history_idx ON spool_installations(printer_id,installed_at DESC);
CREATE INDEX spool_installations_spool_history_idx ON spool_installations(material_spool_id,installed_at DESC);

CREATE FUNCTION guard_spool_installation_history() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.removed_at IS NOT NULL THEN RAISE EXCEPTION 'removed spool installation is immutable'; END IF;
    IF NEW.printer_id <> OLD.printer_id OR NEW.material_spool_id <> OLD.material_spool_id OR NEW.slot_number <> OLD.slot_number
       OR NEW.installed_at <> OLD.installed_at OR NEW.installed_by_admin_user_id <> OLD.installed_by_admin_user_id THEN
        RAISE EXCEPTION 'spool installation identity is immutable';
    END IF;
    IF NEW.removed_at IS NULL THEN RAISE EXCEPTION 'only spool removal may update an installation'; END IF;
    RETURN NEW;
END; $$;
CREATE TRIGGER spool_installations_history_guard BEFORE UPDATE OR DELETE ON spool_installations
FOR EACH ROW EXECUTE FUNCTION guard_spool_installation_history();
