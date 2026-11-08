package advanced

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

var (
	ErrSnippetNotFound = errors.New("snippet not found")
	placeholderPattern = regexp.MustCompile(`\{\{([A-Za-z0-9_]{1,32})\}\}`)
	variablePattern    = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)
)

type SnippetVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

type Snippet struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Variables   []SnippetVariable `json:"variables"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type SnippetInput struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Variables   []SnippetVariable `json:"variables"`
}

type SnippetRun struct {
	SnippetID string `json:"snippet_id"`
	Command   string `json:"command"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

func (s *Service) ListSnippets(ctx context.Context, ownerID string) ([]Snippet, error) {
	rows, err := s.snippets.List(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	result := make([]Snippet, 0, len(rows))
	for _, row := range rows {
		snippet, err := toSnippet(row)
		if err != nil {
			return nil, err
		}
		result = append(result, *snippet)
	}
	return result, nil
}

func (s *Service) GetSnippet(ctx context.Context, ownerID, id string) (*Snippet, error) {
	row, err := s.snippets.Get(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrSnippetNotFound
		}
		return nil, err
	}
	return toSnippet(*row)
}

func (s *Service) CreateSnippet(ctx context.Context, ownerID string, input SnippetInput) (*Snippet, error) {
	normalized, err := s.validateSnippet(input)
	if err != nil {
		return nil, err
	}
	variables, err := json.Marshal(normalized.Variables)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := storage.Snippet{
		ID: storage.NewID(), OwnerUserID: ownerID, Name: normalized.Name,
		Description: normalized.Description, Command: normalized.Command,
		Variables: string(variables), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.snippets.Create(ctx, row); err != nil {
		return nil, err
	}
	s.record(ctx, ownerID, "", "snippet.created", normalized.Name, "success", "")
	return toSnippet(row)
}

func (s *Service) UpdateSnippet(ctx context.Context, ownerID, id string, input SnippetInput) (*Snippet, error) {
	existing, err := s.snippets.Get(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrSnippetNotFound
		}
		return nil, err
	}
	normalized, err := s.validateSnippet(input)
	if err != nil {
		return nil, err
	}
	variables, err := json.Marshal(normalized.Variables)
	if err != nil {
		return nil, err
	}
	existing.Name = normalized.Name
	existing.Description = normalized.Description
	existing.Command = normalized.Command
	existing.Variables = string(variables)
	existing.UpdatedAt = time.Now().UTC()
	if err := s.snippets.Update(ctx, *existing); err != nil {
		return nil, err
	}
	s.record(ctx, ownerID, "", "snippet.updated", normalized.Name, "success", "")
	return toSnippet(*existing)
}

func (s *Service) DeleteSnippet(ctx context.Context, ownerID, id string) error {
	existing, err := s.snippets.Get(ctx, id, ownerID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrSnippetNotFound
		}
		return err
	}
	if err := s.snippets.Delete(ctx, id, ownerID); err != nil {
		return err
	}
	s.record(ctx, ownerID, "", "snippet.deleted", existing.Name, "success", "")
	return nil
}

func (s *Service) PreviewSnippet(ctx context.Context, ownerID, id string, values map[string]string) (string, error) {
	snippet, err := s.GetSnippet(ctx, ownerID, id)
	if err != nil {
		return "", err
	}
	return renderSnippet(snippet, values)
}

func (s *Service) RunSnippet(
	ctx context.Context,
	ownerID, serverID, id string,
	values map[string]string,
	confirm bool,
) (*SnippetRun, error) {
	if !confirm {
		return nil, fmt.Errorf("%w: execution confirmation is required", ErrInvalidTarget)
	}
	snippet, err := s.GetSnippet(ctx, ownerID, id)
	if err != nil {
		return nil, err
	}
	command, err := renderSnippet(snippet, values)
	if err != nil {
		return nil, err
	}
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, command)
	if err != nil && output == "" {
		classified := classify(stderr, err)
		s.record(ctx, ownerID, serverID, "snippet.run", snippet.Name, "failure", "SNIPPET_RUN_FAILED")
		return nil, classified
	}
	if stderr != "" {
		output += stderr
	}
	s.record(ctx, ownerID, serverID, "snippet.run", snippet.Name, "success", "")
	return &SnippetRun{
		SnippetID: snippet.ID, Command: command, Output: output,
		Truncated: strings.Contains(output, "[output truncated]"),
	}, nil
}

func renderSnippet(snippet *Snippet, values map[string]string) (string, error) {
	allowed := make(map[string]SnippetVariable, len(snippet.Variables))
	for _, variable := range snippet.Variables {
		allowed[variable.Name] = variable
	}
	for name := range values {
		if _, ok := allowed[name]; !ok {
			return "", fmt.Errorf("%w: unknown variable %q", ErrInvalidTarget, name)
		}
	}
	command := placeholderPattern.ReplaceAllStringFunc(snippet.Command, func(match string) string {
		name := placeholderPattern.FindStringSubmatch(match)[1]
		value, ok := values[name]
		if !ok {
			value = allowed[name].Default
		}
		return shellQuote(value)
	})
	if placeholderPattern.MatchString(command) {
		return "", fmt.Errorf("%w: unresolved snippet variable", ErrInvalidTarget)
	}
	if len(command) > 64*1024 {
		return "", fmt.Errorf("%w: substituted command is too large", ErrInvalidTarget)
	}
	return command, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (s *Service) validateSnippet(input SnippetInput) (SnippetInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len(input.Name) > 80 {
		return input, fmt.Errorf("%w: snippet name must be 1 to 80 characters", ErrInvalidTarget)
	}
	if len(input.Description) > 500 {
		return input, fmt.Errorf("%w: description is too long", ErrInvalidTarget)
	}
	if strings.TrimSpace(input.Command) == "" || len(input.Command) > s.cfg.SnippetMaxCommandBytes {
		return input, fmt.Errorf("%w: command must be non-empty and bounded", ErrInvalidTarget)
	}
	if strings.ContainsRune(input.Command, 0) {
		return input, fmt.Errorf("%w: command contains a NUL byte", ErrInvalidTarget)
	}
	if len(input.Variables) > 16 {
		return input, fmt.Errorf("%w: at most 16 variables are supported", ErrInvalidTarget)
	}
	seen := make(map[string]struct{}, len(input.Variables))
	for index := range input.Variables {
		variable := &input.Variables[index]
		variable.Name = strings.TrimSpace(variable.Name)
		if !variablePattern.MatchString(variable.Name) {
			return input, fmt.Errorf("%w: invalid variable name", ErrInvalidTarget)
		}
		if _, ok := seen[variable.Name]; ok {
			return input, fmt.Errorf("%w: duplicate variable %q", ErrInvalidTarget, variable.Name)
		}
		seen[variable.Name] = struct{}{}
		if len(variable.Description) > 200 || len(variable.Default) > 512 || strings.ContainsRune(variable.Default, 0) {
			return input, fmt.Errorf("%w: variable metadata is too large", ErrInvalidTarget)
		}
	}
	return input, nil
}

func toSnippet(row storage.Snippet) (*Snippet, error) {
	var variables []SnippetVariable
	if strings.TrimSpace(row.Variables) != "" {
		if err := json.Unmarshal([]byte(row.Variables), &variables); err != nil {
			return nil, fmt.Errorf("decode snippet variables: %w", err)
		}
	}
	if variables == nil {
		variables = []SnippetVariable{}
	}
	return &Snippet{
		ID: row.ID, Name: row.Name, Description: row.Description, Command: row.Command,
		Variables: variables, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
