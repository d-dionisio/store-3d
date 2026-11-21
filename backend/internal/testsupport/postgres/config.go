// Package postgres provides fail-closed helpers for PostgreSQL integration
// tests. It is intentionally separate from the application and homologation
// database configuration.
package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EnvDatabaseURL = "TEST_DATABASE_URL"
	ExpectedHost   = "127.0.0.1"
	ExpectedPort   = uint16(5433)
	ExpectedDB     = "loja_3d_integration_test"
	ExpectedUser   = "formo3d_integration_test"
	PostgresMajor  = 17
)

// LookupEnv is the minimal environment interface used to prove that this
// package never falls back to DATABASE_URL.
type LookupEnv func(string) (string, bool)

// ConfigFromEnv parses TEST_DATABASE_URL only. DATABASE_URL is never read.
func ConfigFromEnv() (*pgxpool.Config, error) {
	return ConfigFromLookup(os.LookupEnv)
}

// ConfigFromEnvWithSearchPath returns an otherwise validated integration pool
// configuration scoped to one simple schema identifier.
func ConfigFromEnvWithSearchPath(searchPath string) (*pgxpool.Config, error) {
	return ConfigFromLookupWithSearchPath(os.LookupEnv, searchPath)
}

// ConfigFromLookupWithSearchPath is the testable environment-backed variant
// of ConfigFromEnvWithSearchPath.
func ConfigFromLookupWithSearchPath(lookup LookupEnv, searchPath string) (*pgxpool.Config, error) {
	if !validSchemaName(searchPath) {
		return nil, errors.New("integration test schema name is invalid")
	}
	config, err := ConfigFromLookup(lookup)
	if err != nil {
		return nil, err
	}
	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = make(map[string]string)
	}
	config.ConnConfig.RuntimeParams["search_path"] = searchPath
	return config, nil
}

