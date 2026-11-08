package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Insert(ctx context.Context, event AuditEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (
			id, actor_user_id, server_id, event_type, target_summary,
			outcome, safe_error_code, correlation_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, nullableString(event.ActorUserID), nullableString(event.ServerID),
		event.EventType, event.TargetSummary, event.Outcome,
		nullableString(event.SafeErrorCode), event.CorrelationID,
		formatTime(event.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// ListRecentForUser returns recent events whose actor or affected server belongs
// to the user. This keeps audit history scoped without requiring RBAC roles.
func (r *AuditRepository) ListRecentForUser(ctx context.Context, userID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, actor_user_id, server_id, event_type, target_summary,
		       outcome, safe_error_code, correlation_id, created_at
		FROM audit_events
		WHERE actor_user_id = ?
		   OR server_id IN (SELECT id FROM servers WHERE owner_user_id = ?)
		ORDER BY created_at DESC LIMIT ?`, userID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events for user: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAuditEvents(rows, limit)
}

func (r *AuditRepository) ListRecent(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, actor_user_id, server_id, event_type, target_summary,
		       outcome, safe_error_code, correlation_id, created_at
		FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanAuditEvents(rows, limit)
}

func scanAuditEvents(rows *sql.Rows, limit int) ([]AuditEvent, error) {
	events := make([]AuditEvent, 0, limit)
	for rows.Next() {
		var (
			event     AuditEvent
			actor     sql.NullString
			server    sql.NullString
			safeCode  sql.NullString
			createdAt string
		)
		if err := rows.Scan(
			&event.ID, &actor, &server, &event.EventType, &event.TargetSummary,
			&event.Outcome, &safeCode, &event.CorrelationID, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if actor.Valid {
			value := actor.String
			event.ActorUserID = &value
		}
		if server.Valid {
			value := server.String
			event.ServerID = &value
		}
		if safeCode.Valid {
			value := safeCode.String
			event.SafeErrorCode = &value
		}
		event.CreatedAt = parseTime(createdAt)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return events, nil
}
