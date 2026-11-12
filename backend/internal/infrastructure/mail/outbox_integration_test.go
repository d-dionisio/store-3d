package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

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
	_, _ = pool.Exec(ctx, `DELETE FROM email_outbox WHERE recipient_hash=$1`, hash("fixture@example.com"))
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
	_, _ = pool.Exec(ctx, `DELETE FROM email_outbox WHERE recipient_hash=$1`, hash("fixture@example.com"))
}
