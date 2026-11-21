package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestNewPoolIntegration(t *testing.T) {
	databaseURL, ok := os.LookupEnv(testpostgres.EnvDatabaseURL)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not configured for the dedicated integration database")
	}
	if _, err := testpostgres.ConfigFromEnv(); err != nil {
		t.Fatalf("invalid dedicated integration database configuration: %v", err)
	}
	config := Config{DatabaseURL: databaseURL, MaxConns: 3, MinConns: 1, ConnectTimeout: 3 * time.Second}

	ctx := context.Background()
	pool, err := NewPool(ctx, config)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	if err := NewHealth(pool, time.Second).Check(context.Background()); err != nil {
		t.Fatalf("check PostgreSQL readiness: %v", err)
	}
	if pool.Config().MaxConns != 3 || pool.Config().MinConns != 1 {
		t.Fatalf("pool limits were not applied: max=%d min=%d", pool.Config().MaxConns, pool.Config().MinConns)
	}
	if pool.Config().HealthCheckPeriod <= 0 {
		t.Fatalf("pool health check period must be positive, got %s", pool.Config().HealthCheckPeriod)
	}
	var database, user string
	var serverVersion int
	if err = pool.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::int`).Scan(&database, &user, &serverVersion); err != nil {
		t.Fatalf("verify integration database identity: %v", err)
	}
	if err = testpostgres.ValidateIdentity(database, user, serverVersion); err != nil {
		t.Fatalf("connected PostgreSQL identity rejected: %v", err)
	}
}
