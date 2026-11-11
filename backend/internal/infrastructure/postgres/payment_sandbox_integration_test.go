package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	paymentapp "example.com/loja-3d/backend/internal/application/payment"
	paymentdomain "example.com/loja-3d/backend/internal/domain/payment"
	"example.com/loja-3d/backend/internal/infrastructure/payment/mercadopago"
)

// TestPaymentSandboxEndToEnd is opt-in because it creates a Mercado Pago sandbox
// order. It persists only test artifacts and removes the local fixture afterward.
func TestPaymentSandboxEndToEnd(t *testing.T) {
	if os.Getenv("RUN_MERCADO_PAGO_SANDBOX") != "1" {
		t.Skip("set RUN_MERCADO_PAGO_SANDBOX=1 to run the PostgreSQL/sandbox flow")
	}
	token := os.Getenv("MERCADO_PAGO_SANDBOX_ACCESS_TOKEN")
	secret := os.Getenv("MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET")
	if token == "" || secret == "" {
		t.Fatal("sandbox token and webhook secret are required")
	}

	ctx := context.Background()
	fixture, reference, repository := paymentFixture(t)
	readyPaymentOrder(t, fixture, reference)

	provider, err := mercadopago.New("https://api.mercadopago.com", token, secret, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service := paymentapp.New(repository, provider, 30*time.Minute)
	pix, err := service.CreatePIX(ctx, fixture.userID, reference)
	if err != nil {
		t.Fatalf("create and persist sandbox PIX: %v", err)
	}
	if pix.QRCode == "" || pix.TicketURL == "" || pix.AmountCents != 1001 {
		t.Fatal("persisted sandbox PIX is incomplete")
	}

	var externalID, attemptStatus string
	err = fixture.pool.QueryRow(ctx, `SELECT a.external_payment_id,a.status FROM payment_attempts a JOIN payments p ON p.id=a.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, reference).Scan(&externalID, &attemptStatus)
	if err != nil || !strings.HasPrefix(externalID, "ORD") || attemptStatus != "pending" {
		t.Fatalf("attempt was not persisted correctly: status=%q err=%v", attemptStatus, err)
	}

	requestID := "sandbox-integration-request"
	timestamp := fmt.Sprint(time.Now().UnixMilli())
	manifest := "id:" + strings.ToLower(externalID) + ";request-id:" + requestID + ";ts:" + timestamp + ";"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(manifest))
	signature := "ts=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	payload := []byte(fmt.Sprintf(`{"action":"order.updated","type":"order","data":{"id":%q}}`, externalID))
	webhook := paymentdomain.Webhook{
		EventID:           "sandbox-order-" + reference,
		EventType:         "order.updated",
		ExternalPaymentID: externalID,
		RequestID:         requestID,
		Signature:         signature,
		Payload:           payload,
	}
	if err = service.Webhook(ctx, webhook); err != nil {
		t.Fatalf("reconcile signed sandbox webhook: %v", err)
	}
	if err = service.Webhook(ctx, webhook); err != nil {
		t.Fatalf("deduplicate signed sandbox webhook: %v", err)
	}

	var events, processed int
	err = fixture.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE processing_status='processed') FROM payment_webhook_events WHERE provider='mercado_pago' AND external_event_id=$1`, webhook.EventID).Scan(&events, &processed)
	if err != nil || events != 1 || processed != 1 {
		t.Fatalf("webhook persistence mismatch: events=%d processed=%d err=%v", events, processed, err)
	}
	t.Log("sandbox payment persisted and signed webhook reconciled idempotently")
}
