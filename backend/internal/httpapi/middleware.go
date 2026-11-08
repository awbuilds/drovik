package httpapi

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/security"
)

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	if !r.wroteHeader {
		r.status = http.StatusSwitchingProtocols
		r.wroteHeader = true
	}
	return hijacker.Hijack()
}

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder, _ := w.(*statusRecorder)
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("panic recovered",
					"panic", recovered,
					"stack", string(debug.Stack()),
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", RequestID(r.Context()),
				)
				if recorder == nil || !recorder.wroteHeader {
					writeError(w, r, NewAPIError(
						http.StatusInternalServerError,
						CodeInternal,
						"An internal error occurred.",
					))
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		level := slog.LevelInfo
		switch {
		case recorder.status >= 500:
			level = slog.LevelError
		case recorder.status >= 400:
			level = slog.LevelWarn
		case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
			level = slog.LevelDebug
		}

		s.logger.Log(r.Context(), level, "http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"bytes", recorder.bytes,
			"duration_ms", duration.Milliseconds(),
			"request_id", RequestID(r.Context()),
		)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"font-src 'self'; "+
				"connect-src 'self'; "+
				"object-src 'none'; "+
				"base-uri 'none'; "+
				"frame-ancestors 'none'; "+
				"form-action 'self'")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Cross-Origin-Resource-Policy", "same-origin")
		if s.cfg.PublicURL.Scheme == "https" {
			header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if security.IsTrustedOrigin(s.cfg.TrustedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "600")
			} else if r.Method == http.MethodOptions {
				writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "Origin is not allowed."))
				return
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) bodyLimit(next http.Handler) http.Handler {
	const maxJSONBody = 4 << 20 // 4 MiB for JSON endpoints, including editor payloads.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := strings.ToLower(r.Header.Get("Content-Type"))
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// Upload handlers apply their own file-size limit.
			next.ServeHTTP(w, r)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if security.IsMutationMethod(r.Method) {
			if origin := r.Header.Get("Origin"); origin != "" &&
				!security.IsTrustedOrigin(s.cfg.TrustedOrigins, origin) {
				writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "Origin is not allowed."))
				return
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeError(w, r, NewAPIError(http.StatusForbidden, CodeAuthForbidden, "Cross-site request blocked."))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.auth.SessionToken(r)
		if token == "" {
			writeError(w, r, NewAPIError(http.StatusUnauthorized, CodeAuthRequired, "Authentication required."))
			return
		}

		session, user, err := s.auth.Validate(r.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidSession) {
				s.auth.ClearSessionCookie(w)
				writeError(w, r, NewAPIError(
					http.StatusUnauthorized,
					CodeAuthSessionExpired,
					"Your session has expired. Sign in again.",
				))
				return
			}
			s.logger.Error("session validation failed",
				"error", err,
				"request_id", RequestID(r.Context()),
			)
			writeError(w, r, NewAPIError(http.StatusInternalServerError, CodeInternal, "An internal error occurred."))
			return
		}

		next.ServeHTTP(w, r.WithContext(withAuth(r.Context(), user, session)))
	})
}

func (s *Server) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !security.IsMutationMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		sessionToken := s.auth.SessionToken(r)
		headerToken := r.Header.Get("X-CSRF-Token")
		if !s.auth.ValidateCSRF(sessionToken, headerToken) {
			writeError(w, r, NewAPIError(http.StatusForbidden, CodeCSRFInvalid, "CSRF validation failed."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return strings.Repeat("0", 32)
	}
	return hex.EncodeToString(raw)
}
