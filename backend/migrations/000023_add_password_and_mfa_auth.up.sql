ALTER TABLE users
    ADD COLUMN password_hash TEXT,
    ADD COLUMN password_changed_at TIMESTAMPTZ,
    ADD COLUMN failed_login_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_attempts >= 0),
    ADD COLUMN locked_until TIMESTAMPTZ;

ALTER TABLE users ADD CONSTRAINT users_password_hash_check
    CHECK (password_hash IS NULL OR length(password_hash) BETWEEN 50 AND 512);

CREATE TABLE auth_mfa_factors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    secret_ciphertext BYTEA NOT NULL,
    key_version SMALLINT NOT NULL CHECK (key_version > 0),
    enabled_at TIMESTAMPTZ,
    last_accepted_counter BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((admin_user_id IS NOT NULL)::integer + (user_id IS NOT NULL)::integer = 1),
    CHECK (octet_length(secret_ciphertext) BETWEEN 32 AND 512)
);
CREATE UNIQUE INDEX auth_mfa_factors_admin_key ON auth_mfa_factors(admin_user_id) WHERE admin_user_id IS NOT NULL;
CREATE UNIQUE INDEX auth_mfa_factors_user_key ON auth_mfa_factors(user_id) WHERE user_id IS NOT NULL;

CREATE TABLE auth_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    factor_id UUID NOT NULL REFERENCES auth_mfa_factors(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL CHECK (code_hash ~ '^[0-9a-f]{64}$'),
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (factor_id, code_hash)
);
CREATE INDEX auth_recovery_codes_unused_idx ON auth_recovery_codes(factor_id) WHERE used_at IS NULL;

CREATE TABLE auth_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('admin_mfa_setup','admin_mfa_login','customer_mfa_setup','customer_mfa_login','strong_reauth')),
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 10),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((admin_user_id IS NOT NULL)::integer + (user_id IS NOT NULL)::integer = 1),
    CHECK (expires_at > created_at)
);
CREATE INDEX auth_challenges_active_idx ON auth_challenges(token_hash,expires_at) WHERE consumed_at IS NULL;

CREATE TABLE customer_auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose TEXT NOT NULL CHECK (purpose IN ('verify_email','reset_password')),
    token_hash TEXT NOT NULL UNIQUE CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX customer_auth_tokens_active_idx ON customer_auth_tokens(token_hash,purpose,expires_at) WHERE consumed_at IS NULL;
CREATE INDEX customer_auth_tokens_user_idx ON customer_auth_tokens(user_id,purpose,created_at DESC);

ALTER TABLE admin_sessions ADD COLUMN authentication_level TEXT NOT NULL DEFAULT 'mfa'
    CHECK (authentication_level IN ('mfa'));
ALTER TABLE customer_sessions ADD COLUMN authentication_level TEXT NOT NULL DEFAULT 'magic'
    CHECK (authentication_level IN ('magic','password','mfa'));

-- Existing administrative sessions predate mandatory application MFA.
UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now());

CREATE TABLE security_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('admin','customer','anonymous','system')),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    outcome TEXT NOT NULL CHECK (outcome IN ('success','denied','failure')),
    ip_address INET,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(btrim(event_type)) > 0)
);
CREATE INDEX security_events_admin_idx ON security_events(admin_user_id,created_at DESC) WHERE admin_user_id IS NOT NULL;
CREATE INDEX security_events_user_idx ON security_events(user_id,created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX security_events_type_idx ON security_events(event_type,created_at DESC);
