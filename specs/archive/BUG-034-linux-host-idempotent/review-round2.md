---
spec: "BUG-034-linux-host-idempotent"
verdict: "FAIL"
reviewed_sha: "854b7992807183962b45669c961f1595d1789bdf"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `BUG-034-linux-host-idempotent` (issue mlorentedev/ts-bridge#454 / epic #450), whole change
`d881023670240b0cc35716e899854e31fec669b4...HEAD` (5 commits, +899/−102). This is round 2; the earlier
verdict (`review-round1.md`, FAIL at `79492ef`) is judged here only as part of the whole, not as a delta.

**Sources**: `specs/BUG-034-linux-host-idempotent/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; the diff over `internal/host/{linux_steps,step,steps_test,linux_steps_test}.go`,
`cmd/cli/{host.go,host_json_contract_test.go}`, `docs/design/host-client-parity.md`,
`docs/lessons/lesson-042-2026-10-22.md`, `docs/lessons/_index.md`.

**Contract digests** verified against `dotf spec digest BUG-034-linux-host-idempotent` at the start and
again after all probing — **identical** to `review-request.json`, so this review describes the current
contract. I did not edit `proposal.md`, `tasks.md` or `features.json`.

**Verification actually run in this session**
- `go build ./...` → exit 0. `go test -race -count=1 ./...` → every package ok. `GOOS=linux|windows|darwin go vet ./...` → clean. `golangci-lint run` → `0 issues`.
- All five `features.json` verification commands, run literally → PASS each; every entry is `state: pending` with empty `evidence`, so no agent-claimed `passing`.
- Mutation A (make `portListHas` accept a port *prefix*): `TestUFWAllows/shorter_port` goes red — the exact-port property is pinned.
- Mutation D (delete `firewallSetupStep`'s `rulePresent` early return): `TestSetupForLinux` goes red — "never `-A` when `-C` exits 0" is pinned.
- **Mutation B** (`xrdpSetupStep`: `x.enabled && x.active` → `x.enabled`): **suite stays green.** **Mutation C** (same edit in `xrdpCheckStep`): **suite stays green.** See F1.
- ufw 0.36.2 probe in `ubuntu:24.04` (this session): `ufw show added` verbatim, incl.
  `ufw allow 3389/tcp`, `ufw allow 22/tcp comment 'ssh port'`,
  `ufw allow from any port 1234 to any port 3389 proto tcp`,
  `ufw route allow in on tailscale0 out on eth0 to any port 3389 proto tcp`,
  `ufw allow in on tailscale0 to any port 3389 proto tcp comment 'rdp'`, `ufw allow 3380:3390/tcp`.
  Fed to `ufwAllows(…, 3389)` in a throwaway in-package test (deleted afterwards, tree clean):
  lines 1, 5 and 6 → `true`; the `route` line → `false` (correct); **line 3 → `false`** (F2).
- `iptables` probe, `ubuntu:24.04 --privileged` (this session): with only
  `-A INPUT -i lo -p tcp --dport 3389 -j ACCEPT` present, `iptables -C INPUT -p tcp --dport 3389 -j ACCEPT`
  exits **1**; after appending the exact rule it exits **0**; `--dport 33890` → exit 1. F4 and AC1's
  read-half are therefore measured, not assumed.
- Round-1's `ufw` comment/range fix re-verified independently: `ufw allow 3389/tcp comment 'xrdp'`,
  `ufw allow in on tailscale0 … comment 'rdp'`, `ufw allow 3380:3390/tcp` all read as present.
- **Not verified by me**: the end-to-end two-run `host setup` transcript, the real-ufw `ufw allow` add,
  and `host check --json` on a real Ubuntu host — UNVERIFIED (no host; container-only probes above).

### Spec and task alignment

- All five ACs have a named test in `internal/host/linux_steps_test.go`, and the `features.json` commands
  resolve to those tests and pass. AC2/AC3 are mutation-verified (A, D). AC1's idempotence holds for the
  state the tool declares (exact rule): measured on iptables (`-C` → 0 after the append) and pinned for
  ufw by `TestSetupForLinux/ufw, converged`.
- **AC4 is over-claimed**: its table covers the *read* (`TestXRDP` has `enabled, not running`) but no
  test asserts the *step* outcome for that state — the branch that decides L4 ("counts as ready …
  whether or not the service runs") is unguarded (F1).
- AC5 holds: not-root / no-backend / unexpected-exit / no-systemctl all land on `unverified` (check) or
  `failed` (setup), never `ok`/`changed`, and check runs no mutating command (asserted in
  `TestCheckForLinux`). The elevation gate is load-bearing (round 1's mutation; I re-read the gate).
- Scope matches the proposal: no unrelated production change outside `internal/host` + two help-text
  lines. `verification.md`, `lesson-042` and the parity doc are updated in the diff. The proposal's ufw
  matching sentence is now narrower than the code (F3).
- `proposal.md` is still `status: implementing`, and the `tasks.md` Closing boxes are unticked — correct
  pre-merge.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | xrdp readiness (AC4/L4) | The "enabled but not running" state is decided correctly in both halves (`x.enabled && x.active`) but **no test pins it at the step level**. A regression that dropped `&& x.active` — i.e. reintroducing exactly L4, "ready when the service is not running" — would ship silently: setup would report `ok` for a crashed xrdp and never run `enable --now`; check would report `ok`. This is the defect class the spec exists to remove, and the only class a future edit is likely to break. | Mutations B and C (both `enabled && active` → `enabled`) leave `go test ./internal/host` **green**; `TestXRDP/enabled, not running` asserts only the `readXRDP` struct, never `xrdpSetupStep`/`xrdpCheckStep`. | **UNTESTED** (no `TestSetupForLinux`/`TestCheckForLinux` case with `is-enabled=enabled` + `is-active=failed`) | tests (add the two cases; nothing in the contract set) |
| Minor | REAL | ufw read (AC3) | A **source-port-scoped** ufw allow is a legitimate A-to-B rule that `ufwRuleMatchesPort` reads as absent: it matches the *first* `port` keyword, which is the source port. `host check` then says `changed` and `host setup` appends a second rule for a port already allowed. Much narrower realism than round 1's comment case (a source-port restriction breaks normal RDP clients), so not a blocker — but it is the same false-negative class. | Verbatim from ufw 0.36.2: `ufw allow from any port 1234 to any port 3389 proto tcp`; probe returns `false`. | **UNTESTED** (no such case in `TestUFWAllows`) | code (`ufwRuleMatchesPort`: prefer the `port` after `to`) + tests |
| Minor | REAL | spec vs code | `proposal.md` ("a rule counts when it is an `allow` on exactly `<port>/tcp`, `<port>`, or `port <port>`") and `tasks.md`'s parser bullet no longer describe the code: it also accepts a trailing `comment '…'`, comma lists and port ranges. The behaviour is the better one (round 1 asked for it) but the contract text was left behind. | `TestUFWAllows` cases `with comment`, `list`, `range`, `full form range`, `full form with comment` pass while the proposal lists none of them. | `TestUFWAllows` (pins the code, not the contract wording) | spec (`proposal.md` "What" + `tasks.md` bullet) — legal now that the verdict is FAIL; digests change, this re-review covers them |
| Minor | REAL | iptables read (AC3, design) | A narrower iptables allow (interface or source scoped) is invisible to the exact-form `-C`, so `host setup` appends an **unrestricted** `-A INPUT -p tcp --dport <port> -j ACCEPT` on a host whose operator had scoped the port (e.g. Tailscale-only). On a `DROP`-policy host this widens RDP beyond the configured scope; the ufw path deliberately treats the equivalent `in on tailscale0` rule as present, so the two backends disagree. Strictly better than the previous unconditional `-A` and consistent with ADR-017 §157, so not a deviation — an undocumented limitation. | Privileged `ubuntu:24.04`: `-A INPUT -i lo -p tcp --dport 3389 -j ACCEPT` present → `iptables -C INPUT -p tcp --dport 3389 -j ACCEPT` exits 1, so `firewallSetupStep` takes the append branch. | `TestSetupForLinux/iptables, rule missing` (pins the append; no case for a scoped pre-existing rule) | spec (proposal "Out of scope") + ticket — not this PR's code |
| Minor | SPECULATIVE | test deletions | The five removed Linux cases in `internal/host/steps_test.go` are analysed only in `review-round1.md`; `verification.md` records no deletion rationale or retention-bar fields. Four have strictly stronger successors; the fifth (`linux, nothing`) uniquely paired a *check* run with Tailscale down and asserted the all-zero legacy keys — now covered only through the Windows variant. Low risk, but the record is not in the spec. | `git log -S "linux, nothing"` → `f3dbc8d`, removed in `c68672e`; no check-side Linux+Tailscale-down case remains; Windows case `windows, registry unreadable and no rule` asserts the same zero legacy. | `TestCheckFor/windows, registry unreadable and no rule` | spec (`verification.md`, outside the staleness set) |
| Minor | THEORETICAL | xrdp read | Carried over from round 1 and ticketed as **#478** (verified OPEN): `LookPath("xrdp")` is the "installed" probe, so a caller without `/usr/sbin` on `PATH` sees `failed` + `apt install` instead of `unverified`; and only the literal `enabled` maps to enabled, so `static`/`indirect` units take the `enable --now` path and fail. | code read; no repro (root has `sbin` on `PATH`); `gh issue view 478` → OPEN. | `TestXRDP/not installed` (asserts the mapping, not the hazard) | code (follow-up #478) |

### Test deletions

Three cases in `internal/host/steps_test.go` (`TestSetupFor` ×2, `TestCheckFor` ×3 — five in total) moved to
`internal/host/linux_steps_test.go`. Successors, shown: "linux with ufw" → `TestSetupForLinux/ufw, converged`
(stricter: pins a converged `ok` run with no mutation); "linux without ufw or xrdp, Tailscale down" →
`TestSetupForLinux/xrdp missing, no systemctl, no backend, Tailscale down` (adds the no-backend and
no-systemctl branches); "linux, port found in ufw" → `TestCheckForLinux/converged`; "linux, firewall
unreadable without root" → `TestCheckForLinux/not root, iptables host`; "linux, nothing" → no exact
successor (see the SPECULATIVE finding). No seam lost a production caller; `git log -S` confirms the
removed cases existed since `f3dbc8d`/`d881023`, i.e. they guarded the pre-#454 behaviour that this spec
deliberately replaces. No retained test was weakened.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1/AC2/AC3/AC5 verified and mutation-checked; one false-negative class left in the ufw read, and AC4's step branch is correct but unprotected. |
| Verification       | B | Named tests per AC, commands reproduce here, container probes reproducible; the "every branch table-tested" claim is overstated (F1) and the end-to-end transcript is not committed. |
| Scope              | B | Diff matches the proposal; only drift is the ufw matching sentence in the contract and the iptables limitation left undocumented. |
| Reliability        | B | Error paths and elevation gating solid; idempotence holds for the declared state, partial for scoped rules on the iptables backend. |
| Maintainability    | B | Small functions, `ufwRuleMatchesPort`/`portListHas` readable and table-tested, `protoTCP` de-duplicated; no dead code. |
| Handoff-readiness  | B | Spec triad, lesson-042 and the parity doc updated in the diff; deletion record and the list/range wording left behind. |

Aggregation: rubric alone would be PASS, but one **REAL Major** (F1) forces **FAIL** by the
severity × reality rule.

### Verdict

**FAIL** — one REAL Major: the step-level decision for "xrdp enabled but not running" is untested, and
mutations that reintroduce L4 (the bug this spec closes) leave the suite green.

### Recommended next steps

1. **Tests only — required to flip the verdict (outside the contract set, so no digest change).** Add two
   cases: `TestSetupForLinux/"enabled, not running"` (`is-enabled=enabled`, `is-active=failed`, expect
   `run systemctl enable --now xrdp` and `OutcomeChanged`) and `TestCheckForLinux/"enabled, not running"`
   (expect `xrdp service=changed`, `Active=&false`). Both must be red against mutations B and C and green
   against the current code.
2. **Tests + code — Minor, can ride the same round.** Add the verbatim
   `ufw allow from any port 1234 to any port 3389 proto tcp` case to `TestUFWAllows` and make
   `ufwRuleMatchesPort` prefer the `port` following `to`, or state in the proposal that a source-port
   scoped rule is out of scope. `TestUFWAllows` is outside the contract set.
3. **Contract set (only because this verdict is FAIL).** Bring `proposal.md`'s ufw sentence and the
   `tasks.md` parser bullet in line with the shipped parser (trailing `comment`, comma lists, ranges).
   These edits change the contract digests; the re-review that follows item 1 covers them. Do not make them
   on a PASS.
4. **Spec + ticket, not code.** Record the iptables asymmetry as an out-of-scope line in the proposal (or a
   follow-up ticket): a scoped pre-existing allow reads as absent and setup appends an unrestricted
   `ACCEPT`. Also restore the deletion record in `verification.md` (retention-bar fields for the five moved
   cases, including why `linux, nothing` has no exact successor).
5. **Already tracked.** #478 (xrdp `LookPath`/`static` units) — verified OPEN; leave it out of this PR.

### Archive advice

`dotf spec archive` is **not** advisable in this state: the review is `FAIL` and `proposal.md` is still
`status: implementing`. Flip to PASS by adding the two named step-level cases (item 1), re-running
`dotf spec review BUG-034-linux-host-idempotent`, then archiving.
