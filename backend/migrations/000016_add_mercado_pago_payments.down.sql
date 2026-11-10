DROP INDEX IF EXISTS payment_webhook_events_external_payment_idx;
DROP INDEX IF EXISTS payment_attempts_pix_expiry_idx;
DROP INDEX IF EXISTS payments_one_open_order_idx;
ALTER TABLE payment_attempts DROP CONSTRAINT IF EXISTS payment_attempts_pix_artifacts_check;
ALTER TABLE payment_attempts
    DROP COLUMN IF EXISTS provider_updated_at,
    DROP COLUMN IF EXISTS pix_ticket_url,
    DROP COLUMN IF EXISTS pix_qr_code_base64,
    DROP COLUMN IF EXISTS pix_qr_code,
    DROP COLUMN IF EXISTS provider_status_detail,
    DROP COLUMN IF EXISTS provider_status;
