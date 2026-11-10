ALTER TABLE payment_attempts
    ADD COLUMN provider_status TEXT,
    ADD COLUMN provider_status_detail TEXT,
    ADD COLUMN pix_qr_code TEXT,
    ADD COLUMN pix_qr_code_base64 TEXT,
    ADD COLUMN pix_ticket_url TEXT,
    ADD COLUMN provider_updated_at TIMESTAMPTZ;

ALTER TABLE payment_attempts ADD CONSTRAINT payment_attempts_pix_artifacts_check CHECK (
    method = 'pix' OR (
        pix_qr_code IS NULL AND pix_qr_code_base64 IS NULL AND
        pix_ticket_url IS NULL AND pix_expires_at IS NULL
    )
);

CREATE UNIQUE INDEX payments_one_open_order_idx
    ON payments(order_id)
    WHERE status IN ('pending','processing','authorized','paid','partially_refunded');

CREATE INDEX payment_attempts_pix_expiry_idx
    ON payment_attempts(pix_expires_at)
    WHERE method = 'pix' AND status IN ('pending','processing');

CREATE INDEX payment_webhook_events_external_payment_idx
    ON payment_webhook_events(provider, external_payment_id, received_at DESC)
    WHERE external_payment_id IS NOT NULL;

