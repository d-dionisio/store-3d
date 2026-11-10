package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	app "example.com/loja-3d/backend/internal/application/shipping"
	domain "example.com/loja-3d/backend/internal/domain/shipping"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ShippingRepository struct{ pool DBTXPool }

// DBTXPool is kept concrete at construction while exposing only operations used here.
type DBTXPool interface {
	Begin(context.Context) (pgx.Tx, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewShippingRepository(pool DBTXPool) *ShippingRepository { return &ShippingRepository{pool: pool} }

func (r *ShippingRepository) LoadRequest(ctx context.Context, userID, reference, origin string) (domain.QuoteRequest, error) {
	var req domain.QuoteRequest
	var shippingIncluded bool
	e := r.pool.QueryRow(ctx, `SELECT o.id,o.order_number,a.postal_code,o.subtotal_cents,o.shipping_included FROM orders o JOIN order_addresses a ON a.order_id=o.id AND a.address_type='shipping' WHERE o.user_id=$1 AND o.order_number=$2 AND o.status='pending_payment'`, userID, reference).Scan(&req.OrderID, &req.Reference, &req.DestinationPostalCode, &req.InsuranceValueCents, &shippingIncluded)
	if errors.Is(e, pgx.ErrNoRows) {
		return req, app.ErrNotFound
	}
	if e != nil {
		return req, e
	}
	if shippingIncluded {
		return req, app.ErrOrderLocked
	}
	req.OriginPostalCode = origin
	if normalized, e := domain.NormalizePostalCode(req.DestinationPostalCode); e == nil {
		req.DestinationPostalCode = normalized
	} else {
		return req, app.ErrInvalid
	}
	rows, e := r.pool.Query(ctx, `SELECT quantity,weight_grams,length_mm,width_mm,height_mm FROM order_packages WHERE order_id=$1 ORDER BY sequence`, req.OrderID)
	if e != nil {
		return req, e
	}
	for rows.Next() {
		var p domain.Package
		if e = rows.Scan(&p.Quantity, &p.WeightGrams, &p.LengthMM, &p.WidthMM, &p.HeightMM); e != nil {
			rows.Close()
			return req, e
		}
		req.Packages = append(req.Packages, p)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return req, e
	}
	if len(req.Packages) > 0 {
		return req, nil
	}
	rows, e = r.pool.Query(ctx, `SELECT oi.quantity,v.weight_grams,v.depth_mm,v.width_mm,v.height_mm FROM order_items oi JOIN product_variants v ON v.id=oi.product_variant_id WHERE oi.order_id=$1 AND oi.item_type='catalog' ORDER BY oi.id`, req.OrderID)
	if e != nil {
		return req, e
	}
	defer rows.Close()
	for rows.Next() {
		var p domain.Package
		if e = rows.Scan(&p.Quantity, &p.WeightGrams, &p.LengthMM, &p.WidthMM, &p.HeightMM); e != nil {
			return req, app.ErrInvalid
		}
		req.Packages = append(req.Packages, p)
	}
	return req, rows.Err()
}

func (r *ShippingRepository) Cached(ctx context.Context, orderID, hash string, now time.Time) ([]domain.Quote, error) {
	rows, e := r.pool.Query(ctx, `SELECT id,external_service_code,carrier_name,service_name,price_cents,estimated_min_days,estimated_max_days,expires_at FROM shipping_quotes WHERE order_id=$1 AND request_hash=$2 AND expires_at>$3 ORDER BY price_cents,id`, orderID, hash, now)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	return scanShippingQuotes(rows)
}

func scanShippingQuotes(rows pgx.Rows) ([]domain.Quote, error) {
	out := []domain.Quote{}
	for rows.Next() {
		var q domain.Quote
		if e := rows.Scan(&q.ID, &q.ServiceCode, &q.CarrierName, &q.ServiceName, &q.PriceCents, &q.MinDays, &q.MaxDays, &q.ExpiresAt); e != nil {
			return nil, e
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *ShippingRepository) Save(ctx context.Context, request domain.QuoteRequest, hash string, quotes []domain.ProviderQuote, expires time.Time) ([]domain.Quote, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, request.OrderID+hash); e != nil {
		return nil, e
	}
	rows, e := tx.Query(ctx, `SELECT id,external_service_code,carrier_name,service_name,price_cents,estimated_min_days,estimated_max_days,expires_at FROM shipping_quotes WHERE order_id=$1 AND request_hash=$2 AND expires_at>now() ORDER BY price_cents,id`, request.OrderID, hash)
	if e != nil {
		return nil, e
	}
	existing, e := scanShippingQuotes(rows)
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(existing) > 0 {
		if e = tx.Commit(ctx); e != nil {
			return nil, e
		}
		return existing, nil
	}
	out := make([]domain.Quote, 0, len(quotes))
	for _, q := range quotes {
		var serviceID string
		e = tx.QueryRow(ctx, `INSERT INTO shipping_services(provider,external_service_code,carrier_name,service_name) VALUES('melhor_envio',$1,$2,$3) ON CONFLICT(provider,external_service_code) DO UPDATE SET carrier_name=excluded.carrier_name,service_name=excluded.service_name,updated_at=now() RETURNING id`, q.ServiceCode, q.CarrierName, q.ServiceName).Scan(&serviceID)
		if e != nil {
			return nil, e
		}
		snapshot, _ := json.Marshal(map[string]any{"request_packages": request.Packages, "provider_packages": q.PackageSnapshot})
		var saved domain.Quote
		e = tx.QueryRow(ctx, `INSERT INTO shipping_quotes(order_id,shipping_service_id,provider,external_service_code,carrier_name,service_name,origin_postal_code,destination_postal_code,price_cents,estimated_min_days,estimated_max_days,package_data,expires_at,request_hash) VALUES($1,$2,'melhor_envio',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id,external_service_code,carrier_name,service_name,price_cents,estimated_min_days,estimated_max_days,expires_at`, request.OrderID, serviceID, q.ServiceCode, q.CarrierName, q.ServiceName, request.OriginPostalCode, request.DestinationPostalCode, q.PriceCents, q.MinDays, q.MaxDays, snapshot, expires, hash).Scan(&saved.ID, &saved.ServiceCode, &saved.CarrierName, &saved.ServiceName, &saved.PriceCents, &saved.MinDays, &saved.MaxDays, &saved.ExpiresAt)
		if e != nil {
			return nil, e
		}
		out = append(out, saved)
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return out, nil
}

func (r *ShippingRepository) Select(ctx context.Context, userID, sessionID, reference, quoteID string, now time.Time) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var orderID, status, readiness string
	e = tx.QueryRow(ctx, `SELECT id,status,payment_readiness FROM orders WHERE user_id=$1 AND order_number=$2 FOR UPDATE`, userID, reference).Scan(&orderID, &status, &readiness)
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	if e != nil {
		return e
	}
	if status != "pending_payment" || readiness == "ready" {
		return app.ErrOrderLocked
	}
	var q domain.Selection
	var serviceID *string
	e = tx.QueryRow(ctx, `SELECT id,order_id,shipping_service_id,external_service_code,carrier_name,service_name,price_cents,estimated_min_days,estimated_max_days,expires_at FROM shipping_quotes WHERE id=$1 AND order_id=$2`, quoteID, orderID).Scan(&q.QuoteID, &q.OrderID, &serviceID, &q.ServiceCode, &q.CarrierName, &q.ServiceName, &q.PriceCents, &q.MinDays, &q.MaxDays, &q.ExpiresAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	if e != nil {
		return e
	}
	if !q.ExpiresAt.After(now) {
		return app.ErrExpired
	}
	_, e = tx.Exec(ctx, `INSERT INTO order_shipping(order_id,shipping_quote_id,shipping_service_id,provider,external_service_code,carrier_name,service_name,price_cents,estimated_min_days,estimated_max_days,estimated_delivery_at) VALUES($1,$2,$3,'melhor_envio',$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(order_id) DO NOTHING`, orderID, q.QuoteID, serviceID, q.ServiceCode, q.CarrierName, q.ServiceName, q.PriceCents, q.MinDays, q.MaxDays, now.Add(time.Duration(q.MaxDays)*24*time.Hour))
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE orders SET shipping_cents=$2,total_cents=subtotal_cents+$2-discount_cents,payment_readiness='ready',updated_at=now() WHERE id=$1`, orderID, q.PriceCents)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO customer_audit_events(user_id,session_id,action,resource_type,resource_id,outcome,metadata) VALUES($1,$2,'shipping.quote.select','order',$3,'success',jsonb_build_object('shipping_quote_id',$4::text))`, userID, sessionID, reference, quoteID)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func (r *ShippingRepository) SavePackages(ctx context.Context, adminID, reference string, packages []domain.Package, sessionID string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var orderID, status string
	e = tx.QueryRow(ctx, `SELECT id,status FROM orders WHERE order_number=$1 FOR UPDATE`, reference).Scan(&orderID, &status)
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	if e != nil {
		return e
	}
	if status != "pending_payment" {
		return app.ErrOrderLocked
	}
	if _, e = tx.Exec(ctx, `DELETE FROM order_packages WHERE order_id=$1`, orderID); e != nil {
		return e
	}
	for i, p := range packages {
		if _, e = tx.Exec(ctx, `INSERT INTO order_packages(order_id,sequence,quantity,weight_grams,length_mm,width_mm,height_mm,created_by_admin_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, orderID, i+1, p.Quantity, p.WeightGrams, p.LengthMM, p.WidthMM, p.HeightMM, adminID); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `DELETE FROM shipping_quotes WHERE order_id=$1 AND id NOT IN (SELECT shipping_quote_id FROM order_shipping WHERE shipping_quote_id IS NOT NULL)`, orderID)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO admin_audit_events(admin_user_id,session_id,action,resource_type,resource_id,outcome) VALUES($1,$2,'order.packages.update','order',$3,'success')`, adminID, sessionID, reference)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
