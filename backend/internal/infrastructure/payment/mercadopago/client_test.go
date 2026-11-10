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

func TestCreatePIX(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/payments" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-Idempotency-Key") != "idem-1" {
			t.Fatalf("unexpected request")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["payment_method_id"] != "pix" || body["external_reference"] != "PED-1" {
			t.Fatalf("body=%v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"status":"pending","status_detail":"pending_waiting_transfer","currency_id":"BRL","external_reference":"PED-1","transaction_amount":25.9,"date_of_expiration":"2026-11-11T12:00:00-03:00","point_of_interaction":{"transaction_data":{"qr_code":"000201","qr_code_base64":"abc","ticket_url":"https://example.test/ticket"}}}`))
	}))
	defer srv.Close()
	c, e := New(srv.URL, "test-token", "secret", srv.Client())
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.CreatePIX(context.Background(), domain.CreateRequest{IdempotencyKey: "idem-1", OrderReference: "PED-1", PayerEmail: "buyer@example.com", AmountCents: 2590, ExpiresAt: time.Now().Add(time.Hour)})
	if e != nil || p.ID != "123" || p.AmountCents != 2590 || p.QRCode != "000201" {
		t.Fatalf("payment=%+v err=%v", p, e)
	}
}
func TestGetPaymentAndProviderFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/payments/456" {
			_, _ = w.Write([]byte(`{"id":456,"status":"approved","status_detail":"accredited","currency_id":"BRL","external_reference":"PED-2","transaction_amount":10}`))
			return
		}
		http.Error(w, "down", 503)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "token", "secret", srv.Client())
	p, e := c.GetPayment(context.Background(), "456")
	if e != nil || p.Status != "approved" {
		t.Fatalf("payment=%+v err=%v", p, e)
	}
	if _, e = c.GetPayment(context.Background(), "invalid"); e == nil {
		t.Fatal("expected invalid id")
	}
}
func TestTimeout(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(50 * time.Millisecond) }))
	defer srv.Close()
	h := srv.Client()
	h.Timeout = 5 * time.Millisecond
	c, _ := New(srv.URL, "token", "secret", h)
	if _, e := c.GetPayment(context.Background(), "1"); e == nil {
		t.Fatal("expected timeout")
	}
}
func TestWebhookSignature(t *testing.T) {
	c, _ := New("https://api.mercadopago.com", "token", "secret", http.DefaultClient)
	manifest := "id:123;request-id:req-1;ts:1700000000000;"
	m := hmac.New(sha256.New, []byte("secret"))
	_, _ = m.Write([]byte(manifest))
	signature := "ts=1700000000000,v1=" + hex.EncodeToString(m.Sum(nil))
	if !c.ValidateWebhook(signature, "req-1", "123") {
		t.Fatal("valid signature rejected")
	}
	if c.ValidateWebhook(signature, "req-2", "123") {
		t.Fatal("invalid signature accepted")
	}
}
