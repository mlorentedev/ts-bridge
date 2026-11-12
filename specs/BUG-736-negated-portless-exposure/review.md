---
spec: "BUG-736-negated-portless-exposure"
verdict: "FAIL"
reviewed_sha: "d167344ac0542cdb867539ae6bff0625578f0d25"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: BUG-736-negated-portless-exposure (`git diff 46b921c1a5c914f517582599bbda05c30386d59b...HEAD`, 10 files, +309/−15)
**Sources**: `specs/BUG-736-negated-portless-exposure/{proposal,tasks,verification}.md`, `review-request.json`, `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/adr/adr-017-host-desired-state.md`, `docs/design/host-client-parity.md`, `docs/lessons/lesson-055-2026-11-04.md`

### What was verified, and how

| Claim | Command | Result |
|---|---|---|
| Contract digests match the request | `dotf spec digest BUG-736-negated-portless-exposure` | identical to `review-request.json` (proposal/tasks/features.json) |
| Build | `go build ./...` | exit 0 |
| Full suite under race | `go test -race -count=1 ./...` | all packages `ok`, 0 failures |
| Formatting / vet | `gofmt -l .`, `go vet ./...` | no files listed; vet clean |
| Linter | `golangci-lint run ./internal/host/...` | `0 issues` |
| New tests actually bite (red-green) | mutated `iptExposes` back to `iptPortCovers` only, re-ran the named tests, reverted | `TestExposureShapes` (v4 and v6) and `TestReadIptablesUnscoped/an_accept_for_every_port_but_SSH_is_foreign` went red, then green on revert; tree restored |
| AC4 (Linux VM e2e) | not run — no throwaway VM in this session | **UNVERIFIED** (no transcript recorded in `verification.md` either) |

### Spec and task alignment

