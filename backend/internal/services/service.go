// Package services provides bounded systemd unit discovery and enumerated actions.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrUnavailable  = errors.New("systemd is unavailable")
	ErrUnitNotFound = errors.New("unit was not found")
	ErrInvalidUnit  = errors.New("unit name is invalid")
	ErrPermission   = errors.New("service operation is not permitted")
	ErrActionFailed = errors.New("service action failed")
)

const (
	ActionStart   = "start"
	ActionStop    = "stop"
	ActionRestart = "restart"
	ActionReload  = "reload"
	ActionEnable  = "enable"
	ActionDisable = "disable"

	maxOutputBytes = 1 << 20
)

const listCommand = `systemctl list-units --type=service --all --no-legend --no-pager --plain
printf '@@drovik-unit-files@@\n'
systemctl list-unit-files --type=service --no-legend --no-pager`

var unitPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*\.service$`)

type Unit struct {
	Name        string `json:"name"`
	Load        string `json:"load"`
	Active      string `json:"active"`
	Sub         string `json:"sub"`
	FileState   string `json:"file_state"`
	Description string `json:"description"`
}

type ListResult struct {
	CollectedAt time.Time `json:"collected_at"`
	Units       []Unit    `json:"units"`
	Truncated   bool      `json:"truncated"`
}

type ActionResult struct {
	Unit          string `json:"unit"`
	Action        string `json:"action"`
	ActiveState   string `json:"active_state"`
	SubState      string `json:"sub_state"`
	UnitFileState string `json:"unit_file_state"`
	StateKnown    bool   `json:"state_known"`
	Message       string `json:"message,omitempty"`
}

type Service struct {
	cfg     *config.Config
	servers *servers.Service
	audit   *audit.Service
	logger  *slog.Logger
}

func NewService(
	cfg *config.Config,
	serverService *servers.Service,
	auditService *audit.Service,
	logger *slog.Logger,
) *Service {
	return &Service{cfg: cfg, servers: serverService, audit: auditService, logger: logger}
}

func (s *Service) List(ctx context.Context, ownerID, serverID string) (*ListResult, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	commandCtx, cancel := context.WithTimeout(ctx, s.cfg.OpsCommandTimeout)
	defer cancel()
	output, stderr, err := sshclient.RunCommand(commandCtx, client, listCommand, maxOutputBytes)
	if err != nil && output == "" {
		classified := classifyError(stderr)
		s.logger.Warn("service listing failed", "code", safeCode(classified), "stderr_len", len(stderr))
		return nil, classified
	}

	units, truncated := parseUnits(output, s.cfg.ServiceMaxUnits)
	if len(units) == 0 {
		return nil, fmt.Errorf("%w: systemctl returned no units", ErrUnavailable)
	}
	return &ListResult{CollectedAt: time.Now().UTC(), Units: units, Truncated: truncated}, nil
}

func parseUnits(output string, maxUnits int) ([]Unit, bool) {
	units := make(map[string]Unit, 64)
	section := "units"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "@@drovik-unit-files@@") {
			section = "files"
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		name := fields[0]
		if !unitPattern.MatchString(name) {
			continue
		}
		if section == "files" {
			unit, exists := units[name]
			if !exists {
				unit = Unit{Name: name, Load: "not-loaded", Active: "inactive", Sub: "dead"}
			}
			unit.FileState = fields[1]
			units[name] = unit
			continue
		}
		if len(fields) < 4 {
			continue
		}
		description := ""
		if len(fields) > 4 {
			description = strings.Join(fields[4:], " ")
		}
		units[name] = Unit{
			Name:        name,
			Load:        fields[1],
			Active:      fields[2],
			Sub:         fields[3],
			Description: description,
		}
	}

	result := make([]Unit, 0, len(units))
	for _, unit := range units {
		result = append(result, unit)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	if len(result) > maxUnits {
		return result[:maxUnits], true
	}
	return result, false
}

func (s *Service) Action(
	ctx context.Context,
	ownerID, serverID, unitName, action string,
) (*ActionResult, error) {
	if len(unitName) > 256 || !unitPattern.MatchString(unitName) {
		return nil, ErrInvalidUnit
	}
	if !validAction(action) {
		return nil, fmt.Errorf("%w: unsupported action", ErrActionFailed)
	}

	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	commandCtx, cancel := context.WithTimeout(ctx, s.cfg.OpsCommandTimeout)
	defer cancel()
	command := fmt.Sprintf("systemctl --no-ask-password --no-pager %s %s", action, unitName)
	_, stderr, err := sshclient.RunCommand(commandCtx, client, command, 64*1024)
	if err != nil {
		classified := classifyError(stderr)
		s.record(ctx, ownerID, serverID, "service.action", "failure", safeCode(classified), action+" "+unitName)
		return nil, classified
	}

	result := &ActionResult{Unit: unitName, Action: action}
	stateCtx, stateCancel := context.WithTimeout(ctx, 10*time.Second)
	defer stateCancel()
	stateOutput, stateStderr, stateErr := sshclient.RunCommand(
		stateCtx, client,
		"systemctl show --no-pager --property=ActiveState,SubState,UnitFileState "+unitName,
		8*1024,
	)
	if stateErr == nil {
		for _, line := range strings.Split(stateOutput, "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			if !ok {
				continue
			}
			switch key {
			case "ActiveState":
				result.ActiveState = value
			case "SubState":
				result.SubState = value
			case "UnitFileState":
				result.UnitFileState = value
			}
		}
		result.StateKnown = result.ActiveState != ""
	}
	if !result.StateKnown {
		result.Message = "The action completed, but the resulting unit state could not be read."
		_ = stateStderr
	}

	s.record(ctx, ownerID, serverID, "service.action", "success", "", action+" "+unitName)
	return result, nil
}

func validAction(action string) bool {
	switch action {
	case ActionStart, ActionStop, ActionRestart, ActionReload, ActionEnable, ActionDisable:
		return true
	default:
		return false
	}
}

func classifyError(stderr string) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "not been booted"), strings.Contains(lower, "failed to connect to bus"),
		strings.Contains(lower, "systemctl: not found"), strings.Contains(lower, "command not found"):
		return ErrUnavailable
	case strings.Contains(lower, "access denied"), strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "interactive authentication required"), strings.Contains(lower, "not permitted"):
		return ErrPermission
	case strings.Contains(lower, "not found"), strings.Contains(lower, "not loaded"),
		strings.Contains(lower, "no such file"):
		return ErrUnitNotFound
	default:
		return fmt.Errorf("%w: %s", ErrActionFailed, safeStderr(stderr))
	}
}

func safeStderr(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 512 {
		stderr = stderr[:512] + "…"
	}
	if stderr == "" {
		return "no diagnostic output"
	}
	return stderr
}

func safeCode(err error) string {
	switch {
	case errors.Is(err, ErrUnavailable):
		return "SERVICE_MANAGER_UNAVAILABLE"
	case errors.Is(err, ErrUnitNotFound):
		return "SERVICE_NOT_FOUND"
	case errors.Is(err, ErrPermission):
		return "SERVICE_PERMISSION_DENIED"
	default:
		return "SERVICE_ACTION_FAILED"
	}
}

func (s *Service) record(ctx context.Context, actorID, serverID, eventType, outcome, code, target string) {
	var safeCodeValue *string
	if code != "" {
		safeCodeValue = &code
	}
	s.audit.Record(ctx, storage.AuditEvent{
		ActorUserID:   &actorID,
		ServerID:      &serverID,
		EventType:     eventType,
		TargetSummary: target,
		Outcome:       outcome,
		SafeErrorCode: safeCodeValue,
		CreatedAt:     time.Now().UTC(),
	})
}
