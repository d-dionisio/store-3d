package editorial

import (
	"context"
	"strings"

	admin "example.com/loja-3d/backend/internal/domain/admin"
	catalog "example.com/loja-3d/backend/internal/domain/catalog"
	domain "example.com/loja-3d/backend/internal/domain/editorial"
)

type Repository interface {
	ListPublished(context.Context) ([]domain.Catalog, error)
	GetPublished(context.Context, string) (domain.Catalog, error)
	ListAdmin(context.Context, string, string, int, int) (domain.Page, error)
	GetAdmin(context.Context, string) (domain.Catalog, error)
	Create(context.Context, domain.Input, string) (domain.Catalog, error)
	Update(context.Context, string, domain.Input, string) (domain.Catalog, error)
	ReplaceProducts(context.Context, string, []string, int, string) (domain.Catalog, error)
	Publish(context.Context, string, int, string) (domain.Catalog, error)
	Unpublish(context.Context, string, int, string) (domain.Catalog, error)
}

type ProductReader interface {
	GetProductsByIDs(context.Context, []string) ([]catalog.Product, error)
}

type Service struct {
	repository Repository
	products   ProductReader
}

func New(repository Repository, products ProductReader) *Service {
	return &Service{repository: repository, products: products}
}

func (s *Service) ListPublished(ctx context.Context) ([]domain.Catalog, error) {
	items, err := s.repository.ListPublished(ctx)
	if err != nil {
		return nil, err
	}
	visible := make([]domain.Catalog, 0, len(items))
	for index := range items {
		if err = s.hydrate(ctx, &items[index]); err != nil {
			return nil, err
		}
		if len(items[index].Products) > 0 {
			visible = append(visible, items[index])
		}
	}
	return visible, nil
}
func (s *Service) GetPublished(ctx context.Context, slug string) (domain.Catalog, error) {
	value, err := s.repository.GetPublished(ctx, slug)
	if err != nil {
		return value, err
	}
	if err = s.hydrate(ctx, &value); err != nil {
		return value, err
	}
	if len(value.Products) == 0 {
		return domain.Catalog{}, domain.ErrNotFound
	}
	return value, nil
}
func (s *Service) ListAdmin(ctx context.Context, user admin.Session, search, status string, page, limit int) (domain.Page, error) {
	if user.Role != "admin" {
		return domain.Page{}, domain.ErrInvalid
	}
	if page < 1 || limit < 1 || limit > 100 || (status != "" && status != "draft" && status != "published") {
		return domain.Page{}, domain.ErrInvalid
	}
	return s.repository.ListAdmin(ctx, strings.TrimSpace(search), status, page, limit)
}
func (s *Service) GetAdmin(ctx context.Context, user admin.Session, id string) (domain.Catalog, error) {
	if user.Role != "admin" {
		return domain.Catalog{}, domain.ErrInvalid
	}
	value, err := s.repository.GetAdmin(ctx, id)
	if err != nil {
		return value, err
	}
	err = s.hydrate(ctx, &value)
	return value, err
}
func (s *Service) Create(ctx context.Context, user admin.Session, input domain.Input) (domain.Catalog, error) {
	if user.Role != "admin" || input.Validate() != nil {
		return domain.Catalog{}, domain.ErrInvalid
	}
	return s.repository.Create(ctx, input, user.UserID)
}
func (s *Service) Update(ctx context.Context, user admin.Session, id string, input domain.Input) (domain.Catalog, error) {
	if user.Role != "admin" || input.ExpectedVersion < 1 || input.Validate() != nil {
		return domain.Catalog{}, domain.ErrInvalid
	}
	return s.repository.Update(ctx, id, input, user.UserID)
}
func (s *Service) ReplaceProducts(ctx context.Context, user admin.Session, id string, ids []string, version int) (domain.Catalog, error) {
	if user.Role != "admin" || version < 1 || duplicate(ids) {
		return domain.Catalog{}, domain.ErrInvalid
	}
	if len(ids) == 0 {
		current, err := s.repository.GetAdmin(ctx, id)
		if err != nil {
			return domain.Catalog{}, err
		}
		if current.Status == "published" {
			return domain.Catalog{}, domain.ErrEmpty
		}
	}
	products, err := s.products.GetProductsByIDs(ctx, ids)
	if err != nil {
		return domain.Catalog{}, err
	}
	if len(products) != len(ids) {
		return domain.Catalog{}, domain.ErrInactiveProduct
	}
	return s.repository.ReplaceProducts(ctx, id, ids, version, user.UserID)
}
func (s *Service) Publish(ctx context.Context, user admin.Session, id string, version int) (domain.Catalog, error) {
	if user.Role != "admin" || version < 1 {
		return domain.Catalog{}, domain.ErrInvalid
	}
	value, err := s.repository.GetAdmin(ctx, id)
	if err != nil {
		return value, err
	}
	products, err := s.products.GetProductsByIDs(ctx, value.ProductIDs)
	if err != nil {
		return value, err
	}
	if len(products) == 0 {
		return domain.Catalog{}, domain.ErrEmpty
	}
	if len(products) != len(value.ProductIDs) {
		return domain.Catalog{}, domain.ErrInactiveProduct
	}
	return s.repository.Publish(ctx, id, version, user.UserID)
}
func (s *Service) Unpublish(ctx context.Context, user admin.Session, id string, version int) (domain.Catalog, error) {
	if user.Role != "admin" || version < 1 {
		return domain.Catalog{}, domain.ErrInvalid
	}
	return s.repository.Unpublish(ctx, id, version, user.UserID)
}
func (s *Service) hydrate(ctx context.Context, value *domain.Catalog) error {
	products, err := s.products.GetProductsByIDs(ctx, value.ProductIDs)
	if err != nil {
		return err
	}
	value.Products = products
	if value.CoverImageURL == "" && len(products) > 0 && len(products[0].Images) > 0 {
		value.CoverImageURL = products[0].Images[0].URL
	}
	return nil
}
func duplicate(ids []string) bool {
	seen := map[string]struct{}{}
	for _, id := range ids {
		if id == "" {
			return true
		}
		if _, ok := seen[id]; ok {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}
