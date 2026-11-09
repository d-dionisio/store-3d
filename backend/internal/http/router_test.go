package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apihttp "example.com/loja-3d/backend/internal/http"
)

type readinessStub struct{ err error }

func (stub readinessStub) Check(context.Context) error { return stub.err }

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()

	apihttp.NewHandler("http://localhost:3000", readinessStub{err: errors.New("database unavailable")}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", contentType)
	}

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "http://localhost:3000" {
		t.Errorf("expected configured CORS origin, got %q", origin)
	}

	var body struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" || body.Service != "loja-3d-api" {
		t.Errorf("unexpected response body: %+v", body)
	}
}

func TestHealthEndpointRejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	response := httptest.NewRecorder()

	apihttp.NewHandler("http://localhost:3000", readinessStub{}, nil).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
}

func TestReadyEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		checker    readinessStub
		wantStatus int
		wantBody   string
	}{
		{name: "database connected", checker: readinessStub{}, wantStatus: http.StatusOK, wantBody: `{"status":"ready","database":"connected"}`},
		{name: "database unavailable", checker: readinessStub{err: errors.New("connection failed")}, wantStatus: http.StatusServiceUnavailable, wantBody: `{"status":"unavailable","database":"disconnected"}`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil)
			response := httptest.NewRecorder()
			apihttp.NewHandler("http://localhost:3000", test.checker, nil).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, response.Code)
			}
			if body := string(response.Body.Bytes()); body != test.wantBody+"\n" {
				t.Fatalf("unexpected body: %s", body)
			}
		})
	}
}
