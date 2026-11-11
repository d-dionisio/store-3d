package payment

import (
	"context"
	"errors"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"testing"
	"time"
)

type fakeProvider struct {
	created int
	payment domain.ProviderPayment
	err     error
	valid   bool
}

func (f *fakeProvider) CreatePIX(context.Context, domain.CreateRequest) (domain.ProviderPayment, error) {
	f.created++
	return f.payment, f.err
}
func (f *fakeProvider) CreateCard(context.Context, domain.CreateCardRequest) (domain.ProviderPayment, error) {
	f.created++
	return f.payment, f.err
}
func (f *fakeProvider) GetPayment(context.Context, string) (domain.ProviderPayment, error) {
	return f.payment, f.err
}
func (f *fakeProvider) ValidateWebhook(string, string, string) bool { return f.valid }

type fakeRepo struct {
	req        domain.CreateRequest
	existing   *domain.PIX
	reserveErr error
	fresh      bool
	reconciled int
	failed     int
}

func (f *fakeRepo) ReservePIX(context.Context, string, string, time.Time) (domain.CreateRequest, *domain.PIX, error) {
	return f.req, f.existing, f.reserveErr
}
func (f *fakeRepo) CompletePIX(context.Context, domain.CreateRequest, domain.ProviderPayment) (domain.PIX, error) {
	return domain.PIX{AttemptID: f.req.AttemptID, Status: "pending"}, nil
}
func (f *fakeRepo) ReserveCard(context.Context, string, string, domain.CardInput) (domain.CreateCardRequest, *domain.Card, error) {
	return domain.CreateCardRequest{}, nil, f.reserveErr
}
func (f *fakeRepo) CompleteCard(context.Context, domain.CreateCardRequest, domain.ProviderPayment) (domain.Card, error) {
	return domain.Card{}, nil
}
func (f *fakeRepo) FailAttempt(context.Context, string, string) error { f.failed++; return nil }
func (f *fakeRepo) Status(context.Context, string, string) (domain.Status, error) {
	return domain.Status{}, nil
}
func (f *fakeRepo) RecordWebhook(context.Context, domain.Webhook) (bool, error) { return f.fresh, nil }
func (f *fakeRepo) Reconcile(context.Context, domain.Webhook, domain.ProviderPayment) error {
	f.reconciled++
	return nil
}
func (f *fakeRepo) MarkWebhookFailed(context.Context, string, string) error { return nil }
func TestCreatePIXValidatesProviderResult(t *testing.T) {
	r := &fakeRepo{req: domain.CreateRequest{AttemptID: "a", OrderReference: "PED-1", AmountCents: 1000}}
	p := &fakeProvider{payment: domain.ProviderPayment{ID: "1", Status: "pending", Currency: "BRL", ExternalReference: "PED-OTHER", AmountCents: 1000}}
	_, e := New(r, p, time.Hour).CreatePIX(context.Background(), "u", "PED-1")
	if !errors.Is(e, ErrValueMismatch) || r.failed != 1 {
		t.Fatalf("err=%v failed=%d", e, r.failed)
	}
}
func TestCreatePIXReusesExistingAttempt(t *testing.T) {
	existing := &domain.PIX{AttemptID: "a", Status: "pending"}
	r := &fakeRepo{existing: existing}
	p := &fakeProvider{}
	got, e := New(r, p, time.Hour).CreatePIX(context.Background(), "u", "PED-1")
	if e != nil || got.AttemptID != "a" || p.created != 0 {
		t.Fatalf("got=%+v calls=%d err=%v", got, p.created, e)
	}
}
func TestCreateCardRejectsInvalidTokenBeforeRepository(t *testing.T) {
	_, err := New(&fakeRepo{}, &fakeProvider{}, time.Hour).CreateCard(context.Background(), "u", "PED-1", domain.CardInput{Token: "short", PaymentMethodID: "visa", Installments: 1, IdempotencyKey: "1234567890123456"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err=%v", err)
	}
}
func TestWebhookValidationAndDeduplication(t *testing.T) {
	r := &fakeRepo{fresh: true}
	p := &fakeProvider{valid: false}
	s := New(r, p, time.Hour)
	w := domain.Webhook{EventID: "e", ExternalPaymentID: "1"}
	if e := s.Webhook(context.Background(), w); !errors.Is(e, ErrInvalidWebhook) {
		t.Fatalf("err=%v", e)
	}
	p.valid = true
	p.payment = domain.ProviderPayment{ID: "1"}
	if e := s.Webhook(context.Background(), w); e != nil || r.reconciled != 1 {
		t.Fatalf("err=%v reconciled=%d", e, r.reconciled)
	}
	r.fresh = false
	if e := s.Webhook(context.Background(), w); e != nil || r.reconciled != 1 {
		t.Fatalf("duplicate err=%v reconciled=%d", e, r.reconciled)
	}
}
