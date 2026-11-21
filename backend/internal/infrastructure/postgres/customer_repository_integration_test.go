package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/customer"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestCustomerMagicLinkIsolationAndConcurrentDecisionIntegration(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Customer PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "customer")
	if err != nil {
		t.Fatalf("create isolated Customer schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated Customer schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated Customer pool: %v", err)
	}
	repo := NewCustomerRepository(pool)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique Customer fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	primaryEmail := "customer-" + suffix + "@example.invalid"
	otherEmail := "other-" + suffix + "@example.invalid"
	unverifiedEmail := "unverified-" + suffix + "@example.invalid"
	quoteNumber := "ORC-CUSTOMER-" + suffix
	magicTokenHash := app.Hash("magic-" + fixtureID)
	sessionTokenHash := app.Hash("session-" + fixtureID)
	csrfTokenHash := app.Hash("csrf-" + fixtureID)
	unverifiedSessionHash := app.Hash("unverified-session-" + fixtureID)
	unverifiedCSRFHash := app.Hash("unverified-csrf-" + fixtureID)

	var unverifiedID string
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,full_name) VALUES($1,'Unverified fixture') RETURNING id`, unverifiedEmail).Scan(&unverifiedID); err != nil {
		t.Fatal("insert isolated unverified-customer fixture:", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')`, unverifiedID, unverifiedSessionHash, unverifiedCSRFHash); err != nil {
		t.Fatal("insert isolated unverified-session fixture:", err)
	}
	if _, err = repo.GetSession(ctx, unverifiedSessionHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("unverified user session=%v", err)
	}
	expiredHash := app.Hash("expired-magic-" + fixtureID)
	if _, err = pool.Exec(ctx, `INSERT INTO customer_magic_links(email,token_hash,created_at,expires_at) VALUES($1,$2,now()-interval '2 hours',now()-interval '1 hour')`, primaryEmail, expiredHash); err != nil {
		t.Fatal("insert isolated expired magic-link fixture:", err)
	}
	if _, err = repo.ConsumeMagicLink(ctx, expiredHash, app.Hash("expired-session-"+fixtureID), app.Hash("expired-csrf-"+fixtureID), "127.0.0.1", "test", time.Now().Add(time.Hour)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expired token=%v", err)
	}

	var materialID, colorID string
	if err = pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id`, "CUS-PLA-"+suffix, "Customer fixture PLA "+suffix).Scan(&materialID); err != nil {
		t.Fatal("insert isolated material fixture:", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#C8B99A',true) RETURNING id`, "customer-fixture-"+suffix, "Customer fixture color "+suffix).Scan(&colorID); err != nil {
		t.Fatal("insert isolated color fixture:", err)
	}
	adminEmail := "customer-proposal-admin-" + suffix + "@example.invalid"
	var adminID string
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,repeat('x',60),'admin') RETURNING id`, adminEmail, "Test admin "+suffix).Scan(&adminID); err != nil {
		t.Fatal("insert isolated proposal-admin fixture:", err)
	}
	var quoteID string
	if err = pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at,quoted_amount_cents,estimated_lead_time_days,quoted_at,valid_until) VALUES($1,$2,$3,'quoted',$4,$5,1,now(),2000,3,now(),now()+interval '2 days') RETURNING id`, quoteNumber, "Customer fixture "+suffix, primaryEmail, materialID, colorID).Scan(&quoteID); err != nil {
		t.Fatal("insert isolated quote fixture:", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,security_checked_at,retention_until) VALUES($1,'r2','private',$2,'model.stl','stl','model/stl',10,repeat('a',64),'clean',now(),now()+interval '1 day')`, quoteID, "customer-test/"+fixtureID+".stl"); err != nil {
		t.Fatal("insert isolated quote-file fixture:", err)
	}
	var proposalID string
	if err = pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at) VALUES($1,1,'sent',2000,3,now()+interval '2 days',$2,$2,now()) RETURNING id`, quoteID, adminID).Scan(&proposalID); err != nil {
		t.Fatal("insert isolated proposal fixture:", err)
	}

	if err = repo.CreateMagicLink(ctx, primaryEmail, magicTokenHash, "127.0.0.1", time.Now().Add(time.Minute)); err != nil {
		t.Fatal("create isolated magic link:", err)
	}
	session, err := repo.ConsumeMagicLink(ctx, magicTokenHash, sessionTokenHash, csrfTokenHash, "127.0.0.1", "test", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal("consume isolated magic link:", err)
	}
	if _, err = repo.ConsumeMagicLink(ctx, magicTokenHash, app.Hash("other-session-"+fixtureID), app.Hash("other-csrf-"+fixtureID), "127.0.0.1", "test", time.Now().Add(time.Hour)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("magic-link replay=%v", err)
	}
	detail, err := repo.GetQuote(ctx, session.UserID, quoteNumber)
	if err != nil || len(detail.Proposals) != 1 {
		t.Fatalf("quote detail proposal count=%d err=%v", len(detail.Proposals), err)
	}
	var otherID string
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Other fixture',now()) RETURNING id`, otherEmail).Scan(&otherID); err != nil {
		t.Fatal("insert isolated second-customer fixture:", err)
	}
	if _, err = repo.GetQuote(ctx, otherID, quoteNumber); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("cross-customer quote access=%v", err)
	}
	var latestProposalID string
	if err = pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at) VALUES($1,2,'sent',2200,4,now()+interval '2 days',$2,$2,now()) RETURNING id`, quoteID, adminID).Scan(&latestProposalID); err != nil {
		t.Fatal("insert isolated latest proposal fixture:", err)
	}
	if err = repo.Decide(ctx, session.UserID, quoteNumber, proposalID, "accepted", session.ID); !errors.Is(err, app.ErrInvalidDecision) {
		t.Fatalf("old proposal version decision=%v", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, decision := range []string{"accepted", "rejected"} {
		wg.Add(1)
		go func(value string) {
			defer wg.Done()
			results <- repo.Decide(ctx, session.UserID, quoteNumber, latestProposalID, value, session.ID)
		}(decision)
	}
	wg.Wait()
	close(results)
	success, failed := 0, 0
	for result := range results {
		if result == nil {
			success++
		} else if errors.Is(result, app.ErrInvalidDecision) {
			failed++
		} else {
			t.Fatal("concurrent proposal decision failed unexpectedly:", result)
		}
	}
	if success != 1 || failed != 1 {
		t.Fatalf("concurrent proposal decisions: success=%d rejected=%d", success, failed)
	}
	if err = repo.RevokeSession(ctx, session.ID); err != nil {
		t.Fatal("revoke isolated customer session:", err)
	}
	if _, err = repo.GetSession(ctx, sessionTokenHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("revoked session=%v", err)
	}
}
