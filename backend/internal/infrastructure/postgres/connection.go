package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates, configures, and verifies a PostgreSQL connection pool.
func NewPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	poolConfig, err := BuildPoolConfig(config)
	if err != nil {
		return nil, err
	}

	return NewPoolWithConfig(ctx, config, poolConfig)
}

// BuildPoolConfig parses and applies the exact configuration passed to pgxpool.
func BuildPoolConfig(config Config) (*pgxpool.Config, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	healthCheckPeriod, err := effectiveHealthCheckPeriod(config.HealthCheckPeriod)
	if err != nil {
		return nil, err
	}

	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = healthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout
	return poolConfig, nil
}

func effectiveHealthCheckPeriod(period time.Duration) (time.Duration, error) {
	if period < 0 {
		return 0, errors.New("database health check period must not be negative")
	}
	if period == 0 {
		return defaultHealthCheckPeriod, nil
	}
	return period, nil
}

// DatabaseTarget contains only non-secret connection identity fields.
type DatabaseTarget struct {
	Host     string
	Port     uint16
	Database string
	User     string
}

// TargetFromPoolConfig reports the identity in the pgx config that will be given to pgxpool.
func TargetFromPoolConfig(config *pgxpool.Config) DatabaseTarget {
	if config == nil || config.ConnConfig == nil {
		return DatabaseTarget{}
	}
	hosts := []string{config.ConnConfig.Host}
	for _, fallback := range config.ConnConfig.Fallbacks {
		hosts = append(hosts, fallback.Host)
	}
	return DatabaseTarget{
		Host:     strings.Join(hosts, ","),
		Port:     config.ConnConfig.Port,
		Database: config.ConnConfig.Database,
		User:     config.ConnConfig.User,
	}
}

// NewPoolWithConfig creates, configures, and verifies a PostgreSQL connection pool
// using the already parsed config, allowing callers to attest the exact target.
func NewPoolWithConfig(ctx context.Context, config Config, poolConfig *pgxpool.Config) (*pgxpool.Pool, error) {
	if poolConfig == nil {
		return nil, errors.New("database pool configuration is required")
	}
	healthCheckPeriod, err := effectiveHealthCheckPeriod(poolConfig.HealthCheckPeriod)
	if err != nil {
		return nil, err
	}
	poolConfigCopy := *poolConfig
	poolConfigCopy.HealthCheckPeriod = healthCheckPeriod

	connectContext, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectContext, &poolConfigCopy)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := pool.Ping(connectContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
