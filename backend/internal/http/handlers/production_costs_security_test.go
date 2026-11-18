package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	app "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

// These handler tests use a test session repository to exercise HTTP/RBAC
// contracts. They are not evidence of a real MFA-authenticated session.
type costSecurityAdminRepo struct {
	adminapp.Repository
	role, csrf string
	audits     []string
}

func (r *costSecurityAdminRepo) GetSession(context.Context, string) (admindomain.Session, error) {
	return admindomain.Session{ID: "session-test", UserID: "admin-test", Role: r.role, CSRFHash: r.csrf, AuthenticationLevel: "mfa", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (r *costSecurityAdminRepo) Audit(_ context.Context, _ *string, _ *string, action, _, _, outcome, _ string) error {
	r.audits = append(r.audits, action+":"+outcome)
	return nil
}

type costSecurityCostRepo struct {
	app.CostRepository
	calls map[string]int
}

func (r *costSecurityCostRepo) called(name string) { r.calls[name]++ }
func (r *costSecurityCostRepo) SaveCostProfile(_ context.Context, printer string, in app.CostProfileInput, _ string) (app.CostProfileView, error) {
	r.called("save_profile")
	return app.CostProfileView{PrinterID: printer, Version: in.ExpectedVersion + 1}, nil
}
func (r *costSecurityCostRepo) GetCurrentCostProfile(context.Context, string) (app.CostProfileView, error) {
	r.called("get_profile")
	return app.CostProfileView{Version: 1}, nil
}
func (r *costSecurityCostRepo) ListCostProfileHistory(context.Context, string) ([]app.CostProfileView, error) {
	r.called("profile_history")
	return []app.CostProfileView{{Version: 1}}, nil
}
func (r *costSecurityCostRepo) LoadRunCostContext(_ context.Context, runID, kind string, _ *int64) (app.CostContext, error) {
	r.called("load_context")
	power := int64(100000)
	profile := domain.CostProfile{PrinterID: "printer-test", Version: 1, PrinterPowerMilliwatts: &power, MachineUsefulLifeSeconds: 3600}
	return app.CostContext{Profile: profile, Input: domain.CostInput{Materials: []domain.MaterialCostInput{{ConsumedGrams: 1}}, MachineUsefulLifeSeconds: 3600, RunSeconds: 60}, RunID: runID, PrinterID: "printer-test", SnapshotType: kind, EnergySource: "configured_power", ProfileID: "profile-test", RunVersion: 1, Quantity: 1, MaterialBreakdown: []byte("[]")}, nil
}
func (r *costSecurityCostRepo) SaveRunCostSnapshot(_ context.Context, c app.CostContext, b domain.CostBreakdown, _ string) (app.CostSnapshotView, error) {
	r.called("save_snapshot")
	return app.CostSnapshotView{ID: "snapshot-test", RunID: c.RunID, CostBreakdown: b}, nil
}
func (r *costSecurityCostRepo) ListRunCostSnapshots(context.Context, string) ([]app.CostSnapshotView, error) {
	r.called("snapshots")
	return []app.CostSnapshotView{{ID: "snapshot-test"}}, nil
}
func (r *costSecurityCostRepo) CostProfitability(context.Context, string, string) ([]app.ProductCostProfitability, error) {
	r.called("profitability")
	return []app.ProductCostProfitability{}, nil
}
func (r *costSecurityCostRepo) CostProfitabilityByPrinter(context.Context, string, string) ([]app.PrinterCostProfitability, error) {
	r.called("printer_profitability")
	return []app.PrinterCostProfitability{}, nil
}
func (r *costSecurityCostRepo) CostEstimateComparison(context.Context, string, string) ([]app.CostEstimateComparison, error) {
	r.called("comparison")
	return []app.CostEstimateComparison{}, nil
}

func newCostSecurityHandler(role string) (*ProductionHandler, *costSecurityAdminRepo, *costSecurityCostRepo) {
	adminRepo := &costSecurityAdminRepo{role: role, csrf: adminapp.Hash("csrf-test")}
	costRepo := &costSecurityCostRepo{calls: map[string]int{}}
	service := app.New(nil)
	service.SetCostRepository(costRepo)
	return NewProductionHandler(service, adminapp.New(adminRepo)), adminRepo, costRepo
}

func TestCostEndpointsAnonymousAndRoleMatrix(t *testing.T) {
	type route struct {
		name, method, path string
		call               func(*ProductionHandler, http.ResponseWriter, *http.Request)
		setPaths           func(*http.Request)
	}
	routes := []route{
		{"profile", http.MethodGet, "/api/v1/admin/production/printers/p/cost-profile", (*ProductionHandler).CostProfile, func(r *http.Request) { r.SetPathValue("printerID", "p") }},
		{"profile_history", http.MethodGet, "/api/v1/admin/production/printers/p/cost-profile/history", (*ProductionHandler).CostProfileHistory, func(r *http.Request) { r.SetPathValue("printerID", "p") }},
		{"run_snapshots", http.MethodGet, "/api/v1/admin/production/runs/r/costs", (*ProductionHandler).RunCostSnapshots, func(r *http.Request) { r.SetPathValue("runID", "r") }},
		{"profitability", http.MethodGet, "/api/v1/admin/production/analytics/cost-profitability", (*ProductionHandler).CostProfitability, nil},
		{"printer_profitability", http.MethodGet, "/api/v1/admin/production/analytics/cost-profitability/printers", (*ProductionHandler).PrinterCostProfitability, nil},
		{"estimate_comparison", http.MethodGet, "/api/v1/admin/production/analytics/cost-estimate-comparison", (*ProductionHandler).CostEstimateComparison, nil},
	}
	for _, rt := range routes {
		t.Run("anonymous/"+rt.name, func(t *testing.T) {
			h, _, _ := newCostSecurityHandler("admin")
			req := httptest.NewRequest(rt.method, rt.path, nil)
			if rt.setPaths != nil {
				rt.setPaths(req)
			}
			res := httptest.NewRecorder()
			rt.call(h, res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want=401 body=%s", res.Code, res.Body.String())
			}
		})
	}
	for _, role := range []string{"quote_viewer", "production_operator", "production_manager", "admin"} {
		for _, rt := range routes {
			t.Run(role+"/"+rt.name, func(t *testing.T) {
				h, auditRepo, _ := newCostSecurityHandler(role)
				req := httptest.NewRequest(rt.method, rt.path, nil)
				req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "session-from-existing-system"})
				if rt.setPaths != nil {
					rt.setPaths(req)
				}
				res := httptest.NewRecorder()
				rt.call(h, res, req)
				want := http.StatusForbidden
				if role == "production_manager" || role == "admin" {
					want = http.StatusOK
				}
				if res.Code != want {
					t.Fatalf("role=%s status=%d want=%d body=%s", role, res.Code, want, res.Body.String())
				}
				if want == http.StatusOK && len(auditRepo.audits) != 1 {
					t.Fatalf("authorized read did not invoke audit; entries=%v", auditRepo.audits)
				}
			})
		}
	}
}

