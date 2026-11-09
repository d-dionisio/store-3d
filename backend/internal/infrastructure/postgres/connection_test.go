package postgres

import (
	"context"
	"testing"
	"time"
)

func TestNewPoolRejectsInvalidDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := NewPool(context.Background(), testConfig("://invalid"))
	if err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestNewPoolReturnsConnectionFailure(t *testing.T) {
	t.Parallel()

	config := testConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	config.ConnectTimeout = 100 * time.Millisecond
	pool, err := NewPool(context.Background(), config)
	if err == nil {
		pool.Close()
		t.Fatal("expected connection error")
	}
}

func testConfig(databaseURL string) Config {
	return Config{
		DatabaseURL:       databaseURL,
		MaxConns:          4,
		MinConns:          1,
		ConnectTimeout:    time.Second,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}
}
