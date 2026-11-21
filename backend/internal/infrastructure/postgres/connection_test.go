package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestBuildPoolConfigDefaultsHealthCheckPeriod(t *testing.T) {
	t.Parallel()

	config := testConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	config.HealthCheckPeriod = 0

	poolConfig, err := BuildPoolConfig(config)
	if err != nil {
		t.Fatalf("build pool config: %v", err)
	}
	if poolConfig.HealthCheckPeriod != defaultHealthCheckPeriod || poolConfig.HealthCheckPeriod <= 0 {
		t.Fatalf("expected positive default health check period %s, got %s", defaultHealthCheckPeriod, poolConfig.HealthCheckPeriod)
	}
}

func TestBuildPoolConfigRejectsNegativeHealthCheckPeriod(t *testing.T) {
	t.Parallel()

	config := testConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	config.HealthCheckPeriod = -time.Second

	if _, err := BuildPoolConfig(config); err == nil {
		t.Fatal("expected negative health check period to be rejected")
	}
}

func TestNewPoolWithConfigDefaultsHealthCheckPeriod(t *testing.T) {
	t.Parallel()

	poolConfig, err := pgxpool.ParseConfig("postgres://user:password@127.0.0.1:1/database?sslmode=disable")
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	poolConfig.HealthCheckPeriod = 0
	config := Config{ConnectTimeout: 100 * time.Millisecond}

	pool, err := NewPoolWithConfig(context.Background(), config, poolConfig)
	if err == nil {
		pool.Close()
		t.Fatal("expected connection failure for unreachable test port")
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
