package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	domain "example.com/loja-3d/backend/internal/domain/admin"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
var ErrNotFound = errors.New("not found")
var ErrInvalid = errors.New("invalid")
var ErrConflict = errors.New("conflict")

type Repository interface {
	FindUserByEmail(context.Context, string) (domain.User, error)
	RecordLoginFailure(context.Context, string, string) error
	CreateSession(context.Context, string, string, string, string, string, time.Time) (domain.Session, error)
	GetSession(context.Context, string) (domain.Session, error)
	RevokeSession(context.Context, string) error
	Audit(context.Context, *string, *string, string, string, string, string, string) error
	Stats(context.Context) (domain.Stats, error)
	ListQuotes(context.Context, string, string, string, int, int) (domain.QuotePage, error)
	GetQuote(context.Context, string) (domain.QuoteDetail, error)
	StartAnalysis(context.Context, string, string) error
	SavePricing(context.Context, string, string, domain.Pricing) error
	SendProposal(context.Context, string, string) error
	ExpireProposals(context.Context, string) (int64, error)
	ListInternalUsers(context.Context) ([]domain.InternalUser, error)
	CreateInternalUser(context.Context, string, string, string, string, string) (domain.InternalUser, error)
	UpdateInternalUser(context.Context, string, string, string, string) (domain.InternalUser, error)
	ResetInternalUserPassword(context.Context, string, string, string) error
	RevokeInternalUserSessions(context.Context, string, string) error
}

func (s *Service) StartAnalysis(ctx context.Context, u domain.Session, ref, ip string) error {
	if u.Role != "admin" {
		return ErrForbidden
	}
	return s.repo.StartAnalysis(ctx, ref, u.UserID)
}
func (s *Service) SavePricing(ctx context.Context, u domain.Session, ref string, p domain.Pricing) error {
	if u.Role != "admin" {
		return ErrForbidden
	}
	return s.repo.SavePricing(ctx, ref, u.UserID, p)
}
func (s *Service) SendProposal(ctx context.Context, u domain.Session, ref string) error {
	if u.Role != "admin" {
		return ErrForbidden
	}
	return s.repo.SendProposal(ctx, ref, u.UserID)
}
func (s *Service) Expire(ctx context.Context, u domain.Session) (int64, error) {
	if u.Role != "admin" {
		return 0, ErrForbidden
	}
	return s.repo.ExpireProposals(ctx, u.UserID)
}

type Service struct {
	repo       Repository
	sessionTTL time.Duration
}

func New(repo Repository) *Service { return &Service{repo: repo, sessionTTL: 8 * time.Hour} }

