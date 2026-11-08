package httpapi

import (
	"net/http"
	"strconv"
	"time"
)

type auditEventResponse struct {
	ID            string    `json:"id"`
	ServerID      *string   `json:"server_id,omitempty"`
	EventType     string    `json:"event_type"`
	TargetSummary string    `json:"target_summary"`
	Outcome       string    `json:"outcome"`
	SafeErrorCode *string   `json:"safe_error_code,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 200 {
			writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The limit must be between 1 and 200."))
			return
		}
		limit = parsed
	}
	events, err := s.audit.ListRecentForUser(r.Context(), user.ID, limit)
	if err != nil {
		s.logger.Error("audit history failed", "code", CodeInternal, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "Audit history could not be loaded."))
		return
	}
	response := make([]auditEventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, auditEventResponse{
			ID:            event.ID,
			ServerID:      event.ServerID,
			EventType:     event.EventType,
			TargetSummary: event.TargetSummary,
			Outcome:       event.Outcome,
			SafeErrorCode: event.SafeErrorCode,
			CreatedAt:     event.CreatedAt,
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"events": response})
}
