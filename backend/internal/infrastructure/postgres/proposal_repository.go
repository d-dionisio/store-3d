package postgres

import (
	"context"
	"errors"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/admin"
	"github.com/jackc/pgx/v5"
)

func auditProposal(ctx context.Context, tx pgx.Tx, user, action, ref string) error {
	_, err := tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,$2,'quote',$3,'success','{}'::jsonb)`, user, action, ref)
	return err
}

func (r *AdminRepository) StartAnalysis(ctx context.Context, ref, user string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM quotes WHERE quote_number=$1 FOR UPDATE`, ref).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidTransition
	}
	if err != nil {
		return err
	}
	if status == "under_review" {
		return nil
	}
	if status != "submitted" {
		return domain.ErrInvalidTransition
	}
	if _, err = tx.Exec(ctx, `UPDATE quotes SET status='under_review',reviewed_at=coalesce(reviewed_at,now()),updated_at=now() WHERE quote_number=$1`, ref); err != nil {
		return err
	}
	if err = auditProposal(ctx, tx, user, "quote.analysis.start", ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AdminRepository) SavePricing(ctx context.Context, ref, user string, p domain.Pricing) error {
	if err := p.Validate(time.Now()); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var quoteID string
	err = tx.QueryRow(ctx, `SELECT id FROM quotes WHERE quote_number=$1 AND status='under_review' FOR UPDATE`, ref).Scan(&quoteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidTransition
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO quote_proposals(quote_id,version,material_cost_cents,energy_cost_cents,machine_cost_cents,finishing_cost_cents,packaging_cost_cents,other_costs_cents,margin_basis_points,final_price_cents,production_lead_time_days,estimated_completion_date,valid_until,commercial_notes,shipping_included,created_by)
		VALUES($1,coalesce((SELECT max(version)+1 FROM quote_proposals WHERE quote_id=$1),1),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,nullif($13,''),$14,$15)
		ON CONFLICT(quote_id) WHERE status='draft' DO UPDATE SET material_cost_cents=$2,energy_cost_cents=$3,machine_cost_cents=$4,finishing_cost_cents=$5,packaging_cost_cents=$6,other_costs_cents=$7,margin_basis_points=$8,final_price_cents=$9,production_lead_time_days=$10,estimated_completion_date=$11,valid_until=$12,commercial_notes=nullif($13,''),shipping_included=$14,created_by=$15,updated_at=now()`, quoteID, p.MaterialCostCents, p.EnergyCostCents, p.MachineCostCents, p.FinishingCostCents, p.PackagingCostCents, p.OtherCostsCents, p.MarginBasisPoints, p.FinalPriceCents, p.ProductionLeadTimeDays, p.EstimatedCompletionDate, p.ValidUntil, p.CommercialNotes, p.ShippingIncluded, user)
	if err != nil {
		return err
	}
	if err = auditProposal(ctx, tx, user, "quote.pricing.save", ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AdminRepository) SendProposal(ctx context.Context, ref, user string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var quoteID string
	err = tx.QueryRow(ctx, `SELECT id FROM quotes WHERE quote_number=$1 AND status='under_review' AND EXISTS(SELECT 1 FROM quote_files WHERE quote_id=quotes.id) AND NOT EXISTS(SELECT 1 FROM quote_files WHERE quote_id=quotes.id AND security_status<>'clean') FOR UPDATE`, ref).Scan(&quoteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidTransition
	}
	if err != nil {
		return err
	}
	var price int64
	var lead int
	var validity time.Time
	err = tx.QueryRow(ctx, `UPDATE quote_proposals SET status='sent',sent_by=$2,sent_at=now(),notification_status='not_configured',updated_at=now() WHERE quote_id=$1 AND status='draft' RETURNING final_price_cents,production_lead_time_days,valid_until`, quoteID, user).Scan(&price, &lead, &validity)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrInvalidTransition
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE quotes SET status='quoted',quoted_amount_cents=$2,estimated_lead_time_days=$3,valid_until=$4,quoted_at=now(),updated_at=now() WHERE id=$1`, quoteID, price, lead, validity); err != nil {
		return err
	}
	if err = auditProposal(ctx, tx, user, "quote.proposal.send", ref); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AdminRepository) ExpireProposals(ctx context.Context, user string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH expired AS (UPDATE quote_proposals SET status='expired',updated_at=now() WHERE status='sent' AND valid_until<now() RETURNING quote_id) UPDATE quotes SET status='expired',updated_at=now() WHERE id IN(SELECT quote_id FROM expired) AND status='quoted' RETURNING quote_number`)
	if err != nil {
		return 0, err
	}
	var refs []string
	for rows.Next() {
		var ref string
		if err = rows.Scan(&ref); err != nil {
			rows.Close()
			return 0, err
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, ref := range refs {
		if err = auditProposal(ctx, tx, user, "quote.proposal.expire", ref); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int64(len(refs)), nil
}
