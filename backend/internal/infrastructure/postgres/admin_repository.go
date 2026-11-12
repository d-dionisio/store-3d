package postgres

import (
	"context"
	"encoding/json"
	"errors"
	app "example.com/loja-3d/backend/internal/application/admin"
	domain "example.com/loja-3d/backend/internal/domain/admin"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type AdminRepository struct{ pool *pgxpool.Pool }

func NewAdminRepository(pool *pgxpool.Pool) *AdminRepository { return &AdminRepository{pool: pool} }
func (r *AdminRepository) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.pool.QueryRow(ctx, `SELECT id,email,full_name,password_hash,role,status,locked_until FROM admin_users WHERE lower(email)=lower($1)`, email).Scan(&u.ID, &u.Email, &u.FullName, &u.PasswordHash, &u.Role, &u.Status, &u.LockedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, app.ErrUnauthorized
	}
	return u, err
}
func (r *AdminRepository) RecordLoginFailure(ctx context.Context, email, ip string) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_users SET failed_login_attempts=failed_login_attempts+1,locked_until=CASE WHEN failed_login_attempts+1>=5 THEN now()+interval '15 minutes' ELSE locked_until END WHERE lower(email)=lower($1)`, email)
	_ = r.Audit(ctx, nil, nil, "admin.login", "session", "", "denied", ip)
	return err
}
func (r *AdminRepository) CreateSession(ctx context.Context, userID, tokenHash, csrfHash, ip, ua string, expires time.Time) (domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, `WITH reset AS (UPDATE admin_users SET failed_login_attempts=0,locked_until=NULL,last_login_at=now() WHERE id=$1) INSERT INTO admin_sessions(admin_user_id,token_hash,csrf_token_hash,ip_address,user_agent,expires_at) VALUES($1,$2,$3,nullif($4,'')::inet,left($5,500),$6) RETURNING id,admin_user_id,expires_at`, userID, tokenHash, csrfHash, ip, ua, expires).Scan(&s.ID, &s.UserID, &s.ExpiresAt)
	return s, err
}
func (r *AdminRepository) GetSession(ctx context.Context, tokenHash string) (domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, `SELECT s.id,s.admin_user_id,u.email,u.full_name,u.role,s.csrf_token_hash,s.authentication_level,s.expires_at FROM admin_sessions s JOIN admin_users u ON u.id=s.admin_user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND s.authentication_level='mfa' AND u.status='active'`, tokenHash).Scan(&s.ID, &s.UserID, &s.Email, &s.FullName, &s.Role, &s.CSRFHash, &s.AuthenticationLevel, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, app.ErrUnauthorized
	}
	if err == nil {
		_, _ = r.pool.Exec(ctx, `UPDATE admin_sessions SET last_seen_at=now() WHERE id=$1`, s.ID)
	}
	return s, err
}
func (r *AdminRepository) RevokeSession(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE id=$1`, id)
	return err
}
func (r *AdminRepository) ListInternalUsers(ctx context.Context) ([]domain.InternalUser, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,email,full_name,role,status,last_login_at,created_at,updated_at FROM admin_users ORDER BY full_name,email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []domain.InternalUser{}
	for rows.Next() {
		var user domain.InternalUser
		if err = rows.Scan(&user.ID, &user.Email, &user.FullName, &user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
func (r *AdminRepository) CreateInternalUser(ctx context.Context, email, name, passwordHash, role, actorID string) (domain.InternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.InternalUser{}, err
	}
	defer tx.Rollback(ctx)
	var user domain.InternalUser
	err = tx.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,$3,$4) RETURNING id,email,full_name,role,status,last_login_at,created_at,updated_at`, email, name, passwordHash, role).Scan(&user.ID, &user.Email, &user.FullName, &user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return user, app.ErrConflict
	}
	if err != nil {
		return user, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,'admin_user.create','admin_user',$2,'success',jsonb_build_object('role',$3::text))`, actorID, user.ID, role)
	if err != nil {
		return user, err
	}
	err = tx.Commit(ctx)
	return user, err
}
func (r *AdminRepository) UpdateInternalUser(ctx context.Context, id, role, status, actorID string) (domain.InternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.InternalUser{}, err
	}
	defer tx.Rollback(ctx)
	var user domain.InternalUser
	err = tx.QueryRow(ctx, `UPDATE admin_users SET role=$2,status=$3,updated_at=now() WHERE id=$1 RETURNING id,email,full_name,role,status,last_login_at,created_at,updated_at`, id, role, status).Scan(&user.ID, &user.Email, &user.FullName, &user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, app.ErrNotFound
	}
	if err != nil {
		return user, err
	}
	if status != "active" {
		if _, err = tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE admin_user_id=$1 AND revoked_at IS NULL`, id); err != nil {
			return user, err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,'admin_user.update','admin_user',$2,'success',jsonb_build_object('role',$3::text,'status',$4::text))`, actorID, id, role, status)
	if err != nil {
		return user, err
	}
	err = tx.Commit(ctx)
	return user, err
}
func (r *AdminRepository) ResetInternalUserPassword(ctx context.Context, id, passwordHash, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE admin_users SET password_hash=$2,failed_login_attempts=0,locked_until=NULL,updated_at=now() WHERE id=$1`, id, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE admin_user_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome) VALUES($1,'admin_user.password_reset','admin_user',$2,'success')`, actorID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *AdminRepository) RevokeInternalUserSessions(ctx context.Context, id, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE admin_user_id=$1 AND revoked_at IS NULL`, id)
	if err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_users WHERE id=$1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return app.ErrNotFound
	}
	if _, err = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,'admin_user.sessions_revoke','admin_user',$2,'success',jsonb_build_object('revoked',$3::bigint))`, actorID, id, result.RowsAffected()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BootstrapAdmin is intentionally one-shot: the transaction-level advisory lock
// prevents concurrent bootstraps and any existing internal user permanently closes it.
func (r *AdminRepository) BootstrapAdmin(ctx context.Context, email, name, passwordHash string) (domain.InternalUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.InternalUser{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(74100321)`); err != nil {
		return domain.InternalUser{}, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&count); err != nil {
		return domain.InternalUser{}, err
	}
	if count != 0 {
		return domain.InternalUser{}, app.ErrForbidden
	}
	var user domain.InternalUser
	err = tx.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,$3,'admin','active') RETURNING id,email,full_name,role,status,last_login_at,created_at,updated_at`, email, name, passwordHash).Scan(&user.ID, &user.Email, &user.FullName, &user.Role, &user.Status, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return user, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,action,resource_type,resource_id,outcome) VALUES($1::uuid,'admin.bootstrap','admin_user',$1::uuid::text,'success')`, user.ID)
	if err != nil {
		return user, err
	}
	err = tx.Commit(ctx)
	return user, err
}
func (r *AdminRepository) Audit(ctx context.Context, userID, sessionID *string, action, resourceType, resourceID, outcome, ip string) error {
	metadata, _ := json.Marshal(map[string]string{})
	_, err := r.pool.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,session_id,action,resource_type,resource_id,outcome,ip_address,metadata) VALUES($1,$2,$3,nullif($4,''),nullif($5,''),$6,nullif($7,'')::inet,$8)`, userID, sessionID, action, resourceType, resourceID, outcome, ip, metadata)
	return err
}
func (r *AdminRepository) Stats(ctx context.Context) (domain.Stats, error) {
	var s domain.Stats
	err := r.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status='submitted'),count(*) FILTER(WHERE status='under_review'),count(*) FILTER(WHERE status='quoted'),count(*) FILTER(WHERE status='approved'),count(*) FILTER(WHERE status IN('rejected','cancelled')) FROM quotes`).Scan(&s.Total, &s.Waiting, &s.Reviewing, &s.Quoted, &s.Approved, &s.Rejected)
	return s, err
}
func (r *AdminRepository) ListQuotes(ctx context.Context, search, status, sort string, page, limit int) (domain.QuotePage, error) {
	order := map[string]string{"newest": "q.created_at DESC", "oldest": "q.created_at ASC", "reference": "q.quote_number ASC"}[sort]
	query := `SELECT q.quote_number,q.customer_name,q.created_at,m.name,q.requested_quantity,q.status,coalesce(min(qf.security_status),'pending'),count(*) OVER() FROM quotes q LEFT JOIN materials m ON m.id=q.material_id LEFT JOIN quote_files qf ON qf.quote_id=q.id WHERE ($1='' OR q.quote_number ILIKE '%'||$1||'%' OR q.customer_name ILIKE '%'||$1||'%' OR q.customer_email ILIKE '%'||$1||'%') AND ($2='' OR q.status=$2) GROUP BY q.id,m.name ORDER BY ` + order + ` LIMIT $3 OFFSET $4`
	rows, err := r.pool.Query(ctx, query, search, status, limit, (page-1)*limit)
	if err != nil {
		return domain.QuotePage{}, err
	}
	defer rows.Close()
	result := domain.QuotePage{Items: []domain.QuoteSummary{}, Page: page, Limit: limit}
	for rows.Next() {
		var item domain.QuoteSummary
		if err = rows.Scan(&item.Reference, &item.CustomerName, &item.CreatedAt, &item.MaterialName, &item.Quantity, &item.Status, &item.SecurityStatus, &result.Total); err != nil {
			return result, err
		}
		result.Items = append(result.Items, item)
	}
	return result, rows.Err()
}
func (r *AdminRepository) GetQuote(ctx context.Context, reference string) (domain.QuoteDetail, error) {
	var q domain.QuoteDetail
	err := r.pool.QueryRow(ctx, `SELECT q.quote_number,q.customer_name,q.customer_email,coalesce(q.customer_phone,''),coalesce(m.name,''),coalesce(c.name,''),q.requested_quantity,q.width_mm,q.height_mm,q.depth_mm,coalesce(q.notes,''),q.status,q.created_at FROM quotes q LEFT JOIN materials m ON m.id=q.material_id LEFT JOIN colors c ON c.id=q.color_id WHERE q.quote_number=$1`, reference).Scan(&q.Reference, &q.CustomerName, &q.CustomerEmail, &q.CustomerPhone, &q.MaterialName, &q.ColorName, &q.Quantity, &q.WidthMM, &q.HeightMM, &q.DepthMM, &q.Notes, &q.Status, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return q, app.ErrNotFound
	}
	if err != nil {
		return q, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,original_filename,file_extension,size_bytes,security_status,created_at FROM quote_files WHERE quote_id=(SELECT id FROM quotes WHERE quote_number=$1) ORDER BY created_at`, reference)
	if err != nil {
		return q, err
	}
	defer rows.Close()
	for rows.Next() {
		var f domain.QuoteFile
		if err = rows.Scan(&f.ID, &f.Filename, &f.Extension, &f.SizeBytes, &f.SecurityStatus, &f.CreatedAt); err != nil {
			return q, err
		}
		q.Files = append(q.Files, f)
		q.History = append(q.History, domain.HistoryEvent{Kind: "file_security", Status: f.SecurityStatus, At: f.CreatedAt})
	}
	if err = rows.Err(); err != nil {
		return q, err
	}
	rows.Close()
	proposalRows, err := r.pool.Query(ctx, `SELECT id,version,status,material_cost_cents,energy_cost_cents,machine_cost_cents,finishing_cost_cents,packaging_cost_cents,other_costs_cents,margin_basis_points,final_price_cents,production_lead_time_days,estimated_completion_date,valid_until,coalesce(commercial_notes,''),shipping_included,notification_status,created_at,updated_at,sent_at FROM quote_proposals WHERE quote_id=(SELECT id FROM quotes WHERE quote_number=$1) ORDER BY version DESC`, reference)
	if err != nil {
		return q, err
	}
	defer proposalRows.Close()
	for proposalRows.Next() {
		var p domain.Proposal
		if err = proposalRows.Scan(&p.ID, &p.Version, &p.Status, &p.MaterialCostCents, &p.EnergyCostCents, &p.MachineCostCents, &p.FinishingCostCents, &p.PackagingCostCents, &p.OtherCostsCents, &p.MarginBasisPoints, &p.FinalPriceCents, &p.ProductionLeadTimeDays, &p.EstimatedCompletionDate, &p.ValidUntil, &p.CommercialNotes, &p.ShippingIncluded, &p.NotificationStatus, &p.CreatedAt, &p.UpdatedAt, &p.SentAt); err != nil {
			return q, err
		}
		q.Proposals = append(q.Proposals, p)
		q.History = append(q.History, domain.HistoryEvent{Kind: "proposal", Status: p.Status, At: p.UpdatedAt})
	}
	q.History = append(q.History, domain.HistoryEvent{Kind: "quote", Status: q.Status, At: q.CreatedAt})
	return q, proposalRows.Err()
}

var _ = fmt.Sprintf
