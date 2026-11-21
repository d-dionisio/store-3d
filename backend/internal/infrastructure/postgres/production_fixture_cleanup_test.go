package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newProductionIntegrationSchema creates a fresh schema for each production
// integration test. TestSupport owns cleanup and drops only that schema after
// its connections close; triggers and foreign keys remain enabled throughout.
func newProductionIntegrationSchema(t *testing.T, label string) (*testpostgres.IsolatedSchema, *pgxpool.Pool, context.Context) {
	t.Helper()
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("Production PostgreSQL integration requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, label)
	if err != nil {
		t.Fatalf("create isolated production schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated production schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated production pool: %v", err)
	}
	return schema, pool, ctx
}

func newProductionIntegrationPool(t *testing.T, label string) (*pgxpool.Pool, context.Context) {
	t.Helper()
	_, pool, ctx := newProductionIntegrationSchema(t, label)
	return pool, ctx
}

func transitionProductionJobFixture(ctx context.Context, tx pgx.Tx, jobID, target, adminID string) error {
	paths := map[string][]string{
		"queued":        {"queued"},
		"preparing":     {"queued", "preparing"},
		"printing":      {"queued", "preparing", "printing"},
		"quality_check": {"queued", "preparing", "printing", "quality_check"},
		"completed":     {"queued", "preparing", "printing", "quality_check", "completed"},
		"failed":        {"queued", "failed"},
		"cancelled":     {"cancelled"},
	}
	for _, next := range paths[target] {
		var version int64
		if err := tx.QueryRow(ctx, `SELECT version FROM production_jobs WHERE id=$1`, jobID).Scan(&version); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT transition_production_job($1,$2,$3,'isolated integration fixture transition',$4)`, jobID, next, adminID, version); err != nil {
			return err
		}
	}
	if target != "awaiting_production" && len(paths[target]) == 0 {
		return fmt.Errorf("unsupported production job fixture target %q", target)
	}
	return nil
}
