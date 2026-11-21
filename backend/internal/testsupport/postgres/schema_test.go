package postgres

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSchemaNameValidation(t *testing.T) {
	t.Parallel()

	valid := []string{
		"it_mail_0123456789abcdef0123456789abcdef",
		"it_costs_concurrency_0123456789abcdef0123456789abcdef",
	}
	for _, name := range valid {
		if !validSchemaName(name) {
			t.Errorf("generated schema name rejected")
		}
	}
	invalid := []string{
		"public",
		"it_shared_test",
		"it_mail_0123456789abcdef0123456789abcdeg",
		"it_Mail_0123456789abcdef0123456789abcdef",
		"it_mail;drop_schema_0123456789abcdef0123456789abcdef",
		"it_mail_0123456789abcdef0123456789abcdef_extra",
	}
	for _, name := range invalid {
		if validSchemaName(name) {
			t.Errorf("invalid schema name accepted")
		}
	}
	for _, label := range []string{"mail", "costs_concurrency", "a1"} {
		if !validSchemaLabel(label) {
			t.Errorf("valid schema label rejected")
		}
	}
	for _, label := range []string{"", "Public", "mail;drop", "1mail", strings.Repeat("a", 21)} {
		if validSchemaLabel(label) {
			t.Errorf("invalid schema label accepted")
		}
	}
}

func TestSanitizedPostgresStageError(t *testing.T) {
	t.Parallel()

	got := sanitizedPostgresStageError("identity_query", &pgconn.PgError{
		Code:    "42501",
		Message: "sensitive server detail must not be returned",
	})
	if got.Error() != "integration PostgreSQL identity_query failed (SQLSTATE 42501)" {
		t.Fatalf("unexpected sanitized diagnostic: %v", got)
	}
	if strings.Contains(got.Error(), "sensitive") {
		t.Fatal("sanitized PostgreSQL diagnostic leaked the raw server message")
	}
	withoutState := sanitizedPostgresStageError("connection_acquisition", context.DeadlineExceeded)
	if strings.Contains(withoutState.Error(), context.DeadlineExceeded.Error()) || !strings.Contains(withoutState.Error(), "SQLSTATE unavailable") {
		t.Fatal("non-PostgreSQL error was not sanitized")
	}
}

