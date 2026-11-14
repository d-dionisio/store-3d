package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// productionFixtureRoots contains only exact IDs created by one integration
// test. Cleanup intentionally never accepts prefixes or broad predicates.
type productionFixtureRoots struct {
	OrderID, UserID, AdminID         string
	PrinterID, SpoolID               string
	VariantID, ProductID, CategoryID string
	MaterialID, ColorID              string
}

func registerProductionFixtureCleanup(t *testing.T, pool *pgxpool.Pool, roots *productionFixtureRoots) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Errorf("acquire fixture cleanup connection: %v", err)
			return
		}
		defer conn.Release()
		if _, err = conn.Exec(ctx, `SET session_replication_role=replica`); err != nil {
			t.Errorf("disable fixture cleanup triggers: %v", err)
			return
		}
		defer conn.Exec(ctx, `RESET session_replication_role`)

		queries := []struct {
			sql string
			arg string
		}{
			{`DELETE FROM quality_inspections WHERE production_run_id IN (SELECT pr.id FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1)`, roots.OrderID},
			{`DELETE FROM production_run_materials WHERE production_run_id IN (SELECT pr.id FROM production_runs pr JOIN production_jobs pj ON pj.id=pr.production_job_id JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1)`, roots.OrderID},
			{`DELETE FROM production_events WHERE production_job_id IN (SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1)`, roots.OrderID},
			{`DELETE FROM production_runs WHERE production_job_id IN (SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1)`, roots.OrderID},
			{`DELETE FROM production_job_legacy_links WHERE job_id IN (SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1) OR assigned_parent_job_id IN (SELECT pj.id FROM production_jobs pj JOIN order_items oi ON oi.id=pj.order_item_id WHERE oi.order_id=$1)`, roots.OrderID},
			{`DELETE FROM production_jobs WHERE order_item_id IN (SELECT id FROM order_items WHERE order_id=$1)`, roots.OrderID},
			{`DELETE FROM material_spool_adjustments WHERE material_spool_id=$1`, roots.SpoolID},
			{`DELETE FROM printer_maintenance_events WHERE printer_id=$1`, roots.PrinterID},
			{`DELETE FROM payment_status_events WHERE payment_id IN (SELECT id FROM payments WHERE order_id=$1)`, roots.OrderID},
			{`DELETE FROM payment_refunds WHERE payment_id IN (SELECT id FROM payments WHERE order_id=$1)`, roots.OrderID},
			{`DELETE FROM payment_attempts WHERE payment_id IN (SELECT id FROM payments WHERE order_id=$1)`, roots.OrderID},
			{`DELETE FROM payments WHERE order_id=$1`, roots.OrderID},
			{`DELETE FROM order_status_events WHERE order_id=$1`, roots.OrderID},
			{`DELETE FROM order_items WHERE order_id=$1`, roots.OrderID},
			{`DELETE FROM orders WHERE id=$1`, roots.OrderID},
			{`DELETE FROM admin_audit_events WHERE admin_user_id=$1`, roots.AdminID},
			{`DELETE FROM admin_sessions WHERE admin_user_id=$1`, roots.AdminID},
			{`DELETE FROM material_spools WHERE id=$1`, roots.SpoolID},
			{`DELETE FROM printers WHERE id=$1`, roots.PrinterID},
			{`DELETE FROM admin_users WHERE id=$1`, roots.AdminID},
			{`DELETE FROM users WHERE id=$1`, roots.UserID},
			{`DELETE FROM product_variants WHERE id=$1`, roots.VariantID},
			{`DELETE FROM products WHERE id=$1`, roots.ProductID},
			{`DELETE FROM categories WHERE id=$1`, roots.CategoryID},
			{`DELETE FROM materials WHERE id=$1`, roots.MaterialID},
			{`DELETE FROM colors WHERE id=$1`, roots.ColorID},
		}
		for _, query := range queries {
			if query.arg == "" {
				continue
			}
			if _, err = conn.Exec(ctx, query.sql, query.arg); err != nil {
				t.Errorf("clean exact production fixture: %v", err)
				return
			}
		}
	})
}
