package auth

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestPasswordArgon2id(t *testing.T) {
	encoded, e := HashPassword("uma passphrase longa e segura")
	if e != nil {
		t.Fatal(e)
	}
	if ok, rehash := VerifyPassword(encoded, "uma passphrase longa e segura"); !ok || rehash {
		t.Fatalf("ok=%v rehash=%v", ok, rehash)
	}
	if ok, _ := VerifyPassword(encoded, "senha incorreta"); ok {
		t.Fatal("accepted invalid password")
	}
}
func TestTOTPReplayAndEncryption(t *testing.T) {
	secret, e := NewTOTPSecret()
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1794500000, 0)
	counter := now.Unix() / 30
	code, e := totpAt(secret, counter)
	if e != nil {
		t.Fatal(e)
	}
	accepted, ok := ValidateTOTP(secret, code, now, nil)
	if !ok || accepted != counter {
		t.Fatal("valid TOTP rejected")
	}
	if _, ok = ValidateTOTP(secret, code, now, &accepted); ok {
		t.Fatal("replayed TOTP accepted")
	}
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	cipher, e := NewCipher(base64.StdEncoding.EncodeToString(key), 1)
	if e != nil {
		t.Fatal(e)
	}
	encrypted, e := cipher.Encrypt([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	plain, e := cipher.Decrypt(encrypted, 1)
	if e != nil || string(plain) != secret {
		t.Fatal("cipher roundtrip failed")
	}
}
