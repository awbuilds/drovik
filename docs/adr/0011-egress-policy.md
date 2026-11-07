# ADR 0011 — Egress policy defaults

- **Status:** Accepted
- **Date:** 2026-11-07

## Context

Drovik connects to user-specified SSH hosts. This is necessary for the product, but it also makes
the backend a network-capable service that could be abused to reach unintended internal services.
Most users manage homelab or private-network servers, so blindly blocking private ranges would
break the primary use case. The PRD requires configurable egress controls and special handling of
loopback, link-local, metadata and reserved ranges.

## Decision

Default policy:

- Allow public internet addresses and RFC1918 private ranges.
- Block loopback, link-local, multicast, unspecified, reserved and cloud metadata endpoints by
  default.
- Block known cloud metadata addresses (including `169.254.169.254`) unless the operator
  explicitly allows them.
- Resolve DNS once for validation, connect to the validated address, and preserve correct SSH
  host-key identity.
- Revalidate every new connection attempt.
- Apply outbound rate limits and a maximum concurrent destination count.
- Log connection attempts as metadata without credentials.

Operators can configure allow/deny CIDR rules and private-range access. Multi-user deployments
should default to an administrator-managed allowlist or approval flow.

## Consequences

- Homelab use works out of the box.
- A compromised authenticated account can still reach allowed private hosts; authorization and
  audit remain essential.
- Multi-tenant hosting requires stricter configuration and is not the default.
- DNS rebinding and TOCTOU risks must be handled by connecting to the validated resolved address.

## Alternatives considered

- Deny all private ranges by default: safer for hosting, unacceptable friction for the target
  homelab user.
- Allow everything: unacceptable SSRF surface.

## References

PRD §4.3 SSH-024, §6.4; AGENTS.md.
