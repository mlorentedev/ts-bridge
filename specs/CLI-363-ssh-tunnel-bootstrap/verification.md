---
tags: [spec, verification, templates]
created: "2026-09-28"
---

# Verification - CLI-363-ssh-tunnel-bootstrap

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] Criterion 1 -> `TestMergeBootstrapSSHPrecedence`, `TestMergeBootstrapSSHDefaultsToLoopback`, `TestMergeRejectsInvalidBootstrapConfig`, `TestDecodeYAMLBootstrapSSH`, `TestCollectFlagsBootstrapSSH`, and `TestBootstrapSSHFlagsReachRunner`.
- [x] Criterion 2 -> `TestBuildSSHArgs`, `TestControlProxySelectsOnlyControlHostname`, and `TestControlProxyPreservesHostnameThroughSOCKS`.
- [x] Criterion 3 -> `TestStartWaitsForSOCKSAndClosesProcess`, `TestStartReportsEarlyExit`, `TestStartReportsCleanEarlyExit`, `TestStartReportsMissingOpenSSH`, `TestRunBootstrapFailureStopsBeforeTailscale`, and the canceled-startup READY guard.
- [x] Criterion 4 -> `TestBootstrapSSHDocumentation` plus updates to `.env.example`, README, site configuration/CLI reference, ADR-005, and lesson-014.

## Test status

- Test suite: `go test ./...` -> all packages passed.
- Static checks: `go vet ./...` -> passed; `golangci-lint run` -> `0 issues`.
- Builds: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/ts-bridge/` and `GOOS=windows go build ./cmd/ts-bridge/` -> passed.
- Documentation: `npm --prefix site run build` -> 5 pages built successfully (existing i18n/404 warnings only).
- Security: `gosec ./...` found no issue in changed code; it still reports the pre-existing G204 in `internal/host/host_windows.go:235`.
- Simulated smoke: `TestControlProxyPreservesHostnameThroughSOCKS` proves the SOCKS peer receives `vpn.example.com`, not a locally resolved address, while the request URL remains HTTPS.
- Real filtered-network smoke: not run in this environment; issue #363 accepts a simulated equivalent. Re-run #183 from the affected or an unrestricted network after merge.
- No regressions in existing test suite: yes.
- Pull requests: #372 (OpenSSH core), #373 (configuration), #374 (CLI wiring/docs; closes #363).

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- Dynamic SOCKS (`ssh -D`) replaces the issue's preliminary local-forward recipe because it moves DNS resolution to the SSH server while preserving the original hostname for TLS validation; no hosts-file mutation or custom CA is required.
- The Tailscale proxy hook is configured before `tsnet.Server.Up` and selects only the configured control-plane hostname. Existing environment/system proxy handling remains the fallback for all other URLs.
- OpenSSH owns authentication and host-key verification. ts-bridge does not add password flags, disable verification, or shell-interpret the endpoint.

## Adversarial review round 1

- **F1 — proxy installation seam (REAL Major): applied.** `TestStartConfiguredBootstrapConfiguresControlProxy` injects the SSH starter and proxy configurer and fails if the production seam stops installing the control-plane proxy.
- **F2 — shutdown closer (REAL Major): applied.** `TestStartBootstrapLifecycleCloserClosesTunnel` fails if the lifecycle cleanup stops closing the SSH child.
- **SOCKS listener collision (Minor): applied.** `TestMergeRejectsBootstrapListenerCollision` rejects overlap with the bridge listener.
- **Shutdown ordering (Minor): applied.** The run context is canceled before the deferred SSH close, preventing a normal cleanup from being classified as an unexpected exit.
- **Unauthenticated local SOCKS pivot (Minor): documented.** ADR-005 now states the local-user/process exposure and trusted-host requirement.
- **Hostname-only proxy selection, process-global hook, TLS wording, and separate-ADR suggestion:** declined for this change. Hostname-only selection is the documented contract; the binary is single-run; the TLS statement remains accurate; ADR-005 is the accepted home for Headscale control-plane reachability.

## Adversarial review round 2

- **Direct hook body (REAL Major): applied.** `TestConfigureControlProxyInstallsHook` calls the real `ConfigureControlProxy` and asserts `tshttpproxy.ProxyFromEnvironment` selects the SOCKS URL.
- **Production lifecycle close (REAL Major): applied.** `withBootstrapLifecycle` is the production wrapper called by `run`; `TestWithBootstrapLifecycleClosesTunnelAfterRun` fails if its deferred close is removed.
- **Git attribution finding:** declined. The Copilot trailer is required by the active session's explicit commit policy and cannot be removed by a lower-priority reviewer rule.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: `docs/lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md`
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: `docs/adr/adr-005-headscale-compat.md`
- [x] New pattern candidate for `00_meta/patterns/`? no: this is a tsnet/Headscale-specific integration, not yet observed across projects.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CLI-363-ssh-tunnel-bootstrap/` -> `specs/archive/CLI-363-ssh-tunnel-bootstrap/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