func validSchemaName(name string) bool {
	if len(name) < len("it_a_")+32 || len(name) > 63 || !strings.HasPrefix(name, "it_") {
		return false
	}
	suffixSeparator := strings.LastIndexByte(name, '_')
	if suffixSeparator < 4 || len(name)-suffixSeparator-1 != 32 {
		return false
	}
	for i, r := range name[:suffixSeparator] {
		if (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	for _, r := range name[suffixSeparator+1:] {
		if !((r >= 'a' && r <= 'f') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// ConfigFromLookup is useful for unit tests and deliberately requests only
// TEST_DATABASE_URL from the supplied lookup function.
func ConfigFromLookup(lookup LookupEnv) (*pgxpool.Config, error) {
	if lookup == nil {
		return nil, errors.New("TEST_DATABASE_URL environment lookup is unavailable")
	}
	dsn, ok := lookup(EnvDatabaseURL)
	if !ok || strings.TrimSpace(dsn) == "" {
		return nil, errors.New("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	return ParseURL(dsn)
}

// ParseURL parses and validates the effective pgx destination before any
// network connection is attempted. Errors are intentionally sanitized.
func ParseURL(dsn string) (*pgxpool.Config, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("TEST_DATABASE_URL is required")
	}

	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return nil, errors.New("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	if u.Hostname() != ExpectedHost || u.Port() != strconv.Itoa(int(ExpectedPort)) {
		return nil, errors.New("TEST_DATABASE_URL must target the integration loopback host and port")
	}
	if u.User == nil || u.User.Username() != ExpectedUser || u.Path != "/"+ExpectedDB {
		return nil, errors.New("TEST_DATABASE_URL must identify the dedicated integration database and user")
	}
	for key := range u.Query() {
		switch strings.ToLower(key) {
		case "host", "hostaddr", "port", "dbname", "database", "user", "username", "service", "servicefile", "target_session_attrs", "load_balance_hosts", "options":
			return nil, errors.New("TEST_DATABASE_URL contains a prohibited destination override")
		}
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("TEST_DATABASE_URL is invalid")
	}
	conn := config.ConnConfig
	if err := validateEffectiveDestination(conn); err != nil {
		return nil, errors.New("TEST_DATABASE_URL effective PostgreSQL destination is not the integration target")
	}
	for key := range conn.RuntimeParams {
		if strings.EqualFold(key, "role") || strings.EqualFold(key, "session_authorization") {
			return nil, errors.New("TEST_DATABASE_URL contains a prohibited PostgreSQL identity override")
		}
	}
	return config, nil
}

// validateEffectiveDestination ensures every pgx connection attempt, including
// TLS-mode fallbacks, remains on the one explicitly authorized integration
// endpoint. Fallbacks inherit database and user from ConnConfig.
func validateEffectiveDestination(conn *pgx.ConnConfig) error {
	if conn == nil || conn.Host != ExpectedHost || conn.Port != ExpectedPort || conn.Database != ExpectedDB || conn.User != ExpectedUser {
		return errors.New("effective PostgreSQL destination is not the integration target")
	}
	for _, fallback := range conn.Fallbacks {
		if fallback == nil || fallback.Host != ExpectedHost || fallback.Port != ExpectedPort {
			return errors.New("PostgreSQL fallback destination is not the integration target")
		}
	}
	return nil
}

// ValidateIdentity verifies the identity observed from an established
// PostgreSQL connection, independently of the URL that requested it.
func ValidateIdentity(database, user string, serverVersionNum int) error {
	if database != ExpectedDB || user != ExpectedUser {
		return errors.New("connected PostgreSQL identity is not the integration target")
	}
	if serverVersionNum < PostgresMajor*10000 || serverVersionNum >= (PostgresMajor+1)*10000 {
		return errors.New("integration PostgreSQL server must be version 17.x")
	}
	return nil
}

// Open validates the URL before connecting, then verifies current_database(),
// current_user and server_version_num. It does not run migrations or seed data.
func Open(ctx context.Context) (*pgxpool.Pool, error) {
	config, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return openConfig(ctx, config)
}

// OpenSchema opens the dedicated integration database with search_path limited
// to a caller-owned schema. Database identity is still verified after connect.
func OpenSchema(ctx context.Context, schema string) (*pgxpool.Pool, error) {
	return OpenSchemaWithApplicationName(ctx, schema, "")
}

// OpenSchemaWithApplicationName opens an independently pooled connection set
// scoped only to schema. The optional application name is useful for observing
// concurrent test backends in pg_stat_activity; it cannot affect routing.
func OpenSchemaWithApplicationName(ctx context.Context, schema, applicationName string) (*pgxpool.Pool, error) {
	config, err := ConfigFromEnvWithSearchPath(schema)
	if err != nil {
		return nil, err
	}
	if applicationName != "" {
		if !validApplicationName(applicationName) {
			return nil, errors.New("integration PostgreSQL application name is invalid")
		}
		config.ConnConfig.RuntimeParams["application_name"] = applicationName
	}
	pool, err := openConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	var currentSchema, searchPath string
	err = pool.QueryRow(ctx, `SELECT current_schema(), current_setting('search_path')`).Scan(&currentSchema, &searchPath)
	if err != nil || currentSchema != schema || searchPath != schema {
		pool.Close()
		return nil, errors.New("integration PostgreSQL search_path is not exclusively scoped to the test schema")
	}
	return pool, nil
}

func validApplicationName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// IsolatedSchema owns one schema created in the dedicated integration
// database. Its fields are private so callers cannot use cleanup to drop an
// arbitrary schema. All pools opened by it use exactly that schema in
// search_path, with no public fallback.
type IsolatedSchema struct {
	name    string
	control *pgxpool.Pool
	mu      sync.Mutex
	pools   []*pgxpool.Pool
	closed  bool
}

// CreateSchema creates a uniquely named schema in the validated integration
// database and registers exact-schema cleanup on t. label must be a short,
// lowercase identifier fragment; it is never used as SQL without validation.
func CreateSchema(ctx context.Context, t CleanupRegistrar, label string) (*IsolatedSchema, error) {
	if t == nil || !validSchemaLabel(label) {
		return nil, errors.New("test registrar and valid schema label are required")
	}
	control, err := Open(ctx)
	if err != nil {
		return nil, err
	}
	id, err := NewFixtureID()
	if err != nil {
		control.Close()
		return nil, err
	}
	name := "it_" + label + "_" + strings.ReplaceAll(id, "-", "")
	if !validSchemaName(name) {
		control.Close()
		return nil, errors.New("generated integration schema name is invalid")
	}
	if _, err = control.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize()); err != nil {
		control.Close()
		return nil, sanitizedPostgresStageError("schema_creation", err)
	}
	schema := &IsolatedSchema{name: name, control: control}
	t.Cleanup(func() {
		if err := schema.Cleanup(); err != nil {
			t.Errorf("isolated PostgreSQL schema cleanup failed")
		}
	})
	return schema, nil
}

// Name returns the generated schema identifier for diagnostics and scoped SQL.
// It is safe to interpolate only through pgx.Identifier{Name()}.Sanitize().
func (s *IsolatedSchema) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// OpenPool creates another independently pooled connection set scoped only to
// this schema. It is suitable for real PostgreSQL concurrency tests.
func (s *IsolatedSchema) OpenPool(ctx context.Context) (*pgxpool.Pool, error) {
	return s.OpenPoolWithApplicationName(ctx, "")
}

// OpenPoolWithApplicationName creates another pool owned by this schema,
// optionally tagged for PostgreSQL-side observation during concurrency tests.
func (s *IsolatedSchema) OpenPoolWithApplicationName(ctx context.Context, applicationName string) (*pgxpool.Pool, error) {
	if s == nil || !validSchemaName(s.name) {
		return nil, errors.New("isolated integration schema is unavailable")
	}
	pool, err := OpenSchemaWithApplicationName(ctx, s.name, applicationName)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		pool.Close()
		return nil, errors.New("isolated integration schema is already closed")
	}
	s.pools = append(s.pools, pool)
	return pool, nil
}

// ApplyOfficialMigrations applies the repository's complete, paired migration
// set to this schema only. The migrate driver receives search_path=<schema>,
// and its version table is consequently created in the same schema. On any
// failure the owned schema is immediately removed to prevent setup residue.
func (s *IsolatedSchema) ApplyOfficialMigrations(ctx context.Context) error {
	if s == nil || !validSchemaName(s.name) {
		return errors.New("isolated integration schema is unavailable")
	}
	if _, err := ConfigFromEnv(); err != nil {
		return err
	}
	dsn, ok := os.LookupEnv(EnvDatabaseURL)
	if !ok {
		return errors.New("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	directory, err := officialMigrationsDirectory()
	if err == nil {
		var migrationDSN string
		migrationDSN, err = migrationURL(dsn, s.name)
		if err == nil {
			var runner *migrate.Migrate
			runner, err = migrate.New(fileSourceURL(directory), migrationDSN)
			if err == nil {
				upErr := runner.Up()
				sourceErr, databaseErr := runner.Close()
				if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
					err = upErr
				} else if sourceErr != nil || databaseErr != nil {
					err = errors.New("could not close isolated migration runner")
				}
			}
		}
	}
	if err == nil {
		var latest uint64
		latest, err = latestOfficialMigration(directory)
		if err == nil {
			var pool *pgxpool.Pool
			pool, err = s.OpenPool(ctx)
			if err == nil {
				var version int64
				var dirty bool
				err = pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty)
				pool.Close()
				if err == nil && (dirty || version < 0 || uint64(version) != latest) {
					err = errors.New("isolated schema migrations did not finish at the latest clean version")
				}
			}
		}
	}
	if err != nil {
		cleanupErr := s.Cleanup()
		if cleanupErr != nil {
			return errors.New("official migrations failed and isolated schema cleanup also failed")
		}
		return errors.New("could not apply official migrations to isolated schema")
	}
	return nil
}

// Cleanup closes schema-scoped pools and drops only the schema created by this
// IsolatedSchema value. It is idempotent and never targets public or the whole
// database.
func (s *IsolatedSchema) Cleanup() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	pools := append([]*pgxpool.Pool(nil), s.pools...)
	s.pools = nil
	control := s.control
	s.control = nil
	s.mu.Unlock()
	for _, pool := range pools {
		pool.Close()
	}
	var dropErr error
	if control != nil {
		if validSchemaName(s.name) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, dropErr = control.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{s.name}.Sanitize()+" CASCADE")
			cancel()
		} else {
			dropErr = errors.New("refusing cleanup for invalid integration schema name")
		}
		control.Close()
	}
	if dropErr != nil {
		return errors.New("could not drop owned integration schema")
	}
	return nil
}

func validSchemaLabel(label string) bool {
	if len(label) == 0 || len(label) > 20 || label[0] < 'a' || label[0] > 'z' {
		return false
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func migrationURL(dsn, schema string) (string, error) {
	if !validSchemaName(schema) {
		return "", errors.New("integration migration schema is invalid")
	}
	if _, err := ParseURL(dsn); err != nil {
		return "", err
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return "", errors.New("TEST_DATABASE_URL is invalid")
	}
	query := u.Query()
	query.Set("search_path", schema)
	query.Set("x-migrations-table", "schema_migrations")
	query.Del("x-migrations-table-quoted")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func officialMigrationsDirectory() (string, error) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("could not locate official integration migrations")
	}
	directory := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "migrations"))
	if _, err := latestOfficialMigration(directory); err != nil {
		return "", err
	}
	return directory, nil
}

