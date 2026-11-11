ALTER TABLE payment_attempts DROP CONSTRAINT IF EXISTS payment_attempts_card_details_check;
ALTER TABLE payment_attempts
    DROP COLUMN IF EXISTS card_authentication_url,
    DROP COLUMN IF EXISTS card_payment_method_id,
    DROP COLUMN IF EXISTS card_installments;
