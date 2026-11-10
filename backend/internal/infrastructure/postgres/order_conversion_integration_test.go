package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/customer"
	"github.com/jackc/pgx/v5/pgxpool"
)

var conversionSequence atomic.Int64

type conversionFixture struct {
	pool                                              *pgxpool.Pool
	repo                                              *CustomerRepository
	userID, sessionID, quoteID, proposalID, reference string
}

func newConversionFixture(t *testing.T) conversionFixture {
	t.Helper()
	ctx := context.Background()
	pool := adminPool(t)
	suffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), conversionSequence.Add(1))
	email := "conversion-" + suffix + "@example.com"
	var material, color, adminID, userID, sessionID, quoteID, proposalID string
	if err := pool.QueryRow(ctx, `SELECT id FROM materials ORDER BY created_at LIMIT 1`).Scan(&material); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM colors ORDER BY created_at LIMIT 1`).Scan(&color); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'Conversion Admin',repeat('x',60),'admin') RETURNING id`, "conversion-admin-"+suffix+"@example.com").Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Cliente Conversão',now()) RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,encode(sha256($2::bytea),'hex'),encode(sha256(($2||'-csrf')::bytea),'hex'),now()+interval '1 hour') RETURNING id`, userID, suffix).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}
	reference := "ORC-CONV-" + suffix
	if err := pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,user_id,customer_name,customer_email,customer_phone,status,material_id,color_id,requested_quantity,width_mm,height_mm,depth_mm,notes,quoted_amount_cents,estimated_lead_time_days,valid_until,submitted_at,quoted_at,approved_at) VALUES($1,$2,'Cliente Conversão',$3,'11999999999','approved',$4,$5,3,10,20,30,'Nota técnica',1001,5,now()+interval '2 days',now(),now(),now()) RETURNING id`, reference, userID, email, material, color).Scan(&quoteID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,security_checked_at,retention_until) VALUES($1,'r2','private',$2,'conversion.stl','stl','model/stl',10,repeat('a',64),'clean',now(),now()+interval '1 day')`, quoteID, "conversion-"+suffix); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO quote_proposals(quote_id,version,status,material_cost_cents,final_price_cents,production_lead_time_days,valid_until,commercial_notes,shipping_included,created_by,sent_by,sent_at) VALUES($1,1,'sent',700,1001,5,now()+interval '2 days','Condição comercial preservada',true,$2,$2,now()) RETURNING id`, quoteID, adminID).Scan(&proposalID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO quote_proposal_decisions(quote_id,proposal_id,user_id,decision,decided_at) VALUES($1,$2,$3,'accepted',now())`, quoteID, proposalID, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customer_audit_events WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quote_proposal_decisions WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quote_proposals WHERE quote_id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM quotes WHERE id=$1`, quoteID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id=$1`, adminID)
	})
	return conversionFixture{pool, NewCustomerRepository(pool), userID, sessionID, quoteID, proposalID, reference}
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
		if status != "pending_payment" || proposalVersion != 1 || lead != 5 || subtotal != 1001 || total != 1001 || !shipping || commercial != "Condição comercial preservada" || quantity != 3 || approvedTotal != 1001 || lineTotal != 1001 || material == "" || color == "" || dimensions != "10 × 20 × 30" {
			t.Fatalf("snapshots invalid: %s %d %d %d %d %v %q %d %d %d %q %q %q", status, proposalVersion, lead, subtotal, total, shipping, commercial, quantity, approvedTotal, lineTotal, material, color, dimensions)
		}
		var events, audits int
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events e JOIN orders o ON o.id=e.order_id WHERE o.quote_id=$1 AND e.status='pending_payment'`, f.quoteID).Scan(&events)
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM customer_audit_events WHERE user_id=$1 AND action='quote.convert'`, f.userID).Scan(&audits)
		if events != 1 || audits != 1 {
			t.Fatalf("events=%d audits=%d", events, audits)
		}
	})
	t.Run("not approved", func(t *testing.T) {
		f := newConversionFixture(t)
		_, _ = f.pool.Exec(ctx, `UPDATE quotes SET status='quoted',approved_at=NULL WHERE id=$1`, f.quoteID)
		_, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("expired at approval", func(t *testing.T) {
		f := newConversionFixture(t)
		_, _ = f.pool.Exec(ctx, `UPDATE quote_proposals SET created_at=now()-interval '3 days',valid_until=now()-interval '2 days' WHERE id=$1`, f.proposalID)
		_, _ = f.pool.Exec(ctx, `UPDATE quote_proposal_decisions SET decided_at=now()-interval '1 day' WHERE proposal_id=$1`, f.proposalID)
		_, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("different owner", func(t *testing.T) {
		f := newConversionFixture(t)
		var other string
		_ = f.pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,'Outro',now()) RETURNING id`, "other-"+f.reference+"@example.com").Scan(&other)
		t.Cleanup(func() { _, _ = f.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, other) })
		_, err := f.repo.ConvertQuote(ctx, other, f.reference, f.sessionID)
		if !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("old proposal version", func(t *testing.T) {
		f := newConversionFixture(t)
		_, err := f.pool.Exec(ctx, `INSERT INTO quote_proposals(quote_id,version,status,final_price_cents,production_lead_time_days,valid_until,created_by) SELECT quote_id,2,'draft',1200,6,now()+interval '2 days',created_by FROM quote_proposals WHERE id=$1`, f.proposalID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if !errors.Is(err, app.ErrInvalidConversion) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		f := newConversionFixture(t)
		var wg sync.WaitGroup
		results := make(chan error, 2)
		refs := make(chan string, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
				results <- e
				refs <- v.Reference
			}()
		}
		wg.Wait()
		close(results)
		close(refs)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		var expected string
		for ref := range refs {
			if expected == "" {
				expected = ref
			} else if ref != expected {
				t.Fatalf("different orders %s %s", expected, ref)
			}
		}
		var count int
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE quote_id=$1`, f.quoteID).Scan(&count)
		if count != 1 {
			t.Fatalf("orders=%d", count)
		}
	})
	t.Run("item failure rolls back", func(t *testing.T) {
		f := newConversionFixture(t)
		_, _ = f.pool.Exec(ctx, `UPDATE quotes SET material_id=NULL WHERE id=$1`, f.quoteID)
		_, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
		if err == nil {
			t.Fatal("expected item constraint failure")
		}
		var orders int
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE quote_id=$1`, f.quoteID).Scan(&orders)
		if orders != 0 {
			t.Fatalf("rollback left %d orders", orders)
		}
	})
}
