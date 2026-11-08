package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Server struct {
	ID                  string
	OwnerUserID         string
	DisplayName         string
	Host                string
	Port                int
	Username            string
	AuthMethod          string
	SecretProvider      string
	SecretRef           string
	EncryptedSecret     string
	SecretNonce         string
	SecretVersion       int
	EncryptedPassphrase string
	PassphraseNonce     string
	PassphraseVersion   int
	Tags                string
	Notes               string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastConnectedAt     *time.Time
	IsActive            bool
}

type KnownHost struct {
	ID          string
	ServerID    string
	Host        string
	Port        int
	KeyType     string
	Fingerprint string
	PublicKey   string
	ApprovedAt  time.Time
	ApprovedBy  *string
	RevokedAt   *time.Time
}

type CapabilityRecord struct {
	ServerID   string
	Data       string
	DetectedAt time.Time
}

type ServerRepository struct {
	db *sql.DB
}

func NewServerRepository(db *sql.DB) *ServerRepository {
	return &ServerRepository{db: db}
}

func (r *ServerRepository) Create(ctx context.Context, server Server) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO servers (
			id, owner_user_id, display_name, host, port, username, auth_method,
			secret_provider, secret_ref,
			encrypted_secret, secret_nonce, secret_version,
			encrypted_passphrase, passphrase_nonce, passphrase_version,
			tags, notes, created_at, updated_at, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		server.ID, server.OwnerUserID, server.DisplayName, server.Host, server.Port,
		server.Username, server.AuthMethod, server.SecretProvider, server.SecretRef,
		server.EncryptedSecret, server.SecretNonce,
		server.SecretVersion, nullableText(server.EncryptedPassphrase), nullableText(server.PassphraseNonce),
		nullableInt(server.PassphraseVersion), server.Tags, server.Notes,
		formatTime(server.CreatedAt), formatTime(server.UpdatedAt), boolToInt(server.IsActive),
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return nil
}

func (r *ServerRepository) ListByOwner(ctx context.Context, ownerID string) ([]Server, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, display_name, host, port, username, auth_method,
		       secret_provider, secret_ref,
		       encrypted_secret, secret_nonce, secret_version,
		       encrypted_passphrase, passphrase_nonce, passphrase_version,
		       tags, notes, created_at, updated_at, last_connected_at, is_active
		FROM servers WHERE owner_user_id = ? AND is_active = 1
		ORDER BY display_name COLLATE NOCASE`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	servers := make([]Server, 0)
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}
	return servers, nil
}

func (r *ServerRepository) ListAll(ctx context.Context) ([]Server, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, display_name, host, port, username, auth_method,
		       secret_provider, secret_ref,
		       encrypted_secret, secret_nonce, secret_version,
		       encrypted_passphrase, passphrase_nonce, passphrase_version,
		       tags, notes, created_at, updated_at, last_connected_at, is_active
		FROM servers WHERE is_active = 1
		ORDER BY display_name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list all servers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanServerRows(rows)
}

func (r *ServerRepository) ListAccessible(ctx context.Context, userID string) ([]Server, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, owner_user_id, display_name, host, port, username, auth_method,
		       secret_provider, secret_ref,
		       encrypted_secret, secret_nonce, secret_version,
		       encrypted_passphrase, passphrase_nonce, passphrase_version,
		       tags, notes, created_at, updated_at, last_connected_at, is_active
		FROM servers
		WHERE is_active = 1
		  AND (owner_user_id = ? OR id IN (SELECT server_id FROM server_assignments WHERE user_id = ?))
		ORDER BY display_name COLLATE NOCASE`, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("list accessible servers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanServerRows(rows)
}

func scanServerRows(rows *sql.Rows) ([]Server, error) {
	servers := make([]Server, 0, 16)
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, *server)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate servers: %w", err)
	}
	return servers, nil
}

