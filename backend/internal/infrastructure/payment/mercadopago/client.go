package mercadopago

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL, token, webhookSecret string
	http                          *http.Client
}

func New(baseURL, token, secret string, h *http.Client) (*Client, error) {
	u, e := url.Parse(strings.TrimRight(baseURL, "/"))
	if e != nil || u.Scheme != "https" || u.Host == "" || token == "" || secret == "" || h == nil {
		return nil, errors.New("invalid Mercado Pago configuration")
	}
	return &Client{strings.TrimRight(baseURL, "/"), token, secret, h}, nil
}

type Unavailable struct{}

func (Unavailable) CreatePIX(context.Context, domain.CreateRequest) (domain.ProviderPayment, error) {
	return domain.ProviderPayment{}, errors.New("Mercado Pago unavailable")
}
func (Unavailable) GetPayment(context.Context, string) (domain.ProviderPayment, error) {
	return domain.ProviderPayment{}, errors.New("Mercado Pago unavailable")
}
func (Unavailable) ValidateWebhook(string, string, string) bool { return false }

type createBody struct {
	TransactionAmount float64 `json:"transaction_amount"`
	Description       string  `json:"description"`
	PaymentMethodID   string  `json:"payment_method_id"`
	ExternalReference string  `json:"external_reference"`
	DateOfExpiration  string  `json:"date_of_expiration"`
	Payer             struct {
		Email string `json:"email"`
	} `json:"payer"`
}
type paymentResponse struct {
	ID                        any     `json:"id"`
	Status                    string  `json:"status"`
	StatusDetail              string  `json:"status_detail"`
	Currency                  string  `json:"currency_id"`
	ExternalReference         string  `json:"external_reference"`
	TransactionAmount         float64 `json:"transaction_amount"`
	TransactionAmountRefunded float64 `json:"transaction_amount_refunded"`
	DateOfExpiration          string  `json:"date_of_expiration"`
	DateApproved              string  `json:"date_approved"`
	DateLastUpdated           string  `json:"date_last_updated"`
	PointOfInteraction        struct {
		TransactionData struct {
			QRCode       string `json:"qr_code"`
			QRCodeBase64 string `json:"qr_code_base64"`
			TicketURL    string `json:"ticket_url"`
		} `json:"transaction_data"`
	} `json:"point_of_interaction"`
}

func (c *Client) CreatePIX(ctx context.Context, in domain.CreateRequest) (domain.ProviderPayment, error) {
	var b createBody
	b.TransactionAmount = float64(in.AmountCents) / 100
	b.Description = "Pedido " + in.OrderReference
	b.PaymentMethodID = "pix"
	b.ExternalReference = in.OrderReference
	b.DateOfExpiration = in.ExpiresAt.Format(time.RFC3339)
	b.Payer.Email = in.PayerEmail
	raw, _ := json.Marshal(b)
	return c.do(ctx, http.MethodPost, "/v1/payments", in.IdempotencyKey, raw)
}
func (c *Client) GetPayment(ctx context.Context, id string) (domain.ProviderPayment, error) {
	if _, e := strconv.ParseInt(id, 10, 64); e != nil {
		return domain.ProviderPayment{}, errors.New("invalid payment id")
	}
	return c.do(ctx, http.MethodGet, "/v1/payments/"+url.PathEscape(id), "", nil)
}
func (c *Client) do(ctx context.Context, method, path, key string, body []byte) (domain.ProviderPayment, error) {
	req, e := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if e != nil {
		return domain.ProviderPayment{}, e
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", key)
	}
	res, e := c.http.Do(req)
	if e != nil {
		return domain.ProviderPayment{}, e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return domain.ProviderPayment{}, e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return domain.ProviderPayment{}, fmt.Errorf("provider status %d", res.StatusCode)
	}
	var p paymentResponse
	if json.Unmarshal(raw, &p) != nil {
		return domain.ProviderPayment{}, errors.New("invalid provider response")
	}
	id := fmt.Sprint(p.ID)
	if id == "<nil>" || p.Status == "" || p.Currency == "" {
		return domain.ProviderPayment{}, errors.New("incomplete provider response")
	}
	out := domain.ProviderPayment{ID: id, Status: p.Status, StatusDetail: p.StatusDetail, Currency: p.Currency, ExternalReference: p.ExternalReference, AmountCents: money(p.TransactionAmount), RefundedCents: money(p.TransactionAmountRefunded), QRCode: p.PointOfInteraction.TransactionData.QRCode, QRCodeBase64: p.PointOfInteraction.TransactionData.QRCodeBase64, TicketURL: p.PointOfInteraction.TransactionData.TicketURL}
	out.ExpiresAt = parseTime(p.DateOfExpiration)
	out.ApprovedAt = parseTime(p.DateApproved)
	out.UpdatedAt = parseTime(p.DateLastUpdated)
	return out, nil
}
func money(v float64) int64 { return int64(v*100 + 0.5) }
func parseTime(v string) *time.Time {
	if v == "" {
		return nil
	}
	t, e := time.Parse(time.RFC3339, v)
	if e != nil {
		t, e = time.Parse("2006-01-02T15:04:05.000-07:00", v)
	}
	if e != nil {
		return nil
	}
	return &t
}
func (c *Client) ValidateWebhook(signature, requestID, dataID string) bool {
	parts := map[string]string{}
	for _, v := range strings.Split(signature, ",") {
		p := strings.SplitN(strings.TrimSpace(v), "=", 2)
		if len(p) == 2 {
			parts[p[0]] = p[1]
		}
	}
	ts, hash := parts["ts"], parts["v1"]
	if ts == "" || hash == "" || requestID == "" || dataID == "" {
		return false
	}
	manifest := "id:" + strings.ToLower(dataID) + ";request-id:" + requestID + ";ts:" + ts + ";"
	m := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = m.Write([]byte(manifest))
	expected := hex.EncodeToString(m.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(hash))) == 1
}
