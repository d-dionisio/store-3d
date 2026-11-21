package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	shippingapp "example.com/loja-3d/backend/internal/application/shipping"
	domain "example.com/loja-3d/backend/internal/domain/shipping"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

// newShippingFixture reuses the isolated conversion fixture so the official
// migration set, UUID schema, schema-only search_path, and cleanup policy stay
// centralized in TestSupport-backed setup.
func newShippingFixture(t *testing.T) conversionFixture {
	t.Helper()
	return newConversionFixture(t)
}

func shippingRequestHash(t *testing.T) string {
	t.Helper()
	id, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique shipping request hash")
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

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
	f := newShippingFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	var address string
	e = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','30110000','Rua A','1','Centro','Belo Horizonte','MG') RETURNING id`, f.userID).Scan(&address)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference); e != nil {
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
	requestHash := shippingRequestHash(t)
	quotes, e := r.Save(ctx, req, requestHash, []domain.ProviderQuote{{ServiceCode: "1", CarrierName: "Correios", ServiceName: "PAC", PriceCents: 2590, MinDays: 4, MaxDays: 6, PackageSnapshot: []any{}}}, time.Now().Add(time.Minute))
	if e != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%v err=%v", quotes, e)
	}
	repeated, e := r.Save(ctx, req, requestHash, []domain.ProviderQuote{{ServiceCode: "1", CarrierName: "Correios", ServiceName: "PAC atualizado", PriceCents: 9999, MinDays: 1, MaxDays: 1}}, time.Now().Add(time.Minute))
	if e != nil || len(repeated) != 1 || repeated[0].ID != quotes[0].ID || repeated[0].PriceCents != quotes[0].PriceCents {
		t.Fatalf("cached shipping quote changed on retry: first=%+v retry=%+v err=%v", quotes, repeated, e)
	}
	poolA, e := f.schema.OpenPoolWithApplicationName(ctx, "shipping-select-a")
	if e != nil {
		t.Fatalf("open first independent shipping pool: %v", e)
	}
	poolB, e := f.schema.OpenPoolWithApplicationName(ctx, "shipping-select-b")
	if e != nil {
		t.Fatalf("open second independent shipping pool: %v", e)
	}
	observer, e := f.schema.OpenPoolWithApplicationName(ctx, "shipping-select-observer")
	if e != nil {
		t.Fatalf("open shipping concurrency observer: %v", e)
	}
	var pidA, pidB int
	if e = poolA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidA); e != nil {
		t.Fatal("identify first PostgreSQL backend:", e)
	}
	if e = poolB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidB); e != nil {
		t.Fatal("identify second PostgreSQL backend:", e)
	}
	if pidA == pidB {
		t.Fatalf("shipping concurrency participants unexpectedly share backend pid=%d", pidA)
	}
	lock, e := observer.Begin(ctx)
	if e != nil {
		t.Fatal("begin deterministic shipping selection barrier:", e)
	}
	t.Cleanup(func() { _ = lock.Rollback(ctx) })
	var orderID string
	if e = lock.QueryRow(ctx, `SELECT id::text FROM orders WHERE order_number=$1 FOR UPDATE`, order.Reference).Scan(&orderID); e != nil {
		t.Fatal("lock isolated shipping order:", e)
	}
	repoA, repoB := NewShippingRepository(poolA), NewShippingRepository(poolB)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for _, repo := range []*ShippingRepository{repoA, repoB} {
		wg.Add(1)
		go func(repo *ShippingRepository) {
			defer wg.Done()
			<-start
			results <- repo.Select(ctx, f.userID, f.sessionID, order.Reference, quotes[0].ID, time.Now())
		}(repo)
	}
	close(start)
	waitForLockWaiters(t, ctx, observer, "shipping-select-a", "shipping-select-b")
	if e = lock.Commit(ctx); e != nil {
		t.Fatal("release shipping order lock:", e)
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
	t.Logf("shipping selection race used independent PostgreSQL backends pidA=%d pidB=%d", pidA, pidB)
	var shipping, total int64
	var readiness, carrier string
	e = f.pool.QueryRow(ctx, `SELECT o.shipping_cents,o.total_cents,o.payment_readiness,s.carrier_name FROM orders o JOIN order_shipping s ON s.order_id=o.id WHERE o.order_number=$1`, order.Reference).Scan(&shipping, &total, &readiness, &carrier)
	if e != nil || shipping != 2590 || total != 3591 || readiness != "ready" || carrier != "Correios" {
		t.Fatalf("shipping=%d total=%d readiness=%s carrier=%s err=%v", shipping, total, readiness, carrier, e)
	}
}

func TestShippingExpiredAndForeignQuote(t *testing.T) {
	ctx := context.Background()
	f := newShippingFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	var address string
	e = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','30110000','Rua A','1','Centro','Belo Horizonte','MG') RETURNING id`, f.userID).Scan(&address)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference); e != nil {
		t.Fatal("disable included shipping for isolated order:", e)
	}
	if e = f.repo.UpdateOrderAddress(ctx, f.userID, order.Reference, address); e != nil {
		t.Fatal(e)
	}
	r := NewShippingRepository(f.pool)
	adminID, adminSession := shippingAdmin(t, f)
	if e = r.SavePackages(ctx, adminID, order.Reference, []domain.Package{{Quantity: 1, WeightGrams: 500, LengthMM: 200, WidthMM: 150, HeightMM: 100}}, adminSession); e != nil {
		t.Fatal(e)
	}
	req, e := r.LoadRequest(ctx, f.userID, order.Reference, "01001000")
	if e != nil {
		t.Fatal("load isolated expired-quote request:", e)
	}
	quotes, e := r.Save(ctx, req, shippingRequestHash(t), []domain.ProviderQuote{{ServiceCode: "1", CarrierName: "Correios", ServiceName: "PAC", PriceCents: 1000, MinDays: 1, MaxDays: 2}}, time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Select(ctx, f.userID, f.sessionID, order.Reference, quotes[0].ID, time.Now().Add(2*time.Second)); !errors.Is(e, shippingapp.ErrExpired) {
		t.Fatalf("expected expired, got %v", e)
	}
	missingReference, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique missing-order reference")
	}
	if e = r.Select(ctx, f.userID, f.sessionID, missingReference, quotes[0].ID, time.Now()); !errors.Is(e, shippingapp.ErrNotFound) {
		t.Fatalf("expected not found, got %v", e)
	}
}
