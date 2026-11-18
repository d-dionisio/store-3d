package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProductionHandlerRequiresAdminSession(t *testing.T) {
	handler := NewProductionHandler(nil, nil)
	request := httptest.NewRequest("GET", "/api/v1/admin/production/jobs", nil)
	response := httptest.NewRecorder()
	handler.ListJobs(response, request)
	if response.Code != 401 {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestProductionPlanningRequiresAdminSession(t *testing.T) {
	handler := NewProductionHandler(nil, nil)
	request := httptest.NewRequest("GET", "/api/v1/admin/production/dashboard", nil)
	recorder := httptest.NewRecorder()
	handler.ProductionDashboard(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestProductionAnalyticsRequiresAdminSession(t *testing.T) {
	handler := NewProductionHandler(nil, nil)
	for _, path := range []string{"/api/v1/admin/production/history", "/api/v1/admin/production/analytics/productivity"} {
		request := httptest.NewRequest("GET", path, nil)
		response := httptest.NewRecorder()
		if path == "/api/v1/admin/production/history" {
			handler.ProductionHistory(response, request)
		} else {
			handler.ProductionAnalytics(response, request)
		}
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("path=%s status=%d", path, response.Code)
		}
	}
}
