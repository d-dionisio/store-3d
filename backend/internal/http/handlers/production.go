package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	app "example.com/loja-3d/backend/internal/application/production"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

type ProductionHandler struct {
	service *app.Service
	admin   *adminapp.Service
}

func NewProductionHandler(service *app.Service, admin *adminapp.Service) *ProductionHandler {
	return &ProductionHandler{service: service, admin: admin}
}
func (h *ProductionHandler) session(r *http.Request, csrf bool) (admindomain.Session, error) {
	cookie, err := r.Cookie("loja3d_admin_session")
	if err != nil {
		return admindomain.Session{}, adminapp.ErrUnauthorized
	}
	s, err := h.admin.Authenticate(r.Context(), cookie.Value)
	if err == nil && csrf && !adminapp.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		return s, adminapp.ErrForbidden
	}
	return s, err
}
func productionHTTPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, adminapp.ErrUnauthorized):
		writeError(w, 401, "unauthorized", "Autenticação administrativa necessária.")
	case errors.Is(err, adminapp.ErrForbidden), errors.Is(err, app.ErrForbidden):
		writeError(w, 403, "forbidden", "Permissão de produção insuficiente.")
	case errors.Is(err, app.ErrInvalid):
		writeError(w, 400, "invalid_production_input", "Dados de produção inválidos.")
	case errors.Is(err, app.ErrNotFound):
		writeError(w, 404, "production_not_found", "Recurso de produção não encontrado.")
	case errors.Is(err, app.ErrConflict):
		writeError(w, 409, "production_conflict", "Conflito de estado, versão ou integridade da produção.")
	default:
		writeError(w, 500, "internal_error", "Falha interna.")
	}
}
func decodeProduction(w http.ResponseWriter, r *http.Request, target any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(target); err != nil {
		return app.ErrInvalid
	}
	return nil
}
func version(r *http.Request) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
	return v
}
func (h *ProductionHandler) audit(r *http.Request, s admindomain.Session, action, kind, id string, err error) {
	out := "success"
	if err != nil {
		out = "failure"
	}
	_ = h.admin.Audit(r.Context(), s, action, kind, id, out, adminIP(r))
}

