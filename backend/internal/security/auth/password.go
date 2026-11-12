package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const MinPasswordRunes = 12
const MaxPasswordBytes = 1024

var ErrInvalidPassword = errors.New("invalid password")

func ValidatePassword(password string) error {
	if len([]rune(password)) < MinPasswordRunes || len([]byte(password)) > MaxPasswordBytes {
		return ErrInvalidPassword
	}
	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s", b64.EncodeToString(salt), b64.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) (bool, bool) {
	if len([]byte(password)) > MaxPasswordBytes {
		return false, false
	}
	if strings.HasPrefix(encoded, "$2") {
		err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(password))
		return err == nil, err == nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, false
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false, false
	}
	memory, e1 := strconv.ParseUint(strings.TrimPrefix(params[0], "m="), 10, 32)
	timeCost, e2 := strconv.ParseUint(strings.TrimPrefix(params[1], "t="), 10, 32)
	parallel, e3 := strconv.ParseUint(strings.TrimPrefix(params[2], "p="), 10, 8)
	if e1 != nil || e2 != nil || e3 != nil || memory > 64*1024 || timeCost > 10 || parallel > 8 {
		return false, false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[4])
	expected, e2 := base64.RawStdEncoding.DecodeString(parts[5])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(expected) != 32 {
		return false, false
	}
	actual := argon2.IDKey([]byte(password), salt, uint32(timeCost), uint32(memory), uint8(parallel), uint32(len(expected)))
	ok := subtle.ConstantTimeCompare(actual, expected) == 1
	needsRehash := ok && (memory != 19*1024 || timeCost != 2 || parallel != 1)
	return ok, needsRehash
}
