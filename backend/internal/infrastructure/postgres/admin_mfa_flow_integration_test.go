package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
	apihttp "example.com/loja-3d/backend/internal/http"
	"example.com/loja-3d/backend/internal/http/handlers"
	security "example.com/loja-3d/backend/internal/security/auth"
	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

// This real PostgreSQL/HTTP flow applies official migrations to a uniquely
// named schema in the dedicated integration database. It never creates or
// uses a real administrator session.
func TestAdminMFASessionPostgreSQL(t *testing.T) {
	if dsn, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok || strings.TrimSpace(dsn) == "" {
		t.Skip("PostgreSQL integration test requires the dedicated TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	schema, err := testpostgres.CreateSchema(ctx, t, "admin_mfa")
	if err != nil {
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to isolated schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated PostgreSQL pool: %v", err)
	}
	observer := pool
	t.Cleanup(pool.Close)
	fixtureID, err := testpostgres.NewFixtureID()
	if err != nil {
		t.Fatal("create unique fixture identifier")
	}

	password := "Fixture-Only-Admin-Password-2026!"
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal("hash fixture password")
	}
	const key = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=" // 32 zero bytes, test-only
	cipher, err := security.NewCipher(key, 1)
	if err != nil {
		t.Fatal("create test-only MFA cipher")
	}
	secret, err := security.NewTOTPSecret()
	if err != nil {
		t.Fatal("create test-only TOTP secret")
	}
	ciphertext, err := cipher.Encrypt([]byte(secret))
	if err != nil {
		t.Fatal("encrypt test-only TOTP secret")
	}
	const recoveryCode = "TEST-RECOVERY-CODE-ONLY"
	var adminID, factorID string
	email := "mfa-flow-" + fixtureID + "@example.invalid"
	if err = observer.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role,status) VALUES($1,'MFA Fixture',$2,'admin','active') RETURNING id::text`, email, passwordHash).Scan(&adminID); err != nil {
		t.Fatal("insert isolated administrator")
	}
	if err = observer.QueryRow(ctx, `INSERT INTO auth_mfa_factors(admin_user_id,secret_ciphertext,key_version,enabled_at) VALUES($1,$2,1,now()) RETURNING id::text`, adminID, ciphertext).Scan(&factorID); err != nil {
		t.Fatal("insert isolated MFA factor")
	}
	if _, err = observer.Exec(ctx, `INSERT INTO auth_recovery_codes(factor_id,code_hash) VALUES($1,$2)`, factorID, security.TokenHash(recoveryCode)); err != nil {
		t.Fatal("insert isolated recovery code hash")
	}

	repository := NewIdentityRepository(pool)
	identityService := identityapp.New(repository, nil, cipher, "")
	adminService := adminapp.New(NewAdminRepository(pool))
	handler := handlers.NewAdminHandler(adminService, true, identityService)
	router := apihttp.NewHandlerWithAdmin("http://localhost:3000", nil, nil, nil, handler)
	serve := func(method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	login := func() string {
		res := serve(http.MethodPost, "/api/v1/admin/session", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), nil, "")
		if res.Code != http.StatusAccepted {
			t.Fatalf("valid password phase status=%d; want 202 challenge", res.Code)
		}
		var payload struct {
			Data struct {
				Challenge string `json:"challenge_token"`
			} `json:"data"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil || payload.Data.Challenge == "" {
			t.Fatal("valid password phase did not return an MFA challenge")
		}
		return payload.Data.Challenge
	}
	complete := func(challenge, code, recovery string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"challenge_token": challenge, "code": code, "recovery_code": recovery})
		return serve(http.MethodPost, "/api/v1/admin/session/mfa", string(body), nil, "")
	}
	validCode := testTOTP(secret, time.Now())

	badPassword := serve(http.MethodPost, "/api/v1/admin/session", fmt.Sprintf(`{"email":%q,"password":"wrong"}`, email), nil, "")
	if badPassword.Code != http.StatusUnauthorized {
		t.Fatalf("incorrect password status=%d; want 401", badPassword.Code)
	}
	var events int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM security_events WHERE admin_user_id=$1 AND event_type='admin.login.password' AND outcome='denied'`, adminID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("denied password security events=%d err=%v; want 1", events, err)
	}
	challenge := login()
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM security_events WHERE admin_user_id=$1 AND event_type='admin.mfa.challenge' AND outcome='success'`, adminID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("MFA challenge security events=%d err=%v; want 1", events, err)
	}
	var sessionCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions WHERE admin_user_id=$1`, adminID).Scan(&sessionCount); err != nil || sessionCount != 0 {
		t.Fatalf("sessions before MFA completion=%d err=%v; want 0", sessionCount, err)
	}
	if invalid := complete(challenge, "not-a-code", ""); invalid.Code != http.StatusUnauthorized {
		t.Fatalf("invalid TOTP status=%d; want 401", invalid.Code)
	}
	first := complete(challenge, validCode, "")
	if first.Code != http.StatusOK {
		t.Fatalf("valid TOTP status=%d; want 200", first.Code)
	}
	firstCookie := findAdminSessionCookie(t, first)
	if securityResponse := serve(http.MethodGet, "/api/v1/admin/security", "", firstCookie, ""); securityResponse.Code != http.StatusOK {
		t.Fatalf("MFA session access status=%d; want 200", securityResponse.Code)
	}
	var firstBody struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		} `json:"data"`
	}
	if err = json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil || firstBody.Data.CSRF == "" {
		t.Fatal("MFA completion response missing CSRF token")
	}
	var activeSessions int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions WHERE admin_user_id=$1 AND authentication_level='mfa' AND revoked_at IS NULL AND expires_at>now()`, adminID).Scan(&activeSessions); err != nil || activeSessions != 1 {
		t.Fatalf("persisted active MFA sessions=%d err=%v; want 1", activeSessions, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM security_events WHERE admin_user_id=$1 AND event_type='admin.mfa.login' AND outcome='success'`, adminID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("successful MFA security events=%d err=%v; want 1", events, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM security_events WHERE admin_user_id=$1 AND event_type='admin.mfa.login' AND outcome='denied'`, adminID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("denied MFA security events=%d err=%v; want 1", events, err)
	}

	// Expiry is enforced by the real session lookup; mutate only this fixture.
	if _, err = pool.Exec(ctx, `UPDATE admin_sessions SET created_at=now()-interval '2 seconds',expires_at=now()-interval '1 second' WHERE token_hash=$1`, security.TokenHash(firstCookie.Value)); err != nil {
		t.Fatal("expire isolated first session")
	}
	if expired := serve(http.MethodGet, "/api/v1/admin/security", "", firstCookie, ""); expired.Code != http.StatusUnauthorized {
		t.Fatalf("expired session status=%d; want 401", expired.Code)
	}

	// A recovery code is accepted once and then consumed in the same transaction
	// that persists the second MFA session.
	secondChallenge := login()
	second := complete(secondChallenge, "", recoveryCode)
	if second.Code != http.StatusOK {
		t.Fatalf("valid recovery-code completion status=%d; want 200", second.Code)
	}
	secondCookie := findAdminSessionCookie(t, second)
	var consumed bool
	if err = pool.QueryRow(ctx, `SELECT used_at IS NOT NULL FROM auth_recovery_codes WHERE factor_id=$1 AND code_hash=$2`, factorID, security.TokenHash(recoveryCode)).Scan(&consumed); err != nil || !consumed {
		t.Fatalf("recovery code consumed=%v err=%v", consumed, err)
	}
	if replay := complete(login(), "", recoveryCode); replay.Code != http.StatusUnauthorized {
		t.Fatalf("reused recovery code status=%d; want 401", replay.Code)
	}
	csrfBody := struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		} `json:"data"`
	}{}
	if err = json.Unmarshal(second.Body.Bytes(), &csrfBody); err != nil || csrfBody.Data.CSRF == "" {
		t.Fatal("recovery completion response missing CSRF token")
	}
	if noCSRF := serve(http.MethodDelete, "/api/v1/admin/session", "", secondCookie, ""); noCSRF.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF status=%d; want 403", noCSRF.Code)
	}
	logout := serve(http.MethodDelete, "/api/v1/admin/session", "", secondCookie, csrfBody.Data.CSRF)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout with CSRF status=%d; want 204", logout.Code)
	}
	if afterLogout := serve(http.MethodGet, "/api/v1/admin/security", "", secondCookie, ""); afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session access status=%d; want 401", afterLogout.Code)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_events WHERE admin_user_id=$1 AND action='admin.logout' AND outcome='success'`, adminID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("logout audit events=%d err=%v; want 1", events, err)
	}
}

func findAdminSessionCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name != "loja3d_admin_session" {
			continue
		}
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge <= 0 || cookie.Value == "" {
			t.Fatal("admin cookie security attributes do not satisfy expected policy")
		}
		return cookie
	}
	t.Fatal("admin session cookie missing")
	return nil
}

func testTOTP(secret string, at time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	counter := uint64(at.Unix() / 30)
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(message[:])
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	value := (uint32(hash[offset])&0x7f)<<24 | uint32(hash[offset+1])<<16 | uint32(hash[offset+2])<<8 | uint32(hash[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}
