package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	domain "example.com/loja-3d/backend/internal/domain/catalog"
)

type listProductsQuery interface {
	Execute(context.Context, domain.ProductFilter) (domain.ProductPage, error)
}

type getProductQuery interface {
	Execute(context.Context, string) (domain.Product, error)
}

type listCategoriesQuery interface {
	Execute(context.Context) ([]domain.Category, error)
}

type CatalogHandler struct {
	listProducts   listProductsQuery
	getProduct     getProductQuery
	listCategories listCategoriesQuery
}

func NewCatalogHandler(list listProductsQuery, get getProductQuery, categories listCategoriesQuery) *CatalogHandler {
	return &CatalogHandler{listProducts: list, getProduct: get, listCategories: categories}
}

func (handler *CatalogHandler) ListProducts(response http.ResponseWriter, request *http.Request) {
	filter, err := parseProductFilter(request)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	page, err := handler.listProducts.Execute(request.Context(), filter)
	if err != nil {
		slog.Error("list products failed")
		writeError(response, http.StatusInternalServerError, "internal_error", "não foi possível consultar os produtos")
		return
	}

	products := make([]productDTO, 0, len(page.Products))
	for _, product := range page.Products {
		products = append(products, toProductDTO(product))
	}
	totalPages := int64(0)
	if page.Total > 0 {
		totalPages = (page.Total + int64(page.Limit) - 1) / int64(page.Limit)
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"data": products,
		"meta": map[string]any{"page": page.Page, "limit": page.Limit, "total": page.Total, "total_pages": totalPages},
	})
}

func (handler *CatalogHandler) GetProduct(response http.ResponseWriter, request *http.Request) {
	slug := strings.TrimSpace(request.PathValue("slug"))
	if slug == "" || len(slug) > 160 {
		writeError(response, http.StatusBadRequest, "invalid_request", "slug inválido")
		return
	}

	product, err := handler.getProduct.Execute(request.Context(), slug)
	if errors.Is(err, domain.ErrProductNotFound) {
		writeError(response, http.StatusNotFound, "product_not_found", "produto não encontrado")
		return
	}
	if err != nil {
		slog.Error("get product failed")
		writeError(response, http.StatusInternalServerError, "internal_error", "não foi possível consultar o produto")
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"data": toProductDTO(product)})
}

func (handler *CatalogHandler) ListCategories(response http.ResponseWriter, request *http.Request) {
	categories, err := handler.listCategories.Execute(request.Context())
	if err != nil {
		slog.Error("list categories failed")
		writeError(response, http.StatusInternalServerError, "internal_error", "não foi possível consultar as categorias")
		return
	}
	result := make([]categoryDTO, 0, len(categories))
	for _, category := range categories {
		result = append(result, categoryDTO{ID: category.ID, Slug: category.Slug, Name: category.Name, Description: category.Description})
	}
	writeJSON(response, http.StatusOK, map[string]any{"data": result})
}

func parseProductFilter(request *http.Request) (domain.ProductFilter, error) {
	query := request.URL.Query()
	page, err := positiveInt(query.Get("page"), 1, 0)
	if err != nil {
		return domain.ProductFilter{}, errors.New("page deve ser um inteiro positivo")
	}
	limit, err := positiveInt(query.Get("limit"), 20, 100)
	if err != nil {
		return domain.ProductFilter{}, errors.New("limit deve estar entre 1 e 100")
	}
	minPrice, err := optionalMoney(query.Get("min_price"))
	if err != nil {
		return domain.ProductFilter{}, errors.New("min_price deve ser um inteiro não negativo em centavos")
	}
	maxPrice, err := optionalMoney(query.Get("max_price"))
	if err != nil {
		return domain.ProductFilter{}, errors.New("max_price deve ser um inteiro não negativo em centavos")
	}
	if minPrice != nil && maxPrice != nil && *minPrice > *maxPrice {
		return domain.ProductFilter{}, errors.New("min_price não pode ser maior que max_price")
	}

	sortValue := query.Get("sort")
	if sortValue == "" {
		sortValue = string(domain.SortNameAscending)
	}
	sort := domain.ProductSort(sortValue)
	if sort != domain.SortNameAscending && sort != domain.SortNameDescending && sort != domain.SortPriceAscending && sort != domain.SortPriceDescending {
		return domain.ProductFilter{}, errors.New("sort deve ser name_asc, name_desc, price_asc ou price_desc")
	}

	category := strings.TrimSpace(query.Get("category"))
	color := strings.TrimSpace(query.Get("color"))
	search := strings.TrimSpace(query.Get("search"))
	if len(category) > 100 || len(color) > 100 || len(search) > 200 {
		return domain.ProductFilter{}, errors.New("filtro excede o tamanho máximo permitido")
	}
	return domain.ProductFilter{Category: category, Search: search, Color: color, MinPriceCents: minPrice, MaxPriceCents: maxPrice, Sort: sort, Page: page, Limit: limit}, nil
}

func positiveInt(raw string, fallback, maximum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 || (maximum > 0 && value > maximum) {
		return 0, errors.New("invalid positive integer")
	}
	return value, nil
}

func optionalMoney(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return nil, errors.New("invalid money value")
	}
	return &value, nil
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
