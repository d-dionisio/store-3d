package handlers

import (
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

