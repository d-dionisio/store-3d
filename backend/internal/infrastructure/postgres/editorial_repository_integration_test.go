package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/editorial"
	admin "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/editorial"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newEditorialIntegrationPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Editorial PostgreSQL integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "editorial")
	if err != nil {
		t.Fatalf("create isolated Editorial schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to Editorial schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated Editorial pool: %v", err)
	}
	return pool, ctx
}

func TestEditorialCatalogLifecycleIntegration(t *testing.T) {
	pool, ctx := newEditorialIntegrationPool(t)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique Editorial fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	var adminID, categoryID, materialID, colorID, firstProductID, secondProductID, inactiveProductID string
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,repeat('x',60),'admin','active') RETURNING id::text`, "editorial-admin-"+suffix+"@example.invalid", "Editorial fixture "+suffix).Scan(&adminID); err != nil {
		t.Fatalf("insert isolated Editorial administrator: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO categories(name,slug,is_active) VALUES($1,$2,true) RETURNING id::text`, "Editorial category "+suffix, "editorial-category-"+suffix).Scan(&categoryID); err != nil {
		t.Fatalf("insert isolated Editorial category: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id::text`, "ED-PLA-"+suffix, "Editorial PLA "+suffix).Scan(&materialID); err != nil {
		t.Fatalf("insert isolated Editorial material: %v", err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#345678',true) RETURNING id::text`, "editorial-color-"+suffix, "Editorial color "+suffix).Scan(&colorID); err != nil {
		t.Fatalf("insert isolated Editorial color: %v", err)
	}
	insertActiveEditorialProduct := func(name, slug, sku string) string {
		t.Helper()
		var productID string
		if insertErr := pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status,production_type) VALUES($1,$2,$3,'Editorial isolated fixture','active','made_to_order') RETURNING id::text`, categoryID, slug, name).Scan(&productID); insertErr != nil {
			t.Fatalf("insert isolated commercial Editorial product: %v", insertErr)
		}
		if _, insertErr := pool.Exec(ctx, `INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,track_inventory,status) VALUES($1,$2,$3,$4,2500,false,'active')`, productID, materialID, colorID, sku); insertErr != nil {
			t.Fatalf("insert isolated commercial Editorial variant: %v", insertErr)
		}
		return productID
	}
	firstProductID = insertActiveEditorialProduct("Editorial product A "+suffix, "editorial-product-a-"+suffix, "ED-A-"+suffix)
	secondProductID = insertActiveEditorialProduct("Editorial product B "+suffix, "editorial-product-b-"+suffix, "ED-B-"+suffix)
	if err = pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status,production_type) VALUES($1,$2,$3,'Editorial inactive fixture','draft','made_to_order') RETURNING id::text`, categoryID, "editorial-inactive-"+suffix, "Editorial inactive "+suffix).Scan(&inactiveProductID); err != nil {
		t.Fatalf("insert isolated inactive Editorial product: %v", err)
	}

	repository := NewEditorialRepository(pool)
	service := app.New(repository, NewCatalogRepository(pool))
	session := admin.Session{UserID: adminID, Role: "admin"}
	input := domain.Input{Slug: "integration-editorial-" + suffix, Title: "Integration edition", Description: "Database integration test", Eyebrow: "Edition"}
	created, err := service.Create(ctx, session, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, session, input); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate slug: %v", err)
	}
	if _, err = service.Publish(ctx, session, created.ID, created.Version); !errors.Is(err, domain.ErrEmpty) {
		t.Fatalf("empty publication: %v", err)
	}
	if _, err = service.ReplaceProducts(ctx, session, created.ID, []string{inactiveProductID}, created.Version); !errors.Is(err, domain.ErrInactiveProduct) {
		t.Fatalf("inactive product: %v", err)
	}
	if _, err = service.ReplaceProducts(ctx, session, created.ID, []string{"00000000-0000-0000-0000-000000000000"}, created.Version); !errors.Is(err, domain.ErrInactiveProduct) {
		t.Fatalf("missing product: %v", err)
	}
	ordered, err := service.ReplaceProducts(ctx, session, created.ID, []string{secondProductID, firstProductID}, created.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered.ProductIDs) != 2 || ordered.ProductIDs[0] != secondProductID {
		t.Fatalf("product order was not preserved")
	}
	if _, err = service.Update(ctx, session, created.ID, domain.Input{Slug: input.Slug, Title: "Concurrent", Description: input.Description, Eyebrow: input.Eyebrow, ExpectedVersion: created.Version}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale update: %v", err)
	}
	published, err := service.Publish(ctx, session, created.ID, ordered.Version)
	if err != nil {
		t.Fatal(err)
	}
	public, err := service.GetPublished(ctx, input.Slug)
	if err != nil || len(public.Products) != 2 {
		t.Fatalf("published catalog products=%d err=%v", len(public.Products), err)
	}
	draft, err := service.Unpublish(ctx, session, created.ID, published.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.GetPublished(ctx, input.Slug); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("draft leaked publicly: %v", err)
	}
	if draft.Status != "draft" || draft.PublishedAt != nil {
		t.Fatal("unpublish state is inconsistent")
	}
}
