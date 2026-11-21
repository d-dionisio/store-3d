package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newCatalogIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("PostgreSQL catalog integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "catalog")
	if err != nil {
		t.Fatalf("create isolated catalog schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated catalog schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated catalog pool: %v", err)
	}
	return pool
}

func insertCatalogReferenceData(t *testing.T, pool *pgxpool.Pool, fixtureID string) (categoryID, materialID, marfimID, grafiteID string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	if err := pool.QueryRow(ctx, `INSERT INTO categories(name,slug,description,is_active) VALUES($1,$2,'Isolated catalog fixture',true) RETURNING id::text`, "Fixture Decor "+suffix, "fixture-decor-"+suffix).Scan(&categoryID); err != nil {
		t.Fatal("insert isolated category fixture")
	}
	if err := pool.QueryRow(ctx, `INSERT INTO materials(code,name,is_active) VALUES($1,$2,true) RETURNING id::text`, "FIX-PLA-"+suffix, "Fixture PLA "+suffix).Scan(&materialID); err != nil {
		t.Fatal("insert isolated material fixture")
	}
	if err := pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#EDE3D2',true) RETURNING id::text`, "fixture-marfim-"+suffix, "Fixture Marfim "+suffix).Scan(&marfimID); err != nil {
		t.Fatal("insert isolated color fixture")
	}
	if err := pool.QueryRow(ctx, `INSERT INTO colors(slug,name,hex_code,is_active) VALUES($1,$2,'#333333',true) RETURNING id::text`, "fixture-grafite-"+suffix, "Fixture Grafite "+suffix).Scan(&grafiteID); err != nil {
		t.Fatal("insert isolated color fixture")
	}
	return categoryID, materialID, marfimID, grafiteID
}

func insertCatalogProduct(t *testing.T, pool *pgxpool.Pool, categoryID, materialID, colorID, slug, name, sku string, price int64, variantStatus string) string {
	t.Helper()
	ctx := context.Background()
	var productID string
	if err := pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status,production_type) VALUES($1,$2,$3,'Isolated fixture description','active','made_to_order') RETURNING id::text`, categoryID, slug, name).Scan(&productID); err != nil {
		t.Fatal("insert isolated product fixture")
	}
	insertCatalogVariant(t, pool, productID, materialID, colorID, sku, price, variantStatus)
	return productID
}

func insertCatalogVariant(t *testing.T, pool *pgxpool.Pool, productID, materialID, colorID, sku string, price int64, variantStatus string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO product_variants(product_id,material_id,color_id,sku,price_cents,track_inventory,status) VALUES($1,$2,$3,$4,$5,false,$6)`, productID, materialID, colorID, sku, price, variantStatus); err != nil {
		t.Fatal("insert isolated product variant fixture")
	}
}

