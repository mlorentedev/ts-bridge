---
id: "HOST-008-host-serve-mode"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-10-30"
issue: "mlorentedev/ts-bridge#522"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, headscale, windows]
template_version: "1.0"
---

# HOST-008: host serve mode

## Why

<!-- from issue #522: HOST-008: persistent host mode that publishes local services through the SSH-bootstrapped control plane -->

A lab host on a network that DNS-sinkholes and TLS-inspects a self-hosted control plane
cannot join that tailnet. Native Tailscale has no `--bootstrap-ssh`, and ts-bridge only
runs as an ephemeral client (`cmd/cli/run.go`, `Ephemeral: true`) that publishes nothing.
Measured on 2026-10-30: a host on such a network created a profile that never
authenticated, while clients on the same network reach the same control plane through
`connect --bootstrap-ssh` (CFG-005 live acceptance, 2026-10-20). Without this, a host
there can only stay on SaaS (lesson-016), and every new host repeats a manual migration
that cannot finish.

## What

`ts-bridge host serve --config <path>`: a foreground process, like `connect`, that joins
the tailnet as a persistent node and publishes the host's declared services. It runs on
Windows and Linux; on macOS a host file with `host.mesh` is refused citing #459, which
owns the macOS host.

- Reads the ADR-017 host file (`--config`, HOST-005). It publishes exactly the services
  `host.services` declares (kind and port), each forwarded from the tailnet to
  `127.0.0.1:<port>`. Nothing else answers on the node. On Windows and Linux this
  includes an already-provisioned SSH service as well as RDP: publishing SSH does not
  require the OS convergence steps tracked by #521. The convergence loader stays
  strict: setup/check reject SSH until those steps exist, and init remains an
  RDP-only generator. Serve does not install or configure OpenSSH. VNC and macOS
  support are unchanged.
- The control plane is the file's `control_url`. A new optional `host.mesh` block holds
  what serve alone needs: `bootstrap_ssh` (`user@host[:port]`, CLI-363),
  `bootstrap_socks_addr` (loopback, default `127.0.0.1:1055` as `connect`; it must differ
  from a bootstrap-enabled `connect` on the same host), `hostname`, `state_dir`,
  `auth_key_file` and `health_addr` (loopback only). Unknown keys warn and empty keys
  error, as HOST-005. Serve requires `host.control_url`, `host.mesh` and
  `host.mesh.state_dir`, and refuses to start without them; `auth_key_file` is required
  only for the first enrolment (no node state yet); `hostname` defaults to the OS
  hostname in lower case; `bootstrap_ssh` and `health_addr` are optional, and
  `bootstrap_socks_addr` defaults as above. AC6's trust rule compares against that
  resolved hostname.
- Persistent node: `Ephemeral: false`, node state in `state_dir`. A restart reuses the
  node identity without a key. The auth key file is read only when no state exists, and
  serve reports whether it used it (`READY ... enrolled=true|false`).
- Control traffic and a same-host DERP go through the CLI-363 bootstrap (system OpenSSH
  dynamic SOCKS plus the loopback HTTP CONNECT adapter, BUG-031). Other DERP regions keep
  today's behaviour.
- Signals and health as `connect`: `READY` on stdout, `ERROR reason=...` on stderr,
  `/health/ready`. When the SSH child dies after `READY`, serve restarts it with backoff
  instead of exiting. When the node's control session drops (tsnet state leaves
  `Running`; tsnet's control client already retries), `/health/ready` reads not ready
  with `reason=control_plane_unreachable` until it is back.
- `host check --json` reports, per declared service, whether serve publishes it, read
  from serve's `/health/ready` on `host.mesh.health_addr`. That body names the node
  (`"node": "<hostname>"`), and `check` trusts only a response that names the file's
  node; anything else on that port reads as serve not running.
- Installing serve as a boot-time task is the provisioning layer's job (ADR-017 keeps
  ts-bridge daemonless): the docs give the Windows Scheduled Task (SYSTEM, at startup,
  restart on failure) and the Linux systemd unit, and the consumer's provisioning
  script registers them.

## Out of scope

