package postgres

import (
	"context"
	"errors"
	app "example.com/loja-3d/backend/internal/application/customer"
	domain "example.com/loja-3d/backend/internal/domain/customer"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCatalogCheckoutIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var category, material, color, product, variant, user, session, address string
	if e := pool.QueryRow(ctx, `INSERT INTO categories(name,slug) VALUES('Checkout','checkout-'||$1) RETURNING id`, suffix).Scan(&category); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO materials(code,name) VALUES('CO-'||$1,'Material Checkout '||$1) RETURNING id`, suffix).Scan(&material); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code) VALUES('checkout-'||$1,'Cor Checkout '||$1,'#112233') RETURNING id`, suffix).Scan(&color); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status,production_type) VALUES($1,'checkout-product-'||$2,'Produto Checkout','Descrição original','active','stocked') RETURNING id`, category, suffix).Scan(&product); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity,status) VALUES($1,$2,$3,'CHK-'||$4,1234,5,'active') RETURNING id`, product, material, color, suffix).Scan(&variant); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES('checkout-'||$1||'@example.com','Cliente Checkout',now()) RETURNING id`, suffix).Scan(&user); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,repeat('a',64),repeat('b',64),now()+interval '1 hour') RETURNING id`, user).Scan(&session); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente Checkout','01001000','Rua Original','10','Centro','São Paulo','SP') RETURNING id`, user).Scan(&address); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM customer_audit_events WHERE user_id=$1`, user)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE user_id=$1`, user)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user)
		_, _ = pool.Exec(ctx, `DELETE FROM product_variants WHERE id=$1`, variant)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, product)
		_, _ = pool.Exec(ctx, `DELETE FROM colors WHERE id=$1`, color)
		_, _ = pool.Exec(ctx, `DELETE FROM materials WHERE id=$1`, material)
		_, _ = pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, category)
	})
	repo := NewCustomerRepository(pool)
	items := []domain.CartInput{{VariantID: variant, Quantity: 2}}
	validated, e := repo.ValidateCart(ctx, items)
	if e != nil || validated.SubtotalCents != 2468 {
		t.Fatalf("validation=%+v err=%v", validated, e)
	}
	_, _ = pool.Exec(ctx, `UPDATE product_variants SET price_cents=1500 WHERE id=$1`, variant)
	order, e := repo.CreateCatalogOrder(ctx, user, session, app.Hash("checkout-key"), items, address)
	if e != nil {
		t.Fatal(e)
	}
	same, e := repo.CreateCatalogOrder(ctx, user, session, app.Hash("checkout-key"), items, address)
	if e != nil || same.Reference != order.Reference {
		t.Fatalf("idempotency=%+v err=%v", same, e)
	}
	var subtotal, line int64
	var street string
	_ = pool.QueryRow(ctx, `SELECT o.subtotal_cents,oi.line_total_cents,a.street FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN order_addresses a ON a.order_id=o.id WHERE o.order_number=$1`, order.Reference).Scan(&subtotal, &line, &street)
	if subtotal != 3000 || line != 3000 || street != "Rua Original" {
		t.Fatalf("snapshot subtotal=%d line=%d street=%s", subtotal, line, street)
	}
	_, _ = pool.Exec(ctx, `UPDATE addresses SET street='Rua Alterada' WHERE id=$1`, address)
	_ = pool.QueryRow(ctx, `SELECT street FROM order_addresses WHERE order_id=(SELECT id FROM orders WHERE order_number=$1)`, order.Reference).Scan(&street)
	if street != "Rua Original" {
		t.Fatal("order address changed retroactively")
	}
	if _, e = repo.PrepareOrder(ctx, user, order.Reference); !errors.Is(e, app.ErrShippingRequired) {
		t.Fatalf("shipping err=%v", e)
	}
	if _, e = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: "00000000-0000-0000-0000-000000000000", Quantity: 1}}); !errors.Is(e, app.ErrInvalidCart) {
		t.Fatalf("missing variant=%v", e)
	}
	_, _ = pool.Exec(ctx, `UPDATE product_variants SET status='inactive' WHERE id=$1`, variant)
	if _, e = repo.ValidateCart(ctx, items); !errors.Is(e, app.ErrInvalidCart) {
		t.Fatalf("inactive=%v", e)
	}
	_, _ = pool.Exec(ctx, `UPDATE product_variants SET status='active' WHERE id=$1`, variant)
	if _, e = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: variant, Quantity: 6}}); !errors.Is(e, app.ErrInsufficientStock) {
		t.Fatalf("stock=%v", e)
	}
	if _, e = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: variant, Quantity: 0}}); !errors.Is(e, app.ErrInvalidCart) {
		t.Fatalf("quantity=%v", e)
	}
	var wg sync.WaitGroup
	refs := make(chan string, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, e := repo.CreateCatalogOrder(ctx, user, session, app.Hash("concurrent-key"), []domain.CartInput{{VariantID: variant, Quantity: 1}}, address)
			refs <- o.Reference
			errs <- e
		}()
	}
	wg.Wait()
	close(refs)
	close(errs)
	for e = range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	first := ""
	for ref := range refs {
		if first == "" {
			first = ref
		} else if ref != first {
			t.Fatalf("concurrent refs %s %s", first, ref)
		}
	}
	_, _ = pool.Exec(ctx, `DELETE FROM addresses WHERE id=$1`, address)
	before := 0
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE checkout_key_hash=$1`, app.Hash("rollback-key")).Scan(&before)
	if _, e = repo.CreateCatalogOrder(ctx, user, session, app.Hash("rollback-key"), items, address); !errors.Is(e, app.ErrInvalidCart) {
		t.Fatalf("rollback error=%v", e)
	}
	after := 0
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE checkout_key_hash=$1`, app.Hash("rollback-key")).Scan(&after)
	if before != 0 || after != 0 {
		t.Fatal("failed checkout left an order")
	}
}

func TestCustomOrderCheckoutPreservesApprovedPriceAndRequiresShipping(t *testing.T) {
	ctx := context.Background()
	f := newConversionFixture(t)
	order, e := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if e != nil {
		t.Fatal(e)
	}
	var address string
	e = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Cliente','01001000','Rua do Orçamento','5','Centro','São Paulo','SP') RETURNING id`, f.userID).Scan(&address)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference); e != nil {
		t.Fatal(e)
	}
	if e = f.repo.UpdateOrderAddress(ctx, f.userID, order.Reference, address); e != nil {
		t.Fatal(e)
	}
	if _, e = f.repo.PrepareOrder(ctx, f.userID, order.Reference); !errors.Is(e, app.ErrShippingRequired) {
		t.Fatalf("shipping=%v", e)
	}
	var subtotal, total int64
	var readiness string
	e = f.pool.QueryRow(ctx, `SELECT subtotal_cents,total_cents,payment_readiness FROM orders WHERE order_number=$1`, order.Reference).Scan(&subtotal, &total, &readiness)
	if e != nil || subtotal != 1001 || total != 1001 || readiness != "awaiting_shipping_quote" {
		t.Fatalf("subtotal=%d total=%d readiness=%s err=%v", subtotal, total, readiness, e)
	}
}
