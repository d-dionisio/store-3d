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
	"github.com/jackc/pgx/v5/pgxpool"
)

type conversionFixture struct {
	pool                                              *pgxpool.Pool
	schema                                            *testpostgres.IsolatedSchema
	repo                                              *CustomerRepository
	userID, sessionID, quoteID, proposalID, reference string
}

func newConversionFixture(t *testing.T) conversionFixture {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Quote order conversion PostgreSQL integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "conversion")
	if err != nil {
		t.Fatalf("create isolated conversion schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to conversion schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated conversion pool: %v", err)
	}
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique conversion fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	var material, color, adminID, userID, sessionID, quoteID, proposalID string
	if err = pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id::text`, "CONV-PLA-"+suffix, "Conversion PLA "+suffix).Scan(&material); err != nil {
		t.Fatalf("insert isolated conversion material: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#123456',true) RETURNING id::text`, "conversion-"+suffix, "Conversion color "+suffix).Scan(&color); err != nil {
		t.Fatalf("insert isolated conversion color: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,repeat('x',60),'admin') RETURNING id::text`, "conversion-admin-"+suffix+"@example.invalid", "Conversion admin "+suffix).Scan(&adminID); err != nil {
		t.Fatalf("insert isolated conversion administrator: %v", err)
	}
	email := "conversion-" + suffix + "@example.invalid"
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,$2,now()) RETURNING id::text`, email, "Conversion customer "+suffix).Scan(&userID); err != nil {
		t.Fatalf("insert isolated conversion customer: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour') RETURNING id::text`, userID, app.Hash("conversion-session-"+fixtureID), app.Hash("conversion-csrf-"+fixtureID)).Scan(&sessionID); err != nil {
		t.Fatalf("insert isolated conversion session: %v", err)
	}
	reference := "ORC-CONV-" + suffix
	if err = pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,user_id,customer_name,customer_email,customer_phone,status,material_id,color_id,requested_quantity,width_mm,height_mm,depth_mm,notes,quoted_amount_cents,estimated_lead_time_days,valid_until,submitted_at,quoted_at,approved_at)
		VALUES($1,$2,$3,$4,'11999999999','approved',$5,$6,3,10,20,30,'Isolated conversion fixture',1001,5,now()+interval '2 days',now(),now(),now()) RETURNING id::text`, reference, userID, "Conversion customer "+suffix, email, material, color).Scan(&quoteID); err != nil {
		t.Fatalf("insert isolated conversion quote: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,security_checked_at,retention_until)
		VALUES($1,'r2','private',$2,'conversion.stl','stl','model/stl',10,repeat('a',64),'clean',now(),now()+interval '1 day')`, quoteID, "conversion/"+fixtureID+".stl"); err != nil {
		t.Fatalf("insert isolated conversion quote file: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,material_cost_cents,final_price_cents,production_lead_time_days,valid_until,commercial_notes,shipping_included,created_by,sent_by,sent_at)
		VALUES($1,1,'sent',700,1001,5,now()+interval '2 days','Preserved commercial condition',true,$2,$2,now()) RETURNING id::text`, quoteID, adminID).Scan(&proposalID); err != nil {
		t.Fatalf("insert isolated conversion proposal: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO quote_proposal_decisions(quote_id,proposal_id,user_id,decision,decided_at) VALUES($1,$2,$3,'accepted',now())`, quoteID, proposalID, userID); err != nil {
		t.Fatalf("insert isolated conversion decision: %v", err)
	}
	return conversionFixture{pool: pool, schema: schema, repo: NewCustomerRepository(pool), userID: userID, sessionID: sessionID, quoteID: quoteID, proposalID: proposalID, reference: reference}
}

