package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/storage"
)

type auditEventResponse struct {
	ID            string    `json:"id"`
	ActorUserID   *string   `json:"actor_user_id,omitempty"`
	ServerID      *string   `json:"server_id,omitempty"`
	EventType     string    `json:"event_type"`
	TargetSummary string    `json:"target_summary"`
	Outcome       string    `json:"outcome"`
	SafeErrorCode *string   `json:"safe_error_code,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func toAuditEventResponse(event storage.AuditEvent) auditEventResponse {
	return auditEventResponse{
		ID: event.ID, ActorUserID: event.ActorUserID, ServerID: event.ServerID,
		EventType: event.EventType, TargetSummary: event.TargetSummary,
		Outcome: event.Outcome, SafeErrorCode: event.SafeErrorCode,
		CorrelationID: event.CorrelationID, CreatedAt: event.CreatedAt,
	}
}

func parseAuditFilter(r *http.Request, maxLimit int) (storage.AuditFilter, error) {
	filter := storage.AuditFilter{Limit: 100}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > maxLimit {
			return filter, fmt.Errorf("the limit must be between 1 and %d", maxLimit)
		}
		filter.Limit = parsed
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 10000 {
			return filter, fmt.Errorf("the offset must be between 0 and 10000")
		}
		filter.Offset = parsed
	}
	filter.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	if len(filter.Query) > 200 {
		return filter, fmt.Errorf("the search text must be at most 200 characters")
	}
	filter.EventType = strings.TrimSpace(r.URL.Query().Get("event_type"))
	if len(filter.EventType) > 100 || strings.ContainsAny(filter.EventType, "\r\n\t") {
		return filter, fmt.Errorf("the event type filter is invalid")
	}
	filter.Outcome = strings.TrimSpace(r.URL.Query().Get("outcome"))
	switch filter.Outcome {
	case "", "success", "failure", "denied", "unknown":
	default:
		return filter, fmt.Errorf("the outcome filter is invalid")
	}
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("the from time must be RFC3339")
		}
		filter.From = &parsed
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("the to time must be RFC3339")
		}
		filter.To = &parsed
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return filter, fmt.Errorf("the from time must be before the to time")
	}
	return filter, nil
}

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	filter, err := parseAuditFilter(r, 200)
	if err != nil {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, err.Error()))
		return
	}
	events, err := s.audit.ListFilteredForUser(r.Context(), user.ID, user.Role == "admin", filter)
	if err != nil {
		s.logger.Error("audit history failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "Audit history could not be loaded."))
		return
	}
	response := make([]auditEventResponse, 0, len(events))
	for _, event := range events {
		response = append(response, toAuditEventResponse(event))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"events": response, "limit": filter.Limit, "offset": filter.Offset})
}

func (s *Server) handleActivityExport(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The export format must be csv or json."))
		return
	}
	filter, err := parseAuditFilter(r, 5000)
	if err != nil {
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, err.Error()))
		return
	}
	events, err := s.audit.ListFilteredForUser(r.Context(), user.ID, user.Role == "admin", filter)
	if err != nil {
		s.logger.Error("audit export failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "Audit history could not be exported."))
		return
	}
	filename := "drovik-audit-" + time.Now().UTC().Format("20060102-150405") + "." + format
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Cache-Control", "no-store")
	if format == "json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		response := make([]auditEventResponse, 0, len(events))
		for _, event := range events {
			response = append(response, toAuditEventResponse(event))
		}
		writeJSON(w, http.StatusOK, map[string]any{"exported_at": time.Now().UTC(), "events": response})
	} else {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"id", "created_at", "actor_user_id", "server_id", "event_type", "target_summary", "outcome", "safe_error_code", "correlation_id"}); err != nil {
			s.logger.Error("audit export header failed", "error", err, "request_id", RequestID(r.Context()))
			return
		}
		for _, event := range events {
			row := []string{
				event.ID, event.CreatedAt.UTC().Format(time.RFC3339Nano), optional(event.ActorUserID),
				optional(event.ServerID), event.EventType, event.TargetSummary, event.Outcome,
				optional(event.SafeErrorCode), event.CorrelationID,
			}
			for index := range row {
				row[index] = csvSafe(row[index])
			}
			if err := writer.Write(row); err != nil {
				s.logger.Error("audit export row failed", "error", err, "request_id", RequestID(r.Context()))
				return
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			s.logger.Error("audit export flush failed", "error", err, "request_id", RequestID(r.Context()))
		}
	}
	s.record(r, user.ID, "audit.exported", format, "success", "")
}

func optional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func csvSafe(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}
	return value
}
