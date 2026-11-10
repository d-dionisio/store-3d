package customer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/customer"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrNotFound = errors.New("not found")
var ErrInvalidDecision = errors.New("invalid decision")
var ErrRateLimited = errors.New("rate limited")
var ErrForbidden = errors.New("forbidden")
var ErrInvalidConversion = errors.New("invalid quote conversion")

type Repository interface {
	CreateMagicLink(context.Context, string, string, string, time.Time) error
	ConsumeMagicLink(context.Context, string, string, string, string, string, time.Time) (domain.Session, error)
	GetSession(context.Context, string) (domain.Session, error)
	RevokeSession(context.Context, string) error
	Profile(context.Context, string) (domain.Profile, error)
	ListQuotes(context.Context, string) ([]domain.QuoteSummary, error)
	GetQuote(context.Context, string, string) (domain.QuoteDetail, error)
	Decide(context.Context, string, string, string, string, string) error
	ConvertQuote(context.Context, string, string, string) (domain.OrderResult, error)
}
type Mailer interface {
	SendMagicLink(context.Context, string, string) error
	SendDecision(context.Context, string, string, string) error
}
type Service struct {
	repo      Repository
	mailer    Mailer
	publicURL string
	now       func() time.Time
}

func New(repo Repository, mailer Mailer, publicURL string) *Service {
	return &Service{repo: repo, mailer: mailer, publicURL: strings.TrimRight(publicURL, "/"), now: time.Now}
}
func token() (string, string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", "", e
	}
	raw := hex.EncodeToString(b)
	return raw, Hash(raw), nil
}
func Hash(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func CSRF(raw, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(Hash(raw)), []byte(expected)) == 1
}
func (s *Service) RequestMagicLink(ctx context.Context, email, ip string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return nil
	}
	raw, hash, e := token()
	if e != nil {
		return e
	}
	if e = s.repo.CreateMagicLink(ctx, email, hash, ip, s.now().Add(15*time.Minute)); e != nil {
		return e
	}
	return s.mailer.SendMagicLink(ctx, email, s.publicURL+"/minha-conta/confirmar?token="+raw)
}
func (s *Service) VerifyMagicLink(ctx context.Context, raw, ip, ua string) (domain.Session, string, string, error) {
	sessionRaw, sessionHash, e := token()
	if e != nil {
		return domain.Session{}, "", "", e
	}
	csrfRaw, csrfHash, e := token()
	if e != nil {
		return domain.Session{}, "", "", e
	}
	v, e := s.repo.ConsumeMagicLink(ctx, Hash(raw), sessionHash, csrfHash, ip, ua, s.now().Add(30*24*time.Hour))
	return v, sessionRaw, csrfRaw, e
}
func (s *Service) Authenticate(ctx context.Context, raw string) (domain.Session, error) {
	if raw == "" {
		return domain.Session{}, ErrUnauthorized
	}
	return s.repo.GetSession(ctx, Hash(raw))
}
func (s *Service) Profile(ctx context.Context, v domain.Session) (domain.Profile, error) {
	return s.repo.Profile(ctx, v.UserID)
}
func (s *Service) List(ctx context.Context, v domain.Session) ([]domain.QuoteSummary, error) {
	return s.repo.ListQuotes(ctx, v.UserID)
}
func (s *Service) Get(ctx context.Context, v domain.Session, ref string) (domain.QuoteDetail, error) {
	return s.repo.GetQuote(ctx, v.UserID, ref)
}
func (s *Service) Decide(ctx context.Context, v domain.Session, ref, proposalID, decision string) error {
	if decision != "accepted" && decision != "rejected" {
		return ErrInvalidDecision
	}
	e := s.repo.Decide(ctx, v.UserID, ref, proposalID, decision, v.ID)
	if e == nil {
		_ = s.mailer.SendDecision(ctx, v.Email, ref, decision)
	}
	return e
}
func (s *Service) Logout(ctx context.Context, v domain.Session) error {
	return s.repo.RevokeSession(ctx, v.ID)
}
func (s *Service) ConvertQuote(ctx context.Context, v domain.Session, ref string) (domain.OrderResult, error) {
	return s.repo.ConvertQuote(ctx, v.UserID, ref, v.ID)
}
