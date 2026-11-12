package handlers

import (
	"encoding/json"
	"errors"
	app "example.com/loja-3d/backend/internal/application/customer"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
	domain "example.com/loja-3d/backend/internal/domain/customer"
	"log/slog"
	"net/http"
	"time"
)

type CustomerHandler struct {
	service  *app.Service
	secure   bool
	limiter  *ipLimiter
	identity *identityapp.Service
}

func NewCustomerHandler(s *app.Service, secure bool, identity ...*identityapp.Service) *CustomerHandler {
	h := &CustomerHandler{service: s, secure: secure, limiter: newIPLimiter(10, time.Minute)}
	if len(identity) > 0 {
		h.identity = identity[0]
	}
	return h
}
func customerError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, app.ErrUnauthorized):
		writeQuoteJSON(w, 401, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "Sessão necessária."}})
	case errors.Is(e, app.ErrForbidden):
		writeQuoteJSON(w, 403, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Token CSRF inválido."}})
	case errors.Is(e, app.ErrNotFound):
		writeQuoteJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "Orçamento não encontrado."}})
	case errors.Is(e, app.ErrInvalidDecision):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "invalid_decision", "message": "A proposta não pode mais ser decidida."}})
	case errors.Is(e, app.ErrInvalidConversion):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "invalid_conversion", "message": "O orçamento não pode ser convertido em pedido."}})
	case errors.Is(e, app.ErrInvalidCart):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_cart", "message": "Carrinho ou endereço inválido."}})
	case errors.Is(e, app.ErrInsufficientStock):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "insufficient_stock", "message": "Estoque insuficiente."}})
	case errors.Is(e, app.ErrShippingRequired):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "shipping_required", "message": "A cotação de frete ainda é obrigatória."}})
	default:
		writeQuoteJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "Falha interna."}})
	}
}
func (h *CustomerHandler) session(r *http.Request) (domain.Session, error) {
	c, e := r.Cookie("loja3d_customer_session")
	if e != nil {
		return domain.Session{}, app.ErrUnauthorized
	}
	return h.service.Authenticate(r.Context(), c.Value)
}
func (h *CustomerHandler) RequestLink(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(adminIP(r)) {
		w.WriteHeader(202)
		return
	}
	var b struct {
		Email string `json:"email"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b) != nil {
		w.WriteHeader(202)
		return
	}
	if e := h.service.RequestMagicLink(r.Context(), b.Email, adminIP(r)); e != nil {
		slog.Warn("magic link request was not delivered")
	}
	writeQuoteJSON(w, 202, map[string]any{"data": map[string]string{"message": "Se o endereço puder acessar a conta, enviaremos um link."}})
}
func (h *CustomerHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b) != nil {
		customerError(w, app.ErrUnauthorized)
		return
	}
	s, raw, csrf, e := h.service.VerifyMagicLink(r.Context(), b.Token, adminIP(r), r.UserAgent())
	if e != nil {
		customerError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: raw, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(time.Until(s.ExpiresAt).Seconds())})
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"csrf_token": csrf, "user": map[string]string{"email": s.Email, "name": s.FullName}}})
}
func (h *CustomerHandler) Profile(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.service.Profile(r.Context(), s)
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.service.List(r.Context(), s)
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.service.Get(r.Context(), s, r.PathValue("reference"))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *CustomerHandler) Decide(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	var b struct {
		ProposalID string `json:"proposal_id"`
		Decision   string `json:"decision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b) != nil {
		customerError(w, app.ErrInvalidDecision)
		return
	}
	if e = h.service.Decide(r.Context(), s, r.PathValue("reference"), b.ProposalID, b.Decision); e != nil {
		customerError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *CustomerHandler) Logout(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	if e = h.service.Logout(r.Context(), s); e != nil {
		customerError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: "", Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (h *CustomerHandler) Convert(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	result, e := h.service.ConvertQuote(r.Context(), s, r.PathValue("reference"))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"reference": result.Reference, "status": result.Status}})
}

type cartRequest struct {
	Items []struct {
		VariantID string `json:"variant_id"`
		Quantity  int    `json:"quantity"`
	} `json:"items"`
	AddressID string `json:"address_id"`
}

func cartInputs(v cartRequest) []domain.CartInput {
	out := make([]domain.CartInput, 0, len(v.Items))
	for _, i := range v.Items {
		out = append(out, domain.CartInput{VariantID: i.VariantID, Quantity: i.Quantity})
	}
	return out
}
func (h *CustomerHandler) ValidateCart(w http.ResponseWriter, r *http.Request) {
	var body cartRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&body) != nil {
		customerError(w, app.ErrInvalidCart)
		return
	}
	result, e := h.service.ValidateCart(r.Context(), cartInputs(body))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": result})
}
func (h *CustomerHandler) CreateCatalogOrder(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	var body cartRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&body) != nil {
		customerError(w, app.ErrInvalidCart)
		return
	}
	result, e := h.service.CreateCatalogOrder(r.Context(), s, r.Header.Get("Idempotency-Key"), body.AddressID, cartInputs(body))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 201, map[string]any{"data": map[string]string{"reference": result.Reference, "status": result.Status}})
}
func (h *CustomerHandler) ListAddresses(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	result, e := h.service.ListAddresses(r.Context(), s)
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": result})
}
func (h *CustomerHandler) CreateAddress(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	var body domain.Address
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&body) != nil {
		customerError(w, app.ErrInvalidCart)
		return
	}
	result, e := h.service.CreateAddress(r.Context(), s, body)
	if e != nil {
		customerError(w, app.ErrInvalidCart)
		return
	}
	writeQuoteJSON(w, 201, map[string]any{"data": result})
}
func (h *CustomerHandler) UpdateOrderAddress(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	var body struct {
		AddressID string `json:"address_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
		customerError(w, app.ErrInvalidCart)
		return
	}
	if e = h.service.UpdateOrderAddress(r.Context(), s, r.PathValue("reference"), body.AddressID); e != nil {
		customerError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *CustomerHandler) PrepareOrder(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !app.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, app.ErrForbidden)
		return
	}
	result, e := h.service.PrepareOrder(r.Context(), s, r.PathValue("reference"))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": result})
}
func (h *CustomerHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	result, e := h.service.ListOrders(r.Context(), s)
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": result})
}
func (h *CustomerHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	result, e := h.service.GetOrder(r.Context(), s, r.PathValue("reference"))
	if e != nil {
		customerError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": result})
}
