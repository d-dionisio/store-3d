ALTER TABLE order_status_events DROP CONSTRAINT order_status_events_status_check;
ALTER TABLE order_status_events ADD CONSTRAINT order_status_events_status_check CHECK (status IN (
    'pending', 'confirmed', 'in_production', 'partially_shipped', 'shipped', 'delivered', 'cancelled'
));

ALTER TABLE order_items DROP CONSTRAINT order_items_custom_total_check;
ALTER TABLE order_items DROP COLUMN line_total_cents;
ALTER TABLE order_items DROP COLUMN approved_total_cents;
ALTER TABLE order_items ADD COLUMN line_total_cents BIGINT GENERATED ALWAYS AS (unit_price_cents * quantity) STORED;

DROP INDEX IF EXISTS orders_quote_proposal_id_key;
ALTER TABLE orders DROP CONSTRAINT orders_quote_proposal_snapshot_check;
ALTER TABLE orders
    DROP COLUMN commercial_notes,
    DROP COLUMN shipping_included,
    DROP COLUMN production_lead_time_days,
    DROP COLUMN quote_proposal_version,
    DROP COLUMN quote_proposal_id;
ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN (
    'pending', 'confirmed', 'in_production', 'partially_shipped', 'shipped', 'delivered', 'cancelled'
));
