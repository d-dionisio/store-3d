package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// TestCardSandboxEndToEnd validates the real sandbox Order and authenticated
// reconciliation while using a locally generated signed webhook trigger. Real
// webhook delivery through the public HTTPS endpoint is reported separately.
func TestCardSandboxEndToEnd(t *testing.T) {
	if os.Getenv("RUN_MERCADO_PAGO_CARD_SANDBOX") != "1" {
		t.Skip("card sandbox test is opt-in")
	}
	cardToken, access, secret := os.Getenv("MERCADO_PAGO_SANDBOX_CARD_TOKEN"), os.Getenv("MERCADO_PAGO_SANDBOX_ACCESS_TOKEN"), os.Getenv("MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET")
	if cardToken == "" || access == "" || secret == "" {
		t.Fatal("sandbox credentials and a single-use card token are required")
	}
	ctx := context.Background()
	fixture, reference, repository := paymentFixture(t)
	readyPaymentOrder(t, fixture, reference)
	provider, err := mercadopago.New("https://api.mercadopago.com", access, secret, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service := paymentapp.New(repository, provider, 30*time.Minute)
	input := paymentdomain.CardInput{Token: cardToken, PaymentMethodID: "master", Installments: 1, IdempotencyKey: "card-" + reference, IdentificationType: "CPF", IdentificationNumber: "12345678909"}
	card, err := service.CreateCard(ctx, fixture.userID, reference, input)
	if err != nil {
		t.Fatalf("create and persist sandbox card: %v", err)
	}
	if card.AmountCents != 1001 {
		t.Fatalf("persisted amount=%d", card.AmountCents)
	}
	repeated, err := service.CreateCard(ctx, fixture.userID, reference, input)
	if err != nil || repeated.AttemptID != card.AttemptID {
		t.Fatalf("idempotent retry created another attempt: err=%v", err)
	}
	if _, err = service.CreatePIX(ctx, fixture.userID, reference); !errors.Is(err, paymentapp.ErrConflict) {
		t.Fatalf("PIX/card conflict was not enforced: %v", err)
	}
	var externalID string
	err = fixture.pool.QueryRow(ctx, `SELECT a.external_payment_id FROM payment_attempts a JOIN payments p ON p.id=a.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, reference).Scan(&externalID)
	if err != nil || !strings.HasPrefix(externalID, "ORD") {
		t.Fatalf("external Order was not persisted: %v", err)
	}
	requestID, timestamp := "sandbox-card-request", fmt.Sprint(time.Now().UnixMilli())
	manifest := "id:" + strings.ToLower(externalID) + ";request-id:" + requestID + ";ts:" + timestamp + ";"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(manifest))
	webhook := paymentdomain.Webhook{EventID: "sandbox-card-" + reference, EventType: "order.updated", ExternalPaymentID: externalID, RequestID: requestID, Signature: "ts=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil)), Payload: []byte(fmt.Sprintf(`{"action":"order.updated","type":"order","data":{"id":%q}}`, externalID))}
	if err = service.Webhook(ctx, webhook); err != nil {
		t.Fatalf("reconcile authenticated sandbox Order: %v", err)
	}
	if err = service.Webhook(ctx, webhook); err != nil {
		t.Fatalf("deduplicate sandbox card event: %v", err)
	}
	var orderStatus, paymentStatus string
	var attempts, events int
	err = fixture.pool.QueryRow(ctx, `SELECT o.status,p.status,(SELECT count(*) FROM payment_attempts WHERE payment_id=p.id),(SELECT count(*) FROM payment_webhook_events WHERE external_event_id=$2) FROM orders o JOIN payments p ON p.order_id=o.id WHERE o.order_number=$1`, reference, webhook.EventID).Scan(&orderStatus, &paymentStatus, &attempts, &events)
	if err != nil || orderStatus != "confirmed" || paymentStatus != "paid" || attempts != 1 || events != 1 {
		t.Fatalf("reconciliation mismatch order=%s payment=%s attempts=%d events=%d err=%v", orderStatus, paymentStatus, attempts, events, err)
	}
	t.Log("real sandbox card Order persisted; authenticated lookup, confirmation and deduplication validated")
}

func TestCardSandboxRealWebhookDelivery(t *testing.T) {
	if os.Getenv("RUN_MERCADO_PAGO_REAL_WEBHOOK") != "1" {
		t.Skip("real webhook delivery test is opt-in")
	}
	cardToken, access, secret := os.Getenv("MERCADO_PAGO_SANDBOX_CARD_TOKEN"), os.Getenv("MERCADO_PAGO_SANDBOX_ACCESS_TOKEN"), os.Getenv("MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET")
	if cardToken == "" || access == "" || secret == "" {
		t.Fatal("sandbox credentials and a single-use card token are required")
	}
	ctx := context.Background()
	fixture, reference, repository := paymentFixture(t)
	readyPaymentOrder(t, fixture, reference)
	provider, err := mercadopago.New("https://api.mercadopago.com", access, secret, &http.Client{Timeout: 15 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service := paymentapp.New(repository, provider, 30*time.Minute)
	card, err := service.CreateCard(ctx, fixture.userID, reference, paymentdomain.CardInput{Token: cardToken, PaymentMethodID: "master", Installments: 1, IdempotencyKey: "webhook-" + reference, IdentificationType: "CPF", IdentificationNumber: "12345678909"})
	if err != nil {
		t.Fatalf("create sandbox card for webhook delivery: %v", err)
	}
	var externalID string
	if err = fixture.pool.QueryRow(ctx, `SELECT external_payment_id FROM payment_attempts WHERE id=$1`, card.AttemptID).Scan(&externalID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		var processed int
		var orderStatus string
		err = fixture.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM payment_webhook_events WHERE provider='mercado_pago' AND external_payment_id=$1 AND processing_status='processed'),o.status FROM orders o WHERE o.order_number=$2`, externalID, reference).Scan(&processed, &orderStatus)
		if err == nil && processed > 0 && orderStatus == "confirmed" {
			t.Log("real HTTPS webhook delivery validated and order confirmed")
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("no processed Mercado Pago webhook reached the local order within 45 seconds")
}