func (r *ServerRepository) UpdateByID(ctx context.Context, server Server) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE servers SET
			display_name = ?, host = ?, port = ?, username = ?, auth_method = ?,
			secret_provider = ?, secret_ref = ?,
			encrypted_secret = ?, secret_nonce = ?, secret_version = ?,
			encrypted_passphrase = ?, passphrase_nonce = ?, passphrase_version = ?,
			tags = ?, notes = ?, updated_at = ?, is_active = ?
		WHERE id = ?`,
		server.DisplayName, server.Host, server.Port, server.Username, server.AuthMethod,
		server.SecretProvider, server.SecretRef,
		server.EncryptedSecret, server.SecretNonce, server.SecretVersion,
		nullableText(server.EncryptedPassphrase), nullableText(server.PassphraseNonce),
		nullableInt(server.PassphraseVersion), server.Tags, server.Notes,
		formatTime(server.UpdatedAt), boolToInt(server.IsActive), server.ID,
	)
	if err != nil {
		return fmt.Errorf("update server by id: %w", err)
	}
	return nil
}

func (r *ServerRepository) DeleteByID(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete server by id: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted servers: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServerRepository) GetByID(ctx context.Context, id string) (*Server, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, owner_user_id, display_name, host, port, username, auth_method,
		       secret_provider, secret_ref,
		       encrypted_secret, secret_nonce, secret_version,
		       encrypted_passphrase, passphrase_nonce, passphrase_version,
		       tags, notes, created_at, updated_at, last_connected_at, is_active
		FROM servers WHERE id = ?`, id)
	return scanServer(row)
}

func (r *ServerRepository) Update(ctx context.Context, server Server) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE servers SET
			display_name = ?, host = ?, port = ?, username = ?, auth_method = ?,
			secret_provider = ?, secret_ref = ?,
			encrypted_secret = ?, secret_nonce = ?, secret_version = ?,
			encrypted_passphrase = ?, passphrase_nonce = ?, passphrase_version = ?,
			tags = ?, notes = ?, updated_at = ?, is_active = ?
		WHERE id = ? AND owner_user_id = ?`,
		server.DisplayName, server.Host, server.Port, server.Username, server.AuthMethod,
		server.SecretProvider, server.SecretRef,
		server.EncryptedSecret, server.SecretNonce, server.SecretVersion,
		nullableText(server.EncryptedPassphrase), nullableText(server.PassphraseNonce),
		nullableInt(server.PassphraseVersion), server.Tags, server.Notes,
		formatTime(server.UpdatedAt), boolToInt(server.IsActive), server.ID, server.OwnerUserID,
	)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	return nil
}

func (r *ServerRepository) Delete(ctx context.Context, id, ownerID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ? AND owner_user_id = ?`, id, ownerID)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted servers: %w", err)
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ServerRepository) UpdateLastConnected(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE servers SET last_connected_at = ? WHERE id = ?`, formatTime(at), id)
	if err != nil {
		return fmt.Errorf("update server last connected: %w", err)
	}
	return nil
}

type KnownHostRepository struct {
	db *sql.DB
}

func NewKnownHostRepository(db *sql.DB) *KnownHostRepository {
	return &KnownHostRepository{db: db}
}

func (r *KnownHostRepository) GetActive(ctx context.Context, serverID string) (*KnownHost, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, server_id, host, port, key_type, fingerprint, public_key,
		       approved_at, approved_by, revoked_at
		FROM known_hosts WHERE server_id = ? AND revoked_at IS NULL`, serverID)
	var (
		record     KnownHost
		approvedAt string
		approvedBy sql.NullString
		revokedAt  sql.NullString
	)
	if err := row.Scan(
		&record.ID, &record.ServerID, &record.Host, &record.Port, &record.KeyType,
		&record.Fingerprint, &record.PublicKey, &approvedAt, &approvedBy, &revokedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan known host: %w", err)
	}
	record.ApprovedAt = parseTime(approvedAt)
	if approvedBy.Valid {
		value := approvedBy.String
		record.ApprovedBy = &value
	}
	record.RevokedAt = nullableTime(revokedAt)
	return &record, nil
}

