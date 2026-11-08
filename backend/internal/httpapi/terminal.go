package httpapi

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/security"
)

func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
		return
	}

	origin := r.Header.Get("Origin")
	if origin == "" || !security.IsTrustedOrigin(s.cfg.TrustedOrigins, origin) {
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "Origin is not allowed for terminal connections."))
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Origin is checked above against the configured trusted origins.
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		s.logger.Warn("terminal websocket upgrade failed", "error", err, "request_id", RequestID(r.Context()))
		return
	}
	defer func() { _ = conn.CloseNow() }()

	session := CurrentSession(r.Context())
	if session == nil {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
		return
	}
	token := s.auth.SessionToken(r)
	s.terminal.Serve(
		r.Context(), conn, chi.URLParam(r, "id"), user.ID, session.ID,
		func(ctx context.Context) error {
			_, _, err := s.auth.Validate(ctx, token)
			return err
		},
	)
}
