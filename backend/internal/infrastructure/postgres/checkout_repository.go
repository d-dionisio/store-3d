package postgres

import (
	"context"
	"errors"
	"strings"

	app "example.com/loja-3d/backend/internal/application/customer"
	domain "example.com/loja-3d/backend/internal/domain/customer"
	"github.com/jackc/pgx/v5"
)

const cartVariantSQL = `SELECT v.id,p.name,v.sku,m.name,c.name,v.price_cents,v.stock_quantity-v.reserved_quantity,v.track_inventory FROM product_variants v JOIN products p ON p.id=v.product_id JOIN materials m ON m.id=v.material_id JOIN colors c ON c.id=v.color_id LEFT JOIN product_sizes s ON s.id=v.size_id LEFT JOIN finishes f ON f.id=v.finish_id WHERE v.id=$1 AND v.status='active' AND v.deleted_at IS NULL AND p.status='active' AND p.deleted_at IS NULL AND m.is_active AND c.is_active AND (v.size_id IS NULL OR s.is_active) AND (v.finish_id IS NULL OR f.is_active)`

func normalizeCart(items []domain.CartInput) ([]domain.CartInput, error) {
	if len(items) == 0 || len(items) > 50 {
		return nil, app.ErrInvalidCart
	}
	seen := map[string]int{}
	for _, v := range items {
		if strings.TrimSpace(v.VariantID) == "" || v.Quantity < 1 || v.Quantity > 100 {
			return nil, app.ErrInvalidCart
		}
		seen[v.VariantID] += v.Quantity
		if seen[v.VariantID] > 100 {
			return nil, app.ErrInvalidCart
		}
	}
	result := make([]domain.CartInput, 0, len(seen))
	for id, q := range seen {
		result = append(result, domain.CartInput{VariantID: id, Quantity: q})
	}
	return result, nil
}
func (r *CustomerRepository) ValidateCart(ctx context.Context, items []domain.CartInput) (domain.Cart, error) {
	items, e := normalizeCart(items)
	if e != nil {
		return domain.Cart{}, e
	}
	cart := domain.Cart{Items: []domain.CartLine{}}
	for _, input := range items {
		var line domain.CartLine
		var tracked bool
		e = r.pool.QueryRow(ctx, cartVariantSQL, input.VariantID).Scan(&line.VariantID, &line.ProductName, &line.SKU, &line.Material, &line.Color, &line.UnitPriceCents, &line.AvailableQuantity, &tracked)
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.Cart{}, app.ErrInvalidCart
		}
		if e != nil {
			return domain.Cart{}, e
		}
		if tracked && input.Quantity > line.AvailableQuantity {
			return domain.Cart{}, app.ErrInsufficientStock
		}
		line.Quantity = input.Quantity
		line.LineTotalCents = line.UnitPriceCents * int64(input.Quantity)
		cart.SubtotalCents += line.LineTotalCents
		cart.Items = append(cart.Items, line)
	}
	return cart, nil
}

