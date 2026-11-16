package handlers

import (
	"encoding/json"
	app "example.com/loja-3d/backend/internal/application/production"
	"net/http"
)

func (h *ProductionHandler) Monitoring(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.GetMonitoring(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) MonitoringConfig(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.GetMonitoringConfig(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) SaveMonitoringConfig(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.PrinterMonitoringConfigInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	v, e := h.service.SaveMonitoringConfig(r.Context(), s, r.PathValue("id"), in)
	h.audit(r, s, "production.printer_monitoring.save", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) TestMonitoringConnection(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.PrinterMonitoringConfigInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	e = h.service.TestMonitoringConnection(r.Context(), s, in)
	h.audit(r, s, "production.printer_monitoring.test", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"status": "connected"}})
}
func (h *ProductionHandler) ConnectionEvents(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.ListConnectionEvents(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) ObservedJobs(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.ListObservedJobs(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
