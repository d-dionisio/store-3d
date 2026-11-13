package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	productionapp "example.com/loja-3d/backend/internal/application/production"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionHandlerAuthCSRFRoleAndDTOIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		t.Fatal("production HTTP integration requires a *_test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	adminRepository := postgres.NewAdminRepository(pool)
	adminService := adminapp.New(adminRepository)
	productionService := productionapp.New(postgres.NewProductionRepository(pool))
	handler := NewProductionHandler(productionService, adminService)
	suffix := time.Now().UnixNano()
	type credentials struct{ token, csrf, sessionID, userID string }
	credentialsByRole := map[string]credentials{}
	for _, role := range []string{"admin", "production_manager", "production_operator", "quote_viewer"} {
		var userID string
		err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,repeat('x',60),$3) RETURNING id`, fmt.Sprintf("production-http-%s-%d@example.test", role, suffix), "HTTP "+role, role).Scan(&userID)
		if err != nil {
			t.Fatal(err)
		}
		token := fmt.Sprintf("production-http-token-%s-%d", role, suffix)
		csrf := fmt.Sprintf("production-http-csrf-%s-%d", role, suffix)
		session, createErr := adminRepository.CreateSession(ctx, userID, adminapp.Hash(token), adminapp.Hash(csrf), "127.0.0.1", "integration test", time.Now().Add(time.Hour))
		if createErr != nil {
			t.Fatal(createErr)
		}
		credentialsByRole[role] = credentials{token: token, csrf: csrf, sessionID: session.ID, userID: userID}
	}
	printerCode := fmt.Sprintf("HTTP-PRINTER-%d", suffix)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM printers WHERE code=$1`, printerCode)
		for _, value := range credentialsByRole {
			_, _ = pool.Exec(ctx, `DELETE FROM admin_audit_events WHERE session_id=$1 OR admin_user_id=$2`, value.sessionID, value.userID)
			_, _ = pool.Exec(ctx, `DELETE FROM admin_sessions WHERE id=$1`, value.sessionID)
			_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id=$1`, value.userID)
		}
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/jobs?page=1&limit=2", nil)
	response := httptest.NewRecorder()
	handler.ListJobs(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", response.Code)
	}

	callList := func(role string) *httptest.ResponseRecorder {
		t.Helper()
		value := credentialsByRole[role]
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/jobs?page=1&limit=2", nil)
		req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: value.token})
		res := httptest.NewRecorder()
		handler.ListJobs(res, req)
		return res
	}
	if got := callList("quote_viewer"); got.Code != http.StatusForbidden {
		t.Fatalf("quote viewer status=%d body=%s", got.Code, got.Body.String())
	}
	for _, role := range []string{"admin", "production_manager", "production_operator"} {
		got := callList(role)
		if got.Code != http.StatusOK {
			t.Fatalf("%s list status=%d body=%s", role, got.Code, got.Body.String())
		}
		var body struct {
			Data struct {
				Items []productionapp.JobView `json:"items"`
				Page  int                     `json:"page"`
				Limit int                     `json:"limit"`
				Total int64                   `json:"total"`
			} `json:"data"`
		}
		if err = json.Unmarshal(got.Body.Bytes(), &body); err != nil || body.Data.Page != 1 || body.Data.Limit != 2 || body.Data.Items == nil {
			t.Fatalf("%s invalid page DTO: %+v err=%v", role, body, err)
		}
	}

	printerBody := fmt.Sprintf(`{"code":%q,"name":"HTTP Printer","manufacturer":"Bambu Lab","model":"A1","build_width_mm":256,"build_depth_mm":256,"build_height_mm":256,"nozzle_diameter_microns":400}`, printerCode)
	callCreatePrinter := func(role string, includeCSRF bool) *httptest.ResponseRecorder {
		t.Helper()
		value := credentialsByRole[role]
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/production/printers", strings.NewReader(printerBody))
		req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: value.token})
		if includeCSRF {
			req.Header.Set("X-CSRF-Token", value.csrf)
		}
		res := httptest.NewRecorder()
		handler.CreatePrinter(res, req)
		return res
	}
	if got := callCreatePrinter("production_manager", false); got.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", got.Code)
	}
	if got := callCreatePrinter("production_operator", true); got.Code != http.StatusForbidden {
		t.Fatalf("operator resource management status=%d", got.Code)
	}
	if got := callCreatePrinter("quote_viewer", true); got.Code != http.StatusForbidden {
		t.Fatalf("viewer resource management status=%d", got.Code)
	}
	created := callCreatePrinter("admin", true)
	if created.Code != http.StatusCreated {
		t.Fatalf("admin create printer status=%d body=%s", created.Code, created.Body.String())
	}
	var printerDTO struct {
		Data productionapp.PrinterView `json:"data"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &printerDTO); err != nil || printerDTO.Data.ID == "" || printerDTO.Data.Code != printerCode || printerDTO.Data.Status != "available" {
		t.Fatalf("invalid printer DTO: %+v err=%v", printerDTO, err)
	}
}
