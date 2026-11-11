package mercadopago

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/payment"
)

// TestSandboxPIX is opt-in because it creates an order in Mercado Pago's test
// environment. It never logs credentials, QR codes, payer data, or provider IDs.
func TestSandboxPIX(t *testing.T) {
	if os.Getenv("RUN_MERCADO_PAGO_SANDBOX") != "1" {
		t.Skip("set RUN_MERCADO_PAGO_SANDBOX=1 to run against Mercado Pago sandbox")
	}
	token := os.Getenv("MERCADO_PAGO_SANDBOX_ACCESS_TOKEN")
	secret := os.Getenv("MERCADO_PAGO_SANDBOX_WEBHOOK_SECRET")
	if token == "" || secret == "" {
		t.Fatal("sandbox token and webhook secret are required")
	}
	client, err := New("https://api.mercadopago.com", token, secret, &http.Client{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	reference := fmt.Sprintf("LOJA3D-SANDBOX-%d", now.UnixNano())
	request := domain.CreateRequest{
		IdempotencyKey: reference,
		OrderReference: reference,
		PayerEmail:     "test_payer@example.com",
		AmountCents:    5000,
		ExpiresAt:      now.Add(30 * time.Minute),
	}
	created, err := client.CreatePIX(context.Background(), request)
	if err != nil {
		t.Fatalf("create sandbox PIX order: %v", err)
	}
	if created.ID == "" || created.Status == "" || created.Currency != "BRL" || created.ExternalReference != reference || created.AmountCents != request.AmountCents {
		t.Fatalf("invalid sandbox response: status=%q currency=%q reference=%t amount=%d", created.Status, created.Currency, created.ExternalReference == reference, created.AmountCents)
	}
	if created.QRCode == "" || created.TicketURL == "" {
		t.Fatal("sandbox response did not include PIX artifacts")
	}
	repeated, err := client.CreatePIX(context.Background(), request)
	if err != nil {
		t.Fatalf("repeat idempotent sandbox PIX order: %v", err)
	}
	if repeated.ID != created.ID {
		t.Fatal("provider did not preserve the order for the idempotency key")
	}
	queried, err := client.GetPayment(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("query sandbox PIX order: %v", err)
	}
	if queried.ID != created.ID || queried.ExternalReference != reference || queried.Currency != "BRL" || queried.AmountCents != request.AmountCents {
		t.Fatal("queried sandbox order does not match the created order")
	}
	t.Logf("sandbox Orders API validated: status=%s currency=%s artifacts=true", queried.Status, queried.Currency)
}
