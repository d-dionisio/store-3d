package catalog

import (
	"context"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type Repository interface {
	ListProducts(context.Context, domain.ProductFilter) (domain.ProductPage, error)
	GetProductBySlug(context.Context, string) (domain.Product, error)
	ListCategories(context.Context) ([]domain.Category, error)
}
