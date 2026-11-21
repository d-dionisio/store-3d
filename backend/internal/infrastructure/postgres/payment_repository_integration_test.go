package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	paymentapp "example.com/loja-3d/backend/internal/application/payment"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

// paymentFixture inherits conversionFixture's UUID schema, official migrations,
// schema-only search_path, and schema-scoped cleanup. It deliberately performs
// no table-level cleanup against shared or preexisting rows.
func paymentFixture(t *testing.T) (conversionFixture, string, *PaymentRepository) {
	t.Helper()
	ctx := context.Background()
	f := newConversionFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	return f, order.Reference, NewPaymentRepository(f.pool)
}
func readyPaymentOrder(t *testing.T, f conversionFixture, ref string) {
	t.Helper()
	ctx := context.Background()
	var address string
	if e := f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','01001000','Rua A','1','Centro','São Paulo','SP') RETURNING id`, f.userID).Scan(&address); e != nil {
		t.Fatal(e)
	}
	if e := f.repo.UpdateOrderAddress(ctx, f.userID, ref, address); e != nil {
		t.Fatal(e)
	}
}
func TestPIXReservationReadinessOwnershipAndConcurrency(t *testing.T) {
	ctx := context.Background()
	f, ref, r := paymentFixture(t)
	if _, _, e := r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour)); !errors.Is(e, paymentapp.ErrNotReady) {
		t.Fatalf("not ready err=%v", e)
	}
	readyPaymentOrder(t, f, ref)
	nonOwnerID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique non-owner identifier")
	}
	if _, _, e := r.ReservePIX(ctx, nonOwnerID, ref, time.Now().Add(time.Hour)); !errors.Is(e, paymentapp.ErrNotFound) {
		t.Fatalf("ownership err=%v", e)
	}
	poolA, err := f.schema.OpenPoolWithApplicationName(ctx, "payment-pix-a")
	if err != nil {
		t.Fatalf("open first independent payment pool: %v", err)
	}
	poolB, err := f.schema.OpenPoolWithApplicationName(ctx, "payment-pix-b")
	if err != nil {
		t.Fatalf("open second independent payment pool: %v", err)
	}
	observer, err := f.schema.OpenPoolWithApplicationName(ctx, "payment-pix-observer")
	if err != nil {
		t.Fatalf("open payment concurrency observer: %v", err)
	}
	var pidA, pidB int
	if err = poolA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidA); err != nil {
		t.Fatal("identify first PostgreSQL backend:", err)
	}
	if err = poolB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pidB); err != nil {
		t.Fatal("identify second PostgreSQL backend:", err)
	}
	if pidA == pidB {
		t.Fatalf("payment concurrency participants unexpectedly share backend pid=%d", pidA)
	}
	repoA, repoB := NewPaymentRepository(poolA), NewPaymentRepository(poolB)
	lock, err := observer.Begin(ctx)
	if err != nil {
		t.Fatal("begin deterministic payment concurrency barrier:", err)
	}
	t.Cleanup(func() { _ = lock.Rollback(ctx) })
	var orderID string
	if err = lock.QueryRow(ctx, `SELECT id::text FROM orders WHERE order_number=$1 FOR UPDATE`, ref).Scan(&orderID); err != nil {
		t.Fatal("lock isolated payment order:", err)
	}
	var wg sync.WaitGroup
	results := make(chan struct {
		attemptID string
		err       error
	}, 2)
	start := make(chan struct{})
	for _, repo := range []*PaymentRepository{repoA, repoB} {
		wg.Add(1)
		go func(repo *PaymentRepository) {
			defer wg.Done()
			<-start
			req, existing, e := repo.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
			result := struct {
				attemptID string
				err       error
			}{err: e}
			if existing != nil {
				result.attemptID = existing.AttemptID
			} else {
				result.attemptID = req.AttemptID
			}
			results <- result
		}(repo)
	}
	close(start)
	waitForLockWaiters(t, ctx, observer, "payment-pix-a", "payment-pix-b")
	if err = lock.Commit(ctx); err != nil {
		t.Fatal("release payment order lock:", err)
	}
	wg.Wait()
	close(results)
	first := ""
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if first == "" {
			first = result.attemptID
		} else if result.attemptID != first {
			t.Fatalf("attempts differ: %s %s", first, result.attemptID)
		}
	}
	t.Logf("payment reservation race used independent PostgreSQL backends pidA=%d pidB=%d", pidA, pidB)
	var payments, attempts int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&payments); err != nil {
		t.Fatal("count isolated payments:", err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_attempts a JOIN payments p ON p.id=a.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&attempts); err != nil {
		t.Fatal("count isolated payment attempts:", err)
	}
	if payments != 1 || attempts != 1 {
		t.Fatalf("payments=%d attempts=%d", payments, attempts)
	}
}
func TestPIXCompletionWebhookApprovalDedupOutOfOrderAndRefund(t *testing.T) {
	ctx := context.Background()
	f, ref, r := paymentFixture(t)
	readyPaymentOrder(t, f, ref)
	req, _, e := r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now().UTC()
	externalPaymentID := "test-pix-" + ref
	pix, e := r.CompletePIX(ctx, req, domain.ProviderPayment{ID: externalPaymentID, Status: "pending", StatusDetail: "pending_waiting_transfer", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, QRCode: "000201", QRCodeBase64: "abc", TicketURL: "https://example.test", UpdatedAt: &now})
	if e != nil || pix.QRCode == "" {
		t.Fatalf("pix=%+v err=%v", pix, e)
	}
	w := domain.Webhook{EventID: "event-approved-" + ref, EventType: "payment.updated", ExternalPaymentID: externalPaymentID, Payload: []byte(`{"id":1,"type":"payment"}`)}
	fresh, e := r.RecordWebhook(ctx, w)
	if e != nil || !fresh {
		t.Fatalf("record=%v fresh=%v", e, fresh)
	}
	fresh, e = r.RecordWebhook(ctx, w)
	if e != nil || fresh {
		t.Fatalf("duplicate=%v fresh=%v", e, fresh)
	}
	approved := now.Add(time.Minute)
	e = r.Reconcile(ctx, w, domain.ProviderPayment{ID: externalPaymentID, Status: "approved", StatusDetail: "accredited", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, ApprovedAt: &approved, UpdatedAt: &approved})
	if e != nil {
		t.Fatal(e)
	}
	var orderStatus, paymentStatus string
	var events, productionJobs int
	e = f.pool.QueryRow(ctx, `SELECT o.status,p.status FROM orders o JOIN payments p ON p.order_id=o.id WHERE o.order_number=$1`, ref).Scan(&orderStatus, &paymentStatus)
	if e != nil || orderStatus != "confirmed" || paymentStatus != "paid" {
		t.Fatalf("order=%s payment=%s err=%v", orderStatus, paymentStatus, e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events e JOIN orders o ON o.id=e.order_id WHERE o.order_number=$1 AND e.status='confirmed'`, ref).Scan(&events); e != nil {
		t.Fatal("count confirmed order events:", e)
	}
	if events != 1 {
		t.Fatalf("confirmed events=%d", events)
	}
	// Reconciliation retries must repair missing work and remain idempotent.
	if e = r.Reconcile(ctx, w, domain.ProviderPayment{ID: externalPaymentID, Status: "approved", StatusDetail: "accredited", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, ApprovedAt: &approved, UpdatedAt: &approved}); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id JOIN orders o ON o.id=oi.order_id WHERE o.order_number=$1 AND pj.parent_job_id IS NULL`, ref).Scan(&productionJobs); e != nil || productionJobs != 1 {
		t.Fatalf("production jobs=%d err=%v", productionJobs, e)
	}
	old := now
	wOld := domain.Webhook{EventID: "event-old-" + ref, EventType: "payment.updated", ExternalPaymentID: externalPaymentID, Payload: []byte(`{"id":2,"type":"payment"}`)}
	if fresh, recordErr := r.RecordWebhook(ctx, wOld); recordErr != nil || !fresh {
		t.Fatalf("record out-of-order webhook: fresh=%v err=%v", fresh, recordErr)
	}
	if e = r.Reconcile(ctx, wOld, domain.ProviderPayment{ID: externalPaymentID, Status: "pending", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, UpdatedAt: &old}); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT p.status FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&paymentStatus); e != nil {
		t.Fatal("read payment after out-of-order webhook:", e)
	}
	if paymentStatus != "paid" {
		t.Fatalf("out of order regressed to %s", paymentStatus)
	}
	refundAt := approved.Add(time.Minute)
	wRefund := domain.Webhook{EventID: "event-refund-" + ref, EventType: "payment.updated", ExternalPaymentID: externalPaymentID, Payload: []byte(`{"id":3,"type":"payment"}`)}
	if fresh, recordErr := r.RecordWebhook(ctx, wRefund); recordErr != nil || !fresh {
		t.Fatalf("record refund webhook: fresh=%v err=%v", fresh, recordErr)
	}
	if e = r.Reconcile(ctx, wRefund, domain.ProviderPayment{ID: externalPaymentID, Status: "refunded", StatusDetail: "refunded", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, RefundedCents: req.AmountCents, ApprovedAt: &approved, UpdatedAt: &refundAt}); e != nil {
		t.Fatal(e)
	}
	var refunded int64
	var refunds int
	if e = f.pool.QueryRow(ctx, `SELECT p.status,p.refunded_amount_cents FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&paymentStatus, &refunded); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_refunds pr JOIN payments p ON p.id=pr.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&refunds); e != nil {
		t.Fatal(e)
	}
	if paymentStatus != "refunded" || refunded != req.AmountCents || refunds != 1 {
		t.Fatalf("status=%s refunded=%d rows=%d", paymentStatus, refunded, refunds)
	}
}
func TestCardReservationCompletionAndMethodConflict(t *testing.T) {
	ctx := context.Background()
	f, ref, r := paymentFixture(t)
	readyPaymentOrder(t, f, ref)
	input := domain.CardInput{Token: "transient-card-token", PaymentMethodID: "master", Installments: 3, IdempotencyKey: "card-" + ref}
	req, existing, err := r.ReserveCard(ctx, f.userID, ref, input)
	if err != nil || existing != nil || req.Token != "transient-card-token" {
		t.Fatalf("req=%+v existing=%+v err=%v", req, existing, err)
	}
	now := time.Now().UTC()
	card, err := r.CompleteCard(ctx, req, domain.ProviderPayment{ID: "ORDCARD" + ref, Status: "pending", StatusDetail: "pending_challenge", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, CardBrand: "master", CardLastFour: "1234", AuthenticationURL: "https://www.mercadopago.com/auth/card/challenge", UpdatedAt: &now})
	if err != nil || !card.RequiresAction || card.LastFour != "1234" {
		t.Fatalf("card=%+v err=%v", card, err)
	}
	var storedToken string
	err = f.pool.QueryRow(ctx, `SELECT coalesce(card_payment_method_id,'') FROM payment_attempts WHERE id=$1`, req.AttemptID).Scan(&storedToken)
	if err != nil || storedToken != "master" {
		t.Fatalf("method=%s err=%v", storedToken, err)
	}
	_, _, err = r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
	if !errors.Is(err, paymentapp.ErrConflict) {
		t.Fatalf("expected method conflict, got %v", err)
	}
}
func TestPaymentValueMismatchRollsBack(t *testing.T) {
	ctx := context.Background()
	f, ref, r := paymentFixture(t)
	readyPaymentOrder(t, f, ref)
	req, _, e := r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	externalPaymentID := "test-pix-mismatch-" + ref
	_, e = r.CompletePIX(ctx, req, domain.ProviderPayment{ID: externalPaymentID, Status: "pending", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, UpdatedAt: &now})
	if e != nil {
		t.Fatal(e)
	}
	w := domain.Webhook{EventID: "event-wrong-" + ref, EventType: "payment.updated", ExternalPaymentID: externalPaymentID, Payload: []byte(`{"id":4}`)}
	if fresh, recordErr := r.RecordWebhook(ctx, w); recordErr != nil || !fresh {
		t.Fatalf("record mismatch webhook: fresh=%v err=%v", fresh, recordErr)
	}
	e = r.Reconcile(ctx, w, domain.ProviderPayment{ID: externalPaymentID, Status: "approved", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents + 1, UpdatedAt: &now})
	if !errors.Is(e, paymentapp.ErrValueMismatch) {
		t.Fatalf("err=%v", e)
	}
	var status string
	if e = f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE order_number=$1`, ref).Scan(&status); e != nil {
		t.Fatal("read order after value mismatch rollback:", e)
	}
	if status != "pending_payment" {
		t.Fatalf("order changed: %s", status)
	}
}
