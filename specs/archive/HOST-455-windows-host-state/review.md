---
spec: "HOST-455-windows-host-state"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: HOST-455-windows-host-state (W1 unattended mode, W2 effective RDP state, W3
firewall rule), `git diff 49e83dc2117187760b93fb0d6c97b4aed21a43c1...HEAD` (59 branch
commits; this spec's code is in `internal/host/windows_steps.go`,
`internal/host/windows_steps_test.go`, `internal/host/steps_test.go`).
**Sources**: `specs/HOST-455-windows-host-state/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`; `internal/host/windows_steps.go`,
`windows_steps_test.go`, `steps_test.go`, `step.go`; `docs/design/host-client-parity.md`
(rows W1–W3); `docs/lessons/lesson-053-2026-11-04.md`; `.github/workflows/ci.yml`
(`test-windows`).

### Spec and task alignment

- Contract digests in `review-request.json` match `dotf spec digest HOST-455-windows-host-state`
  exactly (features.json `21ce1d…`, proposal.md `4855d5…`, tasks.md `fe0d96…`): the contract
  I reviewed is the contract the launcher recorded.
- **AC1** (never `tailscale up`; `ok` when `ForceDaemon: true`, else `set --unattended` →
  `changed`): implemented in `unattendedStep`. `grep` finds no `tailscale up` left in Go
  code (only the doc-referenced `site/**` page, which the spec lists as a follow-up).
  Verified: `TestWindowsUnattendedStep` — 6 subtests pass.
- **AC2** (policy-then-local `fDenyTSConnections`, absent = denied, policy denial `manual`
  naming the policy key, `services[].enabled` = effective): implemented in `rdpDeny.effective`,
  `rdpSetupStep`, `rdpCheckStep`; `checkWindows` assigns `svc.Enabled` from the check step.
  Verified: `TestWindowsRDPDenyState` — 6 subtests pass, and the test asserts the fix names
  `Policies\Microsoft\Windows NT\Terminal Services`.
- **AC3** (read before recreate; `ok` with no `Remove`/`New` when converged; `firewall_ok`
  false for disabled/wrong-port): implemented in `windowsFirewallRuleOK` /
  `windowsFirewallSetupStep` / `windowsFirewallCheckStep`. Verified: `TestWindowsFirewallRule`
  — 16 subtests pass, and the test rejects any `run` other than the recreate.
- **AC4** (table-tested through `Runner` on any OS, runs in `test-windows`): `windows_steps.go`
  and `windows_steps_test.go` carry no `//go:build` tag, and the `test-windows` job runs
  `go test ./...` on `windows-latest`. Verified locally: `GOOS=windows go vet ./internal/host`
  exit 0.
- **Mutation claims in `verification.md` reproduce exactly.** I ran each mutant against the
  real suite and reverted (`git checkout --`), tree clean afterwards:
  - drop `&& prefs.ForceDaemon` → `TestWindowsUnattendedStep/headscale_node,_not_unattended`
    and `/set_fails` fail (as claimed);
  - trust a failed read's stdout (`out, _ :=`) → `/prefs_unreadable` fails (as claimed);
  - ignore the policy value entirely → `/policy_denies_over_local_allow` and
    `/policy_allows_over_local_deny` fail (as claimed, "2 subtests");
  - absent treated as allowed → `/nothing_set` fails (as claimed, "1 subtest");
  - drop the port comparison → `/other_port`; drop `count == 1` → `/duplicated`;
    drop protocol → `/udp`; drop profile → `/public_profile_only`;
  - drop the remote-address comparison → `/restricted_remote_address`,
    `/legacy_any_address`, `/tailnet_plus_one_more_range`.
- **Independent verification I ran** (all fresh, this session): `go test -race ./...` exit 0
  (all packages ok, including `internal/host`); `golangci-lint run ./internal/host/...` →
  "0 issues"; `gosec -quiet ./internal/host/` exit 0; `gofmt -l` on `internal/`, `cmd/` →
  empty; `GOOS=darwin go vet ./internal/host` exit 0; `bash scripts/check-features-selectors.sh`
  → "OK (50 go test -run selectors)", so every `features.json` verification command in the
  repo, this spec's four included, resolves to a real test.
