CREATE TABLE production_job_legacy_links (
    job_id UUID PRIMARY KEY REFERENCES production_jobs(id) ON DELETE CASCADE,
    assigned_parent_job_id UUID NOT NULL REFERENCES production_jobs(id) ON DELETE RESTRICT,
    migrated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (job_id <> assigned_parent_job_id)
);

WITH ranked_jobs AS (
    SELECT id, first_value(id) OVER (
               PARTITION BY order_item_id ORDER BY created_at, id
           ) AS primary_job_id,
           row_number() OVER (
               PARTITION BY order_item_id ORDER BY created_at, id
           ) AS position
      FROM production_jobs
     WHERE parent_job_id IS NULL
)
INSERT INTO production_job_legacy_links(job_id, assigned_parent_job_id)
SELECT id, primary_job_id FROM ranked_jobs WHERE position > 1;

UPDATE production_jobs pj
   SET parent_job_id = legacy.assigned_parent_job_id,
       updated_at = now()
  FROM production_job_legacy_links legacy
 WHERE pj.id = legacy.job_id;

CREATE UNIQUE INDEX production_jobs_initial_order_item_key
    ON production_jobs(order_item_id) WHERE parent_job_id IS NULL;

CREATE TABLE material_spool_adjustments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    material_spool_id UUID NOT NULL REFERENCES material_spools(id) ON DELETE RESTRICT,
    delta_grams INTEGER NOT NULL CHECK (delta_grams <> 0),
    previous_balance_grams INTEGER NOT NULL CHECK (previous_balance_grams >= 0),
    new_balance_grams INTEGER NOT NULL CHECK (new_balance_grams >= 0),
    reason TEXT NOT NULL,
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (new_balance_grams = previous_balance_grams + delta_grams),
    CHECK (length(btrim(reason)) >= 3)
);

CREATE FUNCTION prevent_material_spool_adjustment_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'material spool adjustments are append-only'; END;
$$;
CREATE TRIGGER material_spool_adjustments_append_only_trigger
BEFORE UPDATE OR DELETE ON material_spool_adjustments
FOR EACH ROW EXECUTE FUNCTION prevent_material_spool_adjustment_mutation();

CREATE FUNCTION create_production_jobs_for_paid_order(target_order_id UUID)
RETURNS INTEGER LANGUAGE plpgsql AS $$
DECLARE
    target_order RECORD;
    target_payment RECORD;
    inserted_count INTEGER;
BEGIN
    SELECT id, status, currency, total_cents INTO target_order
      FROM orders WHERE id = target_order_id FOR UPDATE;
    IF target_order.id IS NULL OR target_order.status NOT IN ('confirmed', 'in_production') THEN
        RAISE EXCEPTION 'production jobs require a confirmed order';
    END IF;
    SELECT status, amount_cents, paid_amount_cents, refunded_amount_cents, currency
      INTO target_payment FROM payments
     WHERE order_id = target_order_id AND status = 'paid' FOR UPDATE;
    IF target_payment.status IS NULL
       OR target_payment.amount_cents <> target_order.total_cents
       OR target_payment.paid_amount_cents <> target_order.total_cents
       OR target_payment.refunded_amount_cents <> 0
       OR target_payment.currency <> target_order.currency THEN
        RAISE EXCEPTION 'production jobs require full non-refunded settlement in order currency';
    END IF;

    INSERT INTO production_jobs(order_item_id, material_id, color_id, quantity)
    SELECT oi.id,
           CASE WHEN oi.item_type = 'catalog' THEN pv.material_id ELSE q.material_id END,
           CASE WHEN oi.item_type = 'catalog' THEN pv.color_id ELSE q.color_id END,
           oi.quantity
      FROM order_items oi
      JOIN orders o ON o.id = oi.order_id
      LEFT JOIN product_variants pv ON pv.id = oi.product_variant_id
      LEFT JOIN quotes q ON q.id = o.quote_id
     WHERE oi.order_id = target_order_id
       AND CASE WHEN oi.item_type = 'catalog' THEN pv.material_id IS NOT NULL AND pv.color_id IS NOT NULL
                ELSE q.material_id IS NOT NULL AND q.color_id IS NOT NULL END
    ON CONFLICT (order_item_id) WHERE parent_job_id IS NULL DO NOTHING;
    GET DIAGNOSTICS inserted_count = ROW_COUNT;

    IF EXISTS (
        SELECT 1 FROM order_items oi
         WHERE oi.order_id = target_order_id
           AND NOT EXISTS (SELECT 1 FROM production_jobs pj WHERE pj.order_item_id = oi.id AND pj.parent_job_id IS NULL)
    ) THEN RAISE EXCEPTION 'one or more order items could not generate a production job'; END IF;
    RETURN inserted_count;
END;
$$;

CREATE INDEX material_spool_adjustments_spool_created_idx
    ON material_spool_adjustments(material_spool_id, created_at DESC);

