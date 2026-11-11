package mercadopago

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const pendingOrder = `{"id":"ORD01TEST","status":"action_required","status_detail":"waiting_transfer","country_code":"BRA","external_reference":"PED-1","total_amount":"25.90","last_updated_date":"2026-11-11T12:00:00.123Z","transactions":{"payments":[{"id":"PAY01TEST","status":"action_required","status_detail":"waiting_transfer","amount":"25.90","payment_method":{"id":"pix","type":"bank_transfer","qr_code":"000201","qr_code_base64":"abc","ticket_url":"https://example.test/ticket"}}]}}`

func TestCreatePIXWithOrdersAPI(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orders" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-Idempotency-Key") != "idem-1" {
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
		var body struct {
			Type              string `json:"type"`
			TotalAmount       string `json:"total_amount"`
			ExternalReference string `json:"external_reference"`
			ProcessingMode    string `json:"processing_mode"`
			Payer             struct {
				Email string `json:"email"`
			} `json:"payer"`
			Transactions struct {
				Payments []struct {
					Amount         string `json:"amount"`
					ExpirationTime string `json:"expiration_time"`
					PaymentMethod  struct {
						ID   string `json:"id"`
						Type string `json:"type"`
					} `json:"payment_method"`
				} `json:"payments"`
			} `json:"transactions"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Type != "online" || body.TotalAmount != "25.90" || body.ExternalReference != "PED-1" || body.ProcessingMode != "automatic" || body.Payer.Email != "buyer@example.com" {
			t.Fatalf("body=%+v", body)
		}
		payment := body.Transactions.Payments[0]
		if payment.Amount != "25.90" || payment.ExpirationTime != "PT30M" || payment.PaymentMethod.ID != "pix" || payment.PaymentMethod.Type != "bank_transfer" {
			t.Fatalf("payment=%+v", payment)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(pendingOrder))
	}))
	defer srv.Close()
	c, e := New(srv.URL, "test-token", "secret", srv.Client())
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.CreatePIX(context.Background(), domain.CreateRequest{IdempotencyKey: "idem-1", OrderReference: "PED-1", PayerEmail: "buyer@example.com", AmountCents: 2590, ExpiresAt: time.Now().Add(30 * time.Minute)})
	if e != nil || p.ID != "ORD01TEST" || p.Status != "pending" || p.Currency != "BRL" || p.AmountCents != 2590 || p.QRCode != "000201" {
		t.Fatalf("payment=%+v err=%v", p, e)
	}
}

func TestCreateCardWithOrdersAPIAnd3DS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CaptureMode string `json:"capture_mode"`
			Config      struct {
				Online struct {
					TransactionSecurity struct {
						Validation     string `json:"validation"`
						LiabilityShift string `json:"liability_shift"`
					} `json:"transaction_security"`
				} `json:"online"`
			} `json:"config"`
			Transactions struct {
				Payments []struct {
					PaymentMethod struct {
						ID           string `json:"id"`
						Type         string `json:"type"`
						Token        string `json:"token"`
						Installments int    `json:"installments"`
					} `json:"payment_method"`
				} `json:"payments"`
			} `json:"transactions"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Fatal("invalid request")
		}
		method := body.Transactions.Payments[0].PaymentMethod
		if body.CaptureMode != "automatic" || body.Config.Online.TransactionSecurity.Validation != "on_fraud_risk" || body.Config.Online.TransactionSecurity.LiabilityShift != "required" || method.ID != "master" || method.Type != "credit_card" || method.Token != "card-token" || method.Installments != 3 {
			t.Fatalf("body=%+v", body)
		}
		_, _ = w.Write([]byte(`{"id":"ORD01CARD","status":"action_required","status_detail":"pending_challenge","country_code":"BRA","external_reference":"PED-CARD","total_amount":"120.00","last_updated_date":"2026-11-11T12:00:00Z","transactions":{"payments":[{"status":"action_required","status_detail":"pending_challenge","payment_method":{"id":"master","last_four_digits":"1234","transaction_security":{"url":"https://www.mercadopago.com/auth/card/challenge"}}}]}}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "token", "secret", srv.Client())
	p, err := c.CreateCard(context.Background(), domain.CreateCardRequest{IdempotencyKey: "idem-card", OrderReference: "PED-CARD", PayerEmail: "buyer@example.com", AmountCents: 12000, Token: "card-token", PaymentMethodID: "master", Installments: 3})
	if err != nil || p.Status != "pending" || p.StatusDetail != "pending_challenge" || p.AuthenticationURL == "" || p.CardLastFour != "1234" {
		t.Fatalf("payment=%+v err=%v", p, err)
	}
}

func TestGetOrderAndStatusMapping(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orders/ORD01APPROVED" {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"id":"ORD01APPROVED","status":"processed","status_detail":"partially_refunded","country_code":"BRA","external_reference":"PED-2","total_amount":"10.00","last_updated_date":"2026-11-11T12:00:00Z","transactions":{"payments":[{"status":"processed","status_detail":"partially_refunded","amount":"10.00","payment_method":{"id":"pix","type":"bank_transfer"}}],"refunds":[{"amount":"2.50"}]}}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "token", "secret", srv.Client())
	p, e := c.GetPayment(context.Background(), "ORD01APPROVED")
	if e != nil || p.Status != "approved" || p.RefundedCents != 250 || p.AmountCents != 1000 || p.ApprovedAt == nil {
		t.Fatalf("payment=%+v err=%v", p, e)
	}
	if _, e = c.GetPayment(context.Background(), "invalid"); e == nil {
		t.Fatal("expected invalid order id")
	}
}

func TestProviderFailureAndTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	h := srv.Client()
	h.Timeout = 5 * time.Millisecond
	c, _ := New(srv.URL, "token", "secret", h)
	if _, e := c.GetPayment(context.Background(), "ORD01TIMEOUT"); e == nil {
		t.Fatal("expected timeout")
	}
}

func TestCardDeclineMapsOrderFromPaymentRequiredEnvelope(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"errors":[{"code":"failed"}],"data":{"id":"ORD01DECLINED","status":"failed","status_detail":"failed","country_code":"BRA","external_reference":"PED-DECLINED","total_amount":"50.00","last_updated_date":"2026-11-11T12:00:00Z","transactions":{"payments":[{"status":"failed","status_detail":"cc_rejected_other_reason","payment_method":{"id":"master","type":"credit_card"}}]}}}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "token", "secret", srv.Client())
	p, err := c.CreateCard(context.Background(), domain.CreateCardRequest{IdempotencyKey: "decline-key", OrderReference: "PED-DECLINED", PayerEmail: "buyer@example.com", AmountCents: 5000, Token: "card-token", PaymentMethodID: "master", Installments: 1})
	if err != nil || p.Status != "rejected" || p.ID != "ORD01DECLINED" {
		t.Fatalf("payment=%+v err=%v", p, err)
	}
}

func TestMoneyExpirationAndStatusMapping(t *testing.T) {
	for input, expected := range map[string]int64{`"10"`: 1000, `"10.2"`: 1020, `"10.20"`: 1020, `10.200`: 1020} {
		got, e := parseCents(json.RawMessage(input))
		if e != nil || got != expected {
			t.Errorf("parseCents(%s)=%d,%v want %d", input, got, e, expected)
		}
	}
	if _, e := parseCents(json.RawMessage(`"10.201"`)); e == nil {
		t.Fatal("expected sub-cent error")
	}
	if got := expirationDuration(31*time.Hour + 2*time.Minute); got != "P1DT7H2M" {
		t.Fatalf("expiration=%s", got)
	}
	for _, test := range []struct{ input, detail, status string }{
		{"action_required", "waiting_transfer", "pending"},
		{"processing", "in_process", "in_process"},
		{"processed", "accredited", "approved"},
		{"canceled", "expired", "cancelled"},
		{"failed", "failed", "rejected"},
		{"refunded", "refunded", "refunded"},
	} {
		status, _ := legacyStatus(test.input, test.detail)
		if status != test.status {
			t.Errorf("legacyStatus(%s)=%s want %s", test.input, status, test.status)
		}
	}
}

func TestWebhookSignature(t *testing.T) {
	c, _ := New("https://api.mercadopago.com", "token", "secret", http.DefaultClient)
	manifest := "id:ord01test;request-id:req-1;ts:1700000000000;"
	m := hmac.New(sha256.New, []byte("secret"))
	_, _ = m.Write([]byte(manifest))
	signature := "ts=1700000000000,v1=" + hex.EncodeToString(m.Sum(nil))
	if !c.ValidateWebhook(signature, "req-1", "ORD01TEST") {
		t.Fatal("valid signature rejected")
	}
	if c.ValidateWebhook(signature, "req-2", "ORD01TEST") {
		t.Fatal("invalid signature accepted")
	}
}
