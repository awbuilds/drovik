// Package advanced provides read-first Docker, storage, network, package and
// scheduled-task inspection over SSH, plus user-defined command snippets.
package advanced

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrUnavailable   = errors.New("advanced host tools are unavailable")
	ErrPermission    = errors.New("the SSH account does not have permission")
	ErrInvalidTarget = errors.New("the requested target is invalid")
	ErrActionFailed  = errors.New("the advanced operation failed")
)

type Service struct {
	cfg      *config.Config
	servers  *servers.Service
	snippets *storage.SnippetRepository
	metrics  *storage.MetricRepository
	alerts   *storage.AlertRepository
	audit    *audit.Service
	logger   *slog.Logger
}

func NewService(
	cfg *config.Config,
	serverService *servers.Service,
	snippetRepository *storage.SnippetRepository,
	metricRepository *storage.MetricRepository,
	alertRepository *storage.AlertRepository,
	auditService *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg: cfg, servers: serverService, snippets: snippetRepository,
		metrics: metricRepository, alerts: alertRepository,
		audit: auditService, logger: logger,
	}
}

func (s *Service) run(ctx context.Context, client *ssh.Client, command string) (string, string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, s.cfg.AdvancedTimeout)
	defer cancel()
	return sshclient.RunCommand(commandCtx, client, command, s.cfg.AdvancedMaxOutputBytes)
}

func safeStderr(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 300 {
		stderr = stderr[:300] + "…"
	}
	if stderr == "" {
		return "no diagnostic output"
	}
	return stderr
}

func classify(stderr string, err error) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "permission denied"), strings.Contains(lower, "access denied"),
		strings.Contains(lower, "not authorized"), strings.Contains(lower, "permission"):
		return ErrPermission
	case strings.Contains(lower, "command not found"), strings.Contains(lower, "not found"),
		strings.Contains(lower, "executable file not found"):
		return fmt.Errorf("%w: %s", ErrUnavailable, safeStderr(stderr))
	case strings.Contains(lower, "no such container"), strings.Contains(lower, "no such object"):
		return ErrInvalidTarget
	default:
		return fmt.Errorf("%w: %s", ErrActionFailed, safeStderr(stderr))
	}
}

func (s *Service) record(ctx context.Context, actorID, serverID, eventType, target, outcome, code string) {
	var safeCode *string
	if code != "" {
		safeCode = &code
	}
	s.audit.Record(ctx, storage.AuditEvent{
		ActorUserID: &actorID, ServerID: &serverID, EventType: eventType,
		TargetSummary: target, Outcome: outcome, SafeErrorCode: safeCode,
		CreatedAt: time.Now().UTC(),
	})
}
