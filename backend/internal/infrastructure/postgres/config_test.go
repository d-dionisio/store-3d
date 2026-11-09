package postgres

import (
	"strings"
	"testing"
	"time"
)

func TestLoadConfigRequiresDatabaseURL(t *testing.T) {
	t.Parallel()

	_, err := loadConfig(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing DATABASE_URL error, got %v", err)
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
