package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	security "example.com/loja-3d/backend/internal/security/auth"
)

type reauthRepositoryStub struct {
	Repository
	identity Identity
	factor   Factor
	created  bool
	expires  time.Time
	counter  *int64
}

func (r *reauthRepositoryStub) FindIdentityByID(context.Context, string, string) (Identity, error) {
	return r.identity, nil
}
func (r *reauthRepositoryStub) GetFactor(context.Context, string, string) (Factor, error) {
	return r.factor, nil
}
func (r *reauthRepositoryStub) CreateReauthEvidence(_ context.Context, _, _, _ string, counter *int64, _, _ string, expires time.Time) error {
	r.created, r.expires, r.counter = true, expires, counter
	return nil
}
func (*reauthRepositoryStub) SecurityEvent(context.Context, string, *string, string, string, string) error {
	return nil
}

func TestAdminReauthenticationRequiresMFAAndUsesConfiguredTTL(t *testing.T) {
	const password = "a-long-admin-test-password"
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_794_500_000, 0)
	secret := "JBSWY3DPEHPK3PXP"
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipher, err := security.NewCipher(base64.StdEncoding.EncodeToString(key), 1)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := cipher.Encrypt([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	repo := &reauthRepositoryStub{
		identity: Identity{ID: "admin-test-id", Kind: "admin", Status: "active", PasswordHash: passwordHash},
		factor:   Factor{ID: "factor-test-id", IdentityID: "admin-test-id", Kind: "admin", Ciphertext: ciphertext, KeyVersion: 1, Enabled: true},
	}
	service := New(repo, nil, cipher, "")
	service.now = func() time.Time { return now }
	service.SetReauthenticationTTL(2 * time.Minute)
	code := testTOTP(secret, now)
	raw, err := service.Reauthenticate(context.Background(), "admin", "admin-test-id", "session-test-id", password, code, "", "127.0.0.1")
	if err != nil {
		t.Fatalf("valid password + TOTP should issue evidence: %v", err)
	}
	if !repo.created || len(raw) < 32 || repo.expires.Sub(now) != 2*time.Minute || repo.counter == nil {
		t.Fatalf("evidence issuance was not bound to configured TTL/TOTP: created=%v ttl=%s counter=%v", repo.created, repo.expires.Sub(now), repo.counter != nil)
	}

	withoutFactor := &reauthRepositoryStub{identity: repo.identity, factor: Factor{ID: "factor-disabled", Enabled: false}}
	noMFA := New(withoutFactor, nil, cipher, "")
	noMFA.now = func() time.Time { return now }
	if _, err = noMFA.Reauthenticate(context.Background(), "admin", "admin-test-id", "session-test-id", password, code, "", "127.0.0.1"); err != ErrUnauthorized {
		t.Fatalf("admin without enabled MFA should be rejected, got %v", err)
	}
	if withoutFactor.created {
		t.Fatal("admin without MFA persisted reauthentication evidence")
	}
}

func testTOTP(secret string, now time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(now.Unix()/30))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}
