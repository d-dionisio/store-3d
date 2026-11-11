package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	app "example.com/loja-3d/backend/internal/application/payment"
	domain "example.com/loja-3d/backend/internal/domain/payment"
	"github.com/jackc/pgx/v5"
	"time"
)

type PaymentRepository struct{ pool DBTXPool }

func NewPaymentRepository(pool DBTXPool) *PaymentRepository { return &PaymentRepository{pool} }

func (r *PaymentRepository) ReserveCard(ctx context.Context, userID, ref string, input domain.CardInput) (domain.CreateCardRequest, *domain.Card, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	defer tx.Rollback(ctx)
	var orderID, email, readiness, status, currency string
	var amount int64
	err = tx.QueryRow(ctx, `SELECT id,customer_email,payment_readiness,status,currency,total_cents FROM orders WHERE user_id=$1 AND order_number=$2 FOR UPDATE`, userID, ref).Scan(&orderID, &email, &readiness, &status, &currency, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CreateCardRequest{}, nil, app.ErrNotFound
	}
	if err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	if status != "pending_payment" || readiness != "ready" || currency != "BRL" || amount <= 0 {
		return domain.CreateCardRequest{}, nil, app.ErrNotReady
	}
	var paymentID, attemptID, method, external, state, detail, brand, last4, authURL string
	var installments int
	err = tx.QueryRow(ctx, `SELECT p.id,a.id,a.method,coalesce(a.external_payment_id,''),a.status,coalesce(a.provider_status_detail,''),coalesce(a.card_brand,''),coalesce(a.card_last_four,''),coalesce(a.card_authentication_url,''),coalesce(a.card_installments,0) FROM payments p JOIN payment_attempts a ON a.payment_id=p.id WHERE p.order_id=$1 AND p.status IN ('pending','processing','authorized','paid','partially_refunded') ORDER BY a.created_at DESC LIMIT 1`, orderID).Scan(&paymentID, &attemptID, &method, &external, &state, &detail, &brand, &last4, &authURL, &installments)
	if err == nil {
		if method != "card" {
			return domain.CreateCardRequest{}, nil, app.ErrConflict
		}
		if external != "" {
			card := domain.Card{AttemptID: attemptID, Status: state, StatusDetail: detail, Brand: brand, LastFour: last4, AuthenticationURL: authURL, AmountCents: amount, Installments: installments, RequiresAction: authURL != "" && state == "pending"}
			if err = tx.Commit(ctx); err != nil {
				return domain.CreateCardRequest{}, nil, err
			}
			return domain.CreateCardRequest{}, &card, nil
		}
		return domain.CreateCardRequest{}, nil, app.ErrConflict
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.CreateCardRequest{}, nil, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,currency) VALUES($1,'card','pending',$2,'BRL') RETURNING id`, orderID, amount).Scan(&paymentID)
	if err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO payment_attempts(payment_id,provider,method,idempotency_key,status,amount_cents,card_installments,card_payment_method_id) VALUES($1,'mercado_pago','card',$2,'pending',$3,$4,$5) RETURNING id`, paymentID, input.IdempotencyKey, amount, input.Installments, input.PaymentMethodID).Scan(&attemptID)
	if err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO payment_status_events(payment_id,payment_attempt_id,new_status,source,metadata) VALUES($1,$2,'pending','system',jsonb_build_object('order_reference',$3::text,'method','card'))`, paymentID, attemptID, ref)
	if err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.CreateCardRequest{}, nil, err
	}
	return domain.CreateCardRequest{AttemptID: attemptID, IdempotencyKey: input.IdempotencyKey, OrderReference: ref, PayerEmail: email, Token: input.Token, PaymentMethodID: input.PaymentMethodID, IdentificationType: input.IdentificationType, IdentificationNumber: input.IdentificationNumber, AmountCents: amount, Installments: input.Installments}, nil, nil
}

func (r *PaymentRepository) ReservePIX(ctx context.Context, userID, ref string, expires time.Time) (domain.CreateRequest, *domain.PIX, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.CreateRequest{}, nil, e
	}
	defer tx.Rollback(ctx)
	var orderID, email, readiness, status, currency string
	var amount int64
	e = tx.QueryRow(ctx, `SELECT id,customer_email,payment_readiness,status,currency,total_cents FROM orders WHERE user_id=$1 AND order_number=$2 FOR UPDATE`, userID, ref).Scan(&orderID, &email, &readiness, &status, &currency, &amount)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.CreateRequest{}, nil, app.ErrNotFound
	}
	if e != nil {
		return domain.CreateRequest{}, nil, e
	}
	if status != "pending_payment" || readiness != "ready" || currency != "BRL" || amount <= 0 {
		return domain.CreateRequest{}, nil, app.ErrNotReady
	}
	var paymentID, attemptID, key, external, attemptStatus, method, qr, qr64, ticket string
	var attemptExpires *time.Time
	e = tx.QueryRow(ctx, `SELECT p.id,a.id,a.idempotency_key,coalesce(a.external_payment_id,''),a.status,a.method,coalesce(a.pix_qr_code,''),coalesce(a.pix_qr_code_base64,''),coalesce(a.pix_ticket_url,''),a.pix_expires_at FROM payments p JOIN payment_attempts a ON a.payment_id=p.id WHERE p.order_id=$1 AND p.status IN ('pending','processing','authorized','paid','partially_refunded') ORDER BY a.created_at DESC LIMIT 1`, orderID).Scan(&paymentID, &attemptID, &key, &external, &attemptStatus, &method, &qr, &qr64, &ticket, &attemptExpires)
	if e == nil && method != "pix" {
		return domain.CreateRequest{}, nil, app.ErrConflict
	}
	if e == nil {
		if external != "" && attemptExpires != nil && !attemptExpires.After(time.Now()) && (attemptStatus == "pending" || attemptStatus == "processing") {
			if _, e = tx.Exec(ctx, `UPDATE payment_attempts SET status='expired',updated_at=now() WHERE id=$1`, attemptID); e != nil {
				return domain.CreateRequest{}, nil, e
			}
			if _, e = tx.Exec(ctx, `UPDATE payments SET status='cancelled',updated_at=now() WHERE id=$1`, paymentID); e != nil {
				return domain.CreateRequest{}, nil, e
			}
			e = pgx.ErrNoRows
		}
	}
	if e == nil {
		if external != "" {
			pix := domain.PIX{AttemptID: attemptID, Status: attemptStatus, QRCode: qr, QRCodeBase64: qr64, TicketURL: ticket, AmountCents: amount, ExpiresAt: attemptExpires}
			if e = tx.Commit(ctx); e != nil {
				return domain.CreateRequest{}, nil, e
			}
			return domain.CreateRequest{}, &pix, nil
		}
		if e = tx.Commit(ctx); e != nil {
			return domain.CreateRequest{}, nil, e
		}
		return domain.CreateRequest{AttemptID: attemptID, IdempotencyKey: key, OrderReference: ref, PayerEmail: email, AmountCents: amount, ExpiresAt: expires}, nil, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.CreateRequest{}, nil, e
	}
	e = tx.QueryRow(ctx, `INSERT INTO payments(order_id,method,status,amount_cents,currency) VALUES($1,'pix','pending',$2,'BRL') RETURNING id`, orderID, amount).Scan(&paymentID)
	if e != nil {
		return domain.CreateRequest{}, nil, e
	}
	e = tx.QueryRow(ctx, `INSERT INTO payment_attempts(payment_id,provider,method,idempotency_key,status,amount_cents,pix_expires_at) VALUES($1,'mercado_pago','pix',gen_random_uuid()::text,'pending',$2,$3) RETURNING id,idempotency_key`, paymentID, amount, expires).Scan(&attemptID, &key)
	if e != nil {
		return domain.CreateRequest{}, nil, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO payment_status_events(payment_id,payment_attempt_id,new_status,source,metadata) VALUES($1,$2,'pending','system',jsonb_build_object('order_reference',$3::text))`, paymentID, attemptID, ref)
	if e != nil {
		return domain.CreateRequest{}, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.CreateRequest{}, nil, e
	}
	return domain.CreateRequest{AttemptID: attemptID, IdempotencyKey: key, OrderReference: ref, PayerEmail: email, AmountCents: amount, ExpiresAt: expires}, nil, nil
}

