package identity

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	security "example.com/loja-3d/backend/internal/security/auth"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrInvalid = errors.New("invalid")
var ErrConflict = errors.New("conflict")
var ErrExpired = errors.New("expired")
var ErrRateLimited = errors.New("rate limited")
var ErrUnavailable = errors.New("unavailable")

type Identity struct {
	ID, Kind, Email, FullName, Role, Status, PasswordHash string
	EmailVerified                                         bool
	LockedUntil                                           *time.Time
}
type Factor struct {
	ID, IdentityID, Kind string
	Ciphertext           []byte
	KeyVersion           int16
	Enabled              bool
	LastCounter          *int64
}
type Challenge struct {
	ID, IdentityID, Kind, Purpose string
	Attempts                      int
	ExpiresAt                     time.Time
	Consumed                      bool
}
type Session struct {
	ID, IdentityID, Kind, Email, FullName, Role, CSRFHash string
	ExpiresAt                                             time.Time
}
type LoginResult struct {
	ChallengeToken, Action, SetupURI, SetupSecret, SessionToken, CSRFToken string
	Session                                                                *Session
}
type CompleteResult struct {
	Session                 Session
	SessionToken, CSRFToken string
	RecoveryCodes           []string
}

type Repository interface {
	FindIdentity(context.Context, string, string) (Identity, error)
	FindIdentityByID(context.Context, string, string) (Identity, error)
	RegisterCustomer(context.Context, string, string, string, string, time.Time) (Identity, error)
	CreateCustomerToken(context.Context, string, string, string, time.Time) error
	ConsumeEmailVerification(context.Context, string) error
	ResetPassword(context.Context, string, string) error
	UpdatePassword(context.Context, string, string, string) error
	RecordFailure(context.Context, string, string, string) error
	UpsertPendingFactor(context.Context, string, string, []byte, int16) (Factor, error)
	GetFactor(context.Context, string, string) (Factor, error)
	CreateChallenge(context.Context, string, string, string, string, string, string, time.Time) error
	GetChallenge(context.Context, string) (Challenge, error)
	EnableFactorAndSession(context.Context, string, string, int64, []string, string, string, string, string, time.Time) (Session, error)
	CompleteMFAAndSession(context.Context, string, *int64, string, string, string, string, string, time.Time) (Session, error)
	CreateSession(context.Context, string, string, string, string, string, string, time.Time) (Session, error)
	MFAStatus(context.Context, string, string) (bool, int, error)
	DisableMFA(context.Context, string, string) error
	RevokeSessions(context.Context, string, string) error
	RegenerateRecoveryCodes(context.Context, string, string, []string) error
	SecurityEvent(context.Context, string, *string, string, string, string) error
	CreateReauthEvidence(context.Context, string, string, string, *int64, string, string, time.Time) error
	ConsumeReauthEvidence(context.Context, string, string, string) error
	AllowRateLimit(context.Context, string, string, int, time.Duration) (bool, error)
}

func (s *Service) allow(ctx context.Context, scope, subject string, limit int, window time.Duration) bool {
	ok, err := s.repo.AllowRateLimit(ctx, scope, security.TokenHash(subject), limit, window)
	return err == nil && ok
}

type Mailer interface {
	SendVerification(context.Context, string, string) error
	SendPasswordReset(context.Context, string, string) error
}
type EmailIntent struct {
	To, Kind, Link string
}
type atomicEmailRepository interface {
	RegisterCustomerWithEmail(context.Context, string, string, string, string, time.Time, EmailIntent) (Identity, error)
	CreateCustomerTokenWithEmail(context.Context, string, string, string, time.Time, EmailIntent) error
}
type securityMailer interface {
	SendSecurityEvent(context.Context, string, string) error
}
type Service struct {
	repo            Repository
	mailer          Mailer
	cipher          *security.Cipher
	publicURL       string
	now             func() time.Time
	reauthTTL       time.Duration
	passwordChecker security.PasswordChecker
}

