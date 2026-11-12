CREATE TABLE auth_reauthentication_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_session_id UUID REFERENCES admin_sessions(id) ON DELETE CASCADE,
    customer_session_id UUID REFERENCES customer_sessions(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((admin_session_id IS NOT NULL)::integer + (customer_session_id IS NOT NULL)::integer = 1),
    CHECK (expires_at > created_at)
);
CREATE INDEX auth_reauthentication_active_idx
    ON auth_reauthentication_evidence(token_hash, expires_at) WHERE consumed_at IS NULL;

CREATE TABLE auth_rate_limits (
    scope TEXT NOT NULL CHECK (scope IN ('login','mfa','register','password_reset','email_verification','recovery_code')),
    subject_hash TEXT NOT NULL CHECK (subject_hash ~ '^[0-9a-f]{64}$'),
    window_started_at TIMESTAMPTZ NOT NULL,
    attempts INTEGER NOT NULL CHECK (attempts > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (scope, subject_hash),
    CHECK (expires_at > window_started_at)
);
CREATE INDEX auth_rate_limits_expiry_idx ON auth_rate_limits(expires_at);

