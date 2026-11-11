package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	customerapp "example.com/loja-3d/backend/internal/application/customer"
	paymentapp "example.com/loja-3d/backend/internal/application/payment"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type PaymentHandler struct {
	payments  *paymentapp.Service
	customers *customerapp.Service
}

func NewPaymentHandler(p *paymentapp.Service, c *customerapp.Service) *PaymentHandler {
	return &PaymentHandler{p, c}
}
func paymentError(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, paymentapp.ErrNotFound):
		writeQuoteJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "Pagamento ou pedido não encontrado."}})
	case errors.Is(e, paymentapp.ErrForbidden):
		writeQuoteJSON(w, 403, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Acesso negado."}})
	case errors.Is(e, paymentapp.ErrNotReady):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "order_not_ready", "message": "O pedido ainda não está pronto para pagamento."}})
	case errors.Is(e, paymentapp.ErrConflict):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "payment_conflict", "message": "O pagamento não pode ser alterado."}})
	case errors.Is(e, paymentapp.ErrValueMismatch):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "payment_value_mismatch", "message": "O pagamento não corresponde ao pedido."}})
	case errors.Is(e, paymentapp.ErrUnavailable):
		writeQuoteJSON(w, 503, map[string]any{"error": map[string]string{"code": "payment_unavailable", "message": "O provedor de pagamentos está temporariamente indisponível."}})
	case errors.Is(e, paymentapp.ErrInvalidInput):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_payment", "message": "Os dados tokenizados do pagamento são inválidos."}})
	default:
		customerError(w, e)
	}
}

var paymentMethodPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
var identificationPattern = regexp.MustCompile(`^[0-9]{5,32}$`)

func (h *PaymentHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	user, err := h.session(r, true)
	if err != nil {
		customerError(w, err)
		return
	}
	var body struct {
		Token                string `json:"token"`
		PaymentMethodID      string `json:"payment_method_id"`
		Installments         int    `json:"installments"`
		IdentificationType   string `json:"identification_type"`
		IdentificationNumber string `json:"identification_number"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		paymentError(w, paymentapp.ErrInvalidInput)
		return
	}
	body.PaymentMethodID = strings.ToLower(strings.TrimSpace(body.PaymentMethodID))
	body.IdentificationType = strings.ToUpper(strings.TrimSpace(body.IdentificationType))
	body.IdentificationNumber = strings.TrimSpace(body.IdentificationNumber)
	if !paymentMethodPattern.MatchString(body.PaymentMethodID) || (body.IdentificationNumber != "" && !identificationPattern.MatchString(body.IdentificationNumber)) {
		paymentError(w, paymentapp.ErrInvalidInput)
		return
	}
	input := domain.CardInput{Token: strings.TrimSpace(body.Token), PaymentMethodID: body.PaymentMethodID, Installments: body.Installments, IdentificationType: body.IdentificationType, IdentificationNumber: body.IdentificationNumber, IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key"))}
	v, err := h.payments.CreateCard(r.Context(), user, r.PathValue("reference"), input)
	if err != nil {
		paymentError(w, err)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *PaymentHandler) session(r *http.Request, csrf bool) (string, error) {
	c, e := r.Cookie("loja3d_customer_session")
	if e != nil {
		return "", customerapp.ErrUnauthorized
	}
	s, e := h.customers.Authenticate(r.Context(), c.Value)
	if e != nil {
		return "", e
	}
	if csrf && !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		return "", customerapp.ErrForbidden
	}
	return s.UserID, nil
}
func (h *PaymentHandler) CreatePIX(w http.ResponseWriter, r *http.Request) {
	user, e := h.session(r, true)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.payments.CreatePIX(r.Context(), user, r.PathValue("reference"))
	if e != nil {
		paymentError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *PaymentHandler) Status(w http.ResponseWriter, r *http.Request) {
	user, e := h.session(r, false)
	if e != nil {
		customerError(w, e)
		return
	}
	v, e := h.payments.Status(r.Context(), user, r.PathValue("reference"))
	if e != nil {
		paymentError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": v})
}
func (h *PaymentHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if e != nil {
		http.Error(w, "invalid webhook", 400)
		return
	}
	var b struct {
		ID     json.RawMessage `json:"id"`
		Type   string          `json:"type"`
		Action string          `json:"action"`
		Data   struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &b) != nil {
		http.Error(w, "invalid webhook", 400)
		return
	}
	dataID := r.URL.Query().Get("data.id")
	if dataID == "" {
		dataID = r.URL.Query().Get("data_id")
	}
	if dataID == "" {
		dataID = b.Data.ID
	}
	if dataID == "" || b.Data.ID != "" && b.Data.ID != dataID {
		http.Error(w, "invalid webhook", 400)
		return
	}
	eventID := strings.Trim(strings.TrimSpace(string(b.ID)), "\"")
	if eventID == "" || eventID == "null" {
		// Orders webhooks do not always include a top-level event ID. A digest of
		// the signed payload is stable across retries, unlike X-Request-Id, and
		// therefore preserves idempotent processing.
		eventID = fmt.Sprintf("payload-%x", sha256.Sum256(raw))
	}
	eventType := b.Action
	if eventType == "" {
		eventType = b.Type
	}
	if eventID == "" || eventType == "" {
		http.Error(w, "invalid webhook", 400)
		return
	}
	e = h.payments.Webhook(r.Context(), domain.Webhook{EventID: eventID, EventType: eventType, ExternalPaymentID: dataID, RequestID: r.Header.Get("X-Request-Id"), Signature: r.Header.Get("X-Signature"), Payload: raw})
	if errors.Is(e, paymentapp.ErrInvalidWebhook) {
		http.Error(w, "invalid signature", 401)
		return
	}
	if e != nil {
		http.Error(w, "processing failed", 503)
		return
	}
	w.WriteHeader(http.StatusOK)
}
