CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    method TEXT NOT NULL CHECK (method IN ('pix', 'card')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'processing', 'authorized', 'paid', 'failed', 'cancelled',
        'partially_refunded', 'refunded'
    )),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    paid_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (paid_amount_cents >= 0),
    refunded_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (refunded_amount_cents >= 0),
    currency TEXT NOT NULL DEFAULT 'BRL' CHECK (currency ~ '^[A-Z]{3}$'),
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (paid_amount_cents <= amount_cents),
    CHECK (refunded_amount_cents <= paid_amount_cents),
    CHECK (status NOT IN ('paid', 'partially_refunded', 'refunded') OR paid_at IS NOT NULL),
    CHECK (status <> 'paid' OR paid_amount_cents = amount_cents),
    CHECK (status <> 'refunded' OR refunded_amount_cents = paid_amount_cents)
);

CREATE TABLE payment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    method TEXT NOT NULL CHECK (method IN ('pix', 'card')),
    idempotency_key TEXT NOT NULL UNIQUE,
    external_payment_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'processing', 'authorized', 'paid', 'failed', 'cancelled', 'expired'
    )),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    card_brand TEXT,
    card_last_four TEXT CHECK (card_last_four ~ '^[0-9]{4}$'),
    pix_expires_at TIMESTAMPTZ,
    failure_code TEXT,
    failure_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_attempts_id_payment_key UNIQUE (id, payment_id),
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(idempotency_key)) > 0),
    CHECK (method <> 'pix' OR card_brand IS NULL),
    CHECK (method <> 'pix' OR card_last_four IS NULL)
);

CREATE UNIQUE INDEX payment_attempts_external_key
    ON payment_attempts(provider, external_payment_id)
    WHERE external_payment_id IS NOT NULL;

CREATE TABLE payment_status_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    payment_attempt_id UUID,
    previous_status TEXT,
    new_status TEXT NOT NULL CHECK (new_status IN (
        'pending', 'processing', 'authorized', 'paid', 'failed', 'cancelled',
        'expired', 'partially_refunded', 'refunded'
    )),
    source TEXT NOT NULL CHECK (source IN ('system', 'provider_webhook', 'admin', 'reconciliation')),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_status_events_attempt_payment_fk
        FOREIGN KEY (payment_attempt_id, payment_id)
        REFERENCES payment_attempts(id, payment_id) ON DELETE CASCADE
);

CREATE TABLE payment_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_attempt_id UUID REFERENCES payment_attempts(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    external_event_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    external_payment_id TEXT,
    payload JSONB NOT NULL,
    payload_sha256 TEXT NOT NULL CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    processing_status TEXT NOT NULL DEFAULT 'pending' CHECK (processing_status IN (
        'pending', 'processed', 'ignored', 'failed'
    )),
    processing_error TEXT,
    UNIQUE (provider, external_event_id),
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(external_event_id)) > 0),
    CHECK (length(btrim(event_type)) > 0),
    CHECK ((processing_status IN ('processed', 'ignored', 'failed')) = (processed_at IS NOT NULL))
);

CREATE TABLE payment_refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    payment_attempt_id UUID,
    provider TEXT NOT NULL,
    external_refund_id TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'processing', 'succeeded', 'failed', 'cancelled'
    )),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    CONSTRAINT payment_refunds_attempt_payment_fk
        FOREIGN KEY (payment_attempt_id, payment_id)
        REFERENCES payment_attempts(id, payment_id) ON DELETE RESTRICT,
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(idempotency_key)) > 0),
    CHECK (status NOT IN ('succeeded', 'failed', 'cancelled') OR processed_at IS NOT NULL)
);

CREATE UNIQUE INDEX payment_refunds_external_key
    ON payment_refunds(provider, external_refund_id)
    WHERE external_refund_id IS NOT NULL;

