package postgres

import (
	"context"
	"errors"
	"fmt"

	domain "example.com/loja-3d/backend/internal/domain/editorial"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EditorialRepository struct{ pool *pgxpool.Pool }

func NewEditorialRepository(pool *pgxpool.Pool) *EditorialRepository {
	return &EditorialRepository{pool: pool}
}

const editorialColumns = `ec.id,ec.slug,ec.title,ec.description,ec.eyebrow,ec.edition_label,ec.cover_image_id,coalesce(pi.public_url,pi.object_key,''),ec.status,ec.version,ec.published_at,ec.created_at,ec.updated_at`

func scanEditorial(row pgx.Row) (domain.Catalog, error) {
	var value domain.Catalog
	err := row.Scan(&value.ID, &value.Slug, &value.Title, &value.Description, &value.Eyebrow, &value.EditionLabel, &value.CoverImageID, &value.CoverImageURL, &value.Status, &value.Version, &value.PublishedAt, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, domain.ErrNotFound
	}
	return value, err
}
func (r *EditorialRepository) productIDs(ctx context.Context, id string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT product_id FROM editorial_catalog_products WHERE catalog_id=$1 ORDER BY sort_order`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		ids = append(ids, value)
	}
	return ids, rows.Err()
}
func (r *EditorialRepository) ListPublished(ctx context.Context) ([]domain.Catalog, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+editorialColumns+` FROM editorial_catalogs ec LEFT JOIN product_images pi ON pi.id=ec.cover_image_id WHERE ec.status='published' AND EXISTS(SELECT 1 FROM editorial_catalog_products ecp JOIN products p ON p.id=ecp.product_id JOIN product_variants pv ON pv.product_id=p.id WHERE ecp.catalog_id=ec.id AND p.status='active' AND p.deleted_at IS NULL AND pv.status='active' AND pv.deleted_at IS NULL AND (NOT pv.track_inventory OR pv.stock_quantity-pv.reserved_quantity>0)) ORDER BY ec.published_at DESC,ec.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Catalog{}
	for rows.Next() {
		value, scanErr := scanEditorial(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		value.ProductIDs, _ = r.productIDs(ctx, value.ID)
		items = append(items, value)
	}
	return items, rows.Err()
}
func (r *EditorialRepository) GetPublished(ctx context.Context, slug string) (domain.Catalog, error) {
	value, err := scanEditorial(r.pool.QueryRow(ctx, `SELECT `+editorialColumns+` FROM editorial_catalogs ec LEFT JOIN product_images pi ON pi.id=ec.cover_image_id WHERE ec.slug=$1 AND ec.status='published'`, slug))
	if err == nil {
		value.ProductIDs, err = r.productIDs(ctx, value.ID)
	}
	return value, err
}
func (r *EditorialRepository) ListAdmin(ctx context.Context, search, status string, page, limit int) (domain.Page, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+editorialColumns+`,count(*) OVER() FROM editorial_catalogs ec LEFT JOIN product_images pi ON pi.id=ec.cover_image_id WHERE ($1='' OR ec.title ILIKE '%'||$1||'%' OR ec.slug ILIKE '%'||$1||'%') AND ($2='' OR ec.status=$2) ORDER BY ec.updated_at DESC LIMIT $3 OFFSET $4`, search, status, limit, (page-1)*limit)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()
	result := domain.Page{Items: []domain.Catalog{}, Page: page, Limit: limit}
	for rows.Next() {
		var value domain.Catalog
		if err = rows.Scan(&value.ID, &value.Slug, &value.Title, &value.Description, &value.Eyebrow, &value.EditionLabel, &value.CoverImageID, &value.CoverImageURL, &value.Status, &value.Version, &value.PublishedAt, &value.CreatedAt, &value.UpdatedAt, &result.Total); err != nil {
			return result, err
		}
		value.ProductIDs, _ = r.productIDs(ctx, value.ID)
		result.Items = append(result.Items, value)
	}
	return result, rows.Err()
}
func (r *EditorialRepository) GetAdmin(ctx context.Context, id string) (domain.Catalog, error) {
	value, err := scanEditorial(r.pool.QueryRow(ctx, `SELECT `+editorialColumns+` FROM editorial_catalogs ec LEFT JOIN product_images pi ON pi.id=ec.cover_image_id WHERE ec.id=$1`, id))
	if err == nil {
		value.ProductIDs, err = r.productIDs(ctx, id)
	}
	return value, err
}
func (r *EditorialRepository) Create(ctx context.Context, input domain.Input, userID string) (domain.Catalog, error) {
	value, err := scanEditorial(r.pool.QueryRow(ctx, `INSERT INTO editorial_catalogs(slug,title,description,eyebrow,edition_label,cover_image_id,created_by,updated_by) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id,slug,title,description,eyebrow,edition_label,cover_image_id,'',status,version,published_at,created_at,updated_at`, input.Slug, input.Title, input.Description, input.Eyebrow, input.EditionLabel, input.CoverImageID, userID))
	return value, mapEditorialError(err)
}
func (r *EditorialRepository) Update(ctx context.Context, id string, input domain.Input, userID string) (domain.Catalog, error) {
	value, err := scanEditorial(r.pool.QueryRow(ctx, `UPDATE editorial_catalogs SET slug=$2,title=$3,description=$4,eyebrow=$5,edition_label=$6,cover_image_id=$7,updated_by=$8,updated_at=now(),version=version+1 WHERE id=$1 AND version=$9 RETURNING id,slug,title,description,eyebrow,edition_label,cover_image_id,'',status,version,published_at,created_at,updated_at`, id, input.Slug, input.Title, input.Description, input.Eyebrow, input.EditionLabel, input.CoverImageID, userID, input.ExpectedVersion))
	if errors.Is(err, domain.ErrNotFound) {
		return value, domain.ErrConflict
	}
	return value, mapEditorialError(err)
}
func (r *EditorialRepository) ReplaceProducts(ctx context.Context, id string, ids []string, version int, userID string) (domain.Catalog, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Catalog{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE editorial_catalogs SET updated_by=$3,updated_at=now(),version=version+1 WHERE id=$1 AND version=$2`, id, version, userID)
	if err != nil {
		return domain.Catalog{}, err
	}
	if tag.RowsAffected() != 1 {
		return domain.Catalog{}, domain.ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM editorial_catalog_products WHERE catalog_id=$1`, id); err != nil {
		return domain.Catalog{}, err
	}
	for index, productID := range ids {
		if _, err = tx.Exec(ctx, `INSERT INTO editorial_catalog_products(catalog_id,product_id,sort_order) VALUES($1,$2,$3)`, id, productID, index); err != nil {
			return domain.Catalog{}, mapEditorialError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Catalog{}, err
	}
	return r.GetAdmin(ctx, id)
}
func (r *EditorialRepository) Publish(ctx context.Context, id string, version int, userID string) (domain.Catalog, error) {
	return r.setStatus(ctx, id, version, userID, "published")
}
func (r *EditorialRepository) Unpublish(ctx context.Context, id string, version int, userID string) (domain.Catalog, error) {
	return r.setStatus(ctx, id, version, userID, "draft")
}
func (r *EditorialRepository) setStatus(ctx context.Context, id string, version int, userID, status string) (domain.Catalog, error) {
	value, err := scanEditorial(r.pool.QueryRow(ctx, `UPDATE editorial_catalogs SET status=$3,published_at=CASE WHEN $3='published' THEN coalesce(published_at,now()) ELSE NULL END,updated_by=$4,updated_at=now(),version=version+1 WHERE id=$1 AND version=$2 RETURNING id,slug,title,description,eyebrow,edition_label,cover_image_id,'',status,version,published_at,created_at,updated_at`, id, version, status, userID))
	if errors.Is(err, domain.ErrNotFound) {
		return value, domain.ErrConflict
	}
	return value, err
}
func mapEditorialError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("editorial repository: %w", err)
	}
	return nil
}
