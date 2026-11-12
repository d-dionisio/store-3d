package postgres

import (
	"context"
	"errors"
	app "example.com/loja-3d/backend/internal/application/admin"
	domain "example.com/loja-3d/backend/internal/domain/admin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"testing"
	"time"
)

func adminPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	config, parseErr := pgxpool.ParseConfig(dsn)
	if parseErr != nil || !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		t.Fatal("TEST_DATABASE_URL must reference a database ending in _test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestLastActiveAdministratorGuardIntegration(t *testing.T) {
	ctx := context.Background()
	base := adminPool(t)
	schema := "last_admin_" + time.Now().Format("150405000000000")
	identifier := pgx.Identifier{schema}.Sanitize()
	_, err := base.Exec(ctx, `CREATE SCHEMA `+identifier)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, `DROP SCHEMA IF EXISTS `+identifier+` CASCADE`) })
	_, err = base.Exec(ctx, `CREATE TABLE `+identifier+`.admin_users (id UUID PRIMARY KEY DEFAULT gen_random_uuid(),email TEXT NOT NULL,full_name TEXT NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'active',last_login_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()); CREATE TABLE `+identifier+`.admin_sessions (admin_user_id UUID,revoked_at TIMESTAMPTZ,expires_at TIMESTAMPTZ); CREATE TABLE `+identifier+`.admin_audit_events (admin_user_id UUID,action TEXT,resource_type TEXT,resource_id TEXT,outcome TEXT,metadata JSONB)`)
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var onlyID string
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES('only@example.test','Only Admin',repeat('x',60),'admin','active') RETURNING id`).Scan(&onlyID); err != nil {
		t.Fatal(err)
	}
	repo := NewAdminRepository(pool)
	if _, err = repo.UpdateInternalUser(ctx, onlyID, "quote_viewer", "active", onlyID); !errors.Is(err, app.ErrLastAdmin) {
		t.Fatalf("last administrator was not protected: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES('second@example.test','Second Admin',repeat('x',60),'admin','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateInternalUser(ctx, onlyID, "quote_viewer", "active", onlyID); err != nil {
		t.Fatalf("demotion with another active administrator failed: %v", err)
	}
}
func TestBootstrapAdminOneShotIntegration(t *testing.T) {
	ctx := context.Background()
	base := adminPool(t)
	schema := "bootstrap_" + time.Now().Format("150405000000000")
	identifier := pgx.Identifier{schema}.Sanitize()
	_, err := base.Exec(ctx, `CREATE SCHEMA `+identifier)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(ctx, `DROP SCHEMA IF EXISTS `+identifier+` CASCADE`) })
	_, err = base.Exec(ctx, `CREATE TABLE `+identifier+`.admin_users (id UUID PRIMARY KEY DEFAULT gen_random_uuid(),email TEXT NOT NULL,full_name TEXT NOT NULL,password_hash TEXT NOT NULL,role TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'active',failed_login_attempts INTEGER NOT NULL DEFAULT 0,locked_until TIMESTAMPTZ,last_login_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()); CREATE TABLE `+identifier+`.admin_audit_events (id UUID PRIMARY KEY DEFAULT gen_random_uuid(),admin_user_id UUID,action TEXT NOT NULL,resource_type TEXT,resource_id TEXT,outcome TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	if err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	isolated, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer isolated.Close()
	repo := NewAdminRepository(isolated)
	hash, _ := bcrypt.GenerateFromPassword([]byte("Bootstrap-Integration-2026!"), bcrypt.MinCost)
	created, err := repo.BootstrapAdmin(ctx, "first@example.com", "First Admin", string(hash))
	if err != nil || created.Role != "admin" || created.Status != "active" {
		t.Fatalf("bootstrap failed: user=%+v err=%v", created, err)
	}
	if _, err = repo.BootstrapAdmin(ctx, "second@example.com", "Second Admin", string(hash)); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("second bootstrap not blocked: %v", err)
	}
}

