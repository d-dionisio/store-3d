package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	customerapp "example.com/loja-3d/backend/internal/application/customer"
	shippingapp "example.com/loja-3d/backend/internal/application/shipping"
	domain "example.com/loja-3d/backend/internal/domain/shipping"
)

type ShippingHandler struct {
	shipping *shippingapp.Service
	customer *customerapp.Service
	admin    *adminapp.Service
}

func NewShippingHandler(s *shippingapp.Service, c *customerapp.Service, a *adminapp.Service) *ShippingHandler {
	return &ShippingHandler{s, c, a}
}

func shippingError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, shippingapp.ErrInvalid):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_shipping_data", "message": "CEP, peso ou dimensões inválidos."}})
	case errors.Is(e, shippingapp.ErrNotFound):
		writeQuoteJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "Pedido ou cotação não encontrado."}})
	case errors.Is(e, shippingapp.ErrExpired):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "shipping_quote_expired", "message": "A cotação expirou. Solicite uma nova."}})
	case errors.Is(e, shippingapp.ErrOrderLocked):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "order_locked", "message": "O frete deste pedido não pode mais ser alterado."}})
	case errors.Is(e, shippingapp.ErrUnavailable):
		writeQuoteJSON(w, 503, map[string]any{"error": map[string]string{"code": "shipping_unavailable", "message": "A cotação de frete está temporariamente indisponível."}})
	default:
		customerError(w, e)
	}
}

func (h *ShippingHandler) customerSession(r *http.Request) (string, string, error) {
	c, e := r.Cookie("loja3d_customer_session")
	if e != nil {
		return "", "", customerapp.ErrUnauthorized
	}
	s, e := h.customer.Authenticate(r.Context(), c.Value)
	if e != nil {
		return "", "", e
	}
	if !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		return "", "", customerapp.ErrForbidden
	}
	return s.UserID, s.ID, nil
}
func (h *ShippingHandler) Quote(w http.ResponseWriter, r *http.Request) {
	user, _, e := h.customerSession(r)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.shipping.Quote(r.Context(), user, r.PathValue("reference"))
	if e != nil {
		shippingError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *ShippingHandler) Select(w http.ResponseWriter, r *http.Request) {
	user, session, e := h.customerSession(r)
	if e != nil {
		customerError(w, e)
		return
	}
	var b struct {
		QuoteID string `json:"quote_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&b) != nil {
		shippingError(w, shippingapp.ErrInvalid)
		return
	}
	if e = h.shipping.Select(r.Context(), user, session, r.PathValue("reference"), b.QuoteID); e != nil {
		shippingError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *ShippingHandler) SavePackages(w http.ResponseWriter, r *http.Request) {
	c, e := r.Cookie("loja3d_admin_session")
	if e != nil {
		adminError(w, adminapp.ErrUnauthorized)
		return
	}
	s, e := h.admin.Authenticate(r.Context(), c.Value)
	if e == nil && !adminapp.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		e = adminapp.ErrForbidden
	}
	if e != nil {
		adminError(w, e)
		return
	}
	var b struct {
		Packages []domain.Package `json:"packages"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&b) != nil {
		shippingError(w, shippingapp.ErrInvalid)
		return
	}
	if e = h.shipping.SavePackages(r.Context(), s.UserID, r.PathValue("reference"), b.Packages, s.ID); e != nil {
		shippingError(w, e)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
