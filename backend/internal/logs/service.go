// Package logs provides bounded systemd journal retrieval.
package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
)

var (
	ErrUnavailable  = errors.New("journal is unavailable")
	ErrPermission   = errors.New("journal access is not permitted")
	ErrInvalidUnit  = errors.New("unit name is invalid")
	ErrInvalidRange = errors.New("time range is invalid")
	ErrInvalidLines = errors.New("line count is invalid")
)

const (
	maxMessageBytes = 16 * 1024
	maxOutputBytes  = 1 << 20
)

var unitPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:\\-]{0,254}\.service$`)

var ranges = map[string]string{
	"":    "",
	"15m": "15 min ago",
	"1h":  "1 hour ago",
	"24h": "24 hours ago",
	"7d":  "7 days ago",
	"all": "",
}

type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Priority   int       `json:"priority"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Unit       string    `json:"unit,omitempty"`
	Identifier string    `json:"identifier,omitempty"`
}

type Result struct {
	CollectedAt time.Time `json:"collected_at"`
	Unit        string    `json:"unit,omitempty"`
	Since       string    `json:"since"`
	Lines       int       `json:"lines"`
	Entries     []Entry   `json:"entries"`
	Truncated   bool      `json:"truncated"`
}

type Service struct {
	cfg     *config.Config
	servers *servers.Service
	logger  *slog.Logger
}

func NewService(cfg *config.Config, serverService *servers.Service, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, servers: serverService, logger: logger}
}

func (s *Service) Read(
	ctx context.Context,
	ownerID, serverID, unit, since string,
	lines int,
) (*Result, error) {
	if unit != "" {
		if !unitPattern.MatchString(unit) {
			return nil, ErrInvalidUnit
		}
	}
	sinceValue, ok := ranges[since]
	if !ok {
		return nil, ErrInvalidRange
	}
	if lines <= 0 || lines > s.cfg.LogMaxLines {
		return nil, ErrInvalidLines
	}

	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	command := "journalctl --no-pager --output=json --lines=" + strconv.Itoa(lines)
	if sinceValue != "" {
		command += ` --since="` + sinceValue + `"`
	}
	if unit != "" {
		command += " --unit=" + unit
	}

	commandCtx, cancel := context.WithTimeout(ctx, s.cfg.OpsCommandTimeout)
	defer cancel()
	output, stderr, err := sshclient.RunCommand(commandCtx, client, command, s.cfg.LogMaxOutputBytes)
	if err != nil && output == "" {
		classified := classifyError(stderr)
		s.logger.Warn("journal read failed", "code", safeCode(classified), "stderr_len", len(stderr))
		return nil, classified
	}

	result := &Result{
		CollectedAt: time.Now().UTC(),
		Unit:        unit,
		Since:       since,
		Lines:       lines,
		Entries:     make([]Entry, 0, min(lines, 256)),
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, ok := parseEntry(line)
		if !ok {
			continue
		}
		result.Entries = append(result.Entries, entry)
	}
	result.Truncated = strings.Contains(output, "[output truncated]") || len(result.Entries) >= lines
	return result, nil
}

func parseEntry(line string) (Entry, bool) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return Entry{}, false
	}
	entry := Entry{Priority: 6, Severity: "info"}
	if value, ok := raw["__REALTIME_TIMESTAMP"].(string); ok {
		if micros, err := strconv.ParseInt(value, 10, 64); err == nil {
			entry.Timestamp = time.Unix(micros/1_000_000, (micros%1_000_000)*1000).UTC()
		}
	}
	switch value := raw["PRIORITY"].(type) {
	case string:
		if priority, err := strconv.Atoi(value); err == nil {
			entry.Priority = priority
		}
	case float64:
		entry.Priority = int(value)
	}
	entry.Severity = severity(entry.Priority)
	entry.Message = journalMessage(raw["MESSAGE"])
	entry.Unit = stringField(raw["_SYSTEMD_UNIT"])
	entry.Identifier = stringField(raw["SYSLOG_IDENTIFIER"])
	if entry.Message == "" {
		entry.Message = "(empty message)"
	}
	return entry, true
}

func journalMessage(value any) string {
	var message string
	switch typed := value.(type) {
	case string:
		message = typed
	case []any:
		buffer := make([]byte, 0, len(typed))
		for _, item := range typed {
			if number, ok := item.(float64); ok && number >= 0 && number <= 255 {
				buffer = append(buffer, byte(number))
			}
		}
		message = string(buffer)
	default:
		return ""
	}
	if len(message) > maxMessageBytes {
		message = message[:maxMessageBytes] + "…"
	}
	return strings.ToValidUTF8(message, "�")
}

func stringField(value any) string {
	text, _ := value.(string)
	if len(text) > 256 {
		return text[:256]
	}
	return text
}

func severity(priority int) string {
	switch priority {
	case 0:
		return "emergency"
	case 1:
		return "alert"
	case 2:
		return "critical"
	case 3:
		return "error"
	case 4:
		return "warning"
	case 5:
		return "notice"
	case 6:
		return "info"
	default:
		return "debug"
	}
}

func classifyError(stderr string) error {
	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "command not found"), strings.Contains(lower, "journalctl: not found"):
		return ErrUnavailable
	case strings.Contains(lower, "not seeing messages"), strings.Contains(lower, "access denied"),
		strings.Contains(lower, "permission"), strings.Contains(lower, "failed to open"),
		strings.Contains(lower, "not permitted"):
		return ErrPermission
	default:
		return fmt.Errorf("%w: %s", ErrUnavailable, safeStderr(stderr))
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
		return "JOURNAL_PERMISSION_DENIED"
	default:
		return "JOURNAL_UNAVAILABLE"
	}
}