func TestCatalogRepositoryIntegration(t *testing.T) {
	pool := newCatalogIntegrationPool(t)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique catalog fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	categoryID, materialID, marfimID, grafiteID := insertCatalogReferenceData(t, pool, fixtureID)
	vasoSlug := "vaso-orbita-" + suffix
	vasoID := insertCatalogProduct(t, pool, categoryID, materialID, marfimID, vasoSlug, "Vaso Órbita "+suffix, "VAS-PLA-MAR-"+suffix, 15000, "active")
	insertCatalogVariant(t, pool, vasoID, materialID, grafiteID, "VAS-PLA-GRA-"+suffix, 24000, "active")
	nexoSlug := "organizador-nexo-" + suffix
	nexoActiveSKU := "ORG-NEX-PET-GRA-" + suffix
	nexoID := insertCatalogProduct(t, pool, categoryID, materialID, grafiteID, nexoSlug, "Organizador Nexo "+suffix, nexoActiveSKU, 30000, "active")
	nexoInactiveSKU := "ORG-NEX-PET-MAR-" + suffix
	insertCatalogVariant(t, pool, nexoID, materialID, marfimID, nexoInactiveSKU, 12000, "inactive")
	var parentStatus, inactiveVariantStatus string
	if err := pool.QueryRow(context.Background(), `SELECT p.status, pv.status FROM products p JOIN product_variants pv ON pv.product_id=p.id WHERE p.id=$1 AND pv.sku=$2`, nexoID, nexoInactiveSKU).Scan(&parentStatus, &inactiveVariantStatus); err != nil {
		t.Fatal("verify inactive-variant fixture in isolated schema")
	}
	if parentStatus != "active" || inactiveVariantStatus != "inactive" {
		t.Fatalf("invalid inactive-variant fixture states: product=%q variant=%q", parentStatus, inactiveVariantStatus)
	}
	repository := NewCatalogRepository(pool)

	minimum, maximum := int64(10000), int64(20000)
	colorSlug := "fixture-marfim-" + suffix
	categorySlug := "fixture-decor-" + suffix
	page, err := repository.ListProducts(context.Background(), domain.ProductFilter{
		Category: categorySlug, Color: colorSlug, MinPriceCents: &minimum, MaxPriceCents: &maximum,
		Sort: domain.SortPriceAscending, Page: 1, Limit: 1,
	})
	if err != nil {
		t.Fatalf("list isolated filtered products: %v", err)
	}
	if page.Total != 1 || len(page.Products) != 1 || page.Products[0].Slug != vasoSlug {
		t.Fatalf("unexpected isolated filtered page: total=%d count=%d", page.Total, len(page.Products))
	}
	if len(page.Products[0].Variants) != 2 {
		t.Fatalf("expected both active commercial variants, got %d", len(page.Products[0].Variants))
	}

	product, err := repository.GetProductBySlug(context.Background(), nexoSlug)
	if err != nil {
		t.Fatalf("get isolated product: %v", err)
	}
	if len(product.Variants) != 1 || product.Variants[0].SKU != nexoActiveSKU {
		t.Fatalf("inactive fixture variant leaked from repository: count=%d", len(product.Variants))
	}

	categories, err := repository.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("list isolated categories: %v", err)
	}
	if len(categories) != 1 || categories[0].Slug != categorySlug {
		t.Fatalf("expected only the fixture category, got %d", len(categories))
	}

	_, err = repository.GetProductBySlug(context.Background(), "produto-inexistente")
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("expected product not found, got %v", err)
	}
}

func TestCatalogRepositoryPagination(t *testing.T) {
	pool := newCatalogIntegrationPool(t)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique catalog fixture identifier")
	}
	suffix := strings.ReplaceAll(fixtureID, "-", "")
	categoryID, materialID, _, grafiteID := insertCatalogReferenceData(t, pool, fixtureID)
	for index := 1; index <= 5; index++ {
		insertCatalogProduct(t, pool, categoryID, materialID, grafiteID,
			fmt.Sprintf("pagination-%s-%02d", suffix, index), fmt.Sprintf("Pagination Product %s %02d", suffix, index), fmt.Sprintf("PAGE-%s-%02d", suffix, index), int64(10000+index), "active")
	}
	repository := NewCatalogRepository(pool)

	first, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("first isolated page: %v", err)
	}
	second, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("second isolated page: %v", err)
	}
	if first.Total != 5 || second.Total != 5 || len(first.Products) != 2 || len(second.Products) != 2 {
		t.Fatalf("unexpected fixture pagination: first total/count=%d/%d, second=%d/%d", first.Total, len(first.Products), second.Total, len(second.Products))
	}
	if first.Products[0].ID == second.Products[0].ID {
		t.Fatal("pagination returned duplicate fixture product")
	}
	empty, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 99, Limit: 2})
	if err != nil {
		t.Fatalf("empty isolated page: %v", err)
	}
	if empty.Total != 5 || len(empty.Products) != 0 {
		t.Fatalf("empty page must preserve fixture total metadata: total=%d count=%d", empty.Total, len(empty.Products))
	}
}
