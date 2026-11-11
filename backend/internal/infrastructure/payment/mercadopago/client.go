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
func (Unavailable) CreateCard(context.Context, domain.CreateCardRequest) (domain.ProviderPayment, error) {
	return domain.ProviderPayment{}, errors.New("Mercado Pago unavailable")
}
func (Unavailable) GetPayment(context.Context, string) (domain.ProviderPayment, error) {
	return domain.ProviderPayment{}, errors.New("Mercado Pago unavailable")
}
func (Unavailable) ValidateWebhook(string, string, string) bool { return false }

type createBody struct {
	Type              string `json:"type"`
	TotalAmount       string `json:"total_amount"`
	ExternalReference string `json:"external_reference"`
	ProcessingMode    string `json:"processing_mode"`
	CaptureMode       string `json:"capture_mode,omitempty"`
	Config            *struct {
		Online struct {
			TransactionSecurity struct {
				Validation     string `json:"validation"`
				LiabilityShift string `json:"liability_shift"`
			} `json:"transaction_security"`
		} `json:"online"`
	} `json:"config,omitempty"`
	Payer struct {
		Email          string `json:"email"`
		Identification *struct {
			Type   string `json:"type"`
			Number string `json:"number"`
		} `json:"identification,omitempty"`
	} `json:"payer"`
	Transactions struct {
		Payments []struct {
			Amount         string `json:"amount"`
			ExpirationTime string `json:"expiration_time,omitempty"`
			PaymentMethod  struct {
				ID           string `json:"id"`
				Type         string `json:"type"`
				Token        string `json:"token,omitempty"`
				Installments int    `json:"installments,omitempty"`
			} `json:"payment_method"`
		} `json:"payments"`
	} `json:"transactions"`
}

type orderResponse struct {
	ID                string          `json:"id"`
	Status            string          `json:"status"`
	StatusDetail      string          `json:"status_detail"`
	CountryCode       string          `json:"country_code"`
	Currency          string          `json:"currency_id"`
	ExternalReference string          `json:"external_reference"`
	TotalAmount       json.RawMessage `json:"total_amount"`
	LastUpdatedDate   string          `json:"last_updated_date"`
	Transactions      struct {
		Payments []struct {
			Status         string `json:"status"`
			StatusDetail   string `json:"status_detail"`
			ExpirationTime string `json:"expiration_time"`
			PaymentMethod  struct {
				ID                  string `json:"id"`
				LastFour            string `json:"last_four_digits"`
				TicketURL           string `json:"ticket_url"`
				QRCode              string `json:"qr_code"`
				QRCodeBase64        string `json:"qr_code_base64"`
				TransactionSecurity struct {
					URL string `json:"url"`
				} `json:"transaction_security"`
			} `json:"payment_method"`
		} `json:"payments"`
		Refunds []struct {
			Amount json.RawMessage `json:"amount"`
		} `json:"refunds"`
	} `json:"transactions"`
}

func (c *Client) CreateCard(ctx context.Context, in domain.CreateCardRequest) (domain.ProviderPayment, error) {
	amount := formatCents(in.AmountCents)
	var b createBody
	b.Type, b.TotalAmount, b.ExternalReference = "online", amount, in.OrderReference
	b.ProcessingMode, b.CaptureMode, b.Payer.Email = "automatic", "automatic", in.PayerEmail
	b.Config = &struct {
		Online struct {
			TransactionSecurity struct {
				Validation     string `json:"validation"`
				LiabilityShift string `json:"liability_shift"`
			} `json:"transaction_security"`
		} `json:"online"`
	}{}
	b.Config.Online.TransactionSecurity.Validation = "on_fraud_risk"
	b.Config.Online.TransactionSecurity.LiabilityShift = "required"
	if in.IdentificationType != "" {
		b.Payer.Identification = &struct {
			Type   string `json:"type"`
			Number string `json:"number"`
		}{in.IdentificationType, in.IdentificationNumber}
	}
	b.Transactions.Payments = append(b.Transactions.Payments, struct {
		Amount         string `json:"amount"`
		ExpirationTime string `json:"expiration_time,omitempty"`
		PaymentMethod  struct {
			ID           string `json:"id"`
			Type         string `json:"type"`
			Token        string `json:"token,omitempty"`
			Installments int    `json:"installments,omitempty"`
		} `json:"payment_method"`
	}{})
	p := &b.Transactions.Payments[0]
	p.Amount, p.PaymentMethod.ID, p.PaymentMethod.Type = amount, in.PaymentMethodID, "credit_card"
	p.PaymentMethod.Token, p.PaymentMethod.Installments = in.Token, in.Installments
	raw, err := json.Marshal(b)
	if err != nil {
		return domain.ProviderPayment{}, err
	}
	return c.do(ctx, http.MethodPost, "/v1/orders", in.IdempotencyKey, raw)
}

