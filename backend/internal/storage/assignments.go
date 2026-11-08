package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ServerAssignment struct {
	UserID      string
	ServerID    string
	AccessLevel string
	CreatedAt   time.Time
}

type AssignmentRepository struct {
	db *sql.DB
}

func NewAssignmentRepository(db *sql.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func (r *AssignmentRepository) Upsert(ctx context.Context, assignment ServerAssignment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO server_assignments (user_id, server_id, access_level, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, server_id) DO UPDATE SET access_level = excluded.access_level`,
		assignment.UserID, assignment.ServerID, assignment.AccessLevel, formatTime(assignment.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert server assignment: %w", err)
	}
	return nil
}

func (r *AssignmentRepository) Get(ctx context.Context, userID, serverID string) (*ServerAssignment, error) {
	var (
		assignment ServerAssignment
		createdAt  string
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, server_id, access_level, created_at
		FROM server_assignments WHERE user_id = ? AND server_id = ?`,
		userID, serverID,
	).Scan(&assignment.UserID, &assignment.ServerID, &assignment.AccessLevel, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get server assignment: %w", err)
	}
	assignment.CreatedAt = parseTime(createdAt)
	return &assignment, nil
}

func (r *AssignmentRepository) ListForServer(ctx context.Context, serverID string) ([]ServerAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, server_id, access_level, created_at
		FROM server_assignments WHERE server_id = ? ORDER BY user_id`, serverID)
	if err != nil {
		return nil, fmt.Errorf("list server assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	assignments := make([]ServerAssignment, 0, 8)
	for rows.Next() {
		var (
			assignment ServerAssignment
			createdAt  string
		)
		if err := rows.Scan(&assignment.UserID, &assignment.ServerID, &assignment.AccessLevel, &createdAt); err != nil {
			return nil, fmt.Errorf("scan server assignment: %w", err)
		}
		assignment.CreatedAt = parseTime(createdAt)
		assignments = append(assignments, assignment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate server assignments: %w", err)
	}
	return assignments, nil
}

func (r *AssignmentRepository) Delete(ctx context.Context, userID, serverID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM server_assignments WHERE user_id = ? AND server_id = ?`, userID, serverID)
	if err != nil {
		return fmt.Errorf("delete server assignment: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted assignments: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *AssignmentRepository) HasAccess(ctx context.Context, userID, serverID string) (bool, error) {
	var exists int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM server_assignments WHERE user_id = ? AND server_id = ?`,
		userID, serverID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check server assignment: %w", err)
	}
	return true, nil
}
