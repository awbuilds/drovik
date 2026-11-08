# 2026-11-08 Lima Docker and Keycloak OIDC E2E

## Task / Area
Test real Docker actions and a real OIDC provider through Drovik using Lima.

## Environment
- Second disposable Lima VM: Ubuntu 26.10, 2 CPUs, 3 GiB RAM, 15 GiB sparse disk,
  `--mount-none`; no host filesystem was mounted.
- Docker 29.7.2 installed inside the VM; `busybox` test container.
- Keycloak 26.8.2 served from a container inside the VM, exposed only through a
  localhost SSH tunnel.
- Drovik ran locally with a temporary SQLite DB and a temp encryption key.

## Results
- Docker: list, logs, inspect, stop, start, restart and remove all passed.
- OIDC: Keycloak realm/client/user created through the admin API; Drovik linked the
  local admin account, completed the authorization-code + PKCE flow, and signed in
  through Keycloak without a local password. `OIDC E2E PASSED`.
- The OIDC link callback correctly required the same Drovik session and state cookie.
- Audit export, teams and snippets were previously tested against Lima in the first
  live run; this run did not repeat them.

## Bug found and fixed
- **Removing a running Docker container failed.** The UI offers remove for running
  and exited containers, but the command was `docker rm <id>`, which Docker refuses
  for a running container. After the explicit confirmation, Drovik now runs
  `docker rm -f <id>`. Regression test added.

## Not tested
- FreeBSD under Lima: the template requires QEMU (`vmType: qemu`), which is not
  installed on this Mac. Installing QEMU would change the local system, so it was
  not done. The existing FreeBSD parser fixture remains the only FreeBSD coverage.
- OpenBSD/NetBSD have no Lima templates.
- A hosted OIDC tenant (Okta/Entra/Google), rootless Docker/Podman, and a real cloud
  network still need separate validation.

## Cleanup
The Lima VM, its disk, the Docker/Keycloak containers and the temporary Drovik DB
were deleted. The localhost tunnel was stopped. Colima and user data were untouched.
