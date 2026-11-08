package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/storage"
	teamservice "github.com/awbuilds/drovik/backend/internal/teams"
)

type teamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type teamServerRequest struct {
	AccessLevel string `json:"access_level"`
}

type teamResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MemberCount int       `json:"member_count"`
	ServerCount int       `json:"server_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type teamMemberResponse struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type teamServerResponse struct {
	ServerID    string    `json:"server_id"`
	DisplayName string    `json:"display_name"`
	Host        string    `json:"host"`
	AccessLevel string    `json:"access_level"`
	CreatedAt   time.Time `json:"created_at"`
}

type teamDetailResponse struct {
	Team    teamResponse         `json:"team"`
	Members []teamMemberResponse `json:"members"`
	Servers []teamServerResponse `json:"servers"`
}

func toTeamResponse(team storage.TeamSummary) teamResponse {
	return teamResponse{
		ID: team.ID, Name: team.Name, Description: team.Description,
		MemberCount: team.MemberCount, ServerCount: team.ServerCount,
		CreatedAt: team.CreatedAt, UpdatedAt: team.UpdatedAt,
	}
}

func (s *Server) handleListTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.teams.List(r.Context())
	if err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	response := make([]teamResponse, 0, len(teams))
	for _, team := range teams {
		response = append(response, toTeamResponse(team))
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": response})
}

func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request teamRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	team, err := s.teams.Create(r.Context(), user.ID, request.Name, request.Description)
	if err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTeamResponse(storage.TeamSummary{Team: *team}))
}

func (s *Server) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	detail, err := s.teams.Detail(r.Context(), chi.URLParam(r, "teamID"))
	if err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	response := teamDetailResponse{
		Team:    toTeamResponse(detail.Team),
		Members: make([]teamMemberResponse, 0, len(detail.Members)),
		Servers: make([]teamServerResponse, 0, len(detail.Servers)),
	}
	for _, member := range detail.Members {
		response.Members = append(response.Members, teamMemberResponse{
			UserID: member.UserID, Username: member.Username, Role: member.Role, CreatedAt: member.CreatedAt,
		})
	}
	for _, server := range detail.Servers {
		response.Servers = append(response.Servers, teamServerResponse{
			ServerID: server.ServerID, DisplayName: server.DisplayName, Host: server.Host,
			AccessLevel: server.AccessLevel, CreatedAt: server.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleUpdateTeam(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request teamRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	team, err := s.teams.Update(r.Context(), user.ID, chi.URLParam(r, "teamID"), request.Name, request.Description)
	if err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toTeamResponse(storage.TeamSummary{Team: *team}))
}

func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.teams.Delete(r.Context(), user.ID, chi.URLParam(r, "teamID")); err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.teams.AddMember(r.Context(), user.ID, chi.URLParam(r, "teamID"), chi.URLParam(r, "userID")); err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.teams.RemoveMember(r.Context(), user.ID, chi.URLParam(r, "teamID"), chi.URLParam(r, "userID")); err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSetTeamServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request teamServerRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.teams.SetServer(r.Context(), user.ID, chi.URLParam(r, "teamID"), chi.URLParam(r, "serverID"), request.AccessLevel); err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRemoveTeamServer(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	if err := s.teams.RemoveServer(r.Context(), user.ID, chi.URLParam(r, "teamID"), chi.URLParam(r, "serverID")); err != nil {
		s.writeTeamError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeTeamError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, teamservice.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "Team not found."))
	case errors.Is(err, teamservice.ErrUserNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "User not found."))
	case errors.Is(err, teamservice.ErrServerNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServerNotFound, "Server not found."))
	case errors.Is(err, teamservice.ErrNameTaken):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeConflict, "A team with that name already exists."))
	case errors.Is(err, teamservice.ErrInvalid):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, err.Error()))
	default:
		s.logger.Error("team operation failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
	}
}
