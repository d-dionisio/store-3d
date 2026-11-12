package auth

import (
	"bytes"
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

func TestKeyringRotationAndCorruption(t *testing.T) {
	oldKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	newKey := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	oldCipher, err := NewCipher(oldKey, 1)
	if err != nil {
		t.Fatal(err)
	}
	value, err := oldCipher.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := NewKeyring(2, map[int16]string{1: oldKey, 2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := keyring.Decrypt(value, 1)
	if err != nil || string(plain) != "secret" {
		t.Fatalf("old key: %q %v", plain, err)
	}
	withoutOld, err := NewKeyring(2, map[int16]string{2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = withoutOld.Decrypt(value, 1); err == nil {
		t.Fatal("missing old key accepted")
	}
	value[len(value)-1] ^= 1
	if _, err = keyring.Decrypt(value, 1); err == nil {
		t.Fatal("corrupted ciphertext accepted")
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
