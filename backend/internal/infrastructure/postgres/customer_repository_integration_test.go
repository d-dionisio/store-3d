package postgres

import (
	"context"
	"errors"
	app "example.com/loja-3d/backend/internal/application/customer"
	"sync"
	"testing"
	"time"
)

func TestCustomerMagicLinkIsolationAndConcurrentDecisionIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	repo := NewCustomerRepository(pool)
	email := "customer-integration@example.com"
	other := "other-integration@example.com"
	unverified := "unverified-integration@example.com"
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, unverified)
	_, _ = pool.Exec(ctx, `DELETE FROM customer_magic_links WHERE lower(email) IN(lower($1),lower($2))`, email, other)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email IN($1,$2)`, email, other)
	var unverifiedID string
	if e := pool.QueryRow(ctx, `INSERT INTO users(email,full_name) VALUES($1,'Não verificado') RETURNING id`, unverified).Scan(&unverifiedID); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour')`, unverifiedID, app.Hash("unverified-session"), app.Hash("unverified-csrf")); e != nil {
		t.Fatal(e)
	}
	if _, e := repo.GetSession(ctx, app.Hash("unverified-session")); !errors.Is(e, app.ErrUnauthorized) {
		t.Fatalf("unverified user session=%v", e)
	}
	expiredHash := app.Hash("expired-magic")
	if _, e := pool.Exec(ctx, `INSERT INTO customer_magic_links(email,token_hash,created_at,expires_at) VALUES($1,$2,now()-interval '2 hours',now()-interval '1 hour')`, email, expiredHash); e != nil {
		t.Fatal(e)
	}
	if _, e := repo.ConsumeMagicLink(ctx, expiredHash, app.Hash("expired-session"), app.Hash("expired-csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour)); !errors.Is(e, app.ErrUnauthorized) {
		t.Fatalf("expired token=%v", e)
	}
	var material, color, adminID string
	if e := pool.QueryRow(ctx, `SELECT id FROM materials LIMIT 1`).Scan(&material); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `SELECT id FROM colors LIMIT 1`).Scan(&color); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES('customer-proposal-admin@example.com','Test Admin',repeat('x',60),'admin') ON CONFLICT((lower(email))) DO UPDATE SET role='admin' RETURNING id`).Scan(&adminID); e != nil {
		t.Fatal(e)
	}
	var quoteID string
	if e := pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at,quoted_amount_cents,estimated_lead_time_days,quoted_at,valid_until) VALUES('ORC-CUSTOMER-TEST','Cliente Teste',$1,'quoted',$2,$3,1,now(),2000,3,now(),now()+interval '2 days') RETURNING id`, email, material, color).Scan(&quoteID); e != nil {
		t.Fatal(e)
	}
	if _, e := pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,security_checked_at,retention_until) VALUES($1,'r2','private','customer-test','model.stl','stl','model/stl',10,repeat('a',64),'clean',now(),now()+interval '1 day')`, quoteID); e != nil {
		t.Fatal(e)
	}
	var proposalID string
	if e := pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at) VALUES($1,1,'sent',2000,3,now()+interval '2 days',$2,$2,now()) RETURNING id`, quoteID, adminID).Scan(&proposalID); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customer_audit_events WHERE resource_id='ORC-CUSTOMER-TEST'`)
		_, _ = pool.Exec(ctx, `DELETE FROM customer_magic_links WHERE lower(email) IN(lower($1),lower($2))`, email, other)
		_, _ = pool.Exec(ctx, `DELETE FROM quote_proposal_decisions WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quote_proposals WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quotes WHERE id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email IN($1,$2)`, email, other)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, unverifiedID)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id=$1`, adminID)
	})
	hash := app.Hash("magic")
	if e := repo.CreateMagicLink(ctx, email, hash, "127.0.0.1", time.Now().Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	s, e := repo.ConsumeMagicLink(ctx, hash, app.Hash("session"), app.Hash("csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = repo.ConsumeMagicLink(ctx, hash, app.Hash("other-session"), app.Hash("other-csrf"), "127.0.0.1", "test", time.Now().Add(time.Hour)); !errors.Is(e, app.ErrUnauthorized) {
		t.Fatalf("replay=%v", e)
	}
	detail, e := repo.GetQuote(ctx, s.UserID, "ORC-CUSTOMER-TEST")
	if e != nil || len(detail.Proposals) != 1 {
		t.Fatalf("detail=%+v err=%v", detail, e)
	}
	var otherID string
	if e = pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Outro',now()) RETURNING id`, other).Scan(&otherID); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.GetQuote(ctx, otherID, "ORC-CUSTOMER-TEST"); !errors.Is(e, app.ErrNotFound) {
		t.Fatalf("isolation=%v", e)
	}
	var latestProposalID string
	if e = pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at) VALUES($1,2,'sent',2200,4,now()+interval '2 days',$2,$2,now()) RETURNING id`, quoteID, adminID).Scan(&latestProposalID); e != nil {
		t.Fatal(e)
	}
	if e = repo.Decide(ctx, s.UserID, "ORC-CUSTOMER-TEST", proposalID, "accepted", s.ID); !errors.Is(e, app.ErrInvalidDecision) {
		t.Fatalf("old version=%v", e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, decision := range []string{"accepted", "rejected"} {
		wg.Add(1)
		go func(v string) {
			defer wg.Done()
			results <- repo.Decide(ctx, s.UserID, "ORC-CUSTOMER-TEST", latestProposalID, v, s.ID)
		}(decision)
	}
	wg.Wait()
	close(results)
	success, failed := 0, 0
	for e = range results {
		if e == nil {
			success++
		} else if errors.Is(e, app.ErrInvalidDecision) {
			failed++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || failed != 1 {
		t.Fatalf("success=%d failed=%d", success, failed)
	}
	if e = repo.RevokeSession(ctx, s.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.GetSession(ctx, app.Hash("session")); !errors.Is(e, app.ErrUnauthorized) {
		t.Fatalf("revoked session=%v", e)
	}
}
