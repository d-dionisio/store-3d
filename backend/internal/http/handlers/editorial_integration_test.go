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
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestEditorialHandlerAuthenticationCSRFAndAuditIntegration(t *testing.T) {
	if _, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "editorial_http")
	if err != nil {
		t.Fatalf("create isolated editorial HTTP schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to editorial HTTP schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated editorial HTTP pool: %v", err)
	}
	var userID string
	suffix := time.Now().UnixNano()
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Editorial HTTP fixture',repeat('x',60),'admin') RETURNING id::text`, fmt.Sprintf("editorial-http-%d@example.test", suffix)).Scan(&userID); err != nil {
		t.Fatalf("create isolated editorial admin fixture: %v", err)
	}
	adminRepository := postgres.NewAdminRepository(pool)
	adminService := adminapp.New(adminRepository)
	service := editorialapp.New(postgres.NewEditorialRepository(pool), postgres.NewCatalogRepository(pool))
	handler := NewEditorialHandler(service, adminService)
	rawToken, rawCSRF := fmt.Sprintf("editorial-session-%d", suffix), fmt.Sprintf("editorial-csrf-%d", suffix)
	catalogSlug := fmt.Sprintf("handler-integration-%d", suffix)
	session, err := adminRepository.CreateSession(ctx, userID, adminapp.Hash(rawToken), adminapp.Hash(rawCSRF), "127.0.0.1", "integration test", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

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
