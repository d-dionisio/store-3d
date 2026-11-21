package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/admin"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestProposalLifecycleAndSafetyIntegration(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Proposal PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "proposal")
	if err != nil {
		t.Fatalf("create isolated Proposal schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated Proposal schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated Proposal pool: %v", err)
	}
	concurrentPoolA, err := schema.OpenPoolWithApplicationName(ctx, "proposal-pricing-a")
	if err != nil {
		t.Fatalf("open first independent pricing pool: %v", err)
	}
	concurrentPoolB, err := schema.OpenPoolWithApplicationName(ctx, "proposal-pricing-b")
	if err != nil {
		t.Fatalf("open second independent pricing pool: %v", err)
	}

	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique Proposal fixture UUID")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	var materialID, colorID, adminID, customerID, quoteID string
	if err = pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id`, "PROP-PLA-"+suffix, "Proposal fixture PLA "+suffix).Scan(&materialID); err != nil {
		t.Fatal("insert isolated Proposal material fixture:", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#AABBCC',true) RETURNING id`, "proposal-color-"+suffix, "Proposal fixture color "+suffix).Scan(&colorID); err != nil {
		t.Fatal("insert isolated Proposal color fixture:", err)
	}
	adminEmail := "proposal-admin-" + suffix + "@example.invalid"
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,repeat('x',60),'admin','active') RETURNING id`, adminEmail, "Proposal admin "+suffix).Scan(&adminID); err != nil {
		t.Fatal("insert isolated Proposal administrator fixture:", err)
	}
	customerEmail := "proposal-customer-" + suffix + "@example.invalid"
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,$2,now()) RETURNING id`, customerEmail, "Proposal customer "+suffix).Scan(&customerID); err != nil {
		t.Fatal("insert isolated Proposal customer fixture:", err)
	}
	quoteNumber := "ORC-PROPOSAL-" + strings.ToUpper(suffix)
	if err = pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,user_id,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at) VALUES($1,$2,$3,$4,'submitted',$5,$6,1,now()) RETURNING id`, quoteNumber, customerID, "Proposal customer "+suffix, customerEmail, materialID, colorID).Scan(&quoteID); err != nil {
		t.Fatal("insert isolated Proposal quote fixture:", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,retention_until) VALUES($1,'r2','private',$2,'model.stl','stl','model/stl',84,repeat('a',64),'pending',now()+interval '1 day')`, quoteID, "proposal-test/"+fixtureID+".stl"); err != nil {
		t.Fatal("insert isolated Proposal file fixture:", err)
	}

	repo := NewAdminRepository(pool)
	if err = repo.StartAnalysis(ctx, quoteNumber, adminID); err != nil {
		t.Fatal("start isolated quote analysis:", err)
	}
	pricing := domain.Pricing{MaterialCostCents: 1000, EnergyCostCents: 200, MarginBasisPoints: 1000, FinalPriceCents: 1320, ProductionLeadTimeDays: 5, ValidUntil: time.Now().Add(48 * time.Hour), CommercialNotes: "Frete não incluído"}
	if err = repo.SavePricing(ctx, quoteNumber, adminID, pricing); err != nil {
		t.Fatal("save initial isolated pricing:", err)
	}
	var wg sync.WaitGroup
	errorsCh := make(chan error, 2)
	type pricingAttempt struct {
		repository *AdminRepository
		priceCents int64
	}
	for _, attempt := range []pricingAttempt{
		{repository: NewAdminRepository(concurrentPoolA), priceCents: 1430},
		{repository: NewAdminRepository(concurrentPoolB), priceCents: 1540},
	} {
		wg.Add(1)
		go func(current pricingAttempt) {
			defer wg.Done()
			concurrent := pricing
			concurrent.FinalPriceCents = current.priceCents
			errorsCh <- current.repository.SavePricing(ctx, quoteNumber, adminID, concurrent)
		}(attempt)
	}
	wg.Wait()
	close(errorsCh)
	for saveErr := range errorsCh {
		if saveErr != nil {
			t.Fatalf("concurrent isolated pricing update: %v", saveErr)
		}
	}
	var drafts int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM quote_proposals WHERE quote_id=$1 AND status='draft'`, quoteID).Scan(&drafts); err != nil || drafts != 1 {
		t.Fatalf("isolated draft count=%d err=%v", drafts, err)
	}
	if err = repo.SendProposal(ctx, quoteNumber, adminID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("pending file should block sending the isolated proposal: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE quote_files SET security_status='clean',security_checked_at=now() WHERE quote_id=$1`, quoteID); err != nil {
		t.Fatal("mark isolated quote file as clean:", err)
	}
	if err = repo.SendProposal(ctx, quoteNumber, adminID); err != nil {
		t.Fatal("send isolated proposal:", err)
	}
	if err = repo.SendProposal(ctx, quoteNumber, adminID); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("duplicate proposal send should be rejected: %v", err)
	}
	var status, notification string
	var price int64
	if err = pool.QueryRow(ctx, `SELECT q.status,p.notification_status,p.final_price_cents FROM quotes q JOIN quote_proposals p ON p.quote_id=q.id WHERE q.id=$1`, quoteID).Scan(&status, &notification, &price); err != nil {
		t.Fatal("read isolated proposal snapshot:", err)
	}
	if status != "quoted" || notification != "not_configured" || (price != 1430 && price != 1540) {
		t.Fatalf("unexpected isolated proposal snapshot: %s %s %d", status, notification, price)
	}
	if _, err = pool.Exec(ctx, `UPDATE quote_proposals SET created_at=now()-interval '2 days',valid_until=now()-interval '1 day' WHERE quote_id=$1`, quoteID); err != nil {
		t.Fatal("expire isolated proposal fixture:", err)
	}
	expired, err := repo.ExpireProposals(ctx, adminID)
	if err != nil || expired != 1 {
		t.Fatalf("isolated proposal expiration count=%d err=%v", expired, err)
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM quotes WHERE id=$1`, quoteID).Scan(&status); err != nil || status != "expired" {
		t.Fatalf("isolated quote expiry status=%s err=%v", status, err)
	}
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE admin_user_id=$1 AND resource_id=$2`, adminID, quoteNumber).Scan(&audits); err != nil || audits != 6 {
		t.Fatalf("isolated proposal audit count=%d err=%v", audits, err)
	}
}
