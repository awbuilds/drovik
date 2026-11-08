package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type userResponse struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type authResponse struct {
	User      userResponse `json:"user"`
	CSRFToken string       `json:"csrf_token"`
}

type setupStatusResponse struct {
	Required    bool `json:"required"`
	OIDCEnabled bool `json:"oidc_enabled"`
}

type setupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTP     string `json:"totp,omitempty"`
}

type versionResponse struct {
	Version            string `json:"version"`
	Commit             string `json:"commit"`
	BuildDate          string `json:"build_date"`
	GoVersion          string `json:"go_version"`
	SchemaVersion      int64  `json:"schema_version"`
	DestructiveHostOps bool   `json:"destructive_host_ops"`
	VaultEnabled       bool   `json:"vault_enabled"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := storage.Ping(r.Context(), s.db, s.cfg.ReadinessTimeout); err != nil {
		s.logger.Warn("readiness check failed", "error", err, "request_id", RequestID(r.Context()))
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	schemaVersion, err := storage.SchemaVersion(r.Context(), s.db)
	if err != nil {
		s.logger.Warn("schema version unavailable", "error", err, "request_id", RequestID(r.Context()))
		schemaVersion = 0
	}
	writeJSON(w, http.StatusOK, versionResponse{
		Version:       s.version.Version,
		Commit:        s.version.Commit,
		BuildDate:     s.version.BuildDate,
		GoVersion:     s.version.GoVersion,
		SchemaVersion: schemaVersion,
	})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.users.Count(r.Context())
	if err != nil {
		s.logger.Error("setup status failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{Required: count == 0, OIDCEnabled: s.cfg.OIDCEnabled})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	setupKey := "setup|" + security.ClientIP(r)
	if allowed, retry := s.setupLimiter.Allow(setupKey); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds()+0.5)))
		s.record(r, "", "setup.failed", "", "denied", CodeRateLimited)
		writeError(w, r, NewAPIError(http.StatusTooManyRequests, CodeRateLimited, "Too many setup attempts. Try again shortly."))
		return
	}

	var request setupRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if err := validateUsername(request.Username); err != nil {
		writeError(w, r, err)
		return
	}
	if err := auth.ValidatePassword(request.Password); err != nil {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, err.Error()))
		return
	}

	hash, err := auth.HashPassword(request.Password, auth.DefaultPasswordParams)
	if err != nil {
		s.logger.Error("password hashing failed during setup", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}

	now := time.Now().UTC()
	user := storage.User{
		ID:           storage.NewID(),
		Username:     request.Username,
		PasswordHash: hash,
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	created, err := s.users.CreateFirstAdmin(r.Context(), user)
	if err != nil {
		s.logger.Error("first-admin creation failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if !created {
		s.record(r, "", "setup.completed", request.Username, "denied", CodeSetupAlreadyComplete)
		writeError(w, r, NewAPIError(
			http.StatusForbidden,
			CodeSetupAlreadyComplete,
			"Setup has already been completed.",
		))
		return
	}

	if err := s.users.UpdateLastLogin(r.Context(), user.ID, now); err != nil {
		s.logger.Warn("setup last-login update failed", "error", err, "request_id", RequestID(r.Context()))
	}

	_, token, csrf, err := s.auth.Create(r.Context(), user.ID, r.UserAgent())
	if err != nil {
		s.logger.Error("session creation failed during setup", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	s.auth.SetSessionCookie(w, token)
	s.record(r, user.ID, "setup.completed", user.Username, "success", "")
	writeJSON(w, http.StatusCreated, authResponse{
		User:      toUserResponse(user, &now),
		CSRFToken: csrf,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	if request.Username == "" || request.Password == "" {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Username and password are required."))
		return
	}

	key := security.ClientIP(r)
	if allowed, retry := s.limiter.Allow(key); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds()+0.5)))
		s.record(r, "", "auth.login_failed", request.Username, "denied", CodeRateLimited)
		writeError(w, r, NewAPIError(http.StatusTooManyRequests, CodeRateLimited, "Too many login attempts. Try again shortly."))
		return
	}

	user, err := s.users.GetByUsername(r.Context(), request.Username)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			auth.DummyVerify(request.Password)
			s.record(r, "", "auth.login_failed", request.Username, "denied", CodeAuthInvalidCredentials)
			writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthInvalidCredentials, "Invalid username or password."))
			return
		}
		s.logger.Error("login lookup failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}

	valid, err := auth.VerifyPassword(request.Password, user.PasswordHash)
	if err != nil {
		s.logger.Error("stored password hash is invalid", "error", err, "user_id", user.ID, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if !valid || !user.IsActive {
		s.record(r, user.ID, "auth.login_failed", user.Username, "denied", CodeAuthInvalidCredentials)
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthInvalidCredentials, "Invalid username or password."))
		return
	}

	mfaSecret, mfaEnabled, err := s.mfa.Get(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("login MFA lookup failed", "error", err, "user_id", user.ID, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if mfaEnabled {
		if strings.TrimSpace(request.TOTP) == "" {
			s.record(r, user.ID, "auth.login_failed", user.Username, "denied", CodeAuthMFARequired)
			writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthMFARequired, "Enter your authenticator code."))
			return
		}
		if !s.verifyMFACode(r.Context(), mfaSecret, user.ID, request.TOTP) {
			s.record(r, user.ID, "auth.login_failed", user.Username, "denied", CodeAuthMFAInvalid)
			writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthMFAInvalid, "The authenticator code is invalid."))
			return
		}
	}

	_, token, csrf, err := s.auth.Create(r.Context(), user.ID, r.UserAgent())
	if err != nil {
		s.logger.Error("session creation failed at login", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}

	now := time.Now().UTC()
	if err := s.users.UpdateLastLogin(r.Context(), user.ID, now); err != nil {
		s.logger.Warn("last-login update failed", "error", err, "user_id", user.ID, "request_id", RequestID(r.Context()))
	}
	s.auth.SetSessionCookie(w, token)
	s.record(r, user.ID, "auth.login", user.Username, "success", "")
	writeJSON(w, http.StatusOK, authResponse{
		User:      toUserResponse(*user, &now),
		CSRFToken: csrf,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	token := s.auth.SessionToken(r)
	if err := s.auth.RevokeByToken(r.Context(), token); err != nil {
		s.logger.Error("logout revoke failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if session := CurrentSession(r.Context()); session != nil {
		s.terminal.CloseSession(session.ID)
	}
	s.auth.ClearSessionCookie(w)
	if user != nil {
		s.record(r, user.ID, "auth.logout", user.Username, "success", "")
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
		return
	}
	csrf := s.auth.CSRFToken(s.auth.SessionToken(r))
	if csrf == "" {
		s.logger.Error("csrf token derivation failed", "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	writeJSON(w, http.StatusOK, authResponse{
		User:      toUserResponse(*user, user.LastLoginAt),
		CSRFToken: csrf,
	})
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "Not found."))
		return
	}
	s.serveSPA(w, r)
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	if s.staticHandler == nil || len(s.indexHTML) == 0 {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "Not found."))
		return
	}

	clean := path.Clean("/" + r.URL.Path)
	if path.Ext(clean) != "" {
		setStaticCache(w, clean)
		s.staticHandler.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if _, err := w.Write(s.indexHTML); err != nil {
		s.logger.Warn("serve index failed", "error", err, "request_id", RequestID(r.Context()))
	}
}

func setStaticCache(w http.ResponseWriter, cleanPath string) {
	if strings.HasPrefix(cleanPath, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}

func decodeJSON(r *http.Request, destination any) *APIError {
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, _ := strings.Cut(contentType, ";")
		if strings.TrimSpace(strings.ToLower(mediaType)) != "application/json" {
			return NewAPIError(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json.")
		}
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			return NewAPIError(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "Request body is too large.")
		}
		return NewAPIError(http.StatusBadRequest, CodeValidation, "Request body is not valid JSON.")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return NewAPIError(http.StatusBadRequest, CodeValidation, "Request body must contain a single JSON object.")
	}
	return nil
}

func validateUsername(username string) *APIError {
	if len(username) < 3 || len(username) > 64 {
		return NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Username must be 3 to 64 characters.")
	}
	for i, r := range username {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '_', r == '-':
		default:
			return NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Username contains an unsupported character.")
		}
		if i == 0 && (r == '.' || r == '_' || r == '-') {
			return NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Username must start with a letter or number.")
		}
	}
	return nil
}

func (s *Server) record(r *http.Request, actorID, eventType, target, outcome, code string) {
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	var safeCode *string
	if code != "" {
		safeCode = &code
	}
	s.audit.Record(r.Context(), storage.AuditEvent{
		ActorUserID:   actor,
		EventType:     eventType,
		TargetSummary: truncateAudit(target),
		Outcome:       outcome,
		SafeErrorCode: safeCode,
		CorrelationID: RequestID(r.Context()),
		CreatedAt:     time.Now().UTC(),
	})
}

func toUserResponse(user storage.User, lastLogin *time.Time) userResponse {
	return userResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: lastLogin,
	}
}

func truncateAudit(value string) string {
	const max = 200
	if len(value) <= max {
		return value
	}
	return value[:max]
}
