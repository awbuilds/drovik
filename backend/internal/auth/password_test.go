package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	password := "correct horse battery staple"
	encoded, err := HashPassword(password, DefaultPasswordParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("unexpected hash format: %s", encoded)
	}

	ok, err := VerifyPassword(password, encoded)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword("wrong password", encoded)
	if err != nil {
		t.Fatalf("VerifyPassword wrong: %v", err)
	}
	if ok {
		t.Fatal("wrong password verified")
	}
}

func TestPasswordHashesAreSalted(t *testing.T) {
	first, err := HashPassword("correct horse battery staple", DefaultPasswordParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := HashPassword("correct horse battery staple", DefaultPasswordParams)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if first == second {
		t.Fatal("hashes should differ due to random salt")
	}
}

func TestPasswordPolicy(t *testing.T) {
	cases := []string{"", "short", "          ", strings.Repeat("a", maxPasswordLength+1)}
	for _, password := range cases {
		if _, err := HashPassword(password, DefaultPasswordParams); err == nil {
			t.Fatalf("expected policy error for %q", password)
		}
	}
}

func TestVerifyMalformedHash(t *testing.T) {
	cases := []string{"", "not-a-hash", "$argon2id$v=19$m=65536,t=3,p=2$bad$bad"}
	for _, encoded := range cases {
		if _, err := VerifyPassword("password", encoded); err == nil {
			t.Fatalf("expected malformed hash error for %q", encoded)
		}
	}
}