func fileSourceURL(directory string) string {
	path := filepath.ToSlash(directory)
	if filepath.VolumeName(directory) != "" {
		// golang-migrate's file source joins URL host and path. Using an opaque
		// URL keeps a Windows drive (for example C:/...) intact instead of
		// turning it into a rooted path on whichever drive happens to be active.
		return (&url.URL{Scheme: "file", Opaque: path}).String()
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func latestOfficialMigration(directory string) (uint64, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0, errors.New("official migration directory is unavailable")
	}
	type migrationPair struct {
		up   string
		down string
	}
	pairs := make(map[uint64]migrationPair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			continue
		}
		version, parseErr := strconv.ParseUint(parts[0], 10, 64)
		if parseErr != nil || version == 0 {
			continue
		}
		pair := pairs[version]
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			if pair.up != "" {
				return 0, errors.New("official migration directory contains a duplicate up migration")
			}
			pair.up = strings.TrimSuffix(name, ".up.sql")
		case strings.HasSuffix(name, ".down.sql"):
			if pair.down != "" {
				return 0, errors.New("official migration directory contains a duplicate down migration")
			}
			pair.down = strings.TrimSuffix(name, ".down.sql")
		}
		pairs[version] = pair
	}
	if len(pairs) == 0 {
		return 0, errors.New("official migration directory contains no migration pairs")
	}
	var latest uint64
	for version, pair := range pairs {
		if pair.up == "" || pair.down == "" || pair.up != pair.down {
			return 0, errors.New("official migration directory contains an incomplete migration pair")
		}
		if version > latest {
			latest = version
		}
	}
	for version := uint64(1); version <= latest; version++ {
		pair, ok := pairs[version]
		if !ok || pair.up == "" || pair.down == "" || pair.up != pair.down {
			return 0, errors.New("official migration sequence contains a gap")
		}
	}
	return latest, nil
}