func internalStatus(provider string, refunded int64) string {
	switch provider {
	case "pending":
		return "pending"
	case "in_process":
		return "processing"
	case "authorized":
		return "authorized"
	case "approved":
		if refunded > 0 {
			return "partially_refunded"
		}
		return "paid"
	case "rejected":
		return "failed"
	case "cancelled":
		return "cancelled"
	case "refunded", "charged_back":
		return "refunded"
	default:
		return "processing"
	}
}
func attemptStatus(provider, detail string, refunded int64) string {
	if provider == "cancelled" && detail == "expired" {
		return "expired"
	}
	status := internalStatus(provider, refunded)
	if status == "partially_refunded" || status == "refunded" {
		return "paid"
	}
	return status
}
func (r *PaymentRepository) CompletePIX(ctx context.Context, req domain.CreateRequest, p domain.ProviderPayment) (domain.PIX, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.PIX{}, e
	}
	defer tx.Rollback(ctx)
	status := attemptStatus(p.Status, p.StatusDetail, p.RefundedCents)
	paymentStatus := internalStatus(p.Status, p.RefundedCents)
	expires := p.ExpiresAt
	if expires == nil {
		expires = &req.ExpiresAt
	}
	tag, e := tx.Exec(ctx, `UPDATE payment_attempts SET external_payment_id=$2,status=$3,provider_status=$4,provider_status_detail=nullif($5,''),pix_qr_code=nullif($6,''),pix_qr_code_base64=nullif($7,''),pix_ticket_url=nullif($8,''),pix_expires_at=$9,provider_updated_at=coalesce($10,now()),updated_at=now() WHERE id=$1 AND external_payment_id IS NULL AND amount_cents=$11`, req.AttemptID, p.ID, status, p.Status, p.StatusDetail, p.QRCode, p.QRCodeBase64, p.TicketURL, expires, p.UpdatedAt, req.AmountCents)
	if e != nil {
		return domain.PIX{}, e
	}
	if tag.RowsAffected() != 1 {
		return domain.PIX{}, app.ErrConflict
	}
	_, e = tx.Exec(ctx, `UPDATE payments SET status=$2,updated_at=now() WHERE id=(SELECT payment_id FROM payment_attempts WHERE id=$1)`, req.AttemptID, paymentStatus)
	if e != nil {
		return domain.PIX{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.PIX{}, e
	}
	return domain.PIX{AttemptID: req.AttemptID, Status: status, QRCode: p.QRCode, QRCodeBase64: p.QRCodeBase64, TicketURL: p.TicketURL, AmountCents: req.AmountCents, ExpiresAt: expires}, nil
}
func (r *PaymentRepository) CompleteCard(ctx context.Context, req domain.CreateCardRequest, p domain.ProviderPayment) (domain.Card, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Card{}, err
	}
	defer tx.Rollback(ctx)
	state, paymentState := attemptStatus(p.Status, p.StatusDetail, p.RefundedCents), internalStatus(p.Status, p.RefundedCents)
	// A synchronous provider response is not the final order confirmation. Keep
	// the aggregate processing until the signed webhook is reconciled.
	if paymentState == "paid" {
		paymentState, state = "processing", "processing"
	}
	tag, err := tx.Exec(ctx, `UPDATE payment_attempts SET external_payment_id=$2,status=$3,provider_status=$4,provider_status_detail=nullif($5,''),card_brand=nullif($6,''),card_last_four=nullif($7,''),card_authentication_url=nullif($8,''),provider_updated_at=coalesce($9,now()),updated_at=now() WHERE id=$1 AND external_payment_id IS NULL AND amount_cents=$10`, req.AttemptID, p.ID, state, p.Status, p.StatusDetail, p.CardBrand, p.CardLastFour, p.AuthenticationURL, p.UpdatedAt, req.AmountCents)
	if err != nil {
		return domain.Card{}, err
	}
	if tag.RowsAffected() != 1 {
		return domain.Card{}, app.ErrConflict
	}
	_, err = tx.Exec(ctx, `UPDATE payments SET status=$2,updated_at=now() WHERE id=(SELECT payment_id FROM payment_attempts WHERE id=$1)`, req.AttemptID, paymentState)
	if err != nil {
		return domain.Card{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Card{}, err
	}
	return domain.Card{AttemptID: req.AttemptID, Status: state, StatusDetail: p.StatusDetail, Brand: p.CardBrand, LastFour: p.CardLastFour, AuthenticationURL: p.AuthenticationURL, AmountCents: req.AmountCents, Installments: req.Installments, RequiresAction: p.AuthenticationURL != "" && p.Status == "pending"}, nil
}
func (r *PaymentRepository) FailAttempt(ctx context.Context, id, code string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `UPDATE payment_attempts SET status='failed',failure_code=$2,updated_at=now() WHERE id=$1`, id, code); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE payments SET status='failed',updated_at=now() WHERE id=(SELECT payment_id FROM payment_attempts WHERE id=$1)`, id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *PaymentRepository) Status(ctx context.Context, userID, ref string) (domain.Status, error) {
	var v domain.Status
	e := r.pool.QueryRow(ctx, `SELECT a.id,o.order_number,a.method,a.status,p.amount_cents,p.paid_amount_cents,p.refunded_amount_cents,a.pix_expires_at,p.paid_at FROM orders o JOIN payments p ON p.order_id=o.id JOIN payment_attempts a ON a.payment_id=p.id WHERE o.user_id=$1 AND o.order_number=$2 ORDER BY a.created_at DESC LIMIT 1`, userID, ref).Scan(&v.AttemptID, &v.OrderReference, &v.Method, &v.Status, &v.AmountCents, &v.PaidAmountCents, &v.RefundedAmountCents, &v.ExpiresAt, &v.PaidAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return v, app.ErrNotFound
	}
	return v, e
}
func (r *PaymentRepository) RecordWebhook(ctx context.Context, w domain.Webhook) (bool, error) {
	sum := sha256.Sum256(w.Payload)
	var payload any
	if json.Unmarshal(w.Payload, &payload) != nil {
		return false, app.ErrInvalidWebhook
	}
	raw, _ := json.Marshal(payload)
	tag, e := r.pool.Exec(ctx, `INSERT INTO payment_webhook_events(provider,external_event_id,event_type,external_payment_id,payload,payload_sha256) VALUES('mercado_pago',$1,$2,$3,$4::jsonb,$5) ON CONFLICT(provider,external_event_id) DO UPDATE SET event_type=excluded.event_type,external_payment_id=excluded.external_payment_id,payload=excluded.payload,payload_sha256=excluded.payload_sha256,processing_status='pending',processed_at=null,processing_error=null WHERE payment_webhook_events.processing_status='failed'`, w.EventID, w.EventType, w.ExternalPaymentID, string(raw), hex.EncodeToString(sum[:]))
	if e != nil {
		return false, e
	}
	return tag.RowsAffected() == 1, nil
}
func (r *PaymentRepository) MarkWebhookFailed(ctx context.Context, eventID, message string) error {
	_, e := r.pool.Exec(ctx, `UPDATE payment_webhook_events SET processing_status='failed',processing_error=$2,processed_at=now() WHERE provider='mercado_pago' AND external_event_id=$1 AND processing_status='pending'`, eventID, message)
	return e
}

func (r *PaymentRepository) Reconcile(ctx context.Context, w domain.Webhook, p domain.ProviderPayment) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var attemptID, paymentID, orderID, ref, currency, current string
	var amount int64
	var previousUpdated *time.Time
	e = tx.QueryRow(ctx, `SELECT a.id,p.id,o.id,o.order_number,p.currency,p.status,p.amount_cents,a.provider_updated_at FROM payment_attempts a JOIN payments p ON p.id=a.payment_id JOIN orders o ON o.id=p.order_id WHERE a.provider='mercado_pago' AND a.external_payment_id=$1 FOR UPDATE OF a,p,o`, p.ID).Scan(&attemptID, &paymentID, &orderID, &ref, &currency, &current, &amount, &previousUpdated)
	if errors.Is(e, pgx.ErrNoRows) {
		e = tx.QueryRow(ctx, `SELECT a.id,p.id,o.id,o.order_number,p.currency,p.status,p.amount_cents,a.provider_updated_at FROM orders o JOIN payments p ON p.order_id=o.id JOIN payment_attempts a ON a.payment_id=p.id WHERE o.order_number=$1 AND a.provider='mercado_pago' AND a.external_payment_id IS NULL ORDER BY a.created_at DESC LIMIT 1 FOR UPDATE OF a,p,o`, p.ExternalReference).Scan(&attemptID, &paymentID, &orderID, &ref, &currency, &current, &amount, &previousUpdated)
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE payment_attempts SET external_payment_id=$2 WHERE id=$1`, attemptID, p.ID)
		}
	}
	if e != nil {
		return e
	}
	if currency != "BRL" || p.Currency != "BRL" || amount != p.AmountCents || ref != p.ExternalReference {
		return app.ErrValueMismatch
	}
	if previousUpdated != nil && p.UpdatedAt != nil && p.UpdatedAt.Before(*previousUpdated) {
		_, e = tx.Exec(ctx, `UPDATE payment_webhook_events SET payment_attempt_id=$2,processing_status='ignored',processed_at=now() WHERE provider='mercado_pago' AND external_event_id=$1`, w.EventID, attemptID)
		if e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	status := internalStatus(p.Status, p.RefundedCents)
	attemptState := attemptStatus(p.Status, p.StatusDetail, p.RefundedCents)
	paid := int64(0)
	var paidAt *time.Time
	if p.Status == "approved" || p.Status == "refunded" || p.Status == "charged_back" {
		paid = amount
		paidAt = p.ApprovedAt
		if paidAt == nil {
			n := time.Now()
			paidAt = &n
		}
	}
	_, e = tx.Exec(ctx, `UPDATE payment_attempts SET status=$2,provider_status=$3,provider_status_detail=nullif($4,''),provider_updated_at=coalesce($5,now()),updated_at=now() WHERE id=$1`, attemptID, attemptState, p.Status, p.StatusDetail, p.UpdatedAt)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE payments SET status=$2,paid_amount_cents=$3,refunded_amount_cents=$4,paid_at=$5,updated_at=now() WHERE id=$1`, paymentID, status, paid, p.RefundedCents, paidAt)
	if e != nil {
		return e
	}
	if current != status {
		_, e = tx.Exec(ctx, `INSERT INTO payment_status_events(payment_id,payment_attempt_id,previous_status,new_status,source,metadata) VALUES($1,$2,$3,$4,'provider_webhook',jsonb_build_object('provider_status',$5::text,'provider_status_detail',$6::text))`, paymentID, attemptID, current, status, p.Status, p.StatusDetail)
		if e != nil {
			return e
		}
	}
	if status == "paid" && current != "paid" {
		tag, e := tx.Exec(ctx, `UPDATE orders SET status='confirmed',updated_at=now() WHERE id=$1 AND status='pending_payment' AND total_cents=$2 AND payment_readiness='ready'`, orderID, amount)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return app.ErrConflict
		}
		_, e = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,status,description,metadata) VALUES($1,'confirmed','Pagamento confirmado pelo Mercado Pago',jsonb_build_object('external_payment_id',$2::text))`, orderID, p.ID)
		if e != nil {
			return e
		}
	}
	if p.RefundedCents > 0 {
		refundStatus := "succeeded"
		_, e = tx.Exec(ctx, `INSERT INTO payment_refunds(payment_id,payment_attempt_id,provider,external_refund_id,idempotency_key,amount_cents,status,reason,processed_at) VALUES($1,$2,'mercado_pago',null,'reconcile-'||$3,$4,$5,'Sincronizado pelo status do provedor',now()) ON CONFLICT(idempotency_key) DO UPDATE SET amount_cents=excluded.amount_cents,status=excluded.status,processed_at=now()`, paymentID, attemptID, p.ID, p.RefundedCents, refundStatus)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `UPDATE payment_webhook_events SET payment_attempt_id=$2,processing_status='processed',processed_at=now(),processing_error=null WHERE provider='mercado_pago' AND external_event_id=$1`, w.EventID, attemptID)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
