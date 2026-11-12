CREATE TABLE email_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_type TEXT NOT NULL CHECK (message_type IN ('verify_email','password_reset','magic_link','password_changed','mfa_changed','security_alert','order_confirmed','order_updated','quote_proposal','quote_decision')),
    recipient_hash TEXT NOT NULL CHECK (recipient_hash ~ '^[0-9a-f]{64}$'),
    payload_ciphertext BYTEA NOT NULL CHECK (octet_length(payload_ciphertext) BETWEEN 32 AND 1048576),
    key_version SMALLINT NOT NULL CHECK (key_version > 0),
    idempotency_key TEXT NOT NULL UNIQUE CHECK (length(idempotency_key) BETWEEN 1 AND 200),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','sent','retry','failed','expired','delivered','bounced','complained')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 8),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    locked_at TIMESTAMPTZ,
    provider_message_id TEXT UNIQUE,
    last_error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);
CREATE INDEX email_outbox_dispatch_idx ON email_outbox(next_attempt_at,created_at) WHERE status IN ('pending','retry','processing');

CREATE TABLE email_delivery_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_event_id TEXT NOT NULL UNIQUE,
    provider_message_id TEXT,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ,
    payload_hash TEXT NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE email_suppressions (
    recipient_hash TEXT PRIMARY KEY CHECK (recipient_hash ~ '^[0-9a-f]{64}$'),
    reason TEXT NOT NULL CHECK (reason IN ('bounce','complaint','manual')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

