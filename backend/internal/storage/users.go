package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// CreateFirstAdmin creates the initial administrator only if no user exists.
// The transaction lock plus the count check prevents two concurrent setups.
func (r *UserRepository) CreateFirstAdmin(ctx context.Context, user User) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin first-admin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("count users in setup: %w", err)
	}
	if count > 0 {
		if err := tx.Rollback(); err != nil {
			return false, fmt.Errorf("rollback setup: %w", err)
		}
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, boolToInt(user.IsActive),
		formatTime(user.CreatedAt), formatTime(user.UpdatedAt),
	); err != nil {
		return false, fmt.Errorf("insert first admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit first admin: %w", err)
	}
	return true, nil
}

func (r *UserRepository) Create(ctx context.Context, user User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, boolToInt(user.IsActive),
		formatTime(user.CreatedAt), formatTime(user.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at, last_login_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at, last_login_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE id = ?`, formatTime(at), id)
	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*User, error) {
	var (
		user        User
		isActive    int
		createdAt   string
		updatedAt   string
		lastLoginAt sql.NullString
	)
	if err := row.Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Role, &isActive,
		&createdAt, &updatedAt, &lastLoginAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	user.IsActive = isActive == 1
	user.CreatedAt = parseTime(createdAt)
	user.UpdatedAt = parseTime(updatedAt)
	user.LastLoginAt = nullableTime(lastLoginAt)
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, password_hash, role, is_active, created_at, updated_at, last_login_at
		FROM users ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := make([]User, 0, 8)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r *UserRepository) UpdateRoleAndStatus(ctx context.Context, id, role string, active bool, at time.Time) error {
	// The guard is evaluated in the same statement as the write, so two admins
	// demoting or deactivating each other concurrently cannot leave the
	// installation without an active administrator.
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET role = ?, is_active = ?, updated_at = ?
		WHERE id = ?
		  AND (
		       NOT (role = 'admin' AND is_active = 1 AND (? <> 'admin' OR ? = 0))
		       OR EXISTS (SELECT 1 FROM users WHERE role = 'admin' AND is_active = 1 AND id <> ?)
		  )`,
		role, boolToInt(active), formatTime(at), id, role, boolToInt(active), id,
	)
	if err != nil {
		return fmt.Errorf("update user access: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated users: %w", err)
	}
	if updated == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id, hash string, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, formatTime(at), id,
	)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated users: %w", err)
	}
	if updated == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM users WHERE id = ?
		  AND (role <> 'admin' OR is_active = 0
		       OR EXISTS (SELECT 1 FROM users WHERE role = 'admin' AND is_active = 1 AND id <> ?))`, id, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted users: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepository) CountAdmins(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_active = 1`,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active admins: %w", err)
	}
	return count, nil
}