func New(repo Repository, mailer Mailer, cipher *security.Cipher, publicURL string) *Service {
	return &Service{repo: repo, mailer: mailer, cipher: cipher, publicURL: strings.TrimRight(publicURL, "/"), now: time.Now, reauthTTL: 5 * time.Minute, passwordChecker: security.NoopPasswordChecker{}}
}
func (s *Service) SetReauthenticationTTL(value time.Duration) {
	if value > 0 && value <= 15*time.Minute {
		s.reauthTTL = value
	}
}
func (s *Service) SetPasswordChecker(value security.PasswordChecker) {
	if value != nil {
		s.passwordChecker = value
	}
}
func (s *Service) acceptablePassword(ctx context.Context, password string) bool {
	compromised, err := s.passwordChecker.Compromised(ctx, password)
	return err == nil && !compromised
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, e := mail.ParseAddress(value)
	if e != nil || parsed.Address != value || len(value) > 254 {
		return "", ErrInvalid
	}
	return value, nil
}
func (s *Service) Register(ctx context.Context, name, email, password, ip string) error {
	if !s.allow(ctx, "register", ip, 5, time.Hour) {
		return ErrRateLimited
	}
	name = strings.TrimSpace(name)
	email, e := normalizeEmail(email)
	if e != nil || name == "" || len([]rune(name)) > 150 || security.ValidatePassword(password) != nil || !s.acceptablePassword(ctx, password) {
		return ErrInvalid
	}
	hash, e := security.HashPassword(password)
	if e != nil {
		return e
	}
	raw, tokenHash, e := security.Token()
	if e != nil {
		return e
	}
	expires := s.now().Add(30 * time.Minute)
	link := s.publicURL + "/verificar-email?token=" + raw
	var identity Identity
	if atomic, ok := s.repo.(atomicEmailRepository); ok {
		identity, e = atomic.RegisterCustomerWithEmail(ctx, name, email, hash, tokenHash, expires, EmailIntent{To: email, Kind: "verify_email", Link: link})
	} else {
		identity, e = s.repo.RegisterCustomer(ctx, name, email, hash, tokenHash, expires)
	}
	if e != nil {
		return e
	}
	_ = s.repo.SecurityEvent(ctx, "customer", &identity.ID, "customer.register", "success", ip)
	if _, ok := s.repo.(atomicEmailRepository); !ok {
		e = s.mailer.SendVerification(ctx, email, link)
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) VerifyEmail(ctx context.Context, raw, ip string) error {
	if !s.allow(ctx, "email_verification", ip, 10, time.Hour) {
		return ErrRateLimited
	}
	if len(raw) < 32 {
		return ErrInvalid
	}
	e := s.repo.ConsumeEmailVerification(ctx, security.TokenHash(raw))
	outcome := "success"
	if e != nil {
		outcome = "denied"
	}
	_ = s.repo.SecurityEvent(ctx, "anonymous", nil, "customer.email.verify", outcome, ip)
	return e
}
func (s *Service) ResendVerification(ctx context.Context, email, ip string) error {
	if !s.allow(ctx, "email_verification", ip, 5, time.Hour) {
		return ErrRateLimited
	}
	email, e := normalizeEmail(email)
	if e != nil {
		return nil
	}
	identity, e := s.repo.FindIdentity(ctx, "customer", email)
	if e != nil || identity.EmailVerified {
		return nil
	}
	raw, hash, e := security.Token()
	if e != nil {
		return e
	}
	expires := s.now().Add(30 * time.Minute)
	link := s.publicURL + "/verificar-email?token=" + raw
	if atomic, ok := s.repo.(atomicEmailRepository); ok {
		e = atomic.CreateCustomerTokenWithEmail(ctx, identity.ID, "verify_email", hash, expires, EmailIntent{To: email, Kind: "verify_email", Link: link})
	} else {
		e = s.repo.CreateCustomerToken(ctx, identity.ID, "verify_email", hash, expires)
	}
	if e != nil {
		return e
	}
	if _, ok := s.repo.(atomicEmailRepository); !ok {
		e = s.mailer.SendVerification(ctx, email, link)
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) RequestReset(ctx context.Context, email, ip string) error {
	if !s.allow(ctx, "password_reset", ip, 5, time.Hour) {
		return ErrRateLimited
	}
	email, e := normalizeEmail(email)
	if e != nil {
		return nil
	}
	identity, e := s.repo.FindIdentity(ctx, "customer", email)
	if e != nil || !identity.EmailVerified || identity.PasswordHash == "" {
		return nil
	}
	raw, hash, e := security.Token()
	if e != nil {
		return e
	}
	expires := s.now().Add(20 * time.Minute)
	link := s.publicURL + "/redefinir-senha?token=" + raw
	if atomic, ok := s.repo.(atomicEmailRepository); ok {
		e = atomic.CreateCustomerTokenWithEmail(ctx, identity.ID, "reset_password", hash, expires, EmailIntent{To: email, Kind: "password_reset", Link: link})
	} else {
		e = s.repo.CreateCustomerToken(ctx, identity.ID, "reset_password", hash, expires)
	}
	if e != nil {
		return e
	}
	if _, ok := s.repo.(atomicEmailRepository); !ok {
		e = s.mailer.SendPasswordReset(ctx, email, link)
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) ResetPassword(ctx context.Context, raw, password, ip string) error {
	if security.ValidatePassword(password) != nil || len(raw) < 32 || !s.acceptablePassword(ctx, password) {
		return ErrInvalid
	}
	hash, e := security.HashPassword(password)
	if e != nil {
		return e
	}
	e = s.repo.ResetPassword(ctx, security.TokenHash(raw), hash)
	outcome := "success"
	if e != nil {
		outcome = "denied"
	}
	_ = s.repo.SecurityEvent(ctx, "anonymous", nil, "customer.password.reset", outcome, ip)
	return e
}

func (s *Service) StartLogin(ctx context.Context, kind, email, password, ip, ua string) (LoginResult, error) {
	if !s.allow(ctx, "login", kind+"|"+ip, 10, time.Minute) {
		return LoginResult{}, ErrRateLimited
	}
	email, e := normalizeEmail(email)
	if e != nil {
		return LoginResult{}, ErrUnauthorized
	}
	identity, e := s.repo.FindIdentity(ctx, kind, email)
	valid := e == nil && identity.Status == "active" && (identity.LockedUntil == nil || identity.LockedUntil.Before(s.now()))
	passwordOK, needsRehash := security.VerifyPassword(identity.PasswordHash, password)
	if !passwordOK {
		valid = false
	}
	if kind == "customer" && !identity.EmailVerified {
		valid = false
	}
	if !valid {
		_ = s.repo.RecordFailure(ctx, kind, email, ip)
		return LoginResult{}, ErrUnauthorized
	}
	if needsRehash {
		if upgraded, hashErr := security.HashPassword(password); hashErr == nil {
			_ = s.repo.UpdatePassword(ctx, kind, identity.ID, upgraded)
		}
	}
	factor, e := s.repo.GetFactor(ctx, kind, identity.ID)
	if e != nil || !factor.Enabled {
		if kind == "customer" {
			return s.passwordSession(ctx, identity, ip, ua)
		}
		secret, e := security.NewTOTPSecret()
		if e != nil {
			return LoginResult{}, e
		}
		encrypted, e := s.cipher.Encrypt([]byte(secret))
		if e != nil {
			return LoginResult{}, e
		}
		if _, e = s.repo.UpsertPendingFactor(ctx, kind, identity.ID, encrypted, s.cipher.Version); e != nil {
			return LoginResult{}, e
		}
		raw, hash, e := security.Token()
		if e != nil {
			return LoginResult{}, e
		}
		if e = s.repo.CreateChallenge(ctx, kind, identity.ID, "admin_mfa_setup", hash, ip, ua, s.now().Add(10*time.Minute)); e != nil {
			return LoginResult{}, e
		}
		return LoginResult{ChallengeToken: raw, Action: "mfa_setup_required", SetupSecret: secret, SetupURI: security.TOTPUri("Loja 3D Admin", identity.Email, secret)}, nil
	}
	raw, hash, e := security.Token()
	if e != nil {
		return LoginResult{}, e
	}
	purpose := kind + "_mfa_login"
	if e = s.repo.CreateChallenge(ctx, kind, identity.ID, purpose, hash, ip, ua, s.now().Add(5*time.Minute)); e != nil {
		return LoginResult{}, e
	}
	return LoginResult{ChallengeToken: raw, Action: "mfa_required"}, nil
}
func (s *Service) passwordSession(ctx context.Context, identity Identity, ip, ua string) (LoginResult, error) {
	sessionRaw, sessionHash, e := security.Token()
	if e != nil {
		return LoginResult{}, e
	}
	csrfRaw, csrfHash, e := security.Token()
	if e != nil {
		return LoginResult{}, e
	}
	session, e := s.repo.CreateSession(ctx, identity.Kind, identity.ID, sessionHash, csrfHash, ip, ua, s.now().Add(30*24*time.Hour))
	if e != nil {
		return LoginResult{}, e
	}
	return LoginResult{Action: "authenticated", SessionToken: sessionRaw, CSRFToken: csrfRaw, Session: &session}, nil
}
func (s *Service) CompleteMFA(ctx context.Context, raw, code, recovery, ip, ua string) (CompleteResult, error) {
	if !s.allow(ctx, "mfa", ip, 10, time.Minute) {
		return CompleteResult{}, ErrRateLimited
	}
	challenge, e := s.repo.GetChallenge(ctx, security.TokenHash(raw))
	if e != nil || challenge.Consumed || challenge.ExpiresAt.Before(s.now()) || challenge.Attempts >= 10 {
		return CompleteResult{}, ErrUnauthorized
	}
	factor, e := s.repo.GetFactor(ctx, challenge.Kind, challenge.IdentityID)
	if e != nil {
		return CompleteResult{}, ErrUnauthorized
	}
	secretBytes, e := s.cipher.Decrypt(factor.Ciphertext, factor.KeyVersion)
	if e != nil {
		return CompleteResult{}, ErrUnauthorized
	}
	var counter *int64
	if recovery == "" {
		value, ok := security.ValidateTOTP(string(secretBytes), code, s.now(), factor.LastCounter)
		if !ok {
			return CompleteResult{}, ErrUnauthorized
		}
		counter = &value
	}
	sessionRaw, sessionHash, e := security.Token()
	if e != nil {
		return CompleteResult{}, e
	}
	csrfRaw, csrfHash, e := security.Token()
	if e != nil {
		return CompleteResult{}, e
	}
	expires := s.now().Add(8 * time.Hour)
	if challenge.Kind == "customer" {
		expires = s.now().Add(30 * 24 * time.Hour)
	}
	var session Session
	var codes []string
	if strings.HasSuffix(challenge.Purpose, "mfa_setup") {
		if counter == nil {
			return CompleteResult{}, ErrInvalid
		}
		var hashes []string
		codes, hashes, e = security.RecoveryCodes(10)
		if e != nil {
			return CompleteResult{}, e
		}
		session, e = s.repo.EnableFactorAndSession(ctx, security.TokenHash(raw), factor.ID, *counter, hashes, sessionHash, csrfHash, ip, ua, expires)
	} else {
		recoveryHash := ""
		if recovery != "" {
			recoveryHash = security.TokenHash(strings.ToUpper(strings.TrimSpace(recovery)))
		}
		session, e = s.repo.CompleteMFAAndSession(ctx, security.TokenHash(raw), counter, recoveryHash, sessionHash, csrfHash, ip, ua, expires)
	}
	if e != nil {
		return CompleteResult{}, ErrUnauthorized
	}
	if len(codes) > 0 {
		if notifier, ok := s.mailer.(securityMailer); ok {
			_ = notifier.SendSecurityEvent(ctx, session.Email, "mfa_changed")
		}
	}
	return CompleteResult{Session: session, SessionToken: sessionRaw, CSRFToken: csrfRaw, RecoveryCodes: codes}, nil
}
func (s *Service) Status(ctx context.Context, kind, id string) (bool, int, error) {
	return s.repo.MFAStatus(ctx, kind, id)
}
func (s *Service) BeginSetup(ctx context.Context, kind, id, email, ip, ua string) (LoginResult, error) {
	secret, e := security.NewTOTPSecret()
	if e != nil {
		return LoginResult{}, e
	}
	encrypted, e := s.cipher.Encrypt([]byte(secret))
	if e != nil {
		return LoginResult{}, e
	}
	if _, e = s.repo.UpsertPendingFactor(ctx, kind, id, encrypted, s.cipher.Version); e != nil {
		return LoginResult{}, e
	}
	raw, hash, e := security.Token()
	if e != nil {
		return LoginResult{}, e
	}
	if e = s.repo.CreateChallenge(ctx, kind, id, kind+"_mfa_setup", hash, ip, ua, s.now().Add(10*time.Minute)); e != nil {
		return LoginResult{}, e
	}
	return LoginResult{Action: "mfa_setup_required", ChallengeToken: raw, SetupSecret: secret, SetupURI: security.TOTPUri("Loja 3D", email, secret)}, nil
}
func (s *Service) Disable(ctx context.Context, kind, id string) error {
	if kind == "admin" {
		return ErrInvalid
	}
	err := s.repo.DisableMFA(ctx, kind, id)
	if err == nil {
		_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".mfa.disable", "success", "")
		if notifier, ok := s.mailer.(securityMailer); ok {
			if identity, findErr := s.repo.FindIdentityByID(ctx, kind, id); findErr == nil {
				_ = notifier.SendSecurityEvent(ctx, identity.Email, "mfa_changed")
			}
		}
	}
	return err
}
func (s *Service) RevokeSessions(ctx context.Context, kind, id string) error {
	err := s.repo.RevokeSessions(ctx, kind, id)
	if err == nil {
		_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".sessions.revoke_all", "success", "")
	}
	return err
}
func (s *Service) SetPassword(ctx context.Context, kind, id, password string) error {
	if security.ValidatePassword(password) != nil || !s.acceptablePassword(ctx, password) {
		return ErrInvalid
	}
	hash, e := security.HashPassword(password)
	if e != nil {
		return e
	}
	if e = s.repo.UpdatePassword(ctx, kind, id, hash); e != nil {
		return e
	}
	if e = s.repo.RevokeSessions(ctx, kind, id); e != nil {
		return e
	}
	_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".password.change", "success", "")
	if notifier, ok := s.mailer.(securityMailer); ok {
		if identity, findErr := s.repo.FindIdentityByID(ctx, kind, id); findErr == nil {
			_ = notifier.SendSecurityEvent(ctx, identity.Email, "password_changed")
		}
	}
	return nil
}
func (s *Service) HasPassword(ctx context.Context, kind, id string) bool {
	identity, err := s.repo.FindIdentityByID(ctx, kind, id)
	return err == nil && identity.PasswordHash != ""
}
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, kind, id string) ([]string, error) {
	raw, hashes, e := security.RecoveryCodes(10)
	if e != nil {
		return nil, e
	}
	if e = s.repo.RegenerateRecoveryCodes(ctx, kind, id, hashes); e != nil {
		return nil, e
	}
	_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".recovery_codes.regenerate", "success", "")
	return raw, nil
}

