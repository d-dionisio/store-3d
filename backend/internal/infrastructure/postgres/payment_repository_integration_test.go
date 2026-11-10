package postgres

import (
	"context"
	"errors"
	paymentapp "example.com/loja-3d/backend/internal/application/payment"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"sync"
	"testing"
	"time"
)

func paymentFixture(t *testing.T) (conversionFixture, string, *PaymentRepository) {
	t.Helper()
	ctx := context.Background()
	f := newConversionFixture(t)
	_, _ = f.pool.Exec(ctx, `DELETE FROM payment_refunds WHERE payment_id IN (SELECT payment_id FROM payment_attempts WHERE external_payment_id IN ('900001','900002'))`)
	_, _ = f.pool.Exec(ctx, `DELETE FROM payments WHERE id IN (SELECT payment_id FROM payment_attempts WHERE external_payment_id IN ('900001','900002'))`)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(ctx, `DELETE FROM payment_webhook_events WHERE external_event_id LIKE $1`, "%"+order.Reference)
		_, _ = f.pool.Exec(ctx, `DELETE FROM payment_refunds WHERE payment_id IN (SELECT id FROM payments WHERE order_id=(SELECT id FROM orders WHERE order_number=$1))`, order.Reference)
		_, _ = f.pool.Exec(ctx, `DELETE FROM payments WHERE order_id=(SELECT id FROM orders WHERE order_number=$1)`, order.Reference)
	})
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
	if _, _, e := r.ReservePIX(ctx, "00000000-0000-0000-0000-000000000000", ref, time.Now().Add(time.Hour)); !errors.Is(e, paymentapp.ErrNotFound) {
		t.Fatalf("ownership err=%v", e)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, existing, e := r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
			if existing != nil {
				ids <- existing.AttemptID
			} else {
				ids <- req.AttemptID
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		} else if id != first {
			t.Fatalf("attempts differ: %s %s", first, id)
		}
	}
	var payments, attempts int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&payments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM payment_attempts a JOIN payments p ON p.id=a.payment_id JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&attempts)
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
	pix, e := r.CompletePIX(ctx, req, domain.ProviderPayment{ID: "900001", Status: "pending", StatusDetail: "pending_waiting_transfer", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, QRCode: "000201", QRCodeBase64: "abc", TicketURL: "https://example.test", UpdatedAt: &now})
	if e != nil || pix.QRCode == "" {
		t.Fatalf("pix=%+v err=%v", pix, e)
	}
	w := domain.Webhook{EventID: "event-approved-" + ref, EventType: "payment.updated", ExternalPaymentID: "900001", Payload: []byte(`{"id":1,"type":"payment"}`)}
	fresh, e := r.RecordWebhook(ctx, w)
	if e != nil || !fresh {
		t.Fatalf("record=%v fresh=%v", e, fresh)
	}
	fresh, e = r.RecordWebhook(ctx, w)
	if e != nil || fresh {
		t.Fatalf("duplicate=%v fresh=%v", e, fresh)
	}
	approved := now.Add(time.Minute)
	e = r.Reconcile(ctx, w, domain.ProviderPayment{ID: "900001", Status: "approved", StatusDetail: "accredited", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, ApprovedAt: &approved, UpdatedAt: &approved})
	if e != nil {
		t.Fatal(e)
	}
	var orderStatus, paymentStatus string
	var events int
	e = f.pool.QueryRow(ctx, `SELECT o.status,p.status FROM orders o JOIN payments p ON p.order_id=o.id WHERE o.order_number=$1`, ref).Scan(&orderStatus, &paymentStatus)
	if e != nil || orderStatus != "confirmed" || paymentStatus != "paid" {
		t.Fatalf("order=%s payment=%s err=%v", orderStatus, paymentStatus, e)
	}
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM order_status_events e JOIN orders o ON o.id=e.order_id WHERE o.order_number=$1 AND e.status='confirmed'`, ref).Scan(&events)
	if events != 1 {
		t.Fatalf("confirmed events=%d", events)
	}
	old := now
	wOld := domain.Webhook{EventID: "event-old-" + ref, EventType: "payment.updated", ExternalPaymentID: "900001", Payload: []byte(`{"id":2,"type":"payment"}`)}
	_, _ = r.RecordWebhook(ctx, wOld)
	if e = r.Reconcile(ctx, wOld, domain.ProviderPayment{ID: "900001", Status: "pending", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, UpdatedAt: &old}); e != nil {
		t.Fatal(e)
	}
	_ = f.pool.QueryRow(ctx, `SELECT status FROM payments p JOIN orders o ON o.id=p.order_id WHERE o.order_number=$1`, ref).Scan(&paymentStatus)
	if paymentStatus != "paid" {
		t.Fatalf("out of order regressed to %s", paymentStatus)
	}
	refundAt := approved.Add(time.Minute)
	wRefund := domain.Webhook{EventID: "event-refund-" + ref, EventType: "payment.updated", ExternalPaymentID: "900001", Payload: []byte(`{"id":3,"type":"payment"}`)}
	_, _ = r.RecordWebhook(ctx, wRefund)
	if e = r.Reconcile(ctx, wRefund, domain.ProviderPayment{ID: "900001", Status: "refunded", StatusDetail: "refunded", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, RefundedCents: req.AmountCents, ApprovedAt: &approved, UpdatedAt: &refundAt}); e != nil {
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
func TestPaymentValueMismatchRollsBack(t *testing.T) {
	ctx := context.Background()
	f, ref, r := paymentFixture(t)
	readyPaymentOrder(t, f, ref)
	req, _, e := r.ReservePIX(ctx, f.userID, ref, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	_, e = r.CompletePIX(ctx, req, domain.ProviderPayment{ID: "900002", Status: "pending", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents, UpdatedAt: &now})
	if e != nil {
		t.Fatal(e)
	}
	w := domain.Webhook{EventID: "event-wrong-" + ref, EventType: "payment.updated", ExternalPaymentID: "900002", Payload: []byte(`{"id":4}`)}
	_, _ = r.RecordWebhook(ctx, w)
	e = r.Reconcile(ctx, w, domain.ProviderPayment{ID: "900002", Status: "approved", Currency: "BRL", ExternalReference: ref, AmountCents: req.AmountCents + 1, UpdatedAt: &now})
	if !errors.Is(e, paymentapp.ErrValueMismatch) {
		t.Fatalf("err=%v", e)
	}
	var status string
	_ = f.pool.QueryRow(ctx, `SELECT status FROM orders WHERE order_number=$1`, ref).Scan(&status)
	if status != "pending_payment" {
		t.Fatalf("order changed: %s", status)
	}
}
