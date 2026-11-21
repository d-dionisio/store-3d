package main

import (
	"net/url"
	"testing"

	"github.com/golang-migrate/migrate/v4/database"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func TestMigrateDriversRegisteredWithoutDatabaseConnection(t *testing.T) {
	if !contains(database.List(), "postgres") {
		t.Fatal("golang-migrate PostgreSQL driver is not registered")
	}
	if !contains(source.List(), "file") {
		t.Fatal("golang-migrate file source driver is not registered")
	}
}

func TestMigrationDriverSSLModeCompatibility(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{"disable", false},
		{"require", false},
		{"verify-ca", false},
		{"verify-full", false},
		{"prefer", true},
		{"allow", true},
		{"invalid", true},
		{"DISABLE", true},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			dsn := "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=" + url.QueryEscape(tt.mode)
			err := validateMigrationDriverSSLMode(dsn)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateMigrationDriverSSLMode() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestOfficialMigrationSourceExistsWithoutDatabaseConnection(t *testing.T) {
	// go test runs with this package directory as the working directory.
	// The command's documented working directory is the backend module root.
	driver, err := source.Open("file://../../migrations")
	if err != nil {
		t.Fatalf("open official migration source: %v", err)
	}
	defer driver.Close()
	first, err := driver.First()
	if err != nil || first != 1 {
		t.Fatalf("official migration source first version = %d, err = %v", first, err)
	}
	next, err := driver.Next(first)
	if err != nil || next <= first {
		t.Fatalf("official migration source next version = %d, err = %v", next, err)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
