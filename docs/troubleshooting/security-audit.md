---
id: "ts-bridge-security-audit"
type: audit
status: active
tags: [security, threat-model, supply-chain, audit]
created: "2026-05-18"
owner: manu
---

# ts-bridge: Security Audit

> Defensive document. Captures the trust boundaries, threat model, attack
> surface, and mitigations as of v1.6.0/v1.7.0. Re-audit whenever a new
> external dependency lands, the deployment model changes, or after any
> incident.

## Scope

`ts-bridge` is a single-binary TCP proxy that bridges a local listener to a
target host reachable over a Tailscale (or Tailscale-compatible) mesh. It
runs in userspace via `tailscale.com/tsnet`, configured exclusively via
environment variables.

Out of scope:
- The host running native Tailscale on the target side — that node has its
  own threat model.
- The Tailscale (or Headscale) control plane.
- The OS, file system, and network stack the binary runs on.

---

## Trust boundaries

```text
    Operator                      Tailscale control plane
       |                                    |
       |  --auth-key-file / .env            |  Noise (TLS)
       v                                    v
  +--------+   loopback   +------------+    mesh    +-----------+
  | client | <----------> | ts-bridge  | <--------> |   target  |
  |  app   |   127.0.0.1  |  (tsnet)   |  100.x.x.x | (native   |
  +--------+              +------------+            |  Tailscale)
                                ^
                                |
                          file system: state dir, log files
```

Boundaries to defend (in order of blast radius):

1. **The operator-supplied auth key (`TS_AUTHKEY` / `--auth-key-file`).**
   Owner of this key can register a node on the tailnet under the operator's identity.
   Treat as a secret equivalent to an SSH private key.
   - *Hardened path:* `--auth-key-file /path/to/key` (stored in a `0600` file) prevents process environment inheritance and process list exposure.
   - *Plaintext risk:* `TS_AUTHKEY` in `.env`/environment is inherited by child processes; `--auth-key` is visible in the process table (`ps`, Task Manager).
2. **The local listener** at `TS_LOCAL_ADDR`. Default `127.0.0.1:33389`
   loopback-only. If misconfigured to a routable interface, anyone on
   that network can reach the target via the bridge.
3. **The state directory** (`TS_STATE_DIR` or auto-mode `os.TempDir()/...`).
   Holds the tsnet machine key and runtime logs. Compromise allows
   impersonation of the bridge instance.

---

## Assets

| Asset | Sensitivity | Storage |
|---|---|---|
| Auth key | High — equivalent to login credential | `--auth-key-file` (preferred, `0600` file), `.env` file on disk, or environment variable |
| Machine key (tsnet) | High — node identity | `<StateDir>/tailscaled.state` |
| Session traffic | Medium — confidentiality is delegated to WireGuard (Tailscale L4) | In-memory, transient |
| Logs | Low (no payloads); may contain peer IPs, target host:port | stdout, `<StateDir>/tailscaled.log*.txt` |

---

## Threat model (STRIDE)

| Threat | Vector | In scope? | Mitigation |
|---|---|---|---|
| **Spoofing** — attacker impersonates a legitimate bridge instance | Steals `TS_AUTHKEY`, registers their own node | Yes | Operator hygiene + Tailscale-side `Ephemeral`+`Reusable` key with TTL cap. Compromised key requires revocation + rotation. |
| **Tampering** — attacker modifies in-flight traffic | MITM between bridge and target | No | Out of scope — Tailscale's WireGuard handles confidentiality + integrity. Trust boundary ends at tsnet. |
| **Repudiation** — operator denies sending traffic through the bridge | n/a | No | The bridge does not produce auditable signed records; if needed, run an external session-logger. |
| **Information disclosure** — secrets leaked via logs | Verbose mode dumps tsnet internals | Yes | `TS_VERBOSE=false` by default; auth key is never logged (only its presence is implied by "tailscale ready"). |
| **DoS** — bridge crashes or refuses service | Connection flood, resource exhaustion | Partial | `TS_MAX_CONNECTIONS` (default 1000) caps in-flight conns. `TS_IDLE_TIMEOUT` reclaims zombies. No per-IP rate limit (single-target design). |
| **Elevation of privilege** — local user reads state | Other users on the same machine read `<StateDir>` | Yes | `stateDirPerms = 0700`; Unix-side enforcement only — Windows ACL behavior is OS-default (see Known Limitations). |

---

## Supply chain

### Dependencies