func (h *ProductionHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
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
	v, e := h.service.ListProductionQueue(r.Context(), s, r.URL.Query().Get("status"), page, limit)
	h.audit(r, s, "production.jobs.list", "production_job", "", e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	v, e := h.service.GetProductionJob(r.Context(), s, r.PathValue("id"))
	h.audit(r, s, "production.jobs.view", "production_job", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) CreateOrderJobs(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var n int
	if e == nil {
		n, e = h.service.CreateProductionJobsForPaidOrder(r.Context(), s, r.PathValue("reference"))
	}
	h.audit(r, s, "production.jobs.create", "order", r.PathValue("reference"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": map[string]int{"created": n}})
}
func (h *ProductionHandler) jobTransition(w http.ResponseWriter, r *http.Request, action string) {
	s, e := h.session(r, true)
	var body struct {
		Description, PrinterID string
		Version                int64
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.JobView
	if e == nil {
		switch action {
		case "queue":
			v, e = h.service.QueueProductionJob(r.Context(), s, r.PathValue("id"), body.Description, body.Version)
		case "prepare":
			v, e = h.service.StartPreparation(r.Context(), s, r.PathValue("id"), body.PrinterID, body.Version)
		case "complete":
			v, e = h.service.CompleteProductionJob(r.Context(), s, r.PathValue("id"), body.Version)
		case "cancel":
			v, e = h.service.CancelProductionJob(r.Context(), s, r.PathValue("id"), body.Description, body.Version)
		}
	}
	h.audit(r, s, "production.jobs."+action, "production_job", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) QueueJob(w http.ResponseWriter, r *http.Request) {
	h.jobTransition(w, r, "queue")
}
func (h *ProductionHandler) Prepare(w http.ResponseWriter, r *http.Request) {
	h.jobTransition(w, r, "prepare")
}
func (h *ProductionHandler) CompleteJob(w http.ResponseWriter, r *http.Request) {
	h.jobTransition(w, r, "complete")
}
func (h *ProductionHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	h.jobTransition(w, r, "cancel")
}
func (h *ProductionHandler) runTransition(w http.ResponseWriter, r *http.Request, action string) {
	s, e := h.session(r, true)
	var body struct {
		Reason  string
		Version int64
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.RunView
	if e == nil {
		switch action {
		case "start":
			v, e = h.service.StartProductionRun(r.Context(), s, r.PathValue("id"), body.Version)
		case "finish":
			v, e = h.service.FinishProductionRun(r.Context(), s, r.PathValue("id"), body.Version)
		case "fail":
			v, e = h.service.FailProductionRun(r.Context(), s, r.PathValue("id"), body.Reason, body.Version)
		}
	}
	h.audit(r, s, "production.runs."+action, "production_run", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) StartRun(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, "start")
}
func (h *ProductionHandler) FinishRun(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, "finish")
}
func (h *ProductionHandler) FailRun(w http.ResponseWriter, r *http.Request) {
	h.runTransition(w, r, "fail")
}
func (h *ProductionHandler) Reprint(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct {
		PrinterID string `json:"printer_id"`
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.RunView
	if e == nil {
		v, e = h.service.CreateReprint(r.Context(), s, r.PathValue("id"), body.PrinterID)
	}
	h.audit(r, s, "production.runs.reprint", "production_run", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": v})
}

func (h *ProductionHandler) ListPrinters(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	var v []app.PrinterView
	if e == nil {
		v, e = h.service.ListPrinters(r.Context(), s)
	}
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) CreatePrinter(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var in app.PrinterInput
	if e == nil {
		e = decodeProduction(w, r, &in)
	}
	var v app.PrinterView
	if e == nil {
		v, e = h.service.CreatePrinter(r.Context(), s, in)
	}
	h.audit(r, s, "production.printers.create", "printer", v.ID, e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": v})
}
func (h *ProductionHandler) UpdatePrinter(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var in app.PrinterInput
	if e == nil {
		e = decodeProduction(w, r, &in)
	}
	var v app.PrinterView
	if e == nil {
		v, e = h.service.UpdatePrinter(r.Context(), s, r.PathValue("id"), in)
	}
	h.audit(r, s, "production.printers.update", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) PrinterStatus(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct{ Status string }
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.PrinterView
	if e == nil {
		v, e = h.service.ChangePrinterStatus(r.Context(), s, r.PathValue("id"), body.Status)
	}
	h.audit(r, s, "production.printers.status", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) Maintenance(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var in app.MaintenanceInput
	if e == nil {
		e = decodeProduction(w, r, &in)
	}
	var id string
	if e == nil {
		id, e = h.service.RegisterPrinterMaintenance(r.Context(), s, r.PathValue("id"), in)
	}
	h.audit(r, s, "production.printers.maintenance", "printer", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]string{"id": id}})
}

func (h *ProductionHandler) ListSpools(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	var v []app.SpoolView
	if e == nil {
		v, e = h.service.ListMaterialSpools(r.Context(), s, r.URL.Query().Get("status"))
	}
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) GetSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, false)
	var v app.SpoolView
	if e == nil {
		v, e = h.service.GetMaterialSpool(r.Context(), s, r.PathValue("id"))
	}
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) CreateSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var in app.SpoolInput
	if e == nil {
		e = decodeProduction(w, r, &in)
	}
	var v app.SpoolView
	if e == nil {
		v, e = h.service.RegisterMaterialSpool(r.Context(), s, in)
	}
	h.audit(r, s, "production.spools.create", "material_spool", v.ID, e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": v})
}
func (h *ProductionHandler) UpdateSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct {
		app.SpoolInput
		Version int64 `json:"version"`
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.SpoolView
	if e == nil {
		v, e = h.service.UpdateMaterialSpool(r.Context(), s, r.PathValue("id"), body.SpoolInput, body.Version)
	}
	h.audit(r, s, "production.spools.update", "material_spool", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) AdjustSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct {
		Delta   int    `json:"delta_grams"`
		Reason  string `json:"reason"`
		Version int64  `json:"version"`
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.SpoolView
	if e == nil {
		v, e = h.service.AdjustSpoolBalance(r.Context(), s, r.PathValue("id"), body.Delta, body.Reason, body.Version)
	}
	h.audit(r, s, "production.spools.adjust", "material_spool", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) QuarantineSpool(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct {
		Reason  string
		Version int64
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	var v app.SpoolView
	if e == nil {
		v, e = h.service.QuarantineMaterialSpool(r.Context(), s, r.PathValue("id"), body.Reason, body.Version)
	}
	h.audit(r, s, "production.spools.quarantine", "material_spool", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": v})
}
func (h *ProductionHandler) Consume(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var body struct {
		SpoolID  string `json:"spool_id"`
		Consumed int    `json:"consumed_grams"`
		Wasted   int    `json:"wasted_grams"`
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	if e == nil {
		e = h.service.RecordMaterialConsumption(r.Context(), s, r.PathValue("id"), body.SpoolID, body.Consumed, body.Wasted)
	}
	h.audit(r, s, "production.materials.consume", "production_run", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *ProductionHandler) quality(w http.ResponseWriter, r *http.Request, approve bool) {
	s, e := h.session(r, true)
	var body struct {
		app.InspectionInput
		Version int64 `json:"version"`
	}
	if e == nil {
		e = decodeProduction(w, r, &body)
	}
	if approve {
		var v app.RunView
		h.audit(r, s, "production.quality.approve", "production_run", r.PathValue("id"), e)
		if e == nil {
			v, e = h.service.ApproveProductionRun(r.Context(), s, r.PathValue("id"), body.InspectionInput, body.Version)
		}
		if e == nil {
			writeJSON(w, 200, map[string]any{"data": v})
			return
		}
	} else {
		var id string
		h.audit(r, s, "production.quality.reject", "production_run", r.PathValue("id"), e)
		if e == nil {
			id, e = h.service.RejectProductionRun(r.Context(), s, r.PathValue("id"), body.InspectionInput)
		}
		if e == nil {
			writeJSON(w, 201, map[string]any{"data": map[string]string{"id": id}})
			return
		}
	}
	productionHTTPError(w, e)
}
func (h *ProductionHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r, true)
	var input app.InspectionInput
	if e == nil {
		e = decodeProduction(w, r, &input)
	}
	var id string
	if e == nil {
		id, e = h.service.CreateQualityInspection(r.Context(), s, r.PathValue("id"), input)
	}
	h.audit(r, s, "production.quality.inspect", "production_run", r.PathValue("id"), e)
	if e != nil {
		productionHTTPError(w, e)
		return
	}
	writeJSON(w, 201, map[string]any{"data": map[string]string{"id": id}})
}
func (h *ProductionHandler) Approve(w http.ResponseWriter, r *http.Request) { h.quality(w, r, true) }
func (h *ProductionHandler) Reject(w http.ResponseWriter, r *http.Request)  { h.quality(w, r, false) }