func (r *KnownHostRepository) Approve(ctx context.Context, record KnownHost, replace bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin known-host transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var activeID string
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM known_hosts WHERE server_id = ? AND revoked_at IS NULL`, record.ServerID,
	).Scan(&activeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check active known host: %w", err)
	}
	if err == nil && !replace {
		return fmt.Errorf("a host key is already approved for this server")
	}
	if err == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE known_hosts SET revoked_at = ? WHERE id = ?`,
			formatTime(record.ApprovedAt), activeID); err != nil {
			return fmt.Errorf("revoke old host key: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO known_hosts (
			id, server_id, host, port, key_type, fingerprint, public_key,
			approved_at, approved_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID, record.ServerID, record.Host, record.Port, record.KeyType,
		record.Fingerprint, record.PublicKey, formatTime(record.ApprovedAt),
		nullableString(record.ApprovedBy),
	); err != nil {
		return fmt.Errorf("insert known host: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit known host: %w", err)
	}
	return nil
}

func (r *KnownHostRepository) RevokeActive(ctx context.Context, serverID string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE known_hosts SET revoked_at = ?
		WHERE server_id = ? AND revoked_at IS NULL`, formatTime(at), serverID)
	if err != nil {
		return fmt.Errorf("revoke active host key: %w", err)
	}
	return nil
}

type CapabilitiesRepository struct {
	db *sql.DB
}

func NewCapabilitiesRepository(db *sql.DB) *CapabilitiesRepository {
	return &CapabilitiesRepository{db: db}
}

func (r *CapabilitiesRepository) Upsert(ctx context.Context, serverID, data string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO server_capabilities (server_id, data, detected_at) VALUES (?, ?, ?)
		ON CONFLICT(server_id) DO UPDATE SET data = excluded.data, detected_at = excluded.detected_at`,
		serverID, data, formatTime(at),
	)
	if err != nil {
		return fmt.Errorf("upsert capabilities: %w", err)
	}
	return nil
}

func (r *CapabilitiesRepository) Delete(ctx context.Context, serverID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM server_capabilities WHERE server_id = ?`, serverID); err != nil {
		return fmt.Errorf("delete capabilities: %w", err)
	}
	return nil
}

func (r *CapabilitiesRepository) Get(ctx context.Context, serverID string) (*CapabilityRecord, error) {
	var record CapabilityRecord
	var detectedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT server_id, data, detected_at FROM server_capabilities WHERE server_id = ?`, serverID,
	).Scan(&record.ServerID, &record.Data, &detectedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get capabilities: %w", err)
	}
	record.DetectedAt = parseTime(detectedAt)
	return &record, nil
}

type serverRowScanner interface {
	Scan(dest ...any) error
}

func scanServer(row serverRowScanner) (*Server, error) {
	var (
		server            Server
		encryptedSecret   string
		secretNonce       string
		encryptedPass     sql.NullString
		passphraseNonce   sql.NullString
		passphraseVersion sql.NullInt64
		isActive          int
		createdAt         string
		updatedAt         string
		lastConnectedAt   sql.NullString
	)
	if err := row.Scan(
		&server.ID, &server.OwnerUserID, &server.DisplayName, &server.Host, &server.Port,
		&server.Username, &server.AuthMethod, &server.SecretProvider, &server.SecretRef,
		&encryptedSecret, &secretNonce, &server.SecretVersion,
		&encryptedPass, &passphraseNonce, &passphraseVersion, &server.Tags, &server.Notes,
		&createdAt, &updatedAt, &lastConnectedAt, &isActive,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan server: %w", err)
	}
	server.EncryptedSecret = encryptedSecret
	server.SecretNonce = secretNonce
	if encryptedPass.Valid {
		server.EncryptedPassphrase = encryptedPass.String
	}
	if passphraseNonce.Valid {
		server.PassphraseNonce = passphraseNonce.String
	}
	if passphraseVersion.Valid {
		server.PassphraseVersion = int(passphraseVersion.Int64)
	}
	server.IsActive = isActive == 1
	server.CreatedAt = parseTime(createdAt)
	server.UpdatedAt = parseTime(updatedAt)
	server.LastConnectedAt = nullableTime(lastConnectedAt)
	return &server, nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
