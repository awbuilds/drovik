#!/usr/bin/env bash
# Create a new dated handoff note from the template.
# Usage: scripts/handoff-new.sh "short topic"
set -euo pipefail

if [[ $# -lt 1 || -z "${1// }" ]]; then
    echo "usage: scripts/handoff-new.sh \"short topic\"" >&2
    exit 2
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
date_prefix=$(date +%Y-%m-%d)
slug=$(printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//')
file="$root/docs/handoffs/${date_prefix}-${slug}.md"

if [[ -e "$file" ]]; then
    echo "Handoff already exists: $file" >&2
    exit 1
fi

cat > "$file" <<EOF
# ${date_prefix} ${1}

## Task / Area
One line: what this session worked on.

## Current State
What exists now, what is done, and what is not.

## Decisions
- Key decisions and the reasoning (bullets).

## Files Touched
- \`path\` — why.

## Build / Test Status
- What was verified (commands and results); explicitly list anything not run.

## Next Steps
- Ordered, concrete next actions.

## Open Questions
- Anything blocking or ambiguous.
EOF

echo "Created $file"
