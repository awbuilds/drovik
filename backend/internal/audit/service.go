package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

type Service struct {
	repo   *storage.AuditRepository
	logger *slog.Logger
}

func New(repo *storage.AuditRepository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

// Record writes an audit event. Audit failure must not crash the request, but it
// is logged as an operational error so the failure is visible.
func (s *Service) Record(ctx context.Context, event storage.AuditEvent) {
	if event.ID == "" {
		event.ID = storage.NewID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := s.repo.Insert(ctx, event); err != nil {
		s.logger.Error("audit event insert failed",
			"error", err,
			"event_type", event.EventType,
			"outcome", event.Outcome,
		)
	}
}

func (s *Service) ListRecent(ctx context.Context, limit int) ([]storage.AuditEvent, error) {
	return s.repo.ListRecent(ctx, limit)
}

func (s *Service) ListRecentForUser(ctx context.Context, userID string, limit int) ([]storage.AuditEvent, error) {
	return s.repo.ListRecentForUser(ctx, userID, limit)
}

func (s *Service) ListFilteredForUser(ctx context.Context, userID string, admin bool, filter storage.AuditFilter) ([]storage.AuditEvent, error) {
	return s.repo.ListFilteredForUser(ctx, userID, admin, filter)
}

func StrPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
