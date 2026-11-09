package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

func TestCatalogRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	config := testConfig(databaseURL)
	config.ConnectTimeout = 3 * time.Second
	pool, err := NewPool(context.Background(), config)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	repository := NewCatalogRepository(pool)

	minimum := int64(10000)
	maximum := int64(20000)
	page, err := repository.ListProducts(context.Background(), domain.ProductFilter{
		Category: "decoracao", Color: "marfim", MinPriceCents: &minimum, MaxPriceCents: &maximum,
		Sort: domain.SortPriceAscending, Page: 1, Limit: 1,
	})
	if err != nil {
		t.Fatalf("list filtered products: %v", err)
	}
	if page.Total != 1 || len(page.Products) != 1 || page.Products[0].Slug != "vaso-orbita" {
		t.Fatalf("unexpected filtered page: %+v", page)
	}
	if len(page.Products[0].Variants) != 2 {
		t.Fatalf("expected all active commercial variants, got %d", len(page.Products[0].Variants))
	}

	product, err := repository.GetProductBySlug(context.Background(), "organizador-nexo")
	if err != nil {
		t.Fatalf("get product: %v", err)
	}
	if len(product.Variants) != 1 || product.Variants[0].SKU != "ORG-NEX-PET-GRA" {
		t.Fatalf("inactive variant leaked from repository: %+v", product.Variants)
	}

	categories, err := repository.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	if len(categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(categories))
	}

	_, err = repository.GetProductBySlug(context.Background(), "produto-inexistente")
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("expected product not found, got %v", err)
	}
}

func TestCatalogRepositoryPagination(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	config := testConfig(databaseURL)
	pool, err := NewPool(context.Background(), config)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	repository := NewCatalogRepository(pool)

	first, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	second, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if first.Total != 5 || second.Total != 5 || len(first.Products) != 2 || len(second.Products) != 2 {
		t.Fatalf("unexpected pagination: first=%+v second=%+v", first, second)
	}
	if first.Products[0].ID == second.Products[0].ID {
		t.Fatal("pagination returned duplicate first item")
	}
	empty, err := repository.ListProducts(context.Background(), domain.ProductFilter{Sort: domain.SortNameAscending, Page: 99, Limit: 2})
	if err != nil {
		t.Fatalf("empty page: %v", err)
	}
	if empty.Total != 5 || len(empty.Products) != 0 {
		t.Fatalf("empty page must preserve total metadata: %+v", empty)
	}
}
