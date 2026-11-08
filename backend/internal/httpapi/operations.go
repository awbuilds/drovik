package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/logs"
	"github.com/awbuilds/drovik/backend/internal/processes"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	systemservices "github.com/awbuilds/drovik/backend/internal/services"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type signalProcessRequest struct {
	Signal        string    `json:"signal"`
	ExpectedStart time.Time `json:"expected_started_at"`
}

type serviceActionRequest struct {
	Action string `json:"action"`
}

func (s *Server) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.processes.List(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeOperationsError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSignalProcess(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
	if err != nil || pid <= 0 {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The process identifier is invalid."))
		return
	}
	var request signalProcessRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.processes.Signal(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		pid, request.Signal, request.ExpectedStart,
	)
	if err != nil {
		s.writeOperationsError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.services.List(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeOperationsError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request serviceActionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.services.Action(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		chi.URLParam(r, "unit"), request.Action,
	)
	if err != nil {
		s.writeOperationsError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleReadLogs(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	lines := 200
	if raw := r.URL.Query().Get("lines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > s.cfg.LogMaxLines {
			writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The line count is invalid."))
			return
		}
		lines = parsed
	}
	result, err := s.logs.Read(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		r.URL.Query().Get("unit"), r.URL.Query().Get("since"), lines,
	)
	if err != nil {
		s.writeOperationsError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeOperationsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, processes.ErrPermission):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeProcessPermission, "The SSH account does not have permission to signal this process."))
	case errors.Is(err, processes.ErrPIDReused):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeProcessPIDReused, "The process identity changed. Refresh the list before signaling."))
	case errors.Is(err, processes.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeProcessNotFound, "The process no longer exists."))
	case errors.Is(err, processes.ErrInvalidPID), errors.Is(err, processes.ErrInvalidSignal):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The process action is invalid."))
	case errors.Is(err, processes.ErrUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeProcessUnavailable, "Process information is unavailable on this host."))
	case errors.Is(err, processes.ErrActionFailed):
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeProcessActionFailed, "The process action could not be completed."))

	case errors.Is(err, systemservices.ErrPermission):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeServicePermission, "The SSH account does not have permission for this service action."))
	case errors.Is(err, systemservices.ErrUnitNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServiceNotFound, "The systemd unit was not found."))
	case errors.Is(err, systemservices.ErrInvalidUnit):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The unit name is invalid."))
	case errors.Is(err, systemservices.ErrUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSystemdUnavailable, "systemd is unavailable on this host."))
	case errors.Is(err, systemservices.ErrActionFailed):
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeServiceActionFailed, "The service action could not be completed."))

	case errors.Is(err, logs.ErrPermission):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeJournalPermission, "The SSH account cannot read the journal."))
	case errors.Is(err, logs.ErrInvalidUnit), errors.Is(err, logs.ErrInvalidRange), errors.Is(err, logs.ErrInvalidLines):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The log query is invalid."))
	case errors.Is(err, logs.ErrUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeCapabilityUnavailable, "The journal is unavailable on this host."))

	case errors.Is(err, servers.ErrUnauthorized):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "You do not have access to this server."))
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServerNotFound, "Server not found."))
	case errors.Is(err, security.ErrEgressDenied):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "The destination is not allowed by the egress policy."))
	case errors.Is(err, sshclient.ErrInvalidCredentials), errors.Is(err, sshclient.ErrInvalidPrivateKey):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSSHAuthFailed, "SSH authentication failed."))
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, r, NewAPIError(http.StatusGatewayTimeout, CodeSSHTimeout, "The remote operation timed out."))
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
		s.logger.Error("remote operation failed", "code", CodeSSHConnectionFailed, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeSSHConnectionFailed, "The remote operation could not be completed."))
	}
}
