package processes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testenv"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

const processFixture = `  4242     1 root      Ssl   1.5  2.3  40960   5000 /usr/sbin/sshd -D
   100  4242 ops       R+    0.0  0.1   1024    120 bash -c sleep 10
     1     0 root      Ss    0.1  0.4  16384 999999 /sbin/init
`

func TestParseProcessLine(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	process, ok := parseProcessLine("  4242     1 root      Ssl   1.5  2.3  40960   5000 /usr/sbin/sshd -D", now)
	if !ok {
		t.Fatal("expected a parsable process")
	}
	if process.PID != 4242 || process.PPID != 1 || process.User != "root" || process.State != "Ssl" {
		t.Fatalf("unexpected process: %+v", process)
	}
	if process.CPUPercent != 1.5 || process.MemoryPercent != 2.3 || process.RSSBytes != 40960*1024 {
		t.Fatalf("unexpected metrics: %+v", process)
	}
	if !process.StartedAt.Equal(now.Add(-5000 * time.Second)) {
		t.Fatalf("unexpected start time: %s", process.StartedAt)
	}
	if process.Command != "/usr/sbin/sshd -D" {
		t.Fatalf("unexpected command: %q", process.Command)
	}

	for _, line := range []string{"", "garbage", "-1 0 root S 0 0 0 1 x"} {
		if _, ok := parseProcessLine(line, now); ok {
			t.Fatalf("expected %q to be rejected", line)
		}
	}
}

func newProcessEnv(t *testing.T, hook func(string) (string, string, int, bool)) (*testenv.Env, *Service) {
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
	return env, NewService(env.Cfg, env.ServerService, env.Audit, testLogger())
}

func TestListAndSignalProcess(t *testing.T) {
	killed := false
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 4242 -o etimes=":
			return "5000\n", "", 0, true
		case "kill -s TERM 4242":
			killed = true
			return "", "", 0, true
		case "ps -p 4242 -o stat=":
			return "", "", 1, true
		default:
			return "", "", 0, false
		}
	})

	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list.Processes) != 3 || list.Account != "test" {
		t.Fatalf("unexpected listing: %+v", list)
	}
	process := list.Processes[0]
	result, err := service.Signal(
		context.Background(), env.User.ID, env.Server.ID,
		process.PID, SignalTerm, process.StartedAt,
	)
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if !killed || !result.Delivered || result.Running {
		t.Fatalf("unexpected signal result: %+v (killed=%v)", result, killed)
	}
}

func TestSignalRejectsReusedPID(t *testing.T) {
	killed := false
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 4242 -o etimes=":
			return "10\n", "", 0, true
		case "kill -s TERM 4242":
			killed = true
			return "", "", 0, true
		}
		return "", "", 0, false
	})

	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_, err = service.Signal(
		context.Background(), env.User.ID, env.Server.ID,
		4242, SignalTerm, list.Processes[0].StartedAt,
	)
	if !errors.Is(err, ErrPIDReused) {
		t.Fatalf("expected ErrPIDReused, got %v", err)
	}
	if killed {
		t.Fatal("a reused PID must not receive a signal")
	}
}

func TestSignalPermissionDenied(t *testing.T) {
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 100 -o etimes=":
			return "120\n", "", 0, true
		case "kill -s KILL 100":
			return "", "kill: (100): Operation not permitted\n", 1, true
		}
		return "", "", 0, false
	})

	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_, err = service.Signal(
		context.Background(), env.User.ID, env.Server.ID,
		100, SignalKill, list.Processes[1].StartedAt,
	)
	if !errors.Is(err, ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}
}

func TestSignalValidation(t *testing.T) {
	env, service := newProcessEnv(t, nil)
	if _, err := service.Signal(context.Background(), env.User.ID, env.Server.ID, 0, SignalTerm, time.Now()); !errors.Is(err, ErrInvalidPID) {
		t.Fatalf("expected ErrInvalidPID, got %v", err)
	}
	if _, err := service.Signal(context.Background(), env.User.ID, env.Server.ID, 42, "SIGHUP", time.Now()); !errors.Is(err, ErrInvalidSignal) {
		t.Fatalf("expected ErrInvalidSignal, got %v", err)
	}
	if _, err := service.Signal(context.Background(), env.User.ID, env.Server.ID, 42, SignalTerm, time.Time{}); err == nil {
		t.Fatal("expected a missing identity to be rejected")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func BenchmarkParseProcessLine(b *testing.B) {
	now := time.Now().UTC()
	line := "  4242     1 root      Ssl   1.5  2.3  40960   5000 /usr/sbin/sshd -D"
	for index := 0; index < b.N; index++ {
		if _, ok := parseProcessLine(line, now); !ok {
			b.Fatal("expected a parsable process")
		}
	}
}

func TestSignalReportsUnknownWhenStateCannotBeRead(t *testing.T) {
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 4242 -o etimes=":
			return "5000\n", "", 0, true
		case "kill -s TERM 4242":
			return "", "", 0, true
		case "ps -p 4242 -o stat=":
			return "", "sh: 1: ps: not found\n", 127, true
		}
		return "", "", 0, false
	})
	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	result, err := service.Signal(
		context.Background(), env.User.ID, env.Server.ID,
		4242, SignalTerm, list.Processes[0].StartedAt,
	)
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if !result.Delivered || !result.OutcomeUnknown {
		t.Fatalf("expected delivered unknown outcome: %+v", result)
	}
	events, err := storage.NewAuditRepository(env.DB).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "process.signal" {
			found = true
			if event.Outcome != "unknown" || event.SafeErrorCode == nil || *event.SafeErrorCode != "OUTCOME_UNKNOWN" {
				t.Fatalf("unexpected audit outcome: %+v", event)
			}
		}
	}
	if !found {
		t.Fatal("expected a process.signal audit event")
	}
}

func TestSignalUnobservableIdentityIsNotReportedAsMissing(t *testing.T) {
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 4242 -o etimes=":
			return "", "sh: 1: ps: not found\n", 127, true
		}
		return "", "", 0, false
	})
	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	_, err = service.Signal(
		context.Background(), env.User.ID, env.Server.ID,
		4242, SignalTerm, list.Processes[0].StartedAt,
	)
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("an unobservable identity must not be reported as a missing process: %v", err)
	}
}

func TestSignalStateTimeoutIsUnknown(t *testing.T) {
	env, service := newProcessEnv(t, func(command string) (string, string, int, bool) {
		switch command {
		case listCommand:
			return processFixture, "", 0, true
		case "LC_ALL=C ps -p 4242 -o etimes=":
			return "5000\n", "", 0, true
		case "kill -s TERM 4242":
			return "", "", 0, true
		case "ps -p 4242 -o stat=":
			time.Sleep(600 * time.Millisecond)
			return "", "", 0, true
		}
		return "", "", 0, false
	})
	list, err := service.List(context.Background(), env.User.ID, env.Server.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(400 * time.Millisecond)
		cancel()
	}()
	result, err := service.Signal(ctx, env.User.ID, env.Server.ID, 4242, SignalTerm, list.Processes[0].StartedAt)
	if err != nil {
		t.Fatalf("Signal: %v", err)
	}
	if !result.Delivered || !result.OutcomeUnknown {
		t.Fatalf("a canceled state read must be an unknown outcome: %+v", result)
	}
	events, err := storage.NewAuditRepository(env.DB).ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "process.signal" {
			found = true
			if event.Outcome != "unknown" {
				t.Fatalf("unexpected outcome: %+v", event)
			}
		}
	}
	if !found {
		t.Fatal("expected a process.signal audit row")
	}
}
