ALTER TABLE shipping_quotes
    ADD COLUMN order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
    ADD COLUMN request_hash TEXT;

UPDATE shipping_quotes q
   SET order_id = os.order_id
  FROM order_shipping os
 WHERE os.shipping_quote_id = q.id
   AND q.order_id IS NULL;

ALTER TABLE shipping_quotes
    ADD CONSTRAINT shipping_quotes_order_id_id_key UNIQUE (order_id, id),
    ADD CONSTRAINT shipping_quotes_request_hash_format CHECK (
        request_hash IS NULL OR request_hash ~ '^[0-9a-f]{64}$'
    );

CREATE INDEX shipping_quotes_order_expiry_idx
    ON shipping_quotes(order_id, expires_at DESC);

CREATE INDEX shipping_quotes_order_request_idx
    ON shipping_quotes(order_id, request_hash, expires_at DESC)
    WHERE request_hash IS NOT NULL;

CREATE TABLE order_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity BETWEEN 1 AND 20),
    weight_grams INTEGER NOT NULL CHECK (weight_grams BETWEEN 1 AND 100000),
    length_mm INTEGER NOT NULL CHECK (length_mm BETWEEN 10 AND 2000),
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 10 AND 2000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 10 AND 2000),
    created_by_admin_user_id UUID REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, sequence)
);

ALTER TABLE order_shipping
    ADD CONSTRAINT order_shipping_quote_order_fk
    FOREIGN KEY (order_id, shipping_quote_id)
    REFERENCES shipping_quotes(order_id, id) ON DELETE RESTRICT;

