package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

const DefaultSessionCookieName = "drovik_session"

var (
	ErrInvalidSession = errors.New("invalid session")
	ErrInvalidCSRF    = errors.New("invalid csrf token")
)

type SessionManager struct {
	sessions *storage.SessionRepository
	users    *storage.UserRepository
	logger   *slog.Logger

	idleTTL      time.Duration
	absoluteTTL  time.Duration
	cookieSecure bool

	sessionCookieName string
	csrfKey           []byte

	now func() time.Time
}

func NewSessionManager(
	sessions *storage.SessionRepository,
	users *storage.UserRepository,
	idleTTL, absoluteTTL time.Duration,
	cookieSecure bool,
	csrfKey []byte,
	logger *slog.Logger,
) *SessionManager {
	return &SessionManager{
		sessions:          sessions,
		users:             users,
		logger:            logger,
		idleTTL:           idleTTL,
		absoluteTTL:       absoluteTTL,
		cookieSecure:      cookieSecure,
		sessionCookieName: DefaultSessionCookieName,
		csrfKey:           append([]byte(nil), csrfKey...),
		now:               time.Now,
	}
}

func (m *SessionManager) Create(ctx context.Context, userID, userAgent string) (*storage.Session, string, string, error) {
	token, err := randomToken()
	if err != nil {
		return nil, "", "", err
	}

	now := m.now().UTC()
	session := storage.Session{
		ID:                storage.NewID(),
		TokenHash:         TokenHash(token),
		UserID:            userID,
		CreatedAt:         now,
		LastSeenAt:        now,
		ExpiresAt:         now.Add(m.idleTTL),
		AbsoluteExpiresAt: now.Add(m.absoluteTTL),
		UserAgent:         truncate(userAgent, 512),
	}
	if err := m.sessions.Create(ctx, session); err != nil {
		return nil, "", "", err
	}
	return &session, token, m.CSRFToken(token), nil
}

// Validate resolves a raw token to its session and user, enforcing revocation,
// idle expiry and absolute expiry. It slides the idle window at most once a minute.
func (m *SessionManager) Validate(ctx context.Context, token string) (*storage.Session, *storage.User, error) {
	if token == "" {
		return nil, nil, ErrInvalidSession
	}
	session, err := m.sessions.GetByTokenHash(ctx, TokenHash(token))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil, ErrInvalidSession
		}
		return nil, nil, fmt.Errorf("load session: %w", err)
	}

	now := m.now().UTC()
	if session.RevokedAt != nil || now.After(session.AbsoluteExpiresAt) {
		_ = m.sessions.Revoke(ctx, session.ID, now)
		return nil, nil, ErrInvalidSession
	}
	if now.After(session.ExpiresAt) {
		_ = m.sessions.Revoke(ctx, session.ID, now)
		return nil, nil, ErrInvalidSession
	}

	user, err := m.users.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			_ = m.sessions.Revoke(ctx, session.ID, now)
			return nil, nil, ErrInvalidSession
		}
		return nil, nil, fmt.Errorf("load session user: %w", err)
	}
	if !user.IsActive {
		_ = m.sessions.Revoke(ctx, session.ID, now)
		return nil, nil, ErrInvalidSession
	}

	if now.Sub(session.LastSeenAt) >= time.Minute {
		expiresAt := now.Add(m.idleTTL)
		if expiresAt.After(session.AbsoluteExpiresAt) {
			expiresAt = session.AbsoluteExpiresAt
		}
		if err := m.sessions.Touch(ctx, session.ID, now, expiresAt); err != nil {
			// A failed touch must not invalidate a valid session, but it is a
			// real operational error and must be logged.
			m.logger.Warn("session touch failed", "error", err, "session_id", session.ID)
		} else {
			session.LastSeenAt = now
			session.ExpiresAt = expiresAt
		}
	}

	return session, user, nil
}

func (m *SessionManager) RevokeByToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	session, err := m.sessions.GetByTokenHash(ctx, TokenHash(token))
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("load session for revoke: %w", err)
	}
	return m.sessions.Revoke(ctx, session.ID, m.now().UTC())
}

func (m *SessionManager) RevokeAllForUser(ctx context.Context, userID string) error {
	return m.sessions.RevokeAllForUser(ctx, userID, m.now().UTC())
}

func (m *SessionManager) SetSessionCookie(w http.ResponseWriter, token string) {
	//nolint:gosec // Secure is configuration-driven; HttpOnly and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(m.absoluteTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	//nolint:gosec // Secure is configuration-driven; HttpOnly and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) SessionToken(r *http.Request) string {
	cookie, err := r.Cookie(m.sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// CSRFToken derives a session-bound synchronizer token. It is sent to the
// authenticated client in the response body and must be echoed in the
// X-CSRF-Token header for mutations. It is not stored in a readable cookie.
func (m *SessionManager) CSRFToken(sessionToken string) string {
	mac := hmac.New(sha256.New, m.csrfKey)
	mac.Write([]byte("drovik-csrf-v1:"))
	mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (m *SessionManager) ValidateCSRF(sessionToken, csrfToken string) bool {
	if sessionToken == "" || csrfToken == "" {
		return false
	}
	expected := m.CSRFToken(sessionToken)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(csrfToken)) == 1
}

func (m *SessionManager) SessionCookieName() string { return m.sessionCookieName }

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
