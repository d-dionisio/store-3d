CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number TEXT NOT NULL UNIQUE,
    quote_id UUID REFERENCES quotes(id) ON DELETE RESTRICT,
    user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'confirmed', 'in_production', 'partially_shipped', 'shipped', 'delivered', 'cancelled'
    )),
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    customer_phone TEXT,
    currency TEXT NOT NULL DEFAULT 'BRL' CHECK (currency ~ '^[A-Z]{3}$'),
    subtotal_cents BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    shipping_cents BIGINT NOT NULL DEFAULT 0 CHECK (shipping_cents >= 0),
    discount_cents BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    notes TEXT,
    placed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT orders_id_key UNIQUE (id),
    CHECK (length(btrim(order_number)) > 0),
    CHECK (length(btrim(customer_name)) > 0),
    CHECK (length(btrim(customer_email)) > 3),
    CHECK (discount_cents <= subtotal_cents + shipping_cents),
    CHECK (total_cents = subtotal_cents + shipping_cents - discount_cents),
    CHECK (status <> 'cancelled' OR cancelled_at IS NOT NULL)
);

CREATE UNIQUE INDEX orders_quote_id_key ON orders(quote_id) WHERE quote_id IS NOT NULL;

CREATE TABLE order_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    address_type TEXT NOT NULL CHECK (address_type IN ('shipping', 'billing')),
    recipient_name TEXT NOT NULL,
    postal_code TEXT NOT NULL,
    street TEXT NOT NULL,
    number TEXT NOT NULL,
    complement TEXT,
    district TEXT NOT NULL,
    city TEXT NOT NULL,
    state_code TEXT NOT NULL CHECK (state_code ~ '^[A-Z]{2}$'),
    country_code TEXT NOT NULL DEFAULT 'BR' CHECK (country_code ~ '^[A-Z]{2}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, address_type),
    CHECK (length(btrim(recipient_name)) > 0),
    CHECK (length(btrim(postal_code)) > 0),
    CHECK (length(btrim(street)) > 0),
    CHECK (length(btrim(number)) > 0),
    CHECK (length(btrim(district)) > 0),
    CHECK (length(btrim(city)) > 0)
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL CHECK (item_type IN ('catalog', 'custom')),
    product_id UUID REFERENCES products(id) ON DELETE RESTRICT,
    product_variant_id UUID,
    product_name TEXT NOT NULL,
    description_snapshot TEXT NOT NULL,
    sku_snapshot TEXT,
    material_name TEXT NOT NULL,
    color_name TEXT NOT NULL,
    size_name TEXT,
    finish_name TEXT,
    dimensions_snapshot TEXT,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    line_total_cents BIGINT GENERATED ALWAYS AS (unit_price_cents * quantity) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT order_items_id_order_key UNIQUE (id, order_id),
    CONSTRAINT order_items_variant_product_fk
        FOREIGN KEY (product_variant_id, product_id)
        REFERENCES product_variants(id, product_id) ON DELETE RESTRICT,
    CHECK (length(btrim(product_name)) > 0),
    CHECK (length(btrim(description_snapshot)) > 0),
    CHECK (length(btrim(material_name)) > 0),
    CHECK (length(btrim(color_name)) > 0),
    CHECK (
        (item_type = 'catalog' AND product_id IS NOT NULL AND product_variant_id IS NOT NULL
            AND sku_snapshot IS NOT NULL AND length(btrim(sku_snapshot)) > 0)
        OR
        (item_type = 'custom' AND product_id IS NULL AND product_variant_id IS NULL
            AND sku_snapshot IS NULL)
    )
);

CREATE TABLE order_status_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN (
        'pending', 'confirmed', 'in_production', 'partially_shipped', 'shipped', 'delivered', 'cancelled'
    )),
    description TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION ensure_quote_order_consistency()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    quote_status TEXT;
    quote_amount BIGINT;
BEGIN
    IF NEW.quote_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT status, quoted_amount_cents
      INTO quote_status, quote_amount
      FROM quotes
     WHERE id = NEW.quote_id
     FOR UPDATE;

    IF quote_status IS NULL THEN
        RAISE EXCEPTION 'quote % does not exist', NEW.quote_id;
    END IF;
    IF quote_status <> 'approved' THEN
        RAISE EXCEPTION 'quote % must be approved before order creation', NEW.quote_id;
    END IF;
    IF NEW.subtotal_cents <> quote_amount THEN
        RAISE EXCEPTION 'order subtotal must match approved quote amount';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER orders_quote_consistency_trigger
BEFORE INSERT OR UPDATE OF quote_id, subtotal_cents ON orders
FOR EACH ROW EXECUTE FUNCTION ensure_quote_order_consistency();

CREATE OR REPLACE FUNCTION ensure_order_item_source()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    source_quote_id UUID;
BEGIN
    SELECT quote_id INTO source_quote_id
      FROM orders
     WHERE id = NEW.order_id;

    IF NEW.item_type = 'custom' AND source_quote_id IS NULL THEN
        RAISE EXCEPTION 'custom order items require an order created from a quote';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER order_items_source_trigger
BEFORE INSERT OR UPDATE OF item_type, order_id ON order_items
FOR EACH ROW EXECUTE FUNCTION ensure_order_item_source();

