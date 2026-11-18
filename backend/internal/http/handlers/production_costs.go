package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
	app "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

var costUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func costAuditIP(r *http.Request) string {
	if ip := net.ParseIP(adminIP(r)); ip != nil {
		return ip.String()
	}
	return ""
}

func safeCostResourceID(id string) string {
	if costUUIDPattern.MatchString(id) {
		return id
	}
	return ""
}

func (h *ProductionHandler) recordCostAudit(r *http.Request, session admindomain.Session, action, resourceType, resourceID, outcome string) error {
	if h.admin == nil {
		return errors.New("audit unavailable")
	}
	return h.admin.Audit(r.Context(), session, action, resourceType, safeCostResourceID(resourceID), outcome, costAuditIP(r))
}

func (h *ProductionHandler) recordCostSecurity(r *http.Request, session *admindomain.Session, event string) error {
	if h.admin == nil {
		return errors.New("audit unavailable")
	}
	return h.admin.AuditProductionCostSecurity(r.Context(), session, "production.cost.security."+event, "denied", costAuditIP(r))
}

func costAuditUnavailable(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "A operação não pôde ser confirmada com segurança.")
}

func (h *ProductionHandler) costSession(w http.ResponseWriter, r *http.Request, csrf bool) (admindomain.Session, bool) {
	session, err := h.session(r, csrf)
	if err == nil {
		return session, true
	}
	event := "session_denied"
	var actor *admindomain.Session
	if session.ID != "" {
		actor = &session
		if errors.Is(err, adminapp.ErrForbidden) {
			event = "csrf_invalid"
			if strings.TrimSpace(r.Header.Get("X-CSRF-Token")) == "" {
				event = "csrf_missing"
			}
		}
	}
	if auditErr := h.recordCostSecurity(r, actor, event); auditErr != nil {
		costAuditUnavailable(w)
		return admindomain.Session{}, false
	}
	productionHTTPError(w, err)
	return admindomain.Session{}, false
}

func (h *ProductionHandler) costReauthentication(w http.ResponseWriter, r *http.Request, session admindomain.Session) bool {
	err := h.consumeCostReauthentication(r, session.ID)
	if err == nil {
		return true
	}
	event := "reauth_rejected"
	if strings.TrimSpace(r.Header.Get("X-Reauthentication-Token")) == "" {
		event = "reauth_missing"
	} else if errors.Is(err, identityapp.ErrUnavailable) {
		event = "reauth_unavailable"
	}
	if auditErr := h.recordCostSecurity(r, &session, event); auditErr != nil {
		costAuditUnavailable(w)
		return false
	}
	identityError(w, err)
	return false
}

func costAuditOutcome(err error) string {
	if errors.Is(err, app.ErrForbidden) || errors.Is(err, adminapp.ErrForbidden) {
		return "denied"
	}
	return "failure"
}

func (h *ProductionHandler) costOperationFailed(w http.ResponseWriter, r *http.Request, session admindomain.Session, action, resourceType, resourceID string, err error) {
	if errors.Is(err, app.ErrCostAuditUnavailable) {
		costAuditUnavailable(w)
		return
	}
	if auditErr := h.recordCostAudit(r, session, action, resourceType, resourceID, costAuditOutcome(err)); auditErr != nil {
		costAuditUnavailable(w)
		return
	}
	productionHTTPError(w, err)
}

func (h *ProductionHandler) costReadFinished(w http.ResponseWriter, r *http.Request, session admindomain.Session, action, resourceType, resourceID string, value any, err error) {
	outcome := "success"
	if err != nil {
		outcome = costAuditOutcome(err)
	}
	if auditErr := h.recordCostAudit(r, session, action, resourceType, resourceID, outcome); auditErr != nil {
		costAuditUnavailable(w)
		return
	}
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) CostProfile(w http.ResponseWriter, r *http.Request) {
	mutating := r.Method != http.MethodGet
	s, ok := h.costSession(w, r, mutating)
	if !ok {
		return
	}
	id := r.PathValue("printerID")
	action := "production.cost_profile.read"
	if mutating {
		action = "production.cost_profile.save"
		if err := h.service.AuthorizeProductionCostManagement(s); err != nil {
			h.costOperationFailed(w, r, s, action, "printer", id, err)
			return
		}
		if !h.costReauthentication(w, r, s) {
			return
		}
		var input app.CostProfileInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil {
			h.costOperationFailed(w, r, s, action, "printer", id, app.ErrInvalid)
			return
		}
		value, err := h.service.SaveProductionCostProfile(r.Context(), s, id, input, app.CostAuditContext{SessionID: s.ID, IPAddress: costAuditIP(r)})
		if err != nil {
			h.costOperationFailed(w, r, s, action, "printer", id, err)
			return
		}
		// The repository commits the profile and its success audit atomically.
		writeJSON(w, http.StatusOK, map[string]any{"data": value})
		return
	}
	value, err := h.service.GetProductionCostProfile(r.Context(), s, id)
	h.costReadFinished(w, r, s, action, "printer", id, value, err)
}

