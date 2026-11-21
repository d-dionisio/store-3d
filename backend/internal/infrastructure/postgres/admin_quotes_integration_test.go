package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAdminQuoteListFiltersPaginationAndDetailIntegration(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Admin quotes PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "admin_quotes")
	if err != nil {
		t.Fatalf("create isolated admin-quotes schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated admin-quotes schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated admin-quotes pool: %v", err)
	}

	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique admin-quotes fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	materialID, colorID, adminID, customerID := insertAdminQuoteReferenceFixtures(t, ctx, pool, suffix)
	customerName := "Admin quote customer " + suffix
	customerEmail := "admin-quotes-" + suffix + "@example.invalid"
	quoteRefs := []string{
		"ORC-ADMIN-" + suffix + "-A",
		"ORC-ADMIN-" + suffix + "-B",
		"ORC-ADMIN-" + suffix + "-C",
		"ORC-ADMIN-" + suffix + "-DRAFT",
	}
	createdAt := []time.Time{time.Now().Add(-4 * time.Minute), time.Now().Add(-3 * time.Minute), time.Now().Add(-2 * time.Minute), time.Now().Add(-time.Minute)}
	statuses := []string{"submitted", "submitted", "submitted", "draft"}
	var detailQuoteID string
	for i, reference := range quoteRefs {
		var quoteID string
		var submittedAt *time.Time
		if statuses[i] == "submitted" {
			submittedAt = &createdAt[i]
		}
		err = pool.QueryRow(ctx, `INSERT INTO quotes(user_id,quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at,created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,2,$8,$9) RETURNING id`, customerID, reference, customerName, customerEmail, statuses[i], materialID, colorID, submittedAt, createdAt[i]).Scan(&quoteID)
		if err != nil {
			t.Fatalf("insert isolated quote fixture %d: %v", i, err)
		}
		if reference == quoteRefs[2] {
			detailQuoteID = quoteID
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,retention_until)
		VALUES($1,'r2','private',$2,'fixture.stl','stl','model/stl',84,repeat('a',64),'pending',now()+interval '1 day')`, detailQuoteID, "admin-quotes/"+fixtureID+".stl"); err != nil {
		t.Fatalf("insert isolated quote-file fixture: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by,sent_by,sent_at,notification_status,commercial_notes)
		VALUES($1,1,'sent',12900,5,now()+interval '2 days',$2,$2,now(),'not_configured',$3)`, detailQuoteID, adminID, "Isolated proposal fixture"); err != nil {
		t.Fatalf("insert isolated proposal fixture: %v", err)
	}

	repo := NewAdminRepository(pool)
	newest, err := repo.ListQuotes(ctx, customerName, "submitted", "newest", 1, 2)
	if err != nil {
		t.Fatalf("list isolated submitted quotes (newest): %v", err)
	}
	if newest.Total != 3 || len(newest.Items) != 2 || newest.Items[0].Reference != quoteRefs[2] || newest.Items[1].Reference != quoteRefs[1] {
		t.Fatalf("unexpected newest quote page: total=%d count=%d", newest.Total, len(newest.Items))
	}
	newestSecondPage, err := repo.ListQuotes(ctx, customerName, "submitted", "newest", 2, 2)
	if err != nil || newestSecondPage.Total != 3 || len(newestSecondPage.Items) != 1 || newestSecondPage.Items[0].Reference != quoteRefs[0] {
		t.Fatalf("unexpected second newest quote page: total=%d count=%d err=%v", newestSecondPage.Total, len(newestSecondPage.Items), err)
	}
	oldest, err := repo.ListQuotes(ctx, customerName, "submitted", "oldest", 1, 2)
	if err != nil || oldest.Total != 3 || len(oldest.Items) != 2 || oldest.Items[0].Reference != quoteRefs[0] || oldest.Items[1].Reference != quoteRefs[1] {
		t.Fatalf("unexpected oldest quote page: total=%d count=%d err=%v", oldest.Total, len(oldest.Items), err)
	}
	referenceOrder, err := repo.ListQuotes(ctx, customerName, "", "reference", 1, 4)
	if err != nil || referenceOrder.Total != 4 || len(referenceOrder.Items) != 4 || referenceOrder.Items[0].Reference != quoteRefs[0] {
		t.Fatalf("unexpected reference-ordered quote page: total=%d count=%d err=%v", referenceOrder.Total, len(referenceOrder.Items), err)
	}

	detail, err := repo.GetQuote(ctx, quoteRefs[2])
	if err != nil {
		t.Fatalf("get isolated quote detail: %v", err)
	}
	if detail.CustomerEmail != customerEmail || len(detail.Files) != 1 || detail.Files[0].SecurityStatus != "pending" || len(detail.Proposals) != 1 || detail.Proposals[0].Version != 1 {
		t.Fatalf("unexpected isolated quote detail: files=%d proposals=%d", len(detail.Files), len(detail.Proposals))
	}
	if len(detail.History) != 3 {
		t.Fatalf("expected quote, file, and proposal history entries; got %d", len(detail.History))
	}
}

func insertAdminQuoteReferenceFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string) (materialID, colorID, adminID, customerID string) {
	t.Helper()
	if err := pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id::text`, "AQ-PLA-"+suffix, "Admin quotes PLA "+suffix).Scan(&materialID); err != nil {
		t.Fatalf("insert isolated admin-quotes material fixture: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#C8B99A',true) RETURNING id::text`, "admin-quotes-"+suffix, "Admin quotes color "+suffix).Scan(&colorID); err != nil {
		t.Fatalf("insert isolated admin-quotes color fixture: %v", err)
	}
	adminEmail := "admin-quotes-user-" + suffix + "@example.invalid"
	if err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,repeat('x',60),'quote_viewer','active') RETURNING id::text`, adminEmail, "Admin quotes fixture "+suffix).Scan(&adminID); err != nil {
		t.Fatalf("insert isolated admin fixture: %v", err)
	}
	customerEmail := "admin-quotes-customer-" + suffix + "@example.invalid"
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,$2,now()) RETURNING id::text`, customerEmail, "Admin quotes customer "+suffix).Scan(&customerID); err != nil {
		t.Fatalf("insert isolated customer fixture: %v", err)
	}
	return materialID, colorID, adminID, customerID
}
