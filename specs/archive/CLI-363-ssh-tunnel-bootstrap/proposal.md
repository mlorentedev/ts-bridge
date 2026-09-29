---
id: "CLI-363-ssh-tunnel-bootstrap"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-09-28"
issue: "mlorentedev/ts-bridge#363"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CLI-363-ssh-tunnel-bootstrap

> **Naming**: file lives at `<repo>/specs/CLI-363-ssh-tunnel-bootstrap/proposal.md`. `CLI-363-ssh-tunnel-bootstrap` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #363: connect: support an SSH-tunnel bootstrap when the control plane is network-blocked -->

En redes corporativas que bloquean o interceptan el dominio del control plane, ts-bridge no puede completar tsnet.Up() aunque SSH hacia el servidor siga disponible. Sin un bootstrap SSH soportado, el usuario debe ejecutar un túnel manual, modificar hosts y sobrescribir TS_CONTROL_URL, un procedimiento frágil y difícil de automatizar que mantiene bloqueada la validación E2E de #183.

## What

`ts-bridge connect --bootstrap-ssh <user@host[:port]>` (or `TS_BOOTSTRAP_SSH`) starts the system OpenSSH client as a local dynamic SOCKS proxy before `tsnet.Server.Up`. Tailscale control-plane requests for the configured `ControlURL` use that proxy, so hostname resolution occurs from the SSH server and the original HTTPS hostname remains intact for SNI and certificate verification. The command waits for the proxy to become ready, reports actionable startup failures, monitors the SSH process for premature exit, and stops it when ts-bridge shuts down.

The SOCKS listener defaults to a loopback-only address and can be overridden through the normal flag, environment, and YAML precedence chain. Existing connections without SSH bootstrap behave exactly as before.

## Out of scope

- Automatic detection of filtering or transparent fallback; bootstrap remains explicit and fail-loud.
- General mesh SOCKS5 access, browser routing, or arbitrary application proxying (ARCH-186).
- Bundling an SSH client, disabling SSH host-key verification, managing SSH credentials, or editing the OS hosts file.
- Named auth-key files and credential selection per profile, tracked by #355.

## Risks / open questions

- **Resolved — TLS/DNS integrity:** use OpenSSH dynamic forwarding (`ssh -D`) and route only the control-plane hostname through SOCKS. The original `ControlURL` is not rewritten, so remote DNS bypasses the local sinkhole while TLS SNI and certificate validation remain unchanged.
- **Resolved — dependency boundary:** invoke the system `ssh` executable with `os/exec`; add no Go dependency and fail clearly when OpenSSH is unavailable.
- **Resolved — proxy scope:** install the control-plane-specific proxy hook before tsnet starts and delegate all non-control URLs to the existing environment/system proxy behavior.
- **Resolved — lifecycle:** wait until the local SOCKS listener accepts connections, treat early SSH exit as a startup/runtime failure, and bind process cleanup to the bridge context.
- **Residual verification risk:** the corporate-network reproduction may not be available during implementation. A simulated SOCKS/DNS-path test is mandatory; the real #183 retry is recorded separately when an affected or unrestricted network is available.

## Acceptance criteria

Observable outcomes. Each must be testable.

- [ ] `bootstrap_ssh` and its loopback SOCKS address are accepted from YAML, `TS_BOOTSTRAP_SSH` / `TS_BOOTSTRAP_SOCKS_ADDR`, and `connect` flags with `flags > env > YAML > defaults` precedence; invalid endpoints, non-loopback listeners, or bootstrap without a custom control URL fail before tsnet starts.
- [ ] With bootstrap enabled, ts-bridge starts `ssh -N -D`, waits for its SOCKS listener, and sends only control-plane requests through it while retaining the original control-plane hostname for remote DNS and TLS verification; no hosts-file change is required.
- [ ] Missing OpenSSH, bind/startup failure, readiness timeout, and premature SSH exit produce actionable errors and never emit a false `READY`; normal shutdown terminates the SSH child.
- [ ] Go tests cover config, command construction, proxy selection, readiness, and lifecycle behavior on the production CLI path; `.env.example`, README/config docs, ADR-005, and lesson-014 document the supported mode and its security constraints.

## References

- Bitácora board: `mlorentedev/ts-bridge#363`
- Related ADRs: `docs/adr/adr-001-tsnet-userspace.md`, `docs/adr/adr-005-headscale-compat.md`, `docs/adr/adr-008-cli-architecture.md`, `docs/adr/adr-010-cli-package-layout.md`, `docs/adr/adr-013-cli-tests-in-go.md`, `docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`
- Related lesson: `docs/lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md`

<!-- archived 2026-09-29 — PR: https://github.com/mlorentedev/ts-bridge/pull/375 -->
