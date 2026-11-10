package shipping

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/shipping"
)

var (
	ErrInvalid     = errors.New("invalid shipping data")
	ErrNotFound    = errors.New("shipping resource not found")
	ErrExpired     = errors.New("shipping quote expired")
	ErrUnavailable = errors.New("shipping provider unavailable")
	ErrOrderLocked = errors.New("order cannot be changed")
)

type Provider interface {
	Calculate(context.Context, domain.QuoteRequest) ([]domain.ProviderQuote, error)
}

type Repository interface {
	LoadRequest(context.Context, string, string, string) (domain.QuoteRequest, error)
	Cached(context.Context, string, string, time.Time) ([]domain.Quote, error)
	Save(context.Context, domain.QuoteRequest, string, []domain.ProviderQuote, time.Time) ([]domain.Quote, error)
	Select(context.Context, string, string, string, string, time.Time) error
	SavePackages(context.Context, string, string, []domain.Package, string) error
}

type Service struct {
	repo     Repository
	provider Provider
	origin   string
	ttl      time.Duration
	now      func() time.Time
}

func New(repo Repository, provider Provider, origin string, ttl time.Duration) *Service {
	return &Service{repo: repo, provider: provider, origin: origin, ttl: ttl, now: time.Now}
}

func (s *Service) Quote(ctx context.Context, userID, reference string) ([]domain.Quote, error) {
	origin, e := domain.NormalizePostalCode(s.origin)
	if e != nil {
		return nil, ErrUnavailable
	}
	req, e := s.repo.LoadRequest(ctx, userID, reference, origin)
	if e != nil {
		return nil, e
	}
	if len(req.Packages) == 0 {
		return nil, ErrInvalid
	}
	for _, p := range req.Packages {
		if p.Validate() != nil {
			return nil, ErrInvalid
		}
	}
	b, _ := json.Marshal(req)
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	if cached, e := s.repo.Cached(ctx, req.OrderID, hash, s.now()); e == nil && len(cached) > 0 {
		return cached, nil
	}
	quotes, e := s.provider.Calculate(ctx, req)
	if e != nil {
		return nil, ErrUnavailable
	}
	if len(quotes) == 0 {
		return []domain.Quote{}, nil
	}
	return s.repo.Save(ctx, req, hash, quotes, s.now().Add(s.ttl))
}

func (s *Service) Select(ctx context.Context, userID, sessionID, reference, quoteID string) error {
	if quoteID == "" {
		return ErrInvalid
	}
	return s.repo.Select(ctx, userID, sessionID, reference, quoteID, s.now())
}

func (s *Service) SavePackages(ctx context.Context, adminID, reference string, packages []domain.Package, sessionID string) error {
	if len(packages) < 1 || len(packages) > 20 {
		return ErrInvalid
	}
	for _, p := range packages {
		if p.Validate() != nil {
			return ErrInvalid
		}
	}
	return s.repo.SavePackages(ctx, adminID, reference, packages, sessionID)
}