func TestQuoteOrderConversionIntegration(t *testing.T) {
	ctx := context.Background()
	t.Run("valid idempotent snapshots and history", func(t *testing.T) {
		f := newConversionFixture(t)
		first, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if err != nil {
			t.Fatal(err)
		}
		second, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if err != nil || first != second {
			t.Fatalf("idempotency first=%+v second=%+v err=%v", first, second, err)
		}
		var status string
		var proposalVersion, lead, quantity int
		var subtotal, total, approvedTotal, lineTotal int64
		var shipping bool
		var commercial, material, color, dimensions string
		if err = f.pool.QueryRow(ctx, `SELECT o.status,o.quote_proposal_version,o.production_lead_time_days,o.subtotal_cents,o.total_cents,o.shipping_included,o.commercial_notes,oi.quantity,oi.approved_total_cents,oi.line_total_cents,oi.material_name,oi.color_name,oi.dimensions_snapshot FROM orders o JOIN order_items oi ON oi.order_id=o.id WHERE o.quote_id=$1`, f.quoteID).Scan(&status, &proposalVersion, &lead, &subtotal, &total, &shipping, &commercial, &quantity, &approvedTotal, &lineTotal, &material, &color, &dimensions); err != nil {
			t.Fatal(err)
		}
		if status != "pending_payment" || proposalVersion != 1 || lead != 5 || subtotal != 1001 || total != 1001 || !shipping || commercial != "Preserved commercial condition" || quantity != 3 || approvedTotal != 1001 || lineTotal != 1001 || material == "" || color == "" || dimensions != "10 × 20 × 30" {
			t.Fatalf("snapshots invalid: %s %d %d %d %d %v %q %d %d %d %q %q %q", status, proposalVersion, lead, subtotal, total, shipping, commercial, quantity, approvedTotal, lineTotal, material, color, dimensions)
		}
		var events, audits int
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events e JOIN orders o ON o.id=e.order_id WHERE o.quote_id=$1 AND e.status='pending_payment'`, f.quoteID).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM customer_audit_events WHERE user_id=$1 AND action='quote.convert'`, f.userID).Scan(&audits); err != nil {
			t.Fatal(err)
		}
		if events != 1 || audits != 1 {
			t.Fatalf("events=%d audits=%d", events, audits)
		}
	})
	t.Run("not approved", func(t *testing.T) {
		f := newConversionFixture(t)
		if _, err := f.pool.Exec(ctx, `UPDATE quotes SET status='quoted',approved_at=NULL WHERE id=$1`, f.quoteID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID); !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("expired at approval", func(t *testing.T) {
		f := newConversionFixture(t)
		if _, err := f.pool.Exec(ctx, `UPDATE quote_proposals SET created_at=now()-interval '3 days',valid_until=now()-interval '2 days' WHERE id=$1`, f.proposalID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `UPDATE quote_proposal_decisions SET decided_at=now()-interval '1 day' WHERE proposal_id=$1`, f.proposalID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID); !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("different owner", func(t *testing.T) {
		f := newConversionFixture(t)
		fixtureID, err := testpostgres.NewFixtureID()
		if err != nil {
			t.Fatal("create distinct owner fixture identifier")
		}
		var other string
		if err = f.pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Other isolated customer',now()) RETURNING id::text`, "other-"+strings.ReplaceAll(fixtureID, "-", "")+"@example.invalid").Scan(&other); err != nil {
			t.Fatal("insert second isolated customer:", err)
		}
		if _, err = f.repo.ConvertQuote(ctx, other, f.reference, f.sessionID); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("old proposal version", func(t *testing.T) {
		f := newConversionFixture(t)
		if _, err := f.pool.Exec(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by) SELECT quote_id,2,'draft',1200,6,now()+interval '2 days',created_by FROM quote_proposals WHERE id=$1`, f.proposalID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID); !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		f := newConversionFixture(t)
		pool2, err := f.schema.OpenPoolWithApplicationName(ctx, "conversion-worker-b")
		if err != nil {
			t.Fatalf("open independent conversion pool: %v", err)
		}
		repo2 := NewCustomerRepository(pool2)
		start := make(chan struct{})
		results := make(chan struct {
			orderReference string
			err            error
		}, 2)
		var wg sync.WaitGroup
		for _, repository := range []*CustomerRepository{f.repo, repo2} {
			wg.Add(1)
			go func(current *CustomerRepository) {
				defer wg.Done()
				<-start
				order, convertErr := current.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
				results <- struct {
					orderReference string
					err            error
				}{order.Reference, convertErr}
			}(repository)
		}
		close(start)
		wg.Wait()
		close(results)
		expected := ""
		for result := range results {
			if result.err != nil {
				t.Fatal(result.err)
			}
			if expected == "" {
				expected = result.orderReference
			} else if result.orderReference != expected {
				t.Fatalf("concurrent conversions returned different order references")
			}
		}
		var count int
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE quote_id=$1`, f.quoteID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("orders=%d", count)
		}
	})
	t.Run("item failure rolls back", func(t *testing.T) {
		f := newConversionFixture(t)
		if _, err := f.pool.Exec(ctx, `UPDATE quotes SET material_id=NULL WHERE id=$1`, f.quoteID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID); err == nil {
			t.Fatal("expected item constraint failure")
		}
		var orders int
		if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE quote_id=$1`, f.quoteID).Scan(&orders); err != nil {
			t.Fatal(err)
		}
		if orders != 0 {
			t.Fatalf("rollback left %d orders", orders)
		}
	})
}
