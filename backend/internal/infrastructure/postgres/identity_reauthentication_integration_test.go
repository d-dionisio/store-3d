package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/identity"
	security "example.com/loja-3d/backend/internal/security/auth"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test applies official migrations to a unique integration schema and
// exercises the real repository through independent schema-scoped pools. It
// is not an MFA/browser end-to-end test.
func TestAdminReauthenticationPostgreSQL(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	schema, err := testpostgres.CreateSchema(ctx, t, "reauth")
	if err != nil {
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated schema: %v", err)
	}
	observer, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated PostgreSQL pool: %v", err)
	}
	t.Cleanup(observer.Close)

	newPool := func(name string) *pgxpool.Pool {
		pool, poolErr := schema.OpenPoolWithApplicationName(ctx, name)
		if poolErr != nil {
			t.Fatalf("open independent schema-scoped pool %s", name)
		}
		return pool
	}
	poolA, poolB := newPool("reauth-consumer-a"), newPool("reauth-consumer-b")
	repo := NewIdentityRepository(poolA)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique fixture identifier")
	}
	ids := map[string]string{}
	for _, name := range []string{"owner", "other"} {
		var id string
		if err = observer.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,$3,'admin','active') RETURNING id::text`, name+"-"+fixtureID+"@example.invalid", "Reauth fixture", strings.Repeat("x", 60)).Scan(&id); err != nil {
			t.Fatal("insert isolated admin fixture:", err)
		}
		ids[name] = id
	}
	if _, err = observer.Exec(ctx, `INSERT INTO auth_mfa_factors(admin_user_id,secret_ciphertext,key_version,enabled_at,last_accepted_counter) VALUES($1,$2,1,now(),0)`, ids["owner"], []byte(strings.Repeat("x", 40))); err != nil {
		t.Fatal("insert isolated MFA factor fixture:", err)
	}
	session := func(owner string, expires time.Time, revoked bool) string {
		t.Helper()
		sessionFixtureID, fixtureErr := testpostgres.NewFixtureID()
		if fixtureErr != nil {
			t.Fatal("create unique session fixture identifier")
		}
		var id string
		var revokedAt any
		if revoked {
			revokedAt = time.Now().UTC()
		}
		if e := observer.QueryRow(ctx, `INSERT INTO admin_sessions(admin_user_id,token_hash,csrf_token_hash,expires_at,revoked_at,authentication_level) VALUES($1,$2,$3,$4,$5,'mfa') RETURNING id::text`, ids[owner], security.TokenHash("session-"+owner+"-"+sessionFixtureID), security.TokenHash("csrf-"+owner+"-"+sessionFixtureID), expires, revokedAt).Scan(&id); e != nil {
			t.Fatal("insert isolated session fixture:", e)
		}
		return id
	}
	liveA := session("owner", time.Now().Add(time.Hour), false)
	liveA2 := session("owner", time.Now().Add(time.Hour), false)
	liveOther := session("other", time.Now().Add(time.Hour), false)
	expired := session("owner", time.Now().Add(time.Hour), false)
	if _, err = observer.Exec(ctx, `UPDATE admin_sessions SET created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' WHERE id=$1`, expired); err != nil {
		t.Fatal("expire isolated session fixture:", err)
	}
	newHash := func(label string) string {
		t.Helper()
		id, idErr := testpostgres.NewFixtureID()
		if idErr != nil {
			t.Fatal("create unique evidence fixture identifier")
		}
		return security.TokenHash(label + id)
	}
	var acceptedCounter int64 = 10
	issue := func(id, sessionID string, ttl time.Duration) string {
		t.Helper()
		hash := newHash("evidence")
		acceptedCounter++
		counter := acceptedCounter
		if e := repo.CreateReauthEvidence(ctx, "admin", id, sessionID, &counter, "", hash, time.Now().Add(ttl)); e != nil {
			t.Fatalf("issue evidence for fixture session: %v", e)
		}
		return hash
	}
	if err = repo.CreateReauthEvidence(ctx, "admin", ids["owner"], liveA, nil, "", newHash("no-mfa-proof"), time.Now().Add(time.Minute)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("admin evidence without TOTP/recovery proof should fail closed, got %v", err)
	}

	// Valid issue/consume, then one-use rejection.
	validHash := issue(ids["owner"], liveA, time.Minute)
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA, validHash); err != nil {
		t.Fatalf("valid evidence consumption: %v", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA, validHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("replay should be rejected, got %v", err)
	}
	t.Log("valid evidence accepted once; replay rejected")

	// Expiration is enforced by PostgreSQL time, even if the evidence was
	// issued successfully while it was still valid.
	expiredHash := issue(ids["owner"], liveA, 30*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA, expiredHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expired evidence should be rejected, got %v", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA2, issue(ids["owner"], liveA, time.Minute)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("evidence from another session should be rejected, got %v", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveOther, issue(ids["owner"], liveA, time.Minute)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("evidence presented by another user's session should be rejected, got %v", err)
	}
	if err = repo.CreateReauthEvidence(ctx, "admin", ids["owner"], liveOther, nil, "", newHash("bad-owner"), time.Now().Add(time.Minute)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("issuance for a session owned by another user should be rejected, got %v", err)
	}
	revoked := session("owner", time.Now().Add(time.Hour), true)
	if err = repo.CreateReauthEvidence(ctx, "admin", ids["owner"], revoked, nil, "", newHash("revoked-issue"), time.Now().Add(time.Minute)); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("issuance for a revoked session should be rejected, got %v", err)
	}
	revokedHash := issue(ids["owner"], liveA, time.Minute)
	if _, err = observer.Exec(ctx, `UPDATE admin_sessions SET revoked_at=now() WHERE id=$1`, liveA); err != nil {
		t.Fatal("revoke isolated session:", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA, revokedHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("revoked session should be rejected, got %v", err)
	}
	expiredHash = issue(ids["owner"], liveA2, time.Minute)
	if _, err = observer.Exec(ctx, `UPDATE admin_sessions SET created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' WHERE id=$1`, liveA2); err != nil {
		t.Fatal("expire isolated session:", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA2, expiredHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("expired session should be rejected, got %v", err)
	}
	t.Log("expired evidence, wrong session/user, revoked session and expired session rejected")

	// Two independent pools both reach the same locked session. Only one may
	// commit consumption after the explicit lock is released.
	concurrentSession := session("owner", time.Now().Add(time.Hour), false)
	concurrentHash := issue(ids["owner"], concurrentSession, time.Minute)
	lock, err := observer.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lock.Exec(ctx, `SELECT id FROM admin_sessions WHERE id=$1 FOR UPDATE`, concurrentSession); err != nil {
		_ = lock.Rollback(ctx)
		t.Fatal("lock fixture session for synchronized race:", err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, pool := range []*pgxpool.Pool{poolA, poolB} {
		go func(p *pgxpool.Pool) {
			<-start
			results <- NewIdentityRepository(p).ConsumeReauthEvidence(ctx, "admin", concurrentSession, concurrentHash)
		}(pool)
	}
	close(start)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		if err = observer.QueryRow(ctx, `SELECT count(DISTINCT pid) FROM pg_stat_activity WHERE datname=current_database() AND application_name=ANY($1) AND wait_event_type='Lock'`, []string{"reauth-consumer-a", "reauth-consumer-b"}).Scan(&waiting); err != nil {
			_ = lock.Rollback(ctx)
			t.Fatal("observe real PostgreSQL lock waits:", err)
		}
		if waiting == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var waiting int
	if err = observer.QueryRow(ctx, `SELECT count(DISTINCT pid) FROM pg_stat_activity WHERE datname=current_database() AND application_name=ANY($1) AND wait_event_type='Lock'`, []string{"reauth-consumer-a", "reauth-consumer-b"}).Scan(&waiting); err != nil || waiting != 2 {
		_ = lock.Rollback(ctx)
		t.Fatalf("did not observe two independent PostgreSQL backends waiting; count=%d err=%v", waiting, err)
	}
	if err = lock.Commit(ctx); err != nil {
		t.Fatal("release synchronized session lock:", err)
	}
	passed, rejected := 0, 0
	for range 2 {
		e := <-results
		if e == nil {
			passed++
		} else if errors.Is(e, app.ErrUnauthorized) {
			rejected++
		} else {
			t.Fatalf("unexpected concurrent consume error: %v", e)
		}
	}
	if passed != 1 || rejected != 1 {
		t.Fatalf("concurrent evidence consumption accepted=%d rejected=%d", passed, rejected)
	}
	var consumedCount int
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM auth_reauthentication_evidence WHERE token_hash=$1 AND consumed_at IS NOT NULL`, concurrentHash).Scan(&consumedCount); err != nil || consumedCount != 1 {
		t.Fatalf("concurrent consumption persisted consumed rows=%d err=%v", consumedCount, err)
	}
	t.Log("concurrent independent PostgreSQL connections: exactly one consume committed")

	// Force a database write error and prove the transaction left evidence
	// untouched. This is a scoped trigger in the disposable schema only.
	failureHash := issue(ids["owner"], concurrentSession, time.Minute)
	if _, err = observer.Exec(ctx, `CREATE FUNCTION reject_reauth_consume() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated reauth persistence fault' USING ERRCODE='23514'; END $$`); err != nil {
		t.Fatal("create scoped failure trigger function:", err)
	}
	if _, err = observer.Exec(ctx, `CREATE TRIGGER reject_reauth_consume BEFORE UPDATE ON auth_reauthentication_evidence FOR EACH ROW EXECUTE FUNCTION reject_reauth_consume()`); err != nil {
		t.Fatal("create scoped failure trigger:", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", concurrentSession, failureHash); err == nil || errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("repository should propagate a persistence failure distinctly, got %v", err)
	}
	if _, err = observer.Exec(ctx, `DROP TRIGGER reject_reauth_consume ON auth_reauthentication_evidence`); err != nil {
		t.Fatal("remove scoped failure trigger:", err)
	}
	if _, err = observer.Exec(ctx, `DROP FUNCTION reject_reauth_consume()`); err != nil {
		t.Fatal("remove scoped failure function:", err)
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM auth_reauthentication_evidence WHERE token_hash=$1 AND consumed_at IS NULL`, failureHash).Scan(&consumedCount); err != nil || consumedCount != 1 {
		t.Fatalf("failed persistence changed evidence state; unconsumed rows=%d err=%v", consumedCount, err)
	}
	t.Log("induced PostgreSQL write failure propagated; evidence consumption rolled back")
}
