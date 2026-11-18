package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	app "example.com/loja-3d/backend/internal/application/identity"
	security "example.com/loja-3d/backend/internal/security/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This test is deliberately narrow: it connects only to loja_3d_test, creates
// a disposable schema, and drops that exact schema after exercising the real
// repository against PostgreSQL. It is not an MFA/browser end-to-end test.
func TestAdminReauthenticationPostgreSQL(t *testing.T) {
	testURL := strings.TrimSpace(testDatabaseURL())
	devURL := strings.TrimSpace(testDevelopmentDatabaseURL())
	if testURL == "" || devURL == "" {
		t.Skip("both TEST_DATABASE_URL and DATABASE_URL are required for guarded PostgreSQL reauthentication tests")
	}
	testCfg, err := pgxpool.ParseConfig(testURL)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL; credentials hidden")
	}
	devCfg, err := pgxpool.ParseConfig(devURL)
	if err != nil || testURL == devURL || testCfg.ConnConfig.Database != "loja_3d_test" || devCfg.ConnConfig.Database != "loja_3d" {
		t.Fatal("refusing test run: URLs must be distinct and target loja_3d_test and loja_3d respectively")
	}
	host := strings.ToLower(testCfg.ConnConfig.Host)
	if (host != "localhost" && host != "127.0.0.1" && host != "::1") || testCfg.ConnConfig.Port != 5432 {
		t.Fatal("refusing test run: TEST_DATABASE_URL must target loopback:5432")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	observerCfg := testCfg.Copy()
	observerCfg.MaxConns, observerCfg.MinConns = 6, 0
	observerCfg.ConnConfig.RuntimeParams["application_name"] = "reauth-test-observer"
	observer, err := pgxpool.NewWithConfig(ctx, observerCfg)
	if err != nil {
		t.Fatal("connect to isolated test database")
	}
	t.Cleanup(observer.Close)
	var dbName, serverVersion string
	if err = observer.QueryRow(ctx, `SELECT current_database(),current_setting('server_version')`).Scan(&dbName, &serverVersion); err != nil {
		t.Fatal("verify isolated PostgreSQL connection")
	}
	if dbName != "loja_3d_test" || !strings.HasPrefix(serverVersion, "17.6") {
		t.Fatalf("refusing test run: database=%q server=%q", dbName, serverVersion)
	}
	var version int
	var dirty bool
	if err = observer.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); err != nil || version != 28 || dirty {
		t.Fatalf("expected migration 28 dirty=false; version=%d dirty=%v err=%v", version, dirty, err)
	}

	schema := fmt.Sprintf("reauth_test_%d", time.Now().UnixNano())
	if _, err = observer.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal("create disposable schema:", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := observer.Exec(cleanupCtx, `DROP SCHEMA `+schema+` CASCADE`); cleanupErr != nil {
			t.Errorf("drop exact disposable schema: %v", cleanupErr)
			return
		}
		var remains bool
		if cleanupErr := observer.QueryRow(cleanupCtx, `SELECT to_regnamespace($1) IS NOT NULL`, schema).Scan(&remains); cleanupErr != nil || remains {
			t.Errorf("disposable schema remains=%v err=%v", remains, cleanupErr)
		}
	})
	for _, table := range []string{"admin_users", "admin_sessions", "auth_reauthentication_evidence", "auth_mfa_factors"} {
		if _, err = observer.Exec(ctx, `CREATE TABLE `+schema+`.`+table+` (LIKE public.`+table+` INCLUDING ALL)`); err != nil {
			t.Fatalf("clone isolated %s table: %v", table, err)
		}
	}

	newPool := func(name string) *pgxpool.Pool {
		cfg := testCfg.Copy()
		cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
		cfg.ConnConfig.RuntimeParams["application_name"] = name
		cfg.MaxConns, cfg.MinConns = 2, 0
		pool, poolErr := pgxpool.NewWithConfig(ctx, cfg)
		if poolErr != nil {
			t.Fatalf("open independent test pool %s", name)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	poolA, poolB := newPool("reauth-consumer-a"), newPool("reauth-consumer-b")
	repo := NewIdentityRepository(poolA)
	ids := map[string]string{}
	for _, name := range []string{"owner", "other"} {
		var id string
		if err = observer.QueryRow(ctx, `INSERT INTO `+schema+`.admin_users(email,full_name,password_hash,role,status) VALUES($1,$2,$3,'admin','active') RETURNING id::text`, name+"-"+fmt.Sprint(time.Now().UnixNano())+"@example.invalid", "Reauth fixture", strings.Repeat("x", 60)).Scan(&id); err != nil {
			t.Fatal("insert isolated admin fixture:", err)
		}
		ids[name] = id
	}
	if _, err = observer.Exec(ctx, `INSERT INTO `+schema+`.auth_mfa_factors(admin_user_id,secret_ciphertext,key_version,enabled_at,last_accepted_counter) VALUES($1,$2,1,now(),0)`, ids["owner"], []byte(strings.Repeat("x", 40))); err != nil {
		t.Fatal("insert isolated MFA factor fixture:", err)
	}
	session := func(owner string, expires time.Time, revoked bool) string {
		t.Helper()
		var id string
		var revokedAt any
		if revoked {
			revokedAt = time.Now().UTC()
		}
		if e := observer.QueryRow(ctx, `INSERT INTO `+schema+`.admin_sessions(admin_user_id,token_hash,csrf_token_hash,expires_at,revoked_at,authentication_level) VALUES($1,$2,$3,$4,$5,'mfa') RETURNING id::text`, ids[owner], security.TokenHash(fmt.Sprintf("session-%s-%d", owner, time.Now().UnixNano())), security.TokenHash(fmt.Sprintf("csrf-%s-%d", owner, time.Now().UnixNano())), expires, revokedAt).Scan(&id); e != nil {
			t.Fatal("insert isolated session fixture:", e)
		}
		return id
	}
	liveA := session("owner", time.Now().Add(time.Hour), false)
	liveA2 := session("owner", time.Now().Add(time.Hour), false)
	liveOther := session("other", time.Now().Add(time.Hour), false)
	expired := session("owner", time.Now().Add(time.Hour), false)
	if _, err = observer.Exec(ctx, `UPDATE `+schema+`.admin_sessions SET created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' WHERE id=$1`, expired); err != nil {
		t.Fatal("expire isolated session fixture:", err)
	}
	newHash := func(label string) string { return security.TokenHash(label + fmt.Sprint(time.Now().UnixNano())) }
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
	if _, err = observer.Exec(ctx, `UPDATE `+schema+`.admin_sessions SET revoked_at=now() WHERE id=$1`, liveA); err != nil {
		t.Fatal("revoke isolated session:", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", liveA, revokedHash); !errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("revoked session should be rejected, got %v", err)
	}
	expiredHash = issue(ids["owner"], liveA2, time.Minute)
	if _, err = observer.Exec(ctx, `UPDATE `+schema+`.admin_sessions SET created_at=now()-interval '2 minutes',expires_at=now()-interval '1 minute' WHERE id=$1`, liveA2); err != nil {
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
	if _, err = lock.Exec(ctx, `SELECT id FROM `+schema+`.admin_sessions WHERE id=$1 FOR UPDATE`, concurrentSession); err != nil {
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
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.auth_reauthentication_evidence WHERE token_hash=$1 AND consumed_at IS NOT NULL`, concurrentHash).Scan(&consumedCount); err != nil || consumedCount != 1 {
		t.Fatalf("concurrent consumption persisted consumed rows=%d err=%v", consumedCount, err)
	}
	t.Log("concurrent independent PostgreSQL connections: exactly one consume committed")

	// Force a database write error and prove the transaction left evidence
	// untouched. This is a scoped trigger in the disposable schema only.
	failureHash := issue(ids["owner"], concurrentSession, time.Minute)
	if _, err = observer.Exec(ctx, `CREATE FUNCTION `+schema+`.reject_reauth_consume() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated reauth persistence fault' USING ERRCODE='23514'; END $$`); err != nil {
		t.Fatal("create scoped failure trigger function:", err)
	}
	if _, err = observer.Exec(ctx, `CREATE TRIGGER reject_reauth_consume BEFORE UPDATE ON `+schema+`.auth_reauthentication_evidence FOR EACH ROW EXECUTE FUNCTION `+schema+`.reject_reauth_consume()`); err != nil {
		t.Fatal("create scoped failure trigger:", err)
	}
	if err = repo.ConsumeReauthEvidence(ctx, "admin", concurrentSession, failureHash); err == nil || errors.Is(err, app.ErrUnauthorized) {
		t.Fatalf("repository should propagate a persistence failure distinctly, got %v", err)
	}
	if _, err = observer.Exec(ctx, `DROP TRIGGER reject_reauth_consume ON `+schema+`.auth_reauthentication_evidence`); err != nil {
		t.Fatal("remove scoped failure trigger:", err)
	}
	if _, err = observer.Exec(ctx, `DROP FUNCTION `+schema+`.reject_reauth_consume()`); err != nil {
		t.Fatal("remove scoped failure function:", err)
	}
	if err = observer.QueryRow(ctx, `SELECT count(*) FROM `+schema+`.auth_reauthentication_evidence WHERE token_hash=$1 AND consumed_at IS NULL`, failureHash).Scan(&consumedCount); err != nil || consumedCount != 1 {
		t.Fatalf("failed persistence changed evidence state; unconsumed rows=%d err=%v", consumedCount, err)
	}
	t.Log("induced PostgreSQL write failure propagated; evidence consumption rolled back")
}
