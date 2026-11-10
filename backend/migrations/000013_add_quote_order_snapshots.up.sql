ALTER TABLE orders DROP CONSTRAINT orders_status_check;
ALTER TABLE orders ADD CONSTRAINT orders_status_check CHECK (status IN (
    'pending_payment', 'pending', 'confirmed', 'in_production',
    'partially_shipped', 'shipped', 'delivered', 'cancelled'
));
ALTER TABLE orders
    ADD COLUMN quote_proposal_id UUID REFERENCES quote_proposals(id) ON DELETE RESTRICT,
    ADD COLUMN quote_proposal_version INTEGER CHECK (quote_proposal_version > 0),
    ADD COLUMN production_lead_time_days INTEGER CHECK (production_lead_time_days > 0),
    ADD COLUMN shipping_included BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN commercial_notes TEXT;
UPDATE orders o SET
    quote_proposal_id = p.id,
    quote_proposal_version = p.version,
    production_lead_time_days = p.production_lead_time_days,
    shipping_included = p.shipping_included,
    commercial_notes = p.commercial_notes
FROM quote_proposals p
WHERE o.quote_id = p.quote_id
  AND p.version = (SELECT max(p2.version) FROM quote_proposals p2 WHERE p2.quote_id = o.quote_id);
ALTER TABLE orders ADD CONSTRAINT orders_quote_proposal_snapshot_check CHECK (
        quote_id IS NULL OR (
            quote_proposal_id IS NOT NULL
            AND quote_proposal_version IS NOT NULL
            AND production_lead_time_days IS NOT NULL
        )
    );
CREATE UNIQUE INDEX orders_quote_proposal_id_key ON orders(quote_proposal_id) WHERE quote_proposal_id IS NOT NULL;

ALTER TABLE order_items DROP COLUMN line_total_cents;
ALTER TABLE order_items ADD COLUMN approved_total_cents BIGINT CHECK (approved_total_cents >= 0);
UPDATE order_items SET approved_total_cents = unit_price_cents * quantity WHERE item_type = 'custom';
ALTER TABLE order_items ADD COLUMN line_total_cents BIGINT GENERATED ALWAYS AS (
    CASE WHEN item_type = 'custom' AND approved_total_cents IS NOT NULL
         THEN approved_total_cents
         ELSE unit_price_cents * quantity
    END
) STORED;
ALTER TABLE order_items ADD CONSTRAINT order_items_custom_total_check CHECK (
    (item_type = 'custom' AND approved_total_cents IS NOT NULL)
    OR (item_type = 'catalog' AND approved_total_cents IS NULL)
);

ALTER TABLE order_status_events DROP CONSTRAINT order_status_events_status_check;
ALTER TABLE order_status_events ADD CONSTRAINT order_status_events_status_check CHECK (status IN (
    'pending_payment', 'pending', 'confirmed', 'in_production',
    'partially_shipped', 'shipped', 'delivered', 'cancelled'
));
