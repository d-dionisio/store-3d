package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	editorialapp "example.com/loja-3d/backend/internal/application/editorial"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEditorialHandlerAuthenticationCSRFAndAuditIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var userID string
	if err = pool.QueryRow(ctx, `SELECT id FROM admin_users WHERE role='admin' AND status='active' LIMIT 1`).Scan(&userID); err != nil {
		t.Skip("active admin fixture is required")
	}
	adminRepository := postgres.NewAdminRepository(pool)
	adminService := adminapp.New(adminRepository)
	service := editorialapp.New(postgres.NewEditorialRepository(pool), postgres.NewCatalogRepository(pool))
	handler := NewEditorialHandler(service, adminService)
	suffix := time.Now().UnixNano()
	rawToken, rawCSRF := fmt.Sprintf("editorial-session-%d", suffix), fmt.Sprintf("editorial-csrf-%d", suffix)
	catalogSlug := fmt.Sprintf("handler-integration-%d", suffix)
	session, err := adminRepository.CreateSession(ctx, userID, adminapp.Hash(rawToken), adminapp.Hash(rawCSRF), "127.0.0.1", "integration test", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM editorial_catalogs WHERE slug=$1`, catalogSlug)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_audit_events WHERE session_id=$1`, session.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_sessions WHERE id=$1`, session.ID)
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/catalogs", nil)
	response := httptest.NewRecorder()
	handler.AdminList(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", response.Code)
	}
	body := fmt.Sprintf(`{"slug":%q,"title":"Handler integration","description":"Integration test","eyebrow":"Edition"}`, catalogSlug)
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/catalogs", strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: rawToken})
	response = httptest.NewRecorder()
	handler.Create(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/catalogs", strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: rawToken})
	request.Header.Set("X-CSRF-Token", rawCSRF)
	response = httptest.NewRecorder()
	handler.Create(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE session_id=$1 AND action='catalogs.create' AND outcome='success'`, session.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit count=%d err=%v", audits, err)
	}
}
