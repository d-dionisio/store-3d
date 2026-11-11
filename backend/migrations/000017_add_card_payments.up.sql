ALTER TABLE payment_attempts
    ADD COLUMN card_installments SMALLINT,
    ADD COLUMN card_payment_method_id TEXT,
    ADD COLUMN card_authentication_url TEXT;

ALTER TABLE payment_attempts ADD CONSTRAINT payment_attempts_card_details_check CHECK (
    (method = 'card' AND card_installments BETWEEN 1 AND 24 AND length(btrim(card_payment_method_id)) > 0)
    OR (method = 'pix' AND card_installments IS NULL AND card_payment_method_id IS NULL AND card_authentication_url IS NULL)
);
