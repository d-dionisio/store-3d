package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	app "example.com/loja-3d/backend/internal/application/quote"
	domain "example.com/loja-3d/backend/internal/domain/quote"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type QuoteRepository struct{ pool *pgxpool.Pool }

func NewQuoteRepository(pool *pgxpool.Pool) *QuoteRepository { return &QuoteRepository{pool: pool} }
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func (r *QuoteRepository) Options(ctx context.Context) ([]domain.Options, []domain.Options, error) {
	materials := []domain.Options{}
	colors := []domain.Options{}
	rows, err := r.pool.Query(ctx, `SELECT id,code,name FROM materials WHERE is_active ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var o domain.Options
		if err := rows.Scan(&o.ID, &o.Code, &o.Name); err != nil {
			return nil, nil, err
		}
		materials = append(materials, o)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows2, err := r.pool.Query(ctx, `SELECT id,slug,name,hex_code FROM colors WHERE is_active ORDER BY name`)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var o domain.Options
		if err := rows2.Scan(&o.ID, &o.Slug, &o.Name, &o.HexCode); err != nil {
			return nil, nil, err
		}
		colors = append(colors, o)
	}
	return materials, colors, rows2.Err()
}
func (r *QuoteRepository) CreateUpload(ctx context.Context, req domain.UploadRequest, ext, ownerHash, bucket string, expires time.Time) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO quote_uploads(owner_token_hash,bucket_name,object_key,original_filename,file_extension,media_type,expected_size_bytes,checksum_sha256,expires_at) VALUES($1,$2,'pending',$3,$4,$5,$6,$7,$8) RETURNING id`, ownerHash, bucket, req.Filename, ext, req.ContentType, req.Size, req.Checksum, expires).Scan(&id)
	if err != nil {
		return "", err
	}
	key := "quotes/pending/" + id + "." + ext
	_, err = r.pool.Exec(ctx, `UPDATE quote_uploads SET object_key=$2 WHERE id=$1`, id, key)
	return id, err
}
func (r *QuoteRepository) ConfirmUpload(ctx context.Context, id, owner string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE quote_uploads SET status='confirmed',confirmed_at=now() WHERE id=$1 AND owner_token_hash=$2 AND status='authorized' AND expires_at>now()`, id, hashToken(owner))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return app.ErrForbidden
	}
	return nil
}
func (r *QuoteRepository) GetUpload(ctx context.Context, id, owner string) (app.Upload, error) {
	var u app.Upload
	err := r.pool.QueryRow(ctx, `SELECT id,object_key,media_type,checksum_sha256,expected_size_bytes,expires_at FROM quote_uploads WHERE id=$1 AND owner_token_hash=$2 AND status='authorized' AND expires_at>now()`, id, hashToken(owner)).Scan(&u.ID, &u.ObjectKey, &u.ContentType, &u.Checksum, &u.Size, &u.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, app.ErrForbidden
	}
	return u, err
}
func (r *QuoteRepository) Create(ctx context.Context, req domain.CreateRequest, accessHash string) (domain.Created, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Created{}, err
	}
	defer tx.Rollback(ctx)
	var bucket, key, filename, ext, media, checksum string
	var size int64
	err = tx.QueryRow(ctx, `SELECT bucket_name,object_key,original_filename,file_extension,media_type,expected_size_bytes,checksum_sha256 FROM quote_uploads WHERE id=$1 AND owner_token_hash=$2 AND status='confirmed' AND expires_at>now() FOR UPDATE`, req.UploadID, hashToken(req.UploadToken)).Scan(&bucket, &key, &filename, &ext, &media, &size, &checksum)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Created{}, app.ErrForbidden
	}
	if err != nil {
		return domain.Created{}, err
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM materials WHERE id=$1 AND is_active) AND EXISTS(SELECT 1 FROM colors WHERE id=$2 AND is_active)`, req.MaterialID, req.ColorID).Scan(&valid)
	if err != nil || !valid {
		if err == nil {
			err = domain.ErrInvalid
		}
		return domain.Created{}, err
	}
	var id string
	reference := "ORC-" + strings.ToUpper(strings.ReplaceAll(req.UploadID, "-", "")[:16])
	err = tx.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,customer_phone,status,material_id,color_id,requested_quantity,width_mm,height_mm,depth_mm,notes,submitted_at,public_access_token_hash) VALUES($1,$2,lower($3),nullif($4,''),'submitted',$5,$6,$7,$8,$9,$10,nullif($11,''),now(),$12) RETURNING id`, reference, strings.TrimSpace(req.Name), strings.TrimSpace(req.Email), strings.TrimSpace(req.Phone), req.MaterialID, req.ColorID, req.Quantity, req.WidthMM, req.HeightMM, req.DepthMM, strings.TrimSpace(req.Notes), accessHash).Scan(&id)
	if err != nil {
		return domain.Created{}, fmt.Errorf("insert quote: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,retention_until) VALUES($1,'r2',$2,$3,$4,$5,$6,$7,$8,'pending',now()+interval '90 days')`, id, bucket, key, filename, ext, media, size, checksum)
	if err != nil {
		return domain.Created{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE quote_uploads SET status='consumed',consumed_at=now() WHERE id=$1`, req.UploadID)
	if err != nil {
		return domain.Created{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Created{}, err
	}
	return domain.Created{Reference: reference}, nil
}

func (r *QuoteRepository) Get(ctx context.Context, reference, access string) (string, time.Time, error) {
	var status string
	var submitted time.Time
	err := r.pool.QueryRow(ctx, `SELECT status,submitted_at FROM quotes WHERE quote_number=$1 AND public_access_token_hash=$2`, reference, hashToken(access)).Scan(&status, &submitted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, app.ErrNotFound
	}
	return status, submitted, err
}