func (s *Service) Reauthenticate(ctx context.Context, kind, id, sessionID, password, code, recovery, ip string) (string, error) {
	identity, err := s.repo.FindIdentityByID(ctx, kind, id)
	if err != nil {
		return "", ErrUnauthorized
	}
	ok, _ := security.VerifyPassword(identity.PasswordHash, password)
	if !ok {
		_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".reauth", "denied", ip)
		return "", ErrUnauthorized
	}
	factor, factorErr := s.repo.GetFactor(ctx, kind, id)
	var counter *int64
	recoveryHash := ""
	if factorErr == nil && factor.Enabled {
		secret, decryptErr := s.cipher.Decrypt(factor.Ciphertext, factor.KeyVersion)
		if decryptErr != nil {
			return "", ErrUnauthorized
		}
		if recovery != "" {
			recoveryHash = security.TokenHash(strings.ToUpper(strings.TrimSpace(recovery)))
		} else {
			value, valid := security.ValidateTOTP(string(secret), code, s.now(), factor.LastCounter)
			if !valid {
				return "", ErrUnauthorized
			}
			counter = &value
		}
	} else if kind == "admin" {
		return "", ErrUnauthorized
	}
	raw, hash, err := security.Token()
	if err != nil {
		return "", err
	}
	if err = s.repo.CreateReauthEvidence(ctx, kind, id, sessionID, counter, recoveryHash, hash, s.now().Add(s.reauthTTL)); err != nil {
		return "", ErrUnauthorized
	}
	_ = s.repo.SecurityEvent(ctx, kind, &id, kind+".reauth", "success", ip)
	return raw, nil
}

func (s *Service) ConsumeReauthentication(ctx context.Context, kind, sessionID, raw, ip string) error {
	if len(raw) < 32 {
		return ErrUnauthorized
	}
	err := s.repo.ConsumeReauthEvidence(ctx, kind, sessionID, security.TokenHash(raw))
	if err != nil {
		return ErrUnauthorized
	}
	return nil
}
