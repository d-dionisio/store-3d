package catalog

import (
	"context"
	"errors"
	"testing"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type repositoryStub struct {
	page       domain.ProductPage
	product    domain.Product
	categories []domain.Category
	err        error
}

func (stub repositoryStub) ListProducts(context.Context, domain.ProductFilter) (domain.ProductPage, error) {
	return stub.page, stub.err
}
func (stub repositoryStub) GetProductBySlug(context.Context, string) (domain.Product, error) {
	return stub.product, stub.err
}
func (stub repositoryStub) ListCategories(context.Context) ([]domain.Category, error) {
	return stub.categories, stub.err
}

func TestCatalogUseCasesDelegateToRepository(t *testing.T) {
	t.Parallel()

	stub := repositoryStub{
		page:       domain.ProductPage{Total: 1},
		product:    domain.Product{Slug: "vaso"},
		categories: []domain.Category{{Slug: "decoracao"}},
	}

	page, err := NewListProducts(stub).Execute(context.Background(), domain.ProductFilter{Page: 1, Limit: 20})
	if err != nil || page.Total != 1 {
		t.Fatalf("unexpected list result: %+v, %v", page, err)
	}
	product, err := NewGetProductBySlug(stub).Execute(context.Background(), "vaso")
	if err != nil || product.Slug != "vaso" {
		t.Fatalf("unexpected product result: %+v, %v", product, err)
	}
	categories, err := NewListCategories(stub).Execute(context.Background())
	if err != nil || len(categories) != 1 {
		t.Fatalf("unexpected categories result: %+v, %v", categories, err)
	}
}

func TestCatalogUseCasesPropagateErrors(t *testing.T) {
	t.Parallel()

	expected := errors.New("repository failed")
	_, err := NewListProducts(repositoryStub{err: expected}).Execute(context.Background(), domain.ProductFilter{})
	if !errors.Is(err, expected) {
		t.Fatalf("expected repository error, got %v", err)
	}
}
