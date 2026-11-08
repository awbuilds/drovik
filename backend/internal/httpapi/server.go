package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/awbuilds/drovik/backend/internal/advanced"
	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/auth"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/files"
	"github.com/awbuilds/drovik/backend/internal/logs"
	"github.com/awbuilds/drovik/backend/internal/monitoring"
	"github.com/awbuilds/drovik/backend/internal/oidc"
	"github.com/awbuilds/drovik/backend/internal/processes"
	"github.com/awbuilds/drovik/backend/internal/secrets"
	"github.com/awbuilds/drovik/backend/internal/security"
	"github.com/awbuilds/drovik/backend/internal/servers"
	systemservices "github.com/awbuilds/drovik/backend/internal/services"
	"github.com/awbuilds/drovik/backend/internal/sshclient"
	"github.com/awbuilds/drovik/backend/internal/storage"
	teamservice "github.com/awbuilds/drovik/backend/internal/teams"
	"github.com/awbuilds/drovik/backend/internal/terminal"
	useradmin "github.com/awbuilds/drovik/backend/internal/users"
	"github.com/awbuilds/drovik/backend/internal/vault"
	"github.com/awbuilds/drovik/backend/internal/version"
)

type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	db     *sql.DB

	users        *storage.UserRepository
	sessions     *storage.SessionRepository
	mfa          *storage.MFARepository
	cipher       *secrets.Cipher
	audit        *audit.Service
	auth         *auth.SessionManager
	servers      *servers.Service
	files        *files.Service
	monitoring   *monitoring.Service
	processes    *processes.Service
	services     *systemservices.Service
	logs         *logs.Service
	terminal     *terminal.Manager
	userAdmin    *useradmin.Service
	advanced     *advanced.Service
	teams        *teamservice.Service
	oidc         *oidc.Provider
	limiter      *security.LoginLimiter
	setupLimiter *security.LoginLimiter
	oidcLimiter  *security.LoginLimiter
	version      version.Info

	staticHandler http.Handler
	indexHTML     []byte
}

func New(cfg *config.Config, logger *slog.Logger, db *sql.DB, auditService *audit.Service) (*Server, error) {
	users := storage.NewUserRepository(db)
	sessions := storage.NewSessionRepository(db)
	mfa := storage.NewMFARepository(db)

	cipher, err := secrets.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	var vaultClient *vault.Client
	if cfg.VaultEnabled {
		vaultClient, err = vault.New(cfg)
		if err != nil {
			return nil, err
		}
	}
	knownHosts := storage.NewKnownHostRepository(db)
	capabilities := storage.NewCapabilitiesRepository(db)
	serverRepository := storage.NewServerRepository(db)
	assignmentRepository := storage.NewAssignmentRepository(db)
	teamRepository := storage.NewTeamRepository(db)
	dialer := sshclient.NewDialer(cfg, knownHosts, logger)
	serverService := servers.NewService(
		cfg,
		serverRepository,
		users,
		assignmentRepository,
		teamRepository,
		knownHosts,
		capabilities,
		cipher,
		vaultClient,
		dialer,
		auditService,
		logger,
	)
	filesService := files.NewService(cfg, serverService, auditService, logger)
	monitoringService := monitoring.NewService(cfg, serverService, logger)
	processService := processes.NewService(cfg, serverService, auditService, logger)
	serviceService := systemservices.NewService(cfg, serverService, auditService, logger)
	logService := logs.NewService(cfg, serverService, logger)
	terminalManager := terminal.NewManager(cfg, serverService, logger)
	userService := useradmin.NewService(users, sessions, auditService, logger)
	teamService := teamservice.NewService(teamRepository, users, serverRepository, auditService, logger)
	var oidcProvider *oidc.Provider
	if cfg.OIDCEnabled {
		oidcProvider, err = oidc.New(cfg, logger)
		if err != nil {
			return nil, err
		}
	}
	advancedService := advanced.NewService(
		cfg,
		serverService,
		storage.NewSnippetRepository(db),
		storage.NewMetricRepository(db),
		storage.NewAlertRepository(db),
		cipher,
		auditService,
		logger,
	)
	pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 10*time.Second)
	advancedService.PruneHistory(pruneCtx)
	pruneCancel()

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
		cfg:        cfg,
		logger:     logger,
		db:         db,
		users:      users,
		sessions:   sessions,
		mfa:        mfa,
		cipher:     cipher,
		audit:      auditService,
		servers:    serverService,
		files:      filesService,
		monitoring: monitoringService,
		processes:  processService,
		services:   serviceService,
		logs:       logService,
		terminal:   terminalManager,
		userAdmin:  userService,
		advanced:   advancedService,
		teams:      teamService,
		oidc:       oidcProvider,
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
		oidcLimiter:   security.NewLoginLimiter(30, time.Minute),
		version:       version.Get(),
		staticHandler: staticHandler,
		indexHTML:     indexHTML,
	}, nil
}

