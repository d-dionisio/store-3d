package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
)

func (h *ProductionHandler) CostProfile(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, r.Method != http.MethodGet)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	id := r.PathValue("printerID")
	var value app.CostProfileView
	if r.Method == http.MethodGet {
		value, err = h.service.GetProductionCostProfile(r.Context(), s, id)
	} else {
		if err = h.consumeCostReauthentication(r, s.ID); err != nil {
			identityError(w, err)
			return
		}
		var input app.CostProfileInput
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&input) != nil {
			productionHTTPError(w, app.ErrInvalid)
			return
		}
		value, err = h.service.SaveProductionCostProfile(r.Context(), s, id, input)
	}
	h.audit(r, s, "production.cost_profile."+map[bool]string{true: "read", false: "save"}[r.Method == http.MethodGet], "printer", id, err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) CostProfileHistory(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	id := r.PathValue("printerID")
	value, err := h.service.ProductionCostProfileHistory(r.Context(), s, id)
	h.audit(r, s, "production.cost_profile.history", "printer", id, err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) CreateCostSnapshot(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, true)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	if err = h.consumeCostReauthentication(r, s.ID); err != nil {
		identityError(w, err)
		return
	}
	kind := r.PathValue("kind")
	var input struct {
		EnergyMilliWh      *int64 `json:"energy_milliwh"`
		PreparationSeconds *int64 `json:"preparation_seconds"`
		FinishingSeconds   *int64 `json:"finishing_seconds"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	value, err := h.service.CalculateRunCost(r.Context(), s, r.PathValue("runID"), app.RunCostRequest{Kind: kind, MeasuredEnergyMilliWh: input.EnergyMilliWh, PreparationSeconds: input.PreparationSeconds, FinishingSeconds: input.FinishingSeconds})
	h.audit(r, s, "production.cost_snapshot."+kind, "production_run", r.PathValue("runID"), err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": value})
}

func (h *ProductionHandler) RunCostSnapshots(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	id := r.PathValue("runID")
	value, err := h.service.GetRunCostSnapshots(r.Context(), s, id)
	h.audit(r, s, "production.cost_snapshot.list", "production_run", id, err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) CostProfitability(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	for _, v := range []string{from, to} {
		if v != "" {
			if _, err = time.Parse("2006-01-02", v); err != nil {
				productionHTTPError(w, app.ErrInvalid)
				return
			}
		}
	}
	if from != "" && to != "" && from > to {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	value, err := h.service.GetCostProfitability(r.Context(), s, from, to)
	h.audit(r, s, "production.cost_analytics.read", "production_cost_analytics", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) PrinterCostProfitability(w http.ResponseWriter, r *http.Request) {
	h.costMetric(w, r, "printer")
}

func (h *ProductionHandler) CostEstimateComparison(w http.ResponseWriter, r *http.Request) {
	h.costMetric(w, r, "comparison")
}

func (h *ProductionHandler) costMetric(w http.ResponseWriter, r *http.Request, kind string) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	for _, v := range []string{from, to} {
		if v != "" {
			if _, err = time.Parse("2006-01-02", v); err != nil {
				productionHTTPError(w, app.ErrInvalid)
				return
			}
		}
	}
	if from != "" && to != "" && from > to {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	var value any
	switch kind {
	case "printer":
		value, err = h.service.GetPrinterCostProfitability(r.Context(), s, from, to)
	default:
		value, err = h.service.GetCostEstimateComparison(r.Context(), s, from, to)
	}
	h.audit(r, s, "production.cost_analytics."+kind, "production_cost_analytics", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}
