---
spec: "BUG-736-negated-portless-exposure"
verdict: "FAIL"
reviewed_sha: "96247de03ad0aedff9f678bc1040c884656c17ac"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: BUG-736-negated-portless-exposure — `git diff 46b921c1a5c914f517582599bbda05c30386d59b...HEAD`, whole change (12 files, +460/−15), read at `96247de03ad0aedff9f678bc1040c884656c17ac`.
**Sources**: `specs/BUG-736-negated-portless-exposure/{proposal,tasks,verification}.md`, `review-request-round1.json`, `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/adr/adr-017-host-desired-state.md`, `docs/design/host-client-parity.md`, `docs/lessons/lesson-055-2026-11-04.md`.

### What was verified, and how

| Claim | Command | Result |
|---|---|---|
| Contract digests match the launcher's request | `dotf spec digest BUG-736-negated-portless-exposure` vs `review-request.json` | identical for `proposal.md`, `tasks.md`, `features.json`, `spec.md` |
| `features.json` `state`/`evidence` are normalised out of the digest | edited `state: verified` + `evidence`, re-ran `dotf spec digest`, restored the file | digest unchanged (`eb33baac…`) before, during and after; file restored byte-for-byte, working tree clean |
| Build | `go build ./...` | exit 0 |
| Full suite under race | `go test -race -count=1 ./...` | every package `ok`, 0 failures (17 packages) |
| Named tests | `go test ./internal/host/ -run 'TestExposureShapes\|TestWideAllowFix\|TestIP6Exposure\|TestReadIptablesUnscoped\|TestSetupForLinux\|TestCheckForLinux' -count=1` | `ok`, 128 subtests run |
| Formatting / vet | `gofmt -l .`, `go vet ./...` | no file listed; vet clean |
| Linter | `golangci-lint run ./internal/host/...` (2.14.0, the `mise.toml` pin) | `0 issues` |
| Round-1 F2 test actually bites | mutation: `iptExcludesOthers` forced back to `flags = iptPortFlags` (pre-fix), re-ran, reverted via `git checkout` | `TestExposureShapes/…!_--dport_22_--sport_1024:65535…` red on both reads (`v4 exposed = false, want true`, `v6 … false`), then green; tree clean |
| Round-1 F1 test actually bites | mutation: dropped `\|\| f[0] != "-A"` from `iptAdmitsOthers`, re-ran, reverted | `TestWideAllowFix` red for `ufw allow 3389` and `ufw allow 22,3389/tcp` with the appended iptables caution; then green; tree clean |
| Rule-shape sweep (19 shapes, both reads) | throwaway `internal/host/zz_probe_test.go` calling `iptUnscoped` / `ip6Exposed`, file deleted | results below; see F1 and the two accepted over/under-reports |
| Step-level scenario for the finding below | throwaway probe over `iptablesDecide` + `iptUnscopedRead` + `linuxFirewallOK` | `state=present, foreign=[], scoped=true, firewall_ok=true` on a converged host holding `-A INPUT ! -i lo -p tcp -j ACCEPT` |
| Archive gate pre-flights | `dotf spec archive --help` | three pre-flights (drafts, review, promotions). The promotion block here is fully answered `no: <reason>` → that one passes. This review's verdict gates the second. |
| AC4 (`make e2e-linux`) | **not run** — no throwaway VM in this session | **UNVERIFIED**; `verification.md` records `102 pass, 0 fail at f8ac37b` |
| Residual #743 content | `gh issue view 743` | **UNVERIFIED** — GitHub API rate limit exceeded for this session |
| Pruning of `scripts/tests/` | not run | **UNVERIFIED** (they do not parse `linux.sh`; round 1 confirmed the same) |

