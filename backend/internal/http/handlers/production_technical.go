package handlers

import (
	"encoding/json"
	app "example.com/loja-3d/backend/internal/application/production"
	"net/http"
	"strconv"
)

func (h *ProductionHandler) ListPrinterTechnicalSpecs(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.ListPrinterSpecs(r.Context(), s)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}

func (h *ProductionHandler) PrinterTechnicalSpec(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.GetPrinterSpec(r.Context(), s, r.PathValue("id"))
	h.audit(r, s, "production.printer_spec.read", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) PrinterTechnicalHistory(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.PrinterSpecHistory(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) SavePrinterTechnicalSpec(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.PrinterSpecInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	v, e := h.service.SavePrinterSpec(r.Context(), s, r.PathValue("id"), in)
	h.audit(r, s, "production.printer_spec.save", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) SavePrinterOperationalConfig(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.OperationalConfigInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	e = h.service.SaveOperationalConfig(r.Context(), s, r.PathValue("id"), in)
	h.audit(r, s, "production.printer_config.save", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *ProductionHandler) ListMaterialTechnicalSpecs(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.ListMaterialSpecs(r.Context(), s)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) MaterialTechnicalSpec(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.GetMaterialSpec(r.Context(), s, r.PathValue("id"))
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) SaveMaterialTechnicalSpec(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.MaterialSpecInput
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	v, e := h.service.SaveMaterialSpec(r.Context(), s, r.PathValue("id"), in)
	h.audit(r, s, "production.material_spec.save", "material", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) InstallSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var body struct {
		PrinterID string `json:"printer_id"`
		SpoolID   string `json:"spool_id"`
		Slot      int    `json:"slot_number"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	v, e := h.service.InstallSpool(r.Context(), s, body.PrinterID, body.SpoolID, body.Slot)
	h.audit(r, s, "production.spool.install", "spool", body.SpoolID, e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": v})
}
func (h *ProductionHandler) RemoveInstalledSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	e = h.service.RemoveSpool(r.Context(), s, r.PathValue("id"), body.Reason)
	h.audit(r, s, "production.spool.remove", "spool_installation", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *ProductionHandler) SpoolInstallations(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	history, _ := strconv.ParseBool(r.URL.Query().Get("history"))
	v, e := h.service.ListSpoolInstallations(r.Context(), s, r.URL.Query().Get("printer_id"), history)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) TechnicalCompatibility(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	var in app.TechnicalCompatibilityRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		productionHTTPError(w, app.ErrInvalid)
		return
	}
	v, e := h.service.CheckTechnicalCompatibility(r.Context(), s, in)
	h.audit(r, s, "production.compatibility.simulate", "printer", in.PrinterID, e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
