package security

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	limiter := NewLoginLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	if ok, _ := limiter.Allow("user"); !ok {
		t.Fatal("first attempt should be allowed")
	}
	if ok, _ := limiter.Allow("user"); !ok {
		t.Fatal("second attempt should be allowed")
	}
	ok, retry := limiter.Allow("user")
	if ok {
		t.Fatal("third attempt should be denied")
	}
	if retry <= 0 {
		t.Fatalf("expected positive retry delay, got %s", retry)
	}

	now = now.Add(time.Minute)
	if ok, _ := limiter.Allow("user"); !ok {
		t.Fatal("attempt after window should be allowed")
	}
}

func TestIsTrustedOrigin(t *testing.T) {
	trusted := []string{"https://example.com", "http://localhost:5173"}
	if !IsTrustedOrigin(trusted, "https://example.com") {
		t.Fatal("expected trusted origin")
	}
	if IsTrustedOrigin(trusted, "https://evil.example") {
		t.Fatal("unexpected trusted origin")
	}
	if IsTrustedOrigin(trusted, "") {
		t.Fatal("empty origin must not be trusted")
	}
}
