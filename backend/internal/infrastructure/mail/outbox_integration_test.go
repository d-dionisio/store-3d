package mail

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5"
)

func TestOutboxIntegration(t *testing.T) {
	if os.Getenv(testpostgres.EnvDatabaseURL) == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	controlPool, e := testpostgres.Open(ctx)
	if e != nil {
		t.Fatalf("open validated integration database: %v", e)
	}
	t.Cleanup(func() { controlPool.Close() })

	fixtureID, e := testpostgres.NewFixtureID()
	if e != nil {
		t.Fatal("could not generate isolated outbox schema name")
	}
	schema := "it_email_worker_test_" + strings.ReplaceAll(fixtureID, "-", "")
	schemaIdent := pgx.Identifier{schema}.Sanitize()
	if _, e = controlPool.Exec(ctx, "CREATE SCHEMA "+schemaIdent); e != nil {
		t.Fatal("could not create isolated email-worker test schema")
	}
	if e = testpostgres.RegisterIDCleanup(t, schema, func(cleanupCtx context.Context, id string) error {
		_, cleanupErr := controlPool.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{id}.Sanitize()+" CASCADE")
		return cleanupErr
	}); e != nil {
		t.Fatal("could not register isolated outbox schema cleanup")
	}

	pool, e := testpostgres.OpenSchema(ctx, schema)
	if e != nil {
		t.Fatalf("open schema-scoped integration pool: %v", e)
	}
	t.Cleanup(func() { pool.Close() })

	// Apply the complete official outbox migration to this test-owned schema.
	// No public tables are cloned and the shared/public schema is never written.
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate the official outbox migration")
	}
	migrationPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "migrations", "000025_create_transactional_email_outbox.up.sql")
	migrationSQL, e := os.ReadFile(migrationPath)
	if e != nil {
		t.Fatal("could not read the official outbox migration")
	}
	migrationConn, e := pool.Acquire(ctx)
	if e != nil {
		t.Fatal("could not acquire schema-scoped migration connection")
	}
	_, e = migrationConn.Conn().PgConn().Exec(ctx, string(migrationSQL)).ReadAll()
	migrationConn.Release()
	if e != nil {
		t.Fatal("could not apply the official outbox migration to isolated schema")
	}
	cipher, _ := NewOutboxCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)), 1)
	outbox := NewOutbox(pool, cipher)
	// Each fixture is uniquely identified and cleanup never touches shared rows.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	recipient := "fixture-" + suffix + "@example.test"
	recipientHash := hash(recipient)
	link := "https://example.test/verify?token=SECRET-UNIQUE-" + suffix
	eventID := "evt-" + suffix
	providerID := "provider-" + suffix
	if e = outbox.SendVerification(ctx, recipient, link); e != nil {
		t.Fatal(e)
	}
	if e = outbox.SendVerification(ctx, recipient, link); e != nil {
		t.Fatal(e)
	}
	var count int
	var encrypted []byte
	if e = pool.QueryRow(ctx, `SELECT count(*),(SELECT payload_ciphertext FROM email_outbox WHERE recipient_hash=$1 LIMIT 1) FROM email_outbox WHERE recipient_hash=$1`, recipientHash).Scan(&count, &encrypted); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("dedup count=%d", count)
	}
	if bytes.Contains(encrypted, []byte("SECRET-UNIQUE")) {
		t.Fatal("sensitive token stored in plaintext")
	}
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32))
	rotated, e := NewOutboxKeyring(2, map[int16]string{1: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)), 2: oldKey})
	if e != nil {
		t.Fatal(e)
	}
	plain, e := rotated.Decrypt(encrypted, 1)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(plain, []byte("SECRET-UNIQUE")) {
		t.Fatal("old key could not decrypt during simulated rotation")
	}
	for i := range plain {
		plain[i] = 0
	}
	var claimed atomic.Int32
	claimResults := make(chan []Item, 2)
	claimErrors := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := outbox.Claim(ctx, 1)
			if err != nil {
				claimErrors <- err
				return
			}
			claimed.Add(int32(len(items)))
			claimResults <- items
		}()
	}
	wg.Wait()
	close(claimResults)
	close(claimErrors)
	for claimErr := range claimErrors {
		t.Fatalf("concurrent Claim failed: %v", claimErr)
	}
	if claimed.Load() != 1 {
		t.Fatalf("concurrent claims=%d", claimed.Load())
	}
	var claimedItem Item
	for items := range claimResults {
		if len(items) == 1 {
			claimedItem = items[0]
		}
	}
	if claimedItem.Kind != "verify_email" || claimedItem.Attempts != 1 || claimedItem.KeyVersion != 1 || len(claimedItem.Ciphertext) == 0 || claimedItem.Idempotency == "" || !claimedItem.ExpiresAt.After(time.Now()) {
		t.Fatal("Claim did not return the expected valid message")
	}
	var claimedStatus string
	var persistedAttempts int
	var hasLock bool
	if e = pool.QueryRow(ctx, `SELECT status,attempts,locked_at IS NOT NULL FROM email_outbox WHERE recipient_hash=$1`, recipientHash).Scan(&claimedStatus, &persistedAttempts, &hasLock); e != nil || claimedStatus != "processing" || persistedAttempts != 1 || !hasLock {
		t.Fatal("Claim state did not atomically persist processing, first attempt and lock")
	}
	if items, claimErr := outbox.Claim(ctx, 1); claimErr != nil || len(items) != 0 {
		t.Fatalf("already-locked message was claimed again: items=%d err=%v", len(items), claimErr)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='processing',locked_at=now()-interval '6 minutes' WHERE recipient_hash=$1`, recipientHash); e != nil {
		t.Fatal(e)
	}
	recovered, e := outbox.Claim(ctx, 1)
	if e != nil || len(recovered) != 1 || recovered[0].Attempts != 2 {
		t.Fatalf("restart recovery items=%d err=%v", len(recovered), e)
	}
	claimFailureRecipient := "claim-failure-" + suffix + "@example.test"
	if e = outbox.SendVerification(ctx, claimFailureRecipient, link); e != nil {
		t.Fatal(e)
	}
	if _, e = controlPool.Exec(ctx, "ALTER TABLE "+schemaIdent+".email_suppressions RENAME TO email_suppressions_disabled"); e != nil {
		t.Fatal("induce isolated Claim query failure")
	}
	if _, claimErr := outbox.Claim(ctx, 1); claimErr == nil {
		t.Fatal("Claim query failure was not propagated")
	}
	if _, e = controlPool.Exec(ctx, "ALTER TABLE "+schemaIdent+".email_suppressions_disabled RENAME TO email_suppressions"); e != nil {
		t.Fatal("restore isolated suppression table")
	}
	var claimFailureStatus string
	var claimFailureAttempts int
	if e = pool.QueryRow(ctx, `SELECT status,attempts FROM email_outbox WHERE recipient_hash=$1`, hash(claimFailureRecipient)).Scan(&claimFailureStatus, &claimFailureAttempts); e != nil || claimFailureStatus != "pending" || claimFailureAttempts != 0 {
		t.Fatal("failed Claim did not roll back its row changes")
	}
	if _, e = pool.Exec(ctx, `INSERT INTO email_suppressions(recipient_hash,reason) VALUES($1,'manual')`, hash(claimFailureRecipient)); e != nil {
		t.Fatal("prepare isolated suppression fixture")
	}
	if suppressed, suppressErr := outbox.SuppressPending(ctx); suppressErr != nil || suppressed != 1 {
		t.Fatalf("suppression did not transition pending message: count=%d err=%v", suppressed, suppressErr)
	}
	futureRecipient := "future-retry-" + suffix + "@example.test"
	if e = outbox.SendVerification(ctx, futureRecipient, link); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='retry',next_attempt_at=now()+interval '1 hour' WHERE recipient_hash=$1`, hash(futureRecipient)); e != nil {
		t.Fatal("prepare future-retry fixture")
	}
	if items, claimErr := outbox.Claim(ctx, 1); claimErr != nil || len(items) != 0 {
		t.Fatalf("future retry was claimed early: items=%d err=%v", len(items), claimErr)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='processing',locked_at=now()-interval '6 minutes',attempts=8 WHERE recipient_hash=$1`, recipientHash); e != nil {
		t.Fatal("prepare exhausted abandoned claim")
	}
	if items, claimErr := outbox.Claim(ctx, 1); claimErr != nil || len(items) != 0 {
		t.Fatalf("exhausted claim should not be retried: items=%d err=%v", len(items), claimErr)
	}
	var exhaustedStatus string
	if e = pool.QueryRow(ctx, `SELECT status FROM email_outbox WHERE recipient_hash=$1`, recipientHash).Scan(&exhaustedStatus); e != nil || exhaustedStatus != "failed" {
		t.Fatal("exhausted abandoned claim was not finalized")
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='pending',attempts=0,created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour',processed_at=NULL,locked_at=NULL WHERE recipient_hash=$1`, recipientHash); e != nil {
		t.Fatal(e)
	}
	if items, claimErr := outbox.Claim(ctx, 1); claimErr != nil || len(items) != 0 {
		t.Fatalf("expired message was claimed: items=%d err=%v", len(items), claimErr)
	}
	expired, e := outbox.ExpireDue(ctx)
	if e != nil || expired != 1 {
		t.Fatalf("expire due=%d err=%v", expired, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET processed_at=now()-interval '40 days' WHERE recipient_hash=$1`, recipientHash); e != nil {
		t.Fatal(e)
	}
	removed, _, e := outbox.Cleanup(ctx, 30*24*time.Hour, 90*24*time.Hour)
	if e != nil || removed != 1 {
		t.Fatalf("cleanup removed=%d err=%v", removed, e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO email_outbox(message_type,recipient_hash,payload_ciphertext,key_version,idempotency_key,expires_at,status,provider_message_id,processed_at)
		SELECT 'verify_email',repeat('a',64),decode(repeat('ab',32),'hex'),1,'quota-fixture/'||n,now()+interval '1 day','sent','quota-provider/'||n,now()
		FROM generate_series(1,100) AS n`); e != nil {
		t.Fatal("prepare isolated daily quota fixtures")
	}
	if e = outbox.SendVerification(ctx, recipient, link); e != nil {
		t.Fatal(e)
	}
	if items, claimErr := outbox.Claim(ctx, 1); claimErr != nil || len(items) != 0 {
		t.Fatalf("daily send quota was not enforced: items=%d err=%v", len(items), claimErr)
	}
	secretBytes := bytes.Repeat([]byte{9}, 32)
	secret := "whsec_" + base64.StdEncoding.EncodeToString(secretBytes)
	body := []byte(fmt.Sprintf(`{"type":"email.delivered","data":{"id":%q}}`, providerID))
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, secretBytes)
	_, _ = mac.Write([]byte(eventID + "." + stamp + "." + string(body)))
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	handler := ResendWebhook(pool, secret)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/resend", bytes.NewReader(body))
		req.Header.Set("svix-id", eventID)
		req.Header.Set("svix-timestamp", stamp)
		req.Header.Set("svix-signature", signature)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("webhook status=%d", response.Code)
		}
	}
	var events int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM email_delivery_events WHERE provider_event_id=$1`, eventID).Scan(&events); e != nil || events != 1 {
		t.Fatalf("duplicate webhook events=%d err=%v", events, e)
	}
}
