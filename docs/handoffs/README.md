# Session Handoff Notes

Purpose: let any agent or contributor resume Drovik work without re-discovering state. Notes are
append-only; the newest file (by date prefix) for an area is the current state. Any agent that made
progress in a session writes one note before finishing; any agent starting work reads the latest
note(s) for its area first.

## Rules

- Name notes `YYYY-MM-DD-short-topic.md` (for example `2026-11-07-phase-0-foundation.md`).
- Append-only: never edit or delete an existing note; write a new dated note when state changes.
- Newest wins: the most recent note for an area is authoritative over older ones.
- Keep each note short (aim for under 60 lines): it is a resume point, not a spec.
- If a task is finished and merged, say so in the note so the next agent does not redo it.
- Never include secrets, credentials, real hostnames, private data or absolute user paths.
- Record what was actually run and what was not. Do not invent test results.

## Template

```markdown
# YYYY-MM-DD <short topic>

## Task / Area
One line: what this session worked on.

## Current State
What exists now, what is done, and what is not.

## Decisions
- Key decisions and the reasoning (bullets).

## Files Touched
- `path` — why.

## Build / Test Status
- What was verified (commands and results); explicitly list anything not run.

## Next Steps
- Ordered, concrete next actions.

## Open Questions
- Anything blocking or ambiguous.
```

## Finding the latest note

```bash
ls -1 docs/handoffs/*.md | sort | tail -5
```
