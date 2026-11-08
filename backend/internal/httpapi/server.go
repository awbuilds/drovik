package httpapi

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
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
	servers      *servers.Service
	limiter      *security.LoginLimiter
	setupLimiter *security.LoginLimiter
	version      version.Info

	staticHandler http.Handler
	indexHTML     []byte
}

func New(cfg *config.Config, logger *slog.Logger, db *sql.DB, auditService *audit.Service) (*Server, error) {
	users := storage.NewUserRepository(db)
	sessions := storage.NewSessionRepository(db)

	cipher, err := secrets.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	knownHosts := storage.NewKnownHostRepository(db)
	capabilities := storage.NewCapabilitiesRepository(db)
	serverRepository := storage.NewServerRepository(db)
	dialer := sshclient.NewDialer(cfg, knownHosts, logger)
	serverService := servers.NewService(
		cfg,
		serverRepository,
		knownHosts,
		capabilities,
		cipher,
		dialer,
		auditService,
		logger,
	)

	var staticHandler http.Handler
	var indexHTML []byte
	if cfg.StaticDir != "" {
		index, err := os.ReadFile(filepath.Join(cfg.StaticDir, "index.html"))
		if err == nil {
			indexHTML = index
			staticHandler = http.FileServer(http.Dir(cfg.StaticDir))
		} else {
			logger.Debug("static frontend not available", "dir", cfg.StaticDir, "error", err)
		}
	}

	return &Server{
		cfg:      cfg,
		logger:   logger,
		db:       db,
		users:    users,
		sessions: sessions,
		audit:    auditService,
		servers:  serverService,
		auth: auth.NewSessionManager(
			sessions,
			users,
			cfg.SessionIdleTTL,
			cfg.SessionAbsoluteTTL,
			cfg.CookieSecure,
			cfg.EncryptionKey,
			logger,
		),
		limiter:       security.NewLoginLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow),
		setupLimiter:  security.NewLoginLimiter(5, 10*time.Minute),
		version:       version.Get(),
		staticHandler: staticHandler,
		indexHTML:     indexHTML,
	}, nil
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

			r.Get("/servers", s.handleListServers)
			r.With(s.requireCSRF).Post("/servers", s.handleCreateServer)
			r.Get("/servers/{id}", s.handleGetServer)
			r.With(s.requireCSRF).Patch("/servers/{id}", s.handleUpdateServer)
			r.With(s.requireCSRF).Delete("/servers/{id}", s.handleDeleteServer)
			r.With(s.requireCSRF).Post("/servers/{id}/test", s.handleTestConnection)
			r.With(s.requireCSRF).Post("/servers/{id}/trust-host-key", s.handleTrustHostKey)
			r.Get("/servers/{id}/capabilities", s.handleCapabilities)
		})
	})

	router.NotFound(s.handleNotFound)
	return router
}