func (h *ProductionHandler) CostProfileHistory(w http.ResponseWriter, r *http.Request) {
	s, ok := h.costSession(w, r, false)
	if !ok {
		return
	}
	id := r.PathValue("printerID")
	value, err := h.service.ProductionCostProfileHistory(r.Context(), s, id)
	h.costReadFinished(w, r, s, "production.cost_profile.history", "printer", id, value, err)
}

func (h *ProductionHandler) CreateCostSnapshot(w http.ResponseWriter, r *http.Request) {
	s, ok := h.costSession(w, r, true)
	if !ok {
		return
	}
	if err := h.service.AuthorizeProductionCostManagement(s); err != nil {
		actionKind := r.PathValue("kind")
		if actionKind != "estimate" && actionKind != "actual" {
			actionKind = "invalid"
		}
		h.costOperationFailed(w, r, s, "production.cost_snapshot."+actionKind, "production_run", r.PathValue("runID"), err)
		return
	}
	if !h.costReauthentication(w, r, s) {
		return
	}
	kind := r.PathValue("kind")
	actionKind := kind
	if actionKind != "estimate" && actionKind != "actual" {
		actionKind = "invalid"
	}
	action := "production.cost_snapshot." + actionKind
	var input struct {
		EnergyMilliWh      *int64 `json:"energy_milliwh"`
		PreparationSeconds *int64 `json:"preparation_seconds"`
		FinishingSeconds   *int64 `json:"finishing_seconds"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		h.costOperationFailed(w, r, s, action, "production_run", r.PathValue("runID"), app.ErrInvalid)
		return
	}
	value, err := h.service.CalculateRunCost(r.Context(), s, r.PathValue("runID"), app.RunCostRequest{Kind: kind, MeasuredEnergyMilliWh: input.EnergyMilliWh, PreparationSeconds: input.PreparationSeconds, FinishingSeconds: input.FinishingSeconds}, app.CostAuditContext{SessionID: s.ID, IPAddress: costAuditIP(r)})
	if err != nil {
		h.costOperationFailed(w, r, s, action, "production_run", r.PathValue("runID"), err)
		return
	}
	// Snapshot and its success audit are committed by one repository transaction.
	writeJSON(w, http.StatusCreated, map[string]any{"data": value})
}

func (h *ProductionHandler) RunCostSnapshots(w http.ResponseWriter, r *http.Request) {
	s, ok := h.costSession(w, r, false)
	if !ok {
		return
	}
	id := r.PathValue("runID")
	value, err := h.service.GetRunCostSnapshots(r.Context(), s, id)
	h.costReadFinished(w, r, s, "production.cost_snapshot.list", "production_run", id, value, err)
}

func (h *ProductionHandler) parseCostPeriod(w http.ResponseWriter, r *http.Request, s admindomain.Session, action string) (string, string, bool) {
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	for _, value := range []string{from, to} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				h.costOperationFailed(w, r, s, action, "production_cost_analytics", "", app.ErrInvalid)
				return "", "", false
			}
		}
	}
	if from != "" && to != "" && from > to {
		h.costOperationFailed(w, r, s, action, "production_cost_analytics", "", app.ErrInvalid)
		return "", "", false
	}
	return from, to, true
}

func (h *ProductionHandler) CostProfitability(w http.ResponseWriter, r *http.Request) {
	s, ok := h.costSession(w, r, false)
	if !ok {
		return
	}
	from, to, valid := h.parseCostPeriod(w, r, s, "production.cost_analytics.read")
	if !valid {
		return
	}
	value, err := h.service.GetCostProfitability(r.Context(), s, from, to)
	h.costReadFinished(w, r, s, "production.cost_analytics.read", "production_cost_analytics", "", value, err)
}

func (h *ProductionHandler) PrinterCostProfitability(w http.ResponseWriter, r *http.Request) {
	h.costMetric(w, r, "printer")
}

func (h *ProductionHandler) CostEstimateComparison(w http.ResponseWriter, r *http.Request) {
	h.costMetric(w, r, "comparison")
}

func (h *ProductionHandler) costMetric(w http.ResponseWriter, r *http.Request, kind string) {
	s, ok := h.costSession(w, r, false)
	if !ok {
		return
	}
	action := "production.cost_analytics." + kind
	from, to, valid := h.parseCostPeriod(w, r, s, action)
	if !valid {
		return
	}
	var value any
	var err error
	switch kind {
	case "printer":
		value, err = h.service.GetPrinterCostProfitability(r.Context(), s, from, to)
	default:
		value, err = h.service.GetCostEstimateComparison(r.Context(), s, from, to)
	}
	h.costReadFinished(w, r, s, action, "production_cost_analytics", "", value, err)
}