func (c *Client) CreatePIX(ctx context.Context, in domain.CreateRequest) (domain.ProviderPayment, error) {
	amount := formatCents(in.AmountCents)
	var b createBody
	b.Type = "online"
	b.TotalAmount = amount
	b.ExternalReference = in.OrderReference
	b.ProcessingMode = "automatic"
	b.Payer.Email = in.PayerEmail
	b.Transactions.Payments = append(b.Transactions.Payments, struct {
		Amount         string `json:"amount"`
		ExpirationTime string `json:"expiration_time,omitempty"`
		PaymentMethod  struct {
			ID           string `json:"id"`
			Type         string `json:"type"`
			Token        string `json:"token,omitempty"`
			Installments int    `json:"installments,omitempty"`
		} `json:"payment_method"`
	}{})
	b.Transactions.Payments[0].Amount = amount
	b.Transactions.Payments[0].ExpirationTime = expirationDuration(time.Until(in.ExpiresAt))
	b.Transactions.Payments[0].PaymentMethod.ID = "pix"
	b.Transactions.Payments[0].PaymentMethod.Type = "bank_transfer"
	raw, e := json.Marshal(b)
	if e != nil {
		return domain.ProviderPayment{}, e
	}
	return c.do(ctx, http.MethodPost, "/v1/orders", in.IdempotencyKey, raw)
}

func (c *Client) GetPayment(ctx context.Context, orderID string) (domain.ProviderPayment, error) {
	if !strings.HasPrefix(orderID, "ORD") || len(orderID) < 6 {
		return domain.ProviderPayment{}, errors.New("invalid order id")
	}
	return c.do(ctx, http.MethodGet, "/v1/orders/"+url.PathEscape(orderID), "", nil)
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
	var order orderResponse
	if json.Unmarshal(raw, &order) != nil {
		return domain.ProviderPayment{}, errors.New("invalid provider response")
	}
	return mapOrder(order)
}

func mapOrder(order orderResponse) (domain.ProviderPayment, error) {
	if order.ID == "" || order.Status == "" || order.ExternalReference == "" || len(order.Transactions.Payments) == 0 {
		return domain.ProviderPayment{}, errors.New("incomplete provider response")
	}
	amount, e := parseCents(order.TotalAmount)
	if e != nil || amount <= 0 {
		return domain.ProviderPayment{}, errors.New("invalid provider amount")
	}
	currency := order.Currency
	if currency == "" && order.CountryCode == "BRA" {
		currency = "BRL"
	}
	if currency == "" {
		return domain.ProviderPayment{}, errors.New("incomplete provider currency")
	}
	payment := order.Transactions.Payments[0]
	status, detail := legacyStatus(order.Status, order.StatusDetail)
	refunded := int64(0)
	for _, refund := range order.Transactions.Refunds {
		value, err := parseCents(refund.Amount)
		if err != nil {
			return domain.ProviderPayment{}, errors.New("invalid provider refund amount")
		}
		refunded += value
	}
	out := domain.ProviderPayment{
		ID:                order.ID,
		Status:            status,
		StatusDetail:      detail,
		Currency:          currency,
		ExternalReference: order.ExternalReference,
		QRCode:            payment.PaymentMethod.QRCode,
		QRCodeBase64:      payment.PaymentMethod.QRCodeBase64,
		TicketURL:         payment.PaymentMethod.TicketURL,
		AmountCents:       amount,
		RefundedCents:     refunded,
		UpdatedAt:         parseTime(order.LastUpdatedDate),
		CardBrand:         payment.PaymentMethod.ID,
		CardLastFour:      payment.PaymentMethod.LastFour,
		AuthenticationURL: payment.PaymentMethod.TransactionSecurity.URL,
	}
	if out.AuthenticationURL != "" {
		u, err := url.Parse(out.AuthenticationURL)
		if err != nil || u.Scheme != "https" || !(u.Hostname() == "mercadopago.com" || strings.HasSuffix(u.Hostname(), ".mercadopago.com") || u.Hostname() == "mercadopago.com.br" || strings.HasSuffix(u.Hostname(), ".mercadopago.com.br")) {
			return domain.ProviderPayment{}, errors.New("invalid authentication URL")
		}
	}
	if payment.ExpirationTime != "" {
		out.ExpiresAt = parseTime(payment.ExpirationTime)
	}
	if status == "approved" || status == "refunded" || status == "charged_back" {
		out.ApprovedAt = out.UpdatedAt
	}
	return out, nil
}

