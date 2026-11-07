package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *UserRepository {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewUserRepository(db)
}

func TestMigrateAndSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	version, err := SchemaVersion(context.Background(), db)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version < 1 {
		t.Fatalf("expected schema version >= 1, got %d", version)
	}
}

func TestCreateFirstAdminOnlyOnce(t *testing.T) {
	users := newTestDB(t)
	now := time.Now().UTC()
	first := User{
		ID: NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	created, err := users.CreateFirstAdmin(context.Background(), first)
	if err != nil || !created {
		t.Fatalf("CreateFirstAdmin: created=%v err=%v", created, err)
	}
	second := first
	second.ID = NewID()
	second.Username = "other"
	created, err = users.CreateFirstAdmin(context.Background(), second)
	if err != nil {
		t.Fatalf("CreateFirstAdmin second: %v", err)
	}
	if created {
		t.Fatal("second admin should not be created")
	}
}

func TestUserLookups(t *testing.T) {
	users := newTestDB(t)
	now := time.Now().UTC()
	user := User{
		ID: NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}

	byName, err := users.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if byName.ID != user.ID {
		t.Fatalf("unexpected user id: %s", byName.ID)
	}

	if _, err := users.GetByUsername(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := users.UpdateLastLogin(context.Background(), user.ID, now); err != nil {
		t.Fatalf("UpdateLastLogin: %v", err)
	}
	updated, err := users.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.LastLoginAt == nil {
		t.Fatal("expected last login timestamp")
	}
}

func TestSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	users := NewUserRepository(db)
	sessions := NewSessionRepository(db)
	now := time.Now().UTC()
	user := User{
		ID: NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}

	session := Session{
		ID: NewID(), TokenHash: "hash", UserID: user.ID,
		CreatedAt: now, LastSeenAt: now,
		ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		UserAgent: "test",
	}
	if err := sessions.Create(context.Background(), session); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := sessions.GetByTokenHash(context.Background(), "hash")
	if err != nil {
		t.Fatalf("GetByTokenHash: %v", err)
	}
	if got.UserID != user.ID {
		t.Fatalf("unexpected session user: %s", got.UserID)
	}

	if err := sessions.Touch(context.Background(), got.ID, now.Add(time.Minute), now.Add(2*time.Hour)); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := sessions.Revoke(context.Background(), got.ID, now); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, err := sessions.GetByTokenHash(context.Background(), "hash")
	if err != nil {
		t.Fatalf("GetByTokenHash after revoke: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("expected revoked timestamp")
	}

	deleted, err := sessions.DeleteExpired(context.Background(), now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted session, got %d", deleted)
	}
}

func TestAuditAndSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	audit := NewAuditRepository(db)
	event := AuditEvent{
		ID: NewID(), EventType: "auth.login", Outcome: "success",
		TargetSummary: "admin", CorrelationID: "req-1", CreatedAt: time.Now().UTC(),
	}
	if err := audit.Insert(context.Background(), event); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	events, err := audit.ListRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "auth.login" {
		t.Fatalf("unexpected events: %+v", events)
	}

	settings := NewSettingsRepository(db)
	if _, err := settings.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := settings.Set(context.Background(), "setup.completed", "true", time.Now().UTC()); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, err := settings.Get(context.Background(), "setup.completed")
	if err != nil || value != "true" {
		t.Fatalf("Get: value=%q err=%v", value, err)
	}
}
