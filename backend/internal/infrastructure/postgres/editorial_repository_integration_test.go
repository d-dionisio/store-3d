package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/editorial"
	admin "example.com/loja-3d/backend/internal/domain/admin"
	domain "example.com/loja-3d/backend/internal/domain/editorial"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEditorialCatalogLifecycleIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var userID string
	if err = pool.QueryRow(ctx, `SELECT id FROM admin_users WHERE role='admin' AND status='active' LIMIT 1`).Scan(&userID); err != nil {
		t.Skip("active admin fixture is required")
	}
	rows, err := pool.Query(ctx, `SELECT p.id FROM products p WHERE p.status='active' AND p.deleted_at IS NULL AND EXISTS(SELECT 1 FROM product_variants pv WHERE pv.product_id=p.id AND pv.status='active' AND pv.deleted_at IS NULL AND (NOT pv.track_inventory OR pv.stock_quantity-pv.reserved_quantity>0)) ORDER BY p.id LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 2 {
		t.Skip("two commercial products are required")
	}
	var categoryID string
	if err = pool.QueryRow(ctx, `SELECT id FROM categories WHERE is_active LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	suffix := time.Now().UnixNano()
	inactiveSlug := fmt.Sprintf("editorial-inactive-%d", suffix)
	var inactiveID string
	if err = pool.QueryRow(ctx, `INSERT INTO products(category_id,slug,name,description,status) VALUES($1,$2,'Inactive test','Test fixture','draft') RETURNING id`, categoryID, inactiveSlug).Scan(&inactiveID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM editorial_catalogs WHERE slug LIKE 'integration-editorial-%'`)
		_, _ = pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, inactiveID)
	})

	repository := NewEditorialRepository(pool)
	service := app.New(repository, NewCatalogRepository(pool))
	session := admin.Session{UserID: userID, Role: "admin"}
	input := domain.Input{Slug: fmt.Sprintf("integration-editorial-%d", suffix), Title: "Integration edition", Description: "Database integration test", Eyebrow: "Edition"}
	created, err := service.Create(ctx, session, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Create(ctx, session, input); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate slug: %v", err)
	}
	if _, err = service.Publish(ctx, session, created.ID, created.Version); !errors.Is(err, domain.ErrEmpty) {
		t.Fatalf("empty publication: %v", err)
	}
	if _, err = service.ReplaceProducts(ctx, session, created.ID, []string{inactiveID}, created.Version); !errors.Is(err, domain.ErrInactiveProduct) {
		t.Fatalf("inactive product: %v", err)
	}
	if _, err = service.ReplaceProducts(ctx, session, created.ID, []string{"00000000-0000-0000-0000-000000000000"}, created.Version); !errors.Is(err, domain.ErrInactiveProduct) {
		t.Fatalf("missing product: %v", err)
	}
	ordered, err := service.ReplaceProducts(ctx, session, created.ID, []string{ids[1], ids[0]}, created.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered.ProductIDs) != 2 || ordered.ProductIDs[0] != ids[1] {
		t.Fatalf("order was not preserved: %#v", ordered.ProductIDs)
	}
	if _, err = service.Update(ctx, session, created.ID, domain.Input{Slug: input.Slug, Title: "Concurrent", Description: input.Description, Eyebrow: input.Eyebrow, ExpectedVersion: created.Version}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale update: %v", err)
	}
	published, err := service.Publish(ctx, session, created.ID, ordered.Version)
	if err != nil {
		t.Fatal(err)
	}
	public, err := service.GetPublished(ctx, input.Slug)
	if err != nil || len(public.Products) != 2 {
		t.Fatalf("public catalog: products=%d err=%v", len(public.Products), err)
	}
	draft, err := service.Unpublish(ctx, session, created.ID, published.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.GetPublished(ctx, input.Slug); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("draft leaked publicly: %v", err)
	}
	if draft.Status != "draft" || draft.PublishedAt != nil {
		t.Fatal("unpublish state is inconsistent")
	}
}
