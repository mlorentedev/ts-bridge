---
tags: [spec, verification, templates]
created: "2026-10-15"
---

# Verification - CFG-005-self-contained-connections

## Evidence

AC1-AC4 have Windows package-test evidence from 2026-10-15. Full Windows tests,
vet, build, and ownership cross-builds passed on that checkout. On 2026-10-16
the operator authorized draft-PR publication and automated validation through
the existing GitHub Actions Linux/Windows runners. Published-revision CI,
independent review and live AC5 remain open; no real profile has been migrated
and no completion claim is made.

- [x] AC1 -> `TestProfileRuntimePreserved` covers reads, Set, descriptor Import, SetCredential, and ClearCredential. Existing profile/descriptor tests also pass.
- [x] AC2 -> `TestProfileRuntimeConnectPrecedence`, `TestProfileRuntimeImportPreservesExistingTargetAndCredential`, `TestProfileRuntimeImportRejectsUnsafeOrUnsupportedInput`, and `TestProfileRuntimeBrowserDoesNotInheritTCPTarget`. Repeated configuration leaves identical store bytes and does not ask to replace an existing credential.
- [x] AC3 -> `TestProfileLockLifecycle`, `TestProfileLockReleasedAfterProcessExit`, `TestProfileLockRejectsDuplicateBeforeRunner`, and `TestProfileLockAllowsOtherProfile`. Windows tests use actual OS locks and a forcibly terminated test child; nested connect/browser invocations cannot initialize logging or a second runner.
- [x] AC4 -> `TestStartReportsMissingOpenSSH`, `TestStartReportsEarlyExit`, and `TestStartReportsUnlaunchableOpenSSH`. Tests assert selected-client and IdentityFile/agent guidance, retained error causes, and inherited terminal streams. `TestStartWaitsForSOCKSAndClosesProcess` succeeds without SSH_AUTH_SOCK.
- [ ] AC5 -> Draft profile runbook and README link written; operator-assisted real-service validation deliberately deferred at handoff. The real profile and installed binary remain unchanged.

## Spec consistency

Coverage recorded during the first implementation batch; proposal and tasks existed before code changes. Deterministic `[ACn]` mapping covers AC1 (store tests/implementation), AC2 (CLI import/resolution), AC3 (ownership tests/implementation), AC4 (SSH diagnostics), and AC5 (runbook/live validation). No uncovered criteria or orphan implementation tasks. All five criteria have matching `features.json` entries. Verdict: PASS for plan coverage, not implementation completion.

## Test status