func legacyStatus(status, detail string) (string, string) {
	switch status {
	case "created", "action_required":
		return "pending", detail
	case "processing":
		return "in_process", detail
	case "processed":
		return "approved", detail
	case "canceled":
		return "cancelled", detail
	case "failed":
		if detail == "expired" {
			return "cancelled", "expired"
		}
		return "rejected", detail
	case "refunded":
		return "refunded", detail
	case "charged_back":
		return "charged_back", detail
	default:
		return status, detail
	}
}

func formatCents(value int64) string {
	return fmt.Sprintf("%d.%02d", value/100, value%100)
}

func parseCents(raw json.RawMessage) (int64, error) {
	value := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if value == "" || strings.HasPrefix(value, "-") {
		return 0, errors.New("invalid amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid amount")
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil {
		return 0, e
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		if strings.Trim(fraction[2:], "0") != "" {
			return 0, errors.New("amount has sub-cent precision")
		}
		fraction = fraction[:2]
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents, e := strconv.ParseInt(fraction, 10, 64)
	if e != nil {
		return 0, e
	}
	return whole*100 + cents, nil
}

func expirationDuration(duration time.Duration) string {
	minutes := int64((duration + time.Minute - 1) / time.Minute)
	if minutes < 30 {
		minutes = 30
	}
	if minutes > 30*24*60 {
		minutes = 30 * 24 * 60
	}
	days := minutes / (24 * 60)
	minutes %= 24 * 60
	hours := minutes / 60
	minutes %= 60
	var b strings.Builder
	b.WriteString("P")
	if days > 0 {
		fmt.Fprintf(&b, "%dD", days)
	}
	if hours > 0 || minutes > 0 || days == 0 {
		b.WriteString("T")
		if hours > 0 {
			fmt.Fprintf(&b, "%dH", hours)
		}
		if minutes > 0 || hours == 0 {
			fmt.Fprintf(&b, "%dM", minutes)
		}
	}
	return b.String()
}

func parseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, e := time.Parse(time.RFC3339Nano, value)
	if e != nil {
		return nil
	}
	return &parsed
}

func (c *Client) ValidateWebhook(signature, requestID, dataID string) bool {
	parts := map[string]string{}
	for _, value := range strings.Split(signature, ",") {
		pair := strings.SplitN(strings.TrimSpace(value), "=", 2)
		if len(pair) == 2 {
			parts[pair[0]] = pair[1]
		}
	}
	ts, hash := parts["ts"], parts["v1"]
	if ts == "" || hash == "" || requestID == "" || dataID == "" {
		return false
	}
	manifest := "id:" + strings.ToLower(dataID) + ";request-id:" + requestID + ";ts:" + ts + ";"
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	_, _ = mac.Write([]byte(manifest))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(hash))) == 1
}
