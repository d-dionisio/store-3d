ALTER TABLE orders
    ADD COLUMN checkout_key_hash TEXT,
    ADD COLUMN payment_readiness TEXT NOT NULL DEFAULT 'awaiting_address' CHECK (
        payment_readiness IN ('awaiting_address','awaiting_shipping_quote','ready')
    );
CREATE UNIQUE INDEX orders_checkout_key_hash_key ON orders(checkout_key_hash) WHERE checkout_key_hash IS NOT NULL;
ALTER TABLE orders ADD CONSTRAINT orders_checkout_key_hash_check CHECK (
    checkout_key_hash IS NULL OR checkout_key_hash ~ '^[0-9a-f]{64}$'
);