func (s *Server) Close() {
	s.terminal.Close()
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
		r.Get("/auth/oidc/status", s.handleOIDCStatus)
		r.Get("/auth/oidc/start", s.handleOIDCStart)
		r.Get("/auth/oidc/callback", s.handleOIDCCallback)
		r.With(s.originCheck).Post("/setup", s.handleSetup)
		r.With(s.originCheck).Post("/auth/login", s.handleLogin)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/auth/me", s.handleMe)
			r.Get("/auth/mfa", s.handleMFAStatus)
			r.With(s.requireCSRF).Post("/auth/mfa/setup", s.handleMFASetup)
			r.With(s.requireCSRF).Post("/auth/mfa/enable", s.handleMFAEnable)
			r.With(s.requireCSRF).Post("/auth/mfa/disable", s.handleMFADisable)
			r.Get("/auth/oidc/link", s.handleOIDCLinkStatus)
			r.With(s.requireCSRF).Post("/auth/oidc/link", s.handleOIDCLinkStart)
			r.With(s.requireCSRF).Post("/auth/oidc/unlink", s.handleOIDCUnlink)
			r.Get("/activity", s.handleActivity)
			r.Get("/activity/export", s.handleActivityExport)
			r.With(s.requireCSRF).Post("/auth/logout", s.handleLogout)

			r.Get("/servers", s.handleListServers)
			r.With(s.requireAdmin, s.requireCSRF).Post("/servers", s.handleCreateServer)
			r.Get("/servers/{id}", s.handleGetServer)
			r.With(s.requireAdmin, s.requireCSRF).Patch("/servers/{id}", s.handleUpdateServer)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/servers/{id}", s.handleDeleteServer)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/test", s.handleTestConnection)
			r.With(s.requireAdmin, s.requireCSRF).Post("/servers/{id}/trust-host-key", s.handleTrustHostKey)
			r.Get("/servers/{id}/capabilities", s.handleCapabilities)
			r.With(s.requireServerWrite).Get("/servers/{id}/terminal", s.handleTerminal)

			r.Get("/servers/{id}/access", s.handleListAssignments)
			r.With(s.requireAdmin, s.requireCSRF).Put("/servers/{id}/access/{userID}", s.handleSetAssignment)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/servers/{id}/access/{userID}", s.handleRemoveAssignment)

			r.With(s.requireAdmin).Get("/users", s.handleListUsers)
			r.With(s.requireAdmin, s.requireCSRF).Post("/users", s.handleCreateUser)
			r.With(s.requireAdmin).Get("/teams", s.handleListTeams)
			r.With(s.requireAdmin, s.requireCSRF).Post("/teams", s.handleCreateTeam)
			r.With(s.requireAdmin).Get("/teams/{teamID}", s.handleGetTeam)
			r.With(s.requireAdmin, s.requireCSRF).Patch("/teams/{teamID}", s.handleUpdateTeam)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/teams/{teamID}", s.handleDeleteTeam)
			r.With(s.requireAdmin, s.requireCSRF).Put("/teams/{teamID}/members/{userID}", s.handleAddTeamMember)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/teams/{teamID}/members/{userID}", s.handleRemoveTeamMember)
			r.With(s.requireAdmin, s.requireCSRF).Put("/teams/{teamID}/servers/{serverID}", s.handleSetTeamServer)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/teams/{teamID}/servers/{serverID}", s.handleRemoveTeamServer)
			r.With(s.requireAdmin, s.requireCSRF).Patch("/users/{id}", s.handleUpdateUser)
			r.With(s.requireAdmin, s.requireCSRF).Post("/users/{id}/password", s.handleResetUserPassword)
			r.With(s.requireAdmin, s.requireCSRF).Delete("/users/{id}", s.handleDeleteUser)

			r.Get("/servers/{id}/overview", s.handleServerOverview)
			r.Get("/servers/{id}/processes", s.handleListProcesses)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/processes/{pid}/signal", s.handleSignalProcess)
			r.Get("/servers/{id}/services", s.handleListServices)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/services/{unit}/action", s.handleServiceAction)
			r.Get("/servers/{id}/logs", s.handleReadLogs)
			r.Get("/servers/{id}/history", s.handleServerHistory)
			r.Get("/servers/{id}/alerts", s.handleListAlertRules)
			r.With(s.requireServerWrite, s.requireCSRF).Put("/servers/{id}/alerts/{ruleID}", s.handleSaveAlertRule)
			r.With(s.requireServerWrite, s.requireCSRF).Delete("/servers/{id}/alerts/{ruleID}", s.handleDeleteAlertRule)

			r.Get("/servers/{id}/docker", s.handleDocker)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/docker/{container}/action", s.handleDockerAction)
			r.Get("/servers/{id}/docker/{container}/logs", s.handleDockerLogs)
			r.Get("/servers/{id}/docker/{container}/inspect", s.handleDockerInspect)
			r.Get("/servers/{id}/storage-tools", s.handleStorageTools)
			r.Get("/servers/{id}/network-tools", s.handleNetworkTools)
			r.Get("/servers/{id}/packages", s.handlePackages)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/packages/action", s.handlePackageAction)
			r.Get("/servers/{id}/schedules", s.handleSchedules)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/storage/mount", s.handleStorageMount)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/storage/unmount", s.handleStorageUnmount)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/storage/format", s.handleStorageFormat)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/storage/partition", s.handleStoragePartition)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/network/link", s.handleNetworkLink)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/network/address", s.handleNetworkAddress)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/network/route", s.handleNetworkRoute)
			r.With(s.requireAdmin, s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/network/firewall", s.handleNetworkFirewall)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/tasks", s.handleCreateTask)
			r.With(s.requireServerWrite, s.requireCSRF).Delete("/servers/{id}/tasks/{taskID}", s.handleDeleteTask)

			r.Get("/snippets", s.handleListSnippets)
			r.Get("/snippets/export", s.handleExportSnippets)
			r.With(s.requireCSRF).Post("/snippets/import", s.handleImportSnippets)
			r.With(s.requireWriteAccess, s.requireCSRF).Post("/snippets", s.handleCreateSnippet)
			r.Get("/snippets/{id}", s.handleGetSnippet)
			r.With(s.requireWriteAccess, s.requireCSRF).Patch("/snippets/{id}", s.handleUpdateSnippet)
			r.With(s.requireWriteAccess, s.requireCSRF).Delete("/snippets/{id}", s.handleDeleteSnippet)
			r.With(s.requireWriteAccess, s.requireCSRF).Post("/snippets/{id}/preview", s.handlePreviewSnippet)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/snippets/{snippet}/run", s.handleRunSnippet)
			r.Get("/servers/{id}/files", s.handleListFiles)
			r.Get("/servers/{id}/files/stat", s.handleStatFile)
			r.Get("/servers/{id}/files/download", s.handleDownloadFile)
			r.Get("/servers/{id}/files/content", s.handleReadFile)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/files/upload", s.handleUploadFile)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/files/mkdir", s.handleMkdir)
			r.With(s.requireServerWrite, s.requireCSRF).Post("/servers/{id}/files/touch", s.handleTouch)
			r.With(s.requireServerWrite, s.requireCSRF).Patch("/servers/{id}/files/rename", s.handleRename)
			r.With(s.requireServerWrite, s.requireCSRF).Delete("/servers/{id}/files", s.handleDeleteFile)
			r.With(s.requireServerWrite, s.requireCSRF).Put("/servers/{id}/files/content", s.handleWriteFile)
		})
	})

	router.NotFound(s.handleNotFound)
	return router
}
