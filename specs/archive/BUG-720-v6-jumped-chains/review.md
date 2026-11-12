---
spec: "BUG-720-v6-jumped-chains"
verdict: "PASS"
reviewed_sha: "73bb4845a1a1e069808458f9691c64b9630e583d"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-720-v6-jumped-chains` — whole change `git diff f2663855a8791747aa0b07952f7d62223fa36ce3...HEAD`
(8 first-parent commits, 4 of them spec docs; PR mlorentedev/ts-bridge#752, open, not draft).
**Sources**: `specs/BUG-720-v6-jumped-chains/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`,
`scripts/e2e/linux.sh`, `docs/design/host-client-parity.md`, `docs/runbooks/guide-rdp-host-setup.md`.

### What I ran (fresh, this session, at `73bb484`)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `gofmt -l .` | no output |
| `go vet ./...` | clean |
| `go test -race ./...` | 17/17 packages `ok`, exit 0 |
| `go test ./internal/host/ -run 'TestIP6Exposure|TestIP6Jumps' -count=1 -v` | 12 + 15 subtests PASS |
| `golangci-lint run ./...` (darwin) and `GOOS=linux golangci-lint run ./...` | `0 issues` both |
| `shellcheck scripts/e2e/linux.sh`, `bash -n`, `zsh -n` | clean, exit 0 |
| `scripts/tests/test-e2e-{probe,report,run-teardown,headscale,mesh-preflight}.sh` | all cases passed |
| `dotf spec digest BUG-720-v6-jumped-chains` | matches all four `contract_digests` in `review-request.json` |
| Red-green: `git checkout f2663855 -- internal/host/linux_steps.go` in a `mktemp -d` copy | `TestIP6Exposure` (11 rows) and `TestIP6Jumps` (6 rows) **FAIL**; restored → PASS |
| Probe suite in the `mktemp -d` copy (throwaway `zz_probe*_test.go`, never in the repo) | results quoted per finding below |
| `git diff --stat e9e8c7b..HEAD -- internal/ scripts/ cmd/ Makefile` | empty — the AC5 evidence commit really does carry the last code state |

TDD order holds: `f08f7a1` touches only `linux_steps_test.go` (84+/1-), `b96a0a7` only `linux_steps.go`
(115+/8-), `eb0d4eb` only `scripts/e2e/linux.sh`.

### Spec and task alignment

- **AC1** — `TestIP6Exposure/an_unscoped_v6_allow_inside_ts-input` asserts check *and* setup `manual`,
  the exact `ip6tables -D ts-input -p tcp -m tcp --dport 3389 -j ACCEPT` fix, and
  `linuxFirewallOK == false`. `withIP6` builds the `-D` from the printed `-A <chain>` line, confirmed
  by probe (fix text `ip6tables -D ts-input …`). Verified.
- **AC2** — `TestIP6Exposure/captured_chain{,_iptables-nft}` assert the call list now includes
  `output ip6tables -S ts-input`, and both captured shapes read `ok`. The *fixture provenance* claim
  (capture log `scripts/e2e/out/tsfw-linux-20261112T141322Z.log`) is UNVERIFIED: `scripts/e2e/out/`
  is gitignored (`.gitignore:76`) and absent in this fresh worktree.
- **AC3** — 9 named `TestIP6Jumps` rows cover narrowed interface, narrowed port, loopback, negated
  scope, two-deep, loop, double jump, `-g`. Verified, including by probe (`INPUT→a→b` reports the
  `-A b …` rule).
- **AC4** — `verdicts_are_not_chains`, `a_target_that_is_no_chain_is_skipped_without_a_note`,
  `a_chain_that_cannot_be_read_is_a_note,_INPUT_still_read`, `ufw's_chains_are_left_to_the_ufw_read`
  all pass. Verified (probe: `-j MARK` also skipped silently; two failed chains yield two notes).
- **AC5** — `make e2e-linux` was **not** run (needs a `dotf vm` guest; out of this run's budget and
  outside CI by design). The added block is syntactically clean, `shellcheck`-clean, and its
  assertions match the code path I probed (`manual` naming `ip6tables -D tsbe2e`). **UNVERIFIED as
  an execution claim**, verified as code.
- Out-of-scope items in the proposal (`ufw6-*` not followed, non-modelled qualifiers, rule order,
  v4 read) are each backed by a test row or explicit prose; no silent drift found.
