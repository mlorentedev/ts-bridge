---
spec: "BUG-035-iptables-scoped-allow"
verdict: "FAIL"
reviewed_sha: "ef191e42837abf697835cc22283d5f86987815c0"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: BUG-035-iptables-scoped-allow — `iptables -C` → `iptables -S INPUT` coverage read, plus the
ufw no-destination-port allow.
**Sources**: `specs/BUG-035-iptables-scoped-allow/{proposal,tasks,verification}.md` + `features.json`;
`git diff 8fd5fe855c4411094534945316c64edf1b6c199d...HEAD` (one commit, `ef191e4`) touching
`internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`,
`docs/design/host-client-parity.md`, `docs/lessons/lesson-043-2026-10-22.md`, `docs/lessons/_index.md`.

Contract freshness: `dotf spec digest BUG-035-iptables-scoped-allow` returns exactly the
`contract_digests` in `review-request.json` (all three match), so this verdict describes the current
contract. No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in any spec file.

### Spec and task alignment

- Every AC in `proposal.md` maps to a diff change: AC1/AC3 → `iptablesCovers` + `TestIptablesCovers`;
  AC2/AC5 → `readLinuxFirewall` (`-S INPUT` via `Output`) + `TestIptablesRule` / `TestSetupForLinux`;
  AC4 → the `destPortIndex < 0` branch + `TestUFWAllows`. No AC is unimplemented.
- `tasks.md` all `[x]` except "PR opened", which is the review window itself — not a finding.
  PR #482 exists and is still a **draft**; that is the launcher's context, not a defect, but it should be
  marked ready (`gh pr ready 482`) after the finding-1 fix so the human review reads the finished change.
- Board check (`gh issue list`): #479 (this bug), #480 (duplicate rules), #481 (deny before allow) are
  open and distinct from finding 2 — the no-destination-port ACCEPT gap has no ticket yet.
- `features.json` is honest: five entries, all `state: pending` with empty `evidence` (the harness owns
  the terminal state), each `verification` a focused `go test -run` over the named tests.
- Two `tasks.md`/`verification.md` claims go **beyond** what the tests prove — the comment-stripping
  mechanism (finding 1) and the check-side halves of AC1/AC5 (finding 3).
- The test *deletions* were replacements, not weakenings, and I verified the replacements bite: with
  `-C`'s `exitCode` gone (`grep` finds no remaining caller), removing the `!` check turns
  `TestIptablesCovers/negated_port` + `/negated_multiport` red, and dropping the `-p tcp` requirement
  turns `/udp` red. `TestIptablesRule` still pins present/absent/unknown.

