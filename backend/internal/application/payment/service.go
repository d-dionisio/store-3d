package payment

import (
	"context"
	"errors"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"time"
)

var (
	ErrNotFound       = errors.New("payment not found")
	ErrForbidden      = errors.New("forbidden")
	ErrNotReady       = errors.New("order not ready")
	ErrConflict       = errors.New("payment conflict")
	ErrUnavailable    = errors.New("payment provider unavailable")
	ErrInvalidWebhook = errors.New("invalid webhook")
	ErrValueMismatch  = errors.New("payment value mismatch")
	ErrInvalidInput   = errors.New("invalid payment input")
)

type Provider interface {
	CreatePIX(context.Context, domain.CreateRequest) (domain.ProviderPayment, error)
	CreateCard(context.Context, domain.CreateCardRequest) (domain.ProviderPayment, error)
	GetPayment(context.Context, string) (domain.ProviderPayment, error)
	ValidateWebhook(string, string, string) bool
}
type Repository interface {
	ReservePIX(context.Context, string, string, time.Time) (domain.CreateRequest, *domain.PIX, error)
	CompletePIX(context.Context, domain.CreateRequest, domain.ProviderPayment) (domain.PIX, error)
	ReserveCard(context.Context, string, string, domain.CardInput) (domain.CreateCardRequest, *domain.Card, error)
	CompleteCard(context.Context, domain.CreateCardRequest, domain.ProviderPayment) (domain.Card, error)
	FailAttempt(context.Context, string, string) error
	Status(context.Context, string, string) (domain.Status, error)
	RecordWebhook(context.Context, domain.Webhook) (bool, error)
	Reconcile(context.Context, domain.Webhook, domain.ProviderPayment) error
	MarkWebhookFailed(context.Context, string, string) error
}

func (s *Service) CreateCard(ctx context.Context, userID, reference string, input domain.CardInput) (domain.Card, error) {
	if len(input.Token) < 8 || len(input.Token) > 512 || len(input.PaymentMethodID) < 1 || len(input.PaymentMethodID) > 64 || input.Installments < 1 || input.Installments > 24 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return domain.Card{}, ErrInvalidInput
	}
	if (input.IdentificationType == "") != (input.IdentificationNumber == "") || len(input.IdentificationType) > 16 || len(input.IdentificationNumber) > 32 {
		return domain.Card{}, ErrInvalidInput
	}
	req, existing, err := s.repo.ReserveCard(ctx, userID, reference, input)
	if err != nil {
		return domain.Card{}, err
	}
	if existing != nil {
		return *existing, nil
	}
	result, err := s.provider.CreateCard(ctx, req)
	if err != nil {
		_ = s.repo.FailAttempt(ctx, req.AttemptID, "provider_unavailable")
		return domain.Card{}, ErrUnavailable
	}
	if result.ExternalReference != req.OrderReference || result.Currency != "BRL" || result.AmountCents != req.AmountCents {
		_ = s.repo.FailAttempt(ctx, req.AttemptID, "provider_response_mismatch")
		return domain.Card{}, ErrValueMismatch
	}
	return s.repo.CompleteCard(ctx, req, result)
}

type Service struct {
	repo     Repository
	provider Provider
	now      func() time.Time
	ttl      time.Duration
}

func New(repo Repository, provider Provider, ttl time.Duration) *Service {
	return &Service{repo: repo, provider: provider, now: time.Now, ttl: ttl}
}
func (s *Service) CreatePIX(ctx context.Context, userID, reference string) (domain.PIX, error) {
	req, existing, e := s.repo.ReservePIX(ctx, userID, reference, s.now().Add(s.ttl))
	if e != nil {
		return domain.PIX{}, e
	}
	if existing != nil {
		return *existing, nil
	}
	result, e := s.provider.CreatePIX(ctx, req)
	if e != nil {
		return domain.PIX{}, ErrUnavailable
	}
	if result.ExternalReference != req.OrderReference || result.Currency != "BRL" || result.AmountCents != req.AmountCents {
		_ = s.repo.FailAttempt(ctx, req.AttemptID, "provider_response_mismatch")
		return domain.PIX{}, ErrValueMismatch
	}
	return s.repo.CompletePIX(ctx, req, result)
}
func (s *Service) Status(ctx context.Context, userID, reference string) (domain.Status, error) {
	return s.repo.Status(ctx, userID, reference)
}
func (s *Service) Webhook(ctx context.Context, w domain.Webhook) error {
	if !s.provider.ValidateWebhook(w.Signature, w.RequestID, w.ExternalPaymentID) {
		return ErrInvalidWebhook
	}
	fresh, e := s.repo.RecordWebhook(ctx, w)
	if e != nil {
		return e
	}
	if !fresh {
		return nil
	}
	p, e := s.provider.GetPayment(ctx, w.ExternalPaymentID)
	if e != nil {
		_ = s.repo.MarkWebhookFailed(ctx, w.EventID, "provider lookup failed")
		return ErrUnavailable
	}
	if e = s.repo.Reconcile(ctx, w, p); e != nil {
		_ = s.repo.MarkWebhookFailed(ctx, w.EventID, "reconciliation failed")
		return e
	}
	return nil
}
