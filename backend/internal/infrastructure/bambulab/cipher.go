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
	key     []byte
	version int16
}

func NewCipher(encoded string, version int16) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 || version < 1 {
		return nil, errors.New("BAMBU_LAN_ENCRYPTION_KEY must be base64 for exactly 32 bytes")
	}
	return &Cipher{key, version}, nil
}
func (c *Cipher) Version() int16 { return c.version }
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	block, e := aes.NewCipher(c.key)
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
	if version != c.version {
		return nil, errors.New("unsupported Bambu credential key version")
	}
	block, e := aes.NewCipher(c.key)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil || len(value) < gcm.NonceSize() {
		return nil, errors.New("invalid Bambu credential ciphertext")
	}
	return gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], []byte("loja3d:bambulab:v"+strconv.Itoa(int(version))))
}
