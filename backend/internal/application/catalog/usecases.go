package catalog

import (
	"context"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type ListProducts struct{ repository Repository }

func NewListProducts(repository Repository) *ListProducts {
	return &ListProducts{repository: repository}
}

func (useCase *ListProducts) Execute(ctx context.Context, filter domain.ProductFilter) (domain.ProductPage, error) {
	return useCase.repository.ListProducts(ctx, filter)
}

type GetProductBySlug struct{ repository Repository }

func NewGetProductBySlug(repository Repository) *GetProductBySlug {
	return &GetProductBySlug{repository: repository}
}

func (useCase *GetProductBySlug) Execute(ctx context.Context, slug string) (domain.Product, error) {
	return useCase.repository.GetProductBySlug(ctx, slug)
}

type ListCategories struct{ repository Repository }

func NewListCategories(repository Repository) *ListCategories {
	return &ListCategories{repository: repository}
}

func (useCase *ListCategories) Execute(ctx context.Context) ([]domain.Category, error) {
	return useCase.repository.ListCategories(ctx)
}
