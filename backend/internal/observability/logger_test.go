package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedaction(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New("info", "json", &buf)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	logger.Info("login",
		"username", "admin",
		"password", "super-secret",
		"token", "abc123",
		"authorization", "Bearer xyz",
		"private_key", "-----BEGIN OPENSSH PRIVATE KEY-----",
		"safe_field", "visible",
	)

	out := buf.String()
	for _, secret := range []string{"super-secret", "abc123", "Bearer xyz", "OPENSSH PRIVATE KEY"} {
		if strings.Contains(out, secret) {
			t.Fatalf("log contains secret %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "admin") || !strings.Contains(out, "visible") {
		t.Fatalf("log lost safe context: %s", out)
	}
	if !strings.Contains(out, redacted) {
		t.Fatalf("log did not mark redaction: %s", out)
	}
}

func TestRedactionNestedGroup(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New("info", "text", &buf)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("nested", slog.Group("request",
		slog.String("password", "hidden"),
		slog.String("path", "/api/v1/auth/login"),
	))
	out := buf.String()
	if strings.Contains(out, "hidden") {
		t.Fatalf("nested secret leaked: %s", out)
	}
	if !strings.Contains(out, "/api/v1/auth/login") {
		t.Fatalf("nested safe value missing: %s", out)
	}
}