**Tree state.** Every command ran at `96247de`. Both mutations were reverted with `git checkout internal/host/linux_steps.go`; both probe files were deleted. `git status --porcelain` now lists only `specs/BUG-736-negated-portless-exposure/{review.md, review-request.json}` (mine + the launcher's) and the launcher's untracked `review-round1.md` / `review-request-round1.json`. No file outside `specs/` is modified.

### Probe results (v4 `iptUnscoped` / v6 `ip6Exposed`, port 3389)

| Rule | v4 | v6 | Expected |
|---|---|---|---|
| `-A INPUT ! -i lo -p tcp -j ACCEPT` | **false** | **true** | both true (see F1) |
| `-A INPUT -p tcp ! --dport 22 -j ACCEPT` | true | true | true |
| `-A INPUT -p tcp -j ACCEPT`, `-A INPUT -j ACCEPT` | true | true | true |
| `-A INPUT -p tcp -m tcp ! --dport 22 --sport 22 -j ACCEPT` | true | true | true (accepted over-report) |
| `-A INPUT -p tcp -m tcp ! --sport 22 --dport 22 -j ACCEPT` | false | false | false |
| `-A INPUT -p tcp -m multiport ! --dports 22,3389 -j ACCEPT` | false | false | false |
| `-A INPUT -p tcp -m tcp ! --dport 0:3388 -j ACCEPT` | true | true | true |
| `-A INPUT -p tcp -m tcp ! --dport 22:3389 -j ACCEPT` | false | false | false |
| `-A INPUT -p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN -j ACCEPT` | true | true | true |
| `-A INPUT -p tcp -m conntrack --ctstate NEW -j ACCEPT` | false | false | false (documented fail-closed) |
| `-A INPUT -p tcp -m tcp ! --dport 22 -m conntrack --ctstate NEW -j ACCEPT` | true | true | true |
| `-A INPUT -p tcp ! -i eth0 -j ACCEPT`, `-A INPUT ! -s 10.0.0.0/8 -p tcp -j ACCEPT` | false | false | false (residual, #743) |
| `-A INPUT -i lo -j ACCEPT`, `-i eth0`, `-s 10.0.0.0/8`, conntrack-only, UDP portless, `-A ts-input`, DROP | false | false | false |

### Spec and task alignment

- **AC1 (v4: `! --dport 22`, `-p tcp -j ACCEPT`, `-j ACCEPT` → `manual`, named, `firewall_ok` false, SSH caution, setup deletes none)** — met at unit level and confirmed by the probe: `TestReadIptablesUnscoped/{a port-less accept is foreign, an accept for every port but SSH is foreign}`, `TestCheckForLinux/iptables, an accept for every port but SSH: manual (#736)`, `TestSetupForLinux/iptables, an accept for every port: scoped rule added, nothing deleted, manual (#736)` (call list shows `run iptables -A …` and no `-D`). Idempotency on a converged host is intact: the appended scoped rule makes `scoped=true` on the next run, so setup does not append twice.
- **AC2 (v6 same three shapes)** — met: `ip6Exposed` routes through `iptExposes`; `TestExposureShapes` runs every row through both reads; `TestIP6Exposure` adds the portless and negated-port rows. Fix names the `ip6tables -D` and the SSH caution.
- **AC3 (not reported: `! --dport 3389`, `-i lo`/`-i eth0`/`-s` portless, conntrack-only, UDP portless, `--sport`-only)** — all present as `false` rows in `TestExposureShapes` and confirmed by the sweep above. One shape AC3 does not list, and which the change now reports in v6 but not v4, is F1.
- **AC4 (e2e)** — the eight new cases in `scripts/e2e/linux.sh` read correctly (each adds the rule, asserts `manual`, asserts `firewall_ok` false through the JSON path, asserts the rule survives `setup`, deletes it, asserts `ok`). The recorded run in `verification.md` is a claim I cannot re-execute here → **UNVERIFIED**. The v4 survival assertion is a substring `grep -q -- "! --dport 22 -j ACCEPT"` where the v6 one is an exact `grep -qx` (`F5`-class weakness, Minor below).
- **Non-goals respected**: `iptablesDecide` / `iptMatchesPort` are untouched (diff touches only `iptUnscoped`, `ip6Exposed`, the two fix builders and new helpers); chain-jump and rule-order limits untouched.
- **Round-1 findings** — F1, F2, F3, F4, F5 are applied and each fix is pinned by a named test that I confirmed red under mutation (F1, F2) or by whole-string assertions (F5). F6 was declined and is recorded as accepted in the proposal. The AC4 question is now recorded in `verification.md` but remains unverifiable from here.
- **New drift**: the residual in `verification.md` attributes the surviving negated-interface miss to `iptPortlessCovers`, which is true for `! -i tailscale0` but **not** for `! -i lo` / `! -s 127.0.0.0/8` — those `iptPortlessCovers` accepts (`TestIptablesCovers` asserts they cover the port), and the v4 miss comes from `iptScopedAllow` while v6 reports them. That is F1.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | false negative in the v4 exposure read (negated loopback) | The new v4 scope test `iptScopedAllow` treats **any** negation that is not before a port flag as a scope, so `-A INPUT ! -i lo -p tcp -j ACCEPT` (and `! -s 127.0.0.0/8`, `! -i lo+`) is classed "scoped" and never reaches `foreign`. The shared predicate would report it (`iptPortlessCovers` accepts a negated **loopback** scope by design, and `TestIptablesCovers` already asserts it *covers* the port), and the v6 read does report the identical line. Consequence on a converged v4 host: `iptUnscopedRead` computes `scoped=true` from the scoped rule while `foreign` stays empty, so `check` returns **`ok` with `firewall_ok: true`** while every non-loopback port is open — the exact "worse than silence" symptom the proposal's Why says this spec exists to remove. It also makes AC2 (v6) and AC1 (v4) disagree about the same ruleset, in a change whose "What" promises one predicate for both reads. | Probe at `96247de`: `iptUnscoped("-A INPUT ! -i lo -p tcp -j ACCEPT", 3389)` → 0 lines, `ip6Exposed(…)` → 1 line; step probe on `-P INPUT DROP` + scoped rule + `! -i lo -p tcp -j ACCEPT` → `state=present, foreign=[], scoped=true, firewall_ok=true`. Consistently, `TestIptablesCovers` (`linux_steps_test.go:186-187`) asserts `! -i lo -j ACCEPT` and `! -s 127.0.0.0/8 -j ACCEPT` cover the port, and `iptAcceptOffTailnet` already classifies a negated `-i` as off the tailnet. | **UNTESTED** — no row in `TestExposureShapes`, `TestReadIptablesUnscoped`, `TestSetupForLinux` or `TestCheckForLinux` uses a negated-loopback portless allow; the v6 half is equally unpinned. | code (`iptScopedAllow`: a `!` counts as a scope only when the negated value is **not** loopback, mirroring `iptPortlessCovers`'s own `!` branch) + tests (`TestExposureShapes` rows for `! -i lo` / `! -s 127.0.0.0/8`, v4 and v6; one step row showing `manual` + `firewall_ok` false on the converged host) + spec (`verification.md` residual text — outside the contract set) |
| Minor | REAL | user-facing fix text (`withIP6`) | The v6 caution is spliced as `"run %s, after you %s, as root to delete the v6 rule (setup never writes one)"`, so the sentence reads "run `ip6tables -D …`, after you keep loopback, replies and SSH open first (…), as root to delete the v6 rule", where "as root" now attaches to the wrong clause. `TestWideAllowFix` pins this exact awkward string, so it cannot be tidied without touching the test. | `linux_steps.go` `withIP6`; expected string in `TestWideAllowFix`. | `TestWideAllowFix` (pins the wording, i.e. locks it in) | code (reorder to `"run %s as root … after you %s"`) + tests |
| Minor | REAL | e2e assertion strength (AC4) | The v4 survival assertion is a substring match (`grep -q -- "! --dport 22 -j ACCEPT"`), so a rule that merely contains that text somewhere passes; the v6 assertion beside it is an exact `grep -qx`. Round-1 F5 was about exactly this weakness in unit tests. | `scripts/e2e/linux.sh` diff (new v4/v6 blocks). | `scripts/e2e/linux.sh` (itself; not covered by the shim suites) | tests (`grep -qx` against the printed rule, as the v6 block does) |
| Minor | REAL | handoff / archive checklist | `tasks.md`'s last closing box is still unticked and `features.json` still records all three features `state: pending`, `evidence: ""`, although both unit layers and the e2e rows exist. `proposal.md`'s four acceptance boxes are also still `[ ]` while `verification.md` maps each to commits and named tests. | File contents at `96247de`. | **UNTESTED** | spec — box ticks and `features.json` `state`/`evidence` are both normalised out of the contract digest (verified above: digest identical with `state: verified`), so replaying them does **not** stale this review; `verification.md`/`tasks.md` were only partly carried |
| Question | — | residual coverage | Is #743's scope the *whole* negated-interface class, including the v4 `iptScopedAllow` miss in F1, or only `! -i tailscale0`? The `verification.md` residual text claims `iptPortlessCovers` is the reason, which is inaccurate for `! -i lo`. #743's body could not be read (GitHub rate limit). If #743 covers F1, the fix may be a deferral — but the residual text then needs to say that v4 can read `ok`/`firewall_ok: true`, not merely "not reported". | `verification.md` residual line; `gh issue view 743` → rate-limited. | **UNTESTED** | spec (`verification.md`) |

Reality note on F1: it is **REAL**, not theoretical — reproduced twice (predicate probe and step-level probe), and corroborated by two existing tests that assert the opposite of what the exposure read does.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC1–AC3 hold for every named shape (unit-verified, mutations confirm both round-1 fixes bite) and setup is idempotent, but the new v4 scope test drops the negated-loopback exposure the shared predicate and the decide path both accept, leaving `ok` + `firewall_ok: true` (F1); AC4 unverified here. |
| Verification       | B | `verification.md` now maps AC1–AC4 to commits, named tests and a red-green mutation, and I reproduced both mutations; the e2e run cannot be re-executed in this session and the residual's stated mechanism is wrong for one shape. |
| Scope              | B | Diff matches the proposal's file set (predicate, tests, e2e rows, parity doc, ADR, lesson, spec) and the two reversed test rows are the bug itself; the negated-loopback carve-in and the F1 asymmetry are the drift. |
| Reliability        | C | Error paths unchanged and setup converges idempotently, but one host class still reads a false all-clear in v4 — the read is the product here. |
| Maintainability    | B | `iptExposes` / `iptExcludesOthers` / `iptExcludes` / `iptScopedAllow` / `keepOpenFirst` are small, documented, CC well under 10, `golangci-lint` 0 issues; the weakness is that the `!` rule is now stated in three places (`iptScopedAllow`, `iptPortlessCovers`, `iptAcceptOffTailnet`) with one of them disagreeing. |
| Handoff-readiness  | B | ADR-017, the parity row and lesson-055 are updated in place and `verification.md` is substantive; `tasks.md`'s last box, `features.json` replay and the proposal AC ticks are outstanding (all digest-normalised). |

### Verdict
**FAIL** — one **REAL Major** (F1) under the `severity × reality` rule. The rubric alone would allow PASS WITH GAPS (two Cs, no D), but a REAL Major that needs a fix and a re-review cannot archive. The round-1 findings are genuinely closed: F1 and F2 are applied with tests I confirmed red under mutation, F3/F4/F5 are applied, F6 is recorded as accepted. The remaining defect is in the one place the change rewrote the v4 scope test, and it is the same false-report class the spec was opened for.

### Recommended next steps

Code + tests (no contract-set edit needed, so the contract digests stay valid; a re-review follows the fix):
- **F1**: in `iptScopedAllow`, stop treating a negation of a loopback `-i`/`-s`/`-d` as a scope (mirror `iptPortlessCovers`'s `!` branch: a negated loopback scope leaves everything else open). Then `! -i lo`, `! -i lo+`, `! -s 127.0.0.0/8` reach `foreign` in v4 as they already do in v6. Add `TestExposureShapes` rows for `-A INPUT ! -i lo -p tcp -j ACCEPT` and `-A INPUT ! -s 127.0.0.0/8 -j ACCEPT` (both reads), plus a `TestCheckForLinux` row on the converged host asserting `manual` and `FirewallOK` false; verify red-green. If the maintainer instead keeps v4 lenient, the residual must state that a converged host reads `ok`/`firewall_ok: true` — a declared false all-clear, which the proposal's own Why rejects.
- **F2/Minor**: reorder the `withIP6` sentence so "as root" stays with the command, and update `TestWideAllowFix` with it.
- **F3/Minor**: make the v4 e2e survival assertion exact (`grep -qx` against the printed rule) like its v6 neighbour.

Spec / handoff (outside the contract set, or digest-normalised — apply without staling this review):
- **F4/Minor**: replay `features.json` (`state`/`evidence` are normalised out of the digest — verified), tick `tasks.md`'s last closing box and `proposal.md`'s four AC boxes (checkbox ticks are normalised out too), so the spec stops reading as unstarted.
- **Question**: confirm what #743 covers. If it is only `! -i tailscale0`, F1 needs its own ticket or the code fix above; if it covers v4 negated loopback, say so in `verification.md` and record the `ok`/`firewall_ok: true` consequence there.
- **AC4**: `verification.md` now records `make e2e-linux` at `f8ac37b` (102/0) — I could not re-run it, so it stays my one **UNVERIFIED** acceptance criterion; a transcript path or the guest image would make it checkable.

Do not archive yet: this verdict is FAIL and the archive gate refuses one. Once F1 is fixed with its named regression test, a fresh re-review can flip it.
