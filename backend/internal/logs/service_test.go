package logs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/testenv"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

const journalFixture = `{"__REALTIME_TIMESTAMP":"1700000000123456","PRIORITY":"3","MESSAGE":"failed to start","_SYSTEMD_UNIT":"ssh.service","SYSLOG_IDENTIFIER":"systemd"}
{"__REALTIME_TIMESTAMP":"1700000001123456","PRIORITY":"6","MESSAGE":"<script>alert(1)</script> accepted connection","_SYSTEMD_UNIT":"ssh.service","SYSLOG_IDENTIFIER":"sshd"}
`

func TestParseEntry(t *testing.T) {
	entry, ok := parseEntry(`{"__REALTIME_TIMESTAMP":"1700000000123456","PRIORITY":"3","MESSAGE":"failed to start","_SYSTEMD_UNIT":"ssh.service","SYSLOG_IDENTIFIER":"systemd"}`)
	if !ok {
		t.Fatal("expected a parsable entry")
	}
	if entry.Severity != "error" || entry.Priority != 3 || entry.Unit != "ssh.service" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if entry.Timestamp.Unix() != 1700000000 {
		t.Fatalf("unexpected timestamp: %s", entry.Timestamp)
	}
	if entry.Message != "failed to start" {
		t.Fatalf("unexpected message: %q", entry.Message)
	}

	byteEntry, ok := parseEntry(`{"PRIORITY":"4","MESSAGE":[104,101,108,108,111]}`)
	if !ok || byteEntry.Message != "hello" || byteEntry.Severity != "warning" {
		t.Fatalf("unexpected byte-array message: %+v", byteEntry)
	}

	if _, ok := parseEntry("not-json"); ok {
		t.Fatal("invalid JSON must be skipped")
	}
}

func newLogsEnv(t *testing.T, hook func(string) (string, string, int, bool)) (*testenv.Env, *Service) {
	t.Helper()
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	t.Cleanup(sshServer.Close)
	sshServer.CommandHook = hook
	env, err := testenv.New(t.TempDir(), sshServer)
	if err != nil {
		t.Fatalf("testenv.New: %v", err)
	}
	t.Cleanup(func() { _ = env.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return env, NewService(env.Cfg, env.ServerService, logger)
}

func TestReadJournal(t *testing.T) {
	var seen string
	env, service := newLogsEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "journalctl") {
			seen = command
			if strings.Contains(command, "--lines=10") && !strings.Contains(command, "--unit=") {
				return "", "", 0, true
			}
			return journalFixture, "", 0, true
		}
		return "", "", 0, false
	})

	result, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "ssh.service", "1h", 50)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("unexpected entries: %+v", result)
	}
	if result.Entries[0].Severity != "error" || result.Entries[1].Severity != "info" {
		t.Fatalf("unexpected severities: %+v", result.Entries)
	}
	if !strings.Contains(result.Entries[1].Message, "<script>") {
		t.Fatalf("log text must be preserved as text: %q", result.Entries[1].Message)
	}
	if !strings.Contains(seen, "--unit=ssh.service") || !strings.Contains(seen, "--lines=50") {
		t.Fatalf("unexpected journal command: %q", seen)
	}

	empty, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "all", 10)
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}
	if len(empty.Entries) != 0 {
		t.Fatalf("expected no entries: %+v", empty)
	}
}

func TestReadJournalValidation(t *testing.T) {
	env, service := newLogsEnv(t, nil)
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "bad unit", "", 10); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit, got %v", err)
	}
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "yesterday", 10); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("expected ErrInvalidRange, got %v", err)
	}
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "", 0); !errors.Is(err, ErrInvalidLines) {
		t.Fatalf("expected ErrInvalidLines, got %v", err)
	}
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "", env.Cfg.LogMaxLines+1); !errors.Is(err, ErrInvalidLines) {
		t.Fatalf("expected ErrInvalidLines above the maximum, got %v", err)
	}
}

func TestReadJournalPermissionAndUnavailable(t *testing.T) {
	env, service := newLogsEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "journalctl") {
			return "", "Hint: You are currently not seeing messages from other users and the system.\n", 1, true
		}
		return "", "", 0, false
	})
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "", 10); !errors.Is(err, ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}

	env, service = newLogsEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "journalctl") {
			return "", "journalctl: not found\n", 127, true
		}
		return "", "", 0, false
	})
	if _, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "", 10); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func BenchmarkParseJournalEntry(b *testing.B) {
	line := `{"__REALTIME_TIMESTAMP":"1700000000123456","PRIORITY":"3","MESSAGE":"failed to start","_SYSTEMD_UNIT":"ssh.service","SYSLOG_IDENTIFIER":"systemd"}`
	for index := 0; index < b.N; index++ {
		if _, ok := parseEntry(line); !ok {
			b.Fatal("expected a parsable entry")
		}
	}
}

func TestReadStopsAtRequestedLineCount(t *testing.T) {
	line := `{"__REALTIME_TIMESTAMP":"1700000000123456","PRIORITY":"6","MESSAGE":"x"}`
	flood := strings.Repeat(line+"\n", 1000)
	env, service := newLogsEnv(t, func(command string) (string, string, int, bool) {
		if strings.Contains(command, "journalctl") {
			return flood, "", 0, true
		}
		return "", "", 0, false
	})
	result, err := service.Read(context.Background(), env.User.ID, env.Server.ID, "", "1h", 50)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(result.Entries) != 50 || !result.Truncated {
		t.Fatalf("expected 50 entries and truncation, got %d (truncated=%v)", len(result.Entries), result.Truncated)
	}
}
