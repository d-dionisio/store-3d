package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	app "example.com/loja-3d/backend/internal/application/quote"
	domain "example.com/loja-3d/backend/internal/domain/quote"
	"github.com/jackc/pgx/v5/pgxpool"
)

func quoteTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
func quoteFixtures(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var material, color string
	err := pool.QueryRow(ctx, `INSERT INTO materials(code,name) VALUES('QUOTE_TEST_PLA','Quote Test PLA') ON CONFLICT(code) DO UPDATE SET is_active=true RETURNING id`).Scan(&material)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code) VALUES('quote-test-black','Quote Test Black','#101010') ON CONFLICT(slug) DO UPDATE SET is_active=true RETURNING id`).Scan(&color)
	if err != nil {
		t.Fatal(err)
	}
	return material, color
}
func stagedUpload(t *testing.T, pool *pgxpool.Pool, token, bucket string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO quote_uploads(owner_token_hash,bucket_name,object_key,original_filename,file_extension,media_type,expected_size_bytes,checksum_sha256,status,expires_at,confirmed_at) VALUES($1,$2,gen_random_uuid()::text,'test.stl','stl','model/stl',84,$3,'confirmed',now()+interval '1 hour',now()) RETURNING id`, hashToken(token), bucket, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM quotes WHERE quote_number LIKE 'ORC-%' AND customer_email='integration@example.com'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM quote_uploads WHERE id=$1`, id)
	})
	return id
}
func requestFor(upload, token, material, color string) domain.CreateRequest {
	return domain.CreateRequest{Name: "Integration", Email: "integration@example.com", MaterialID: material, ColorID: color, Quantity: 1, UploadID: upload, UploadToken: token}
}

func TestQuoteCreateConsumesUploadAndRejectsReuseIntegration(t *testing.T) {
	pool := quoteTestPool(t)
	material, color := quoteFixtures(t, pool)
	token := "owner-token"
	id := stagedUpload(t, pool, token, "private-test")
	repo := NewQuoteRepository(pool)
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color), hashToken("access")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color), hashToken("access-2")); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("expected forbidden reuse, got %v", err)
	}
	var files int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM quote_files qf JOIN quotes q ON q.id=qf.quote_id WHERE q.customer_email='integration@example.com' AND qf.object_key=(SELECT object_key FROM quote_uploads WHERE id=$1)`, id).Scan(&files); err != nil || files != 1 {
		t.Fatalf("expected one associated file, count=%d err=%v", files, err)
	}
}

func TestQuoteConcurrentConsumptionIntegration(t *testing.T) {
	pool := quoteTestPool(t)
	material, color := quoteFixtures(t, pool)
	token := "concurrent-owner"
	id := stagedUpload(t, pool, token, "private-test")
	repo := NewQuoteRepository(pool)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Create(context.Background(), requestFor(id, token, material, color), hashToken("access-concurrent"))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	success, forbidden := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, app.ErrForbidden) {
			forbidden++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if success != 1 || forbidden != 1 {
		t.Fatalf("success=%d forbidden=%d", success, forbidden)
	}
}

func TestQuoteCreateRollbackIntegration(t *testing.T) {
	pool := quoteTestPool(t)
	material, color := quoteFixtures(t, pool)
	token := "rollback-owner"
	id := stagedUpload(t, pool, token, "")
	repo := NewQuoteRepository(pool)
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color), hashToken("access-rollback")); err == nil {
		t.Fatal("expected quote_files constraint failure")
	}
	var quotes int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM quotes WHERE customer_email='integration@example.com' AND quote_number='ORC-'||upper(substr(replace($1::text,'-',''),1,16))`, id).Scan(&quotes)
	if quotes != 0 {
		t.Fatalf("quote insert was not rolled back: %d", quotes)
	}
	var status string
	_ = pool.QueryRow(context.Background(), `SELECT status FROM quote_uploads WHERE id=$1`, id).Scan(&status)
	if status != "confirmed" {
		t.Fatalf("upload changed despite rollback: %s", status)
	}
}
