package postgres

import (
	"context"
	"errors"
	app "example.com/loja-3d/backend/internal/application/admin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"os"
	"testing"
)

func adminPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
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
