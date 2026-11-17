package bambulab

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
)

type Cipher struct {
	keys    map[int16][]byte
	version int16
}

func NewCipher(encoded string, version int16) (*Cipher, error) {
	return NewKeyring(version, map[int16]string{version: encoded})
}
func NewKeyring(active int16, encoded map[int16]string) (*Cipher, error) {
	if active < 1 {
		return nil, errors.New("BAMBU_LAN_KEY_VERSION must be positive")
	}
	keys := make(map[int16][]byte, len(encoded))
	for version, value := range encoded {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 || version < 1 {
			return nil, errors.New("Bambu LAN encryption keys must be base64 for exactly 32 bytes")
		}
		keys[version] = key
	}
	if _, ok := keys[active]; !ok {
		return nil, errors.New("active Bambu LAN key is missing")
	}
	return &Cipher{keys: keys, version: active}, nil
}
func (c *Cipher) Version() int16 { return c.version }
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	block, e := aes.NewCipher(c.keys[c.version])
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return nil, e
	}
	return gcm.Seal(nonce, nonce, plain, []byte("loja3d:bambulab:v"+strconv.Itoa(int(c.version)))), nil
}
func (c *Cipher) Decrypt(value []byte, version int16) ([]byte, error) {
	key, ok := c.keys[version]
	if !ok {
		return nil, errors.New("unsupported Bambu credential key version")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil || len(value) < gcm.NonceSize() {
		return nil, errors.New("invalid Bambu credential ciphertext")
	}
	return gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], []byte("loja3d:bambulab:v"+strconv.Itoa(int(version))))
}
