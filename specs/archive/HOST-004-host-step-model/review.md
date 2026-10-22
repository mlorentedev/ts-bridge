---
spec: "HOST-004-host-step-model"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "f3815424a338bc8c888d976813ba118510aa525b"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-21"
---

## Adversarial review

**Scope**: `HOST-004-host-step-model` — the whole change `7c5ead3cdf9bf7e5a9cebf63d86aa7d9187fc7fc...HEAD`
(`f3815424a338bc8c888d976813ba118510aa525b`, 7 commits, 58 files). HOST-004's own commits are
`f3dbc8d` (PR A), `ae81b56` (PR B1), `1fe8783` (PR B2), `eb24192` (F1 fix) and `f381542` (round-1
dispositions); the diff additionally carries `ed3ccb1` (CFG-005 #442) and `fad5e57` (spec archive),
which belong to other lanes — see the Scope rubric line and Finding F8.

**Sources**: `specs/HOST-004-host-step-model/{proposal,tasks,verification}.md` + `features.json`;
`git diff 7c5ead3...HEAD`; `git log --stat` per commit; source under `internal/host/` and `cmd/cli/`.

### Spec and task alignment

Line-by-line against `proposal.md`'s acceptance criteria. Every claim below was produced in this
session; commands and outputs are named.

- **AC1 (outcomes + fix, `Success` derived)** — `Outcome` with all five values and a `Fix` field in
  `internal/host/step.go`; `Succeeded()` is `ok|changed`. `TestOutcomeSucceeded` reddens when the
  derivation is mutated to `o != OutcomeFailed` (M4). Caveat in F1 below: `Success` is a stored field, and
  only the setup path's test asserts it equals the outcome.
- **AC2 (injected seam; per-OS lists built from an OS argument; recorded argv equals today's calls)** —
  `Runner`/`execRunner`, `setupFor(goos, …)`/`checkFor(goos, …)` untagged, only `isElevatedImpl`
  build-tagged; `TestSetupFor`/`TestCheckFor` run Windows, Linux and darwin cases on darwin. I
  independently compared the ported command literals with `git show 7c5ead3:internal/host/host_{linux,windows}.go`:
  ufw `route allow in to any port N proto tcp`, `iptables -A INPUT -p tcp --dport N -j ACCEPT`,
  `tailscale up --unattended`, `sc config|query`, `net start`, the two PowerShell scripts, `powercfg`
  and the firewall script are byte-identical. The golden test bites: `-Profile Any` → `-Profile Private`
  reddens `TestSetupFor` and `TestSetupForWindowsRDPFailureFallsBackToDefaultPort` (M6).
- **AC3 (`check` is the read half and never mutates)** — `TestCheckFor` asserts the exact read argv and
  fails on any `Run` call; adding `r.Run("ufw", "reload")` to `checkLinux` reddens two cases (M1).
  Windows `check` reports 3 of the 8 setup steps, which is what the proposal's PR B1 table prescribes.
- **AC4 (JSON adds `services[]`, `steps[].outcome`, `steps[].fix`; legacy keys stay and are derived)** —
  verified end to end on this runner: `/tmp/tsb host check --json` emits exactly one JSON object with
  keys `tailscale_ip, rdp_port, rdp_enabled, firewall_ok, tailscale_up, platform, services, steps`,
  `services: []`, `steps[].outcome`/`fix` present. `TestCheckJSON_AddsServicesAndSteps`,
  `TestCheckJSON_EmptyListsAreArraysNotNull`, `TestSetupJSON_StepsCarryOutcomeAndFix` pin the shape,
  including `active: null`. The "`smoke.bats` unchanged" half of AC4 held in B1 (#470) and is
  contradicted in the final state by B2 — the proposal pre-resolves this (owner decision, 2026-10-21,
  AC4 scoped to B1 and AC5 to B2), so it is spec-consistent, but it does mean AC4's second half is not
  re-verifiable at this sha. `features.json` f4 scopes its command to the JSON/check tests only, which
  is honest about that.
- **AC5 (`check` exits 0 only when every step is `ok`)** — `checkConvergence` + `ExitError`/`ExitCode`.
  Observed: `/tmp/tsb host check` on this runner exits **2** with
  `host not converged: 1 of 1 steps not ok: Tailscale IP (failed)` on stderr and pure JSON on stdout
  under `--json`. Mutations: `exitNotConverged = 2 → 1` reddens `TestHostCheckCmd_ExitStatus` (M3);
  counting only `failed` reddens the drift cases (M5); reverting `main.go` to `os.Exit(1)` reddens the
  `host check` smoke cases (bats, exit 1 vs `assert_status_in 0 2`). `docs/qa-coverage.md` and
  `site/src/content/docs/cli-reference.md` both carry the new contract; ADR-017 §2 records it.
- **Round-1 Major (an unreadable firewall must not read as `changed`)** — genuinely applied.
  Removing the `!readable` override restores `changed` and reddens
  `TestCheckFor/linux, firewall unreadable without root` (M2), so the regression test is real, not
  decorative. The `linux, nothing` case still reports `changed` because `iptables -L` is readable
  there, which is the distinction the finding asked for.
- **Process**: no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in the spec folder. `tasks.md`'s
  Closing boxes and `verification.md`'s promotion candidates are still unticked/unanswered (F7),
  and `proposal.md` still reads `status: implementing` with `- [ ]` AC boxes.

Full suite and gates as run here (`go1.27.1 darwin/arm64`): `go build ./...` exit 0;
`go test -race -count=1 ./...` all 14 packages ok; `golangci-lint run ./...` **0 issues**;
`GOOS={windows,darwin,linux} go vet ./internal/host/ ./cmd/cli/` clean; `bats scripts/tests/smoke.bats`
**56/56 ok**. (Pre-existing and unrelated: `gofmt -l` flags `cmd/cli/status.go` and
`cmd/cli/status_test.go`, untouched by this diff.)

### Findings

| # | Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| F1 | Minor | THEORETICAL | AC1 invariant / tests | `SetupStep.Success` is a stored field set only by `newStep`; nothing re-derives it, and no test asserts `Success == Outcome.Succeeded()` for **check** steps. A future step built as a struct literal can emit `"outcome": "unverified", "success": true` in `host check --json` with every test green. | M7: replacing the unverified step with `SetupStep{Success: true, Outcome: OutcomeUnverified, …}` leaves `go test ./internal/host` green. `TestSetupFor` asserts the invariant; `TestCheckFor` does not. | UNTESTED | tests (add the assertion to `TestCheckFor`'s step loop) |
| F2 | Minor | THEORETICAL | resilience | `linuxFirewallAllows` returns `readable = ufwErr == nil` on the iptables error path, so a *readable* ufw listing with no matching port plus an *unreadable* iptables listing still reports `changed` ("run host setup") — the unknown-as-absent conflation F1 fixed, on a narrower path. Needs a `ufw` that answers without root. | code read `internal/host/linux_steps.go:62-73`; no test case covers "ufw readable, iptables unreadable" | UNTESTED | code + tests |
| F3 | Minor | REAL | docs | The breaking exit-code change is recorded in `cli-reference.md` and ADR-017 §2, but `site/src/content/docs/getting-started.md` and `docs/runbooks/guide-rdp-host-setup.md`/`guide-deployment-windows.md` still present `host check` as a plain verification step with no mention of exit 2. | grep over `docs/`, `site/`, `README.md`: only the two updated files mention the exit status | n/a (docs); `host check: read-only, runs without admin…` is the executable proof for smoke | docs (free-to-edit set) |
| F4 | Minor | REAL | text output | `host check` prints `RDP: ✗ NOT OK (port 0)` whenever no service is enabled (macOS today), which reads as a defect to an operator; the port-0 decision is documented for JSON only, and `printCheckText` has no test. | observed: `/tmp/tsb host check` on this darwin runner | UNTESTED (`printCheckText` has no covering test) | code (+ test) |
| F5 | Minor | REAL | test strength (deliberate weakening) | `assert_success` was widened to `assert_status_in 0 2` in six `host check` smoke cases, so bats alone no longer proves drift → nonzero. | diff of `scripts/tests/smoke.bats`; mitigations verified: `main.go` → `os.Exit(1)` reddens the suite (M-main), and `TestHostCheckCmd_ExitStatus` pins 0/2 both ways (M3, M5) | `TestHostCheckCmd_ExitStatus` is the pinning test; bats keeps the assertion and adds a named helper | tests — **accepted, no change requested**: the exit-status contract stays enforced, just in Go where it is platform-independent |
| F6 | Minor | REAL | exit status | `host setup` still returns `nil` (exit 0) when steps are `failed`, so `host setup && next` cannot detect a failed convergence — the same argument AC5 makes for `check`. Declared as out of AC5's scope. | code read `cmd/cli/host.go:162-197`; `verification.md` "Decisions made during implementation" | UNTESTED (no test asserts setup's exit status) | code + tests, follow-up ticket |
| F7 | Minor | REAL | handoff / archive gate | `verification.md`'s three "Promotion candidates" lines are unanswered and the archive checklist is unticked; `tasks.md`'s Closing boxes are unticked although the work is done. `dotf spec archive` refuses a promotion line left unanswered. | file read | n/a | spec: `verification.md` (free) for the promotions; `tasks.md` is contract-set — see next steps, do not edit it in this state |
| F8 | Question | — | launcher / archive gate (UNVERIFIED) | `review-request.json`'s `contract_digests` for `proposal.md` and `features.json` match no normalization I could reproduce, while `tasks.md` matches under checkbox normalization. If the gate recomputes digests, this review may be refused as stale for a cause that lives outside the spec (and that no contract edit would fix). The gate is outside this repository, so I could not verify which it does. | sha256 over raw, tick-normalized, CRLF, frontmatter-stripped, comment-stripped, whitespace-collapsed and canonical-JSON forms | n/a | harness/launcher — raise with the human, not with the spec |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All five ACs verified by running; negative paths (unreadable firewall, unreadable registry, empty lists, unknown port) are covered by named cases; no defect observed in the shipped behaviour. |
| Verification       | A | Reproducible here: `go test -race ./...`, `golangci-lint` 0, three-GOOS `vet`, 56/56 bats, and five mutations each reddening a named test — I re-ran the round-1 claims rather than trusting them. |
| Scope              | C | HOST-004's own commits match proposal PR A/B1/B2 exactly, but the reviewed diff also carries CFG-005 (#442, ~1700 lines) and a spec-archive commit from another lane, which I did not review (that spec has its own archived `review.md`). |
| Reliability        | B | Every error path is mapped to an outcome and a fix, and "cannot read" is now distinct from "absent"; idempotency is still partial by design (a re-run reports `changed` until #454/#455), which the proposal declares. |
| Maintainability    | A | Longest new function is 32 lines, complexity is low, `Runner` is a clean seam, comments say why (`lesson 040`), no dead code (`golangci-lint` unused clean). |
| Handoff-readiness  | B | ADR-017, `cli-reference.md`, `qa-coverage.md` and `verification.md` are updated with per-finding dispositions, but no lesson was captured and the promotion-candidate lines are blank. |

### Verdict

PASS WITH GAPS

No Blocker and no REAL Major; the two structural concerns (F1's `Success` invariant, F2's narrower
unknown-as-absent path) are THEORETICAL with named-but-missing tests, and everything else is Minor.
Scope carries a C, which by the aggregation rule puts this at PASS WITH GAPS rather than PASS.

### Recommended next steps

Route by set — the **contract set** (`proposal.md`, `tasks.md`, `features.json`) is closed by this
verdict: editing any of them now would invalidate the review that permits the archive, so do **not**
tick the AC boxes, the Closing boxes, or flip `status:` in this state. Everything else below is free
to change and belongs in `verification.md`'s disposition list.

1. **`verification.md` (free) — answer the three promotion candidates**, which is what currently
   blocks `dotf spec archive`:
   - Lesson: `yes: docs/lessons/lesson-041-<date>.md` for "an unreadable state is not an absent state
     — report `unverified`, not `changed`" (the round-1 fix; it will recur in #454/#455).
   - ADR: `no: ADR-017 §2 already records the outcome set and the exit-status contract`.
   - Pattern: `no: single-project mechanism (one CLI's check semantics)`.
2. **`verification.md` (free) — record a disposition per finding** above (applied / ticketed /
   declined with reason). F1's M7 mutation is worth writing down: it is the evidence that the
   invariant is unguarded.
3. **Tickets**: file one for F4 (the `port 0` text rendering and the missing `printCheckText` test)
   and one for F6 (`host setup` exiting 0 with `failed` steps, to be aligned with AC5 or explicitly
   documented as the contract). F1 (add the `Success` assertion to `TestCheckFor`) and F2 (the
   ufw-readable/iptables-unreadable case) can ride on either ticket.
4. **Docs (free)**: add the exit status to `getting-started.md` and the two host runbooks (F3). This
   is a user-facing breaking change (`host check && …` changes meaning), so it deserves the sentence
   `cli-reference.md` already has.
5. **Raise the digest mismatch (F8) with the human/launcher.** If `dotf spec archive` refuses this
   review as stale, that is the cause, and no edit to the spec will resolve it. For the record:
   `reviewed_sha` here is `f3815424a338bc8c888d976813ba118510aa525b`, which is `HEAD` as of this run.

**`dotf spec archive` advisability**: **yes, once step 1 is done** — the verdict does not block it.
