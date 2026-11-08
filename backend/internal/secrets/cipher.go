// Package secrets provides authenticated encryption for credentials stored at rest.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const CurrentVersion = 1

var (
	ErrInvalidKey = errors.New("encryption key must be 32 bytes")
	ErrDecrypt    = errors.New("secret could not be decrypted")
)

type Encrypted struct {
	Version    int
	Nonce      []byte
	Ciphertext []byte
}

type Cipher struct {
	aead cipher.AEAD
}

func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext, aad []byte) (Encrypted, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Encrypted{}, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nil, nonce, plaintext, aad)
	return Encrypted{Version: CurrentVersion, Nonce: nonce, Ciphertext: ciphertext}, nil
}

func (c *Cipher) Decrypt(encrypted Encrypted, aad []byte) ([]byte, error) {
	if encrypted.Version != CurrentVersion {
		return nil, fmt.Errorf("%w: unsupported key version %d", ErrDecrypt, encrypted.Version)
	}
	if len(encrypted.Nonce) != c.aead.NonceSize() {
		return nil, fmt.Errorf("%w: invalid nonce", ErrDecrypt)
	}
	plaintext, err := c.aead.Open(nil, encrypted.Nonce, encrypted.Ciphertext, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
