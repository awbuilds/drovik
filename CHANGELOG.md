# Changelog

All notable changes to Drovik are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Project foundation: AGENTS.md contract, Hydra-style handoff notes, AGPL-3.0 licensing,
  contribution, security and governance documentation.
- Architecture decision records and threat model.
- React + Tailwind design system foundation.
- Go backend foundation with structured logging, recovery middleware, health checks and graceful
  shutdown.
- First-run setup, local authentication, sessions and CSRF protection.
- Docker Compose deployment with a separately mounted encryption secret.

### Security

- No telemetry or external error reporting by default.
- Secret scanning and dependency review in CI.

<!--
When releasing, rename [Unreleased] to [x.y.z] - YYYY-MM-DD and add a new Unreleased section.
-->