Verification I ran fresh, in this session, on `ef191e4`:

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race -count=1 ./...` | 13 packages ok, 0 failures |
| `GOOS=linux\|windows\|darwin go vet ./...` | clean, all three |
| `mise exec -- golangci-lint run` | `0 issues.`, exit 0 |
| mutation: `!` negation check removed | 2 red — `TestIptablesCovers/negated_port`, `/negated_multiport` |
| mutation: `-p tcp` requirement removed | red — `TestIptablesCovers/udp` |
| mutation: ufw no-dest-port branch reverted | 3 red — `TestUFWAllows/source_only`, `/interface_only`, `/source_only,_tcp` |
| mutation: comment-stripping removed | **green — no test notices** (finding 1) |

Working tree left clean apart from this file and the launcher's `review-request.json`.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | iptables parser / test strength | The comment-stripping line (`strings.Fields(iptComment.ReplaceAllString(line, ""))`) is guarded by no test that fails without it, contrary to `proposal.md` "Risks" ("The parser removes the comment value before splitting fields, so a quoted word cannot pose as a flag") and the `tasks.md` step it ticks. `TestIptablesCovers/comment spelling a covering rule` passes for an unrelated reason, so a refactor dropping the strip would ship silently and reintroduce firewall widening. | Replacing the strip with `strings.Fields(line)` leaves `go test -run TestIptablesCovers` **ok**. It passes because the comment's `-j` becomes the first `-j`, yielding `ACCEPT"`, so the line is skipped — not because the comment was removed. Red-green demonstrated both ways on a probe test: as shipped, `-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "x -j ACCEPT" -j ACCEPT` reads present; with the strip removed it reads **absent** (setup would then append the unrestricted rule — the exact #479 harm). | `TestIptablesCovers/comment spelling a covering rule` (vacuous for this mechanism) — **UNTESTED** | tests (add a case whose comment holds a bare `-j ACCEPT` after the real `--dport`, asserted red without the strip; reword the existing case to say what it actually pins) |
| Major | THEORETICAL | iptables coverage vs ufw parity (not #480, not #481) | A blanket ACCEPT with no destination port — `-A INPUT -i tailscale0 -j ACCEPT`, `-A INPUT -s 100.64.0.0/10 -j ACCEPT`, `-A INPUT -p tcp -j ACCEPT` — reads **absent**, so `host setup` appends the unrestricted `-A INPUT -p tcp --dport 3389 -j ACCEPT`; the ufw equivalents (`ufw allow in on tailscale0`, `ufw allow from 100.64.0.0/10`) read **present**. The proposal's headline ("Both backends answer one question the same way") holds for every case tested but not for the most common hardened Tailscale shape, which is the residual of the very widening #479 records. Declared in "Not covering" (rules without a destination port), so not an AC violation — an unclosed gap. | Probe (this session): `iptablesCovers("-A INPUT -i tailscale0 -j ACCEPT\n", 3389)` → `false`; `ufwAllows("ufw allow in on tailscale0\n", 3389)` → `true`. Harm chain is deterministic: `firewallSetupStep` takes the append branch whenever the read is absent, pinned by `TestSetupForLinux/iptables, rule missing`. | UNTESTED (no case for a no-dport ACCEPT on either backend) | code + tests, as a **follow-up ticket** beside #480/#481 — distinguishing blanket ACCEPTs from `-m conntrack --ctstate` rules is a contract decision, so do **not** edit `proposal.md` under this review |
| Minor | REAL | test traceability (AC1, AC5 second halves) | AC1's "check reports `ok`" on an iptables host and AC5's "a non-zero `-S` exit is `unverified` in check" have no named test. `iptScoped` is used only by `TestIptablesRule/present` and `TestSetupForLinux/iptables, converged`; `TestCheckForLinux` has no iptables covering-rule case and no `-S`-fails case, and `verification.md` claims neither. The behaviour is shared (`checkLinux` → `readLinuxFirewall`) and works, so this is coverage, not a defect. | `grep -rn iptScoped` → 3 hits, none in `TestCheckForLinux`; `TestCheckForLinux/xrdp not installed` is the only iptables check case and uses `iptEmpty` (→ `changed`). | `TestCheckForLinux/not root, iptables host` and `.../no systemctl, ufw unreadable` cover `ruleUnknown → unverified` generically; the iptables-specific paths are **UNTESTED** | tests (two `TestCheckForLinux` cases: scoped rule → `ok`, `-S` exit 2 → `unverified`); `verification.md` update is free |
| Minor | REAL | maintainability (stale comments) | Two comments still describe the `-C` read: the block comment "only exit 0 and exit 1 of `-C` are answers" now sits directly above `TestIptablesCovers` (function renamed, comment not moved), and `TestCheckForLinux`'s doc says "no run call except the read-only iptables `-C`" while the read is now an `Output` of `-S`. Both mislead the next parser change. | `internal/host/linux_steps_test.go:79-80` and `:273`; `git diff` shows `TestIptablesCovers` inserted under the old `-C` comment. | UNTESTED (comments) | code (comment-only) |
| Minor | THEORETICAL | iptables parser | A destination-qualified rule (`-A INPUT -d 192.168.1.5 -p tcp -m tcp --dport 3389 -j ACCEPT`) reads present, so `check` reports "Firewall allows TCP 3389 (iptables)" while tailnet traffic may not match it. `-d` is not operator *scope* in the way `-i`/`-s` are; it selects a different address on the host. Declared by "Its other qualifiers … do not matter", so this is a message-overstates-state note, not a widening. | Probe: `iptablesCovers("-A INPUT -d 192.168.1.5 -p tcp -m tcp --dport 3389 -j ACCEPT\n", 3389)` → `true`. | UNTESTED | surface only — track with #481 if it recurs |
| Question | SPECULATIVE | iptables parser | `iptFlag` takes the **first** occurrence of a flag, so a legal but unusual rule with two `-m tcp --dport` clauses (`--dport 22 -m tcp --dport 3389`) reads absent; `ufw allow … proto any …` reads absent because `ufwProtoTCP` accepts only `tcp` or no `proto` (pre-existing for the destination-port branch). | Probe: both → `false`. iptables-save normalises `-p 6` to `-p tcp`, so the protocol-number variant is not a risk. | UNTESTED | — (surface only; do not gate) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All five ACs implemented and the negative table is genuinely broad (21 cases, three mutations red), but one guard (comment stripping) is unguarded and two AC clauses have no named test. |
| Verification       | B | Re-runnable commands with real outputs for build/race/vet/lint, but one claimed mechanism is proven neither by test nor by a reproducible measurement here; the privileged-container run I could not reproduce (no privileged run in budget) — UNVERIFIED, not contradicted. |
| Scope              | A | Diff is exactly the proposal plus the lesson/parity rows `tasks.md` names; no creep, no dependency, no unrelated file. |
| Reliability        | A | Unknown reads stay `unverified` (check) / `failed` (setup) with no mutation; append happens only on `ruleAbsent`, so a second run converges — pinned by `TestSetupForLinux/iptables, rule missing` and `/iptables -S fails`. |
| Maintainability    | B | Functions are short (CC well under 10), names say what they do, and both helpers carry why-comments — but two comments describe the deleted `-C` read, one of them attached to the wrong test. |
| Handoff-readiness  | A | Lesson 043 records the widening lesson and the measured `-S` syntax, parity doc updated, `verification.md` filled, promotion questions answered with reasons. |

Rubric alone would be PASS (no C, no D); the severity axis governs: one **REAL Major** (finding 1)
forces **FAIL** under the `severity × reality` rule.

### Verdict

FAIL

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) — **leave untouched**; none of the findings
requires a contract change, and editing one would invalidate this review. Every item below is outside
the staleness check.