- Parity doc rows W1–W3 are updated with the fixed behavior (verified by reading
  `docs/design/host-client-parity.md` §"Windows defects found in the audit (#455)").
  `docs/lessons/lesson-053-2026-11-04.md` exists and is indexed (lesson row 053).
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec's markdown files.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | ordering / fresh-host convergence | `unattendedStep` runs **before** `serviceStep(r, "Tailscale", …)`. On a host whose `Tailscale` service is stopped or not yet started (fresh install), both `tailscale debug prefs` and `tailscale set --unattended` talk to a daemon that is not running, so the step takes its failure branch and reports `failed`; `setupConvergence` (cmd/cli/host.go:292) then makes `host setup` exit `exitNotConverged` even though the next step starts the service and the rest converges. The spec's premise — "prefs output that cannot be read falls through to the `set`, which is idempotent" — holds for a *parse* failure but not for a *daemon-down* failure, which is a command failure with a fix that names the GUI. Pre-existing (the old `tailscale up --unattended` failed the same way), so **not a regression** from this change, but it is the residual hole in the spec's own idempotency claim and it is untested. | Code read of `setupWindows` order (windows_steps.go:15-30) + `unattendedStep` failure branch (windows_steps.go:44-52); `setupConvergence` in cmd/cli/host.go:292-298. No Windows host available to reproduce; no test makes the daemon-down read fail while the service step would then start it. | UNTESTED (the golden `windowsSetupCalls` in `steps_test.go` pins the order but asserts nothing about the daemon; `TestSetupFor` never models a stopped service) | code + tests (start/verify the service first, or retry the read after the service step); ticket as follow-up. Pre-existing, so not a blocker for archive. |
| Minor | THEORETICAL | check accuracy | `windowsFirewallCheckStep` receives `svc.Port`, which is 0 when the RDP port read failed; `windowsFirewallRuleOK` skips the port comparison on `port == 0`, so a rule on the wrong port can read as converged (`firewall_ok: true`) in exactly the unreadable-port case. Mitigated in practice: that path already emits a `failed` RDP step and sets `svc.Unread`, so `host check` exits non-zero anyway. This is a documented decision in `verification.md` ("A port read of 0 (unknown) skips the firewall port comparison in check, as before"). | windows_steps.go `checkWindows` + `windowsFirewallRuleOK`'s `(port == 0 \|\| …)`; verification.md §"Decisions made during implementation". | `TestCheckFor` (golden check table with the `psPort: "0"` case) pins the skip; no test asserts `firewall_ok` is false when the port is unknown — that path is UNTESTED | code (report `unverified` when the port is unknown) + tests, if ever taken up |
| Minor | REAL | spec artifacts | The spec's own closing record lags the shipped state. `verification.md` §"Test status" records only "PR 1: go test -race … exit 0", while tasks 2 and 3 are ticked complete; `tasks.md` §Closing leaves all three boxes unticked although the substance holds (every AC has a named test, the four `features.json` selectors resolve — `check-features-selectors` OK — and `-race`/lint/gosec/vet/parity-doc are all green as re-verified above). | `specs/HOST-455-windows-host-state/verification.md` lines under "## Test status"; `tasks.md` "## Closing". | n/a (documentation) | spec artifacts — `verification.md` (outside the contract set, safe to edit now) and the `tasks.md` closing ticks (normalised out of the digest, so they do not stale this review) |
| Minor | REAL | spec vs code (interaction with #631) | AC3 in `proposal.md` describes W3's converged test as "exactly one rule, enabled, on the RDP port", but the shipped `windowsFirewallRuleOK` additionally requires `action=Allow`, `direction=Inbound`, `protocol=TCP`, `profile=Any` and the tailnet `RemoteAddress`, and reports an any-address rule as legacy. The widening is correct and safer — without it an any-address or Block rule would read as converged — and it is recorded in `verification.md` ("beyond the AC's port and state … (review on #600)") and in ADR-017 Amendment B / the parity doc, i.e. it is not a silent override. It is nonetheless a contract-text divergence, and it arrived from SEC-631's scope work landing in the same function. | proposal.md AC3 vs windows_steps.go `windowsFirewallRuleOK` + `remoteIsTailnet`; verification.md §"Decisions made during implementation". | `TestWindowsFirewallRule` (blocks / outbound / udp / public profile / restricted remote / legacy cases) | spec (next round or SEC-631's contract; do **not** edit `proposal.md` now — it would stale this review) |

No Blocker and no REAL Major was found in W1–W3. I looked specifically for and did not find:
a remaining `tailscale up` path; a policy-denial path that writes locally; an
`effectiveDeny` that trusts the local value over the policy; a firewall read that reports
`ok` for a duplicated, disabled, wrong-port or any-address rule; a check that reports
`firewall_ok` true on a failed rule read; a write inside a check path; a fail-open on the
W2 registry read (an unparseable read maps to `failed`, not to `enabled`); a runner call
the tests do not pin (the setup/check golden tables assert the full call sequence).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four ACs verified with negative paths and mutation-tested tests; one residual fresh-host ordering gap folds W1's read into a daemon-down failure. |
| Verification       | A | Every claim in `verification.md` reproduced here — 3 named tests, 5 mutants red-green, `-race` suite, lint 0, gosec 0, Windows vet 0, selector guard OK. |
| Scope              | B | W1–W3 diff matches the proposal; AC3's converged test is a documented superset (from #631's scope work in the same function). |
| Reliability        | B | Error paths and fail-open choices are deliberate and pinned by tests; the daemon-down ordering and the unknown-port firewall skip remain. |
| Maintainability    | B | Small named helpers, comments explain why; `windowsFirewallRuleOK` is a six-term boolean with a `nolint:gocyclo` baseline entry (#629). |
| Handoff-readiness  | B | Parity doc rows, ADR amendment and lesson-053 present; `verification.md` test status and `tasks.md` closing boxes have not caught up. |

### Verdict
PASS WITH GAPS

### Recommended next steps

- **Ticket the ordering gap (Major, THEORETICAL, code + tests):** make the Tailscale service
  step precede (or the W1 read retry after) `unattendedStep`, so a first `host setup` on a host
  with a stopped tailscaled converges instead of exiting `exitNotConverged`; add a case to
  `TestSetupFor` / `TestWindowsUnattendedStep` where the daemon is down until the service step
  runs. Pre-existing, so a follow-up ticket is the right disposition, not a re-review.
- **Record the disposition in `verification.md`** (outside the contract set, so it does not
  stale this verdict): the test status for PR 2 and PR 3, the two Minor notes above, and the
  closing record. Tick the three `tasks.md` closing boxes there too — the digest normalisation
  ignores ticks, so this cannot invalidate the review.
- **Leave `proposal.md` / `tasks.md` / `features.json` untouched** now. AC3's text is narrower
  than the shipped behavior; if the maintainer wants it to match, that belongs to a new round
  or to SEC-631's contract, not to this archive.
- `dotf spec archive HOST-455-windows-host-state` is **advisable in the current state**: the
  contract digests are current, all four ACs have named, passing, mutation-tested coverage, and
  the one open Major is THEORETICAL and pre-existing, carried as a follow-up. The live Windows
  acceptance in `verification.md` ("run `host setup` twice on a Headscale-joined Windows host")
  remains Manu's, as the proposal declares.
