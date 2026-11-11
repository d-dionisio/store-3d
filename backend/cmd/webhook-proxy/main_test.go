package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProxyOnlyForwardsTheWebhookRoute(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("data.id") != "ORDER1" || r.Header.Get("X-Signature") != "ts=1,v1=abc" || r.Header.Get("X-Request-Id") != "request-1" {
			t.Errorf("required Mercado Pago metadata was not preserved")
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Errorf("browser credentials reached upstream")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"type":"order"}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	handler := newHandler(u, 10)

	req := httptest.NewRequest(http.MethodPost, webhookPath+"?data.id=ORDER1", strings.NewReader(`{"type":"order"}`))
	req.Header.Set("X-Signature", "ts=1,v1=abc")
	req.Header.Set("X-Request-Id", "request-1")
	req.Header.Set("Authorization", "Bearer must-not-forward")
	req.Header.Set("Cookie", "session=must-not-forward")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK || calls != 1 {
		t.Fatalf("status=%d calls=%d", response.Code, calls)
	}

	for _, path := range []string{"/api/v1/health", "/api/v1/admin/quotes", "/api/v1/customer/profile", webhookPath + "/extra"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("path %q: got %d, want 404", path, response.Code)
		}
	}
	if calls != 1 {
		t.Fatalf("blocked routes reached upstream: calls=%d", calls)
	}
}

func TestProxyEnforcesMethodSizeAndRateLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)

	handler := newHandler(u, 1)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, webhookPath, nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d", response.Code)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader("ok")))
	if response.Code != http.StatusOK {
		t.Fatalf("first POST status=%d", response.Code)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader("again")))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited POST status=%d", response.Code)
	}

	handler = newHandler(u, 10)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader(strings.Repeat("x", maxWebhookBody+1))))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized POST status=%d", response.Code)
	}
}

func TestUpstreamMustBeLoopback(t *testing.T) {
	for _, raw := range []string{"https://127.0.0.1:8080", "http://example.com", "http://127.0.0.1:8080/path", "http://user:pass@127.0.0.1:8080"} {
		if _, err := parseLoopbackUpstream(raw); err == nil {
			t.Errorf("accepted unsafe upstream %q", raw)
		}
	}
	if _, err := parseLoopbackUpstream("http://127.0.0.1:8080"); err != nil {
		t.Fatalf("rejected loopback upstream: %v", err)
	}
}
