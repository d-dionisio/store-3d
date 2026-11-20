package postgres

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ValidateHomologationDatabaseURL accepts only the isolated local test database.
// It parses the URL with pgx so URL query overrides are checked as the pool sees them.
func ValidateHomologationDatabaseURL(databaseURL string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DATABASE_URL is required for homologation")
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil || (parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql") {
		return errors.New("homologation DATABASE_URL must be a PostgreSQL URL")
	}
	if parsedURL.User == nil || parsedURL.User.Username() == "" || parsedURL.Path != "/loja_3d_test" {
		return errors.New("homologation DATABASE_URL must identify the test database and user")
	}

	parsedConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		// pgx parse errors may include connection input; do not propagate it to logs.
		return errors.New("homologation DATABASE_URL is invalid")
	}
	return ValidateHomologationPoolConfig(&pgxpool.Config{ConnConfig: parsedConfig})
}

// ValidateHomologationPoolConfig checks the already parsed target that will be
// handed to pgxpool, preventing attestation and pool configuration from drifting.
func ValidateHomologationPoolConfig(config *pgxpool.Config) error {
	if config == nil || config.ConnConfig == nil {
		return errors.New("homologation database configuration is invalid")
	}
	conn := config.ConnConfig
	if conn.Database != "loja_3d_test" || conn.User != "formo3d_test" {
		return errors.New("homologation DATABASE_URL must use the isolated test database and user")
	}
	if !isLoopbackHost(conn.Host) || conn.Port != 5433 {
		return errors.New("homologation DATABASE_URL must target loopback on port 5433")
	}
	for _, fallback := range conn.Fallbacks {
		if !isLoopbackHost(fallback.Host) || fallback.Port != 5433 {
			return errors.New("homologation DATABASE_URL contains a non-loopback fallback")
		}
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