- Initial RED: `go test .\internal\profile .\cmd\cli -run 'TestProfileRuntime' -count=1` failed on discarded runtime fields, missing profile resolution, and the rejected `--config` import.
- Additional RED: unsupported versions and trailing YAML documents were initially accepted; secret material in a control URL and redundant credential-onboarding guidance were also caught before their fixes.
- Batch 1 GREEN: `go test .\internal\profile .\internal\config .\cmd\cli -count=1` -> all passed on Windows (`internal/profile` 2.460s, `internal/config` 0.853s, `cmd/cli` 19.228s).
- Ownership RED: `go test .\cmd\cli -run '^TestProfileLock' -count=1` showed duplicate connect/browser calls reaching runtime and logging three times.
- Ownership GREEN: `go test .\internal\profile .\cmd\cli -count=1` -> both passed on Windows.
- SSH diagnostics RED: `go test .\internal\bootstrapssh -run 'TestStartReports(MissingOpenSSH|EarlyExit|UnlaunchableOpenSSH)' -count=1` failed on missing selected-client/authentication/install guidance.
- Latest GREEN: `go test .\internal\bootstrapssh .\internal\profile .\internal\config .\cmd\cli -count=1` -> all passed on Windows (`internal/bootstrapssh` 3.736s, `internal/profile` 0.756s, `internal/config` 0.832s, `cmd/cli` 18.752s).
- Full Windows suite: `go test .\... -count=1` -> passed all packages, including host, proxy, browser, credential and discovery tests.
- Vet: `go vet .\...` -> passed.
- Windows CLI build -> passed; the exact local output command is in the private session journal. This is a local candidate, not an installed or approved release; a later version-only smoke attempt was stopped after an endpoint-protection alert.
- Ownership cross-builds: `go build .\internal\profile` with each GOOS/GOARCH in linux/darwin/windows x amd64/arm64 -> all six passed. Unix lock behavior has not been executed on this Windows host.
- Lint: `golangci-lint run` -> could not load config: installed tool was built with Go 1.26, below the module's 1.27.1 floor. Session-local v2.14.0 install/run and a bounded retry produced no successful lint result. The operator then supplied an endpoint-protection prevention alert for that exact local utility; read-only hashing confirmed the artifact match. Stop retries and use authorized review/validation, not an exclusion or path/name workaround.
- Live feature verification -> confirmed fail-closed with missing non-secret probe inputs; no network connection was attempted. The gate additionally requires Gitea JSON status=pass, not only curl exit success. Its feature state remains pending.
- Race detector: not run; CGO_ENABLED=0 and the Windows job's documented no-race limitation applies.
- Manual smoke test: not run; neither process startup nor bridge readiness will substitute for a TLS-valid destination-service probe.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- Keep the existing merge precedence: flags > environment > YAML > profile > defaults. Browser inherits runtime defaults without importing the profile's TCP target.
- Import only the supported non-secret profile schema; reject unknown fields, credential paths/references, unsupported versions, and multiple YAML documents before writing.
- Keep YAML path spelling intact; the Windows test compares `filepath.Clean` paths rather than treating equivalent slash representations as different directories.
- Preserve the existing managed credential and omit unnecessary `auth set` guidance when it is already associated.
- Hash profile names into persistent files under the profile-store ownership directory; Windows uses nonblocking LockFileEx and Unix uses nonblocking flock. Close handles on command return, never unlink the files, and log unexpected close failures.
- Promote the already-pinned `golang.org/x/sys v0.48.0` from indirect to direct use for platform locking. No module or version was added; `go.sum` is unchanged.
- Startup SSH failures identify the selected executable and retain the underlying error. Guidance offers IdentityFile or the matching agent, rather than making agent presence mandatory; no terminal streams are captured.
- Keep endpoint-protection telemetry and artifact provenance in the private handoff, not public GitHub comments. Matching a reported digest identifies the artifact; it does not establish that the detection is a false positive.
- Operator pause: no migration or application connection today. Resolve the authorized lint/execution path before resuming live acceptance.

## Authorized CI validation - 2026-10-16

The operator approved commit/push and a draft PR against `master`. Existing CI
provides Linux build/vet/race tests/lint and native Windows build/vet/tests.
No new mesh secrets, local protection exceptions or prevented executables are
needed for that path. Workstation alert disposition remains separate.

- `git diff --check` -> passed before staging the publication changes.
- Current-revision Go tests, vet and lint -> not rerun locally; execution is
  delegated to the existing authorized CI runners. Record their actual results
  after publication, including any dependency updates integrated from `master`.
- Live AC5 -> not run. An authorized Windows/Git Bash service probe, duplicate
  attempt and unrelated-profile/process observations are still required.
- Independent adversarial review -> not yet run; draft publication is not an
  archive, merge or completion claim.

## Related Findings and Disposition

- Corrected the related SOCKS runbook's host-key-verification bypass; normal known_hosts storage and first-use fingerprint verification remain required.
- Existing context toolchain-label drift and ADR-009 status reconciliation are tracked on #412, with root cause and fix options recorded there. Do not change the human-owned context body through context-refresh.
- `status`/readiness remain informational contracts, not proof of application reachability; the runbook explicitly requires a destination-service probe. No status-exit-code redesign is part of this feature.
- #417's misleading profile help is corrected in this change; its broader source-diagnostics work remains separate.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-038-windows-path-spelling-in-config-tests.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: docs/adr/adr-016-self-contained-profile-connections.md
- [x] New pattern candidate for `00_meta/patterns/`? no: this session establishes a project-specific connection contract, not a verified cross-project recurrence.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CFG-005-self-contained-connections/` -> `specs/archive/CFG-005-self-contained-connections/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
