package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckPeriod
	poolConfig.ConnConfig.ConnectTimeout = config.ConnectTimeout
	return poolConfig, nil
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

	connectContext, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectContext, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	if err := pool.Ping(connectContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
