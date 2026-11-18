package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProductionCostEndpointsRequireExistingAdminSession(t *testing.T) {
	h, _, _ := newCostSecurityHandler("admin")
	cases := []struct {
		name    string
		method  string
		path    string
		handler func(*ProductionHandler, http.ResponseWriter, *http.Request)
	}{
		{"profile", "GET", "/api/v1/admin/production/printers/printer/cost-profile", (*ProductionHandler).CostProfile},
		{"snapshot", "POST", "/api/v1/admin/production/runs/run/cost-actual", (*ProductionHandler).CreateCostSnapshot},
		{"profitability", "GET", "/api/v1/admin/production/analytics/cost-profitability", (*ProductionHandler).CostProfitability},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			response := httptest.NewRecorder()
			tc.handler(h, response, request)
			if response.Code != 401 {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
