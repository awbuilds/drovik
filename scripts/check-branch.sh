#!/usr/bin/env bash
# Refuse to commit on main. Run from the repository root.
set -euo pipefail

branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

if [[ "$branch" == "main" || "$branch" == "master" ]]; then
    echo "Refusing to work on '$branch'. Create a feature branch first:" >&2
    echo "  git switch -c feat/short-description" >&2
    exit 1
fi

if [[ "$branch" == "HEAD" ]]; then
    echo "Refusing to commit from a detached HEAD. Create a branch first." >&2
    exit 1
fi

echo "Branch guard OK: $branch"
