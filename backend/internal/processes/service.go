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
	PID       int    `json:"pid"`
	Signal    string `json:"signal"`
	Delivered bool   `json:"delivered"`
	Running   bool   `json:"running"`
	State     string `json:"state,omitempty"`
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
		s.record(ctx, ownerID, server.ID, "process.signal", "failure", safeCode(classified), fmt.Sprintf("pid %d", pid))
		return nil, classified
	}

	result := &SignalResult{PID: pid, Signal: signal, Delivered: true}
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-timer.C:
	}

	stateCtx, stateCancel := context.WithTimeout(ctx, 5*time.Second)
	defer stateCancel()
	state, _, stateErr := sshclient.RunCommand(stateCtx, client, fmt.Sprintf("ps -p %d -o stat=", pid), 4*1024)
	if stateErr != nil && state == "" {
		result.Running = false
	} else {
		state = strings.TrimSpace(state)
		result.Running = state != ""
		result.State = state
	}

	s.record(ctx, ownerID, server.ID, "process.signal", "success", "", fmt.Sprintf("pid %d %s", pid, flag))
	return result, nil
}

func (s *Service) verifyIdentity(ctx context.Context, client *ssh.Client, pid int, expectedStart time.Time) error {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, stderr, err := sshclient.RunCommand(checkCtx, client, fmt.Sprintf("LC_ALL=C ps -p %d -o etimes=", pid), 4*1024)
	if err != nil && output == "" {
		_ = stderr
		return ErrNotFound
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
