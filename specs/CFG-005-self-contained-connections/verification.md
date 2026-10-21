---
tags: [spec, verification, templates]
created: "2026-10-15"
---

# Verification - CFG-005-self-contained-connections

## Evidence

AC1-AC4 have Windows package-test evidence from 2026-10-15. Full Windows tests,
vet, build, and ownership cross-builds passed on that checkout. On 2026-10-16
the operator authorized draft-PR publication and automated validation through
the existing GitHub Actions Linux/Windows runners. Published-revision CI run
`38952352232` passed on head `43da717`, and CodeRabbit reviewed that head on
2026-10-19. Fresh independent archive review remains open; live AC5 was
satisfied on 2026-10-20 (see Live acceptance notes).
The authorized `projects-toolkit-gitea` runtime import is now complete;
`office`, managed credentials and the installed binary remain unchanged.
No feature-completion claim is made.

- [x] AC1 -> `TestProfileRuntimePreserved` covers reads, Set, descriptor Import, SetCredential, and ClearCredential. Existing profile/descriptor tests also pass.
- [x] AC2 -> `TestProfileRuntimeConnectPrecedence`, `TestProfileRuntimeImportPreservesExistingTargetAndCredential`, `TestProfileRuntimeImportRejectsUnsafeOrUnsupportedInput`, and `TestProfileRuntimeBrowserDoesNotInheritTCPTarget`. Repeated configuration leaves identical store bytes and does not ask to replace an existing credential.
- [x] AC3 -> `TestProfileLockLifecycle`, `TestProfileLockReleasedAfterProcessExit`, `TestProfileLockRejectsDuplicateBeforeRunner`, and `TestProfileLockAllowsOtherProfile`. Windows tests use actual OS locks and a forcibly terminated test child; nested connect/browser invocations cannot initialize logging or a second runner.
- [x] AC4 -> `TestStartReportsMissingOpenSSH`, `TestStartReportsEarlyExit`, and `TestStartReportsUnlaunchableOpenSSH`. Tests assert selected-client and IdentityFile/agent guidance, retained error causes, and inherited terminal streams. `TestStartWaitsForSOCKSAndClosesProcess` succeeds without SSH_AUTH_SOCK.
- [x] AC5 -> The existing `office` profile passed live RDP negotiation and duplicate rejection. Gitea runtime import is idempotent and its live HTTPS probe passed on 2026-10-20 (HTTP 200, verified TLS) once the upstream was powered on. The installed binary remains unchanged.

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
- Base integration: retain `master` at `06cfb87` and its dependency versions,
  including `golang.org/x/sys v0.49.0` as a direct dependency for OS locking.
  The merge conflict was only direct-versus-indirect placement/version context;
  upstream `go.sum` is preserved without manually changing checksums.
- Secret-scanning alert 33 points to the synthetic rejected-input fixture in
  `cmd/cli/profile_runtime_test.go`. The detected value was compared privately:
  it is a substring of that known test literal, not an issued credential.
  Use only the incomplete `tskey-` marker in those negative cases; this still
  exercises auth-key-prefix rejection without a credential-shaped suffix.
  No real credential was read, published or used by these tests.

### First CI result and lint disposition

CI run `38951054818` tested published head `ab8e279`:

- Linux `go test -v -race -coverprofile=coverage.out ./...` -> passed.
- Native Windows `go test ./...` -> passed.
- Linux/Windows `go vet ./...` and builds -> passed.
- Smoke, security, hygiene and six release-target builds -> passed.
- `golangci-lint run` -> failed with three gosec taint findings: G703 at
  `Lock`'s `MkdirAll`/`OpenFile`, and G702 at the test helper subprocess.

The production store path is supplied by `config.ProfileStorePath`; lock
directory suffixes are fixed and profile names become SHA-256 filenames.
`TestProfileLockLifecycle` already covers a traversal-shaped profile name.
The subprocess executes this test binary with a fixed selector and no shell;
`TestProfileLockReleasedAfterProcessExit` covers its ownership lifecycle.
Record narrowly scoped G703/G702 justifications at those calls, following the
existing repository annotation convention. Do not disable rules globally or
change path/command behavior to appease the analyzer. Fresh CI is required
after the annotation-only fix; the earlier run is not a lint pass.

