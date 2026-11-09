CREATE TABLE shipping_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    external_service_code TEXT NOT NULL,
    carrier_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_service_code),
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(external_service_code)) > 0),
    CHECK (length(btrim(carrier_name)) > 0),
    CHECK (length(btrim(service_name)) > 0)
);

CREATE TABLE shipping_quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipping_service_id UUID REFERENCES shipping_services(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    external_quote_id TEXT,
    external_service_code TEXT NOT NULL,
    carrier_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    origin_postal_code TEXT NOT NULL,
    destination_postal_code TEXT NOT NULL,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    estimated_min_days INTEGER NOT NULL CHECK (estimated_min_days >= 0),
    estimated_max_days INTEGER NOT NULL,
    package_data JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (estimated_max_days >= estimated_min_days),
    CHECK (expires_at > created_at),
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(external_service_code)) > 0),
    CHECK (length(btrim(carrier_name)) > 0),
    CHECK (length(btrim(service_name)) > 0),
    CHECK (length(btrim(origin_postal_code)) > 0),
    CHECK (length(btrim(destination_postal_code)) > 0)
);

CREATE UNIQUE INDEX shipping_quotes_external_key
    ON shipping_quotes(provider, external_quote_id)
    WHERE external_quote_id IS NOT NULL;

CREATE TABLE order_shipping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    shipping_quote_id UUID REFERENCES shipping_quotes(id) ON DELETE SET NULL,
    shipping_service_id UUID REFERENCES shipping_services(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    external_service_code TEXT NOT NULL,
    external_quote_id TEXT,
    carrier_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    estimated_min_days INTEGER NOT NULL CHECK (estimated_min_days >= 0),
    estimated_max_days INTEGER NOT NULL,
    estimated_delivery_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_shipping_id_order_key UNIQUE (id, order_id),
    CHECK (estimated_max_days >= estimated_min_days),
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(external_service_code)) > 0),
    CHECK (length(btrim(carrier_name)) > 0),
    CHECK (length(btrim(service_name)) > 0)
);

CREATE TABLE shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    order_shipping_id UUID,
    provider TEXT NOT NULL,
    external_shipment_id TEXT,
    external_label_id TEXT,
    carrier_name TEXT NOT NULL,
    service_name TEXT NOT NULL,
    tracking_code TEXT,
    tracking_url TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'ready', 'shipped', 'in_transit', 'delivered', 'failed', 'returned', 'cancelled'
    )),
    shipping_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (shipping_cost_cents >= 0),
    posted_at TIMESTAMPTZ,
    shipped_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT shipments_id_order_key UNIQUE (id, order_id),
    CONSTRAINT shipments_order_shipping_fk
        FOREIGN KEY (order_shipping_id, order_id)
        REFERENCES order_shipping(id, order_id) ON DELETE RESTRICT,
    CHECK (length(btrim(provider)) > 0),
    CHECK (length(btrim(carrier_name)) > 0),
    CHECK (length(btrim(service_name)) > 0),
    CHECK (status <> 'shipped' OR shipped_at IS NOT NULL),
    CHECK (status <> 'delivered' OR delivered_at IS NOT NULL)
);

CREATE UNIQUE INDEX shipments_external_key
    ON shipments(provider, external_shipment_id)
    WHERE external_shipment_id IS NOT NULL;

CREATE UNIQUE INDEX shipments_tracking_code_key
    ON shipments(tracking_code)
    WHERE tracking_code IS NOT NULL;

CREATE TABLE shipment_items (
    shipment_id UUID NOT NULL,
    order_id UUID NOT NULL,
    order_item_id UUID NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (shipment_id, order_item_id),
    FOREIGN KEY (shipment_id, order_id)
        REFERENCES shipments(id, order_id) ON DELETE CASCADE,
    FOREIGN KEY (order_item_id, order_id)
        REFERENCES order_items(id, order_id) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION enforce_shipment_item_quantity()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    ordered_quantity INTEGER;
    already_shipped INTEGER;
    previous_shipment_id UUID;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        previous_shipment_id := OLD.shipment_id;
    END IF;

    SELECT quantity
      INTO ordered_quantity
      FROM order_items
     WHERE id = NEW.order_item_id AND order_id = NEW.order_id
     FOR UPDATE;

    IF ordered_quantity IS NULL THEN
        RAISE EXCEPTION 'order item % does not belong to order %', NEW.order_item_id, NEW.order_id;
    END IF;

    SELECT COALESCE(sum(quantity), 0)
      INTO already_shipped
      FROM shipment_items
     WHERE order_item_id = NEW.order_item_id
       AND order_id = NEW.order_id
       AND (previous_shipment_id IS NULL OR shipment_id <> previous_shipment_id);

    IF already_shipped + NEW.quantity > ordered_quantity THEN
        RAISE EXCEPTION 'shipped quantity exceeds ordered quantity for item %', NEW.order_item_id;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER shipment_items_quantity_trigger
BEFORE INSERT OR UPDATE OF quantity, order_item_id, order_id, shipment_id ON shipment_items
FOR EACH ROW EXECUTE FUNCTION enforce_shipment_item_quantity();

