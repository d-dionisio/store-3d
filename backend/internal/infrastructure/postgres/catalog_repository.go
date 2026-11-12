package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	application "example.com/loja-3d/backend/internal/application/catalog"
	domain "example.com/loja-3d/backend/internal/domain/catalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ application.Repository = (*CatalogRepository)(nil)

type CatalogRepository struct{ pool *pgxpool.Pool }

func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{pool: pool}
}

func (repository *CatalogRepository) ListProducts(ctx context.Context, filter domain.ProductFilter) (domain.ProductPage, error) {
	orderBy := map[domain.ProductSort]string{
		domain.SortNameAscending:   "p.name ASC, p.id ASC",
		domain.SortNameDescending:  "p.name DESC, p.id ASC",
		domain.SortPriceAscending:  "candidate.min_price_cents ASC, p.id ASC",
		domain.SortPriceDescending: "candidate.min_price_cents DESC, p.id ASC",
	}[filter.Sort]
	if orderBy == "" {
		orderBy = "p.name ASC, p.id ASC"
	}

	query := `
        WITH candidate AS (
            SELECT pv.product_id, min(pv.price_cents) AS min_price_cents
            FROM product_variants pv
            JOIN materials m ON m.id = pv.material_id AND m.is_active
            JOIN colors c ON c.id = pv.color_id AND c.is_active
            LEFT JOIN product_sizes s ON s.id = pv.size_id AND s.is_active
            LEFT JOIN finishes f ON f.id = pv.finish_id AND f.is_active
            WHERE pv.status = 'active'
              AND pv.deleted_at IS NULL
              AND (NOT pv.track_inventory OR pv.stock_quantity - pv.reserved_quantity > 0)
              AND (pv.size_id IS NULL OR s.id IS NOT NULL)
              AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
              AND ($3 = '' OR c.slug = $3)
              AND ($4::bigint IS NULL OR pv.price_cents >= $4)
              AND ($5::bigint IS NULL OR pv.price_cents <= $5)
            GROUP BY pv.product_id
        )
        SELECT p.id, p.slug, p.name, coalesce(p.short_description, ''), p.description,
               p.is_featured, p.production_type,
               c.id, c.slug, c.name, coalesce(c.description, ''),
               count(*) OVER () AS total_count
        FROM products p
        JOIN categories c ON c.id = p.category_id
        JOIN candidate ON candidate.product_id = p.id
        WHERE p.status = 'active'
          AND p.deleted_at IS NULL
          AND c.is_active
          AND c.deleted_at IS NULL
          AND ($1 = '' OR c.slug = $1)
          AND ($2 = '' OR p.name ILIKE '%' || $2 || '%' OR p.description ILIKE '%' || $2 || '%')
        ORDER BY ` + orderBy + `
        LIMIT $6 OFFSET $7`

	rows, err := repository.pool.Query(ctx, query,
		filter.Category, filter.Search, filter.Color,
		filter.MinPriceCents, filter.MaxPriceCents, filter.Limit, filter.Offset(),
	)
	if err != nil {
		return domain.ProductPage{}, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]domain.Product, 0, filter.Limit)
	var total int64
	for rows.Next() {
		var product domain.Product
		if err := rows.Scan(
			&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.Description,
			&product.IsFeatured, &product.ProductionType,
			&product.Category.ID, &product.Category.Slug, &product.Category.Name, &product.Category.Description,
			&total,
		); err != nil {
			return domain.ProductPage{}, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return domain.ProductPage{}, fmt.Errorf("iterate products: %w", err)
	}
	if len(products) == 0 {
		total, err = repository.countProducts(ctx, filter)
		if err != nil {
			return domain.ProductPage{}, err
		}
	}

	if err := repository.hydrateProducts(ctx, products); err != nil {
		return domain.ProductPage{}, err
	}
	return domain.ProductPage{Products: products, Total: total, Page: filter.Page, Limit: filter.Limit}, nil
}

