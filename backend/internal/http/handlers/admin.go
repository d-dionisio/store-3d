package handlers

import (
	"encoding/json"
	"errors"
	app "example.com/loja-3d/backend/internal/application/admin"
	domain "example.com/loja-3d/backend/internal/domain/admin"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AdminHandler struct {
	service       *app.Service
	secureCookies bool
	limiter       *ipLimiter
}

func NewAdminHandler(service *app.Service, secure bool) *AdminHandler {
	return &AdminHandler{service: service, secureCookies: secure, limiter: newIPLimiter(10, time.Minute)}
}
func adminIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func adminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrUnauthorized):
		writeQuoteJSON(w, 401, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "Autenticação administrativa necessária."}})
	case errors.Is(err, app.ErrForbidden):
		writeQuoteJSON(w, 403, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Acesso negado."}})
	case errors.Is(err, app.ErrInvalid):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "Dados inválidos."}})
	case errors.Is(err, app.ErrConflict):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "conflict", "message": "O e-mail informado já pertence a outro usuário."}})
	case errors.Is(err, domain.ErrInvalidPricing):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_pricing", "message": "Composição de preço inválida."}})
	case errors.Is(err, domain.ErrInvalidTransition):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "invalid_transition", "message": "A operação não é permitida no estado atual ou há arquivos não aprovados."}})
	case errors.Is(err, app.ErrNotFound):
		writeQuoteJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "Orçamento não encontrado."}})
	default:
		writeQuoteJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "Falha interna."}})
	}
}
func (h *AdminHandler) session(r *http.Request) (domain.Session, error) {
	cookie, err := r.Cookie("loja3d_admin_session")
	if err != nil {
		return domain.Session{}, app.ErrUnauthorized
	}
	return h.service.Authenticate(r.Context(), cookie.Value)
}
func (h *AdminHandler) mutationSession(r *http.Request) (domain.Session, error) {
	session, err := h.session(r)
	if err == nil && !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), session.CSRFHash) {
		err = app.ErrForbidden
	}
	return session, err
}
func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(adminIP(r)) {
		writeQuoteJSON(w, 429, map[string]any{"error": map[string]string{"code": "rate_limited", "message": "Muitas tentativas."}})
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
		adminError(w, app.ErrInvalid)
		return
	}
	session, token, csrf, err := h.service.Login(r.Context(), body.Email, body.Password, adminIP(r), r.UserAgent())
	if err != nil {
		adminError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_admin_session", Value: token, Path: "/", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int(time.Until(session.ExpiresAt).Seconds())})
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"csrf_token": csrf, "user": map[string]string{"email": session.Email, "name": session.FullName, "role": session.Role}}})
}
func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		adminError(w, err)
		return
	}
	if !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), session.CSRFHash) {
		adminError(w, app.ErrForbidden)
		return
	}
	if err = h.service.Logout(r.Context(), session, adminIP(r)); err != nil {
		adminError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_admin_session", Value: "", Path: "/", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) ListInternalUsers(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		adminError(w, err)
		return
	}
	users, err := h.service.ListInternalUsers(r.Context(), session)
	if err != nil {
		adminError(w, err)
		return
	}
	writeQuoteJSON(w, http.StatusOK, map[string]any{"data": users})
}
func (h *AdminHandler) CreateInternalUser(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutationSession(r)
	if err != nil {
		adminError(w, err)
		return
	}
	var body struct {
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body) != nil {
		adminError(w, app.ErrInvalid)
		return
	}
	user, err := h.service.CreateInternalUser(r.Context(), session, body.Email, body.FullName, body.Password, body.Role, adminIP(r))
	if err != nil {
		adminError(w, err)
		return
	}
	writeQuoteJSON(w, http.StatusCreated, map[string]any{"data": user})
}
func (h *AdminHandler) UpdateInternalUser(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutationSession(r)
	if err != nil {
		adminError(w, err)
		return
	}
	var body struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
		adminError(w, app.ErrInvalid)
		return
	}
	user, err := h.service.UpdateInternalUser(r.Context(), session, r.PathValue("id"), body.Role, body.Status, adminIP(r))
	if err != nil {
		adminError(w, err)
		return
	}
	writeQuoteJSON(w, http.StatusOK, map[string]any{"data": user})
}
func (h *AdminHandler) ResetInternalUserPassword(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutationSession(r)
	if err != nil {
		adminError(w, err)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
		adminError(w, app.ErrInvalid)
		return
	}
	if err = h.service.ResetInternalUserPassword(r.Context(), session, r.PathValue("id"), body.Password, adminIP(r)); err != nil {
		adminError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) RevokeInternalUserSessions(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutationSession(r)
	if err != nil {
		adminError(w, err)
		return
	}
	if err = h.service.RevokeInternalUserSessions(r.Context(), session, r.PathValue("id"), adminIP(r)); err != nil {
		adminError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		adminError(w, err)
		return
	}
	value, err := h.service.Stats(r.Context(), session, adminIP(r))
	if err != nil {
		adminError(w, err)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]int64{"total": value.Total, "waiting": value.Waiting, "reviewing": value.Reviewing, "quoted": value.Quoted, "approved": value.Approved, "rejected": value.Rejected}})
}
func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		adminError(w, err)
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
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "newest"
	}
	value, err := h.service.List(r.Context(), session, strings.TrimSpace(r.URL.Query().Get("search")), r.URL.Query().Get("status"), sort, page, limit, adminIP(r))
	if err != nil {
		adminError(w, err)
		return
	}
	items := make([]adminQuoteDTO, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, mapAdminSummary(item))
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"items": items, "meta": map[string]any{"page": value.Page, "limit": value.Limit, "total": value.Total, "total_pages": int((value.Total + int64(value.Limit) - 1) / int64(value.Limit))}}})
}
func (h *AdminHandler) Get(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		adminError(w, err)
		return
	}
	value, err := h.service.Get(r.Context(), session, r.PathValue("reference"), adminIP(r))
	if err != nil {
		adminError(w, err)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": mapAdminDetail(value)})
}
func (h *AdminHandler) StartAnalysis(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e == nil && !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		e = app.ErrForbidden
	}
	if e == nil {
		e = h.service.StartAnalysis(r.Context(), s, r.PathValue("reference"), adminIP(r))
	}
	if e != nil {
		adminError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) SavePricing(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		adminError(w, e)
		return
	}
	if !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		adminError(w, app.ErrForbidden)
		return
	}
	var body struct {
		MaterialCostCents       int64      `json:"material_cost_cents"`
		EnergyCostCents         int64      `json:"energy_cost_cents"`
		MachineCostCents        int64      `json:"machine_cost_cents"`
		FinishingCostCents      int64      `json:"finishing_cost_cents"`
		PackagingCostCents      int64      `json:"packaging_cost_cents"`
		OtherCostsCents         int64      `json:"other_costs_cents"`
		MarginBasisPoints       int        `json:"margin_basis_points"`
		FinalPriceCents         int64      `json:"final_price_cents"`
		ProductionLeadTimeDays  int        `json:"production_lead_time_days"`
		EstimatedCompletionDate *time.Time `json:"estimated_completion_date"`
		ValidUntil              time.Time  `json:"valid_until"`
		CommercialNotes         string     `json:"commercial_notes"`
		ShippingIncluded        bool       `json:"shipping_included"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body) != nil {
		adminError(w, app.ErrInvalid)
		return
	}
	p := domain.Pricing{MaterialCostCents: body.MaterialCostCents, EnergyCostCents: body.EnergyCostCents, MachineCostCents: body.MachineCostCents, FinishingCostCents: body.FinishingCostCents, PackagingCostCents: body.PackagingCostCents, OtherCostsCents: body.OtherCostsCents, MarginBasisPoints: body.MarginBasisPoints, FinalPriceCents: body.FinalPriceCents, ProductionLeadTimeDays: body.ProductionLeadTimeDays, EstimatedCompletionDate: body.EstimatedCompletionDate, ValidUntil: body.ValidUntil, CommercialNotes: body.CommercialNotes, ShippingIncluded: body.ShippingIncluded}
	if e = h.service.SavePricing(r.Context(), s, r.PathValue("reference"), p); e != nil {
		adminError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) SendProposal(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e == nil && !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		e = app.ErrForbidden
	}
	if e == nil {
		e = h.service.SendProposal(r.Context(), s, r.PathValue("reference"))
	}
	if e != nil {
		adminError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *AdminHandler) Expire(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e == nil && !app.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		e = app.ErrForbidden
	}
	var count int64
	if e == nil {
		count, e = h.service.Expire(r.Context(), s)
	}
	if e != nil {
		adminError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]int64{"expired": count}})
}
