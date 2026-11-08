package teams

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

func TestTeamLifecycleAndAccess(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "teams.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	users := storage.NewUserRepository(db)
	now := time.Now().UTC()
	if _, err := users.CreateFirstAdmin(context.Background(), storage.User{
		ID: storage.NewID(), Username: "admin", PasswordHash: "hash", Role: "admin",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	alice := storage.User{
		ID: storage.NewID(), Username: "alice", PasswordHash: "hash", Role: "operator",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	bob := storage.User{
		ID: storage.NewID(), Username: "bob", PasswordHash: "hash", Role: "operator",
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	for _, user := range []storage.User{alice, bob} {
		if err := users.Create(context.Background(), user); err != nil {
			t.Fatalf("Create user: %v", err)
		}
	}
	servers := storage.NewServerRepository(db)
	server := storage.Server{
		ID: storage.NewID(), OwnerUserID: alice.ID, DisplayName: "Team host",
		Host: "192.0.2.10", Port: 22, Username: "deploy", AuthMethod: "password",
		SecretProvider: "stored", EncryptedSecret: "ciphertext", SecretNonce: "nonce",
		SecretVersion: 1, CreatedAt: now, UpdatedAt: now, IsActive: true,
	}
	if err := servers.Create(context.Background(), server); err != nil {
		t.Fatalf("Create server: %v", err)
	}
	repo := storage.NewTeamRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewService(repo, users, servers, audit.New(storage.NewAuditRepository(db), logger), logger)
	ctx := context.Background()

	team, err := service.Create(ctx, "admin", "Operations", "Production operators")
	if err != nil {
		t.Fatalf("Create team: %v", err)
	}
	if team.Name != "Operations" {
		t.Fatalf("unexpected team: %+v", team)
	}
	if err := service.AddMember(ctx, "admin", team.ID, bob.ID); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := service.SetServer(ctx, "admin", team.ID, server.ID, "read"); err != nil {
		t.Fatalf("SetServer read: %v", err)
	}
	detail, err := service.Detail(ctx, team.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(detail.Members) != 1 || detail.Members[0].Username != "bob" {
		t.Fatalf("unexpected members: %+v", detail.Members)
	}
	if len(detail.Servers) != 1 || detail.Servers[0].AccessLevel != "read" {
		t.Fatalf("unexpected team servers: %+v", detail.Servers)
	}

	level, allowed, err := repo.AccessLevel(ctx, bob.ID, server.ID)
	if err != nil || !allowed || level != "read" {
		t.Fatalf("unexpected team access: %q %v %v", level, allowed, err)
	}
	accessible, err := servers.ListAccessible(ctx, bob.ID)
	if err != nil || len(accessible) != 1 || accessible[0].ID != server.ID {
		t.Fatalf("team server missing from list: %+v %v", accessible, err)
	}
	if err := service.SetServer(ctx, "admin", team.ID, server.ID, "write"); err != nil {
		t.Fatalf("SetServer write: %v", err)
	}
	level, allowed, err = repo.AccessLevel(ctx, bob.ID, server.ID)
	if err != nil || !allowed || level != "write" {
		t.Fatalf("unexpected upgraded access: %q %v %v", level, allowed, err)
	}
	teams, err := service.List(ctx)
	if err != nil || len(teams) != 1 || teams[0].MemberCount != 1 || teams[0].ServerCount != 1 {
		t.Fatalf("unexpected team list: %+v %v", teams, err)
	}
	if err := service.RemoveMember(ctx, "admin", team.ID, bob.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if _, allowed, _ := repo.AccessLevel(ctx, bob.ID, server.ID); allowed {
		t.Fatal("removed member still has access")
	}
	if err := service.Delete(ctx, "admin", team.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := service.Detail(ctx, team.ID); err != ErrNotFound {
		t.Fatalf("expected deleted team to be gone, got %v", err)
	}
}
