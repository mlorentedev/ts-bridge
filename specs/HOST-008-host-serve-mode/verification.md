---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - HOST-008-host-serve-mode

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] AC1 (enrol, READY enrolled=true, live SSH/RDP handshake) -> PRs #558/#559 / tests `TestNodeConfig_Enrolment`, `TestPublish_ForwardsToLoopback`, `TestHostServeCmd_Ready`, `TestRootCmd_ReachesHostServe` / live run pending mlorentedev/kubelab#2862
- [ ] AC2 (restart without the key, enrolled=false, one node) -> PR #558 / tests `TestNodeConfig_Enrolment` (state present never opens the key), `TestHostServeCmd_Ready` / live node count pending mlorentedev/kubelab#2862
- [x] AC3 (only declared ports answer) -> PR #558 / tests `TestPublish_OnlyDeclaredPorts`, `TestPublish_ForwardsToLoopback`, `TestPublish_ListenFailureClosesOpened`
- [ ] AC4 (bootstrap and control-session loss, retry, recovery) -> PRs #558/#559 / tests `TestHostServeCmd_Bootstrap` (start failure), `TestSupervisedBootstrap_Retries`, `TestHostServeCmd_BootstrapRestarts`, `TestServeHealth_ControlSession` / live sinkholed run pending mlorentedev/kubelab#2862; the live run must also show `/health/ready` going not ready when the control plane drops, since the IPN bus may stay `Running` (#565)
- [x] AC5 (host.mesh keys, state_dir and auth_key_file guards) -> PRs #556/PR 4 / tests `TestLoadFile_Mesh`, `TestLoadFile_MeshInvalid`, `TestCheckUnixStateDir`, `TestCheckStateDir_Symlink`, `TestEnsureStateDir`, `TestCheckWindowsDACL`, `TestCheckStateDir_Windows`, `TestCheckStateDir_Junction`, `TestCheckUnixSecretFile`, `TestCheckWindowsSecretDACL`, `TestHostServeCmd_Requires`. On Windows `TestEnsureStateDir` needs an elevated token (CI `test-windows`); unelevated it skips and f5 fails by design.
- [x] AC6 (host check published, trusted responder) -> PRs #559/PR 4 / tests `TestStartServer_Services`, `TestHostServeCmd_Health`, `TestHostCheckCmd_Published`
- [x] AC7 (untagged node refused) -> PR #558 / test `TestHostServeCmd_RefusesUntaggedNode`
## Test status

- Test suite: `go test ./... -count=1` -> all 14 packages ok (Windows, 2026-11-02); `go vet ./...` clean for windows, linux, darwin; `features.json` f1-f4, f6, f7 pass; f5 passes except `TestEnsureStateDir` on an unelevated Windows token (Linux run in WSL passed).
- Manual smoke test: none against a live tailnet yet; mutation probes via `dotf pr reproduce` recorded in each PR's `## Review triage`.
- No regressions in existing test suite: yes
## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- `enrolled=` in READY means "this start used the auth key", so provisioning knows when to delete the key; defined in the help (review of #558).
- A bootstrap that fails before READY exits 1 for the OS supervisor; one that dies after READY is retried in place (review of #525).
- `host check`'s `published` trusts a responder naming the file's node: a diagnostic, not a security boundary, since no secret is reachable by an unelevated check and not by a squatter (review of #525).
- The Windows DACL check refuses any untrusted access, inherit-only ACEs included, and links or junctions on the path (review of #556).
- PR 3 split into PR 3 (reconnect, health) and PR 4 (`host check`, key guard, docs) for the ~300 LOC cap.
## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/HOST-008-host-serve-mode/` -> `specs/archive/HOST-008-host-serve-mode/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
