package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testenv"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

const unitFixture = `ssh.service loaded active running OpenBSD Secure Shell server
cron.service loaded inactive dead Regular background program processing daemon
@@drovik-unit-files@@
ssh.service enabled enabled
cron.service disabled disabled
`

func TestParseUnits(t *testing.T) {
	units, truncated := parseUnits(unitFixture, 10)
	if truncated || len(units) != 2 {
		t.Fatalf("unexpected units: %+v truncated=%v", units, truncated)
	}
	if units[0].Name != "cron.service" || units[0].Active != "inactive" || units[0].FileState != "disabled" {
		t.Fatalf("unexpected cron unit: %+v", units[0])
	}
	if units[1].Name != "ssh.service" || units[1].Active != "active" || units[1].FileState != "enabled" {
		t.Fatalf("unexpected ssh unit: %+v", units[1])
	}
	if units[1].Description != "OpenBSD Secure Shell server" {
		t.Fatalf("unexpected description: %q", units[1].Description)
	}

	limited, truncated := parseUnits(unitFixture, 1)
	if len(limited) != 1 || !truncated {
		t.Fatalf("expected truncation: %+v %v", limited, truncated)
	}
}

func newServicesEnv(t *testing.T, hook func(string) (string, string, int, bool)) (*testenv.Env, *Service) {
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
	return env, NewService(env.Cfg, env.ServerService, env.Audit, logger)
}

func systemdHook(t *testing.T, mutation map[string]string) func(string) (string, string, int, bool) {
	t.Helper()
	return func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "systemctl list-units"):
			return unitFixture, "", 0, true
		case command == "systemctl --no-ask-password --no-pager restart ssh.service":
			mutation["restarted"] = "yes"
			return "", "", 0, true
		case command == "systemctl --no-ask-password --no-pager stop cron.service":
			return "", "Failed to stop cron.service: Interactive authentication required.\n", 1, true
		case command == "systemctl show --no-pager --property=ActiveState,SubState,UnitFileState ssh.service":
			return "ActiveState=active\nSubState=running\nUnitFileState=enabled\n", "", 0, true
		}
		return "", "", 0, false
	}
}

func TestServiceListAndAction(t *testing.T) {
	mutation := map[string]string{}
	env, service := newServicesEnv(t, systemdHook(t, mutation))

	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Units) != 2 {
		t.Fatalf("unexpected units: %+v", list)
	}

	result, err := service.Action(context.Background(), env.User.ID, env.Server.ID, "ssh.service", ActionRestart)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}
	if mutation["restarted"] != "yes" || !result.StateKnown || result.ActiveState != "active" || result.SubState != "running" {
		t.Fatalf("unexpected action result: %+v", result)
	}

	_, err = service.Action(context.Background(), env.User.ID, env.Server.ID, "cron.service", ActionStop)
	if !errors.Is(err, ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func TestServiceValidation(t *testing.T) {
	env, service := newServicesEnv(t, nil)
	for _, unit := range []string{"ssh", "ssh.service;reboot", "../ssh.service", "-ssh.service", ""} {
		if _, err := service.Action(context.Background(), env.User.ID, env.Server.ID, unit, ActionRestart); !errors.Is(err, ErrInvalidUnit) {
			t.Fatalf("expected ErrInvalidUnit for %q, got %v", unit, err)
		}
	}
	if _, err := service.Action(context.Background(), env.User.ID, env.Server.ID, "ssh.service", "mask"); err == nil {
		t.Fatal("expected an unsupported action to be rejected")
	}
}

func TestSystemdUnavailable(t *testing.T) {
	env, service := newServicesEnv(t, func(command string) (string, string, int, bool) {
		return "", "System has not been booted with systemd as init system (PID 1). Can't operate.\n", 1, true
	})
	if _, err := service.List(context.Background(), env.User.ID, env.Server.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if _, err := service.Action(context.Background(), env.User.ID, env.Server.ID, "ssh.service", ActionRestart); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func BenchmarkParseUnits(b *testing.B) {
	for index := 0; index < b.N; index++ {
		if units, _ := parseUnits(unitFixture, 100); len(units) != 2 {
			b.Fatalf("unexpected units: %+v", units)
		}
	}
}

func TestServiceActionReportsUnknownState(t *testing.T) {
	env, service := newServicesEnv(t, func(command string) (string, string, int, bool) {
		switch {
		case strings.Contains(command, "systemctl list-units"):
			return unitFixture, "", 0, true
		case strings.Contains(command, "systemctl --no-ask-password --no-pager restart ssh.service"):
			return "", "", 0, true
		case strings.Contains(command, "systemctl show"):
			return "", "Failed to connect to bus\n", 1, true
		}
		return "", "", 0, false
	})
	result, err := service.Action(context.Background(), env.User.ID, env.Server.ID, "ssh.service", ActionRestart)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}
	if result.StateKnown {
		t.Fatalf("expected an unknown state: %+v", result)
	}
	events, err := storage.NewAuditRepository(env.DB).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "service.action" {
			found = true
			if event.Outcome != "unknown" {
				t.Fatalf("unexpected audit outcome: %+v", event)
			}
		}
	}
	if !found {
		t.Fatal("expected a service.action audit event")
	}
}
