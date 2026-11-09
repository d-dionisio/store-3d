package postgres

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultMaxConns       = 10
	defaultMinConns       = 2
	defaultConnectTimeout = 5 * time.Second
)

// Config contains the database pool settings used by the API.
type Config struct {
	DatabaseURL       string
	MaxConns          int32
	MinConns          int32
	ConnectTimeout    time.Duration
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// LoadConfig reads and validates PostgreSQL settings from the environment.
func LoadConfig() (Config, error) {
	return loadConfig(os.Getenv)
}

func loadConfig(getenv func(string) string) (Config, error) {
	databaseURL := getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	maxConns, err := parseInt32(getenv("DB_MAX_CONNS"), defaultMaxConns, "DB_MAX_CONNS")
	if err != nil {
		return Config{}, err
	}
	minConns, err := parseInt32(getenv("DB_MIN_CONNS"), defaultMinConns, "DB_MIN_CONNS")
	if err != nil {
		return Config{}, err
	}
	if minConns > maxConns {
		return Config{}, errors.New("DB_MIN_CONNS cannot be greater than DB_MAX_CONNS")
	}

	connectTimeout, err := parseDuration(getenv("DB_CONNECT_TIMEOUT"), defaultConnectTimeout, "DB_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	return Config{
		DatabaseURL:       databaseURL,
		MaxConns:          maxConns,
		MinConns:          minConns,
		ConnectTimeout:    connectTimeout,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}, nil
}

func parseInt32(raw string, fallback int32, name string) (int32, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int32(value), nil
}

func parseDuration(raw string, fallback time.Duration, name string) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}
