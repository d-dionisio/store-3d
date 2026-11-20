package main

import (
	"strings"
	"testing"
	"time"

	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func TestPrepareDatabaseHomologationAttestsThePoolTarget(t *testing.T) {
	t.Parallel()

	config := postgres.Config{
		DatabaseURL:       "postgres://formo3d_test:never-log-this@localhost:5433/loja_3d_test?sslmode=disable",
		MaxConns:          4,
		MinConns:          1,
		ConnectTimeout:    time.Second,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}

	poolConfig, target, result, err := prepareDatabase("homologation", config)
	if err != nil {
		t.Fatalf("prepare valid homologation database: %v", err)
	}
	if result != "approved" {
		t.Fatalf("guard result = %q, want approved", result)
	}
	if got := postgres.TargetFromPoolConfig(poolConfig); got != target {
		t.Fatalf("attested target %+v differs from pool target %+v", target, got)
	}
	if !strings.HasPrefix(target.Host, "localhost") || target.Port != 5433 || target.Database != "loja_3d_test" || target.User != "formo3d_test" {
		t.Fatalf("unexpected sanitized target: %+v", target)
	}
}

func TestPrepareDatabaseRejectsInvalidHomologationTargetsBeforePoolCreation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
	}{
		{name: "port 5432", dsn: "postgres://formo3d_test:secret@localhost:5432/loja_3d_test"},
		{name: "commercial database", dsn: "postgres://formo3d_test:secret@localhost:5433/loja_3d"},
		{name: "wrong user", dsn: "postgres://other:secret@localhost:5433/loja_3d_test"},
		{name: "external host", dsn: "postgres://formo3d_test:secret@example.invalid:5433/loja_3d_test"},
		{name: "multiple hosts", dsn: "postgres://formo3d_test:secret@localhost,example.invalid:5433/loja_3d_test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config := postgres.Config{
				DatabaseURL:       tt.dsn,
				MaxConns:          4,
				MinConns:          1,
				ConnectTimeout:    time.Second,
				MaxConnLifetime:   30 * time.Minute,
				MaxConnIdleTime:   5 * time.Minute,
				HealthCheckPeriod: time.Minute,
			}
			poolConfig, _, result, err := prepareDatabase("homologation", config)
			if err == nil || result != "rejected" || poolConfig != nil {
				t.Fatalf("expected pre-pool rejection, got poolConfig=%v result=%q err=%v", poolConfig != nil, result, err)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("guard error leaked a credential")
			}
		})
	}
}

func TestPrepareDatabaseRejectsMissingOrUnparseableHomologationURL(t *testing.T) {
	t.Parallel()

	for _, dsn := range []string{"", "not a postgres URL"} {
		dsn := dsn
		t.Run(dsn, func(t *testing.T) {
			t.Parallel()
			config := postgres.Config{DatabaseURL: dsn}
			poolConfig, target, result, err := prepareDatabase("homologation", config)
			if err == nil || result != "rejected" || poolConfig != nil {
				t.Fatalf("expected rejection before pool creation, got poolConfig=%v target=%+v result=%q err=%v", poolConfig != nil, target, result, err)
			}
			if strings.Contains(err.Error(), dsn) && dsn != "" {
				t.Fatal("parse error leaked the supplied connection string")
			}
		})
	}
}

func TestPrepareDatabaseLeavesOtherEnvironmentsUnrestrictedByHomologationGuard(t *testing.T) {
	t.Parallel()

	config := postgres.Config{
		DatabaseURL:    "postgres://user:password@db.internal:5432/store",
		MaxConns:       4,
		MinConns:       1,
		ConnectTimeout: time.Second,
	}
	poolConfig, target, result, err := prepareDatabase("production", config)
	if err != nil {
		t.Fatalf("production config should retain existing parsing behavior: %v", err)
	}
	if result != "not_applicable" || poolConfig == nil || target.Database != "store" {
		t.Fatalf("unexpected non-homologation result: target=%+v result=%q", target, result)
	}
}
