package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/monitoring"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

func (s *Server) handleServerOverview(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	snapshot, err := s.monitoring.Snapshot(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeMonitoringError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) writeMonitoringError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, servers.ErrUnauthorized):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "You do not have access to this server."))
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServerNotFound, "Server not found."))
	case errors.Is(err, monitoring.ErrMetricsUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeCapabilityUnavailable, "The host does not expose readable /proc metrics."))
	case errors.Is(err, security.ErrEgressDenied):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "The destination is not allowed by the egress policy."))
	case errors.Is(err, sshclient.ErrInvalidCredentials), errors.Is(err, sshclient.ErrInvalidPrivateKey):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSSHAuthFailed, "SSH authentication failed."))
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, r, NewAPIError(http.StatusGatewayTimeout, CodeSSHTimeout, "The metrics collection timed out."))
	default:
		var unknown *sshclient.HostKeyUnknownError
		if errors.As(err, &unknown) {
			writeError(w, r, WithDetails(
				NewAPIError(http.StatusConflict, CodeSSHHostKeyUnknown, "The SSH host key is not trusted yet."),
				unknown.Info,
			))
			return
		}
		var changed *sshclient.HostKeyChangedError
		if errors.As(err, &changed) {
			writeError(w, r, WithDetails(
				NewAPIError(http.StatusConflict, CodeSSHHostKeyChanged, "The SSH host key has changed."),
				map[string]any{"previous": changed.Previous, "presented": changed.Presented},
			))
			return
		}
		s.logger.Error("metrics collection failed", "code", CodeSSHConnectionFailed, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeSSHConnectionFailed, "Host metrics could not be collected."))
	}
}
