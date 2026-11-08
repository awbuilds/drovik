package secrets

import (
	"errors"
	"testing"
)

func testKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestEncryptDecrypt(t *testing.T) {
	cipher, err := New(testKey())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plaintext := []byte("correct horse battery staple")
	aad := []byte("server:123:secret")

	encrypted, err := cipher.Encrypt(plaintext, aad)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encrypted.Version != CurrentVersion {
		t.Fatalf("unexpected version: %d", encrypted.Version)
	}

	decrypted, err := cipher.Decrypt(encrypted, aad)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("unexpected plaintext: %q", decrypted)
	}
}

func TestDecryptWrongAAD(t *testing.T) {
	cipher, _ := New(testKey())
	encrypted, _ := cipher.Encrypt([]byte("secret"), []byte("server:a"))
	if _, err := cipher.Decrypt(encrypted, []byte("server:b")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
}

func TestDecryptTampered(t *testing.T) {
	cipher, _ := New(testKey())
	encrypted, _ := cipher.Encrypt([]byte("secret"), []byte("server:a"))
	encrypted.Ciphertext[0] ^= 0xff
	if _, err := cipher.Decrypt(encrypted, []byte("server:a")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
}

func TestInvalidKey(t *testing.T) {
	if _, err := New([]byte("short")); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	cipher, _ := New(testKey())
	encrypted, _ := cipher.Encrypt([]byte("secret"), []byte("server:a"))
	encrypted.Version = 99
	if _, err := cipher.Decrypt(encrypted, []byte("server:a")); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt, got %v", err)
	}
}
