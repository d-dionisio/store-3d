package postgres

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestParseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{"valid integration target", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=disable", true},
		{"TLS prefer fallback remains on integration target", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=prefer", true},
		{"TLS allow fallback remains on integration target", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=allow", true},
		{"TLS require has no alternate destination", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=require", true},
		{"TLS verify-ca has no alternate destination", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=verify-ca", true},
		{"TLS verify-full has no alternate destination", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=verify-full", true},
		{"wrong database", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_test", false},
		{"commercial database", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d", false},
		{"wrong host", "postgres://formo3d_integration_test:secret@localhost:5433/loja_3d_integration_test", false},
		{"external host", "postgres://formo3d_integration_test:secret@example.invalid:5433/loja_3d_integration_test", false},
		{"wrong port", "postgres://formo3d_integration_test:secret@127.0.0.1:5432/loja_3d_integration_test", false},
		{"wrong user", "postgres://formo3d_test:secret@127.0.0.1:5433/loja_3d_integration_test", false},
		{"host override", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?host=example.invalid", false},
		{"port override", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?port=5432", false},
		{"session role override", "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?options=-c%20role%3Dother", false},
		{"multi-host", "postgres://formo3d_integration_test:secret@127.0.0.1,example.invalid:5433/loja_3d_integration_test", false},
		{"missing URL", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseURL(tt.dsn)
			if tt.want && err != nil {
				t.Fatalf("valid target rejected: %v", err)
			}
			if !tt.want && err == nil {
				t.Fatal("invalid target was accepted")
			}
		})
	}
}

func TestValidateEffectiveDestinationFallbacks(t *testing.T) {
	t.Parallel()

	base, err := pgxpool.ParseConfig("postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=disable")
	if err != nil {
		t.Fatalf("parse base test config: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*pgx.ConnConfig)
		nilCfg  bool
		wantErr bool
	}{
		{name: "no fallback"},
		{name: "TLS fallback same target", mutate: func(c *pgx.ConnConfig) {
			c.Fallbacks = []*pgconn.FallbackConfig{{Host: ExpectedHost, Port: ExpectedPort}}
		}},
		{name: "fallback external host", mutate: func(c *pgx.ConnConfig) {
			c.Fallbacks = []*pgconn.FallbackConfig{{Host: "example.invalid", Port: ExpectedPort}}
		}, wantErr: true},
		{name: "fallback alternate port", mutate: func(c *pgx.ConnConfig) {
			c.Fallbacks = []*pgconn.FallbackConfig{{Host: ExpectedHost, Port: 5432}}
		}, wantErr: true},
		{name: "nil fallback", mutate: func(c *pgx.ConnConfig) {
			c.Fallbacks = []*pgconn.FallbackConfig{nil}
		}, wantErr: true},
		{name: "wrong database", mutate: func(c *pgx.ConnConfig) { c.Database = "loja_3d_test" }, wantErr: true},
		{name: "wrong user", mutate: func(c *pgx.ConnConfig) { c.User = "formo3d_test" }, wantErr: true},
		{name: "wrong primary host", mutate: func(c *pgx.ConnConfig) { c.Host = "localhost" }, wantErr: true},
		{name: "wrong primary port", mutate: func(c *pgx.ConnConfig) { c.Port = 5432 }, wantErr: true},
		{name: "nil config", nilCfg: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var config *pgx.ConnConfig
			if !tt.nilCfg {
				config = base.ConnConfig.Copy()
			}
			if config != nil && tt.mutate != nil {
				tt.mutate(config)
			}
			err := validateEffectiveDestination(config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEffectiveDestination() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestConfigFromLookupRequiresTestURLAndNeverFallsBack(t *testing.T) {
	t.Parallel()

	requested := make([]string, 0, 1)
	_, err := ConfigFromLookup(func(key string) (string, bool) {
		requested = append(requested, key)
		if key == "DATABASE_URL" {
			return "postgres://commercial:secret@127.0.0.1:5432/loja_3d", true
		}
		return "", false
	})
	if err == nil || !strings.Contains(err.Error(), "TEST_DATABASE_URL") {
		t.Fatalf("expected missing TEST_DATABASE_URL error, got %v", err)
	}
	if len(requested) != 1 || requested[0] != EnvDatabaseURL {
		t.Fatalf("unexpected environment lookup keys: %v", requested)
	}
}

func TestConfigFromEnvWithSearchPath(t *testing.T) {
	tests := []struct {
		name      string
		search    string
		wantValid bool
	}{
		{"generated schema", "it_mail_test_0123456789abcdef0123456789abcdef", true},
		{"SQL punctuation", "mail;DROP_SCHEMA", false},
		{"leading digit", "8mail_test", false},
		{"shared public schema", "public", false},
		{"missing random suffix", "it_shared_test", false},
		{"uppercase identifier", "it_Mail_0123456789abcdef0123456789abcdef", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				if key == EnvDatabaseURL {
					return "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=disable", true
				}
				return "", false
			}
			config, err := ConfigFromLookupWithSearchPath(lookup, tt.search)
			if tt.wantValid {
				if err != nil {
					t.Fatalf("expected valid schema-scoped config: %v", err)
				}
				if config.ConnConfig.RuntimeParams["search_path"] != tt.search {
					t.Fatal("search_path not set")
				}
				return
			}
			if err == nil {
				t.Fatal("invalid schema-scoped config was accepted")
			}
		})
	}
}

func TestValidateIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		database string
		user     string
		version  int
		wantErr  bool
	}{
		{"valid PostgreSQL 17.6", ExpectedDB, ExpectedUser, 170006, false},
		{"wrong database", "loja_3d_test", ExpectedUser, 170006, true},
		{"wrong user", ExpectedDB, "formo3d_test", 170006, true},
		{"PostgreSQL 16", ExpectedDB, ExpectedUser, 160009, true},
		{"PostgreSQL 18", ExpectedDB, ExpectedUser, 180000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateIdentity(tt.database, tt.user, tt.version)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateIdentity() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestNewFixtureIDIsUniqueUUID(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id, err := NewFixtureID()
		if err != nil {
			t.Fatalf("generate fixture ID: %v", err)
		}
		if len(id) != 36 || id[14] != '4' || !strings.Contains(id, "-") {
			t.Fatalf("generated ID is not a UUIDv4: %q", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate fixture ID at iteration %d", i)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != 1000 {
		t.Fatalf("generated %d unique IDs, expected 1000", len(seen))
	}
}
