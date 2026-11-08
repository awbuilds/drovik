package users

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/testenv"
	"github.com/awbuilds/drovik/backend/internal/testssh"
)

func TestUserLifecycleAndLastAdminGuard(t *testing.T) {
	sshServer, err := testssh.Start(t.TempDir())
	if err != nil {
		t.Fatalf("testssh.Start: %v", err)
	}
	defer sshServer.Close()
	env, err := testenv.New(t.TempDir(), sshServer)
	if err != nil {
		t.Fatalf("testenv.New: %v", err)
	}
	defer func() { _ = env.Close() }()
	repository := storage.NewUserRepository(env.DB)
	service := NewService(
		repository,
		storage.NewSessionRepository(env.DB),
		env.Audit,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	ctx := context.Background()

	created, err := service.Create(ctx, env.User.ID, CreateInput{
		Username: "operator-one", Password: "correct horse battery staple", Role: "operator",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Role != "operator" {
		t.Fatalf("unexpected role: %s", created.Role)
	}
	if _, err := service.Create(ctx, env.User.ID, CreateInput{
		Username: "bad role", Password: "correct horse battery staple", Role: "root",
	}); err == nil {
		t.Fatal("expected invalid role to be rejected")
	}

	readOnly := "read_only"
	updated, err := service.Update(ctx, env.User.ID, created.ID, &readOnly, nil)
	if err != nil || updated.Role != "read_only" {
		t.Fatalf("Update: %v %+v", err, updated)
	}

	adminRole := "admin"
	if _, err := service.Update(ctx, env.User.ID, env.User.ID, &adminRole, nil); err != nil {
		t.Fatalf("self admin role update: %v", err)
	}
	// The environment has a second admin only if the update created one; the
	// seeded admin is still the last active admin and cannot be demoted.
	operatorRole := "operator"
	if _, err := service.Update(ctx, env.User.ID, env.User.ID, &operatorRole, nil); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
	if err := service.Delete(ctx, env.User.ID, env.User.ID); !errors.Is(err, ErrSelfDelete) {
		t.Fatalf("expected ErrSelfDelete, got %v", err)
	}
	if err := service.Delete(ctx, env.User.ID, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repository.GetByID(ctx, created.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected deleted user, got %v", err)
	}
}