- **AC1 (v4 three shapes → `manual`, named, `firewall_ok` false, SSH caution, setup deletes none)** — met at unit level: `TestExposureShapes`, `TestReadIptablesUnscoped` (`a port-less accept is foreign`, `an accept for every port but SSH is foreign`), `TestCheckForLinux` (`…! --dport 22…` → `manual` + caution), `TestSetupForLinux` (`-A INPUT -j ACCEPT` → tailscale ok / firewall `manual` / xrdp ok, fix carries `keep loopback, replies and SSH open first`). "Deletes none" follows from `legacyDeletes` only matching setup's own line, and is pinned by the setup row's call list.
- **AC2 (v6 same three shapes)** — `ip6Exposed` now routes through `iptExposes`; `TestExposureShapes` runs every row through both reads, and `TestIP6Exposure` adds the portless and negated-port rows with `manual` + the `after you keep loopback…` fix.
- **AC3 (not reported: `! --dport 3389`, `-i lo`, `-i eth0`/`-s`, conntrack-only, UDP portless, `--sport`-only)** — all present as `false` rows in `TestExposureShapes`. One shape that AC3 does **not** list and that the implementation also misses is a real open port — finding F2.
- **AC4 (e2e)** — script rows exist (`scripts/e2e/linux.sh`, 14 added lines) and read correctly, but nothing shows they ran; finding F3.
- **Non-goals respected**: `iptablesDecide` untouched (only `iptExposes` reached it indirectly? no — `iptMatchesPort` is unchanged, so `iptablesDecide` semantics are identical); chain-jump and order limits untouched.
- **Unstated behaviour change**: `scoped` now reads `false` for a portless accept, so `setup` appends its own scoped rule where it previously added none — correct and necessary, but absent from `proposal.md` and the ADR sentence — finding F4.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | wrong operator instruction (mixed rule syntax) | `foreignFix` runs `iptAdmitsOthers` over `read.foreign`, but on a ufw host that slice also holds `ufw show added` lines (`linux_steps.go:356,367`). A ufw line never carries `--dport`, so **every** foreign ufw allow is judged "admits other ports too" and the v4 iptables caution is appended, with a false claim and iptables commands on a ufw-managed host. `ufw allow 3389` (bare port, outside setup's legacy form `ufw allow 3389/tcp`) is enough. | Reproduced through the real read path: `readUFW` → `legacyCheckStep` on `ufw allow 3389` returns `manual` with `Fix = "scope ufw allow 3389 to tailscale0, or remove it; …; the rule admits other ports too, so as root keep loopback, replies and SSH open first (`iptables -A INPUT -i lo -j ACCEPT`, …) before you change it"`. Also `iptAdmitsOthers("ufw allow 3389/tcp")` → `true`. | **UNTESTED** — the only ufw-foreign row (`linux_steps_test.go:1561`) asserts the substring `setup does not delete rules it did not write`, so it stays green with the bogus suffix; no test asserts the absence of the caution for a rule that names the port | code (`iptAdmitsOthers` must refuse non-iptables lines, or be fed only `ipt.foreign`) + tests (a row pinning the exact fix for `ufw allow 3389`) |
| Major | REAL | false negative in the exposure predicate | `-A INPUT -p tcp -m tcp ! --dport 22 --sport 1024:65535 -j ACCEPT` admits 3389 from every ordinary client (ephemeral source ports are 1024–65535), but `iptExcludesOthers` returns `false` because the `--sport` match is not negated — the loop bails on the first non-negated port flag. A real port on every interface stays unreported, which is the class of bug this spec closes. | Netfilter semantics of the rule; the new table pins the miss with `want=false`. My mutation run confirms the row is exercised in both reads. | `TestExposureShapes/-A_INPUT_-p_tcp_-m_tcp_!_--dport_22_--sport_1024:65535_-j_ACCEPT` (asserts the miss, i.e. it locks the gap in) | code (`--sport` should not defeat a negated destination port, or should be modelled as narrow-by-value) + tests + spec (proposal wording "every port match it carries is negated") |
| Major | REAL | verification artifacts / handoff | `verification.md` is the untouched template: criteria still mapped to `` `<hash>` / `<name>` ``, no test-status command output, and every `Promotion candidates` line unanswered — `dotf spec archive` refuses unanswered lines (gate itself not run: **UNVERIFIED**). `tasks.md` has **every** box unticked although the code, both test layers, the e2e script and the docs are in the diff, so the spec reads as unstarted while the branch is at `verifying`. | File contents in the reviewed tree; diff shows `verification.md` added at exactly the 46-line template size. | **UNTESTED** | spec (`verification.md` is outside the contract set; the `tasks.md` ticks are inside it and normalised out of the digest, so ticking them does not invalidate this review by itself — the contract is still being edited for F4, so re-review follows anyway) |
| Minor | REAL | spec drift | The change also flips `scoped` for a portless accept (`TestReadIptablesUnscoped/"a port-less accept is foreign"` now expects `scoped false` where it expected `true`), so on such a host `setup` now appends `-A INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT` where it previously added nothing. Neither `proposal.md` ("`iptablesDecide` is unchanged") nor the ADR sentence says the write path changes. | Test-row diff + `iptUnscopedRead`'s `scoped` computation, which now excludes the portless line from `walked`. | `TestSetupForLinux/"iptables, an accept for every port: scoped rule added, nothing deleted, manual (#736)"` (pins the behaviour, not the documentation of it) | spec (proposal "What") + ADR-017 wording |
| Minor | REAL | test strength | The new table asserts booleans and the step rows assert *substrings* of `fix`. Nothing pins that a rule which does name the port does **not** receive the SSH caution — exactly the surface F1 broke on ufw hosts. | `linux_steps_test.go` added rows use `fix:` substrings (`"keep loopback, replies and SSH open first (`iptables -A INPUT -i lo -j ACCEPT`"`). | **UNTESTED** (no exact-fix assertion exists) | tests |
| Minor | SPECULATIVE | over-report | A negated *source* port on its own (`! --sport 1024:65535`, matching only privileged source ports) is reported as an exposure. Documented as accepted in the proposal ("a negated-port rule with an unmodelled narrowing is reported"), so surface-only; the cost is an extra `manual` on a very unusual rule. | `TestExposureShapes` `--sport`-negated row (`! --sport 22` → true) | `TestExposureShapes/-A_INPUT_-p_tcp_-m_tcp_!_--sport_22_-j_ACCEPT` | — (surface only; do not gate) |
| Question | — | AC4 evidence | The e2e rows are plausible (they mirror the #526 v6 block, delete by `-D`, and assert `firewall_ok` false via JSON), but I cannot run `make e2e-linux` here and no transcript was recorded. Confirm the run happened before archiving, or record it in `verification.md`. | `scripts/e2e/linux.sh` diff; absent from `verification.md` | **UNTESTED** (shim suites `scripts/tests/test-e2e-*.sh` are CI's only proof of harness logic; they were not run here) | spec (`verification.md` evidence) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | AC1/AC2 hold and the predicate is sound for the named shapes, but two REAL defects: a wrong operator fix on ufw hosts (F1) and a missed open-port shape (F2). |
| Verification       | D | `verification.md` is an empty template: no criterion mapped to a commit, test or output, no test-status command, e2e unrecorded (F3). |
| Scope              | B | Diff matches the proposal's file set (predicate, tests, e2e rows, parity doc, ADR, lesson); the undocumented `scoped` flip is the one drift (F4). |
| Reliability        | C | No crash paths, but the change's user-visible payload is the fix text, and one host class now receives a false claim plus iptables commands (F1). |
| Maintainability    | B | New helpers are small and commented (`iptExposes`, `iptScopedAllow`, `iptExcludesOthers`, `keepOpenFirst`), CC well under 10, duplicated predicate logic confined to two call sites; the ufw/iptables line-format conflation in `iptAdmitsOthers` is the smell. |
| Handoff-readiness  | C | ADR, parity doc and lesson updated well, but `tasks.md` unticked and `verification.md` unsubstantiated — the spec does not yet carry the evidence its own archive gate wants. |

### Verdict
**FAIL** — a **D** in Verification (rubric rule) plus two **REAL Majors** (F1, F2). The predicate itself is good work and the red-green evidence is strong; the failures are in the message path this change adds and in the spec's own evidence trail.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md` — a re-review follows):
- F4: state in `proposal.md` that a portless unscoped allow no longer counts as the scoped rule, so `setup` appends its own `-i tailscale0` rule (behaviour already pinned by a test).
- F3: tick the `tasks.md` boxes that the diff actually delivers, then get re-reviewed once the contract is final.

Outside the contract set (apply now, no re-review needed for these alone):
- F1 (code + tests): make `iptAdmitsOthers` refuse lines that are not iptables rules (e.g. require the line to start with `-A`), or restrict the guard to `ipt.foreign`; add a `TestCheckForLinux`/`TestSetupForLinux` row for a ufw host holding `ufw allow 3389` asserting the exact fix, and one asserting a port-naming foreign rule gets no caution.
- F2 (code + tests): treat a `--sport` match as not defeating a negated destination port (or model source-port values); add a row to `TestExposureShapes` expecting the `! --dport 22 --sport 1024:65535` shape to be reported, and keep the `--sport`-only `false` row.
- F3/F6 (`verification.md`): map AC1–AC4 to the named tests and the red-green mutation above; record the `make e2e-linux` run (exit, guest, the five new assertions) or mark AC4 explicitly unverified with the reason.
- F5: pin the full `fix` string in the two new step rows rather than a prefix, so an appended sentence cannot pass silently.

Do not archive yet: the archive gate will refuse an unfilled `verification.md` and an unanswered promotion block regardless of this verdict.
