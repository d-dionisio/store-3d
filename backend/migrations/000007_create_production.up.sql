CREATE TABLE production_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id UUID NOT NULL REFERENCES order_items(id) ON DELETE RESTRICT,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE RESTRICT,
    color_id UUID NOT NULL REFERENCES colors(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'queued', 'printing', 'post_processing', 'quality_check',
        'completed', 'failed', 'cancelled'
    )),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    printer_reference TEXT,
    estimated_minutes INTEGER CHECK (estimated_minutes > 0),
    actual_minutes INTEGER CHECK (actual_minutes > 0),
    material_estimated_grams INTEGER CHECK (material_estimated_grams > 0),
    material_actual_grams INTEGER CHECK (material_actual_grams > 0),
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status NOT IN ('printing', 'post_processing', 'quality_check') OR started_at IS NOT NULL),
    CHECK (status <> 'completed' OR completed_at IS NOT NULL),
    CHECK (status <> 'failed' OR failure_reason IS NOT NULL)
);

CREATE OR REPLACE FUNCTION ensure_production_payment_confirmed()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    related_order_id UUID;
BEGIN
    SELECT order_id INTO related_order_id
      FROM order_items
     WHERE id = NEW.order_item_id;

    IF related_order_id IS NULL THEN
        RAISE EXCEPTION 'order item % does not exist', NEW.order_item_id;
    END IF;

    IF NOT EXISTS (
        SELECT 1
          FROM payments
         WHERE order_id = related_order_id
           AND status IN ('paid', 'partially_refunded')
           AND paid_amount_cents > refunded_amount_cents
    ) THEN
        RAISE EXCEPTION 'production requires a confirmed payment for order %', related_order_id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER production_jobs_payment_trigger
BEFORE INSERT OR UPDATE OF order_item_id ON production_jobs
FOR EACH ROW EXECUTE FUNCTION ensure_production_payment_confirmed();

