package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	app "example.com/loja-3d/backend/internal/application/customer"
	domain "example.com/loja-3d/backend/internal/domain/customer"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository struct{ pool *pgxpool.Pool }

func NewCustomerRepository(p *pgxpool.Pool) *CustomerRepository { return &CustomerRepository{p} }
func (r *CustomerRepository) CreateMagicLink(ctx context.Context, email, hash, ip string, expires time.Time) error {
	var n int
	if e := r.pool.QueryRow(ctx, `SELECT count(*) FROM customer_magic_links WHERE lower(email)=lower($1) AND created_at>now()-interval '15 minutes'`, email).Scan(&n); e != nil {
		return e
	}
	if n >= 3 {
		return app.ErrRateLimited
	}
	_, e := r.pool.Exec(ctx, `INSERT INTO customer_magic_links(email,token_hash,requested_ip,expires_at) VALUES(lower($1),$2,nullif($3,'')::inet,$4)`, email, hash, ip, expires)
	return e
}
func (r *CustomerRepository) ConsumeMagicLink(ctx context.Context, hash, sessionHash, csrfHash, ip, ua string, expires time.Time) (domain.Session, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.Session{}, e
	}
	defer tx.Rollback(ctx)
	var email string
	e = tx.QueryRow(ctx, `UPDATE customer_magic_links SET consumed_at=now() WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() RETURNING email`, hash).Scan(&email)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Session{}, app.ErrUnauthorized
	}
	if e != nil {
		return domain.Session{}, e
	}
	var userID, name string
	e = tx.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES(lower($1),coalesce((SELECT customer_name FROM quotes WHERE lower(customer_email)=lower($1) ORDER BY created_at DESC LIMIT 1),split_part($1,'@',1)),now()) ON CONFLICT((lower(email))) DO UPDATE SET email_verified_at=coalesce(users.email_verified_at,now()),updated_at=now() RETURNING id,full_name`, email).Scan(&userID, &name)
	if e != nil {
		return domain.Session{}, e
	}
	if _, e = tx.Exec(ctx, `UPDATE quotes SET user_id=$1,updated_at=now() WHERE user_id IS NULL AND lower(customer_email)=lower($2)`, userID, email); e != nil {
		return domain.Session{}, e
	}
	var s domain.Session
	e = tx.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,ip_address,user_agent,expires_at) VALUES($1,$2,$3,nullif($4,'')::inet,left($5,500),$6) RETURNING id,expires_at`, userID, sessionHash, csrfHash, ip, ua, expires).Scan(&s.ID, &s.ExpiresAt)
	if e != nil {
		return s, e
	}
	s.UserID = userID
	s.Email = email
	s.FullName = name
	s.CSRFHash = csrfHash
	if _, e = tx.Exec(ctx, `INSERT INTO customer_audit_events(user_id,session_id,action,resource_type,resource_id,outcome,ip_address) VALUES($1,$2::uuid,'customer.login','session',$2::text,'success',nullif($3,'')::inet)`, userID, s.ID, ip); e != nil {
		return s, e
	}
	e = tx.Commit(ctx)
	return s, e
}
func (r *CustomerRepository) GetSession(ctx context.Context, hash string) (domain.Session, error) {
	var s domain.Session
	e := r.pool.QueryRow(ctx, `SELECT s.id,s.user_id,u.email,u.full_name,s.csrf_token_hash,s.expires_at FROM customer_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.status='active' AND u.email_verified_at IS NOT NULL`, hash).Scan(&s.ID, &s.UserID, &s.Email, &s.FullName, &s.CSRFHash, &s.ExpiresAt)
	if errors.Is(e, pgx.ErrNoRows) {
		e = app.ErrUnauthorized
	}
	return s, e
}
func (r *CustomerRepository) RevokeSession(ctx context.Context, id string) error {
	_, e := r.pool.Exec(ctx, `UPDATE customer_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE id=$1`, id)
	return e
}
func (r *CustomerRepository) Profile(ctx context.Context, id string) (domain.Profile, error) {
	var p domain.Profile
	e := r.pool.QueryRow(ctx, `SELECT id,email,full_name,coalesce(phone,'') FROM users WHERE id=$1 AND email_verified_at IS NOT NULL`, id).Scan(&p.ID, &p.Email, &p.FullName, &p.Phone)
	return p, e
}
func (r *CustomerRepository) ListQuotes(ctx context.Context, user string) ([]domain.QuoteSummary, error) {
	rows, e := r.pool.Query(ctx, `SELECT q.quote_number,q.status,coalesce(m.name,''),q.requested_quantity,q.created_at FROM quotes q LEFT JOIN materials m ON m.id=q.material_id WHERE q.user_id=$1 ORDER BY q.created_at DESC`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []domain.QuoteSummary{}
	for rows.Next() {
		var v domain.QuoteSummary
		if e = rows.Scan(&v.Reference, &v.Status, &v.Material, &v.Quantity, &v.CreatedAt); e != nil {
			return nil, e
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (r *CustomerRepository) GetQuote(ctx context.Context, user, ref string) (domain.QuoteDetail, error) {
	var q domain.QuoteDetail
	e := r.pool.QueryRow(ctx, `SELECT q.quote_number,q.status,coalesce(m.name,''),q.requested_quantity,q.created_at,coalesce(c.name,''),coalesce(q.notes,'') FROM quotes q LEFT JOIN materials m ON m.id=q.material_id LEFT JOIN colors c ON c.id=q.color_id WHERE q.user_id=$1 AND q.quote_number=$2`, user, ref).Scan(&q.Reference, &q.Status, &q.Material, &q.Quantity, &q.CreatedAt, &q.Color, &q.Notes)
	if errors.Is(e, pgx.ErrNoRows) {
		return q, app.ErrNotFound
	}
	if e != nil {
		return q, e
	}
	rows, e := r.pool.Query(ctx, `SELECT p.id,p.version,p.status,p.final_price_cents,p.production_lead_time_days,p.valid_until,coalesce(p.commercial_notes,''),p.shipping_included,coalesce(d.decision,''),d.decided_at FROM quote_proposals p JOIN quotes q ON q.id=p.quote_id LEFT JOIN quote_proposal_decisions d ON d.proposal_id=p.id WHERE q.user_id=$1 AND q.quote_number=$2 AND p.status IN('sent','expired') ORDER BY p.version DESC`, user, ref)
	if e != nil {
		return q, e
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Proposal
		if e = rows.Scan(&p.ID, &p.Version, &p.Status, &p.FinalPriceCents, &p.ProductionLeadTimeDays, &p.ValidUntil, &p.CommercialNotes, &p.ShippingIncluded, &p.Decision, &p.DecidedAt); e != nil {
			return q, e
		}
		q.Proposals = append(q.Proposals, p)
	}
	return q, rows.Err()
}
func (r *CustomerRepository) Decide(ctx context.Context, user, ref, proposalID, decision, session string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var quoteID, status string
	var version, maxVersion int
	var valid time.Time
	e = tx.QueryRow(ctx, `SELECT q.id,q.status,p.version,p.valid_until,(SELECT max(version) FROM quote_proposals WHERE quote_id=q.id) FROM quotes q JOIN quote_proposals p ON p.quote_id=q.id WHERE q.user_id=$1 AND q.quote_number=$2 AND p.id=$3 AND p.status='sent' FOR UPDATE OF q`, user, ref, proposalID).Scan(&quoteID, &status, &version, &valid, &maxVersion)
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrInvalidDecision
	}
	if e != nil {
		return e
	}
	if status != "quoted" || !valid.After(time.Now()) || version != maxVersion {
		return app.ErrInvalidDecision
	}
	var unsafe int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM quote_files WHERE quote_id=$1 AND security_status<>'clean'`, quoteID).Scan(&unsafe); e != nil {
		return e
	}
	if unsafe > 0 {
		return app.ErrInvalidDecision
	}
	tag, e := tx.Exec(ctx, `INSERT INTO quote_proposal_decisions(quote_id,proposal_id,user_id,decision) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, quoteID, proposalID, user, decision)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return app.ErrInvalidDecision
	}
	newStatus := "rejected"
	field := "rejected_at"
	if decision == "accepted" {
		newStatus = "approved"
		field = "approved_at"
	}
	_, e = tx.Exec(ctx, `UPDATE quotes SET status=$2,approved_at=CASE WHEN $3='approved' THEN now() ELSE approved_at END,rejected_at=CASE WHEN $3='rejected' THEN now() ELSE rejected_at END,updated_at=now() WHERE id=$1`, quoteID, newStatus, newStatus)
	_ = field
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO customer_audit_events(user_id,session_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,$2,'quote.proposal.'||$3,'quote',$4,'success',jsonb_build_object('proposal_id',$5::text,'version',$6::integer))`, user, session, decision, ref, proposalID, version)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (r *CustomerRepository) ConvertQuote(ctx context.Context, user, ref, session string) (domain.OrderResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.OrderResult{}, err
	}
	defer tx.Rollback(ctx)
	var quoteID, status, name, email, phone, material, color, dimensions, notes string
	var quantity int
	err = tx.QueryRow(ctx, `SELECT q.id,q.status,q.customer_name,q.customer_email,coalesce(q.customer_phone,''),q.requested_quantity,coalesce(m.name,''),coalesce(c.name,''),concat_ws(' × ',q.width_mm,q.height_mm,q.depth_mm),coalesce(q.notes,'') FROM quotes q LEFT JOIN materials m ON m.id=q.material_id LEFT JOIN colors c ON c.id=q.color_id WHERE q.user_id=$1 AND q.quote_number=$2 FOR UPDATE OF q`, user, ref).Scan(&quoteID, &status, &name, &email, &phone, &quantity, &material, &color, &dimensions, &notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrderResult{}, app.ErrNotFound
	}
	if err != nil {
		return domain.OrderResult{}, err
	}
	var existing domain.OrderResult
	err = tx.QueryRow(ctx, `SELECT order_number,status FROM orders WHERE quote_id=$1`, quoteID).Scan(&existing.Reference, &existing.Status)
	if err == nil {
		if e := tx.Commit(ctx); e != nil {
			return domain.OrderResult{}, e
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.OrderResult{}, err
	}
	if status != "approved" {
		return domain.OrderResult{}, app.ErrInvalidConversion
	}
	var proposalID, commercialNotes string
	var version, lead int
	var price int64
	var shippingIncluded bool
	var validUntil, decidedAt time.Time
	err = tx.QueryRow(ctx, `SELECT p.id,p.version,p.final_price_cents,p.production_lead_time_days,p.shipping_included,coalesce(p.commercial_notes,''),p.valid_until,d.decided_at FROM quote_proposal_decisions d JOIN quote_proposals p ON p.id=d.proposal_id WHERE d.quote_id=$1 AND d.user_id=$2 AND d.decision='accepted'`, quoteID, user).Scan(&proposalID, &version, &price, &lead, &shippingIncluded, &commercialNotes, &validUntil, &decidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrderResult{}, app.ErrInvalidConversion
	}
	if err != nil {
		return domain.OrderResult{}, err
	}
	var latest int
	if err = tx.QueryRow(ctx, `SELECT max(version) FROM quote_proposals WHERE quote_id=$1`, quoteID).Scan(&latest); err != nil {
		return domain.OrderResult{}, err
	}
	if version != latest || decidedAt.After(validUntil) {
		return domain.OrderResult{}, app.ErrInvalidConversion
	}
	orderNumber := "PED-" + strings.ToUpper(strings.ReplaceAll(quoteID, "-", "")[:16])
	var orderID string
	err = tx.QueryRow(ctx, `INSERT INTO orders(order_number,quote_id,user_id,status,customer_name,customer_email,customer_phone,subtotal_cents,shipping_cents,discount_cents,total_cents,notes,placed_at,quote_proposal_id,quote_proposal_version,production_lead_time_days,shipping_included,commercial_notes) VALUES($1,$2,$3,'pending_payment',$4,$5,nullif($6,''),$7,0,0,$7,$8,now(),$9,$10,$11,$12,nullif($13,'')) RETURNING id`, orderNumber, quoteID, user, name, email, phone, price, notes, proposalID, version, lead, shippingIncluded, commercialNotes).Scan(&orderID)
	if err != nil {
		return domain.OrderResult{}, err
	}
	unitPrice := price / int64(quantity)
	_, err = tx.Exec(ctx, `INSERT INTO order_items(order_id,item_type,product_name,description_snapshot,material_name,color_name,dimensions_snapshot,quantity,unit_price_cents,approved_total_cents) VALUES($1,'custom','Impressão 3D personalizada',$2,$3,$4,nullif($5,''),$6,$7,$8)`, orderID, "Serviço personalizado do orçamento "+ref+" — proposta v"+strconv.Itoa(version), material, color, dimensions, quantity, unitPrice, price)
	if err != nil {
		return domain.OrderResult{}, err
	}
	metadata := map[string]any{"quote_reference": ref, "proposal_id": proposalID, "proposal_version": version, "customer_user_id": user}
	raw, _ := json.Marshal(metadata)
	_, err = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,status,description,created_by_user_id,metadata) VALUES($1,'pending_payment','Pedido criado a partir de proposta aprovada',$2,$3)`, orderID, user, raw)
	if err != nil {
		return domain.OrderResult{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_audit_events(user_id,session_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,$2,'quote.convert','order',$3,'success',$4)`, user, session, orderNumber, raw)
	if err != nil {
		return domain.OrderResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.OrderResult{}, err
	}
	return domain.OrderResult{Reference: orderNumber, Status: "pending_payment"}, nil
}
