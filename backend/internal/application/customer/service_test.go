package customer

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/customer"
)

type fakeRepository struct{ created bool }

func (f *fakeRepository) CreateMagicLink(context.Context, string, string, string, time.Time) error {
	f.created = true
	return nil
}
func (*fakeRepository) ConsumeMagicLink(context.Context, string, string, string, string, string, time.Time) (domain.Session, error) {
	return domain.Session{}, nil
}
func (*fakeRepository) GetSession(context.Context, string) (domain.Session, error) {
	return domain.Session{}, nil
}
func (*fakeRepository) RevokeSession(context.Context, string) error { return nil }
func (*fakeRepository) Profile(context.Context, string) (domain.Profile, error) {
	return domain.Profile{}, nil
}
func (*fakeRepository) ListQuotes(context.Context, string) ([]domain.QuoteSummary, error) {
	return nil, nil
}
func (*fakeRepository) GetQuote(context.Context, string, string) (domain.QuoteDetail, error) {
	return domain.QuoteDetail{}, nil
}
func (*fakeRepository) Decide(context.Context, string, string, string, string, string) error {
	return nil
}

type failingMailer struct{}

func (failingMailer) SendMagicLink(context.Context, string, string) error {
	return errors.New("provider unavailable")
}
func (failingMailer) SendDecision(context.Context, string, string, string) error {
	return errors.New("provider unavailable")
}

func TestRequestMagicLinkReportsProviderFailureWithoutLosingToken(t *testing.T) {
	repo := &fakeRepository{}
	service := New(repo, failingMailer{}, "http://localhost:3000")
	if err := service.RequestMagicLink(context.Background(), "customer@example.com", "127.0.0.1"); err == nil {
		t.Fatal("expected provider failure")
	}
	if !repo.created {
		t.Fatal("hashed token was not persisted before delivery")
	}
}
func TestCSRFUsesHashedToken(t *testing.T) {
	if !CSRF("secret", Hash("secret")) {
		t.Fatal("valid CSRF rejected")
	}
	if CSRF("other", Hash("secret")) {
		t.Fatal("invalid CSRF accepted")
	}
}
