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
)

type PasswordHasher struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func NewPasswordHasher(memory, iterations uint32, parallelism uint8) PasswordHasher {
	return PasswordHasher{Memory: memory, Iterations: iterations, Parallelism: parallelism, SaltLength: 16, KeyLength: 32}
}

func (h PasswordHasher) Hash(password string) (string, error) {
	if len(password) < 10 || len(password) > 1024 {
		return "", ErrWeakPassword
	}
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.Iterations, h.Memory, h.Parallelism, h.KeyLength)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, h.Memory, h.Iterations, h.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

func (h PasswordHasher) Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid password hash encoding")
	}
	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil || version != argon2.Version {
		return false, errors.New("unsupported argon2 version")
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, errors.New("invalid argon2 parameters")
	}
	if memory > 1024*1024 || iterations > 20 || parallelism > 64 || memory < 8*1024 || iterations < 1 || parallelism < 1 {
		return false, errors.New("unsafe argon2 parameters")
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false, errors.New("invalid argon2 salt")
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 128 {
		return false, errors.New("invalid argon2 key")
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
