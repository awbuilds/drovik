package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/storage"
	"github.com/awbuilds/drovik/backend/internal/version"
)

type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *sql.DB

	users        *storage.UserRepository
	sessions     *storage.SessionRepository
	audit        *audit.Service
	auth         *auth.SessionManager
	limiter      *security.LoginLimiter
	setupLimiter *security.LoginLimiter
	version      version.Info
}

func New(cfg *config.Config, logger *slog.Logger, db *sql.DB, auditService *audit.Service) *Server {
	users := storage.NewUserRepository(db)
	sessions := storage.NewSessionRepository(db)
	return &Server{
		cfg:      cfg,
		logger:   logger,
		db:       db,
		users:    users,
		sessions: sessions,
		audit:    auditService,
		auth: auth.NewSessionManager(
			sessions,
			users,
			cfg.SessionIdleTTL,
			cfg.SessionAbsoluteTTL,
			cfg.CookieSecure,
			logger,
		),
		limiter:      security.NewLoginLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow),
		setupLimiter: security.NewLoginLimiter(5, 10*time.Minute),
		version:      version.Get(),
	}
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	router.Use(s.requestID)
	router.Use(s.accessLog)
	router.Use(s.recoverer)
	router.Use(s.securityHeaders)
	router.Use(s.cors)
	router.Use(s.bodyLimit)

	router.Get("/healthz", s.handleHealth)
	router.Get("/readyz", s.handleReady)

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/version", s.handleVersion)
		r.Get("/setup/status", s.handleSetupStatus)
		r.With(s.originCheck).Post("/setup", s.handleSetup)
		r.With(s.originCheck).Post("/auth/login", s.handleLogin)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/auth/me", s.handleMe)
			r.With(s.requireCSRF).Post("/auth/logout", s.handleLogout)
		})
	})

	router.NotFound(s.handleNotFound)
	return router
}
