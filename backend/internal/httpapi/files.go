package httpapi

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/files"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

type pathRequest struct {
	Path string `json:"path"`
}

type renameRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Overwrite bool   `json:"overwrite"`
}

type writeFileRequest struct {
	Path               string     `json:"path"`
	Content            string     `json:"content"`
	ExpectedModifiedAt *time.Time `json:"expected_modified_at"`
	ExpectedSize       *int64     `json:"expected_size"`
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	result, err := s.files.List(r.Context(), user.ID, chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeFileError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStatFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	entry, err := s.files.Stat(r.Context(), user.ID, chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeFileError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	content, err := s.files.ReadFile(r.Context(), user.ID, chi.URLParam(r, "id"), r.URL.Query().Get("path"))
	if err != nil {
		s.writeFileError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, content)
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request writeFileRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	var expected *files.ExpectedVersion
	if request.ExpectedModifiedAt != nil || request.ExpectedSize != nil {
		expected = &files.ExpectedVersion{ModifiedAt: request.ExpectedModifiedAt, Size: request.ExpectedSize}
	}
	if err := s.files.WriteFile(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		request.Path, request.Content, expected,
	); err != nil {
		s.writeFileError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request pathRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.files.Mkdir(r.Context(), user.ID, chi.URLParam(r, "id"), request.Path); err != nil {
		s.writeFileError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": request.Path})
}

func (s *Server) handleTouch(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request pathRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.files.Touch(r.Context(), user.ID, chi.URLParam(r, "id"), request.Path); err != nil {
		s.writeFileError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"path": request.Path})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	var request renameRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, r, err)
		return
	}
	if err := s.files.Rename(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		request.From, request.To, request.Overwrite,
	); err != nil {
		s.writeFileError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	recursive, _ := strconv.ParseBool(r.URL.Query().Get("recursive"))
	if err := s.files.Delete(
		r.Context(), user.ID, chi.URLParam(r, "id"),
		r.URL.Query().Get("path"), recursive,
	); err != nil {
		s.writeFileError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	download, err := s.files.OpenDownload(
		r.Context(), user.ID, chi.URLParam(r, "id"), r.URL.Query().Get("path"),
	)
	if err != nil {
		s.writeFileError(w, r, err)
		return
	}
	defer download.Close()

	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": download.Name})
	if disposition == "" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.FormatInt(download.Size, 10))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, download.Reader); err != nil {
		s.logger.Debug("file download interrupted", "error", err, "request_id", RequestID(r.Context()))
	}
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r.Context())
	limit := s.cfg.SFTPMaxUploadBytes + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, r, NewAPIError(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Uploads must use multipart/form-data."))
		return
	}
	overwrite, _ := strconv.ParseBool(r.URL.Query().Get("overwrite"))
	target := r.URL.Query().Get("path")

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxBytes *http.MaxBytesError
			if errors.As(err, &maxBytes) {
				writeError(w, r, NewAPIError(http.StatusRequestEntityTooLarge, CodeFileTooLarge, "The upload exceeds the configured limit."))
				return
			}
			s.writeFileError(w, r, err)
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		if uploadErr := s.files.Upload(
			r.Context(), user.ID, chi.URLParam(r, "id"),
			target, part, -1, overwrite,
		); uploadErr != nil {
			_ = part.Close()
			s.writeFileError(w, r, uploadErr)
			return
		}
		_ = part.Close()
		writeJSON(w, http.StatusCreated, map[string]string{"path": target})
		return
	}
	writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "No file part was provided."))
}

func (s *Server) writeFileError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, files.ErrPathInvalid):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeValidation, "The remote path is invalid."))
	case errors.Is(err, files.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeNotFound, "The remote path was not found."))
	case errors.Is(err, files.ErrSymlink):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSymlinkNotAllowed, "Symlink operations are not allowed for this action."))
	case errors.Is(err, files.ErrIsDirectory):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeIsDirectory, "The path is a directory."))
	case errors.Is(err, files.ErrTooLarge):
		writeError(w, r, NewAPIError(http.StatusRequestEntityTooLarge, CodeFileTooLarge, "The file exceeds the configured limit."))
	case errors.Is(err, files.ErrBinary):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeFileBinary, "The file is binary or not valid UTF-8."))
	case errors.Is(err, files.ErrStale):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeFileStale, "The remote file changed since it was opened. Reload before saving."))
	case errors.Is(err, files.ErrConflict):
		writeError(w, r, NewAPIError(http.StatusConflict, CodeFileConflict, "The remote path already exists."))
	case errors.Is(err, files.ErrSFTPUnavailable):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSFTPUnavailable, "The host does not expose an SFTP subsystem."))
	case errors.Is(err, files.ErrPermission):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeSFTPPermissionDenied, "The remote SSH user does not have permission for this path or action."))
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, r, NewAPIError(http.StatusNotFound, CodeServerNotFound, "Server not found."))
	case errors.Is(err, servers.ErrUnauthorized):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "You do not have access to this server."))
	case errors.Is(err, security.ErrEgressDenied):
		writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "The destination is not allowed by the egress policy."))
	case errors.Is(err, sshclient.ErrInvalidCredentials), errors.Is(err, sshclient.ErrInvalidPrivateKey):
		writeError(w, r, NewAPIError(http.StatusUnprocessableEntity, CodeSSHAuthFailed, "SSH authentication failed."))
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, r, NewAPIError(http.StatusGatewayTimeout, CodeSSHTimeout, "The SFTP operation timed out."))
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
		s.logger.Error("file operation failed",
			"code", CodeSFTPUnavailable,
			"request_id", RequestID(r.Context()),
		)
		writeError(w, r, NewAPIError(http.StatusBadGateway, CodeSFTPUnavailable, "The SFTP operation could not be completed."))
	}
}
