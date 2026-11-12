package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Cipher struct {
	keys    map[int16][]byte
	Version int16
}

func NewCipher(encoded string, version int16) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 || version < 1 {
		return nil, errors.New("AUTH_MFA_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
	}
	return &Cipher{keys: map[int16][]byte{version: key}, Version: version}, nil
}
func NewKeyring(active int16, encoded map[int16]string) (*Cipher, error) {
	keys := make(map[int16][]byte, len(encoded))
	for version, value := range encoded {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || version < 1 {
			return nil, errors.New("every MFA key must be base64 for exactly 32 bytes")
		}
		keys[version] = key
	}
	if _, ok := keys[active]; !ok {
		return nil, errors.New("active MFA key version is unavailable")
	}
	return &Cipher{keys: keys, Version: active}, nil
}
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.keys[c.Version])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, []byte("loja3d:mfa:v"+strconv.Itoa(int(c.Version)))), nil
}
func (c *Cipher) Decrypt(value []byte, version int16) ([]byte, error) {
	key, ok := c.keys[version]
	if !ok {
		return nil, errors.New("unsupported MFA key version")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(value) < gcm.NonceSize() {
		return nil, errors.New("invalid ciphertext")
	}
	nonce, ciphertext := value[:gcm.NonceSize()], value[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, []byte("loja3d:mfa:v"+strconv.Itoa(int(version))))
}

func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
func TOTPUri(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}
func totpAt(secret string, counter int64) (string, error) {
	key, e := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if e != nil {
		return "", e
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	m := hmac.New(sha1.New, key)
	_, _ = m.Write(msg[:])
	sum := m.Sum(nil)
	offset := sum[len(sum)-1] & 15
	value := (uint32(sum[offset])&0x7f)<<24 | uint32(sum[offset+1])<<16 | uint32(sum[offset+2])<<8 | uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1000000), nil
}
func ValidateTOTP(secret, code string, now time.Time, last *int64) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	current := now.Unix() / 30
	for _, d := range []int64{0, -1, 1} {
		counter := current + d
		if last != nil && counter <= *last {
			continue
		}
		expected, e := totpAt(secret, counter)
		if e == nil && hmac.Equal([]byte(expected), []byte(code)) {
			return counter, true
		}
	}
	return 0, false
}
func Token() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	s := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(s[:])
	return
}
func TokenHash(raw string) string { s := sha256.Sum256([]byte(raw)); return hex.EncodeToString(s[:]) }
func RecoveryCodes(count int) ([]string, []string, error) {
	raw := make([]string, 0, count)
	hashes := make([]string, 0, count)
	for range count {
		b := make([]byte, 10)
		if _, e := rand.Read(b); e != nil {
			return nil, nil, e
		}
		code := strings.ToUpper(hex.EncodeToString(b[:5]) + "-" + hex.EncodeToString(b[5:]))
		raw = append(raw, code)
		hashes = append(hashes, TokenHash(code))
	}
	return raw, hashes, nil
}
