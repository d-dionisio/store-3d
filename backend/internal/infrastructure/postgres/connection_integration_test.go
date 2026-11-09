package postgres

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewPoolIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	config := testConfig(databaseURL)
	config.MaxConns = 3
	config.MinConns = 1
	config.ConnectTimeout = 3 * time.Second

	pool, err := NewPool(context.Background(), config)
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
}