// OpenURL validates and opens a pool using an explicitly supplied integration
// URL, then verifies the server-side database identity and PostgreSQL major.
func OpenURL(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := ParseURL(dsn)
	if err != nil {
		return nil, err
	}
	return openConfig(ctx, config)
}

func openConfig(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, sanitizedPostgresStageError("pool_creation", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		pool.Close()
		return nil, sanitizedPostgresStageError("connection_acquisition", err)
	}
	var database, user string
	var version int
	err = conn.QueryRow(ctx, `SELECT current_database(), current_user, current_setting('server_version_num')::int`).Scan(&database, &user, &version)
	conn.Release()
	if err != nil {
		pool.Close()
		return nil, sanitizedPostgresStageError("identity_query", err)
	}
	if err := ValidateIdentity(database, user, version); err != nil {
		pool.Close()
		return nil, errors.New("connected PostgreSQL identity was rejected (database, user, or version mismatch)")
	}
	return pool, nil
}

// sanitizedPostgresStageError retains only the operation stage and SQLSTATE.
// Driver messages may contain host, username, or other connection details and
// are deliberately excluded from returned errors.
func sanitizedPostgresStageError(stage string, err error) error {
	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) && postgresErr.Code != "" {
		return fmt.Errorf("integration PostgreSQL %s failed (SQLSTATE %s)", stage, postgresErr.Code)
	}
	return fmt.Errorf("integration PostgreSQL %s failed (SQLSTATE unavailable)", stage)
}

// NewFixtureID returns a cryptographically random UUID suitable for scoping a
// test fixture and its cleanup. It does not access PostgreSQL.
func NewFixtureID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", errors.New("could not generate fixture identifier")
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:]), nil
}

// CleanupRegistrar is implemented by *testing.T and keeps the helper usable
// without making the production package depend on testing.T directly.
type CleanupRegistrar interface {
	Cleanup(func())
	Errorf(string, ...any)
}

// BeginTx begins a transaction and registers a bounded rollback for test
// cleanup. A successful Commit makes the later rollback a harmless no-op.
func BeginTx(t CleanupRegistrar, ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	if t == nil || pool == nil {
		return nil, errors.New("test and PostgreSQL pool are required")
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, errors.New("could not begin PostgreSQL test transaction")
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(cleanupCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("PostgreSQL test transaction rollback failed")
		}
	})
	return tx, nil
}

// RegisterIDCleanup registers cleanup for one explicitly created fixture ID.
// The callback must use parameterized SQL scoped to that ID, never broad table
// cleanup. For transactional fixtures prefer BeginTx and rollback instead.
func RegisterIDCleanup(t CleanupRegistrar, id string, cleanup func(context.Context, string) error) error {
	if t == nil || strings.TrimSpace(id) == "" || cleanup == nil {
		return errors.New("test, fixture ID and cleanup callback are required")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cleanup(ctx, id); err != nil {
			t.Errorf("isolated PostgreSQL fixture cleanup failed")
		}
	})
	return nil
}
