package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session Session) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (
			id, token_hash, user_id, created_at, last_seen_at, expires_at,
			absolute_expires_at, user_agent
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.TokenHash, session.UserID,
		formatTime(session.CreatedAt), formatTime(session.LastSeenAt),
		formatTime(session.ExpiresAt), formatTime(session.AbsoluteExpiresAt),
		session.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, token_hash, user_id, created_at, last_seen_at, expires_at,
		       absolute_expires_at, revoked_at, user_agent
		FROM sessions WHERE token_hash = ?`, tokenHash)
	return scanSession(row)
}

func (r *SessionRepository) Touch(ctx context.Context, id string, lastSeen, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id = ?`,
		formatTime(lastSeen), formatTime(expiresAt), id,
	)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (r *SessionRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, ?) WHERE id = ?`,
		formatTime(at), id,
	)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeAllForUser(ctx context.Context, userID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`,
		formatTime(at), userID,
	)
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE absolute_expires_at < ?
		   OR (revoked_at IS NOT NULL AND revoked_at < ?)`,
		formatTime(now), formatTime(now.Add(-7*24*time.Hour)),
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted sessions: %w", err)
	}
	return deleted, nil
}

func scanSession(row rowScanner) (*Session, error) {
	var (
		session    Session
		createdAt  string
		lastSeenAt string
		expiresAt  string
		absolute   string
		revokedAt  sql.NullString
	)
	if err := row.Scan(
		&session.ID, &session.TokenHash, &session.UserID,
		&createdAt, &lastSeenAt, &expiresAt, &absolute, &revokedAt,
		&session.UserAgent,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}
	session.CreatedAt = parseTime(createdAt)
	session.LastSeenAt = parseTime(lastSeenAt)
	session.ExpiresAt = parseTime(expiresAt)
	session.AbsoluteExpiresAt = parseTime(absolute)
	session.RevokedAt = nullableTime(revokedAt)
	return &session, nil
}
