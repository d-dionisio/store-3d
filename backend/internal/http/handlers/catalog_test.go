package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type listProductsStub struct {
	page   domain.ProductPage
	err    error
	filter domain.ProductFilter
}

func (stub *listProductsStub) Execute(_ context.Context, filter domain.ProductFilter) (domain.ProductPage, error) {
	stub.filter = filter
	return stub.page, stub.err
}

type getProductStub struct {
	product domain.Product
	err     error
}

func (stub getProductStub) Execute(context.Context, string) (domain.Product, error) {
	return stub.product, stub.err
}

type listCategoriesStub struct {
	categories []domain.Category
	err        error
}

func (stub listCategoriesStub) Execute(context.Context) ([]domain.Category, error) {
	return stub.categories, stub.err
}

func TestListProductsReturnsDataAndPagination(t *testing.T) {
	t.Parallel()

	list := &listProductsStub{page: domain.ProductPage{
		Products: []domain.Product{testProduct()}, Total: 21, Page: 2, Limit: 10,
	}}
	handler := NewCatalogHandler(list, getProductStub{}, listCategoriesStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products?category=decoracao&color=marfim&min_price=1000&max_price=9000&sort=price_desc&page=2&limit=10", nil)
	response := httptest.NewRecorder()

	handler.ListProducts(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if list.filter.Category != "decoracao" || list.filter.Color != "marfim" || list.filter.Page != 2 || list.filter.Limit != 10 || list.filter.Sort != domain.SortPriceDescending {
		t.Fatalf("unexpected parsed filter: %+v", list.filter)
	}
	var body struct {
		Data []productDTO `json:"data"`
		Meta struct {
			TotalPages int64 `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].MinPriceCents != 2500 || body.Meta.TotalPages != 3 {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestListProductsRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	tests := []string{
		"?page=0", "?limit=101", "?min_price=-1", "?max_price=nope",
		"?min_price=200&max_price=100", "?sort=drop_table",
	}
	for _, query := range tests {
		query := query
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			handler := NewCatalogHandler(&listProductsStub{}, getProductStub{}, listCategoriesStub{})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/products"+query, nil)
			response := httptest.NewRecorder()
			handler.ListProducts(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", response.Code)
			}
		})
	}
}

func TestGetProductNotFound(t *testing.T) {
	t.Parallel()

	handler := NewCatalogHandler(&listProductsStub{}, getProductStub{err: domain.ErrProductNotFound}, listCategoriesStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products/inexistente", nil)
	request.SetPathValue("slug", "inexistente")
	response := httptest.NewRecorder()
	handler.GetProduct(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}

func TestCatalogHandlersHideInternalErrors(t *testing.T) {
	t.Parallel()

	handler := NewCatalogHandler(&listProductsStub{err: errors.New("sql secret")}, getProductStub{}, listCategoriesStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	response := httptest.NewRecorder()
	handler.ListProducts(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", response.Code)
	}
	if body := response.Body.String(); body == "" || contains(body, "sql secret") {
		t.Fatalf("internal error leaked in response: %s", body)
	}
}

func TestListCategories(t *testing.T) {
	t.Parallel()

	handler := NewCatalogHandler(&listProductsStub{}, getProductStub{}, listCategoriesStub{categories: []domain.Category{{ID: "c1", Slug: "decoracao", Name: "Decoração"}}})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	response := httptest.NewRecorder()
	handler.ListCategories(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
}

func testProduct() domain.Product {
	return domain.Product{
		ID: "p1", Slug: "vaso", Name: "Vaso", Description: "Vaso demonstrativo",
		Category: domain.Category{ID: "c1", Slug: "decoracao", Name: "Decoração"},
		Images:   []domain.ProductImage{},
		Variants: []domain.ProductVariant{{
			ID: "v1", SKU: "VASO-1", PriceCents: 2500, TrackInventory: true, AvailableQuantity: 1,
			Material: domain.Material{ID: "m1", Code: "PLA", Name: "PLA"},
			Color:    domain.Color{ID: "co1", Slug: "marfim", Name: "Marfim", HexCode: "#F7F5F0"},
		}},
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