func (repository *CatalogRepository) countProducts(ctx context.Context, filter domain.ProductFilter) (int64, error) {
	const query = `
        WITH candidate AS (
            SELECT pv.product_id
            FROM product_variants pv
            JOIN materials m ON m.id = pv.material_id AND m.is_active
            JOIN colors color ON color.id = pv.color_id AND color.is_active
            LEFT JOIN product_sizes s ON s.id = pv.size_id AND s.is_active
            LEFT JOIN finishes f ON f.id = pv.finish_id AND f.is_active
            WHERE pv.status = 'active'
              AND pv.deleted_at IS NULL
              AND (NOT pv.track_inventory OR pv.stock_quantity - pv.reserved_quantity > 0)
              AND (pv.size_id IS NULL OR s.id IS NOT NULL)
              AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
              AND ($3 = '' OR color.slug = $3)
              AND ($4::bigint IS NULL OR pv.price_cents >= $4)
              AND ($5::bigint IS NULL OR pv.price_cents <= $5)
            GROUP BY pv.product_id
        )
        SELECT count(*)
        FROM products p
        JOIN categories c ON c.id = p.category_id
        JOIN candidate ON candidate.product_id = p.id
        WHERE p.status = 'active'
          AND p.deleted_at IS NULL
          AND c.is_active
          AND c.deleted_at IS NULL
          AND ($1 = '' OR c.slug = $1)
          AND ($2 = '' OR p.name ILIKE '%' || $2 || '%' OR p.description ILIKE '%' || $2 || '%')`

	var total int64
	if err := repository.pool.QueryRow(ctx, query, filter.Category, filter.Search, filter.Color, filter.MinPriceCents, filter.MaxPriceCents).Scan(&total); err != nil {
		return 0, fmt.Errorf("count products: %w", err)
	}
	return total, nil
}

func (repository *CatalogRepository) GetProductBySlug(ctx context.Context, slug string) (domain.Product, error) {
	const query = `
        SELECT p.id, p.slug, p.name, coalesce(p.short_description, ''), p.description,
               p.is_featured, p.production_type,
               c.id, c.slug, c.name, coalesce(c.description, '')
        FROM products p
        JOIN categories c ON c.id = p.category_id
        WHERE p.slug = $1
          AND p.status = 'active'
          AND p.deleted_at IS NULL
          AND c.is_active
          AND c.deleted_at IS NULL
          AND EXISTS (
              SELECT 1
              FROM product_variants pv
              JOIN materials m ON m.id = pv.material_id AND m.is_active
              JOIN colors color ON color.id = pv.color_id AND color.is_active
              LEFT JOIN product_sizes s ON s.id = pv.size_id AND s.is_active
              LEFT JOIN finishes f ON f.id = pv.finish_id AND f.is_active
              WHERE pv.product_id = p.id
                AND pv.status = 'active'
                AND pv.deleted_at IS NULL
                AND (NOT pv.track_inventory OR pv.stock_quantity - pv.reserved_quantity > 0)
                AND (pv.size_id IS NULL OR s.id IS NOT NULL)
                AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
          )`

	var product domain.Product
	err := repository.pool.QueryRow(ctx, query, slug).Scan(
		&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.Description,
		&product.IsFeatured, &product.ProductionType,
		&product.Category.ID, &product.Category.Slug, &product.Category.Name, &product.Category.Description,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, domain.ErrProductNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("get product by slug: %w", err)
	}

	products := []domain.Product{product}
	if err := repository.hydrateProducts(ctx, products); err != nil {
		return domain.Product{}, err
	}
	return products[0], nil
}

