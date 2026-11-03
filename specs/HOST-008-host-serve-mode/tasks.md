---
tags: [spec, tasks]
created: "2026-10-30"
---

# Tasks - HOST-008-host-serve-mode

> TDD order. One task = one focused commit. Tick as you go. Frozen on 2026-11-02 (owner
> approved the three-PR plan and the AC6 channel, `host.mesh.health_addr`, in #525).
>
> **Inline markers:** `[P]` = no dependency on another unchecked task. `[AC<n>]` = serves
> acceptance criterion `n` of `proposal.md`.
>
> **Boundaries.** `internal/host` must not import `internal/config`, `internal/profile` or
> `internal/credential` (ADR-017 C9, `TestHostPackage_DoesNotImportUserStores`), so the
> tsnet node and the forwarding live in a new `internal/serve` package, which reuses
> `proxy.AcceptLoop` with a dialer to `127.0.0.1:<port>`. Every test runs against fakes
> (no tsnet login, no real SSH), as CLI-363 did; the live AC1, AC2 and AC4 runs gate on
> mlorentedev/kubelab#2862 and are recorded in `verification.md`.

## Setup

- [x] Worktree and branch from master: `feat/host-008-host-serve-mode`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions" (owner decisions 2026-11-01 and 2026-11-02)

## Implementation — PR 1 (`host.mesh` schema and the state directory guard)

### The schema (`internal/host/file.go`, `internal/host/file_test.go`)

- [x] [P] [AC5] Failing table test `TestLoadFile_Mesh`: a full `host.mesh` (`bootstrap_ssh`, `bootstrap_socks_addr`, `hostname`, `state_dir`, `auth_key_file`, `health_addr`) decodes into `FileHost.Mesh`; an unknown `host.mesh.x` key is named in the warning and ignored; a `host.mesh.<key>:` with no value errors naming it. Expected: FAIL, `Mesh` undefined.
- [x] [AC5] Add `FileMesh` (pointer fields), `meshKeys`, and the `host.mesh` level to `checkKeys`. Expected: PASS.
- [x] [AC5] Failing table test `TestLoadFile_MeshInvalid`: each key `""` errors naming it; `bootstrap_ssh` `-oProxyCommand=x`, `user@` and `host:0` error (same rules as `connect --bootstrap-ssh`); `hostname` with a space or longer than 63 errors; a relative `state_dir` or `auth_key_file` errors; `health_addr` `0.0.0.0:9099`, `10.0.0.1:9099` and `127.0.0.1:0` error (loopback only, port 1-65535), and `bootstrap_socks_addr` follows the same loopback rule; `bootstrap_ssh` without `host.control_url` errors naming both; `host.mesh` on darwin errors citing #459. Expected: FAIL.
- [x] [AC5] Implement `validateMesh`; export `bootstrapssh.ValidateEndpoint` (wraps `parseEndpoint`) so file and flag share one rule; add `serve bool` to `osCapability` (windows, linux). Expected: PASS, and `go test ./internal/host ./internal/bootstrapssh -count=1` green.

### The state directory (`internal/host/statedir.go`, `statedir_windows.go`, `statedir_unix.go`)

- [x] [P] [AC5] Failing table test `TestCheckUnixStateDir` (pure, runs on every OS): a directory with mode `0700` and uid 0 is accepted; `0750`, `0707`, uid 1000, and a regular file are refused, each naming the path and what is wrong. Expected: FAIL.
- [x] [AC5] Implement `checkUnixStateDir(info fs.FileInfo, uid uint32) error` and `CheckStateDir(path)` on Unix (stat + `Sys().(*syscall.Stat_t).Uid`). Expected: PASS.
- [x] [AC5] Failing tests (`//go:build windows`): `TestCheckWindowsDACL` feeds SDDL descriptors to the DACL check: SYSTEM and Administrators only, deny ACEs, and CREATOR OWNER on an inherit-only ACE are accepted; any allow grant to another SID (read included, and inherit-only ones, which reach the node key inside), an owner outside SYSTEM/Administrators, and a null DACL are refused naming the SID. `TestCheckStateDir_Windows`: a `t.TempDir()` (the user's own directory) is refused naming the user's SID, through the real `GetNamedSecurityInfo` read. `TestCheckStateDir_Junction`: a junction above `state_dir` is refused and nothing is created through it. Expected: FAIL.
- [x] [AC5] Implement the Windows check with `windows.GetNamedSecurityInfo`: refuse any allow ACE with a nonzero mask for a SID other than SYSTEM or Administrators (inherit-only included; only CREATOR OWNER on an inherit-only ACE passes), an owner outside them, and a link or junction at `state_dir` or above it (review of #556). Expected: PASS on the `test-windows` CI job.
- [x] [AC5] Failing test `TestEnsureStateDir`: an absent path is created and then passes `CheckStateDir` (Linux `0700`; Windows protected DACL SYSTEM + Administrators); an existing loose directory is refused, never repaired. Expected: FAIL, then implement `EnsureStateDir`. Expected: PASS. (Linux, unprivileged: the test still asserts the created mode `0700` and that a first and a second run refuse the directory for its owner, unchanged; only the uid-0 acceptance needs root.)

- [x] PR 1 close: `go test ./internal/host ./internal/bootstrapssh`, `go vet` for linux/windows/darwin; `test`, `test-windows`, `lint` and `security` CI jobs green.

## Implementation — PR 2 (`host serve`: persistent node, declared services only)

### Forwarding (`internal/serve/publish.go`, `internal/serve/publish_test.go`)

- [x] [P] [AC3] Failing test `TestPublish_OnlyDeclaredPorts`: with a fake `Listener` (`Listen(network, addr string) (net.Listener, error)`, the `tsnet.Server` method), `Publish` for `[{rdp 3389} {ssh 22}]` listens on exactly `:3389` and `:22`. Expected: FAIL, package missing.
- [x] [AC3] Failing test `TestPublish_ForwardsToLoopback`: a connection accepted on the fake `:3389` reaches a local echo server standing in for `127.0.0.1:3389` (dialer injected), and bytes round-trip; `Published.Close()` stops every accept loop and the port refuses afterwards. Expected: FAIL.
- [x] [AC3] Implement `Publish(node, services, dialer, base config.Config, logger) (*Published, error)`: one `proxy.AcceptLoop` per service with a dialer to `127.0.0.1:<port>`; `Published.Close()` is the shutdown (review of #558). Expected: PASS.

### Enrolment (`internal/serve/node.go`)

- [x] [P] [AC2] Failing table test `TestNodeConfig_Enrolment`: with no `tailscaled.state` in `state_dir`, the auth key is read from `auth_key_file` and `enrolled=true`; with state present, the key file is never opened (a missing or `0000` file is fine) and `enrolled=false`; no state and no `auth_key_file` errors naming it. The `tsnet.Server` it builds has `Ephemeral: false`, `Dir: state_dir`, `Hostname` and `ControlURL` from the file. Expected: FAIL.
- [x] [AC2] Implement `nodeConfig(mesh, controlURL) (tsnetServerConfig, enrolled bool, err error)`. Expected: PASS.

### The command (`cmd/cli/host_serve.go`, `cmd/cli/host_serve_test.go`)

- [x] [AC5] Failing table test `TestHostServeCmd_Requires`: without `--config`, without `host.mesh`, or without `host.control_url`, serve exits 1 naming what is missing and starts nothing; a loose `state_dir` (fake `CheckStateDir` error) exits 1 naming it. Expected: FAIL, command missing.
- [x] [AC1] [AC2] Failing test `TestHostServeCmd_Ready`: with a fake node starter, serve prints `READY node=<hostname> services=rdp:3390 enrolled=true` on stdout (then `enrolled=false` with state present) and publishes the resolved services; `TestHostServeCmd_Bootstrap` (fake tunnel and control proxy) covers `bootstrap_ssh` on the default and the declared SOCKS address and `ssh_bootstrap_failed` when the tunnel cannot start; `TestHostServeCmd_PublishFailure` covers `publish_failed`; the help defines `enrolled=` (`TestHostServeCmd_HelpDefinesEnrolled`). Expected: FAIL.
- [x] [AC1] [AC2] Implement `host serve --config`: `LoadFile`, `Resolve`, `EnsureStateDir`, the CLI-363 bootstrap when `bootstrap_ssh` is set (SOCKS on `bootstrap_socks_addr`, default `127.0.0.1:1055` as `connect`), `tsnet.Server.Up`, `Publish`, `READY`; init failures emit `ERROR reason=...` as `connect`. Expected: PASS.
- [x] [AC7] Failing test `TestHostServeCmd_RefusesUntaggedNode`: when the fake node's status after `Up` has no tags, serve emits `ERROR reason=node_not_tagged`, prints no `READY`, never calls `Publish`, closes the node and exits 1. Expected: FAIL, then implement the check on `status.Self.Tags`. Expected: PASS.
- [x] [AC1] Wiring test `TestRootCmd_ReachesHostServe`: `NewRootCmd().Find(["host","serve"])` resolves the serve command under `host`, with its local `--config` (#279 / ADR-013 class; review of #525). Expected: FAIL until registered.
- [x] [AC5] Extend `TestHostCmds_NeverReadUserStore` to `host serve` (fake node): no store read under the malformed-store fixture. Expected: PASS.

- [x] PR 2 close: `go test ./...`, cross-GOOS vet, `smoke.bats` green; CI `test` (`-race`), `test-windows`, `lint`, `security` green.

## Implementation — PR 3 (reconnect, health and `host check`)

### Reconnect (`internal/serve/bootstrap.go`)

- [x] [P] [AC4] Failing test `TestSupervisedBootstrap_Retries`: with a fake tunnel starter and a fake sleep, a tunnel that exits after `READY` emits `ERROR reason=ssh_bootstrap_failed` once per outage, marks health not ready with that reason, restarts with backoff from 1 s doubling to 30 s with jitter (`DialBackoffBase`/`DialBackoffMax` defaults), and on a successful restart marks health ready and resets the backoff; a cancelled context stops it. Expected: FAIL.
- [x] [AC4] Implement the supervisor; `host serve` uses it instead of `monitorBootstrap`'s cancel-on-exit. Expected: PASS.
- [x] [AC4] Command tests: `TestHostServeCmd_Bootstrap` (PR 2: a tunnel that cannot start emits `ssh_bootstrap_failed`, starts no node, exits 1) and `TestHostServeCmd_BootstrapRestarts` (a tunnel that dies after `READY` gives one `ERROR` line, `/health/ready` 503 with `ssh_bootstrap_failed`, a restart and 200 again, serve still running) (review of #525).
- [x] [P] [AC4] Failing test `TestServeHealth_ControlSession`: with a fake node-state source (the IPN bus states `tsnet` reports), a state other than `Running` after `READY` marks `/health/ready` not ready with `reason=control_plane_unreachable`, and `Running` again marks it ready; serve itself does not reconnect (tsnet's control client retries). Expected: FAIL, then implement the watcher over `LocalClient.WatchIPNBus`. Expected: PASS.

### Health (`internal/health`, `internal/serve`)

- [x] [P] [AC6] Failing test `TestStartServer_Services`: with a services provider, `/health/ready` adds `node: <hostname>` and `services: [{kind, port, published}]` to its body in both the ready and not-ready states; without one, the body is unchanged (connect's contract). Expected: FAIL.
- [x] [AC6] Implement the optional provider; `host serve` starts the server on `host.mesh.health_addr` when declared and reports a service `published` once its listener is up. Expected: PASS.

> **Split (2026-11-02, ~300 LOC cap):** the tasks above land in PR 3 (reconnect and health, wired into `host serve`). `host check`, the auth key file guard and the docs below land in PR 4, which carries `Closes #522`.

### `host check` (`cmd/cli/host.go`)

- [x] [AC6] Failing table test `TestHostCheckCmd_Published`: with `health_addr` declared and a fake serve health endpoint, `check --json` reports `published: true|false` per declared service; with the endpoint down, or answered by a responder whose body does not name the file's node, every service is `published: false`; without `health_addr` the field is absent. Expected: FAIL.
- [x] [AC6] Implement the probe (loopback GET, 2 s timeout) and `serviceJSON.Published *bool` (`omitempty`). Expected: PASS.

### The auth key file (`internal/host/statedir*.go`)

- [x] [P] [AC5] Failing tests `TestCheckUnixSecretFile` (pure: a root-owned regular file `0600`/`0400` is accepted; group or other bits, another owner, a directory or a link are refused) and, on Windows, `TestCheckWindowsSecretDACL` (SDDL: SYSTEM and Administrators only accepted; a read or write grant to another SID refused). Expected: FAIL, then implement `CheckSecretFile`, reusing the DACL walk with a read-and-write mask, and call it in `serve.NewNodeConfig` before reading the key. Expected: PASS.

### Docs

- [x] `docs/runbooks/host-serve.md`: the Windows Scheduled Task (SYSTEM, at startup, restart on failure) and the Linux systemd unit, the protected `state_dir`, deleting the auth key file after `enrolled=true`. `site/src/content/docs/cli-reference.md` (`host serve`), `configuration.md` (`host.mesh`), README host section, `docs/design/host-client-parity.md`. If PR 3 passes ~300 LOC, the docs move to a PR 4.

- [ ] PR 3 close: `go test ./...`, cross-GOOS vet, `smoke.bats`; CI green.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command (each greps its `--- PASS:` line)
- [ ] Live acceptance after mlorentedev/kubelab#2862: AC1, AC2 and AC4 on the lab host, evidence in `verification.md`
- [ ] Lint passes (CI `lint`; golangci-lint is blocked on the maintainer's workstation)
- [x] No unrelated changes in the diff (no scope creep)
- [ ] `verification.md` filled in
- [x] PRs reference this spec folder (`Closes #522` on the last one only)
