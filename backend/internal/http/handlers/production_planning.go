package handlers

import (
	"net/http"
	"strconv"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
)

func (h *ProductionHandler) ProductionDashboard(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetProductionDashboard(r.Context(), s)
	h.audit(r, s, "production.planning.dashboard", "production_planning", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func planningTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, app.ErrInvalid
	}
	return &parsed, nil
}

func (h *ProductionHandler) ProductionPlanning(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	from, e1 := planningTime(r.URL.Query().Get("due_from"))
	to, e2 := planningTime(r.URL.Query().Get("due_to"))
	if e1 != nil || e2 != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	filter := app.PlanningFilter{Status: r.URL.Query().Get("status"), Risk: r.URL.Query().Get("risk"), PrinterID: r.URL.Query().Get("printer_id"), DueFrom: from, DueTo: to, Page: page, Limit: limit}
	value, err := h.service.ListProductionPlanning(r.Context(), s, filter)
	h.audit(r, s, "production.planning.list", "production_planning", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) ProductionCapacity(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetPrinterCapacity(r.Context(), s)
	h.audit(r, s, "production.planning.capacity", "printer", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (h *ProductionHandler) ProductionMaterials(w http.ResponseWriter, r *http.Request) {
	s, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetMaterialPlanning(r.Context(), s)
	h.audit(r, s, "production.planning.materials", "material_spool", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": value})
}
