package terminal

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
)

func testManager() *Manager {
	cfg := &config.Config{SSHMaxSessionsPerUser: 1, SSHMaxSessionsGlobal: 2}
	return NewManager(cfg, (*servers.Service)(nil), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestAcquireLimits(t *testing.T) {
	manager := testManager()
	release, err := manager.Acquire("user-a")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := manager.Acquire("user-a"); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("expected per-user limit, got %v", err)
	}
	release()
	if _, err := manager.Acquire("user-a"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}

	releaseB, err := manager.Acquire("user-b")
	if err != nil {
		t.Fatalf("second user acquire: %v", err)
	}
	if _, err := manager.Acquire("user-c"); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("expected global limit, got %v", err)
	}
	releaseB()
}

func TestClosePreventsAcquire(t *testing.T) {
	manager := testManager()
	manager.Close()
	if _, err := manager.Acquire("user-a"); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}
