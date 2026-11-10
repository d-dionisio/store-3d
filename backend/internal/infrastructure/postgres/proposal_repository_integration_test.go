package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/admin"
)

func TestProposalLifecycleAndSafetyIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	var material, color, userID string
	if err := pool.QueryRow(ctx, `SELECT id FROM materials LIMIT 1`).Scan(&material); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM colors LIMIT 1`).Scan(&color); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES('proposal-test@example.com','Proposal Test',repeat('x',60),'admin') ON CONFLICT((lower(email))) DO UPDATE SET role='admin' RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var quoteID string
	if err := pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at) VALUES('ORC-PROPOSAL-TEST','Cliente','client@example.com','submitted',$1,$2,1,now()) RETURNING id`, material, color).Scan(&quoteID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,retention_until) VALUES($1,'r2','private','proposal-test','model.stl','stl','model/stl',84,repeat('a',64),'pending',now()+interval '1 day')`, quoteID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM admin_audit_events WHERE resource_id='ORC-PROPOSAL-TEST'`)
		_, _ = pool.Exec(ctx, `DELETE FROM quote_proposals WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quotes WHERE id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id=$1`, userID)
	})
	repo := NewAdminRepository(pool)
	if err := repo.StartAnalysis(ctx, "ORC-PROPOSAL-TEST", userID); err != nil {
		t.Fatal(err)
	}
	pricing := domain.Pricing{MaterialCostCents: 1000, EnergyCostCents: 200, MarginBasisPoints: 1000, FinalPriceCents: 1320, ProductionLeadTimeDays: 5, ValidUntil: time.Now().Add(48 * time.Hour), CommercialNotes: "Frete não incluído"}
	if err := repo.SavePricing(ctx, "ORC-PROPOSAL-TEST", userID, pricing); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errorsCh := make(chan error, 2)
	for _, finalPrice := range []int64{1430, 1540} {
		wg.Add(1)
		go func(value int64) {
			defer wg.Done()
			concurrent := pricing
			concurrent.FinalPriceCents = value
			errorsCh <- repo.SavePricing(ctx, "ORC-PROPOSAL-TEST", userID, concurrent)
		}(finalPrice)
	}
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}
	var drafts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM quote_proposals WHERE quote_id=$1 AND status='draft'`, quoteID).Scan(&drafts); err != nil || drafts != 1 {
		t.Fatalf("draft count=%d err=%v", drafts, err)
	}
	if err := repo.SendProposal(ctx, "ORC-PROPOSAL-TEST", userID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("pending file should block send: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE quote_files SET security_status='clean',security_checked_at=now() WHERE quote_id=$1`, quoteID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SendProposal(ctx, "ORC-PROPOSAL-TEST", userID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SendProposal(ctx, "ORC-PROPOSAL-TEST", userID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("duplicate send should fail: %v", err)
	}
	var status, notification string
	var price int64
	if err := pool.QueryRow(ctx, `SELECT q.status,p.notification_status,p.final_price_cents FROM quotes q JOIN quote_proposals p ON p.quote_id=q.id WHERE q.id=$1`, quoteID).Scan(&status, &notification, &price); err != nil {
		t.Fatal(err)
	}
	if status != "quoted" || notification != "not_configured" || (price != 1430 && price != 1540) {
		t.Fatalf("unexpected snapshot: %s %s %d", status, notification, price)
	}
	if _, err := pool.Exec(ctx, `UPDATE quote_proposals SET created_at=now()-interval '2 days',valid_until=now()-interval '1 day' WHERE quote_id=$1`, quoteID); err != nil {
		t.Fatal(err)
	}
	expired, err := repo.ExpireProposals(ctx, userID)
	if err != nil || expired != 1 {
		t.Fatalf("expire count=%d err=%v", expired, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM quotes WHERE id=$1`, quoteID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("quote expiry status=%s err=%v", status, err)
	}
	var audits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE resource_id='ORC-PROPOSAL-TEST'`).Scan(&audits); err != nil || audits != 6 {
		t.Fatalf("audit count=%d err=%v", audits, err)
	}
}
