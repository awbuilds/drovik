package httpapi

import (
	"encoding/json"
	"net/http"
)

const (
	CodeAuthRequired = "AUTH_REQUIRED"
	//nolint:gosec // stable error code, not a credential
	CodeAuthInvalidCredentials = "AUTH_INVALID_CREDENTIALS"
	CodeAuthSessionExpired     = "AUTH_SESSION_EXPIRED"
	CodeAuthForbidden          = "AUTH_FORBIDDEN"
	CodeCSRFInvalid            = "CSRF_INVALID"
	CodeValidation             = "VALIDATION_ERROR"
	CodeSetupAlreadyComplete   = "SETUP_ALREADY_COMPLETE"
	CodeRateLimited            = "RATE_LIMITED"
	CodeNotFound               = "NOT_FOUND"
	CodeMethodNotAllowed       = "METHOD_NOT_ALLOWED"
	CodePayloadTooLarge        = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMediaType   = "UNSUPPORTED_MEDIA_TYPE"
	CodeInternal               = "INTERNAL_ERROR"
	CodeServiceUnavailable     = "SERVICE_UNAVAILABLE"
	CodeConflict               = "CONFLICT"
	CodeServerNotFound         = "SERVER_NOT_FOUND"
	CodeSSHConnectionFailed    = "SSH_CONNECTION_FAILED"
	CodeSSHHostKeyUnknown      = "SSH_HOST_KEY_UNKNOWN"
	CodeSSHHostKeyChanged      = "SSH_HOST_KEY_CHANGED"
	CodeSSHAuthFailed          = "SSH_AUTH_FAILED"
	CodeSSHTimeout             = "SSH_OPERATION_TIMEOUT"
	CodeFileTooLarge           = "FILE_TOO_LARGE"
	CodeFileBinary             = "FILE_BINARY"
	CodeFileStale              = "FILE_STALE"
	CodeFileConflict           = "FILE_CONFLICT"
	CodeSFTPUnavailable        = "SFTP_UNAVAILABLE"
	CodeSymlinkNotAllowed      = "SYMLINK_NOT_ALLOWED"
	CodeIsDirectory            = "IS_DIRECTORY"
	CodeSFTPPermissionDenied   = "SFTP_PERMISSION_DENIED"
)

type APIError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func WithDetails(err *APIError, details any) *APIError {
	err.Details = details
	return err
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Details       any    `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, err *APIError) {
	if err == nil {
		err = NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred.")
	}
	if err.Status == 0 {
		err.Status = http.StatusInternalServerError
	}
	writeJSON(w, err.Status, errorEnvelope{Error: errorBody{
		Code:          err.Code,
		Message:       err.Message,
		CorrelationID: RequestID(r.Context()),
		Details:       err.Details,
	}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	_ = encoder.Encode(value)
}