### Follow-up CI result

CI run `38952352232` tested published head `43da717` and passed:

- Linux `go test -v -race -coverprofile=coverage.out ./...` -> passed.
- Native Windows `go test ./...` -> passed.
- Linux/Windows `go vet ./...` and builds -> passed.
- `golangci-lint run` v2.14.0 -> passed.
- Smoke, security and all six platform builds -> passed.
- Hygiene run `38952352200` and GitGuardian -> passed on the same head.

### Code review - 2026-10-19

CodeRabbit reviewed `43da717` and published one minor finding: this document
had not recorded the successful follow-up CI run. The update above addresses
that finding. Review dispositions are recorded on PR #442.

PR-Agent run `39294183027` rejected `config.num_retries` as a forbidden
command-line override and published no review; it provides no review coverage.
CodeRabbit's review does not replace the fresh independent archive review
artifact. Live Windows/Git Bash AC5 remains pending.

### Non-admin Windows live acceptance - 2026-10-19

The operator authorized candidate execution on this laptop without elevation.
A separate candidate was built from `61dc2d1`; the installed binary was not
replaced. Git Bash selected Git for Windows OpenSSH.

- The Windows token was not elevated. No services, agents, firewall, DNS or
  system-proxy settings were changed.
- `connect --profile office` reached `READY` on `127.0.0.1:33914`, targeting
  `acemagic-office:45000` through Tailscale SaaS, without profile changes.
- Two actual destination probes returned 19-byte RDP X.224 Connection Confirm
  messages with negotiation response type 2 and selected protocol 2 (NLA).
  These are service-level negotiations, not interactive desktop logins.
- A second invocation of the same profile exited 1 with `already connected`,
  without reaching `READY`. The original process still served the second RDP
  probe; exactly one candidate process remained.
- Profile-store bytes, all managed credential files and the installed binary
  matched their pre-probe hashes. This includes the unrelated Gitea profile.
- The operator then authorized importing only Gitea's supported runtime fields.
  Repeating the import produced identical store bytes. The unrelated `office`
  entry, Gitea target/control URL/credential reference, credential-file contents
  and installed binary were preserved; the office process remained running.
- The existing Gitea YAML contains browser-specific `socks5_routes`; it was not
  imported directly. A separate non-secret input used the supported runtime
  fields and distinct loopback bridge/health/bootstrap listeners.
- A strict-host-key, noninteractive Git OpenSSH probe returned
  `Permission denied (publickey)`, with no accessible agent. The operator then
  unlocked the existing identity interactively in Git Bash and reached `READY`
  without starting an agent or replacing a credential.
- Both profiles ran concurrently. A duplicate Gitea invocation exited 1 before
  `READY`, preserving both original processes; another real office RDP/NLA
  negotiation passed. No interactive desktop login was attempted.
- The operator stopped Gitea with Ctrl+C during an HTTPS probe. The resulting
  reset was not evidence of a spontaneous crash. Its SSH child and listeners
  exited, office still passed RDP negotiation, and a subsequent interactive
  Gitea startup acquired the profile without a stale-lock error.
- Live Gitea HTTPS had **not passed** in the first run (superseded by the
  2026-10-20 acceptance below, where the stall was a powered-off upstream). Windows and Git Bash curl (both Schannel)
  timed out. An independent Go HTTPS probe negotiated HTTP/2 with a normally
  verified certificate chain, but received no response headers within 15 seconds.
  The same happened through the already-running SSH SOCKS proxy without the
  mesh, so the application-response failure is not proven to be mesh-only.
