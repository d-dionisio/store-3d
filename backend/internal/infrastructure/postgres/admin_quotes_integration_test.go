package postgres

import (
	"context"
	"testing"
)

func TestAdminQuoteListFiltersPaginationAndDetailIntegration(t *testing.T) {
	ctx := context.Background()
	pool := adminPool(t)
	var material, color string
	if err := pool.QueryRow(ctx, `SELECT id FROM materials LIMIT 1`).Scan(&material); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM colors LIMIT 1`).Scan(&color); err != nil {
		t.Fatal(err)
	}
	var quoteID string
	err := pool.QueryRow(ctx, `INSERT INTO quotes(quote_number,customer_name,customer_email,status,material_id,color_id,requested_quantity,submitted_at) VALUES('ORC-ADMIN-TEST','Cliente Privado','private@example.com','submitted',$1,$2,2,now()) RETURNING id`, material, color).Scan(&quoteID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO quote_files(quote_id,storage_provider,bucket_name,object_key,original_filename,file_extension,media_type,size_bytes,checksum_sha256,security_status,retention_until) VALUES($1,'r2','private','admin-test-object','private.stl','stl','model/stl',84,$2,'pending',now()+interval '1 day')`, quoteID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM quotes WHERE id=$1`, quoteID) })
	repo := NewAdminRepository(pool)
	page, err := repo.ListQuotes(ctx, "Cliente Privado", "submitted", "newest", 1, 1)
	if err != nil || page.Total < 1 || len(page.Items) != 1 {
		t.Fatalf("list failed: %+v %v", page, err)
	}
	detail, err := repo.GetQuote(ctx, "ORC-ADMIN-TEST")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CustomerEmail != "private@example.com" || len(detail.Files) != 1 || detail.Files[0].SecurityStatus != "pending" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}
