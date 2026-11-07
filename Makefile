SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

.PHONY: help verify backend-test backend-lint backend-fmt frontend-install frontend-dev frontend-lint frontend-typecheck frontend-test frontend-build security hooks

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

verify: ## Run all available checks
	@bash scripts/verify.sh

backend-test: ## Run Go tests with the race detector
	cd backend && go test -race ./...

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
	cd frontend && pnpm test -- --run

frontend-build: ## Build the frontend
	cd frontend && pnpm build

security: ## Run local security checks
	@command -v gitleaks >/dev/null 2>&1 && gitleaks detect --source . --redact -v || echo "gitleaks not installed; skipped"
	@command -v govulncheck >/dev/null 2>&1 && (cd backend && govulncheck ./...) || echo "govulncheck not installed; skipped"
	@command -v pnpm >/dev/null 2>&1 && (cd frontend && pnpm audit --audit-level high) || echo "pnpm audit skipped"

hooks: ## Install repository git hooks
	git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks"
