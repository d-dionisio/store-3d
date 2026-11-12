package postgres

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	security "example.com/loja-3d/backend/internal/security/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIdentityRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		t.Fatal("identity integration test requires a dedicated *_test database")
	}
	pool, e := pgxpool.NewWithConfig(context.Background(), config)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewIdentityRepository(pool)
	nonce := time.Now().Format("20060102150405.000000000")
	email := "identity-" + nonce + "@example.com"
	tokenHash := func(value string) string { return security.TokenHash(nonce + "-" + value) }
	passwordHash, e := security.HashPassword("uma passphrase de integracao segura")
	if e != nil {
		t.Fatal(e)
	}
	identity, e := repo.RegisterCustomer(ctx, "Cliente Teste", email, passwordHash, tokenHash("verify-token"), time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM security_events WHERE user_id=$1`, identity.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, identity.ID)
	})
	if e = repo.ConsumeEmailVerification(ctx, tokenHash("verify-token")); e != nil {
		t.Fatal(e)
	}
	found, e := repo.FindIdentity(ctx, "customer", email)
	if e != nil || !found.EmailVerified {
		t.Fatalf("verified=%v err=%v", found.EmailVerified, e)
	}
	factor, e := repo.UpsertPendingFactor(ctx, "customer", identity.ID, []byte(strings.Repeat("x", 40)), 1)
	if e != nil {
		t.Fatal(e)
	}
	challengeHash := tokenHash("setup-challenge")
	if e = repo.CreateChallenge(ctx, "customer", identity.ID, "customer_mfa_setup", challengeHash, "127.0.0.1", "test", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	session, e := repo.EnableFactorAndSession(ctx, challengeHash, factor.ID, 123, []string{tokenHash("RECOVERY"), tokenHash("CONCURRENT")}, tokenHash("session"), tokenHash("csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour))
	if e != nil || session.ID == "" {
		t.Fatalf("enable err=%v", e)
	}
	if _, e = repo.EnableFactorAndSession(ctx, challengeHash, factor.ID, 124, nil, tokenHash("other"), tokenHash("other-csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour)); e == nil {
		t.Fatal("reused setup challenge accepted")
	}
	loginHash := tokenHash("login-challenge")
	if e = repo.CreateChallenge(ctx, "customer", identity.ID, "customer_mfa_login", loginHash, "127.0.0.1", "test", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.CompleteMFAAndSession(ctx, loginHash, nil, tokenHash("RECOVERY"), tokenHash("session-2"), tokenHash("csrf-2"), "127.0.0.1", "test", time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	secondHash := tokenHash("login-challenge-2")
	_ = repo.CreateChallenge(ctx, "customer", identity.ID, "customer_mfa_login", secondHash, "127.0.0.1", "test", time.Now().Add(time.Minute))
	if _, e = repo.CompleteMFAAndSession(ctx, secondHash, nil, tokenHash("RECOVERY"), tokenHash("session-3"), tokenHash("csrf-3"), "127.0.0.1", "test", time.Now().Add(time.Hour)); e == nil {
		t.Fatal("reused recovery code accepted")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			raw := "concurrent-challenge-" + string(rune('a'+index))
			hash := tokenHash(raw)
			if repo.CreateChallenge(ctx, "customer", identity.ID, "customer_mfa_login", hash, "127.0.0.1", "test", time.Now().Add(time.Minute)) != nil {
				return
			}
			if _, err := repo.CompleteMFAAndSession(ctx, hash, nil, tokenHash("CONCURRENT"), tokenHash(raw+"-session"), tokenHash(raw+"-csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour)); err == nil {
				successes.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent recovery successes=%d", successes.Load())
	}
}