- **tests (this is the flip item)** — add one `TestIptablesCovers` case to `internal/host/linux_steps_test.go`:
  a covering rule whose comment mentions `-j ACCEPT`
  (`-A INPUT -p tcp -m tcp --dport 3389 -m comment --comment "x -j ACCEPT" -j ACCEPT`, want `true`).
  Confirm red-green: it must fail with `strings.Fields(iptComment.ReplaceAllString(line, ""))` replaced by
  `strings.Fields(line)`, and pass with the shipped line. Reword `comments spelling a covering rule` so it
  states what it actually pins (a comment's *port* token cannot count), not the stripping mechanism.
- **tests** — two `TestCheckForLinux` cases for the AC1/AC5 check-side clauses: `iptScoped` → all `ok`,
  and `iptRead` exit 2 → `Firewall configuration=unverified`, no mutating command.
- **verification.md** — record the new cases against AC1/AC5 and the mutation evidence (comment strip,
  negation, protocol, ufw branch), so the next round reads it as a proven suite.
- **code (comment-only)** — move the `-C`/exit-0-1 comment to `TestIptablesRule` (or rewrite it) and fix
  `TestCheckForLinux`'s "read-only iptables `-C`" sentence; both are now wrong.
- **follow-up ticket** — file the no-destination-port ACCEPT gap (finding 2) beside #480/#481 (neither
  covers it): "an
  interface- or source-scoped blanket ACCEPT reads absent on the iptables backend while the ufw
  equivalent reads present, so setup still appends an unrestricted rule on a hardened Tailscale host."
  Root cause: the "no destination port" exclusion was written for `-m conntrack --ctstate` rules and
  over-generalises to blanket ACCEPTs; fix needs a contract decision.
- **UNVERIFIED, not a finding** — the privileged `ubuntu:24.04` end-to-end run in `verification.md` and the
  ufw 0.36.2 measurements were not reproducible in this run's budget; the unit-level claims they back all
  held under my own runs.

`dotf spec archive` is **not** advisable in the current state: the review verdict is FAIL until the
comment-stripping test exists and a re-review passes. Minimum flip set: that one named red-green test
(finding 1) — the remaining items are tracked gaps that a PASS-WITH-GAPS would carry, not blockers.
