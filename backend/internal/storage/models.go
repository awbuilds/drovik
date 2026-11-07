package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

type Session struct {
	ID                string
	TokenHash         string
	UserID            string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	UserAgent         string
}

type AuditEvent struct {
	ID            string
	ActorUserID   *string
	ServerID      *string
	EventType     string
	TargetSummary string
	Outcome       string
	SafeErrorCode *string
	CorrelationID string
	CreatedAt     time.Time
}

func NewID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand failure is unrecoverable for ID generation. This is a
		// programmer/runtime state, caught by the calling boundary.
		panic("storage: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(raw[:])
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func nullableTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	parsed := parseTime(value.String)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
