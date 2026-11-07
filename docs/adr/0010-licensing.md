# ADR 0010 — Licensing

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik is a self-hosted network service. The project intends to remain open source and to require
that modified versions served over a network also make their source available, while allowing
self-hosting and commercial use.

## Decision

- License: GNU Affero General Public License v3.0 only (`AGPL-3.0-only`).
- Copyright holder: Abhijith Warrier.
- Source files carry SPDX identifiers where practical.
- Third-party dependencies must have compatible licenses; review before adding.
- The project name “Drovik” remains a working name pending trademark clearance.

## Consequences

- Network operators who modify Drovik must offer the corresponding source.
- The license is compatible with self-hosting and most open-source contributions.
- Some organizations avoid AGPL software; that is an accepted tradeoff for the product's intended
  protection.
- Legal review is recommended before a public launch or commercial relaunch.

## Alternatives considered

- MIT/Apache-2.0: permissive, but does not require source disclosure for network-served
  modifications.
- GPL-3.0: covers distributed binaries but not network-only use.
- Proprietary/open-core: contradicts the open-source product goal.

## References

PRD §0.1, §15; `LICENSE`.
