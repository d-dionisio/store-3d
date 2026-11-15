package handlers

import (
	"encoding/json"
	"net/http"

	app "example.com/loja-3d/backend/internal/application/production"
	domain "example.com/loja-3d/backend/internal/domain/production"
)

func (h *ProductionHandler) ProductionOptimization(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetOptimization(r.Context(), s)
	h.audit(r, s, "production.optimization.read", "production_optimization", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) ProductionRecommendations(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetOptimization(r.Context(), s)
	h.audit(r, s, "production.optimization.recommendations", "production_optimization", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value.Recommendations})
}

func (h *ProductionHandler) SimulateProductionOptimization(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, true)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	var body struct {
		Strategy domain.OptimizationStrategy `json:"strategy"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	value, err := h.service.SimulateOptimization(r.Context(), s, body.Strategy)
	h.audit(r, s, "production.optimization.simulate", "production_optimization", string(body.Strategy), err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}
