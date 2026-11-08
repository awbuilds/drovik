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

func TestLoadSSHAndEgressConfiguration(t *testing.T) {
	env := validEnv()
	env["DROVIK_SSH_DIAL_TIMEOUT"] = "5s"
	env["DROVIK_SSH_HANDSHAKE_TIMEOUT"] = "7s"
	env["DROVIK_SSH_OPERATION_TIMEOUT"] = "11s"
	env["DROVIK_EGRESS_ALLOW_PRIVATE"] = "false"
	env["DROVIK_EGRESS_ALLOWED_CIDRS"] = "127.0.0.1/32, 192.0.2.0/24"
	env["DROVIK_EGRESS_DENIED_CIDRS"] = "10.0.0.0/8"
	env["DROVIK_SSH_MAX_SESSIONS_PER_USER"] = "3"
	env["DROVIK_SSH_MAX_SESSIONS_GLOBAL"] = "8"

	cfg, err := load(lookupFrom(env))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.SSHDialTimeout != 5*time.Second || cfg.SSHHandshakeTimeout != 7*time.Second || cfg.SSHOperationTimeout != 11*time.Second {
		t.Fatalf("unexpected SSH timeouts: %s %s %s", cfg.SSHDialTimeout, cfg.SSHHandshakeTimeout, cfg.SSHOperationTimeout)
	}
	if cfg.EgressAllowPrivate {
		t.Fatal("expected private egress to be disabled")
	}
	if len(cfg.EgressAllowedPrefixes) != 2 || len(cfg.EgressDeniedPrefixes) != 1 {
		t.Fatalf("unexpected egress prefixes: %v %v", cfg.EgressAllowedPrefixes, cfg.EgressDeniedPrefixes)
	}
	if cfg.SSHMaxSessionsPerUser != 3 || cfg.SSHMaxSessionsGlobal != 8 {
		t.Fatalf("unexpected session limits: %d %d", cfg.SSHMaxSessionsPerUser, cfg.SSHMaxSessionsGlobal)
	}
}

func TestLoadRejectsInvalidEgressCIDR(t *testing.T) {
	env := validEnv()
	env["DROVIK_EGRESS_ALLOWED_CIDRS"] = "not-a-cidr"
	_, err := load(lookupFrom(env))
	if err == nil || !strings.Contains(err.Error(), "invalid CIDR") {
		t.Fatalf("expected CIDR error, got %v", err)
	}
}

func TestLoadMonitoringConfiguration(t *testing.T) {
	env := validEnv()
	env["DROVIK_MONITOR_REFRESH_INTERVAL"] = "3s"
	env["DROVIK_MONITOR_STALE_AFTER"] = "30s"
	env["DROVIK_MONITOR_TIMEOUT"] = "8s"
	env["DROVIK_MONITOR_MAX_OUTPUT_BYTES"] = "131072"
	env["DROVIK_MONITOR_MAX_FILESYSTEMS"] = "32"
	env["DROVIK_MONITOR_MAX_INTERFACES"] = "16"

	cfg, err := load(lookupFrom(env))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.MonitorRefreshInterval != 3*time.Second || cfg.MonitorStaleAfter != 30*time.Second || cfg.MonitorTimeout != 8*time.Second {
		t.Fatalf("unexpected monitoring durations: %s %s %s", cfg.MonitorRefreshInterval, cfg.MonitorStaleAfter, cfg.MonitorTimeout)
	}
	if cfg.MonitorMaxOutputBytes != 131072 || cfg.MonitorMaxFilesystems != 32 || cfg.MonitorMaxInterfaces != 16 {
		t.Fatalf("unexpected monitoring limits: %d %d %d", cfg.MonitorMaxOutputBytes, cfg.MonitorMaxFilesystems, cfg.MonitorMaxInterfaces)
	}
}

func TestLoadRejectsInvalidMonitoringConfiguration(t *testing.T) {
	env := validEnv()
	env["DROVIK_MONITOR_REFRESH_INTERVAL"] = "500ms"
	if _, err := load(lookupFrom(env)); err == nil {
		t.Fatal("expected a too-short refresh interval to be rejected")
	}

	env = validEnv()
	env["DROVIK_MONITOR_REFRESH_INTERVAL"] = "10s"
	env["DROVIK_MONITOR_STALE_AFTER"] = "5s"
	if _, err := load(lookupFrom(env)); err == nil {
		t.Fatal("expected a stale window shorter than the refresh interval to be rejected")
	}
}

func TestLoadOperationsConfiguration(t *testing.T) {
	env := validEnv()
	env["DROVIK_OPS_COMMAND_TIMEOUT"] = "9s"
	env["DROVIK_PROCESS_MAX_ENTRIES"] = "750"
	env["DROVIK_SERVICE_MAX_UNITS"] = "400"
	env["DROVIK_LOG_MAX_LINES"] = "300"
	env["DROVIK_LOG_MAX_OUTPUT_BYTES"] = "524288"

	cfg, err := load(lookupFrom(env))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.OpsCommandTimeout != 9*time.Second || cfg.ProcessMaxEntries != 750 || cfg.ServiceMaxUnits != 400 {
		t.Fatalf("unexpected operations config: %+v", cfg)
	}
	if cfg.LogMaxLines != 300 || cfg.LogMaxOutputBytes != 524288 {
		t.Fatalf("unexpected log config: %d %d", cfg.LogMaxLines, cfg.LogMaxOutputBytes)
	}
}

func TestLoadRejectsInvalidOperationsConfiguration(t *testing.T) {
	env := validEnv()
	env["DROVIK_PROCESS_MAX_ENTRIES"] = "0"
	if _, err := load(lookupFrom(env)); err == nil {
		t.Fatal("expected zero process entries to be rejected")
	}

	env = validEnv()
	env["DROVIK_LOG_MAX_LINES"] = "-1"
	if _, err := load(lookupFrom(env)); err == nil {
		t.Fatal("expected a negative log line limit to be rejected")
	}
}
