package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/advanced"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type dockerActionRequest struct {
	Action string `json:"action"`
}

type snippetRequest struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Command     string                     `json:"command"`
	Variables   []advanced.SnippetVariable `json:"variables"`
}

type snippetRunRequest struct {
	Values  map[string]string `json:"values"`
	Confirm bool              `json:"confirm"`
}

type alertRuleRequest struct {
	Metric     string  `json:"metric"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Enabled    bool    `json:"enabled"`
}

func (s *Server) handleDocker(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.Docker(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDockerAction(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request dockerActionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.advanced.DockerAction(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		chi.URLParam(r, "container"), request.Action,
	); err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDockerLogs(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	tail := 200
	if raw := r.URL.Query().Get("tail"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 10 || parsed > 1000 {
			writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Tail must be between 10 and 1000."))
			return
		}
		tail = parsed
	}
	result, err := s.advanced.DockerLogs(
		r.Context(), user.ID, chi.URLParam(r, "id"), chi.URLParam(r, "container"), tail,
	)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDockerInspect(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.DockerInspect(
		r.Context(), user.ID, chi.URLParam(r, "id"), chi.URLParam(r, "container"),
	)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStorageTools(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.Storage(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleNetworkTools(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.Network(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePackages(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.Packages(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.advanced.Schedules(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleServerHistory(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	rangeName := r.URL.Query().Get("range")
	if rangeName == "" {
		rangeName = "1h"
	}
	result, err := s.advanced.History(r.Context(), user.ID, chi.URLParam(r, "id"), rangeName)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListAlertRules(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	rules, err := s.advanced.ListAlertRules(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Server) handleSaveAlertRule(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request alertRuleRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	ruleID := chi.URLParam(r, "ruleID")
	if ruleID == "new" {
		ruleID = ""
	}
	rule, err := s.advanced.SaveAlertRule(
		r.Context(), user.ID, chi.URLParam(r, "id"), ruleID,
		advanced.AlertRuleInput{
			Metric: request.Metric, Comparator: request.Comparator,
			Threshold: request.Threshold, Enabled: request.Enabled,
		},
	)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) handleDeleteAlertRule(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.advanced.DeleteAlertRule(
		r.Context(), user.ID, chi.URLParam(r, "id"), chi.URLParam(r, "ruleID"),
	); err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListSnippets(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	list, err := s.advanced.ListSnippets(r.Context(), user.ID)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snippets": list})
}

func (s *Server) handleGetSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	snippet, err := s.advanced.GetSnippet(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snippet)
}

func (s *Server) handleCreateSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request snippetRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	snippet, err := s.advanced.CreateSnippet(r.Context(), user.ID, snippetInput(request))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, snippet)
}

func (s *Server) handleUpdateSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request snippetRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	snippet, err := s.advanced.UpdateSnippet(r.Context(), user.ID, chi.URLParam(r, "id"), snippetInput(request))
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snippet)
}

func (s *Server) handleDeleteSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.advanced.DeleteSnippet(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePreviewSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request snippetRunRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	command, err := s.advanced.PreviewSnippet(r.Context(), user.ID, chi.URLParam(r, "id"), request.Values)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"command": command})
}

func (s *Server) handleRunSnippet(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request snippetRunRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.advanced.RunSnippet(
		r.Context(), user.ID, chi.URLParam(r, "id"), chi.URLParam(r, "snippet"),
		request.Values, request.Confirm,
	)
	if err != nil {
		s.writeAdvancedError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func snippetInput(request snippetRequest) advanced.SnippetInput {
	return advanced.SnippetInput{
		Name: request.Name, Description: request.Description,
		Command: request.Command, Variables: request.Variables,
	}
}

func (s *Server) writeAdvancedError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, advanced.ErrSnippetNotFound), errors.Is(err, storage.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "The requested item was not found."))
	case errors.Is(err, advanced.ErrPermission):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "The SSH account does not have permission for this operation."))
	case errors.Is(err, advanced.ErrInvalidTarget):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The advanced operation request is invalid."))
	case errors.Is(err, advanced.ErrUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeCapabilityUnavailable, "The required host tool is unavailable."))
	case errors.Is(err, advanced.ErrActionFailed):
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeSSHConnectionFailed, "The advanced operation could not be completed."))
	default:
		s.writeServerError(w, r, err)
	}
}
