package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	customerapp "example.com/loja-3d/backend/internal/application/customer"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuthenticationTokenAndEmailAreAtomic(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		t.Fatal("requires *_test database")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	cipher, err := mail.NewOutboxCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)), 1)
	if err != nil {
		t.Fatal(err)
	}
	outbox := mail.NewOutbox(pool, cipher)
	identityRepo := NewIdentityRepository(pool)
	identityRepo.SetEmailOutbox(outbox)
	customerRepo := NewCustomerRepository(pool)
	customerRepo.SetEmailOutbox(outbox)
	email := fmt.Sprintf("atomic-%d@example.test", time.Now().UnixNano())
	failedEmail := "failed-" + email
	recipientSum := sha256.Sum256([]byte(strings.ToLower(email)))
	recipientHash := hex.EncodeToString(recipientSum[:])
	tokenHash := func(value string) string {
		sum := sha256.Sum256([]byte(email + ":" + value))
		return hex.EncodeToString(sum[:])
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM email_outbox WHERE recipient_hash=$1`, recipientHash)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_magic_links WHERE lower(email)=lower($1)`, email)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE lower(email)=lower($1)`, email)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE lower(email)=lower($1)`, failedEmail)
	})
	if _, err = identityRepo.RegisterCustomerWithEmail(ctx, "Rollback Test", failedEmail, strings.Repeat("x", 60), tokenHash("failed"), time.Now().Add(time.Hour), identityapp.EmailIntent{To: "invalid", Kind: "verify_email", Link: "https://example.test/verify?token=failed"}); err == nil {
		t.Fatal("expected registration enqueue failure")
	}
	var failedUsers int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(email)=lower($1)`, failedEmail).Scan(&failedUsers); err != nil || failedUsers != 0 {
		t.Fatalf("registration rollback users=%d err=%v", failedUsers, err)
	}
	firstHash := tokenHash("first")
	identity, err := identityRepo.RegisterCustomerWithEmail(ctx, "Atomic Test", email, strings.Repeat("x", 60), firstHash, time.Now().Add(time.Hour), identityapp.EmailIntent{To: email, Kind: "verify_email", Link: "https://example.test/verify?token=one"})
	if err != nil {
		t.Fatal(err)
	}
	assertCounts := func(tokens, messages int) {
		t.Helper()
		var tokenCount, messageCount int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM customer_auth_tokens WHERE user_id=$1`, identity.ID).Scan(&tokenCount); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE recipient_hash=$1`, recipientHash).Scan(&messageCount); err != nil {
			t.Fatal(err)
		}
		if tokenCount != tokens || messageCount != messages {
			t.Fatalf("tokens=%d messages=%d", tokenCount, messageCount)
		}
	}
	assertCounts(1, 1)
	// Invalid enqueue rolls the token mutation back and keeps the previous token valid.
	err = identityRepo.CreateCustomerTokenWithEmail(ctx, identity.ID, "verify_email", tokenHash("second"), time.Now().Add(time.Hour), identityapp.EmailIntent{To: "invalid", Kind: "verify_email", Link: "https://example.test/verify?token=two"})
	if err == nil {
		t.Fatal("expected enqueue failure")
	}
	assertCounts(1, 1)
	var consumed bool
	if err = pool.QueryRow(ctx, `SELECT consumed_at IS NOT NULL FROM customer_auth_tokens WHERE token_hash=$1`, firstHash).Scan(&consumed); err != nil || consumed {
		t.Fatalf("previous token invalidated err=%v", err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			results <- identityRepo.CreateCustomerTokenWithEmail(ctx, identity.ID, "verify_email", tokenHash(fmt.Sprintf("concurrent-%d", i)), time.Now().Add(time.Hour), identityapp.EmailIntent{To: email, Kind: "verify_email", Link: fmt.Sprintf("https://example.test/verify?token=concurrent-%d", i)})
		}()
	}
	for i := 0; i < 2; i++ {
		if concurrentErr := <-results; concurrentErr != nil {
			t.Fatal(concurrentErr)
		}
	}
	var activeTokens, pendingMessages int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM customer_auth_tokens WHERE user_id=$1 AND purpose='verify_email' AND consumed_at IS NULL`, identity.ID).Scan(&activeTokens); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE recipient_hash=$1 AND message_type='verify_email' AND status='pending'`, recipientHash).Scan(&pendingMessages); err != nil {
		t.Fatal(err)
	}
	if activeTokens != 1 || pendingMessages != 1 {
		t.Fatalf("concurrent active_tokens=%d pending_messages=%d", activeTokens, pendingMessages)
	}
	magicOne := tokenHash("magic-one")
	if err = customerRepo.CreateMagicLinkWithEmail(ctx, email, magicOne, "127.0.0.1", time.Now().Add(time.Hour), customerapp.MagicLinkIntent{To: email, Link: "https://example.test/magic?token=one"}); err != nil {
		t.Fatal(err)
	}
	var links int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM customer_magic_links WHERE token_hash=$1`, magicOne).Scan(&links); err != nil || links != 1 {
		t.Fatalf("links=%d err=%v", links, err)
	}
	magicTwo := tokenHash("magic-two")
	err = customerRepo.CreateMagicLinkWithEmail(ctx, email, magicTwo, "127.0.0.1", time.Now().Add(time.Hour), customerapp.MagicLinkIntent{To: "invalid", Link: "https://example.test/magic?token=two"})
	if err == nil {
		t.Fatal("expected magic enqueue failure")
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM customer_magic_links WHERE token_hash=$1`, magicTwo).Scan(&links); err != nil || links != 0 {
		t.Fatalf("rolled back magic links=%d err=%v", links, err)
	}
}