func TestMigrationURLRestrictsSearchPathToOwnedSchema(t *testing.T) {
	t.Parallel()

	const schema = "it_support_0123456789abcdef0123456789abcdef"
	base := "postgres://formo3d_integration_test:secret@127.0.0.1:5433/loja_3d_integration_test?sslmode=disable&search_path=public"
	got, err := migrationURL(base, schema)
	if err != nil {
		t.Fatalf("build isolated migration URL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal("migration URL did not parse")
	}
	if u.Host != "127.0.0.1:5433" || u.Path != "/loja_3d_integration_test" || u.User.Username() != ExpectedUser {
		t.Fatal("migration URL changed the validated integration destination")
	}
	query := u.Query()
	if query.Get("search_path") != schema || query.Get("x-migrations-table") != "schema_migrations" {
		t.Fatal("migration URL is not scoped to the owned schema")
	}
	if strings.Contains(strings.ToLower(query.Get("search_path")), "public") {
		t.Fatal("migration URL contains a public fallback")
	}
}

func TestOfficialMigrationsAreComplete(t *testing.T) {
	t.Parallel()

	directory, err := officialMigrationsDirectory()
	if err != nil {
		t.Fatalf("locate official migration set: %v", err)
	}
	latest, err := latestOfficialMigration(directory)
	if err != nil || latest != 28 {
		t.Fatalf("official migration set is incomplete or unexpected")
	}
	driver, err := source.Open(fileSourceURL(directory))
	if err != nil {
		t.Fatal("official migration source URL could not be opened")
	}
	if err = driver.Close(); err != nil {
		t.Fatal("official migration source could not be closed")
	}
}

func TestIsolatedSchemaPostgreSQL(t *testing.T) {
	if strings.TrimSpace(os.Getenv(EnvDatabaseURL)) == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	schema, err := CreateSchema(ctx, t, "support")
	if err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	if err := schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated schema: %v", err)
	}
	first, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatal("open first schema-scoped pool")
	}
	second, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatal("open second schema-scoped pool")
	}
	defer first.Close()
	defer second.Close()

	var firstSchema, firstPath string
	var firstPID int
	if err = first.QueryRow(ctx, `SELECT current_schema(), current_setting('search_path'), pg_backend_pid()`).Scan(&firstSchema, &firstPath, &firstPID); err != nil {
		t.Fatal("verify first isolated connection")
	}
	var secondSchema, secondPath string
	var secondPID int
	if err = second.QueryRow(ctx, `SELECT current_schema(), current_setting('search_path'), pg_backend_pid()`).Scan(&secondSchema, &secondPath, &secondPID); err != nil {
		t.Fatal("verify second isolated connection")
	}
	if firstSchema != schema.Name() || secondSchema != schema.Name() || firstPath != schema.Name() || secondPath != schema.Name() || firstPID == secondPID {
		t.Fatal("independent connections did not remain in the exclusive test schema")
	}
	var tableSchema string
	if err = first.QueryRow(ctx, `SELECT table_schema FROM information_schema.tables WHERE table_schema=$1 AND table_name='schema_migrations'`, schema.Name()).Scan(&tableSchema); err != nil || tableSchema != schema.Name() {
		t.Fatal("migration state was not created in the isolated schema")
	}
	var publicFallback bool
	if err = first.QueryRow(ctx, `SELECT 'public' = ANY(current_schemas(false))`).Scan(&publicFallback); err != nil || publicFallback {
		t.Fatal("connection search_path includes public")
	}

	first.Close()
	second.Close()
	if err = schema.Cleanup(); err != nil {
		t.Fatal("clean up isolated schema")
	}
	verify, err := Open(ctx)
	if err != nil {
		t.Fatal("open integration database to verify cleanup")
	}
	defer verify.Close()
	var exists bool
	if err = verify.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname=$1)`, schema.Name()).Scan(&exists); err != nil || exists {
		t.Fatal("isolated schema remains after cleanup")
	}
}

func TestMigrationFailureRemovesOwnedSchemaPostgreSQL(t *testing.T) {
	if strings.TrimSpace(os.Getenv(EnvDatabaseURL)) == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	schema, err := CreateSchema(ctx, t, "failed_setup")
	if err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatal("open schema-scoped pool")
	}
	if _, err = pool.Exec(ctx, `CREATE TABLE schema_migrations (invalid_column integer)`); err != nil {
		pool.Close()
		t.Fatal("prepare isolated migration failure")
	}
	pool.Close()
	if err = schema.ApplyOfficialMigrations(ctx); err == nil {
		t.Fatal("invalid migration state unexpectedly accepted")
	}
	verify, err := Open(ctx)
	if err != nil {
		t.Fatal("open integration database to verify failed setup cleanup")
	}
	defer verify.Close()
	var exists bool
	if err = verify.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname=$1)`, schema.Name()).Scan(&exists); err != nil || exists {
		t.Fatal("schema from failed setup was not removed")
	}
}

func TestOpenSchemaHasNoPublicFallbackPostgreSQL(t *testing.T) {
	if strings.TrimSpace(os.Getenv(EnvDatabaseURL)) == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema, err := CreateSchema(ctx, t, "empty_path")
	if err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatal("open empty schema")
	}
	defer pool.Close()
	var resolved bool
	if err = pool.QueryRow(ctx, `SELECT to_regclass('users') IS NOT NULL`).Scan(&resolved); err != nil || resolved {
		t.Fatal("object lookup fell through to public")
	}
}
