package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/quote"
	domain "example.com/loja-3d/backend/internal/domain/quote"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func quoteTestPool(t *testing.T) (*pgxpool.Pool, *testpostgres.IsolatedSchema) {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Quote PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "quote")
	if err != nil {
		t.Fatalf("create isolated Quote schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated Quote schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated Quote pool: %v", err)
	}
	return pool, schema
}

func quoteFixtureID(t *testing.T) string {
	t.Helper()
	id, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique Quote fixture UUID:", err)
	}
	return id
}

func quoteFixtures(t *testing.T, pool *pgxpool.Pool, fixtureID string) (string, string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	var material, color string
	err := pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id`, "QUOTE-PLA-"+suffix, "Quote fixture PLA "+suffix).Scan(&material)
	if err != nil {
		t.Fatal("insert isolated Quote material fixture:", err)
	}
	err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#101010',true) RETURNING id`, "quote-test-"+suffix, "Quote fixture color "+suffix).Scan(&color)
	if err != nil {
		t.Fatal("insert isolated Quote color fixture:", err)
	}
	return material, color
}

func stagedUpload(t *testing.T, pool *pgxpool.Pool, fixtureID, token, bucket string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `INSERT INTO quote_uploads(owner_token_hash,bucket_name,object_key,original_filename,file_extension,media_type,expected_size_bytes,checksum_sha256,status,expires_at,confirmed_at) VALUES($1,$2,gen_random_uuid()::text,$3,'stl','model/stl',84,$4,'confirmed',now()+interval '1 hour',now()) RETURNING id`, hashToken(token), bucket, "fixture-"+fixtureID+".stl", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").Scan(&id)
	if err != nil {
		t.Fatal("insert isolated staged-upload fixture:", err)
	}
	return id
}

func requestFor(upload, token, material, color, email, fixtureID string) domain.CreateRequest {
	return domain.CreateRequest{Name: "Quote fixture " + strings.ReplaceAll(fixtureID, "-", ""), Email: email, MaterialID: material, ColorID: color, Quantity: 1, UploadID: upload, UploadToken: token}
}

func TestQuoteCreateConsumesUploadAndRejectsReuseIntegration(t *testing.T) {
	pool, _ := quoteTestPool(t)
	fixtureID := quoteFixtureID(t)
	material, color := quoteFixtures(t, pool, fixtureID)
	token := "owner-" + fixtureID
	email := "quote-" + strings.ReplaceAll(fixtureID, "-", "") + "@example.invalid"
	id := stagedUpload(t, pool, fixtureID, token, "private-"+strings.ReplaceAll(fixtureID, "-", ""))
	repo := NewQuoteRepository(pool)
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color, email, fixtureID), hashToken("access-"+fixtureID)); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color, email, fixtureID), hashToken("access-replay-"+fixtureID)); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("expected forbidden upload reuse, got %v", err)
	}
	var files int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM quote_files qf JOIN quotes q ON q.id=qf.quote_id WHERE q.customer_email=$1 AND qf.object_key=(SELECT object_key FROM quote_uploads WHERE id=$2)`, email, id).Scan(&files); err != nil || files != 1 {
		t.Fatalf("expected exactly one associated file, count=%d err=%v", files, err)
	}
}

func TestQuoteConcurrentConsumptionIntegration(t *testing.T) {
	pool, schema := quoteTestPool(t)
	fixtureID := quoteFixtureID(t)
	material, color := quoteFixtures(t, pool, fixtureID)
	token := "concurrent-owner-" + fixtureID
	email := "quote-concurrent-" + strings.ReplaceAll(fixtureID, "-", "") + "@example.invalid"
	id := stagedUpload(t, pool, fixtureID, token, "private-"+strings.ReplaceAll(fixtureID, "-", ""))
	poolA, err := schema.OpenPoolWithApplicationName(context.Background(), "quote-race-a")
	if err != nil {
		t.Fatalf("open first independent quote pool: %v", err)
	}
	poolB, err := schema.OpenPoolWithApplicationName(context.Background(), "quote-race-b")
	if err != nil {
		t.Fatalf("open second independent quote pool: %v", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, callerPool := range []*pgxpool.Pool{poolA, poolB} {
		wg.Add(1)
		go func(repositoryPool *pgxpool.Pool) {
			defer wg.Done()
			<-start
			_, err := NewQuoteRepository(repositoryPool).Create(context.Background(), requestFor(id, token, material, color, email, fixtureID), hashToken("access-concurrent-"+fixtureID))
			results <- err
		}(callerPool)
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
			t.Fatalf("unexpected concurrent consumption error: %v", err)
		}
	}
	if success != 1 || forbidden != 1 {
		t.Fatalf("concurrent upload consumption: success=%d forbidden=%d", success, forbidden)
	}
}

func TestQuoteCreateRollbackIntegration(t *testing.T) {
	pool, _ := quoteTestPool(t)
	fixtureID := quoteFixtureID(t)
	material, color := quoteFixtures(t, pool, fixtureID)
	token := "rollback-owner-" + fixtureID
	email := "quote-rollback-" + strings.ReplaceAll(fixtureID, "-", "") + "@example.invalid"
	id := stagedUpload(t, pool, fixtureID, token, "")
	repo := NewQuoteRepository(pool)
	if _, err := repo.Create(context.Background(), requestFor(id, token, material, color, email, fixtureID), hashToken("access-rollback-"+fixtureID)); err == nil {
		t.Fatal("expected quote_files constraint failure")
	}
	reference := "ORC-" + strings.ToUpper(strings.ReplaceAll(id, "-", "")[:16])
	var quotes int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM quotes WHERE customer_email=$1 AND quote_number=$2`, email, reference).Scan(&quotes); err != nil {
		t.Fatal("verify quote rollback:", err)
	}
	if quotes != 0 {
		t.Fatalf("quote insert was not rolled back: %d", quotes)
	}
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM quote_uploads WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal("verify upload state after rollback:", err)
	}
	if status != "confirmed" {
		t.Fatalf("upload changed despite rollback: %s", status)
	}
}
