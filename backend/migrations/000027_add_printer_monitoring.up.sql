CREATE TABLE printer_integrations (
    printer_id UUID PRIMARY KEY REFERENCES printers(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('bambulab_lan')),
    host INET NOT NULL,
    port INTEGER NOT NULL DEFAULT 8883 CHECK (port BETWEEN 1 AND 65535),
    serial_number TEXT NOT NULL CHECK (length(btrim(serial_number)) BETWEEN 5 AND 128),
    certificate_sha256 TEXT NOT NULL CHECK (certificate_sha256 ~ '^[a-fA-F0-9]{64}$'),
    connection_type TEXT NOT NULL DEFAULT 'lan_mqtt_tls' CHECK (connection_type IN ('lan_mqtt_tls')),
    credentials_ciphertext BYTEA NOT NULL,
    key_version SMALLINT NOT NULL CHECK (key_version > 0),
    enabled BOOLEAN NOT NULL DEFAULT true,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    updated_by_admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, serial_number)
);

CREATE TABLE printer_current_telemetry (
    printer_id UUID PRIMARY KEY REFERENCES printers(id) ON DELETE CASCADE,
    connection_state TEXT NOT NULL CHECK (connection_state IN ('unknown','connecting','online','offline','error')),
    printer_state TEXT NOT NULL DEFAULT 'unknown' CHECK (printer_state IN ('unknown','idle','heating','printing','paused','finished','failed')),
    progress_percent NUMERIC(5,2) CHECK (progress_percent BETWEEN 0 AND 100),
    nozzle_temperature_c NUMERIC(6,2),
    bed_temperature_c NUMERIC(6,2),
    speed_level INTEGER,
    observed_job_name TEXT,
    elapsed_seconds BIGINT CHECK (elapsed_seconds >= 0),
    error_code TEXT,
    last_seen_at TIMESTAMPTZ,
    payload_observed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE printer_connection_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('connecting','connected','disconnected','authentication_failed','timeout','error')),
    message TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deduplication_key TEXT NOT NULL UNIQUE
);
CREATE INDEX printer_connection_events_history_idx ON printer_connection_events(printer_id, occurred_at DESC);

CREATE TABLE printer_observed_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    printer_id UUID NOT NULL REFERENCES printers(id) ON DELETE CASCADE,
    provider_job_key TEXT NOT NULL,
    job_name TEXT,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    last_state TEXT NOT NULL,
    max_progress_percent NUMERIC(5,2) CHECK (max_progress_percent BETWEEN 0 AND 100),
    UNIQUE(printer_id, provider_job_key)
);
CREATE INDEX printer_observed_jobs_history_idx ON printer_observed_jobs(printer_id, last_observed_at DESC);
