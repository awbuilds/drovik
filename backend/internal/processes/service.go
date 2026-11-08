// Package processes provides bounded process discovery and enumerated signals.
package processes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
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
	ErrUnavailable   = errors.New("process tools are unavailable")
	ErrNotFound      = errors.New("process no longer exists")
	ErrPermission    = errors.New("process operation is not permitted")
	ErrPIDReused     = errors.New("the process identity changed")
	ErrInvalidPID    = errors.New("process identifier is invalid")
	ErrInvalidSignal = errors.New("signal is not supported")
	ErrActionFailed  = errors.New("process action failed")
)

const (
	SignalTerm = "SIGTERM"
	SignalKill = "SIGKILL"

	maxCommandBytes   = 2048
	maxOutputBytes    = 1 << 20
	identityTolerance = 3 * time.Second
)

const listCommand = `LC_ALL=C ps -eo pid=,ppid=,user=,stat=,pcpu=,pmem=,rss=,etimes=,args=`

type Process struct {
	PID            int       `json:"pid"`
	PPID           int       `json:"ppid"`
	User           string    `json:"user"`
	State          string    `json:"state"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryPercent  float64   `json:"memory_percent"`
	RSSBytes       int64     `json:"rss_bytes"`
	ElapsedSeconds int64     `json:"elapsed_seconds"`
	StartedAt      time.Time `json:"started_at"`
	Command        string    `json:"command"`
}

type ListResult struct {
	CollectedAt time.Time `json:"collected_at"`
	Account     string    `json:"account"`
	Processes   []Process `json:"processes"`
	Truncated   bool      `json:"truncated"`
}

type SignalResult struct {
	PID            int    `json:"pid"`
	Signal         string `json:"signal"`
	Delivered      bool   `json:"delivered"`
	Running        bool   `json:"running"`
	State          string `json:"state,omitempty"`
	OutcomeUnknown bool   `json:"outcome_unknown"`
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
	client, server, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	collectCtx, cancel := context.WithTimeout(ctx, s.cfg.OpsCommandTimeout)
	defer cancel()
	output, stderr, err := sshclient.RunCommand(collectCtx, client, listCommand, maxOutputBytes)
	if err != nil {
		if output == "" {
			s.logger.Warn("process listing failed", "code", "PROCESS_LIST_FAILED", "stderr_len", len(stderr))
			return nil, fmt.Errorf("%w: %s", ErrUnavailable, safeStderr(stderr))
		}
	}

	collectedAt := time.Now().UTC()
	result := &ListResult{
		CollectedAt: collectedAt,
		Account:     server.Username,
		Processes:   make([]Process, 0, 128),
	}
	for _, line := range strings.Split(output, "\n") {
		process, ok := parseProcessLine(line, collectedAt)
		if !ok {
			continue
		}
		if len(result.Processes) >= s.cfg.ProcessMaxEntries {
			result.Truncated = true
			break
		}
		result.Processes = append(result.Processes, process)
	}
	if len(result.Processes) == 0 {
		return nil, fmt.Errorf("%w: ps returned no parsable processes", ErrUnavailable)
	}
	return result, nil
}

func parseProcessLine(line string, collectedAt time.Time) (Process, bool) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return Process{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return Process{}, false
	}
	ppid, _ := strconv.Atoi(fields[1])
	cpu, _ := strconv.ParseFloat(fields[4], 64)
	memory, _ := strconv.ParseFloat(fields[5], 64)
	rssKiB, _ := strconv.ParseInt(fields[6], 10, 64)
	elapsed, err := strconv.ParseInt(fields[7], 10, 64)
	if err != nil || elapsed < 0 {
		return Process{}, false
	}
	command := strings.Join(fields[8:], " ")
	if len(command) > maxCommandBytes {
		command = command[:maxCommandBytes] + "…"
	}
	return Process{
		PID:            pid,
		PPID:           ppid,
		User:           fields[2],
		State:          fields[3],
		CPUPercent:     max(cpu, 0),
		MemoryPercent:  max(memory, 0),
		RSSBytes:       max(rssKiB, 0) * 1024,
		ElapsedSeconds: elapsed,
		StartedAt:      collectedAt.Add(-time.Duration(elapsed) * time.Second),
		Command:        command,
	}, true
}

func (s *Service) Signal(
	ctx context.Context,
	ownerID, serverID string,
	pid int,
	signal string,
	expectedStart time.Time,
) (*SignalResult, error) {
	if pid <= 1 {
		return nil, ErrInvalidPID
	}
	var flag string
	switch signal {
	case SignalTerm:
		flag = "TERM"
	case SignalKill:
		flag = "KILL"
	default:
		return nil, ErrInvalidSignal
	}
	if expectedStart.IsZero() {
		return nil, fmt.Errorf("%w: expected_start is required", ErrInvalidPID)
	}

	client, server, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	if err := s.verifyIdentity(ctx, client, pid, expectedStart); err != nil {
		s.record(ctx, ownerID, server.ID, "process.signal", "failure", safeCode(err), fmt.Sprintf("pid %d", pid))
		return nil, err
	}

	actionCtx, cancel := context.WithTimeout(ctx, s.cfg.OpsCommandTimeout)
	defer cancel()
	command := fmt.Sprintf("kill -s %s %d", flag, pid)
	_, stderr, err := sshclient.RunCommand(actionCtx, client, command, 8*1024)
	if err != nil {
		classified := classifyError(stderr, err)
		s.recordAudit(ownerID, server.ID, "process.signal", "failure", safeCode(classified), fmt.Sprintf("pid %d", pid))
		return nil, classified
	}

	result := &SignalResult{PID: pid, Signal: signal, Delivered: true}
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		// The signal was already delivered. Persist the accountability record
		// with a detached context before returning to the canceled request.
		result.OutcomeUnknown = true
		s.recordAudit(ownerID, server.ID, "process.signal", "unknown", "OUTCOME_UNKNOWN", fmt.Sprintf("pid %d %s", pid, flag))
		return result, ctx.Err()
	case <-timer.C:
	}

	stateCtx, stateCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stateCancel()
	state, stateStderr, stateErr := sshclient.RunCommand(stateCtx, client, fmt.Sprintf("ps -p %d -o stat=", pid), 4*1024)
	state = strings.TrimSpace(state)
	switch {
	case stateErr == nil && state != "":
		result.Running = true
		result.State = state
	case stateErr == nil:
		// ps succeeded with no state: the process is gone.
		result.Running = false
	case state == "" && strings.TrimSpace(stateStderr) == "" && isRemoteExit(stateErr):
		// ps commonly exits non-zero with no diagnostic when the PID is gone.
		// Only an actual remote exit proves absence; a timeout, cancellation or
		// transport failure with empty buffers is unobservable.
		result.Running = false
	case state != "":
		// The command produced a state but also a diagnostic; report the state.
		result.Running = true
		result.State = state
	default:
		// An unobservable status is not evidence of absence. Report an unknown
		// outcome instead of claiming the process stopped.
		result.OutcomeUnknown = true
		result.Running = false
	}

	if result.OutcomeUnknown {
		s.recordAudit(ownerID, server.ID, "process.signal", "unknown", "OUTCOME_UNKNOWN", fmt.Sprintf("pid %d %s", pid, flag))
	} else {
		s.recordAudit(ownerID, server.ID, "process.signal", "success", "", fmt.Sprintf("pid %d %s", pid, flag))
	}
	return result, nil
}

func (s *Service) verifyIdentity(ctx context.Context, client *ssh.Client, pid int, expectedStart time.Time) error {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, stderr, err := sshclient.RunCommand(checkCtx, client, fmt.Sprintf("LC_ALL=C ps -p %d -o etimes=", pid), 4*1024)
	if err != nil && output == "" {
		// ps exits non-zero without a diagnostic when the PID does not exist.
		// A diagnostic means the observation itself failed (missing ps,
		// timeout, transport), which must not be reported as absence.
		if strings.TrimSpace(stderr) == "" {
			return ErrNotFound
		}
		return fmt.Errorf("%w: %s", ErrActionFailed, safeStderr(stderr))
	}
	elapsed, parseErr := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
	if parseErr != nil || elapsed < 0 {
		return ErrNotFound
	}
	currentStart := time.Now().UTC().Add(-time.Duration(elapsed) * time.Second)
	delta := currentStart.Sub(expectedStart)
	if delta < 0 {
		delta = -delta
	}
	s.logger.Debug("process identity check", "pid", pid, "elapsed_seconds", elapsed, "delta_ms", delta.Milliseconds())
	if delta > identityTolerance {
		return ErrPIDReused
	}
	return nil
}

// isRemoteExit reports whether the remote command ran and exited non-zero. A
// missing exit status, canceled context or transport failure is not evidence
// that a process is absent.
func isRemoteExit(err error) bool {
	var exit *ssh.ExitError
	return errors.As(err, &exit)
}

func classifyError(stderr string, err error) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "no such process"), strings.Contains(lower, "no process"):
		return ErrNotFound
	case strings.Contains(lower, "operation not permitted"), strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "not allowed"):
		return ErrPermission
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
	case errors.Is(err, ErrPermission):
		return "PROCESS_PERMISSION_DENIED"
	case errors.Is(err, ErrPIDReused):
		return "PROCESS_PID_REUSED"
	case errors.Is(err, ErrNotFound):
		return "PROCESS_NOT_FOUND"
	default:
		return "PROCESS_ACTION_FAILED"
	}
}

// recordAudit writes the accountability record with a detached, bounded context
// so a client disconnect after a destructive action cannot erase it.
func (s *Service) recordAudit(actorID, serverID, eventType, outcome, code, target string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.record(ctx, actorID, serverID, eventType, outcome, code, target)
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
