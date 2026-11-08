SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

.PHONY: help verify backend-test backend-bench backend-lint backend-fmt frontend-install frontend-dev frontend-lint frontend-typecheck frontend-test frontend-build site-check site-serve security hooks

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

verify: ## Run all available checks
	@bash scripts/verify.sh

backend-test: ## Run Go tests with the race detector
	cd backend && go test -race ./...

backend-bench: ## Run parser benchmarks
	cd backend && go test -run '^$$' -bench . -benchmem ./internal/monitoring ./internal/processes ./internal/logs ./internal/services

backend-lint: ## Run Go vet and golangci-lint
	cd backend && go vet ./...
	cd backend && golangci-lint run

backend-fmt: ## Format Go code
	cd backend && gofmt -w .

frontend-install: ## Install frontend dependencies
	cd frontend && pnpm install --frozen-lockfile

frontend-dev: ## Start the frontend dev server
	cd frontend && pnpm dev

frontend-lint: ## Lint the frontend
	cd frontend && pnpm lint

frontend-typecheck: ## Typecheck the frontend
	cd frontend && pnpm typecheck

frontend-test: ## Run frontend tests
	cd frontend && pnpm test

frontend-build: ## Build the frontend
	cd frontend && pnpm build

site-check: ## Check the static project website
	python3 scripts/check-site.py

site-serve: ## Preview the project website at http://127.0.0.1:4173/
	python3 -m http.server 4173 --directory site

security: ## Run local security checks
	@command -v gitleaks >/dev/null 2>&1 && gitleaks detect --source . --redact -v || echo "gitleaks not installed; skipped"
	@command -v govulncheck >/dev/null 2>&1 && (cd backend && govulncheck ./...) || echo "govulncheck not installed; skipped"
	@command -v pnpm >/dev/null 2>&1 && (cd frontend && pnpm audit --audit-level high) || echo "pnpm audit skipped"

hooks: ## Install repository git hooks
	git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks"
