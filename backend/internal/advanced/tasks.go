package advanced

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	maxTaskCommandBytes = 500
	taskMarkerPrefix    = "# drovik-task:"
)

var (
	scheduleFieldPattern = regexp.MustCompile(`^[0-9*/,-]{1,32}$`)
	taskIDPattern        = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

type ManagedTask struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
}

type TaskInput struct {
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
}

type TaskResult struct {
	Task ManagedTask `json:"task"`
}

func (s *Service) CreateTask(
	ctx context.Context,
	ownerID, serverID string,
	input TaskInput,
	confirm bool,
) (*TaskResult, error) {
	if !confirm {
		return nil, fmt.Errorf("%w: task confirmation is required", ErrInvalidTarget)
	}
	task, err := validateTask(input)
	if err != nil {
		return nil, err
	}
	task.ID, err = newTaskID()
	if err != nil {
		return nil, err
	}

	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()

	current, err := s.readCrontab(ctx, client)
	if err != nil {
		return nil, err
	}
	updated := current
	if strings.TrimSpace(updated) != "" && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	updated += taskMarkerPrefix + task.ID + "\n" + task.Schedule + " " + task.Command + "\n"
	if err := s.installCrontab(ctx, client, updated); err != nil {
		return nil, err
	}
	s.record(ctx, ownerID, serverID, "task.created", task.Schedule+" "+task.Command, "success", "")
	return &TaskResult{Task: task}, nil
}

func (s *Service) DeleteTask(
	ctx context.Context,
	ownerID, serverID, taskID string,
	confirm bool,
) error {
	if !confirm {
		return fmt.Errorf("%w: task confirmation is required", ErrInvalidTarget)
	}
	if !taskIDPattern.MatchString(taskID) {
		return fmt.Errorf("%w: invalid task identifier", ErrInvalidTarget)
	}

	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	current, err := s.readCrontab(ctx, client)
	if err != nil {
		return err
	}
	lines := strings.Split(current, "\n")
	kept := make([]string, 0, len(lines))
	removed := false
	for index := 0; index < len(lines); index++ {
		if lines[index] == taskMarkerPrefix+taskID {
			// Remove the marker and its cron line only when the next line
			// exists and is not another marker.
			if index+1 >= len(lines) || strings.HasPrefix(lines[index+1], taskMarkerPrefix) {
				return fmt.Errorf("%w: the managed task block is malformed", ErrInvalidTarget)
			}
			removed = true
			index++
			continue
		}
		kept = append(kept, lines[index])
	}
	if !removed {
		return fmt.Errorf("%w: task not found", ErrInvalidTarget)
	}
	updated := strings.Join(kept, "\n")
	if err := s.installCrontab(ctx, client, updated); err != nil {
		return err
	}
	s.record(ctx, ownerID, serverID, "task.deleted", taskID, "success", "")
	return nil
}

func (s *Service) readCrontab(ctx context.Context, client *ssh.Client) (string, error) {
	output, stderr, err := s.run(ctx, client, "crontab -l")
	if err != nil {
		message := strings.TrimSpace(stderr)
		// Only the explicit no-crontab diagnostic means the account has no
		// entries. A silent failure (timeout, channel error or an unexpected
		// implementation) must fail closed: overwriting an unknown crontab
		// would delete existing jobs.
		if strings.Contains(strings.ToLower(message), "no crontab for") {
			return "", nil
		}
		if message == "" {
			return "", fmt.Errorf("%w: crontab read failed without a diagnostic", ErrUnavailable)
		}
		return "", classify(stderr, err)
	}
	if strings.Contains(output, "[output truncated]") {
		return "", fmt.Errorf("%w: the crontab is too large to edit safely", ErrInvalidTarget)
	}
	return output, nil
}

func (s *Service) installCrontab(ctx context.Context, client *ssh.Client, content string) error {
	temporary := "/tmp/drovik-crontab-" + randomToken()
	sshClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("%w: SFTP is unavailable for crontab staging", ErrUnavailable)
	}
	defer func() { _ = sshClient.Close() }()

	file, err := sshClient.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("%w: create crontab staging file", ErrActionFailed)
	}
	if err := sshClient.Chmod(temporary, 0o600); err != nil {
		_ = file.Close()
		_ = sshClient.Remove(temporary)
		return fmt.Errorf("%w: secure crontab staging file", ErrActionFailed)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		_ = file.Close()
		_ = sshClient.Remove(temporary)
		return fmt.Errorf("%w: write crontab staging file", ErrActionFailed)
	}
	if err := file.Close(); err != nil {
		_ = sshClient.Remove(temporary)
		return fmt.Errorf("%w: close crontab staging file", ErrActionFailed)
	}
	_, stderr, err := s.run(ctx, client, "crontab "+temporary)
	_ = sshClient.Remove(temporary)
	if err != nil {
		return classify(stderr, err)
	}
	return nil
}

func validateTask(input TaskInput) (ManagedTask, error) {
	fields := strings.Fields(input.Schedule)
	if len(fields) != 5 {
		return ManagedTask{}, fmt.Errorf("%w: schedule must contain five cron fields", ErrInvalidTarget)
	}
	for _, field := range fields {
		if !scheduleFieldPattern.MatchString(field) {
			return ManagedTask{}, fmt.Errorf("%w: invalid cron field", ErrInvalidTarget)
		}
	}
	command := strings.TrimSpace(input.Command)
	if command == "" || len(command) > maxTaskCommandBytes {
		return ManagedTask{}, fmt.Errorf("%w: command must be 1 to %d bytes", ErrInvalidTarget, maxTaskCommandBytes)
	}
	if strings.ContainsAny(command, "\n\r\x00%") {
		return ManagedTask{}, fmt.Errorf("%w: command contains an unsupported character", ErrInvalidTarget)
	}
	return ManagedTask{Schedule: strings.Join(fields, " "), Command: command}, nil
}

func newTaskID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate task identifier: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func randomToken() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

// managedTasks parses Drovik-owned blocks from the user's crontab.
func managedTasks(crontab string) []ManagedTask {
	tasks := make([]ManagedTask, 0, 4)
	lines := strings.Split(crontab, "\n")
	for index := 0; index < len(lines); index++ {
		if !strings.HasPrefix(lines[index], taskMarkerPrefix) {
			continue
		}
		taskID := strings.TrimPrefix(lines[index], taskMarkerPrefix)
		if !taskIDPattern.MatchString(taskID) || index+1 >= len(lines) {
			continue
		}
		line := strings.TrimSpace(lines[index+1])
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		tasks = append(tasks, ManagedTask{
			ID:       taskID,
			Schedule: strings.Join(fields[:5], " "),
			Command:  strings.Join(fields[5:], " "),
		})
		index++
	}
	return tasks
}
