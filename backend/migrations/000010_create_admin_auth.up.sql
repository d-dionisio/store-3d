CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    full_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'quote_viewer' CHECK (role IN ('quote_viewer', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    failed_login_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    locked_until TIMESTAMPTZ,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(email)) > 3),
    CHECK (length(btrim(full_name)) > 0),
    CHECK (length(password_hash) >= 50)
);
CREATE UNIQUE INDEX admin_users_email_lower_key ON admin_users(lower(email));

CREATE TABLE admin_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    csrf_token_hash TEXT NOT NULL CHECK (csrf_token_hash ~ '^[0-9a-f]{64}$'),
    ip_address INET,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX admin_sessions_active_idx ON admin_sessions(token_hash, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX admin_sessions_user_idx ON admin_sessions(admin_user_id, created_at DESC);

CREATE TABLE admin_audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    session_id UUID REFERENCES admin_sessions(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'denied', 'failure')),
    ip_address INET,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(action)) > 0)
);
CREATE INDEX admin_audit_events_user_created_idx ON admin_audit_events(admin_user_id, created_at DESC);
CREATE INDEX admin_audit_events_resource_idx ON admin_audit_events(resource_type, resource_id, created_at DESC);
