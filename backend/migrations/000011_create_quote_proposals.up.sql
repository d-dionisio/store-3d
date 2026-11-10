CREATE TABLE quote_proposals (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE RESTRICT,
 version INTEGER NOT NULL CHECK(version>0), status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN('draft','sent','expired','superseded')),
 material_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK(material_cost_cents>=0), energy_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK(energy_cost_cents>=0),
 machine_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK(machine_cost_cents>=0), finishing_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK(finishing_cost_cents>=0),
 packaging_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK(packaging_cost_cents>=0), other_costs_cents BIGINT NOT NULL DEFAULT 0 CHECK(other_costs_cents>=0),
 margin_basis_points INTEGER NOT NULL DEFAULT 0 CHECK(margin_basis_points>=0 AND margin_basis_points<=100000), final_price_cents BIGINT NOT NULL CHECK(final_price_cents>=0),
 production_lead_time_days INTEGER NOT NULL CHECK(production_lead_time_days>0), estimated_completion_date DATE, valid_until TIMESTAMPTZ NOT NULL,
 commercial_notes TEXT, shipping_included BOOLEAN NOT NULL DEFAULT FALSE, created_by UUID NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
 sent_by UUID REFERENCES admin_users(id) ON DELETE RESTRICT, sent_at TIMESTAMPTZ, notification_status TEXT NOT NULL DEFAULT 'pending' CHECK(notification_status IN('pending','sent','failed','not_configured')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(quote_id,version),
 CHECK(valid_until>created_at),
 CHECK ((status = 'draft' AND sent_at IS NULL AND sent_by IS NULL) OR
        (status IN ('sent','expired','superseded') AND sent_at IS NOT NULL AND sent_by IS NOT NULL))
);
CREATE UNIQUE INDEX quote_proposals_one_draft_idx ON quote_proposals(quote_id) WHERE status='draft';
CREATE INDEX quote_proposals_quote_created_idx ON quote_proposals(quote_id,created_at DESC);
