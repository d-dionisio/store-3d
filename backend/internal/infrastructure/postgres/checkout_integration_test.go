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
	domain "example.com/loja-3d/backend/internal/domain/customer"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestCatalogCheckoutIntegration(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Catalog checkout PostgreSQL integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "checkout")
	if err != nil {
		t.Fatalf("create isolated checkout schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to checkout schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated checkout pool: %v", err)
	}
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique checkout fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	var category, material, color, product, variant, user, session, address string
	if err = pool.QueryRow(ctx, `INSERT INTO categories(name,slug) VALUES($1,$2) RETURNING id::text`, "Checkout fixture "+suffix, "checkout-"+suffix).Scan(&category); err != nil {
		t.Fatalf("insert isolated checkout category: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO materials(code,name) VALUES($1,$2) RETURNING id::text`, "CO-"+suffix, "Checkout material "+suffix).Scan(&material); err != nil {
		t.Fatalf("insert isolated checkout material: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code) VALUES($1,$2,'#112233') RETURNING id::text`, "checkout-"+suffix, "Checkout color "+suffix).Scan(&color); err != nil {
		t.Fatalf("insert isolated checkout color: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status,production_type) VALUES($1,$2,$3,'Checkout isolated fixture','active','stocked') RETURNING id::text`, category, "checkout-product-"+suffix, "Checkout product "+suffix).Scan(&product); err != nil {
		t.Fatalf("insert isolated checkout product: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,stock_quantity,status) VALUES($1,$2,$3,$4,1234,5,'active') RETURNING id::text`, product, material, color, "CHK-"+suffix).Scan(&variant); err != nil {
		t.Fatalf("insert isolated checkout variant: %v", err)
	}
	email := "checkout-" + suffix + "@example.invalid"
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,full_name,email_verified_at) VALUES($1,$2,now()) RETURNING id::text`, email, "Checkout customer "+suffix).Scan(&user); err != nil {
		t.Fatalf("insert isolated checkout customer: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO customer_sessions(user_id,token_hash,csrf_token_hash,expires_at) VALUES($1,$2,$3,now()+interval '1 hour') RETURNING id::text`, user, app.Hash("checkout-session-"+fixtureID), app.Hash("checkout-csrf-"+fixtureID)).Scan(&session); err != nil {
		t.Fatalf("insert isolated checkout session: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,$2,'01001000','Original Street','10','Center','Sao Paulo','SP') RETURNING id::text`, user, "Checkout recipient "+suffix).Scan(&address); err != nil {
		t.Fatalf("insert isolated checkout address: %v", err)
	}

	repo := NewCustomerRepository(pool)
	items := []domain.CartInput{{VariantID: variant, Quantity: 2}}
	validated, err := repo.ValidateCart(ctx, items)
	if err != nil || validated.SubtotalCents != 2468 {
		t.Fatalf("cart validation subtotal=%d err=%v", validated.SubtotalCents, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE product_variants SET price_cents=1500 WHERE id=$1`, variant); err != nil {
		t.Fatalf("update isolated variant price: %v", err)
	}
	checkoutKey := app.Hash("checkout-key-" + fixtureID)
	order, err := repo.CreateCatalogOrder(ctx, user, session, checkoutKey, items, address)
	if err != nil {
		t.Fatalf("create isolated checkout order: %v", err)
	}
	same, err := repo.CreateCatalogOrder(ctx, user, session, checkoutKey, items, address)
	if err != nil || same.Reference != order.Reference {
		t.Fatalf("idempotency first=%+v second=%+v err=%v", order, same, err)
	}
	var subtotal, line int64
	var street string
	if err = pool.QueryRow(ctx, `SELECT o.subtotal_cents,oi.line_total_cents,a.street FROM orders o JOIN order_items oi ON oi.order_id=o.id JOIN order_addresses a ON a.order_id=o.id WHERE o.order_number=$1`, order.Reference).Scan(&subtotal, &line, &street); err != nil {
		t.Fatalf("read isolated order snapshots: %v", err)
	}
	if subtotal != 3000 || line != 3000 || street != "Original Street" {
		t.Fatalf("snapshot subtotal=%d line=%d street=%s", subtotal, line, street)
	}
	if _, err = pool.Exec(ctx, `UPDATE addresses SET street='Changed Street' WHERE id=$1`, address); err != nil {
		t.Fatalf("update isolated customer address: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT street FROM order_addresses WHERE order_id=(SELECT id FROM orders WHERE order_number=$1)`, order.Reference).Scan(&street); err != nil {
		t.Fatalf("read immutable order address snapshot: %v", err)
	}
	if street != "Original Street" {
		t.Fatal("order address changed retroactively")
	}
	if _, err = repo.PrepareOrder(ctx, user, order.Reference); !errors.Is(err, app.ErrShippingRequired) {
		t.Fatalf("shipping err=%v", err)
	}
	missingVariantID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create absent variant identifier")
	}
	if _, err = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: missingVariantID, Quantity: 1}}); !errors.Is(err, app.ErrInvalidCart) {
		t.Fatalf("missing variant=%v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE product_variants SET status='inactive' WHERE id=$1`, variant); err != nil {
		t.Fatalf("deactivate isolated variant: %v", err)
	}
	if _, err = repo.ValidateCart(ctx, items); !errors.Is(err, app.ErrInvalidCart) {
		t.Fatalf("inactive variant=%v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE product_variants SET status='active' WHERE id=$1`, variant); err != nil {
		t.Fatalf("reactivate isolated variant: %v", err)
	}
	if _, err = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: variant, Quantity: 6}}); !errors.Is(err, app.ErrInsufficientStock) {
		t.Fatalf("stock validation=%v", err)
	}
	if _, err = repo.ValidateCart(ctx, []domain.CartInput{{VariantID: variant, Quantity: 0}}); !errors.Is(err, app.ErrInvalidCart) {
		t.Fatalf("quantity validation=%v", err)
	}

	pool2, err := schema.OpenPoolWithApplicationName(ctx, "checkout-worker-b")
	if err != nil {
		t.Fatalf("open independent checkout pool: %v", err)
	}
	repo2 := NewCustomerRepository(pool2)
	start := make(chan struct{})
	refs := make(chan string, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	concurrentKey := app.Hash("concurrent-checkout-" + fixtureID)
	for _, current := range []*CustomerRepository{repo, repo2} {
		wg.Add(1)
		go func(current *CustomerRepository) {
			defer wg.Done()
			<-start
			created, createErr := current.CreateCatalogOrder(ctx, user, session, concurrentKey, []domain.CartInput{{VariantID: variant, Quantity: 1}}, address)
			refs <- created.Reference
			errs <- createErr
		}(current)
	}
	close(start)
	wg.Wait()
	close(refs)
	close(errs)
	for createErr := range errs {
		if createErr != nil {
			t.Fatal(createErr)
		}
	}
	first := ""
	for ref := range refs {
		if first == "" {
			first = ref
		} else if ref != first {
			t.Fatal("concurrent idempotent checkouts returned different references")
		}
	}
	if _, err = pool.Exec(ctx, `DELETE FROM addresses WHERE id=$1`, address); err != nil {
		t.Fatalf("delete only the isolated fixture address: %v", err)
	}
	rollbackKey := app.Hash("rollback-checkout-" + fixtureID)
	var before int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE checkout_key_hash=$1`, rollbackKey).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateCatalogOrder(ctx, user, session, rollbackKey, items, address); !errors.Is(err, app.ErrInvalidCart) {
		t.Fatalf("rollback error=%v", err)
	}
	var after int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE checkout_key_hash=$1`, rollbackKey).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != 0 || after != 0 {
		t.Fatal("failed checkout left an order")
	}
}

