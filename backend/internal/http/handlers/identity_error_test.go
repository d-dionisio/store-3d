package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identityapp "example.com/loja-3d/backend/internal/application/identity"
)

func TestIdentityUnavailableIsNotReportedAsEmailFailure(t *testing.T) {
	admin := NewAdminHandler(nil, false, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", bytes.NewBufferString(`{"email":"admin@example.test","password":"not-logged"}`))
	rec := httptest.NewRecorder()
	admin.Login(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(rec.Body.String(), "email_unavailable") || strings.Contains(rec.Body.String(), "Envio transacional") {
		t.Fatalf("authentication outage was mislabeled as email failure: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "service_unavailable") {
		t.Fatalf("expected generic authentication service error, got %s", rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("unavailable identity service must not create a session cookie")
	}
}

func TestIdentityErrorKeepsEmailFailureSpecific(t *testing.T) {
	rec := httptest.NewRecorder()
	identityError(rec, identityapp.ErrEmailUnavailable)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "email_unavailable") {
		t.Fatalf("email-specific failure mapping changed: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
