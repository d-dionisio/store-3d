package handlers

import (
	"net/http"
	"strconv"
	"time"

	app "example.com/loja-3d/backend/internal/application/production"
)

func analyticsTime(value string, end bool) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, app.ErrInvalid
	}
	if end {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed, nil
}
func analyticsFilter(r *http.Request, paginated bool) (app.AnalyticsFilter, error) {
	from, e1 := analyticsTime(r.URL.Query().Get("from"), false)
	to, e2 := analyticsTime(r.URL.Query().Get("to"), true)
	if e1 != nil || e2 != nil {
		return app.AnalyticsFilter{}, app.ErrInvalid
	}
	page, limit := 1, 20
	if paginated {
		if value := r.URL.Query().Get("page"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return app.AnalyticsFilter{}, app.ErrInvalid
			}
			page = parsed
		}
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return app.AnalyticsFilter{}, app.ErrInvalid
			}
			limit = parsed
		}
	} else {
		limit = 100
	}
	return app.AnalyticsFilter{From: from, To: to, PrinterID: r.URL.Query().Get("printer_id"), ProductID: r.URL.Query().Get("product_id"), MaterialID: r.URL.Query().Get("material_id"), ProductQuery: r.URL.Query().Get("product"), MaterialQuery: r.URL.Query().Get("material"), Status: r.URL.Query().Get("status"), Quality: r.URL.Query().Get("quality"), Page: page, Limit: limit}, nil
}

func (h *ProductionHandler) ProductionHistory(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	filter, err := analyticsFilter(r, true)
	if err == nil {
		var value app.HistoryPage
		value, err = h.service.ListProductionHistory(r.Context(), session, filter)
		if err == nil {
			writeJSON(w, 200, map[string]any{"data": value})
		}
	}
	h.audit(r, session, "production.analytics.history", "production_run", "", err)
	if err != nil {
		productionHTTPError(w, err)
	}
}
func (h *ProductionHandler) ProductionHistoryDetail(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	value, err := h.service.GetProductionHistory(r.Context(), session, r.PathValue("runID"))
	h.audit(r, session, "production.analytics.history.detail", "production_run", r.PathValue("runID"), err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": value})
}
func (h *ProductionHandler) ProductionAnalytics(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	filter, err := analyticsFilter(r, false)
	var value app.ProductivityAnalytics
	if err == nil {
		value, err = h.service.GetProductivityAnalytics(r.Context(), session, filter)
	}
	h.audit(r, session, "production.analytics.read", "production_analytics", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": value})
}
func (h *ProductionHandler) PrinterAnalytics(w http.ResponseWriter, r *http.Request) {
	h.analyticsSlice(w, r, "printers")
}
func (h *ProductionHandler) MaterialAnalytics(w http.ResponseWriter, r *http.Request) {
	h.analyticsSlice(w, r, "materials")
}
func (h *ProductionHandler) CostAnalytics(w http.ResponseWriter, r *http.Request) {
	h.analyticsSlice(w, r, "costs")
}
func (h *ProductionHandler) TimeAccuracyAnalytics(w http.ResponseWriter, r *http.Request) {
	h.analyticsSlice(w, r, "time_accuracy")
}
func (h *ProductionHandler) analyticsSlice(w http.ResponseWriter, r *http.Request, kind string) {
	session, err := h.session(r, false)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	filter, err := analyticsFilter(r, false)
	var value app.ProductivityAnalytics
	if err == nil {
		value, err = h.service.GetProductivityAnalytics(r.Context(), session, filter)
	}
	h.audit(r, session, "production.analytics."+kind, "production_analytics", "", err)
	if err != nil {
		productionHTTPError(w, err)
		return
	}
	var data any
	switch kind {
	case "printers":
		data = value.Printers
	case "materials":
		data = value.Materials
	case "time_accuracy":
		data = value.TimeAccuracy
	default:
		data = map[string]any{"summary": value.Summary, "timeline": value.Timeline, "products": value.Products}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
