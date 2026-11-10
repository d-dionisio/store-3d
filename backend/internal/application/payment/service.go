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
)

type Provider interface {
	CreatePIX(context.Context, domain.CreateRequest) (domain.ProviderPayment, error)
	GetPayment(context.Context, string) (domain.ProviderPayment, error)
	ValidateWebhook(string, string, string) bool
}
type Repository interface {
	ReservePIX(context.Context, string, string, time.Time) (domain.CreateRequest, *domain.PIX, error)
	CompletePIX(context.Context, domain.CreateRequest, domain.ProviderPayment) (domain.PIX, error)
	FailAttempt(context.Context, string, string) error
	Status(context.Context, string, string) (domain.Status, error)
	RecordWebhook(context.Context, domain.Webhook) (bool, error)
	Reconcile(context.Context, domain.Webhook, domain.ProviderPayment) error
	MarkWebhookFailed(context.Context, string, string) error
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