var internalRoles = map[string]bool{"admin": true, "quote_viewer": true, "production_operator": true, "production_manager": true}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return ErrInvalid
	}
	return nil
}
func (s *Service) ListInternalUsers(ctx context.Context, session domain.Session) ([]domain.InternalUser, error) {
	if session.Role != "admin" {
		return nil, ErrForbidden
	}
	return s.repo.ListInternalUsers(ctx)
}
func (s *Service) CreateInternalUser(ctx context.Context, session domain.Session, email, name, password, role, ip string) (domain.InternalUser, error) {
	email, name, role = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(name), strings.TrimSpace(role)
	if session.Role != "admin" {
		return domain.InternalUser{}, ErrForbidden
	}
	if !strings.Contains(email, "@") || name == "" || !internalRoles[role] || validatePassword(password) != nil {
		return domain.InternalUser{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.InternalUser{}, err
	}
	return s.repo.CreateInternalUser(ctx, email, name, string(hash), role, session.UserID)
}
func (s *Service) UpdateInternalUser(ctx context.Context, session domain.Session, id, role, status, ip string) (domain.InternalUser, error) {
	if session.Role != "admin" {
		return domain.InternalUser{}, ErrForbidden
	}
	if !internalRoles[role] || (status != "active" && status != "blocked") || (id == session.UserID && (role != "admin" || status != "active")) {
		return domain.InternalUser{}, ErrInvalid
	}
	return s.repo.UpdateInternalUser(ctx, id, role, status, session.UserID)
}
func (s *Service) ResetInternalUserPassword(ctx context.Context, session domain.Session, id, password, ip string) error {
	if session.Role != "admin" {
		return ErrForbidden
	}
	if validatePassword(password) != nil {
		return ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.ResetInternalUserPassword(ctx, id, string(hash), session.UserID)
}
func (s *Service) RevokeInternalUserSessions(ctx context.Context, session domain.Session, id, ip string) error {
	if session.Role != "admin" {
		return ErrForbidden
	}
	return s.repo.RevokeInternalUserSessions(ctx, id, session.UserID)
}
func randomToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(b)
	return raw, Hash(raw), nil
}
func Hash(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func ConstantHashEqual(raw, expected string) bool {
	actual := Hash(raw)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
func (s *Service) Login(ctx context.Context, email, password, ip, ua string) (domain.Session, string, string, error) {
	user, err := s.repo.FindUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || user.Status != "active" || (user.LockedUntil != nil && user.LockedUntil.After(time.Now())) || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		_ = s.repo.RecordLoginFailure(ctx, strings.ToLower(strings.TrimSpace(email)), ip)
		return domain.Session{}, "", "", ErrUnauthorized
	}
	token, tokenHash, err := randomToken()
	if err != nil {
		return domain.Session{}, "", "", err
	}
	csrf, csrfHash, err := randomToken()
	if err != nil {
		return domain.Session{}, "", "", err
	}
	session, err := s.repo.CreateSession(ctx, user.ID, tokenHash, csrfHash, ip, ua, time.Now().Add(s.sessionTTL))
	if err != nil {
		return domain.Session{}, "", "", err
	}
	session.Email = user.Email
	session.FullName = user.FullName
	session.Role = user.Role
	_ = s.repo.Audit(ctx, &user.ID, &session.ID, "admin.login", "session", session.ID, "success", ip)
	return session, token, csrf, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, ErrUnauthorized
	}
	return s.repo.GetSession(ctx, Hash(token))
}
func (s *Service) Logout(ctx context.Context, session domain.Session, ip string) error {
	err := s.repo.RevokeSession(ctx, session.ID)
	outcome := "success"
	if err != nil {
		outcome = "failure"
	}
	_ = s.repo.Audit(ctx, &session.UserID, &session.ID, "admin.logout", "session", session.ID, outcome, ip)
	return err
}
func (s *Service) Audit(ctx context.Context, session domain.Session, action, resourceType, resourceID, result, ip string) error {
	return s.repo.Audit(ctx, &session.UserID, &session.ID, action, resourceType, resourceID, result, ip)
}
func (s *Service) Stats(ctx context.Context, session domain.Session, ip string) (domain.Stats, error) {
	v, e := s.repo.Stats(ctx)
	_ = s.repo.Audit(ctx, &session.UserID, &session.ID, "quotes.stats", "quote", "", outcome(e), ip)
	return v, e
}
func (s *Service) List(ctx context.Context, session domain.Session, search, status, sort string, page, limit int, ip string) (domain.QuotePage, error) {
	if page < 1 || limit < 1 || limit > 100 {
		return domain.QuotePage{}, ErrInvalid
	}
	switch sort {
	case "newest", "oldest", "reference":
	default:
		return domain.QuotePage{}, ErrInvalid
	}
	v, e := s.repo.ListQuotes(ctx, search, status, sort, page, limit)
	_ = s.repo.Audit(ctx, &session.UserID, &session.ID, "quotes.list", "quote", "", outcome(e), ip)
	return v, e
}
func (s *Service) Get(ctx context.Context, session domain.Session, reference, ip string) (domain.QuoteDetail, error) {
	v, e := s.repo.GetQuote(ctx, reference)
	_ = s.repo.Audit(ctx, &session.UserID, &session.ID, "quotes.view", "quote", reference, outcome(e), ip)
	return v, e
}
func outcome(err error) string {
	if err != nil {
		return "failure"
	}
	return "success"
}
