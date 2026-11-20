package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := loadConfig(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing DATABASE_URL error, got %v", err)
	}
}

func TestValidateHomologationPoolConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "valid target", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d_test", want: true},
		{name: "commercial port", dsn: "postgres://formo3d_test:secret@localhost:5432/loja_3d_test"},
		{name: "commercial database", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d"},
		{name: "wrong user", dsn: "postgres://other:secret@localhost:5433/loja_3d_test"},
		{name: "external host", dsn: "postgres://formo3d_test:secret@example.invalid:5433/loja_3d_test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config, err := pgxpool.ParseConfig(tt.dsn)
			if err != nil {
				t.Fatalf("parse test config: %v", err)
			}
			err = ValidateHomologationPoolConfig(config)
			if tt.want && err != nil {
				t.Fatalf("expected valid target: %v", err)
			}
			if !tt.want && err == nil {
				t.Fatal("expected invalid target to be rejected")
			}
		})
	}

	if err := ValidateHomologationPoolConfig(nil); err == nil {
		t.Fatal("expected nil pool config to be rejected")
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "max connections", key: "DB_MAX_CONNS", value: "zero"},
		{name: "min connections", key: "DB_MIN_CONNS", value: "-1"},
		{name: "connect timeout", key: "DB_CONNECT_TIMEOUT", value: "soon"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := loadConfig(func(key string) string {
				if key == "DATABASE_URL" {
					return "postgres://user:password@localhost/database"
				}
				if key == test.key {
					return test.value
				}
				return ""
			})
			if err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestLoadConfigRejectsMinGreaterThanMax(t *testing.T) {
	t.Parallel()

	_, err := loadConfig(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://user:password@localhost/database"
		case "DB_MAX_CONNS":
			return "2"
		case "DB_MIN_CONNS":
			return "3"
		default:
			return ""
		}
	})
	if err == nil {
		t.Fatal("expected min/max validation error")
	}
}

func TestLoadConfigUsesConfiguredValues(t *testing.T) {
	t.Parallel()

	config, err := loadConfig(func(key string) string {
		values := map[string]string{
			"DATABASE_URL":       "postgres://user:password@localhost/database",
			"DB_MAX_CONNS":       "12",
			"DB_MIN_CONNS":       "3",
			"DB_CONNECT_TIMEOUT": "750ms",
		}
		return values[key]
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.MaxConns != 12 || config.MinConns != 3 || config.ConnectTimeout != 750*time.Millisecond {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestValidateHomologationDatabaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "valid local test database", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d_test?sslmode=disable", want: true},
		{name: "valid IPv4 loopback", dsn: "postgres://formo3d_test:secret@127.0.0.1:5433/loja_3d_test", want: true},
		{name: "valid IPv6 loopback", dsn: "postgres://formo3d_test:secret@[::1]:5433/loja_3d_test", want: true},
		{name: "missing URL", dsn: "", want: false},
		{name: "commercial port", dsn: "postgres://formo3d_test:secret@localhost:5432/loja_3d_test", want: false},
		{name: "commercial database", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d", want: false},
		{name: "wrong user", dsn: "postgres://other:secret@localhost:5433/loja_3d_test", want: false},
		{name: "external host", dsn: "postgres://formo3d_test:secret@db.example.com:5433/loja_3d_test", want: false},
		{name: "multiple hosts", dsn: "postgres://formo3d_test:secret@localhost,db.example.com:5433/loja_3d_test", want: false},
		{name: "query host override", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d_test?host=db.example.com", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateHomologationDatabaseURL(test.dsn)
			if test.want && err != nil {
				t.Fatalf("expected valid test database URL, got %v", err)
			}
			if !test.want && err == nil {
				t.Fatal("expected test database URL to be rejected")
			}
		})
	}
}
