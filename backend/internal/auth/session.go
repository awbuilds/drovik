package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

const (
	DefaultSessionCookieName = "drovik_session"
	DefaultCSRFCookieName    = "drovik_csrf"
)

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
	csrfCookieName    string

	now func() time.Time
}

func NewSessionManager(
	sessions *storage.SessionRepository,
	users *storage.UserRepository,
	idleTTL, absoluteTTL time.Duration,
	cookieSecure bool,
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
		csrfCookieName:    DefaultCSRFCookieName,
		now:               time.Now,
	}
}

func (m *SessionManager) Create(ctx context.Context, userID, userAgent string) (*storage.Session, string, string, error) {
	token, err := randomToken()
	if err != nil {
		return nil, "", "", err
	}
	csrf, err := randomToken()
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
	return &session, token, csrf, nil
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

func (m *SessionManager) SetCookies(w http.ResponseWriter, token, csrf string) {
	maxAge := int(m.absoluteTTL.Seconds())
	//nolint:gosec // Secure is configuration-driven; HttpOnly and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	//nolint:gosec // CSRF cookie is intentionally readable by the client; Secure and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.csrfCookieName,
		Value:    csrf,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) ClearCookies(w http.ResponseWriter) {
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
	//nolint:gosec // CSRF cookie is intentionally readable by the client; Secure and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.csrfCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *SessionManager) SetCSRFCookie(w http.ResponseWriter, csrf string) {
	//nolint:gosec // CSRF cookie is intentionally readable by the client; Secure and SameSite are set
	http.SetCookie(w, &http.Cookie{
		Name:     m.csrfCookieName,
		Value:    csrf,
		Path:     "/",
		MaxAge:   int(m.absoluteTTL.Seconds()),
		HttpOnly: false,
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

func (m *SessionManager) CSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(m.csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (m *SessionManager) NewCSRFToken() (string, error) {
	return randomToken()
}

func (m *SessionManager) SessionCookieName() string { return m.sessionCookieName }
func (m *SessionManager) CSRFCookieName() string    { return m.csrfCookieName }

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
