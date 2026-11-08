package storage

import (
	"context"
	"database/sql"
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

func newTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func createTestUser(t *testing.T, db *sql.DB) User {
	t.Helper()
	users := NewUserRepository(db)
	now := time.Now().UTC()
	user := User{
		ID: NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := users.CreateFirstAdmin(context.Background(), user); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	return user
}

func TestServerRepository(t *testing.T) {
	db := newTestDatabase(t)
	user := createTestUser(t, db)
	servers := NewServerRepository(db)
	now := time.Now().UTC()
	server := Server{
		ID: NewID(), OwnerUserID: user.ID, DisplayName: "test host",
		Host: "192.0.2.10", Port: 22, Username: "ops", AuthMethod: "password",
		EncryptedSecret: "ciphertext", SecretNonce: "nonce", SecretVersion: 1,
		Tags: "lab", Notes: "test", CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	if err := servers.Create(context.Background(), server); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := servers.GetByID(context.Background(), server.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DisplayName != server.DisplayName || got.OwnerUserID != user.ID {
		t.Fatalf("unexpected server: %+v", got)
	}

	list, err := servers.ListByOwner(context.Background(), user.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByOwner: %v %+v", err, list)
	}

	if err := servers.UpdateLastConnected(context.Background(), server.ID, now); err != nil {
		t.Fatalf("UpdateLastConnected: %v", err)
	}
	got, _ = servers.GetByID(context.Background(), server.ID)
	if got.LastConnectedAt == nil {
		t.Fatal("expected last connected timestamp")
	}

	got.DisplayName = "renamed"
	got.UpdatedAt = now.Add(time.Minute)
	if err := servers.Update(context.Background(), *got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := servers.GetByID(context.Background(), server.ID)
	if updated.DisplayName != "renamed" {
		t.Fatalf("update did not persist: %+v", updated)
	}

	if err := servers.Delete(context.Background(), server.ID, user.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := servers.GetByID(context.Background(), server.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestKnownHostRepository(t *testing.T) {
	db := newTestDatabase(t)
	user := createTestUser(t, db)
	servers := NewServerRepository(db)
	hosts := NewKnownHostRepository(db)
	now := time.Now().UTC()
	server := Server{
		ID: NewID(), OwnerUserID: user.ID, DisplayName: "test host",
		Host: "192.0.2.10", Port: 22, Username: "ops", AuthMethod: "password",
		EncryptedSecret: "ciphertext", SecretNonce: "nonce", SecretVersion: 1,
		CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	if err := servers.Create(context.Background(), server); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := hosts.GetActive(context.Background(), server.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected no active host, got %v", err)
	}

	first := KnownHost{
		ID: NewID(), ServerID: server.ID, Host: server.Host, Port: server.Port,
		KeyType: "ssh-ed25519", Fingerprint: "SHA256:first", PublicKey: "first",
		ApprovedAt: now, ApprovedBy: &user.ID,
	}
	if err := hosts.Approve(context.Background(), first, false); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	active, err := hosts.GetActive(context.Background(), server.ID)
	if err != nil || active.Fingerprint != "SHA256:first" {
		t.Fatalf("GetActive: %v %+v", err, active)
	}
	if err := hosts.Approve(context.Background(), first, false); err == nil {
		t.Fatal("expected duplicate approval to fail")
	}

	second := first
	second.ID = NewID()
	second.Fingerprint = "SHA256:second"
	second.ApprovedAt = now.Add(time.Minute)
	if err := hosts.Approve(context.Background(), second, true); err != nil {
		t.Fatalf("Approve replace: %v", err)
	}
	active, err = hosts.GetActive(context.Background(), server.ID)
	if err != nil || active.Fingerprint != "SHA256:second" {
		t.Fatalf("GetActive after replace: %v %+v", err, active)
	}
	if err := hosts.RevokeActive(context.Background(), server.ID, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("RevokeActive: %v", err)
	}
	if _, err := hosts.GetActive(context.Background(), server.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected no active host after revoke, got %v", err)
	}
}

func TestCapabilitiesRepository(t *testing.T) {
	db := newTestDatabase(t)
	user := createTestUser(t, db)
	servers := NewServerRepository(db)
	caps := NewCapabilitiesRepository(db)
	now := time.Now().UTC()
	server := Server{
		ID: NewID(), OwnerUserID: user.ID, DisplayName: "test host",
		Host: "192.0.2.10", Port: 22, Username: "ops", AuthMethod: "password",
		EncryptedSecret: "ciphertext", SecretNonce: "nonce", SecretVersion: 1,
		CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	if err := servers.Create(context.Background(), server); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := caps.Upsert(context.Background(), server.ID, `{"os":"Linux"}`, now); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	record, err := caps.Get(context.Background(), server.ID)
	if err != nil || record.Data != `{"os":"Linux"}` {
		t.Fatalf("Get: %v %+v", err, record)
	}
	if err := caps.Upsert(context.Background(), server.ID, `{"os":"Linux","systemd":true}`, now.Add(time.Minute)); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	record, _ = caps.Get(context.Background(), server.ID)
	if record.Data != `{"os":"Linux","systemd":true}` {
		t.Fatalf("unexpected updated capabilities: %+v", record)
	}
	if err := caps.Delete(context.Background(), server.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := caps.Get(context.Background(), server.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAuditListScopesToUser(t *testing.T) {
	db := newTestDatabase(t)
	users := NewUserRepository(db)
	first := createTestUser(t, db)
	second := User{
		ID: NewID(), Username: "second", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := users.Create(context.Background(), second); err != nil {
		t.Fatalf("Create second user: %v", err)
	}

	servers := NewServerRepository(db)
	audit := NewAuditRepository(db)
	now := time.Now().UTC()
	firstServer := Server{
		ID: NewID(), OwnerUserID: first.ID, DisplayName: "first", Host: "192.0.2.10", Port: 22,
		Username: "ops", AuthMethod: "password", EncryptedSecret: "x", SecretNonce: "x",
		SecretVersion: 1, CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	secondServer := firstServer
	secondServer.ID = NewID()
	secondServer.OwnerUserID = second.ID
	secondServer.DisplayName = "second"
	secondServer.Host = "192.0.2.11"
	for _, server := range []Server{firstServer, secondServer} {
		if err := servers.Create(context.Background(), server); err != nil {
			t.Fatalf("Create server: %v", err)
		}
	}
	for index, event := range []AuditEvent{
		{ID: NewID(), ActorUserID: &first.ID, ServerID: &firstServer.ID, EventType: "file.write", Outcome: "success", CreatedAt: now},
		{ID: NewID(), ActorUserID: &second.ID, ServerID: &secondServer.ID, EventType: "process.signal", Outcome: "success", CreatedAt: now.Add(time.Second)},
		{ID: NewID(), ActorUserID: &second.ID, EventType: "auth.login", Outcome: "success", CreatedAt: now.Add(2 * time.Second)},
	} {
		if err := audit.Insert(context.Background(), event); err != nil {
			t.Fatalf("Insert event %d: %v", index, err)
		}
	}

	events, err := audit.ListRecentForUser(context.Background(), first.ID, 100)
	if err != nil {
		t.Fatalf("ListRecentForUser: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "file.write" {
		t.Fatalf("expected only the first user's event, got %+v", events)
	}
	secondEvents, err := audit.ListRecentForUser(context.Background(), second.ID, 100)
	if err != nil {
		t.Fatalf("ListRecentForUser second: %v", err)
	}
	if len(secondEvents) != 2 {
		t.Fatalf("expected the second user's events, got %+v", secondEvents)
	}
}

func TestUserRepositoryGuardBlocksLastAdminDeactivation(t *testing.T) {
	db := newTestDatabase(t)
	users := NewUserRepository(db)
	first := createTestUser(t, db)
	now := time.Now().UTC()
	second := User{
		ID: NewID(), Username: "second-admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := users.Create(context.Background(), second); err != nil {
		t.Fatalf("Create second admin: %v", err)
	}

	// Deactivating one of two active admins is allowed.
	if err := users.UpdateRoleAndStatus(context.Background(), first.ID, "admin", false, now); err != nil {
		t.Fatalf("deactivate first admin: %v", err)
	}
	// The statement-level guard must reject deactivating the last active admin
	// even when the service-level pre-check is bypassed (concurrent writers).
	if err := users.UpdateRoleAndStatus(context.Background(), second.ID, "admin", false, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected last-admin guard rejection, got %v", err)
	}
	count, err := users.CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one active admin, got %d", count)
	}
}