- A separate DERP proxy incompatibility is confirmed: `controlProxyFunc`
  returns `socks5h` for the control host, while Tailscale v1.104.1's
  `derphttp.Client.dialNodeUsingProxy` sends HTTP CONNECT to that endpoint.
  OpenSSH's SOCKS listener cannot accept that protocol; live logs show
  `CONNECT dial to vpn.kubelab.live:443: unexpected EOF` for region 999.
  This explains the relay failure, not necessarily the separate HTTPS stall.
- Candidate acceptance (2026-10-20): a local-only merge of this branch with
  master (including #444), commit `1af01a3`, SHA-256
  `1E53D40EBC0647D4E0D9D3CAD470BCDF12C2117CDD224B2D03F0B4683E7AC590`, was run
  alongside the operator's live office session without touching it.
  `bootstrapssh` and `cmd/cli` tests passed (`-count=1`).
  - Office profile with a distinct hostname: READY; the bridge returned the real
    destination banner `SSH-2.0-OpenSSH_for_Windows_9.5`; `/health/ready` was
    `ok` with zero errors.
  - Gitea profile (control via SSH bootstrap): the SDK's HTTP CONNECT through
    the loopback proxy returned `200 Connection Established` and the node
    reached READY, so the DERP/control incompatibility is resolved by #444.
  - Through the same bridge and port, `vpn.kubelab.live` answered HTTP 200 in
    0.59s with verified TLS. Gitea still completes TLS (ALPN http/1.1) and
    sends the request but returns no response within 20s, with either Host
    header. The bridge is healthy, so the Gitea stall is attributed to the
    backend or vhost on the VPS, not to ts-bridge. Gitea application response
    remains unverified.
  - The candidate worktree, binary and state were removed; nothing was pushed.
- Final acceptance (2026-10-20): after the Gitea upstream was powered on, a
  binary built from this branch merged with master (`63d4e8c`; binary hash not
  captured) reached READY for the Gitea profile. An HTTPS request through the
  bridge returned HTTP 200 in 1.6s with verified TLS, zero bridge errors. The
  earlier stall was the powered-off upstream (the VPS answered 200 locally
  once it was on), not ts-bridge. AC5 live probe is satisfied.
- AC5 is satisfied. Do not weaken TLS/SSH, rotate keys or force login to hide
  these failures. Preserve the existing interactive RDP session: do not open
  another desktop login or disrupt the network during this check.

## Related Findings and Disposition

- Independent review on 2026-10-21: see the dedicated review and disposition
  section below. Its signed result is PASS-WITH-GAPS for the feature-only diff;
  it is not evidence that the broader launcher range was reviewed in full.
- Corrected the related SOCKS runbook's host-key-verification bypass; normal known_hosts storage and first-use fingerprint verification remain required.
- Existing context toolchain-label drift and ADR-009 status reconciliation are tracked on #412, with root cause and fix options recorded there. Do not change the human-owned context body through context-refresh.
- `status`/readiness remain informational contracts, not proof of application reachability; the runbook explicitly requires a destination-service probe. No status-exit-code redesign is part of this feature.
- #417's misleading profile help is corrected in this change; its broader source-diagnostics work remains separate.

## Independent review and disposition - 2026-10-21

`dotf spec review CFG-005-self-contained-connections --reviewer nan/deepseek-v4-flash --foreground`
completed with exit 0 and wrote `review.md` plus `review-request.json` for head
`3c61bd2db1abe6b09523fc68934f7b691f74bea9`. The signed verdict is
**PASS-WITH-GAPS**: one THEORETICAL Major, no Blocker, and seven Minor findings.
The operator approved the documentation-only dispositions below. No production
code, tests, installed binary, runtime profile or managed credential was changed.

### Scope qualification

The launcher specified `c2e598eb9dd39e2f9ccb158fe59cc4e34f29f9d3...HEAD`
(44 files, including inherited BUG-031 and archived review-request changes).
The signed report instead describes `ecfa215...HEAD` (the 26-file CFG-005
feature diff). These ranges are not equivalent. Preserve the signed report
unchanged, but do not claim full coverage of the launcher-requested range or
treat this disposition as final integration/merge approval. BUG-031 has its own
review under #443/#444; that does not silently enlarge this review's scope.

### Current-head CI and fresh review evidence

GitHub Actions CI run `39492831677` and hygiene run `39492831746` passed on
`3c61bd2`. Linux race tests, native Windows tests, lint, security, smoke, all six
platform builds, hygiene and GitGuardian reported success. The Linux job runs
the ownership tests, so Unix lock behavior and race validation have CI evidence;
the earlier statements about these not running refer only to the local Windows
checkout. No local race-detector or prevented lint executable was run.
The PR-Agent job on this head was SKIPPED, not a review. CodeRabbit's earlier
finding was already resolved; its latest draft-skip notice is not fresh coverage.

The independent review transcript records fresh Windows runs:

- `go test ./internal/profile -run Runtime -count=1` -> passed.
- `go test ./cmd/cli -run ProfileRuntime -count=1` -> passed.
- `go test ./internal/profile ./cmd/cli -run ProfileLock -count=1` -> both passed.
- `go test ./internal/bootstrapssh -run 'TestStartReports(MissingOpenSSH|EarlyExit|UnlaunchableOpenSSH)' -count=1` -> passed.
- `go test ./... -count=1` -> all tested packages passed.
- `go build ./...` and `go vet ./...` -> passed.

The live f5 command was not rerun by the reviewer. Its required Gitea JSON
`status=pass` output was not independently captured in this review; the historical
HTTP 200/TLS-valid acceptance narrative above is not a new execution of that
machine-readable gate. Feature states remain harness-owned and unchanged.

### Finding dispositions

| ID | Finding | Disposition |
|---|---|---|
| F1 | THEORETICAL Major: profile-store writes can race or be interrupted | Deferred to existing #391 with the concrete CFG-005 mutation paths. The store has multiple writers, not only `init`; a runtime ownership lock is not a store-wide transaction. Cross-process mutation serialization and atomic/recoverable writes need focused tests and remain outside this feature's approved scope. No corruption was reproduced here. |
| F2 | Runbook incorrectly says live acceptance is still pending | Applied: the runbook points to the recorded 2026-10-20 candidate acceptance. It remains draft because final integration/delivery is pending, not because the historical live probe is unperformed. |
| F3 | Current-head CI and platform evidence are missing | Applied in the current-head evidence section above. Local Windows limitations and successful Linux CI are distinguished rather than conflated. |
| F4 | Contract checkboxes and feature states remain pending | Deferred to feature closure. No contract text or feature state was changed. Only a harness may set `passing`; do not certify f5 without its required captured result. |
| F5 | THEORETICAL Minor: generic URL userinfo is not rejected by the non-secret profile guard | Deferred under existing #399 for explicit URL-userinfo validation and synthetic negative tests. Keep normal OpenSSH `user@host` syntax valid. No real credential was inspected or persisted to investigate this finding. |
| F6 | Runtime imports cannot clear an existing setting | Applied as a documented limitation; explicit unset semantics are follow-up work under #399. The current preserve-omitted/empty contract is intentional and remains unchanged. |
| F7 | Duplicate-ownership wording is broader than the implementation | Applied: the runbook scopes ownership to explicit `--profile` selection and distinguishes listener conflicts for other configuration paths. |
| F8 | SPECULATIVE future serialization of caller-facing `Profile` | Declined: there is no production marshal/export path demonstrating this exposure. Shareable descriptors use a distinct type without runtime or credential-reference fields; removing tags or expanding the API is not justified by the hypothetical alone. |

The follow-up issue comments and PR triage record carry these dispositions on
GitHub. No merge or archive is claimed; #442 still requires conflict resolution
and a deliberate decision on the review-scope qualification.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-039-windows-path-spelling-in-config-tests.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: docs/adr/adr-016-self-contained-profile-connections.md
- [x] New pattern candidate for `00_meta/patterns/`? no: this session establishes a project-specific connection contract, not a verified cross-project recurrence.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CFG-005-self-contained-connections/` -> `specs/archive/CFG-005-self-contained-connections/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
