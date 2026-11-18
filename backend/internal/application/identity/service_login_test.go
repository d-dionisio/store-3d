package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	security "example.com/loja-3d/backend/internal/security/auth"
)

type loginRepositoryStub struct {
	Repository
	identity    Identity
	factor      Factor
	factorErr   error
	upsertCalls int
	purpose     string
}

func (r *loginRepositoryStub) FindIdentity(_ context.Context, _, _ string) (Identity, error) {
	return r.identity, nil
}
func (r *loginRepositoryStub) AllowRateLimit(context.Context, string, string, int, time.Duration) (bool, error) {
	return true, nil
}
func (r *loginRepositoryStub) GetFactor(context.Context, string, string) (Factor, error) {
	return r.factor, r.factorErr
}
func (r *loginRepositoryStub) UpsertPendingFactor(context.Context, string, string, []byte, int16) (Factor, error) {
	r.upsertCalls++
	return r.factor, nil
}
func (r *loginRepositoryStub) CreateChallenge(_ context.Context, _, _, purpose, _, _, _ string, _ time.Time) error {
	r.purpose = purpose
	return nil
}

func TestAdminLoginFailsClosedWhenMFAFactorLookupFails(t *testing.T) {
	passwordHash, err := security.HashPassword("Correct-Horse-Battery-2026!")
	if err != nil {
		t.Fatal(err)
	}
	repository := &loginRepositoryStub{
		identity:  Identity{ID: "admin-id", Kind: "admin", Email: "admin@example.test", Status: "active", PasswordHash: passwordHash},
		factorErr: errors.New("database temporarily unavailable"),
	}
	service := New(repository, nil, nil, "")
	result, err := service.StartLogin(context.Background(), "admin", "admin@example.test", "Correct-Horse-Battery-2026!", "127.0.0.1", "test-agent")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("StartLogin error = %v, want ErrUnavailable", err)
	}
	if result.Session != nil || result.SessionToken != "" || result.ChallengeToken != "" {
		t.Fatal("factor lookup failure must not create a session or challenge")
	}
	if repository.upsertCalls != 0 {
		t.Fatalf("pending factor upserts = %d, want 0", repository.upsertCalls)
	}
}

func TestAdminPasswordLoginRequiresTOTPAndDoesNotUseEmail(t *testing.T) {
	passwordHash, err := security.HashPassword("Correct-Horse-Battery-2026!")
	if err != nil {
		t.Fatal(err)
	}
	repository := &loginRepositoryStub{
		identity: Identity{ID: "admin-id", Kind: "admin", Email: "admin@example.test", Status: "active", PasswordHash: passwordHash},
		factor:   Factor{ID: "factor-id", IdentityID: "admin-id", Kind: "admin", Enabled: true},
	}
	// A nil mailer is deliberate: the password/TOTP login path must not need it.
	service := New(repository, nil, nil, "")
	result, err := service.StartLogin(context.Background(), "admin", "admin@example.test", "Correct-Horse-Battery-2026!", "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "mfa_required" || result.ChallengeToken == "" || result.Session != nil || result.SessionToken != "" {
		t.Fatalf("first factor must return only an MFA challenge, got action=%q session=%v", result.Action, result.Session != nil)
	}
	if repository.purpose != "admin_mfa_login" {
		t.Fatalf("challenge purpose = %q, want admin_mfa_login", repository.purpose)
	}
}
