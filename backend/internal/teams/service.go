// Package teams implements administrator-managed groups of users and the
// servers shared with them.
package teams

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrNotFound       = errors.New("team not found")
	ErrNameTaken      = errors.New("team name is already in use")
	ErrInvalid        = errors.New("invalid team input")
	ErrUserNotFound   = errors.New("user not found")
	ErrServerNotFound = errors.New("server not found")
)

type Service struct {
	repo    *storage.TeamRepository
	users   *storage.UserRepository
	servers *storage.ServerRepository
	audit   *audit.Service
	logger  *slog.Logger
}

func NewService(
	repo *storage.TeamRepository,
	users *storage.UserRepository,
	servers *storage.ServerRepository,
	auditService *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{repo: repo, users: users, servers: servers, audit: auditService, logger: logger}
}

type Detail struct {
	Team    storage.TeamSummary
	Members []storage.TeamMember
	Servers []storage.TeamServer
}

func (s *Service) Create(ctx context.Context, actorID, name, description string) (*storage.Team, error) {
	name, description, err := validate(name, description)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	team := storage.Team{
		ID: storage.NewID(), Name: name, Description: description,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, team); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrNameTaken
		}
		return nil, err
	}
	s.record(ctx, actorID, "team.created", name)
	return &team, nil
}

func (s *Service) List(ctx context.Context) ([]storage.TeamSummary, error) {
	return s.repo.List(ctx)
}

func (s *Service) Detail(ctx context.Context, teamID string) (*Detail, error) {
	team, err := s.repo.Get(ctx, teamID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	servers, err := s.repo.ListServers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	return &Detail{
		Team: storage.TeamSummary{
			Team: *team, MemberCount: len(members), ServerCount: len(servers),
		},
		Members: members, Servers: servers,
	}, nil
}

func (s *Service) Update(ctx context.Context, actorID, teamID, name, description string) (*storage.Team, error) {
	name, description, err := validate(name, description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, teamID, name, description, time.Now().UTC()); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrNameTaken
		}
		return nil, err
	}
	team, err := s.repo.Get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, actorID, "team.updated", name)
	return team, nil
}

func (s *Service) Delete(ctx context.Context, actorID, teamID string) error {
	team, err := s.repo.Get(ctx, teamID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := s.repo.Delete(ctx, teamID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	s.record(ctx, actorID, "team.deleted", team.Name)
	return nil
}

func (s *Service) AddMember(ctx context.Context, actorID, teamID, userID string) error {
	team, err := s.team(ctx, teamID)
	if err != nil {
		return err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if err := s.repo.AddMember(ctx, teamID, userID, time.Now().UTC()); err != nil {
		return err
	}
	s.record(ctx, actorID, "team.member_added", team.Name+" | "+user.Username)
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID, teamID, userID string) error {
	team, err := s.team(ctx, teamID)
	if err != nil {
		return err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if err := s.repo.RemoveMember(ctx, teamID, userID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	target := userID
	if user != nil {
		target = user.Username
	}
	s.record(ctx, actorID, "team.member_removed", team.Name+" | "+target)
	return nil
}

func (s *Service) SetServer(ctx context.Context, actorID, teamID, serverID, accessLevel string) error {
	team, err := s.team(ctx, teamID)
	if err != nil {
		return err
	}
	if accessLevel != "read" && accessLevel != "write" {
		return fmt.Errorf("%w: access level must be read or write", ErrInvalid)
	}
	server, err := s.servers.GetByID(ctx, serverID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrServerNotFound
		}
		return err
	}
	if err := s.repo.SetServer(ctx, teamID, serverID, accessLevel, time.Now().UTC()); err != nil {
		return err
	}
	s.record(ctx, actorID, "team.server_assigned", team.Name+" | "+server.DisplayName+" | "+accessLevel)
	return nil
}

func (s *Service) RemoveServer(ctx context.Context, actorID, teamID, serverID string) error {
	team, err := s.team(ctx, teamID)
	if err != nil {
		return err
	}
	server, err := s.servers.GetByID(ctx, serverID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	if err := s.repo.RemoveServer(ctx, teamID, serverID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	target := serverID
	if server != nil {
		target = server.DisplayName
	}
	s.record(ctx, actorID, "team.server_removed", team.Name+" | "+target)
	return nil
}

func (s *Service) team(ctx context.Context, id string) (*storage.Team, error) {
	team, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return team, nil
}

func (s *Service) record(ctx context.Context, actorID, eventType, target string) {
	auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx
	s.audit.Record(auditCtx, storage.AuditEvent{
		ActorUserID: audit.StrPtr(actorID), EventType: eventType,
		TargetSummary: target, Outcome: "success",
	})
}

func validate(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if len(name) < 2 || len(name) > 64 || strings.ContainsAny(name, "\r\n\t") {
		return "", "", fmt.Errorf("%w: name must be 2 to 64 characters without control characters", ErrInvalid)
	}
	if len(description) > 500 {
		return "", "", fmt.Errorf("%w: description must be at most 500 characters", ErrInvalid)
	}
	return name, description, nil
}