func TestInternalUserManagementAndBootstrapGuardIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	suffix := time.Now().Format("150405.000000000")
	actorHash, _ := bcrypt.GenerateFromPassword([]byte("Actor-Integration-2026!"), bcrypt.MinCost)
	var actorID string
	err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Actor Admin',$2,'admin') RETURNING id`, "actor-"+suffix+"@example.com", string(actorHash)).Scan(&actorID)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewAdminRepository(pool)
	service := app.New(repo)
	actor := adminSession(actorID, "admin")
	created, err := service.CreateInternalUser(ctx, actor, "operator-"+suffix+"@example.com", "Production Operator", "Operator-Password-2026!", "production_operator", "127.0.0.1")
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id IN($1,$2)`, actorID, created.ID) })
	operatorSession, token, _, err := service.Login(ctx, created.Email, "Operator-Password-2026!", "127.0.0.1", "test")
	if err != nil || token == "" {
		t.Fatalf("operator login: %v", err)
	}
	if _, err = service.CreateInternalUser(ctx, operatorSession, "forbidden@example.com", "Forbidden", "Forbidden-Password-2026!", "admin", "127.0.0.1"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("privilege escalation not blocked: %v", err)
	}
	if _, err = service.UpdateInternalUser(ctx, actor, actorID, "quote_viewer", "active", "127.0.0.1"); !errors.Is(err, app.ErrInvalid) {
		t.Fatalf("self demotion not blocked: %v", err)
	}
	if err = service.ResetInternalUserPassword(ctx, actor, created.ID, "New-Operator-Password-2026!", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("password reset did not revoke session: %v", err)
	}
	updated, err := service.UpdateInternalUser(ctx, actor, created.ID, "production_operator", "blocked", "127.0.0.1")
	if err != nil || updated.Status != "blocked" {
		t.Fatalf("block user: %v", err)
	}
	if _, _, _, err = service.Login(ctx, created.Email, "New-Operator-Password-2026!", "127.0.0.1", "test"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("blocked account logged in: %v", err)
	}
	bootstrapHash, _ := bcrypt.GenerateFromPassword([]byte("Bootstrap-Password-2026!"), bcrypt.MinCost)
	if _, err = repo.BootstrapAdmin(ctx, "bootstrap@example.com", "Bootstrap", string(bootstrapHash)); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("bootstrap guard failed: %v", err)
	}
	var auditCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE admin_user_id=$1 AND action LIKE 'admin_user.%'`, actorID).Scan(&auditCount); err != nil || auditCount < 3 {
		t.Fatalf("missing management audit: count=%d err=%v", auditCount, err)
	}
}

func adminSession(id, role string) (session domain.Session) {
	session.UserID, session.Role = id, role
	return
}
func TestAdminAuthenticationAuthorizationAndAuditIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("Integration-Password-2026!"), bcrypt.MinCost)
	var userID string
	err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES('admin-test@example.com','Admin Test',$1,'quote_viewer') ON CONFLICT((lower(email))) DO UPDATE SET password_hash=excluded.password_hash,status='active' RETURNING id`, string(hash)).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id=$1`, userID) })
	repo := NewAdminRepository(pool)
	service := app.New(repo)
	session, token, csrf, err := service.Login(ctx, "admin-test@example.com", "Integration-Password-2026!", "127.0.0.1", "integration-test")
	if err != nil || token == "" || csrf == "" {
		t.Fatalf("login failed: %v", err)
	}
	authenticated, err := service.Authenticate(ctx, token)
	if err != nil || authenticated.UserID != userID {
		t.Fatalf("authentication failed: %v", err)
	}
	if _, err = service.Authenticate(ctx, "another-users-token"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expected isolated unauthorized token, got %v", err)
	}
	if _, err = service.Stats(ctx, session, "127.0.0.1"); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if err = service.StartAnalysis(ctx, session, "ORC-NOT-USED", "127.0.0.1"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("quote_viewer should not mutate proposals: %v", err)
	}
	if err = service.Logout(ctx, session, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("revoked session still valid: %v", err)
	}
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE admin_user_id=$1`, userID).Scan(&audits); err != nil || audits < 3 {
		t.Fatalf("expected audit events, count=%d err=%v", audits, err)
	}
}
