## Summary

Describe the change and why it is needed.

## Requirement / issue

- Requirement ID(s):
- Closes / relates to:

## Type of change

- [ ] Feature
- [ ] Bug fix
- [ ] Security fix
- [ ] Refactor
- [ ] Documentation
- [ ] CI/build
- [ ] Chore

## Crash safety

- [ ] Affected entry points are guarded (HTTP, WebSocket, goroutine, job, CLI).
- [ ] Errors are returned/wrapped, mapped to stable codes, and logged once at the boundary.
- [ ] No secrets or sensitive content are logged.
- [ ] Frontend async/event/WebSocket paths have explicit error handling and user-visible states.
- [ ] A crash-safety test was added or updated, or this change does not touch an entry point.

## Security

- [ ] Backend authorization is enforced where relevant.
- [ ] Inputs and outputs are validated and bounded.
- [ ] No command construction from untrusted input.
- [ ] No host-key verification bypass, plaintext credential, or weakened security control.
- [ ] No secrets or private data are added to the repository.
- [ ] Dependency changes were reviewed.

## Tests run

List the exact commands and results. Do not claim tests you did not run.

```text
```

## Documentation

- [ ] Docs/ADR updated where behavior or decisions changed.
- [ ] `docs/product/traceability.md` updated.
- [ ] `CHANGELOG.md` updated for user-visible changes.
- [ ] Handoff note written in `docs/handoffs/`.

## UI (if applicable)

- [ ] Loading, empty, success, warning, permission-denied, timeout and error states handled.
- [ ] Keyboard accessible; visible focus; reduced motion respected.
- [ ] No sensitive values in browser storage or console.

## Checklist

- [ ] Scope is clear and limited.
- [ ] `main` was not modified directly.
- [ ] CI is green.
- [ ] No known critical/high security issue introduced.
