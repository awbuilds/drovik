package httpapi

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/oidc"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

const oidcStateCookie = "drovik_oidc_state"

type oidcLinkResponse struct {
	Linked bool `json:"linked"`
}

type oidcLinkStartRequest struct {
	Password string `json:"password"`
	Code     string `json:"code,omitempty"`
}

type oidcUnlinkRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleOIDCStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": s.oidc != nil && s.oidc.Enabled()})
}

func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "OIDC is not enabled."))
		return
	}
	if allowed, retryAfter := s.oidcLimiter.Allow("oidc-start|" + security.ClientIP(r)); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		writeError(w, r, NewAPIError(http.StatusTooManyRequests, CodeRateLimited, "Too many sign-in attempts. Try again later."))
		return
	}
	target, state, err := s.oidc.Begin("login", "", "")
	if err != nil {
		s.logger.Error("OIDC start failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "The identity provider could not be reached."))
		return
	}
	s.setOIDCStateCookie(w, state)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}

// handleOIDCLinkStart requires fresh proof of the local account before adding a
// new federated login. A stolen session alone must not become a passwordless
// login for that account.
func (s *Server) handleOIDCLinkStart(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "OIDC is not enabled."))
		return
	}
	user := CurrentUser(r.Context())
	if allowed, retryAfter := s.oidcLimiter.Allow("oidc-link|" + user.ID); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		writeError(w, r, NewAPIError(http.StatusTooManyRequests, CodeRateLimited, "Too many linking attempts. Try again later."))
		return
	}
	var request oidcLinkStartRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	valid, err := auth.VerifyPassword(request.Password, user.PasswordHash)
	if err != nil || !valid {
		s.record(r, user.ID, "auth.oidc_link_failed", user.Username, "denied", CodeAuthInvalidCredentials)
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthInvalidCredentials, "The password is invalid."))
		return
	}
	mfaSecret, mfaEnabled, err := s.mfa.Get(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("OIDC link MFA lookup failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	if mfaEnabled && !s.verifyMFACode(r.Context(), mfaSecret, user.ID, request.Code) {
		s.record(r, user.ID, "auth.oidc_link_failed", user.Username, "denied", CodeAuthMFAInvalid)
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthMFAInvalid, "The authenticator code is invalid."))
		return
	}
	session := CurrentSession(r.Context())
	if session == nil {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
		return
	}
	target, state, err := s.oidc.Begin("link", user.ID, session.ID)
	if err != nil {
		s.logger.Error("OIDC link start failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "The identity provider could not be reached."))
		return
	}
	s.setOIDCStateCookie(w, state)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{"url": target})
}

func (s *Server) handleOIDCLinkStatus(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "OIDC is not enabled."))
		return
	}
	user := CurrentUser(r.Context())
	linked, err := s.users.OIDCLinked(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("OIDC link status failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	writeJSON(w, http.StatusOK, oidcLinkResponse{Linked: linked})
}

func (s *Server) handleOIDCUnlink(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "OIDC is not enabled."))
		return
	}
	user := CurrentUser(r.Context())
	var request oidcUnlinkRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	valid, err := auth.VerifyPassword(request.Password, user.PasswordHash)
	if err != nil || !valid {
		s.record(r, user.ID, "auth.oidc_unlink_failed", user.Username, "denied", CodeAuthInvalidCredentials)
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthInvalidCredentials, "The password is invalid."))
		return
	}
	if err := s.users.ClearOIDC(r.Context(), user.ID); err != nil {
		s.logger.Error("OIDC unlink failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	s.record(r, user.ID, "auth.oidc_unlinked", user.Username, "success", "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		http.Redirect(w, r, "/login?oidc_error=disabled", http.StatusFound)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	cookie, err := r.Cookie(oidcStateCookie)
	if state == "" || code == "" || err != nil ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		s.clearOIDCStateCookie(w)
		http.Redirect(w, r, "/login?oidc_error=invalid", http.StatusFound)
		return
	}
	s.clearOIDCStateCookie(w)
	result, err := s.oidc.Complete(r.Context(), state, code)
	if err != nil {
		s.logger.Warn("OIDC callback failed", "error", err, "request_id", RequestID(r.Context()))
		target := "/login?oidc_error=failed"
		if errors.Is(err, oidc.ErrInvalidState) {
			target = "/login?oidc_error=invalid"
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	if result.Mode == "link" {
		s.completeOIDCLink(w, r, result)
		return
	}

	user, err := s.users.FindByOIDC(r.Context(), s.cfg.OIDCIssuer, result.Identity.Subject)
	if err != nil || user == nil || !user.IsActive {
		s.record(r, "", "auth.oidc_denied", result.Identity.Username, "denied", CodeAuthForbidden)
		http.Redirect(w, r, "/login?oidc_error=no_account", http.StatusFound)
		return
	}
	s.issueOIDCSession(w, r, user)
}

// completeOIDCLink binds the provider subject to the local user who started the
// flow. The callback must arrive on the same server-side session that started
// it, so a leaked callback URL cannot bind an identity from another browser.
func (s *Server) completeOIDCLink(w http.ResponseWriter, r *http.Request, result *oidc.Result) {
	session, _, err := s.auth.Validate(r.Context(), s.auth.SessionToken(r))
	if result.UserID == "" || result.SessionID == "" || err != nil || session == nil ||
		session.ID != result.SessionID || session.UserID != result.UserID {
		http.Redirect(w, r, "/settings?oidc_error=failed", http.StatusFound)
		return
	}
	user, err := s.users.GetByID(r.Context(), result.UserID)
	if err != nil || user == nil || !user.IsActive {
		http.Redirect(w, r, "/settings?oidc_error=failed", http.StatusFound)
		return
	}
	if err := s.users.BindOIDC(r.Context(), result.UserID, s.cfg.OIDCIssuer, result.Identity.Subject); err != nil {
		s.logger.Warn("OIDC identity bind failed", "error", err, "user_id", result.UserID)
		target := "/settings?oidc_error=failed"
		if errors.Is(err, storage.ErrNotFound) {
			target = "/settings?oidc_error=already_linked"
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	s.record(r, result.UserID, "auth.oidc_linked",
		s.cfg.OIDCIssuer+"|"+result.Identity.Subject, "success", "")
	http.Redirect(w, r, "/settings?oidc_linked=1", http.StatusFound)
}

func (s *Server) issueOIDCSession(w http.ResponseWriter, r *http.Request, user *storage.User) {
	_, token, _, err := s.auth.Create(r.Context(), user.ID, r.UserAgent())
	if err != nil {
		s.logger.Error("OIDC session creation failed", "error", err, "user_id", user.ID)
		http.Redirect(w, r, "/login?oidc_error=failed", http.StatusFound)
		return
	}
	if err := s.users.UpdateLastLogin(r.Context(), user.ID, time.Now().UTC()); err != nil {
		s.logger.Warn("OIDC last-login update failed", "error", err, "user_id", user.ID)
	}
	s.auth.SetSessionCookie(w, token)
	s.record(r, user.ID, "auth.oidc_login", user.Username, "success", "")
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) setOIDCStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure follows the deployment cookie policy

		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/api/v1/auth/oidc/callback",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearOIDCStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure follows the deployment cookie policy

		Name:     oidcStateCookie,
		Value:    "",
		Path:     "/api/v1/auth/oidc/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