func TestCustomOrderCheckoutPreservesApprovedPriceAndRequiresShipping(t *testing.T) {
	ctx := context.Background()
	f := newConversionFixture(t)
	order, err := f.repo.ConvertQuote(ctx, f.userID, f.reference, f.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	var address string
	err = f.pool.QueryRow(ctx, `INSERT INTO addresses(user_id,recipient_name,postal_code,street,number,district,city,state_code) VALUES($1,'Checkout fixture','01001000','Quote Street','5','Center','Sao Paulo','SP') RETURNING id::text`, f.userID).Scan(&address)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE orders SET shipping_included=false WHERE order_number=$1`, order.Reference); err != nil {
		t.Fatal(err)
	}
	if err = f.repo.UpdateOrderAddress(ctx, f.userID, order.Reference, address); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.PrepareOrder(ctx, f.userID, order.Reference); !errors.Is(err, app.ErrShippingRequired) {
		t.Fatalf("shipping=%v", err)
	}
	var subtotal, total int64
	var readiness string
	err = f.pool.QueryRow(ctx, `SELECT subtotal_cents,total_cents,payment_readiness FROM orders WHERE order_number=$1`, order.Reference).Scan(&subtotal, &total, &readiness)
	if err != nil || subtotal != 1001 || total != 1001 || readiness != "awaiting_shipping_quote" {
		t.Fatalf("subtotal=%d total=%d readiness=%s err=%v", subtotal, total, readiness, err)
	}
}
