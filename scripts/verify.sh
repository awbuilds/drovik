#!/usr/bin/env bash
# Run the full local verification suite. Never claims success for a step that did not run.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

step() {
    printf '\n==> %s\n' "$1"
}

on_error() {
    status=$?
    printf '\nVERIFY FAILED (exit %s) at line %s\n' "$status" "$1" >&2
    exit "$status"
}
trap 'on_error $LINENO' ERR

ran=()
skipped=()

if [[ -f backend/go.mod ]]; then
    step "backend: gofmt"
    unformatted=$(gofmt -l backend)
    if [[ -n "$unformatted" ]]; then
        echo "Unformatted Go files:" >&2
        echo "$unformatted" >&2
        exit 1
    fi
    ran+=("gofmt")

    step "backend: go vet"
    (cd backend && go vet ./...)
    ran+=("go vet")

    step "backend: go test -race"
    (cd backend && go test -race ./...)
    ran+=("go test -race")

    if command -v golangci-lint >/dev/null 2>&1; then
        step "backend: golangci-lint"
        (cd backend && golangci-lint run)
        ran+=("golangci-lint")
    else
        skipped+=("golangci-lint (not installed)")
    fi
else
    skipped+=("backend (no go.mod yet)")
fi

if [[ -f frontend/package.json ]]; then
    step "frontend: install"
    if [[ -f frontend/pnpm-lock.yaml ]]; then
        (cd frontend && pnpm install --frozen-lockfile)
        ran+=("pnpm install --frozen-lockfile")
    else
        (cd frontend && pnpm install)
        ran+=("pnpm install")
    fi

    step "frontend: lint"
    (cd frontend && pnpm lint)
    ran+=("frontend lint")

    step "frontend: typecheck"
    (cd frontend && pnpm typecheck)
    ran+=("frontend typecheck")

    step "frontend: test"
    (cd frontend && pnpm test)
    ran+=("frontend test")

    step "frontend: build"
    (cd frontend && pnpm build)
    ran+=("frontend build")
else
    skipped+=("frontend (no package.json yet)")
fi

printf '\n==> Summary\n'
printf 'Ran:\n'
for item in "${ran[@]}"; do printf '  - %s\n' "$item"; done
if [[ ${#skipped[@]} -gt 0 ]]; then
    printf 'Skipped:\n'
    for item in "${skipped[@]}"; do printf '  - %s\n' "$item"; done
fi

printf '\nAll executed checks passed.\n'
