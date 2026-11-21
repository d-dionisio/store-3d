package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/admin"
	domain "example.com/loja-3d/backend/internal/domain/admin"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func newAdminRepositoryIntegrationPool(t *testing.T, label string) (*pgxpool.Pool, context.Context) {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Admin repository PostgreSQL integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, label)
	if err != nil {
		t.Fatalf("create isolated admin schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated admin schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated admin pool: %v", err)
	}
	return pool, ctx
}

func uniqueAdminFixtureSuffix(t *testing.T) string {
	t.Helper()
	id, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique admin fixture identifier")
	}
	return strings.ReplaceAll(id, "-", "")
}

func insertAdminFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix, label, role string) string {
	t.Helper()
	var id string
	email := label + "-" + suffix + "@example.invalid"
	if err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status)
		VALUES($1,$2,repeat('x',60),$3,'active') RETURNING id::text`, email, "Admin fixture "+suffix, role).Scan(&id); err != nil {
		t.Fatalf("insert isolated administrator fixture: %v", err)
	}
	return id
}

func TestLastActiveAdministratorGuardIntegration(t *testing.T) {
	pool, ctx := newAdminRepositoryIntegrationPool(t, "admin_last")
	suffix := uniqueAdminFixtureSuffix(t)
	onlyAdminID := insertAdminFixture(t, ctx, pool, suffix, "only-admin", "admin")
	repo := NewAdminRepository(pool)
	if _, err := repo.UpdateInternalUser(ctx, onlyAdminID, "quote_viewer", "active", onlyAdminID); !errors.Is(err, app.ErrLastAdmin) {
		t.Fatalf("last administrator was not protected: %v", err)
	}
	secondAdminID := insertAdminFixture(t, ctx, pool, suffix, "second-admin", "admin")
	if _, err := repo.UpdateInternalUser(ctx, onlyAdminID, "quote_viewer", "active", secondAdminID); err != nil {
		t.Fatalf("demotion with another active administrator failed: %v", err)
	}
}

func TestBootstrapAdminOneShotIntegration(t *testing.T) {
	pool, ctx := newAdminRepositoryIntegrationPool(t, "admin_bootstrap")
	suffix := uniqueAdminFixtureSuffix(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("Bootstrap-Integration-2026!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal("hash isolated bootstrap fixture password")
	}
	repo := NewAdminRepository(pool)
	created, err := repo.BootstrapAdmin(ctx, "bootstrap-"+suffix+"@example.invalid", "Bootstrap fixture "+suffix, string(hash))
	if err != nil || created.Role != "admin" || created.Status != "active" {
		t.Fatalf("bootstrap failed: role=%q status=%q err=%v", created.Role, created.Status, err)
	}
	if _, err = repo.BootstrapAdmin(ctx, "second-"+suffix+"@example.invalid", "Second bootstrap fixture", string(hash)); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("second bootstrap not blocked: %v", err)
	}
}

func TestInternalUserManagementAndBootstrapGuardIntegration(t *testing.T) {
	pool, ctx := newAdminRepositoryIntegrationPool(t, "admin_management")
	suffix := uniqueAdminFixtureSuffix(t)
	actorID := insertAdminFixture(t, ctx, pool, suffix, "actor", "admin")
	repo := NewAdminRepository(pool)
	service := app.New(repo)
	actor := adminSession(actorID, "admin")
	created, err := service.CreateInternalUser(ctx, actor, "operator-"+suffix+"@example.invalid", "Production Operator", "Operator-Password-2026!", "production_operator", "127.0.0.1")
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	operatorSession, token, _, err := service.Login(ctx, created.Email, "Operator-Password-2026!", "127.0.0.1", "test")
	if err != nil || token == "" {
		t.Fatalf("operator login: %v", err)
	}
	if _, err = service.CreateInternalUser(ctx, operatorSession, "forbidden-"+suffix+"@example.invalid", "Forbidden fixture", "Forbidden-Password-2026!", "admin", "127.0.0.1"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("privilege escalation not blocked: %v", err)
	}
	if _, err = service.UpdateInternalUser(ctx, actor, actorID, "quote_viewer", "active", "127.0.0.1"); !errors.Is(err, app.ErrInvalid) {
		t.Fatalf("self demotion not blocked: %v", err)
	}
	if err = service.ResetInternalUserPassword(ctx, actor, created.ID, "New-Operator-Password-2026!", "127.0.0.1"); err != nil {
		t.Fatal("reset isolated operator password:", err)
	}
	if _, err = service.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("password reset did not revoke session: %v", err)
	}
	updated, err := service.UpdateInternalUser(ctx, actor, created.ID, "production_operator", "blocked", "127.0.0.1")
	if err != nil || updated.Status != "blocked" {
		t.Fatalf("block user: status=%q err=%v", updated.Status, err)
	}
	if _, _, _, err = service.Login(ctx, created.Email, "New-Operator-Password-2026!", "127.0.0.1", "test"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("blocked account logged in: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("Bootstrap-Password-2026!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal("hash isolated bootstrap fixture password")
	}
	if _, err = repo.BootstrapAdmin(ctx, "bootstrap-"+suffix+"@example.invalid", "Bootstrap fixture", string(hash)); !errors.Is(err, app.ErrForbidden) {
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
	pool, ctx := newAdminRepositoryIntegrationPool(t, "admin_auth")
	suffix := uniqueAdminFixtureSuffix(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("Integration-Password-2026!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal("hash isolated authentication fixture password")
	}
	email := "admin-auth-" + suffix + "@example.invalid"
	var userID string
	err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,$3,'quote_viewer') RETURNING id::text`, email, "Admin auth fixture "+suffix, string(hash)).Scan(&userID)
	if err != nil {
		t.Fatal("insert isolated authentication fixture:", err)
	}
	repo := NewAdminRepository(pool)
	service := app.New(repo)
	session, token, csrf, err := service.Login(ctx, email, "Integration-Password-2026!", "127.0.0.1", "integration-test")
	if err != nil || token == "" || csrf == "" {
		t.Fatalf("login failed: %v", err)
	}
	authenticated, err := service.Authenticate(ctx, token)
	if err != nil || authenticated.UserID != userID {
		t.Fatalf("authentication failed: err=%v", err)
	}
	if _, err = service.Authenticate(ctx, "another-users-token"); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expected isolated unauthorized result, got %v", err)
	}
	if _, err = service.Stats(ctx, session, "127.0.0.1"); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if err = service.StartAnalysis(ctx, session, "ORC-NOT-USED-"+suffix, "127.0.0.1"); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("quote_viewer should not mutate proposals: %v", err)
	}
	if err = service.Logout(ctx, session, "127.0.0.1"); err != nil {
		t.Fatal("logout isolated session:", err)
	}
	if _, err = service.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("revoked session still valid: %v", err)
	}
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE admin_user_id=$1`, userID).Scan(&audits); err != nil || audits < 3 {
		t.Fatalf("expected audit events, count=%d err=%v", audits, err)
	}
}
