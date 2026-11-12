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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOutboxIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	config, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasSuffix(strings.ToLower(config.ConnConfig.Database), "_test") {
		t.Fatal("requires *_test database")
	}
	pool, e := pgxpool.NewWithConfig(context.Background(), config)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	ctx := context.Background()
	cipher, _ := NewOutboxCipher(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)), 1)
	outbox := NewOutbox(pool, cipher)
	// This suite is destructive only in a database explicitly suffixed with _test.
	_, _ = pool.Exec(ctx, `DELETE FROM email_delivery_events`)
	_, _ = pool.Exec(ctx, `DELETE FROM email_outbox`)
	link := "https://example.test/verify?token=SECRET-UNIQUE"
	if e = outbox.SendVerification(ctx, "fixture@example.com", link); e != nil {
		t.Fatal(e)
	}
	if e = outbox.SendVerification(ctx, "fixture@example.com", link); e != nil {
		t.Fatal(e)
	}
	var count int
	var encrypted []byte
	if e = pool.QueryRow(ctx, `SELECT count(*),(SELECT payload_ciphertext FROM email_outbox WHERE recipient_hash=$1 LIMIT 1) FROM email_outbox WHERE recipient_hash=$1`, hash("fixture@example.com")).Scan(&count, &encrypted); e != nil {
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
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := outbox.Claim(ctx, 1)
			if err == nil {
				claimed.Add(int32(len(items)))
			}
		}()
	}
	wg.Wait()
	if claimed.Load() != 1 {
		t.Fatalf("concurrent claims=%d", claimed.Load())
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='processing',locked_at=now()-interval '6 minutes' WHERE recipient_hash=$1`, hash("fixture@example.com")); e != nil {
		t.Fatal(e)
	}
	recovered, e := outbox.Claim(ctx, 1)
	if e != nil || len(recovered) != 1 {
		t.Fatalf("restart recovery items=%d err=%v", len(recovered), e)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET status='pending',created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour',processed_at=NULL,locked_at=NULL WHERE recipient_hash=$1`, hash("fixture@example.com")); e != nil {
		t.Fatal(e)
	}
	expired, e := outbox.ExpireDue(ctx)
	if e != nil || expired != 1 {
		t.Fatalf("expire due=%d err=%v", expired, e)
	}
	if _, e = pool.Exec(ctx, `UPDATE email_outbox SET processed_at=now()-interval '40 days' WHERE recipient_hash=$1`, hash("fixture@example.com")); e != nil {
		t.Fatal(e)
	}
	removed, _, e := outbox.Cleanup(ctx, 30*24*time.Hour, 90*24*time.Hour)
	if e != nil || removed != 1 {
		t.Fatalf("cleanup removed=%d err=%v", removed, e)
	}
	secretBytes := bytes.Repeat([]byte{9}, 32)
	secret := "whsec_" + base64.StdEncoding.EncodeToString(secretBytes)
	eventID := fmt.Sprintf("evt-%d", time.Now().UnixNano())
	providerID := fmt.Sprintf("provider-%d", time.Now().UnixNano())
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
	_, _ = pool.Exec(ctx, `DELETE FROM email_delivery_events WHERE provider_event_id=$1`, eventID)
	_, _ = pool.Exec(ctx, `DELETE FROM email_outbox WHERE recipient_hash=$1`, hash("fixture@example.com"))
}
