package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DROVIK_ENV":                  EnvDevelopment,
		"DROVIK_PUBLIC_URL":           "http://localhost:8080",
		"DROVIK_ENCRYPTION_KEY":       testKey(),
		"DROVIK_SESSION_IDLE_TTL":     "1h",
		"DROVIK_SESSION_ABSOLUTE_TTL": "24h",
	}
}

func TestLoadValid(t *testing.T) {
	cfg, err := load(lookupFrom(validEnv()))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("unexpected HTTP addr: %q", cfg.HTTPAddr)
	}
	if cfg.SessionIdleTTL != time.Hour {
		t.Fatalf("unexpected idle TTL: %s", cfg.SessionIdleTTL)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Fatalf("unexpected key length: %d", len(cfg.EncryptionKey))
	}
	if len(cfg.TrustedOrigins) == 0 || cfg.TrustedOrigins[0] != "http://localhost:8080" {
		t.Fatalf("unexpected trusted origins: %v", cfg.TrustedOrigins)
	}
}

func TestLoadMissingEncryptionKey(t *testing.T) {
	env := validEnv()
	delete(env, "DROVIK_ENCRYPTION_KEY")
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Fatalf("expected encryption key error, got %v", err)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	env := validEnv()
	env["DROVIK_SESSION_IDLE_TTL"] = "not-a-duration"
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "DROVIK_SESSION_IDLE_TTL") {
		t.Fatalf("expected duration error, got %v", err)
	}
}

func TestLoadRejectsShortKey(t *testing.T) {
	env := validEnv()
	env["DROVIK_ENCRYPTION_KEY"] = base64.StdEncoding.EncodeToString([]byte("short"))
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected key length error, got %v", err)
	}
}

func TestLoadRejectsProductionHTTP(t *testing.T) {
	env := validEnv()
	env["DROVIK_ENV"] = EnvProduction
	env["DROVIK_COOKIE_SECURE"] = "true"
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https error, got %v", err)
	}
}

func TestLoadRejectsAbsoluteTTLNotGreaterThanIdle(t *testing.T) {
	env := validEnv()
	env["DROVIK_SESSION_IDLE_TTL"] = "24h"
	env["DROVIK_SESSION_ABSOLUTE_TTL"] = "24h"
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "ABSOLUTE_TTL") {
		t.Fatalf("expected TTL ordering error, got %v", err)
	}
}

func TestLoadRejectsTrustedOriginWithPath(t *testing.T) {
	env := validEnv()
	env["DROVIK_TRUSTED_ORIGINS"] = "https://example.com/app"
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "must not include a path") {
		t.Fatalf("expected origin path error, got %v", err)
	}
}
