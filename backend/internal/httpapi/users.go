package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/storage"
	useradmin "github.com/awbuilds/drovik/backend/internal/users"
)

type userAdminResponse struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

type assignmentResponse struct {
	UserID      string    `json:"user_id"`
	AccessLevel string    `json:"access_level"`
	CreatedAt   time.Time `json:"created_at"`
}

type setAssignmentRequest struct {
	AccessLevel string `json:"access_level"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	list, err := s.userAdmin.List(r.Context(), user.ID)
	if err != nil {
		s.writeUserAdminError(w, r, err)
		return
	}
	response := make([]userAdminResponse, 0, len(list))
	for _, item := range list {
		response = append(response, toUserAdminResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": response})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	var request createUserRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	user, err := s.userAdmin.Create(r.Context(), actor.ID, useradmin.CreateInput{
		Username: request.Username, Password: request.Password, Role: request.Role,
	})
	if err != nil {
		s.writeUserAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserAdminResponse(*user))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	var request updateUserRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	user, err := s.userAdmin.Update(r.Context(), actor.ID, chi.URLParam(r, "id"), request.Role, request.IsActive)
	if err != nil {
		s.writeUserAdminError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserAdminResponse(*user))
}

func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	var request resetPasswordRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.userAdmin.ResetPassword(r.Context(), actor.ID, chi.URLParam(r, "id"), request.Password); err != nil {
		s.writeUserAdminError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	if err := s.userAdmin.Delete(r.Context(), actor.ID, chi.URLParam(r, "id")); err != nil {
		s.writeUserAdminError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListAssignments(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	list, err := s.servers.ListAssignments(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		s.writeServerError(w, r, err)
		return
	}
	response := make([]assignmentResponse, 0, len(list))
	for _, item := range list {
		response = append(response, assignmentResponse{
			UserID: item.UserID, AccessLevel: item.AccessLevel, CreatedAt: item.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": response})
}

func (s *Server) handleSetAssignment(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	var request setAssignmentRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.servers.SetAssignment(
		r.Context(), actor.ID, chi.URLParam(r, "id"), chi.URLParam(r, "userID"), request.AccessLevel,
	); err != nil {
		s.writeServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveAssignment(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r.Context())
	if err := s.servers.RemoveAssignment(
		r.Context(), actor.ID, chi.URLParam(r, "id"), chi.URLParam(r, "userID"),
	); err != nil {
		s.writeServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeUserAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, useradmin.ErrForbidden):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "Administrator role required."))
	case errors.Is(err, useradmin.ErrLastAdmin):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "At least one active administrator must remain."))
	case errors.Is(err, useradmin.ErrSelfDelete):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "You cannot delete your own account."))
	case errors.Is(err, useradmin.ErrInvalidRole):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "Role must be admin, operator or read_only."))
	case errors.Is(err, auth.ErrPasswordPolicy):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, err.Error()))
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "User not found."))
	case strings.Contains(strings.ToLower(err.Error()), "unique constraint"):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "That username already exists."))
	default:
		s.logger.Warn("user administration failed", "code", CodeValidation, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The user request could not be completed."))
	}
}

func toUserAdminResponse(user storage.User) userAdminResponse {
	return userAdminResponse{
		ID: user.ID, Username: user.Username, Role: user.Role, IsActive: user.IsActive,
		CreatedAt: user.CreatedAt, LastLoginAt: user.LastLoginAt,
	}
}