- **Single external Go dependency:** `tailscale.com/tsnet v1.80.0`. Pinned via `go.mod`. `go.sum` is checked in. CI runs `go mod verify`.
- **No transitive npm/PyPI surface** — pure Go, no container, no FFI.
- **stdlib for everything else** — `net`, `log/slog`, `sync/atomic`, `math/rand/v2`. No new deps added in v1.5.x or v1.6.x.

### Build process

- Built in CI by GitHub-hosted `ubuntu-latest` runners.
- Multi-arch matrix: `linux/{amd64,arm64}`, `windows/{amd64,arm64}`, `darwin/{amd64,arm64}`.
- LDflags strip debug info and embed version + commit:
  ```bash
  -s -w -X main.version=${VERSION} -X main.commit=${COMMIT}
  ```
  No private keys, paths, or secrets baked into the binary.
- Release artifacts: `.zip` (Windows) / `.tar.gz` (others), each containing
  binary + `.env.example` + `README.md` + `LICENSE` + `scripts/`. SHA-256
  checksums in `checksums.txt`.
- Distribution: GitHub Releases page, signed by GitHub's TLS to that
  endpoint. **No additional artifact signing (Sigstore/cosign).**
  Acceptable risk given operator manually downloads from a trusted URL
  and verifies checksum; documented for future improvement.

### CI / release automation

- `release-please-action@v4` opens release PRs based on Conventional Commits.
- Authenticated via a **fine-grained PAT** (`RELEASE_PLEASE_PAT` secret, see
  `CHANGELOG.md` entry 2026-05-18) scoped to this repo only with
  `contents: write` + `pull_requests: write`. Limits blast radius if the
  release-please-action upstream is compromised.
- Branch protection on `master`: required checks `test`, `lint`, `security`
  (gosec), `shellcheck`; strict up-to-date; no force-push.
- `gosec` runs as a separate step (containerised `securego/gosec@v2.23.0`)
  on every PR. Findings fail the build (see ARCH-004 PR #24 retry for an
  example).
- `GitGuardian` scans every commit for leaked secrets.

---

## Attack surface

### Input vectors

| Vector | Source | Validation |
|---|---|---|
| `TS_AUTHKEY` | env var / `.env` file | Format prefix check (`tskey-` / `hskey-`). Length not validated. Value never logged. |
| `TS_TARGET` | env var | `net.SplitHostPort` + port range 1-65535. |
| `TS_CONTROL_URL` | env var | Passed verbatim to tsnet. Tsnet validates as URL. |
| `TS_LOCAL_ADDR` | env var | `net.Listen` rejects malformed addresses. **Operator can bind to `0.0.0.0`** — that's a config error, not an injection. Document as a known risk. |
| `TS_STATE_DIR` | env var | Path is used in `os.MkdirAll` — no input sanitization. Operator is trusted not to point it at a system dir. |
| `TS_PORT_RANGE` | env var | Validated `START-END`, bounds-checked. |
| Other numeric env vars (`TS_*_TIMEOUT`, `TS_DIAL_*`, etc.) | env var | All validated via `parseDurationEnv` / `parseInt64Env` / `parseDialConfig` with negative-value rejection. |
| Incoming TCP connections | local listener | No per-conn auth — the bridge is a transparent proxy. Authentication is delegated to the target service (RDP NLA, SSH, etc.). |

### Egress vectors

- tsnet outbound to control plane (default Tailscale SaaS at `controlplane.tailscale.com`).
- tsnet outbound to DERP relays + magicsock peers as needed.
- The target host:port specified in `TS_TARGET`.
- **No telemetry or call-home** beyond Tailscale's own (which is part of using Tailscale, documented at tailscale.com/security).

---

## Mitigations in place

