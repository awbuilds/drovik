package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/awbuilds/drovik/backend/internal/audit"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/httpapi"
	"github.com/awbuilds/drovik/backend/internal/safe"
	"github.com/awbuilds/drovik/backend/internal/storage"
)

func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn("database close failed", "error", closeErr)
		}
	}()

	if err := storage.Migrate(ctx, db); err != nil {
		return err
	}

	auditService := audit.New(storage.NewAuditRepository(db), logger)
	api := httpapi.New(cfg, logger, db, auditService)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	startSessionJanitor(ctx, storage.NewSessionRepository(db), logger)

	serverErr := make(chan error, 1)
	safe.Go(ctx, logger, "http-server", func(context.Context) {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	})

	logger.Info("drovik listening", "addr", cfg.HTTPAddr, "env", cfg.Env)

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErr:
		logger.Error("http server failed", "error", err)
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

func startSessionJanitor(ctx context.Context, sessions *storage.SessionRepository, logger *slog.Logger) {
	safe.Go(ctx, logger, "session-janitor", func(ctx context.Context) {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, err := sessions.DeleteExpired(ctx, time.Now().UTC())
				if err != nil {
					if !errors.Is(err, sql.ErrConnDone) {
						logger.Warn("session cleanup failed", "error", err)
					}
					continue
				}
				if deleted > 0 {
					logger.Info("expired sessions removed", "count", deleted)
				}
			}
		}
	})
}
