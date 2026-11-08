package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Snippet struct {
	ID          string
	OwnerUserID string
	Name        string
	Description string
	Command     string
	Variables   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SnippetRepository struct {
	db *sql.DB
}

func NewSnippetRepository(db *sql.DB) *SnippetRepository {
	return &SnippetRepository{db: db}
}

func (r *SnippetRepository) Create(ctx context.Context, snippet Snippet) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO snippets (id, owner_user_id, name, description, command, variables, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		snippet.ID, snippet.OwnerUserID, snippet.Name, snippet.Description, snippet.Command,
		snippet.Variables, formatTime(snippet.CreatedAt), formatTime(snippet.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create snippet: %w", err)
	}
	return nil
}

func (r *SnippetRepository) CreateMany(ctx context.Context, snippets []Snippet) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin snippet import: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, snippet := range snippets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO snippets (id, owner_user_id, name, description, command, variables, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			snippet.ID, snippet.OwnerUserID, snippet.Name, snippet.Description, snippet.Command,
			snippet.Variables, formatTime(snippet.CreatedAt), formatTime(snippet.UpdatedAt),
		); err != nil {
			return fmt.Errorf("import snippet: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit snippet import: %w", err)
	}
	return nil
}

func (r *SnippetRepository) Update(ctx context.Context, snippet Snippet) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE snippets SET name = ?, description = ?, command = ?, variables = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`,
		snippet.Name, snippet.Description, snippet.Command, snippet.Variables,
		formatTime(snippet.UpdatedAt), snippet.ID, snippet.OwnerUserID,
	)
	if err != nil {
		return fmt.Errorf("update snippet: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated snippets: %w", err)
	}
	if updated == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SnippetRepository) Delete(ctx context.Context, id, ownerID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM snippets WHERE id = ? AND owner_user_id = ?`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete snippet: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted snippets: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SnippetRepository) List(ctx context.Context, ownerID string) ([]Snippet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, name, description, command, variables, created_at, updated_at
		FROM snippets WHERE owner_user_id = ? ORDER BY name COLLATE NOCASE`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list snippets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	snippets := make([]Snippet, 0, 16)
	for rows.Next() {
		snippet, err := scanSnippet(rows)
		if err != nil {
			return nil, err
		}
		snippets = append(snippets, *snippet)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snippets: %w", err)
	}
	return snippets, nil
}

func (r *SnippetRepository) Get(ctx context.Context, id, ownerID string) (*Snippet, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, name, description, command, variables, created_at, updated_at
		FROM snippets WHERE id = ? AND owner_user_id = ?`, id, ownerID)
	return scanSnippet(row)
}

func scanSnippet(row rowScanner) (*Snippet, error) {
	var (
		snippet   Snippet
		createdAt string
		updatedAt string
	)
	if err := row.Scan(
		&snippet.ID, &snippet.OwnerUserID, &snippet.Name, &snippet.Description,
		&snippet.Command, &snippet.Variables, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan snippet: %w", err)
	}
	snippet.CreatedAt = parseTime(createdAt)
	snippet.UpdatedAt = parseTime(updatedAt)
	return &snippet, nil
}