| Risk | Mitigation | Code |
|---|---|---|
| Auth key in `.env` checked into git | `.env` is `.gitignore`'d; `.env.example` ships a placeholder | repo root `.gitignore` |
| State directory readable by others | `os.MkdirAll(dir, 0700)` | `main.go:87` (`stateDirPerms`) |
| Idle connections holding handles | `TS_IDLE_TIMEOUT` configurable per-direction deadline | `internal/proxy/proxy.go` (`idleConn`, REL-003) |
| Transient dial failures cascade to user-visible error | `ReconnectDialer` with exponential backoff + jitter | `internal/proxy/reconnect.go` (ARCH-004) |
| Auth key bad/expired produces opaque error | Diagnostic WARN with remediation pointing to control plane admin | `main.go` `diagnoseTailscaleInitError` (v1.5.1, PR #18) |
| Backoff stampede on simultaneous reconnect attempts | Random jitter in `[d, d+d/2]` per attempt | `internal/proxy/reconnect.go` `computeBackoff` |
| `gosec` false positives bypass review | `// #nosec G404` (jitter) is the only inline exclusion, narrowly scoped + commented | `internal/proxy/reconnect.go:105` |
| Connection flood | `TS_MAX_CONNECTIONS` cap, reject + log when reached | `internal/proxy/proxy.go` `AcceptLoop` |
| Ephemeral node lingers on tailnet after crash | `tsnet.Server.Ephemeral=true` — control plane removes after disconnect | `main.go:204` |
| Tsnet workers leak on init failure | `server.Close()` on `Up()` error path | `main.go:initTailscale` (v1.5.1, PR #18) |
| Secrets in logs | `TS_VERBOSE=false` default; no `cfg.AuthKey` log statement anywhere | `main.go` (grep verified) |
| Inner secrets serialised | `Config.AuthKey` documented as `#nosec G117` no-serialize | `internal/config/config.go:31` |

---

## Operator responsibilities

These are NOT enforced by the bridge — the operator must internalise them.

- **Rotate `TS_AUTHKEY` on a calendar before expiry.** Tailscale SaaS caps
  reusable+ephemeral keys at 90 days. Schedule a reminder at TTL-7d.
  Affects every client running ts-bridge simultaneously, since `Ephemeral:
  true` consumes the key on every restart (see [lessons.md](../lessons.md)
  2026-05-18 entry).
- **Never commit `.env`.** Repo `.gitignore` covers it; verify before push.
- **Use loopback for `TS_LOCAL_ADDR`** unless you have a deliberate reason
  to expose the bridge on a routable interface. The default is correct.
- **Verify checksums on release downloads.** `sha256sum -c checksums.txt`
  against the downloaded archive before installing on clients.
- **Restrict access to `<StateDir>`** at the OS level. The bridge sets
  `0700` on creation; subsequent edits (e.g., antivirus quarantine,
  backup tools) might widen perms.

---

## Known limitations / acceptable risks

| Limitation | Rationale | Future work |
|---|---|---|
| No release-artifact signing (Sigstore/cosign) | Single-maintainer project, trust anchored in GitHub Releases TLS | Adopt cosign once cosign-action is stable in upstream catalogs |
| Windows state-dir ACLs are OS-default, not enforced | Cross-platform Unix-style perms unreliable on NTFS | Document; if hardening needed, call `icacls` from a setup script |
| No per-IP rate limit at listener | Single-target loopback design — DoS surface is the host itself | Add token-bucket if multi-tenant deployments emerge |
| `TS_AUTHKEY` lives in `.env` on disk (not in OS keychain) | Portability across Windows/Linux/macOS without a keystore dep | If a use case emerges, integrate with `keyring` lib (adds dep) |
| `isPermanentDialError` uses string match on `"tsnet: backend in state ..."` | tsnet does not expose a sentinel error; verified by reading source | Watch for tsnet upstream API change; integration test on tsnet version bump |
| Connection authentication delegated to target | Bridge is L4 transparent; design constraint | If needed, run mTLS at the target service layer |

---

## Audit checklist (run on every release)

- [ ] `go mod verify` clean
- [ ] `gosec ./...` clean (or every finding has `// #nosec` with rationale)
- [ ] `golangci-lint run` clean
- [ ] `git secrets` or equivalent run on the diff
- [ ] No `cfg.AuthKey` reference in any `logger.*` call (grep)
- [ ] `.env.example` shipped without real values
- [ ] Release tag matches the version embedded in `main.version` via ldflags
- [ ] `checksums.txt` published alongside artifacts
- [ ] CHANGELOG entry mentions any new env var or trust-boundary change
- [ ] This document re-reviewed if any of the above changed

---

## Related

- [adr-002-single-binary-no-config.md](../adr/adr-002-single-binary-no-config.md) — env-var-only config rationale
- [adr-005-headscale-compat.md](../adr/adr-005-headscale-compat.md) — `TS_CONTROL_URL` semantics
- [lessons.md](../lessons.md) entry 2026-05-18 — ephemeral mode + rotation cadence
- [error-auth-failure.md](error-auth-failure.md) — operator playbook on `API key does not exist`
- [error-state-permissions.md](error-state-permissions.md) — state-dir perms issues
- Changelog: versioned trust-boundary changes (see repo `CHANGELOG.md`)
