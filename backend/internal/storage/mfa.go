package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type MFARepository struct {
	db *sql.DB
}

func NewMFARepository(db *sql.DB) *MFARepository {
	return &MFARepository{db: db}
}

func (r *MFARepository) Get(ctx context.Context, userID string) (string, bool, error) {
	var (
		secret  sql.NullString
		enabled int
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT mfa_secret, mfa_enabled FROM users WHERE id = ?`, userID,
	).Scan(&secret, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("get user MFA state: %w", err)
	}
	return secret.String, enabled == 1, nil
}

func (r *MFARepository) SetSecret(ctx context.Context, userID, secret string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET mfa_secret = ?, mfa_enabled = 0, updated_at = ? WHERE id = ?`,
		secret, formatTime(time.Now().UTC()), userID,
	)
	if err != nil {
		return fmt.Errorf("set user MFA secret: %w", err)
	}
	return requireAffected(result, "set user MFA secret")
}

func (r *MFARepository) SetEnabled(ctx context.Context, userID string, enabled bool) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET mfa_enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), formatTime(time.Now().UTC()), userID,
	)
	if err != nil {
		return fmt.Errorf("set user MFA enabled: %w", err)
	}
	return requireAffected(result, "set user MFA enabled")
}

func (r *MFARepository) Clear(ctx context.Context, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET mfa_secret = NULL, mfa_enabled = 0, updated_at = ? WHERE id = ?`,
		formatTime(time.Now().UTC()), userID,
	)
	if err != nil {
		return fmt.Errorf("clear user MFA: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete MFA recovery codes: %w", err)
	}
	return requireAffected(result, "clear user MFA")
}

func (r *MFARepository) CreateRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recovery code transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM mfa_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete old recovery codes: %w", err)
	}
	now := formatTime(time.Now().UTC())
	for _, hash := range hashes {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mfa_recovery_codes (id, user_id, code_hash, created_at)
			VALUES (?, ?, ?, ?)`, NewID(), userID, hash, now,
		); err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recovery codes: %w", err)
	}
	return nil
}

func (r *MFARepository) ConsumeRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mfa_recovery_codes SET used_at = ?
		WHERE user_id = ? AND code_hash = ? AND used_at IS NULL`,
		formatTime(time.Now().UTC()), userID, hash,
	)
	if err != nil {
		return false, fmt.Errorf("consume recovery code: %w", err)
	}
	consumed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count consumed recovery codes: %w", err)
	}
	return consumed == 1, nil
}

func requireAffected(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: count rows: %w", operation, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