func (r *CustomerRepository) CreateCatalogOrder(ctx context.Context, user, session, keyHash string, items []domain.CartInput, addressID string) (domain.OrderResult, error) {
	items, e := normalizeCart(items)
	if e != nil {
		return domain.OrderResult{}, e
	}
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.OrderResult{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, keyHash); e != nil {
		return domain.OrderResult{}, e
	}
	var existing domain.OrderResult
	e = tx.QueryRow(ctx, `SELECT order_number,status FROM orders WHERE user_id=$1 AND checkout_key_hash=$2`, user, keyHash).Scan(&existing.Reference, &existing.Status)
	if e == nil {
		if e = tx.Commit(ctx); e != nil {
			return domain.OrderResult{}, e
		}
		return existing, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.OrderResult{}, e
	}
	var name, email, phone string
	e = tx.QueryRow(ctx, `SELECT full_name,email,coalesce(phone,'') FROM users WHERE id=$1 AND status='active' AND email_verified_at IS NOT NULL`, user).Scan(&name, &email, &phone)
	if e != nil {
		return domain.OrderResult{}, app.ErrUnauthorized
	}
	var addr domain.Address
	e = tx.QueryRow(ctx, `SELECT id,recipient_name,postal_code,street,number,coalesce(complement,''),district,city,state_code,country_code,is_default FROM addresses WHERE id=$1 AND user_id=$2`, addressID, user).Scan(&addr.ID, &addr.RecipientName, &addr.PostalCode, &addr.Street, &addr.Number, &addr.Complement, &addr.District, &addr.City, &addr.StateCode, &addr.CountryCode, &addr.IsDefault)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.OrderResult{}, app.ErrInvalidCart
	}
	if e != nil {
		return domain.OrderResult{}, e
	}
	lines := make([]domain.CartLine, 0, len(items))
	var subtotal int64
	for _, input := range items {
		var line domain.CartLine
		var tracked bool
		e = tx.QueryRow(ctx, cartVariantSQL+` FOR UPDATE OF v`, input.VariantID).Scan(&line.VariantID, &line.ProductName, &line.SKU, &line.Material, &line.Color, &line.UnitPriceCents, &line.AvailableQuantity, &tracked)
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.OrderResult{}, app.ErrInvalidCart
		}
		if e != nil {
			return domain.OrderResult{}, e
		}
		if tracked && input.Quantity > line.AvailableQuantity {
			return domain.OrderResult{}, app.ErrInsufficientStock
		}
		line.Quantity = input.Quantity
		line.LineTotalCents = line.UnitPriceCents * int64(input.Quantity)
		subtotal += line.LineTotalCents
		lines = append(lines, line)
	}
	var orderID, reference string
	e = tx.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,customer_name,customer_email,customer_phone,subtotal_cents,total_cents,placed_at,checkout_key_hash,payment_readiness) VALUES('PED-'||upper(substr(replace(gen_random_uuid()::text,'-',''),1,16)),$1,'pending_payment',$2,$3,nullif($4,''),$5,$5,now(),$6,'awaiting_shipping_quote') RETURNING id,order_number`, user, name, email, phone, subtotal, keyHash).Scan(&orderID, &reference)
	if e != nil {
		return domain.OrderResult{}, e
	}
	for _, line := range lines {
		_, e = tx.Exec(ctx, `INSERT INTO order_items(order_id,item_type,product_id,product_variant_id,product_name,description_snapshot,sku_snapshot,material_name,color_name,quantity,unit_price_cents) SELECT $1,'catalog',v.product_id,v.id,$2,p.description,$3,$4,$5,$6,$7 FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=$8`, orderID, line.ProductName, line.SKU, line.Material, line.Color, line.Quantity, line.UnitPriceCents, line.VariantID)
		if e != nil {
			return domain.OrderResult{}, e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO order_addresses(order_id,address_type,recipient_name,postal_code,street,number,complement,district,city,state_code,country_code) VALUES($1,'shipping',$2,$3,$4,$5,nullif($6,''),$7,$8,$9,$10)`, orderID, addr.RecipientName, addr.PostalCode, addr.Street, addr.Number, addr.Complement, addr.District, addr.City, addr.StateCode, addr.CountryCode)
	if e != nil {
		return domain.OrderResult{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO order_status_events(order_id,status,description,created_by_user_id,metadata) VALUES($1,'pending_payment','Pedido de catálogo criado; frete pendente',$2,jsonb_build_object('checkout_key_hash',$3::text))`, orderID, user, keyHash)
	if e != nil {
		return domain.OrderResult{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO customer_audit_events(user_id,session_id,action,resource_type,resource_id,outcome) VALUES($1,$2,'checkout.catalog.create','order',$3,'success')`, user, session, reference)
	if e != nil {
		return domain.OrderResult{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.OrderResult{}, e
	}
	return domain.OrderResult{Reference: reference, Status: "pending_payment"}, nil
}

func (r *CustomerRepository) ListAddresses(ctx context.Context, user string) ([]domain.Address, error) {
	rows, e := r.pool.Query(ctx, `SELECT id,recipient_name,postal_code,street,number,coalesce(complement,''),district,city,state_code,country_code,is_default FROM addresses WHERE user_id=$1 ORDER BY is_default DESC,created_at DESC`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Address{}
	for rows.Next() {
		var a domain.Address
		if e = rows.Scan(&a.ID, &a.RecipientName, &a.PostalCode, &a.Street, &a.Number, &a.Complement, &a.District, &a.City, &a.StateCode, &a.CountryCode, &a.IsDefault); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (r *CustomerRepository) CreateAddress(ctx context.Context, user string, a domain.Address) (domain.Address, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return a, e
	}
	defer tx.Rollback(ctx)
	if a.IsDefault {
		if _, e = tx.Exec(ctx, `UPDATE addresses SET is_default=false,updated_at=now() WHERE user_id=$1 AND is_default`, user); e != nil {
			return a, e
		}
	}
	e = tx.QueryRow(ctx, `INSERT INTO addresses(user_id,label,recipient_name,postal_code,street,number,complement,district,city,state_code,country_code,is_default) VALUES($1,'Entrega',$2,$3,$4,$5,nullif($6,''),$7,$8,upper($9),upper($10),$11) RETURNING id`, user, a.RecipientName, a.PostalCode, a.Street, a.Number, a.Complement, a.District, a.City, a.StateCode, a.CountryCode, a.IsDefault).Scan(&a.ID)
	if e != nil {
		return a, e
	}
	return a, tx.Commit(ctx)
}
func (r *CustomerRepository) UpdateOrderAddress(ctx context.Context, user, ref, addressID string) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var orderID string
	var included bool
	e = tx.QueryRow(ctx, `SELECT id,shipping_included FROM orders WHERE order_number=$1 AND user_id=$2 AND status='pending_payment' FOR UPDATE`, ref, user).Scan(&orderID, &included)
	if errors.Is(e, pgx.ErrNoRows) {
		return app.ErrNotFound
	}
	if e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, `INSERT INTO order_addresses(order_id,address_type,recipient_name,postal_code,street,number,complement,district,city,state_code,country_code) SELECT $1,'shipping',recipient_name,postal_code,street,number,complement,district,city,state_code,country_code FROM addresses WHERE id=$2 AND user_id=$3 ON CONFLICT(order_id,address_type) DO UPDATE SET recipient_name=excluded.recipient_name,postal_code=excluded.postal_code,street=excluded.street,number=excluded.number,complement=excluded.complement,district=excluded.district,city=excluded.city,state_code=excluded.state_code,country_code=excluded.country_code`, orderID, addressID, user)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return app.ErrForbidden
	}
	readiness := "awaiting_shipping_quote"
	if included {
		readiness = "ready"
	}
	_, e = tx.Exec(ctx, `UPDATE orders SET payment_readiness=$2,updated_at=now() WHERE id=$1`, orderID, readiness)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *CustomerRepository) PrepareOrder(ctx context.Context, user, ref string) (domain.OrderResult, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.OrderResult{}, e
	}
	defer tx.Rollback(ctx)
	var id, status string
	var included bool
	var addresses, shipping int
	e = tx.QueryRow(ctx, `SELECT o.id,o.status,o.shipping_included,(SELECT count(*) FROM order_addresses WHERE order_id=o.id AND address_type='shipping'),(SELECT count(*) FROM order_shipping WHERE order_id=o.id) FROM orders o WHERE o.order_number=$1 AND o.user_id=$2 FOR UPDATE`, ref, user).Scan(&id, &status, &included, &addresses, &shipping)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.OrderResult{}, app.ErrNotFound
	}
	if e != nil {
		return domain.OrderResult{}, e
	}
	if status != "pending_payment" || addresses == 0 {
		return domain.OrderResult{}, app.ErrInvalidCart
	}
	if !included && shipping == 0 {
		return domain.OrderResult{}, app.ErrShippingRequired
	}
	if shipping > 0 {
		_, e = tx.Exec(ctx, `UPDATE orders o SET shipping_cents=s.price_cents,total_cents=subtotal_cents+s.price_cents-discount_cents,payment_readiness='ready',updated_at=now() FROM order_shipping s WHERE o.id=$1 AND s.order_id=o.id`, id)
	} else {
		_, e = tx.Exec(ctx, `UPDATE orders SET payment_readiness='ready',updated_at=now() WHERE id=$1`, id)
	}
	if e != nil {
		return domain.OrderResult{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.OrderResult{}, e
	}
	return domain.OrderResult{Reference: ref, Status: "pending_payment"}, nil
}
func (r *CustomerRepository) ListOrders(ctx context.Context, user string) ([]domain.OrderSummary, error) {
	rows, e := r.pool.Query(ctx, `SELECT order_number,status,payment_readiness,total_cents,created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, user)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.OrderSummary{}
	for rows.Next() {
		var o domain.OrderSummary
		if e = rows.Scan(&o.Reference, &o.Status, &o.PaymentReadiness, &o.TotalCents, &o.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (r *CustomerRepository) GetOrder(ctx context.Context, user, ref string) (domain.OrderDetail, error) {
	var o domain.OrderDetail
	e := r.pool.QueryRow(ctx, `SELECT order_number,status,payment_readiness,total_cents,created_at,NOT shipping_included FROM orders WHERE user_id=$1 AND order_number=$2`, user, ref).Scan(&o.Reference, &o.Status, &o.PaymentReadiness, &o.TotalCents, &o.CreatedAt, &o.ShippingRequired)
	if errors.Is(e, pgx.ErrNoRows) {
		return o, app.ErrNotFound
	}
	if e != nil {
		return o, e
	}
	rows, e := r.pool.Query(ctx, `SELECT coalesce(product_variant_id::text,''),product_name,coalesce(sku_snapshot,''),material_name,color_name,quantity,unit_price_cents,line_total_cents,quantity FROM order_items WHERE order_id=(SELECT id FROM orders WHERE order_number=$1)`, ref)
	if e != nil {
		return o, e
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.CartLine
		var ignored int
		if e = rows.Scan(&v.VariantID, &v.ProductName, &v.SKU, &v.Material, &v.Color, &v.Quantity, &v.UnitPriceCents, &v.LineTotalCents, &ignored); e != nil {
			return o, e
		}
		o.Items = append(o.Items, v)
	}
	var a domain.Address
	e = r.pool.QueryRow(ctx, `SELECT recipient_name,postal_code,street,number,coalesce(complement,''),district,city,state_code,country_code FROM order_addresses WHERE order_id=(SELECT id FROM orders WHERE order_number=$1) AND address_type='shipping'`, ref).Scan(&a.RecipientName, &a.PostalCode, &a.Street, &a.Number, &a.Complement, &a.District, &a.City, &a.StateCode, &a.CountryCode)
	if e == nil {
		o.Address = &a
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return o, e
	}
	return o, nil
}
