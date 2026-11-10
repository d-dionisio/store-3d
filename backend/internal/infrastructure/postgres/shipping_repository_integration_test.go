package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	shippingapp "example.com/loja-3d/backend/internal/application/shipping"
	domain "example.com/loja-3d/backend/internal/domain/shipping"
)

func shippingAdmin(t *testing.T, f conversionFixture) (string, string) {
	t.Helper()
	var adminID, sessionID string
	if e := f.pool.QueryRow(context.Background(), `SELECT created_by FROM quote_proposals WHERE id=$1`, f.proposalID).Scan(&adminID); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(context.Background(), `INSERT INTO admin_sessions(admin_user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,encode(sha256(gen_random_uuid()::text::bytea),'hex'),encode(sha256(gen_random_uuid()::text::bytea),'hex'),now()+interval '1 hour') RETURNING id`, adminID).Scan(&sessionID); e != nil {
		t.Fatal(e)
	}
	return adminID, sessionID
}

func TestShippingQuoteSelectionAndSnapshot(t *testing.T) {
	ctx := context.Background()
	f := newConversionFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	var address string
	e = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','30110000','Rua A','1','Centro','Belo Horizonte','MG') RETURNING id`, f.userID).Scan(&address)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.repo.UpdateOrderAddress(ctx, f.userID, order.Reference, address); e != nil {
		t.Fatal(e)
	}
	r := NewShippingRepository(f.pool)
	adminID, adminSession := shippingAdmin(t, f)
	packages := []domain.Package{{Quantity: 1, WeightGrams: 500, LengthMM: 200, WidthMM: 150, HeightMM: 100}}
	if e = r.SavePackages(ctx, adminID, order.Reference, packages, adminSession); e != nil {
		t.Fatal(e)
	}
	req, e := r.LoadRequest(ctx, f.userID, order.Reference, "01001000")
	if e != nil {
		t.Fatal(e)
	}
	quotes, e := r.Save(ctx, req, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", []domain.ProviderQuote{{ServiceCode: "1", CarrierName: "Correios", ServiceName: "PAC", PriceCents: 2590, MinDays: 4, MaxDays: 6, PackageSnapshot: []any{}}}, time.Now().Add(time.Minute))
	if e != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%v err=%v", quotes, e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- r.Select(ctx, f.userID, f.sessionID, order.Reference, quotes[0].ID, time.Now())
		}()
	}
	wg.Wait()
	close(results)
	success, locked := 0, 0
	for e = range results {
		if e == nil {
			success++
		} else if errors.Is(e, shippingapp.ErrOrderLocked) {
			locked++
		} else {
			t.Fatalf("unexpected selection error: %v", e)
		}
	}
	if success != 1 || locked != 1 {
		t.Fatalf("success=%d locked=%d", success, locked)
	}
	var shipping, total int64
	var readiness, carrier string
	e = f.pool.QueryRow(ctx, `SELECT o.shipping_cents,o.total_cents,o.payment_readiness,s.carrier_name FROM orders o JOIN order_shipping s ON s.order_id=o.id WHERE o.order_number=$1`, order.Reference).Scan(&shipping, &total, &readiness, &carrier)
	if e != nil || shipping != 2590 || total != 3591 || readiness != "ready" || carrier != "Correios" {
		t.Fatalf("shipping=%d total=%d readiness=%s carrier=%s err=%v", shipping, total, readiness, carrier, e)
	}
}

func TestShippingExpiredAndForeignQuote(t *testing.T) {
	ctx := context.Background()
	f := newConversionFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	var address string
	e = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','30110000','Rua A','1','Centro','Belo Horizonte','MG') RETURNING id`, f.userID).Scan(&address)
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference)
	if e = f.repo.UpdateOrderAddress(ctx, f.userID, order.Reference, address); e != nil {
		t.Fatal(e)
	}
	r := NewShippingRepository(f.pool)
	adminID, adminSession := shippingAdmin(t, f)
	if e = r.SavePackages(ctx, adminID, order.Reference, []domain.Package{{Quantity: 1, WeightGrams: 500, LengthMM: 200, WidthMM: 150, HeightMM: 100}}, adminSession); e != nil {
		t.Fatal(e)
	}
	req, _ := r.LoadRequest(ctx, f.userID, order.Reference, "01001000")
	quotes, e := r.Save(ctx, req, "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", []domain.ProviderQuote{{ServiceCode: "1", CarrierName: "Correios", ServiceName: "PAC", PriceCents: 1000, MinDays: 1, MaxDays: 2}}, time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Select(ctx, f.userID, f.sessionID, order.Reference, quotes[0].ID, time.Now().Add(2*time.Second)); !errors.Is(e, shippingapp.ErrExpired) {
		t.Fatalf("expected expired, got %v", e)
	}
	if e = r.Select(ctx, f.userID, f.sessionID, "PED-NAO-EXISTE", quotes[0].ID, time.Now()); !errors.Is(e, shippingapp.ErrNotFound) {
		t.Fatalf("expected not found, got %v", e)
	}
}
