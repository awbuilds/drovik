package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/awbuilds/drovik/backend/internal/app"
	"github.com/awbuilds/drovik/backend/internal/config"
	"github.com/awbuilds/drovik/backend/internal/observability"
	"github.com/awbuilds/drovik/backend/internal/version"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			info := version.Get()
			fmt.Printf("drovik %s (commit %s, built %s, %s)\n", info.Version, info.Commit, info.BuildDate, info.GoVersion)
			return
		case "healthcheck":
			os.Exit(runHealthcheck())
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "drovik: configuration error:", err)
		os.Exit(1)
	}

	logger, err := observability.New(cfg.LogLevel, cfg.LogFormat, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drovik: logger initialization failed:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("drovik stopped with an error", "error", err, slog.String("version", version.Version))
		os.Exit(1)
	}
}

// runHealthcheck is used by container health checks. It makes one bounded local
// request and never logs configuration or secrets.
func runHealthcheck() int {
	target := os.Getenv("DROVIK_HEALTHCHECK_URL")
	if target == "" {
		target = "http://127.0.0.1:8080/healthz"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	//nolint:gosec // target is an operator-controlled local healthcheck URL, not user input
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drovik healthcheck: request creation failed:", err)
		return 1
	}
	client := &http.Client{Timeout: 3 * time.Second}
	//nolint:gosec // target is an operator-controlled local healthcheck URL, not user input
	response, err := client.Do(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "drovik healthcheck: request failed:", err)
		return 1
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "drovik healthcheck: unexpected status:", response.StatusCode)
		return 1
	}
	return 0
}