func (repository *CatalogRepository) GetProductsByIDs(ctx context.Context, productIDs []string) ([]domain.Product, error) {
	if len(productIDs) == 0 {
		return []domain.Product{}, nil
	}
	const query = `
        SELECT p.id, p.slug, p.name, coalesce(p.short_description, ''), p.description,
               p.is_featured, p.production_type,
               c.id, c.slug, c.name, coalesce(c.description, '')
        FROM products p
        JOIN categories c ON c.id = p.category_id
        WHERE p.id = ANY($1::uuid[]) AND p.status='active' AND p.deleted_at IS NULL
          AND c.is_active AND c.deleted_at IS NULL
          AND EXISTS (
            SELECT 1 FROM product_variants pv
            JOIN materials m ON m.id=pv.material_id AND m.is_active
            JOIN colors color ON color.id=pv.color_id AND color.is_active
            LEFT JOIN product_sizes s ON s.id=pv.size_id AND s.is_active
            LEFT JOIN finishes f ON f.id=pv.finish_id AND f.is_active
            WHERE pv.product_id=p.id AND pv.status='active' AND pv.deleted_at IS NULL
              AND (NOT pv.track_inventory OR pv.stock_quantity-pv.reserved_quantity>0)
              AND (pv.size_id IS NULL OR s.id IS NOT NULL)
              AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
          )`
	rows, err := repository.pool.Query(ctx, query, productIDs)
	if err != nil {
		return nil, fmt.Errorf("get products by ids: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]domain.Product, len(productIDs))
	for rows.Next() {
		var product domain.Product
		if err = rows.Scan(&product.ID, &product.Slug, &product.Name, &product.ShortDescription, &product.Description, &product.IsFeatured, &product.ProductionType, &product.Category.ID, &product.Category.Slug, &product.Category.Name, &product.Category.Description); err != nil {
			return nil, err
		}
		byID[product.ID] = product
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	products := make([]domain.Product, 0, len(byID))
	for _, id := range productIDs {
		if product, ok := byID[id]; ok {
			products = append(products, product)
		}
	}
	if err = repository.hydrateProducts(ctx, products); err != nil {
		return nil, err
	}
	return products, nil
}

func (repository *CatalogRepository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	const query = `
        SELECT c.id, c.slug, c.name, coalesce(c.description, '')
        FROM categories c
        WHERE c.is_active AND c.deleted_at IS NULL
          AND EXISTS (
              SELECT 1 FROM products p
              JOIN product_variants pv ON pv.product_id = p.id
              JOIN materials m ON m.id = pv.material_id AND m.is_active
              JOIN colors color ON color.id = pv.color_id AND color.is_active
              LEFT JOIN product_sizes s ON s.id = pv.size_id AND s.is_active
              LEFT JOIN finishes f ON f.id = pv.finish_id AND f.is_active
              WHERE p.category_id = c.id
                AND p.status = 'active' AND p.deleted_at IS NULL
                AND pv.status = 'active' AND pv.deleted_at IS NULL
                AND (NOT pv.track_inventory OR pv.stock_quantity - pv.reserved_quantity > 0)
                AND (pv.size_id IS NULL OR s.id IS NOT NULL)
                AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
          )
        ORDER BY c.sort_order, c.name`

	rows, err := repository.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := make([]domain.Category, 0)
	for rows.Next() {
		var category domain.Category
		if err := rows.Scan(&category.ID, &category.Slug, &category.Name, &category.Description); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return categories, nil
}

func (repository *CatalogRepository) hydrateProducts(ctx context.Context, products []domain.Product) error {
	if len(products) == 0 {
		return nil
	}

	ids := make([]pgtype.UUID, 0, len(products))
	positions := make(map[string]int, len(products))
	for index := range products {
		var id pgtype.UUID
		if err := id.Scan(products[index].ID); err != nil {
			return fmt.Errorf("parse product identifier: %w", err)
		}
		ids = append(ids, id)
		positions[products[index].ID] = index
		products[index].Variants = make([]domain.ProductVariant, 0)
		products[index].Images = make([]domain.ProductImage, 0)
	}

	if err := repository.loadVariants(ctx, ids, positions, products); err != nil {
		return err
	}
	return repository.loadImages(ctx, ids, positions, products)
}

func (repository *CatalogRepository) loadVariants(ctx context.Context, ids []pgtype.UUID, positions map[string]int, products []domain.Product) error {
	const query = `
        SELECT pv.product_id, pv.id, pv.sku, coalesce(pv.name, ''), pv.price_cents,
               pv.stock_quantity, pv.stock_quantity - pv.reserved_quantity,
               pv.track_inventory, pv.width_mm, pv.height_mm, pv.depth_mm, pv.weight_grams,
               pv.production_lead_time_min_days, pv.production_lead_time_max_days,
               m.id, m.code, m.name,
               c.id, c.slug, c.name, c.hex_code,
               s.id, s.code, s.name, s.width_mm, s.height_mm, s.depth_mm,
               f.id, f.code, f.name, f.description
        FROM product_variants pv
        JOIN materials m ON m.id = pv.material_id AND m.is_active
        JOIN colors c ON c.id = pv.color_id AND c.is_active
        LEFT JOIN product_sizes s ON s.id = pv.size_id AND s.is_active
        LEFT JOIN finishes f ON f.id = pv.finish_id AND f.is_active
        WHERE pv.product_id = ANY($1::uuid[])
          AND pv.status = 'active'
          AND pv.deleted_at IS NULL
          AND (NOT pv.track_inventory OR pv.stock_quantity - pv.reserved_quantity > 0)
          AND (pv.size_id IS NULL OR s.id IS NOT NULL)
          AND (pv.finish_id IS NULL OR f.id IS NOT NULL)
        ORDER BY pv.product_id, pv.price_cents, pv.sku`

	rows, err := repository.pool.Query(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("load product variants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var productID string
		var variant domain.ProductVariant
		var width, height, depth, weight sql.NullInt32
		var sizeID, sizeCode, sizeName sql.NullString
		var sizeWidth, sizeHeight, sizeDepth sql.NullInt32
		var finishID, finishCode, finishName, finishDescription sql.NullString
		if err := rows.Scan(
			&productID, &variant.ID, &variant.SKU, &variant.Name, &variant.PriceCents,
			&variant.StockQuantity, &variant.AvailableQuantity, &variant.TrackInventory,
			&width, &height, &depth, &weight,
			&variant.LeadTimeMinDays, &variant.LeadTimeMaxDays,
			&variant.Material.ID, &variant.Material.Code, &variant.Material.Name,
			&variant.Color.ID, &variant.Color.Slug, &variant.Color.Name, &variant.Color.HexCode,
			&sizeID, &sizeCode, &sizeName, &sizeWidth, &sizeHeight, &sizeDepth,
			&finishID, &finishCode, &finishName, &finishDescription,
		); err != nil {
			return fmt.Errorf("scan product variant: %w", err)
		}
		variant.Dimensions = domain.Dimensions{WidthMM: int32Pointer(width), HeightMM: int32Pointer(height), DepthMM: int32Pointer(depth)}
		variant.WeightGrams = int32Pointer(weight)
		if sizeID.Valid {
			variant.Size = &domain.Size{ID: sizeID.String, Code: sizeCode.String, Name: sizeName.String, WidthMM: int32Pointer(sizeWidth), HeightMM: int32Pointer(sizeHeight), DepthMM: int32Pointer(sizeDepth)}
		}
		if finishID.Valid {
			variant.Finish = &domain.Finish{ID: finishID.String, Code: finishCode.String, Name: finishName.String, Description: finishDescription.String}
		}
		products[positions[productID]].Variants = append(products[positions[productID]].Variants, variant)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate product variants: %w", err)
	}
	return nil
}

func (repository *CatalogRepository) loadImages(ctx context.Context, ids []pgtype.UUID, positions map[string]int, products []domain.Product) error {
	const query = `
        SELECT product_id, id, variant_id, coalesce(public_url, object_key), alt_text,
               media_type, sort_order, is_primary
        FROM product_images
        WHERE product_id = ANY($1::uuid[])
        ORDER BY product_id, is_primary DESC, sort_order, id`

	rows, err := repository.pool.Query(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("load product images: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var productID string
		var image domain.ProductImage
		var variantID sql.NullString
		if err := rows.Scan(&productID, &image.ID, &variantID, &image.URL, &image.AltText, &image.MediaType, &image.SortOrder, &image.IsPrimary); err != nil {
			return fmt.Errorf("scan product image: %w", err)
		}
		if variantID.Valid {
			image.VariantID = &variantID.String
		}
		products[positions[productID]].Images = append(products[positions[productID]].Images, image)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate product images: %w", err)
	}
	return nil
}

func int32Pointer(value sql.NullInt32) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}