func TestCostMutationsFailClosedOnCSRFAndMissingReauthentication(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		call                     func(*ProductionHandler, http.ResponseWriter, *http.Request)
		setPaths                 func(*http.Request)
	}{
		{"profile", http.MethodPut, "/api/v1/admin/production/printers/p/cost-profile", `{"expected_version":0,"electricity_rate_cents_per_kwh":1,"machine_useful_life_seconds":3600,"power_measurement_note":"medição"}`, (*ProductionHandler).CostProfile, func(r *http.Request) { r.SetPathValue("printerID", "p") }},
		{"estimate_snapshot", http.MethodPost, "/api/v1/admin/production/runs/r/costs/estimate", `{}`, (*ProductionHandler).CreateCostSnapshot, func(r *http.Request) { r.SetPathValue("runID", "r"); r.SetPathValue("kind", "estimate") }},
		{"actual_snapshot", http.MethodPost, "/api/v1/admin/production/runs/r/costs/actual", `{"preparation_seconds":0,"finishing_seconds":0}`, (*ProductionHandler).CreateCostSnapshot, func(r *http.Request) { r.SetPathValue("runID", "r"); r.SetPathValue("kind", "actual") }},
	}
	for _, tc := range tests {
		t.Run(tc.name+"/anonymous", func(t *testing.T) {
			h, _, _ := newCostSecurityHandler("production_manager")
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			tc.setPaths(req)
			res := httptest.NewRecorder()
			tc.call(h, res, req)
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d want=401 body=%s", res.Code, res.Body.String())
			}
		})
		for _, csrf := range []struct{ name, value string }{{"missing", ""}, {"invalid", "csrf-wrong"}} {
			t.Run(tc.name+"/csrf_"+csrf.name, func(t *testing.T) {
				h, _, repo := newCostSecurityHandler("production_manager")
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "session-from-existing-system"})
				if csrf.value != "" {
					req.Header.Set("X-CSRF-Token", csrf.value)
				}
				tc.setPaths(req)
				res := httptest.NewRecorder()
				tc.call(h, res, req)
				if res.Code != http.StatusForbidden || len(repo.calls) != 0 {
					t.Fatalf("status=%d calls=%v body=%s", res.Code, repo.calls, res.Body.String())
				}
			})
		}
		t.Run(tc.name+"/reauth_missing_fails_closed", func(t *testing.T) {
			h, _, repo := newCostSecurityHandler("production_manager")
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "session-from-existing-system"})
			req.Header.Set("X-CSRF-Token", "csrf-test")
			tc.setPaths(req)
			res := httptest.NewRecorder()
			tc.call(h, res, req)
			if res.Code != http.StatusServiceUnavailable || len(repo.calls) != 0 {
				t.Fatalf("missing reauthentication must fail before mutation: status=%d calls=%v", res.Code, repo.calls)
			}
		})
	}
}

func TestCostEndpointAuditUsesSensitiveActionNames(t *testing.T) {
	h, auditRepo, _ := newCostSecurityHandler("production_manager")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/production/analytics/cost-profitability", nil)
	req.AddCookie(&http.Cookie{Name: "loja3d_admin_session", Value: "session-from-existing-system"})
	res := httptest.NewRecorder()
	h.CostProfitability(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if len(auditRepo.audits) != 1 || auditRepo.audits[0] != "production.cost_analytics.read:success" {
		t.Fatalf("unexpected audit entries: %v", auditRepo.audits)
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil || response["data"] == nil {
		t.Fatalf("invalid success response/audit test: %s err=%v", res.Body.String(), err)
	}
}
