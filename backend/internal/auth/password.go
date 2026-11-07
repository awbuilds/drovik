package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	minPasswordLength = 12
	maxPasswordLength = 256
)

var (
	ErrInvalidHash    = errors.New("invalid password hash")
	ErrPasswordPolicy = errors.New("password does not meet the policy")
)

type PasswordParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var DefaultPasswordParams = PasswordParams{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: must be at least %d characters", ErrPasswordPolicy, minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		return fmt.Errorf("%w: must be at most %d characters", ErrPasswordPolicy, maxPasswordLength)
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("%w: must not be only whitespace", ErrPasswordPolicy)
	}
	return nil
}

func HashPassword(password string, params PasswordParams) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		params.KeyLength,
	)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey(
		[]byte(password),
		salt,
		params.Iterations,
		params.Memory,
		params.Parallelism,
		//nolint:gosec // hash length is bounded by the Argon2 parameters
		uint32(len(expected)),
	)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

// DummyVerify burns comparable time when a username does not exist, reducing
// the timing signal used for user enumeration.
func DummyVerify(password string) {
	_, _ = VerifyPassword(password, dummyHash())
}

var dummyHash = sync.OnceValue(func() string {
	encoded, err := HashPassword("drovik-dummy-password-not-a-real-secret", DefaultPasswordParams)
	if err != nil {
		return ""
	}
	return encoded
})

func decodeHash(encoded string) (PasswordParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	params := PasswordParams{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.Memory, &params.Iterations, &params.Parallelism); err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}
	if params.Memory < 8*1024 || params.Iterations < 1 || params.Parallelism < 1 {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(hash) < 16 {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}
	//nolint:gosec // salt and hash lengths are bounded by Argon2 parameters
	params.SaltLength = uint32(len(salt))
	//nolint:gosec // hash length is bounded by the Argon2 parameters
	params.KeyLength = uint32(len(hash))
	return params, salt, hash, nil
}
