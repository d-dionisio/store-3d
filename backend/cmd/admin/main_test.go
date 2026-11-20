package main

import (
	"testing"

	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func TestValidateBootstrapDatabaseTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appEnv  string
		dsn     string
		wantErr bool
	}{
		{name: "homologation valid isolated target", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d_test?sslmode=disable"},
		{name: "homologation rejects commercial port", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@localhost:5432/loja_3d_test", wantErr: true},
		{name: "homologation rejects commercial database", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d", wantErr: true},
		{name: "homologation rejects wrong user", appEnv: "homologation", dsn: "postgres://other:secret@localhost:5433/loja_3d_test", wantErr: true},
		{name: "homologation rejects external host", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@db.example.test:5433/loja_3d_test", wantErr: true},
		{name: "homologation rejects missing URL", appEnv: "homologation", dsn: "", wantErr: true},
		{name: "homologation rejects multiple hosts", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@localhost,db.example.test:5433/loja_3d_test", wantErr: true},
		{name: "homologation rejects query host override", appEnv: "homologation", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d_test?host=db.example.test", wantErr: true},
		{name: "development behavior unchanged", appEnv: "development", dsn: "postgres://user:secret@db.example.test:5432/store"},
		{name: "production behavior unchanged", appEnv: "production", dsn: "postgres://user:secret@db.example.test:5432/store"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBootstrapDatabaseTarget(tt.appEnv, postgres.Config{DatabaseURL: tt.dsn})
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBootstrapDatabaseTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequireBootstrapEnabled(t *testing.T) {
	t.Parallel()

	if err := requireBootstrapEnabled("true"); err != nil {
		t.Fatalf("enabled bootstrap rejected: %v", err)
	}
	for _, value := range []string{"", "false", "TRUE", "1"} {
		if err := requireBootstrapEnabled(value); err == nil {
			t.Errorf("bootstrap flag %q unexpectedly accepted", value)
		}
	}
}
