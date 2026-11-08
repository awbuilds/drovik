package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type createServerRequest struct {
	DisplayName string `json:"display_name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	AuthMethod  string `json:"auth_method"`
	Password    string `json:"password"`
	PrivateKey  string `json:"private_key"`
	Passphrase  string `json:"passphrase"`
	Tags        string `json:"tags"`
	Notes       string `json:"notes"`
}

type updateServerRequest struct {
	DisplayName *string `json:"display_name"`
	Host        *string `json:"host"`
	Port        *int    `json:"port"`
	Username    *string `json:"username"`
	AuthMethod  *string `json:"auth_method"`
	Password    *string `json:"password"`
	PrivateKey  *string `json:"private_key"`
	Passphrase  *string `json:"passphrase"`
	Tags        *string `json:"tags"`
	Notes       *string `json:"notes"`
}

type trustHostKeyRequest struct {
	Fingerprint string `json:"fingerprint"`
	KeyType     string `json:"key_type"`
	PublicKey   string `json:"public_key"`
	Replace     bool   `json:"replace"`
}

type serverResponse struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Username        string     `json:"username"`
	AuthMethod      string     `json:"auth_method"`
	Tags            string     `json:"tags"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastConnectedAt *time.Time `json:"last_connected_at,omitempty"`
	IsActive        bool       `json:"is_active"`
	AccessLevel     string     `json:"access_level,omitempty"`
}

type connectionTestResponse struct {
	Server       serverResponse         `json:"server"`
	HostKey      sshclient.HostKeyInfo  `json:"host_key"`
	Capabilities sshclient.Capabilities `json:"capabilities"`
}

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if user == nil {
		writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
		return
	}
	list, err := s.servers.List(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("list servers failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
		return
	}
	response := make([]serverResponse, 0, len(list))
	for _, server := range list {
		item := toServerResponse(server)
		if level, err := s.servers.AccessLevel(r.Context(), user.ID, server.ID); err == nil {
			item.AccessLevel = level
		}
		response = append(response, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": response})
}

func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request createServerRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	server, err := s.servers.Create(r.Context(), user.ID, servers.CreateInput{
		DisplayName: request.DisplayName,
		Host:        request.Host,
		Port:        request.Port,
		Username:    request.Username,
		AuthMethod:  request.AuthMethod,
		Password:    request.Password,
		PrivateKey:  request.PrivateKey,
		Passphrase:  request.Passphrase,
		Tags:        request.Tags,
		Notes:       request.Notes,
	})
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toServerResponse(*server))
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	server, err := s.servers.Get(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	item := toServerResponse(*server)
	if level, err := s.servers.AccessLevel(r.Context(), user.ID, server.ID); err == nil {
		item.AccessLevel = level
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request updateServerRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	server, err := s.servers.Update(r.Context(), user.ID, chi.URLParam(r, "id"), servers.UpdateInput{
		DisplayName: request.DisplayName,
		Host:        request.Host,
		Port:        request.Port,
		Username:    request.Username,
		AuthMethod:  request.AuthMethod,
		Password:    request.Password,
		PrivateKey:  request.PrivateKey,
		Passphrase:  request.Passphrase,
		Tags:        request.Tags,
		Notes:       request.Notes,
	})
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toServerResponse(*server))
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.servers.Delete(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		s.writeServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.servers.TestConnection(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	server, err := s.servers.Get(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	item := toServerResponse(*server)
	if level, err := s.servers.AccessLevel(r.Context(), user.ID, server.ID); err == nil {
		item.AccessLevel = level
	}
	writeJSON(w, http.StatusOK, connectionTestResponse{
		Server:       item,
		HostKey:      result.HostKey,
		Capabilities: result.Capabilities,
	})
}

func (s *Server) handleTrustHostKey(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request trustHostKeyRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if len(request.PublicKey) > 16*1024 {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Host key is too large."))
		return
	}
	if err := s.servers.ApproveHostKey(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		request.Fingerprint, request.KeyType, request.PublicKey, request.Replace,
	); err != nil {
		s.writeServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	capabilities, err := s.servers.Capabilities(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, capabilities)
}

func toServerResponse(server storage.Server) serverResponse {
	return serverResponse{
		ID:              server.ID,
		DisplayName:     server.DisplayName,
		Host:            server.Host,
		Port:            server.Port,
		Username:        server.Username,
		AuthMethod:      server.AuthMethod,
		Tags:            server.Tags,
		Notes:           server.Notes,
		CreatedAt:       server.CreatedAt,
		UpdatedAt:       server.UpdatedAt,
		LastConnectedAt: server.LastConnectedAt,
		IsActive:        server.IsActive,
	}
}

func (s *Server) writeServerError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *servers.ValidationError
	if errors.As(err, &validation) {
		writeError(w, r, WithDetails(
			NewAPIError(http.StatusUnprocessableEntity, CodeValidation, validation.Message),
			map[string]string{"field": validation.Field},
		))
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServerNotFound, "Server not found."))
		return
	}
	if errors.Is(err, servers.ErrUnauthorized) {
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "You do not have access to this server."))
		return
	}
	if errors.Is(err, servers.ErrHostKeyAlreadyApproved) {
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "A host key is already approved for this server."))
		return
	}

	var unknown *sshclient.HostKeyUnknownError
	if errors.As(err, &unknown) {
		writeError(w, r, WithDetails(
			NewAPIError(http.StatusConflict, CodeSSHHostKeyUnknown, "The SSH host key is not trusted yet. Verify the fingerprint and approve it."),
			unknown.Info,
		))
		return
	}
	var changed *sshclient.HostKeyChangedError
	if errors.As(err, &changed) {
		writeError(w, r, WithDetails(
			NewAPIError(http.StatusConflict, CodeSSHHostKeyChanged, "The SSH host key has changed. Verify the server before re-approving."),
			map[string]any{"previous": changed.Previous, "presented": changed.Presented},
		))
		return
	}
	if errors.Is(err, sshclient.ErrInvalidCredentials) || errors.Is(err, sshclient.ErrInvalidPrivateKey) {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSSHAuthFailed, "SSH authentication failed. Check the username and credentials."))
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeError(w, r, NewAPIError(http.StatusGatewayTimeout, CodeSSHTimeout, "The SSH operation timed out."))
		return
	}
	if strings.Contains(err.Error(), "ssh: handshake failed") || strings.Contains(err.Error(), "unable to authenticate") {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSSHAuthFailed, "SSH authentication failed. Check the username and credentials."))
		return
	}
	if errors.Is(err, security.ErrEgressDenied) {
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "The destination is not allowed by the egress policy."))
		return
	}

	s.logger.Error("server operation failed",
		"code", CodeSSHConnectionFailed,
		"request_id", RequestID(r.Context()),
	)
	writeError(w, r, NewAPIError(http.StatusBadGateway, CodeSSHConnectionFailed, "The SSH operation could not be completed."))
}
