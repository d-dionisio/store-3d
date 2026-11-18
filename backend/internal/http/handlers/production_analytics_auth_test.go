package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	productionapp "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	"time"
)

// This HTTP-level test verifies the analytics RBAC after an authenticated
// session has been supplied. Session authentication/MFA remains owned by the
// existing admin service and is not simulated as an MFA validation test.
type analyticsSessionRepositoryStub struct {
	adminapp.Repository
	role string
}

func (s analyticsSessionRepositoryStub) GetSession(context.Context, string) (admindomain.Session, error) {
	return admindomain.Session{ID: "session-test", UserID: "user-test", Role: s.role, ExpiresAt: time.Now().Add(time.Hour), AuthenticationLevel: "mfa"}, nil
}

func (analyticsSessionRepositoryStub) Audit(context.Context, *string, *string, string, string, string, string, string) error {
	return nil
}

type analyticsHTTPRepositoryStub struct{}

func (analyticsHTTPRepositoryStub) ListHistory(_ context.Context, f productionapp.AnalyticsFilter) (productionapp.HistoryPage, error) {
	return productionapp.HistoryPage{Items: []productionapp.HistoryItem{}, Page: f.Page, Limit: f.Limit}, nil
}
func (analyticsHTTPRepositoryStub) GetHistory(context.Context, string) (productionapp.HistoryDetail, error) {
	return productionapp.HistoryDetail{}, productionapp.ErrNotFound
}
func (analyticsHTTPRepositoryStub) Productivity(context.Context, productionapp.AnalyticsFilter) (productionapp.ProductivityAnalytics, error) {
	return productionapp.ProductivityAnalytics{}, nil
}

func TestProductionAnalyticsHTTPRoleMatrix(t *testing.T) {
	for _, role := range []string{"quote_viewer", "production_operator", "production_manager", "admin"} {
		t.Run(role, func(t *testing.T) {
			admin := adminapp.New(analyticsSessionRepositoryStub{role: role})
			service := productionapp.New(nil)
			service.SetAnalyticsRepository(analyticsHTTPRepositoryStub{})
			handler := NewProductionHandler(service, admin)

			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/history?page=1&limit=20", nil)
			if role != "" {
				request.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "existing-session-token"})
			}
			response := httptest.NewRecorder()
			handler.ProductionHistory(response, request)
			want := http.StatusOK
			if role == "quote_viewer" {
				want = http.StatusForbidden
			}
			if response.Code != want {
				t.Fatalf("role=%s status=%d want=%d body=%s", role, response.Code, want, response.Body.String())
			}
		})
	}
}

func TestProductionAnalyticsHTTPAnonymousAndInvalidFilter(t *testing.T) {
	admin := adminapp.New(analyticsSessionRepositoryStub{role: "production_operator"})
	service := productionapp.New(nil)
	service.SetAnalyticsRepository(analyticsHTTPRepositoryStub{})
	handler := NewProductionHandler(service, admin)

	anonymous := httptest.NewRecorder()
	handler.ProductionHistory(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/history", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d want=%d", anonymous.Code, http.StatusUnauthorized)
	}

	invalid := httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/history?page=0&limit=20", nil)
	invalid.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "existing-session-token"})
	response := httptest.NewRecorder()
	handler.ProductionHistory(response, invalid)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid filter status=%d want=%d", response.Code, http.StatusBadRequest)
	}
}
