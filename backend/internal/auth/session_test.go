package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func newTestManager(t *testing.T) (*SessionManager, *storage.UserRepository, *fakeClock, *storage.User) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatalf("storage.Migrate: %v", err)
	}

	users := storage.NewUserRepository(db)
	sessions := storage.NewSessionRepository(db)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	clock := &fakeClock{now: now}
	user := &storage.User{
		ID: storage.NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), *user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}

	manager := NewSessionManager(
		sessions, users, time.Hour, 24*time.Hour, false,
		[]byte("0123456789abcdef0123456789abcdef"),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	manager.now = clock.Now
	return manager, users, clock, user
}

func TestSessionLifecycle(t *testing.T) {
	manager, _, _, user := newTestManager(t)
	ctx := context.Background()

	session, token, csrf, err := manager.Create(ctx, user.ID, "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if session.UserID != user.ID || token == "" || csrf == "" {
		t.Fatalf("unexpected session: %+v token=%q csrf=%q", session, token, csrf)
	}

	got, gotUser, err := manager.Validate(ctx, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got.ID != session.ID || gotUser.ID != user.ID {
		t.Fatalf("unexpected validated session/user: %+v %+v", got, gotUser)
	}

	if _, _, err := manager.Validate(ctx, "wrong-token"); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession, got %v", err)
	}
}

func TestSessionIdleExpiry(t *testing.T) {
	manager, _, clock, user := newTestManager(t)
	ctx := context.Background()

	_, token, _, err := manager.Create(ctx, user.ID, "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	clock.now = clock.now.Add(2 * time.Hour)
	if _, _, err := manager.Validate(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected expired session, got %v", err)
	}
}

func TestSessionRevoke(t *testing.T) {
	manager, _, _, user := newTestManager(t)
	ctx := context.Background()

	_, token, _, err := manager.Create(ctx, user.ID, "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := manager.RevokeByToken(ctx, token); err != nil {
		t.Fatalf("RevokeByToken: %v", err)
	}
	if _, _, err := manager.Validate(ctx, token); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected revoked session, got %v", err)
	}
}

func TestSessionCookie(t *testing.T) {
	manager, _, _, _ := newTestManager(t)
	recorder := httptest.NewRecorder()
	manager.SetSessionCookie(recorder, "session-token")

	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != DefaultSessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected session cookie: %+v", cookies[0])
	}
	if cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected SameSite policy: %v", cookies[0].SameSite)
	}
}

func TestCSRFToken(t *testing.T) {
	manager, _, _, user := newTestManager(t)
	_, token, csrf, err := manager.Create(context.Background(), user.ID, "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if csrf == "" {
		t.Fatal("expected a CSRF token")
	}
	if !manager.ValidateCSRF(token, csrf) {
		t.Fatal("expected CSRF token to validate")
	}
	if manager.ValidateCSRF(token, "wrong") {
		t.Fatal("wrong CSRF token validated")
	}
	if manager.ValidateCSRF(token+"x", csrf) {
		t.Fatal("CSRF token validated for a different session")
	}
	if manager.ValidateCSRF("", csrf) {
		t.Fatal("empty session validated")
	}
}
