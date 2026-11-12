package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func upstream(t *testing.T, status int) (*url.URL, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, r.URL.Path)
	}))
	u, _ := url.Parse(server.URL)
	return u, server
}
func TestPublicGatewayBlocksAdministrativeSurface(t *testing.T) {
	next, ns := upstream(t, 200)
	defer ns.Close()
	api, as := upstream(t, 204)
	defer as.Close()
	h := Public(Config{NextUpstream: next, APIUpstream: api})
	blocked := []string{"/admin", "/admin/login?next=/", "/api/v1/admin", "/api/v1/admin/users", "/%61dmin", "/api/v1/%61dmin/users", "/api%2fv1%2fadmin", "/api/v1/customer/../admin/users", `/admin\\login`}
	for _, target := range blocked {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			req := httptest.NewRequest(method, target, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s status=%d", method, target, rec.Code)
			}
		}
	}
	for _, target := range []string{"/", "/produtos", "/checkout", "/minha-conta", "/catalogo/edicao"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != 200 {
			t.Errorf("public page %s=%d", target, rec.Code)
		}
	}
	for _, target := range []string{"/api/v1/products", "/api/v1/customer/profile", "/api/v1/quotes/ORC-1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != 204 {
			t.Errorf("public api %s=%d", target, rec.Code)
		}
	}
	for _, target := range []string{"/api/v1/webhooks/mercado-pago", "/api/v1/internal", "/api/v1/payments", "/api/v1/quotes-evil", "/api/v1/customer-evil"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, target, nil))
		if rec.Code != 404 {
			t.Errorf("private api %s=%d", target, rec.Code)
		}
	}
}
func TestAdminGatewayFailsClosedAndAllowlistsRoutes(t *testing.T) {
	next, ns := upstream(t, 200)
	defer ns.Close()
	api, as := upstream(t, 401)
	defer as.Close()
	h := Admin(Config{NextUpstream: next, APIUpstream: api, LocalAccessToken: "test-edge-secret"})
	request := func(method, target, token string) int {
		req := httptest.NewRequest(method, target, nil)
		if token != "" {
			req.Header.Set("X-Local-Zero-Trust", token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := request(http.MethodGet, "/admin", ""); got != 401 {
		t.Fatalf("missing access=%d", got)
	}
	if got := request(http.MethodGet, "/admin", "wrong"); got != 401 {
		t.Fatalf("invalid access=%d", got)
	}
	if got := request(http.MethodGet, "/admin", "test-edge-secret"); got != 200 {
		t.Fatalf("admin page=%d", got)
	}
	if got := request(http.MethodGet, "/api/v1/admin/dashboard", "test-edge-secret"); got != 401 {
		t.Fatalf("internal login must remain required=%d", got)
	}
	for _, target := range []string{"/produtos", "/api/v1/customer/profile", "/api/v1/webhooks/mercado-pago", "/api/v1/admin/../customer/profile"} {
		if got := request(http.MethodGet, target, "test-edge-secret"); got != 404 {
			t.Errorf("route %s=%d", target, got)
		}
	}
}
func TestConfiguredHostCannotBeSpoofed(t *testing.T) {
	next, ns := upstream(t, 200)
	defer ns.Close()
	api, as := upstream(t, 200)
	defer as.Close()
	h := Admin(Config{NextUpstream: next, APIUpstream: api, AdminHost: "admin.localhost", LocalAccessToken: "secret"})
	req := httptest.NewRequest(http.MethodGet, "http://evil.local/admin", nil)
	req.Header.Set("X-Local-Zero-Trust", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("spoofed host=%d", rec.Code)
	}
}
func TestParseUpstreamRequiresLoopback(t *testing.T) {
	for _, raw := range []string{"https://example.com:8080", "http://api:8080", "http://127.0.0.1", "http://user:pass@127.0.0.1:8080"} {
		if _, err := ParseUpstream(raw); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if u, err := ParseUpstream("http://127.0.0.1:8080"); err != nil || !strings.Contains(u.Host, "8080") {
		t.Fatalf("loopback rejected: %v", err)
	}
}
