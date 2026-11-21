// Command integration-db inspects or initializes only the dedicated
// PostgreSQL integration database. It never reads DATABASE_URL.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "status" && os.Args[1] != "migrate") {
		fail("usage: go run ./cmd/integration-db <status|migrate>")
	}
	dsn, ok := os.LookupEnv(postgres.EnvDatabaseURL)
	if !ok || dsn == "" {
		fail("TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := postgres.OpenURL(ctx, dsn)
	if err != nil {
		fail(err.Error())
	}
	defer pool.Close()

	state, err := readMigrationState(ctx, pool)
	if err != nil {
		fail("could not read integration migration state")
	}
	if os.Args[1] == "status" {
		printState(state)
		return
	}
	if state.present {
		fail("refusing migration: integration database already has a migration state; inspect it and use an explicit recovery plan")
	}
	var relationCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname <> 'information_schema'
		  AND n.nspname !~ '^pg_'
		  AND c.relkind IN ('r', 'p', 'v', 'm', 'S', 'f')`).Scan(&relationCount)
	if err != nil || relationCount != 0 {
		fail("refusing migration: integration database is not an empty schema")
	}
	if err := validateMigrationDriverSSLMode(dsn); err != nil {
		fail(err.Error())
	}

	runner, err := migrate.New("file://migrations", dsn)
	if err != nil {
		fail("could not initialize official integration migrations")
	}
	upErr := runner.Up()
	sourceErr, databaseErr := runner.Close()
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		fail("official integration migrations failed")
	}
	if sourceErr != nil || databaseErr != nil {
		fail("could not close integration migration runner cleanly")
	}
	state, err = readMigrationState(ctx, pool)
	if err != nil || !state.present || state.dirty {
		fail("integration migrations did not finish in a clean versioned state")
	}
	fmt.Printf("database=%s user=%s postgres=17.x migration_version=%d dirty=false\n", postgres.ExpectedDB, postgres.ExpectedUser, state.version)
}

// validateMigrationDriverSSLMode checks compatibility with golang-migrate's
// PostgreSQL driver (lib/pq) before its initialization can open a connection.
// pgx accepts prefer/allow with TLS fallbacks, but lib/pq does not.
func validateMigrationDriverSSLMode(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return errors.New("integration migration URL is invalid")
	}
	mode := u.Query().Get("sslmode")
	switch mode {
	case "", "disable", "require", "verify-ca", "verify-full":
		return nil
	case "prefer", "allow":
		return errors.New("integration migration driver does not support this TLS fallback mode; use sslmode=disable for the local test server or require verified TLS")
	default:
		return errors.New("integration migration URL has an unsupported sslmode")
	}
}

type migrationState struct {
	present bool
	version int64
	dirty   bool
}

func readMigrationState(ctx context.Context, pool *pgxpool.Pool) (migrationState, error) {
	var present bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&present); err != nil {
		return migrationState{}, err
	}
	state := migrationState{present: present}
	if !present {
		return state, nil
	}
	if err := pool.QueryRow(ctx, `SELECT version, dirty FROM public.schema_migrations`).Scan(&state.version, &state.dirty); err != nil {
		return migrationState{}, err
	}
	return state, nil
}

func printState(state migrationState) {
	if !state.present {
		fmt.Printf("database=%s user=%s postgres=17.x migration_version=none dirty=false\n", postgres.ExpectedDB, postgres.ExpectedUser)
		return
	}
	fmt.Printf("database=%s user=%s postgres=17.x migration_version=%d dirty=%t\n", postgres.ExpectedDB, postgres.ExpectedUser, state.version, state.dirty)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