- An OS service, supervisor or daemon inside ts-bridge (#451). Serve is foreground by
  contract; the OS starts and restarts it.
- Publishing anything that `host.services` does not declare, UDP, or subnet routes.
- Changes to `connect`, profiles or the per-user credential store (ADR-017 C9: host
  commands never read them).
- Native-Tailscale hosts: still supported, untouched.

## Risks / open questions

- **ADR-017 boundary (decided 2026-11-01).** ADR-017 is "daemonless host setup", and its
  consequences put "installing the bridge as an OS service" in #451. The operator accepted
  ADR-017 Amendment A in #525: serve is a foreground host command, the OS supervises it,
  and #451 keeps the question of a service inside the binary.
- **Bootstrap identity on the SSH endpoint (gates live acceptance only, decided
  2026-11-01).** Today the forward works only because the live control host allows it for
  the operator's account, while its IaC template forbids it (mlorentedev/kubelab#2862).
  A host must get its own narrow identity (forward to the control plane only) before serve
  depends on it in production. The operator decided in #525 that #2862 gates the live AC4
  run and the archive, not the tasks: the PRs test against a fake SSH/SOCKS endpoint, as
  CLI-363 did.
- **State and key protection.** `state_dir` holds the node key and the SSH identity
  sits beside it. Windows: under ProgramData, ACL SYSTEM and Administrators only; Linux
  `0700` root. The auth key file is deleted by the provisioning script after
  `enrolled=true`, never by serve. It holds a live tagged key, so serve refuses an
  `auth_key_file` that others may read: on Unix a regular file owned by root with no
  group or other bits; on Windows one whose DACL grants read or write only to SYSTEM and
  Administrators (review of #525; PR 3).
- **Tag ownership (enforced, 2026-11-02, from the #525 review).** The node must enrol with
  the key's tag (Headscale tagged node), not as a user node; a key without a tag would put
  a host under a user identity. Serve checks the node after `Up`: a node without tags
  emits `ERROR reason=node_not_tagged`, prints no `READY`, publishes nothing and exits 1.
- **Reconnect storms (decided 2026-11-01).** Serve reuses `connect`'s values, so that no
  new numbers are invented: exponential backoff with jitter between `DialBackoffBase`
  (1 s) and `DialBackoffMax` (30 s), from `internal/config`, and readiness on
  `/health/ready` as `connect` does. The operator chose this in #525.
- **How `host check` learns what serve publishes (decided 2026-11-02).** `check` runs
  unelevated and `state_dir` is readable only by SYSTEM/Administrators or root, so a
  status file there is out. The operator chose an optional loopback-only
  `host.mesh.health_addr`: serve serves `/health/ready` there with a `services` list, and
  `check` reads it when the key is declared. Serve down means every service reads as not
  published; without the key, `check` omits the field. Rejected: a fixed default address
  (port collisions) and moving AC6 to its own ticket.
  Limit (review of #525): matching the node name proves the responder speaks serve's
  contract, not that it is serve. A local process that binds `health_addr` while serve is
  stopped could answer for it. A shared secret does not help, since `check` runs
  unelevated and anything it can read the squatter can too, so `check`'s `published` is a
  diagnostic, not a security boundary; the published ports themselves are what the
  tailnet ACL governs.
- **~300 LOC cap (decided 2026-11-02).** Four PRs: the `host.mesh` schema and the
  `state_dir` guard, the serve command with forwarding and enrolment, reconnect and
  health, then `host check`, the auth key guard and the docs.
- **Serve/convergence validation (correction authorized 2026-11-06).** The pinned
  lab candidate rejects SSH before node startup because it uses the OS step capability
  table, although `Publish` forwards TCP and the lab already runs OpenSSH. Give serve
  its own loader over the same decoding and validation rules, permitting SSH only on
  OSes that already support serve. Do not widen the setup/check/init capability table
  or implement #521. Until #521 lands, a combined SSH/RDP file cannot be used with
  `host check`; inspect serve's health endpoint for its publication diagnostic and
  keep the combined-file part of AC6 open.

## Acceptance criteria

AC1, AC2 and AC4 need the live control plane, so they gate on mlorentedev/kubelab#2862 (see Risks). AC3, AC5, AC6 and AC7 run against fakes.

- [ ] AC1: with a fresh `state_dir` and a tagged one-off key, serve enrols, prints
  `READY ... enrolled=true`, and an authorized tailnet client completes an SSH and an RDP
  handshake through the published ports.
- [ ] AC2: restarted with the key file removed, serve prints `READY ... enrolled=false`
  and the control plane still lists exactly one node for the host, with the same
  identity.
- [ ] AC3: a port that `host.services` does not declare refuses connections through the
  tailnet.
- [ ] AC4: with the control hostname unresolvable locally, enrolment and reconnect
  succeed through `bootstrap_ssh`. With the SSH endpoint down at start, serve emits `ERROR
  reason=ssh_bootstrap_failed` and exits 1, so the OS supervisor restarts it (Amendment
  A); when it goes down after `READY`, serve emits that line once, keeps retrying, and
  recovers when the endpoint returns. While the control session is down, `/health/ready`
  reads not ready with `reason=control_plane_unreachable`.
- [ ] AC5: a `host.mesh` block with an unknown key warns, an empty key errors, and a
  `state_dir` writable by non-administrators is refused on Windows, and one that is not
  `0700` and owned by root is refused on Linux; an `auth_key_file` that others may read is refused
  on both.
- [ ] AC6: with `host.mesh.health_addr` declared, `host check --json` reports, for each
  declared service, whether serve publishes it, and a service that serve does not
  publish (or serve not running, or a responder that does not name the file's node)
  reads as not published.
- [ ] AC7: a node that comes up without tags is refused: `ERROR reason=node_not_tagged`,
  no `READY`, nothing published, exit 1.

## References

- Issue: mlorentedev/ts-bridge#522
- ADR-017 (host desired state), ADR-005 (Headscale compatibility), ADR-016
- Specs: CLI-363 (SSH bootstrap), BUG-031 (DERP through the bootstrap), HOST-005 (host
  file), CFG-005 (live bootstrap acceptance)
- Lessons: lesson-014, lesson-016
- Consumer: a lab provisioning script that runs one idempotent command per host
- Dependency: mlorentedev/kubelab#2862 (VPN-ACL-015)