- `tasks.md` "Closing" checklist is entirely unticked although the work exists and PR #752 is open —
  stale bookkeeping (Minor, finding 3), not a code claim.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Minor | REAL | maintainability | Two doc comments are misattached. `ip6Match`'s explanation sits directly above `iptIfTarget`, so godoc attaches it to the wrong symbol and `ip6Match`/`ip6ScopeFlags` are undocumented; the new `iptNoChain` const was inserted *inside* `chainMissing`'s doc comment, leaving `chainMissing` undocumented and the const owning a comment written for the function | `internal/host/linux_steps.go:334-345` (comment for `ip6Match` followed by `func iptIfTarget`), `:754-762`; `golangci-lint run` reports 0 issues, so CI does not catch it | UNTESTED (prose) | code |
| Minor | REAL | verification artifact | Promotion-candidate reason for the ADR is factually wrong: "ADR-017 Amendment B covers the v6 exposure scan". ADR-017 never mentions #526, `ip6tables` exposure or `ts-input` (`grep -n "526\|exposure scan\|ts-input" docs/adr/adr-017-host-desired-state.md` → no hits); the scan is documented in `docs/design/host-client-parity.md`, which this diff does update. The conclusion ("no new ADR") may still be right, but the recorded reason is not, and the archive gate reads that line | grep above; `docs/design/host-client-parity.md:52` (updated, `#720`) | UNTESTED (prose) | spec — `verification.md` only (not the contract set) |
| Minor | REAL | spec bookkeeping | `tasks.md` Closing checklist is all `[ ]` (`acceptance covered by tests`, `features.json`, `verification.md filled in`, `PR opened`) although all four are true and PR #752 is open. Contract set: do **not** edit `tasks.md` under this verdict | `gh pr list --head fix/issue-720-v6-jumped-chains` → #752 OPEN; `verification.md` filled; `features.json` maps both criteria | n/a | record the disposition in `verification.md`, or carry to a follow-up ticket |
| Minor | THEORETICAL | correctness (scan sensitivity) | A jumped chain ending in a **portless** `-j ACCEPT` is not reported, so `host check` still says `firewall_ok` while every port is open on every interface over IPv6. Probe: `-A INPUT -j ts-input` + `-A ts-input -i tailscale0 -j ACCEPT` + `-A ts-input -j ACCEPT` → `v6Foreign=[]`, `check=ok`, `firewall_ok=true`. This mirrors the long-standing INPUT doctrine (`ip6Exposed` uses `iptPortCovers`, which requires `-p tcp` + a dport, and the row "a port-less v6 accept is the operator's posture" pins it), so it is a scope decision the proposal states ("the same exposure test as INPUT"), not a regression — but no row pins it *for a chain*, so a future reader cannot tell intent from oversight | probe output above; `linux_steps.go:369-377` (`ip6Exposed` → `iptPortCovers`) | UNTESTED — no row covers a portless accept inside `ts-input` | tests (add the row) or a follow-up ticket; not contract set |
| Minor | REAL | test coverage | The `ufw6-*` exclusion is load-bearing end-to-end — `readIP6` runs on ufw hosts too (`linux_steps.go:242`), so a followed `ufw6-*` chain would double-report and hand the operator an `ip6tables -D` that ufw undoes. Only the isolated `TestIP6Jumps/ufw's_chains_are_left_to_the_ufw_read` row covers it; no step-level ufw-host test asserts "one report, ufw's fix". Probe (ufw host, `ip6tables -S INPUT` jumping to `ufw6-before-input` holding an unscoped 3389 allow, `ufw show added` listing it): `v6Foreign=[]`, no `ip6tables -D` in the fix — correct today, unproven by a named test | probe `PROBE-K`; `internal/host/linux_steps_test.go:1989` | UNTESTED at step level | tests |
| Minor | REAL | test strength | `TestIP6Jumps` asserts a note only via `strings.Contains(note, "ip6tables -S <chain>")`, so the failure text (`%v` of the error plus `toolSaid`) is never asserted; a regression that drops the tool's words, or prints a nil error, still passes | `linux_steps_test.go:2010` (`(tt.note == "") != (note == "") || !strings.Contains(note, tt.note)`) | `TestIP6Jumps/a_chain_that_cannot_be_read_is_a_note,_INPUT_still_read` (partial) | tests |
| Question | SPECULATIVE | scope | A jump carrying a qualifier outside the model (`-m conntrack --ctstate NEW`, `-m mark`, `-m set`, `-f`) is silently not followed, so an unscoped allow behind it is missed. Declared ("Out of scope"), and inherited from the v4 model. For the record: `-m comment --comment "…"` does **not** narrow — probed, the jump is still followed. No action requested, confirm the trade-off is intended | `linux_steps.go:928-940` (`iptPortlessToken` returns 0 for unmodelled tokens); probe `PROBE-B` | UNTESTED (by design) | none (surfaced only) |

