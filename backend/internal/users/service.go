// Package users provides administrator user-management operations.
package users

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrForbidden   = errors.New("administrator role required")
	ErrLastAdmin   = errors.New("at least one active administrator must remain")
	ErrSelfDelete  = errors.New("an administrator cannot delete their own account")
	ErrInvalidRole = errors.New("role must be admin, operator or read_only")
)

type Service struct {
	users    *storage.UserRepository
	sessions *storage.SessionRepository
	audit    *audit.Service
	logger   *slog.Logger
}

func NewService(
	users *storage.UserRepository,
	sessions *storage.SessionRepository,
	auditService *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{users: users, sessions: sessions, audit: auditService, logger: logger}
}

type CreateInput struct {
	Username string
	Password string
	Role     string
}

func (s *Service) List(ctx context.Context, actorID string) ([]storage.User, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	return s.users.List(ctx)
}

func (s *Service) Create(ctx context.Context, actorID string, input CreateInput) (*storage.User, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	username := strings.TrimSpace(input.Username)
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := auth.ValidatePassword(input.Password); err != nil {
		return nil, err
	}
	if !validRole(input.Role) {
		return nil, ErrInvalidRole
	}
	hash, err := auth.HashPassword(input.Password, auth.DefaultPasswordParams)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user := storage.User{
		ID: storage.NewID(), Username: username, PasswordHash: hash, Role: input.Role,
		IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "user.created", username)
	return &user, nil
}

func (s *Service) Update(ctx context.Context, actorID, targetID string, role *string, active *bool) (*storage.User, error) {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	newRole := target.Role
	if role != nil {
		newRole = *role
		if !validRole(newRole) {
			return nil, ErrInvalidRole
		}
	}
	newActive := target.IsActive
	if active != nil {
		newActive = *active
	}
	if target.Role == "admin" && target.IsActive && (newRole != "admin" || !newActive) {
		count, err := s.users.CountAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, ErrLastAdmin
		}
	}
	now := time.Now().UTC()
	if err := s.users.UpdateRoleAndStatus(ctx, targetID, newRole, newActive, now); err != nil {
		return nil, err
	}
	if !newActive {
		if err := s.sessions.RevokeAllForUser(ctx, targetID, now); err != nil {
			return nil, err
		}
	}
	target.Role = newRole
	target.IsActive = newActive
	target.UpdatedAt = now
	s.record(ctx, actorID, "user.updated", target.Username)
	return target, nil
}

func (s *Service) ResetPassword(ctx context.Context, actorID, targetID, password string) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password, auth.DefaultPasswordParams)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := s.users.UpdatePassword(ctx, targetID, hash, now); err != nil {
		return err
	}
	if err := s.sessions.RevokeAllForUser(ctx, targetID, now); err != nil {
		return err
	}
	s.record(ctx, actorID, "user.password_reset", target.Username)
	return nil
}

func (s *Service) Delete(ctx context.Context, actorID, targetID string) error {
	if err := s.requireAdmin(ctx, actorID); err != nil {
		return err
	}
	if actorID == targetID {
		return ErrSelfDelete
	}
	target, err := s.users.GetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.Role == "admin" && target.IsActive {
		count, err := s.users.CountAdmins(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastAdmin
		}
	}
	if err := s.users.Delete(ctx, targetID); err != nil {
		return err
	}
	s.record(ctx, actorID, "user.deleted", target.Username)
	return nil
}

func (s *Service) requireAdmin(ctx context.Context, actorID string) error {
	actor, err := s.users.GetByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor.Role != "admin" || !actor.IsActive {
		return ErrForbidden
	}
	return nil
}

func validRole(role string) bool {
	switch role {
	case "admin", "operator", "read_only":
		return true
	default:
		return false
	}
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return errors.New("username must be 3 to 64 characters")
	}
	for i, r := range username {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return errors.New("username contains an unsupported character")
		}
		if i == 0 && (r == '.' || r == '_' || r == '-') {
			return errors.New("username must start with a letter or number")
		}
	}
	return nil
}

func (s *Service) record(ctx context.Context, actorID, eventType, target string) {
	s.audit.Record(ctx, storage.AuditEvent{
		ActorUserID: &actorID, EventType: eventType, TargetSummary: target,
		Outcome: "success", CreatedAt: time.Now().UTC(),
	})
	if err := ctx.Err(); err != nil {
		s.logger.Debug("user audit context ended", "event", eventType, "error", err)
	}
}
