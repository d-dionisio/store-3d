package postgres

import (
	"context"
	"errors"
	"time"

	app "example.com/loja-3d/backend/internal/application/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IdentityRepository struct{ pool *pgxpool.Pool }

func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}
func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return app.ErrUnauthorized
	}
	return err
}
func conflict(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23505" {
		return app.ErrConflict
	}
	return err
}

func (r *IdentityRepository) FindIdentity(ctx context.Context, kind, email string) (app.Identity, error) {
	var v app.Identity
	v.Kind = kind
	var verified *time.Time
	var err error
	if kind == "admin" {
		err = r.pool.QueryRow(ctx, `SELECT id,email,full_name,role,status,password_hash,locked_until FROM admin_users WHERE lower(email)=lower($1)`, email).Scan(&v.ID, &v.Email, &v.FullName, &v.Role, &v.Status, &v.PasswordHash, &v.LockedUntil)
		v.EmailVerified = true
	} else {
		err = r.pool.QueryRow(ctx, `SELECT id,email,full_name,''::text,status,coalesce(password_hash,''),locked_until,email_verified_at FROM users WHERE lower(email)=lower($1)`, email).Scan(&v.ID, &v.Email, &v.FullName, &v.Role, &v.Status, &v.PasswordHash, &v.LockedUntil, &verified)
		v.EmailVerified = verified != nil
	}
	return v, noRows(err)
}
func (r *IdentityRepository) RegisterCustomer(ctx context.Context, name, email, passwordHash, tokenHash string, expires time.Time) (app.Identity, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return app.Identity{}, e
	}
	defer tx.Rollback(ctx)
	var v app.Identity
	v.Kind = "customer"
	e = tx.QueryRow(ctx, `INSERT INTO users(email,full_name,password_hash,password_changed_at,status) VALUES(lower($1),$2,$3,now(),'active') RETURNING id,email,full_name,status`, email, name, passwordHash).Scan(&v.ID, &v.Email, &v.FullName, &v.Status)
	if e != nil {
		return v, conflict(e)
	}
	_, e = tx.Exec(ctx, `INSERT INTO customer_auth_tokens(user_id,purpose,token_hash,expires_at) VALUES($1,'verify_email',$2,$3)`, v.ID, tokenHash, expires)
	if e != nil {
		return v, e
	}
	return v, tx.Commit(ctx)
}
func (r *IdentityRepository) CreateCustomerToken(ctx context.Context, userID, purpose, hash string, expires time.Time) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, `UPDATE customer_auth_tokens SET consumed_at=coalesce(consumed_at,now()) WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL`, userID, purpose)
	if e == nil {
		_, e = tx.Exec(ctx, `INSERT INTO customer_auth_tokens(user_id,purpose,token_hash,expires_at) VALUES($1,$2,$3,$4)`, userID, purpose, hash, expires)
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *IdentityRepository) ConsumeEmailVerification(ctx context.Context, hash string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var user string
	e = tx.QueryRow(ctx, `UPDATE customer_auth_tokens SET consumed_at=now() WHERE token_hash=$1 AND purpose='verify_email' AND consumed_at IS NULL AND expires_at>now() RETURNING user_id`, hash).Scan(&user)
	if e != nil {
		return noRows(e)
	}
	_, e = tx.Exec(ctx, `UPDATE users SET email_verified_at=coalesce(email_verified_at,now()),updated_at=now() WHERE id=$1`, user)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *IdentityRepository) ResetPassword(ctx context.Context, hash, passwordHash string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var user string
	e = tx.QueryRow(ctx, `UPDATE customer_auth_tokens SET consumed_at=now() WHERE token_hash=$1 AND purpose='reset_password' AND consumed_at IS NULL AND expires_at>now() RETURNING user_id`, hash).Scan(&user)
	if e != nil {
		return noRows(e)
	}
	_, e = tx.Exec(ctx, `UPDATE users SET password_hash=$2,password_changed_at=now(),failed_login_attempts=0,locked_until=NULL,updated_at=now() WHERE id=$1`, user, passwordHash)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE customer_auth_tokens SET consumed_at=coalesce(consumed_at,now()) WHERE user_id=$1 AND consumed_at IS NULL`, user)
	}
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE customer_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE user_id=$1`, user)
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *IdentityRepository) UpdatePassword(ctx context.Context, kind, id, hash string) error {
	if kind == "admin" {
		_, e := r.pool.Exec(ctx, `UPDATE admin_users SET password_hash=$2,updated_at=now() WHERE id=$1`, id, hash)
		return e
	}
	_, e := r.pool.Exec(ctx, `UPDATE users SET password_hash=$2,password_changed_at=now(),updated_at=now() WHERE id=$1`, id, hash)
	return e
}
func (r *IdentityRepository) RecordFailure(ctx context.Context, kind, email, ip string) error {
	if kind == "admin" {
		_, e := r.pool.Exec(ctx, `UPDATE admin_users SET failed_login_attempts=failed_login_attempts+1,locked_until=CASE WHEN failed_login_attempts+1>=5 THEN now()+least(interval '30 minutes',interval '1 minute'*power(2,least(failed_login_attempts-4,4))) ELSE locked_until END WHERE lower(email)=lower($1)`, email)
		return e
	}
	_, e := r.pool.Exec(ctx, `UPDATE users SET failed_login_attempts=failed_login_attempts+1,locked_until=CASE WHEN failed_login_attempts+1>=5 THEN now()+least(interval '30 minutes',interval '1 minute'*power(2,least(failed_login_attempts-4,4))) ELSE locked_until END WHERE lower(email)=lower($1)`, email)
	return e
}
func (r *IdentityRepository) UpsertPendingFactor(ctx context.Context, kind, id string, ciphertext []byte, version int16) (app.Factor, error) {
	var f app.Factor
	f.Kind, f.IdentityID = kind, id
	column := "user_id"
	if kind == "admin" {
		column = "admin_user_id"
	}
	query := `INSERT INTO auth_mfa_factors(` + column + `,secret_ciphertext,key_version,enabled_at,last_accepted_counter) VALUES($1,$2,$3,NULL,NULL) ON CONFLICT (` + column + `) WHERE ` + column + ` IS NOT NULL DO UPDATE SET secret_ciphertext=excluded.secret_ciphertext,key_version=excluded.key_version,enabled_at=NULL,last_accepted_counter=NULL,updated_at=now() RETURNING id,secret_ciphertext,key_version,enabled_at IS NOT NULL,last_accepted_counter`
	e := r.pool.QueryRow(ctx, query, id, ciphertext, version).Scan(&f.ID, &f.Ciphertext, &f.KeyVersion, &f.Enabled, &f.LastCounter)
	return f, e
}
func (r *IdentityRepository) GetFactor(ctx context.Context, kind, id string) (app.Factor, error) {
	var f app.Factor
	f.Kind, f.IdentityID = kind, id
	column := "user_id"
	if kind == "admin" {
		column = "admin_user_id"
	}
	e := r.pool.QueryRow(ctx, `SELECT id,secret_ciphertext,key_version,enabled_at IS NOT NULL,last_accepted_counter FROM auth_mfa_factors WHERE `+column+`=$1`, id).Scan(&f.ID, &f.Ciphertext, &f.KeyVersion, &f.Enabled, &f.LastCounter)
	return f, noRows(e)
}
func (r *IdentityRepository) CreateChallenge(ctx context.Context, kind, id, purpose, hash, ip, ua string, expires time.Time) error {
	column := "user_id"
	if kind == "admin" {
		column = "admin_user_id"
	}
	_, e := r.pool.Exec(ctx, `INSERT INTO auth_challenges(`+column+`,purpose,token_hash,ip_address,user_agent,expires_at) VALUES($1,$2,$3,nullif($4,'')::inet,left($5,500),$6)`, id, purpose, hash, ip, ua, expires)
	return e
}
func (r *IdentityRepository) GetChallenge(ctx context.Context, hash string) (app.Challenge, error) {
	var c app.Challenge
	var adminID, userID *string
	var consumed *time.Time
	e := r.pool.QueryRow(ctx, `UPDATE auth_challenges SET attempts=attempts+1 WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() AND attempts<10 RETURNING id,admin_user_id,user_id,purpose,attempts,expires_at,consumed_at`, hash).Scan(&c.ID, &adminID, &userID, &c.Purpose, &c.Attempts, &c.ExpiresAt, &consumed)
	if adminID != nil {
		c.Kind = "admin"
		c.IdentityID = *adminID
	} else if userID != nil {
		c.Kind = "customer"
		c.IdentityID = *userID
	}
	c.Consumed = consumed != nil
	return c, noRows(e)
}
func createIdentitySession(ctx context.Context, tx pgx.Tx, kind, id, tokenHash, csrfHash, ip, ua string, expires time.Time) (app.Session, error) {
	var s app.Session
	s.Kind, s.IdentityID = kind, id
	if kind == "admin" {
		e := tx.QueryRow(ctx, `INSERT INTO admin_sessions(admin_user_id,token_hash,csrf_token_hash,ip_address,user_agent,expires_at,authentication_level) VALUES($1,$2,$3,nullif($4,'')::inet,left($5,500),$6,'mfa') RETURNING id,expires_at`, id, tokenHash, csrfHash, ip, ua, expires).Scan(&s.ID, &s.ExpiresAt)
		if e == nil {
			e = tx.QueryRow(ctx, `SELECT email,full_name,role FROM admin_users WHERE id=$1`, id).Scan(&s.Email, &s.FullName, &s.Role)
		}
		return s, e
	}
	level := "password"
	e := tx.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,ip_address,user_agent,expires_at,authentication_level) VALUES($1,$2,$3,nullif($4,'')::inet,left($5,500),$6,$7) RETURNING id,expires_at`, id, tokenHash, csrfHash, ip, ua, expires, level).Scan(&s.ID, &s.ExpiresAt)
	if e == nil {
		e = tx.QueryRow(ctx, `SELECT email,full_name FROM users WHERE id=$1`, id).Scan(&s.Email, &s.FullName)
	}
	return s, e
}
func (r *IdentityRepository) CreateSession(ctx context.Context, kind, id, tokenHash, csrfHash, ip, ua string, expires time.Time) (app.Session, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return app.Session{}, e
	}
	defer tx.Rollback(ctx)
	s, e := createIdentitySession(ctx, tx, kind, id, tokenHash, csrfHash, ip, ua, expires)
	if e != nil {
		return s, e
	}
	if kind == "admin" {
		_, e = tx.Exec(ctx, `UPDATE admin_users SET failed_login_attempts=0,locked_until=NULL,last_login_at=now() WHERE id=$1`, id)
	} else {
		_, e = tx.Exec(ctx, `UPDATE users SET failed_login_attempts=0,locked_until=NULL WHERE id=$1`, id)
	}
	if e != nil {
		return s, e
	}
	return s, tx.Commit(ctx)
}
func (r *IdentityRepository) EnableFactorAndSession(ctx context.Context, challengeHash, factorID string, counter int64, codeHashes []string, tokenHash, csrfHash, ip, ua string, expires time.Time) (app.Session, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return app.Session{}, e
	}
	defer tx.Rollback(ctx)
	var kind, id, purpose string
	var adminID, userID *string
	e = tx.QueryRow(ctx, `SELECT admin_user_id,user_id,purpose FROM auth_challenges WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, challengeHash).Scan(&adminID, &userID, &purpose)
	if e != nil {
		return app.Session{}, noRows(e)
	}
	if adminID != nil {
		kind, id = "admin", *adminID
	} else {
		kind, id = "customer", *userID
	}
	if purpose != kind+"_mfa_setup" {
		return app.Session{}, app.ErrUnauthorized
	}
	tag, e := tx.Exec(ctx, `UPDATE auth_mfa_factors SET enabled_at=now(),last_accepted_counter=$2,updated_at=now() WHERE id=$1 AND enabled_at IS NULL`, factorID, counter)
	if e != nil || tag.RowsAffected() != 1 {
		return app.Session{}, app.ErrUnauthorized
	}
	_, e = tx.Exec(ctx, `DELETE FROM auth_recovery_codes WHERE factor_id=$1`, factorID)
	if e != nil {
		return app.Session{}, e
	}
	for _, hash := range codeHashes {
		if _, e = tx.Exec(ctx, `INSERT INTO auth_recovery_codes(factor_id,code_hash) VALUES($1,$2)`, factorID, hash); e != nil {
			return app.Session{}, e
		}
	}
	_, e = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE token_hash=$1`, challengeHash)
	if e != nil {
		return app.Session{}, e
	}
	s, e := createIdentitySession(ctx, tx, kind, id, tokenHash, csrfHash, ip, ua, expires)
	if e != nil {
		return s, e
	}
	return s, tx.Commit(ctx)
}
func (r *IdentityRepository) CompleteMFAAndSession(ctx context.Context, challengeHash string, counter *int64, recoveryHash, tokenHash, csrfHash, ip, ua string, expires time.Time) (app.Session, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return app.Session{}, e
	}
	defer tx.Rollback(ctx)
	var kind, id, purpose string
	var adminID, userID *string
	e = tx.QueryRow(ctx, `SELECT admin_user_id,user_id,purpose FROM auth_challenges WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now() FOR UPDATE`, challengeHash).Scan(&adminID, &userID, &purpose)
	if e != nil {
		return app.Session{}, noRows(e)
	}
	if adminID != nil {
		kind, id = "admin", *adminID
	} else {
		kind, id = "customer", *userID
	}
	if purpose != kind+"_mfa_login" && purpose != kind+"_strong_reauth" {
		return app.Session{}, app.ErrUnauthorized
	}
	var factor string
	e = tx.QueryRow(ctx, `SELECT id FROM auth_mfa_factors WHERE (($1='admin' AND admin_user_id=$2) OR ($1='customer' AND user_id=$2)) AND enabled_at IS NOT NULL FOR UPDATE`, kind, id).Scan(&factor)
	if e != nil {
		return app.Session{}, noRows(e)
	}
	if counter != nil {
		tag, e := tx.Exec(ctx, `UPDATE auth_mfa_factors SET last_accepted_counter=$2,updated_at=now() WHERE id=$1 AND (last_accepted_counter IS NULL OR last_accepted_counter<$2)`, factor, *counter)
		if e != nil || tag.RowsAffected() != 1 {
			return app.Session{}, app.ErrUnauthorized
		}
	} else {
		tag, e := tx.Exec(ctx, `UPDATE auth_recovery_codes SET used_at=now() WHERE factor_id=$1 AND code_hash=$2 AND used_at IS NULL`, factor, recoveryHash)
		if e != nil || tag.RowsAffected() != 1 {
			return app.Session{}, app.ErrUnauthorized
		}
	}
	_, e = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE token_hash=$1`, challengeHash)
	if e != nil {
		return app.Session{}, e
	}
	s, e := createIdentitySession(ctx, tx, kind, id, tokenHash, csrfHash, ip, ua, expires)
	if e != nil {
		return s, e
	}
	return s, tx.Commit(ctx)
}
func (r *IdentityRepository) MFAStatus(ctx context.Context, kind, id string) (bool, int, error) {
	column := "user_id"
	if kind == "admin" {
		column = "admin_user_id"
	}
	var enabled bool
	var count int
	e := r.pool.QueryRow(ctx, `SELECT enabled_at IS NOT NULL,(SELECT count(*) FROM auth_recovery_codes WHERE factor_id=f.id AND used_at IS NULL) FROM auth_mfa_factors f WHERE `+column+`=$1`, id).Scan(&enabled, &count)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, 0, nil
	}
	return enabled, count, e
}
func (r *IdentityRepository) DisableMFA(ctx context.Context, kind, id string) error {
	if kind == "admin" {
		return app.ErrInvalid
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, `DELETE FROM auth_mfa_factors WHERE user_id=$1`, id)
	if e == nil {
		_, e = tx.Exec(ctx, `UPDATE customer_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE user_id=$1`, id)
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *IdentityRepository) RevokeSessions(ctx context.Context, kind, id string) error {
	if kind == "admin" {
		_, e := r.pool.Exec(ctx, `UPDATE admin_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE admin_user_id=$1`, id)
		return e
	}
	_, e := r.pool.Exec(ctx, `UPDATE customer_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE user_id=$1`, id)
	return e
}
func (r *IdentityRepository) RegenerateRecoveryCodes(ctx context.Context, kind, id string, hashes []string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var factor string
	column := "user_id"
	if kind == "admin" {
		column = "admin_user_id"
	}
	e = tx.QueryRow(ctx, `SELECT id FROM auth_mfa_factors WHERE `+column+`=$1 AND enabled_at IS NOT NULL FOR UPDATE`, id).Scan(&factor)
	if e != nil {
		return noRows(e)
	}
	_, e = tx.Exec(ctx, `DELETE FROM auth_recovery_codes WHERE factor_id=$1`, factor)
	if e != nil {
		return e
	}
	for _, hash := range hashes {
		if _, e = tx.Exec(ctx, `INSERT INTO auth_recovery_codes(factor_id,code_hash) VALUES($1,$2)`, factor, hash); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
func (r *IdentityRepository) SecurityEvent(ctx context.Context, kind string, id *string, event, outcome, ip string) error {
	var adminID, userID *string
	if kind == "admin" {
		adminID = id
	} else if kind == "customer" {
		userID = id
	}
	_, e := r.pool.Exec(ctx, `INSERT INTO security_events(actor_type,admin_user_id,user_id,event_type,outcome,ip_address) VALUES($1,$2,$3,$4,$5,nullif($6,'')::inet)`, kind, adminID, userID, event, outcome, ip)
	return e
}