No Blocker and no REAL Major survived. Things I tried to break and could not:
`-g`/goto (`ip6Match` rewrites `-g` to `-j` for the port test, `ip6Jump` accepts it — reported),
duplicate reporting (INPUT + chain → two distinct entries, both deletable and printed as such),
a chain read whose output names another chain (filtered by `f[1] == chain`), loop through `INPUT`
itself (`seen` starts with `iptInput`), two unreadable chains (both noted, joined by `"; "`),
`-s ::/0` inside a chain (reported, matching the INPUT row), `-s <tailnet>/48` inside a chain
(not reported), `! -i tailscale0` jump (followed), `-p tcp` without `--dport` jump (followed),
`ip6tables -S ts-input` shaped like nft mode with `lo+`/`tailscale0+` (reads `ok`).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All five criteria verified by named tests and probes, red-green proven; two deliberate scan limits (portless accept in a chain, unmodelled jump qualifiers) leave negative-path gaps, none an observed defect |
| Verification       | B | Every criterion cites a named test, commit and command, and the commands I re-ran pass; AC2 fixture provenance and the AC5 `make e2e-linux` run rest on gitignored logs in another working tree — UNVERIFIED here, and one promotion reason is factually wrong |
| Scope              | A | Diff is the proposal line for line: one walk in `readIP6`, one shared const, one e2e block, two doc rows, spec; no creep (v4 walk, ufw read and policies untouched) |
| Reliability        | A | Unreadable chain degrades to a note and keeps everything else, loop bounded by `seen`, nothing written by check or setup, idempotent read |
| Maintainability    | B | `ip6Walk` 24 lines, helpers 3-15 lines, no nesting past 3, no dead code; two doc comments attached to the wrong symbol and two undocumented helpers |
| Handoff-readiness   | B | Design doc + runbook row updated, `verification.md` complete with decisions and evidence; `tasks.md` closing boxes stale and the ADR promotion reason wrong |

### Verdict

**PASS** — no Blocker, no Major; rubric all B or above (one mechanism: minors only, no dimension D or C).

`dotf spec archive` is **advisable now**: the contract digests match `review-request.json`, no
`[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain, and the three promotion lines are answered. The
early archive is on the operator, not on this review — the e2e claim (AC5) is the one piece of
evidence I could not reproduce, and the reviewer signature policy here permits an independent signer
only, which this run is.

### Recommended next steps

All of these are outside the contract set, so applying them does not stale this review. Nothing here
requires a re-review.

- **code** — re-attach the two doc comments: move the `ip6Match` paragraph onto `ip6Match` (and give
  `ip6ScopeFlags`/`iptIfTarget` their own one-liners or inline `iptIfTarget`), and move `iptNoChain`
  out of `chainMissing`'s comment so `chainMissing` keeps its own.
- **tests** — add the portless-accept row to `TestIP6Jumps` (and optionally to `TestIP6Exposure`) so
  the current `ok` is a pinned decision instead of an unstated consequence; add a step-level ufw-host
  row proving the `ufw6-*` exclusion yields one report and a ufw fix; assert the note's failure text,
  not only the chain name.
- **`verification.md`** — correct the ADR promotion reason (the v6 exposure scan lives in
  `docs/design/host-client-parity.md`, not in ADR-017 Amendment B), and record the disposition of
  every finding above (applied / ticketed / declined with a reason). Leave `tasks.md` alone: it is in
  the contract set, and ticking it here would invalidate this verdict — if the closing boxes must be
  honest, that is a `/spec archive`-time concern, not a pre-archive edit.
- **follow-up ticket (optional)** — decide explicitly whether a portless `-j ACCEPT` inside a jumped
  v6 chain (all ports open on every interface, still `firewall_ok`) is exposure the scan should
  report. If yes, it applies to INPUT too and is its own change.
