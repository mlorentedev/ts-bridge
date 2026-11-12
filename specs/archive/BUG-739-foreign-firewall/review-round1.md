---
spec: "BUG-739-foreign-firewall"
verdict: "FAIL"
reviewed_sha: "0342f1dbcf4c198d10d65278766930771f153bb6"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-739-foreign-firewall` — whole range `git diff 5651418ede6c6bcd6fdd9e77778e401e2df2ab41...HEAD` (6 branch commits, 2 of them the spec's own; 464 insertions, 11 files).
**Sources**: `specs/BUG-739-foreign-firewall/{proposal,tasks,verification,features.json}`, `review-request.json` (contract digests checked with `dotf spec digest` — all three match), `internal/host/linux_foreign.go`, `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `scripts/e2e/linux.sh`, `docs/{adr,design,runbooks}`.

### Spec and task alignment

- The diff matches the proposal's four-part design: a probe ahead of the backend choice (`readForeignFirewall`), an early return so no policy note, no v6 scan and no write (`foreignStep`), `manual` for setup / `unverified` for check, and the fix naming each tool's own commands. No scope creep: ADR-017 Amendment B, the parity table and the two runbook rows are the ones `tasks.md` declares.
- AC1–AC4 are implemented and pinned by named tests I ran (`TestForeignFirewall` 9 subtests, `TestNFTForeignInput` 9 subtests, plus `TestIP6Exposure` for ordering): **all pass**.
- **AC5 is not verified.** `verification.md` says "run pending"; `tasks.md` leaves `make e2e-linux` and "verification.md filled; spec review committed" unticked; `features.json` carries `f2.state: pending` with an empty `evidence`; every acceptance box in `proposal.md` is still `[ ]` and the spec `status:` is `implementing`. The e2e block exists in `scripts/e2e/linux.sh` (`bash -n` clean) but has never executed against a guest.
- The declared design decisions hold up under reading: `setup` is `manual` because ADR-017 gives setup no `unverified` outcome; a failed probe lands on the pre-existing `ruleUnknown` path (`ruleUnknown` is the zero value), so AC3 is met without new machinery (`firewallUnreadable` → check `unverified`, setup `failed`, no write). `checkLinux` reads the firewall once and feeds both the step and `FirewallOK: linuxFirewallOK(fw)`, so a foreign host cannot be reported as `firewall_ok`.
- The `ip`/`ip6` standard-name exclusion is sound, not luck: `tsInputInNFT`'s own comment records that tailscaled's nftables mode writes `ip filter`, which iptables-nft would share and iptables-legacy never prints — so refusing to call `ip filter` foreign preserves the existing nftables-mode path (the full suite still passes on that fixture).

**Commands I ran in this session** (all fresh, all green): `go build ./...`; `gofmt -l .` (empty); `go vet ./...` and `GOOS=linux|windows|darwin go vet ./...`; `golangci-lint run ./internal/host/...` → 0 issues; `go test -race -count=1 ./...` → 19 packages ok; `go test -v -run 'TestForeignFirewall|TestNFTForeignInput'` → 18/18 subtests pass; `bash -n scripts/e2e/linux.sh`; `bash scripts/tests/test-features-shape.sh` (56 cases OK) and `test-features-selectors.sh` (64 cases OK).

**Mutation probes** (applied, observed, reverted; working tree left clean apart from this review): (a) removing the `r.Elevated()` guard around the probe → `TestIP6Exposure` fails ("an unprivileged read ran [lookpath firewall-cmd lookpath nft]"), so the privilege gate and head-of-read position are genuinely pinned; (b) setting the `firewalld not running` row to `fwExit: 0` → the read becomes `Cannot read the firewall: firewall-cmd --state: printed "not running", neither running nor not running` and setup goes `failed`, which reproduces finding 3; (c) dropping `mangle`/`raw` from `iptNFTTables` → the whole `internal/host` suite stays **green**, which reproduces finding 4.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | verification / AC5 | AC5 is written-but-unrun: nothing in the repo proves the probe behaves on a real host — and this change inserts `nft list chains` into *every* elevated read, which is exactly what can flip the many pre-existing e2e assertions (`fwstep ok`, `firewall_ok`) on a guest that turns out to carry an `inet` table the author did not anticipate | `verification.md` AC5 "[ ] … run pending"; `tasks.md` closing items `[ ] make e2e-linux`, `[ ] verification.md filled`; `features.json` f2 `state: pending, evidence: ""` | `scripts/e2e/linux.sh` "native nft table" block — **never executed** | tests (`make e2e-linux`) + `verification.md` |
| Major | THEORETICAL | nft parser | The `nft list chains` text format has no captured-from-the-tool golden: the only in-repo samples are the implementer's own constants (`nftInetFilter`, `nftFirewalld`, `nftIptOnly`, `nftNoInput`). `nftForeignInput` needs `chain <name> {` followed by a `type … hook input …` line in the same block; if a real nft prints the declaration differently, the parser returns empty, the probe reports "nothing found", and iptables is read again — the exact #739 defect, silently, with nothing to alert on | code read of `nftForeignInput`; fixtures self-authored; the only real-bytes evidence is the deleted-guest hand probe described in `verification.md` | `TestForeignFirewall` / `TestNFTForeignInput` (**fixtures do not come from the tool**) | tests (commit a real `nft list chains` capture; the AC5 run produces one) |
| Minor | THEORETICAL | firewalld probe | "not running" is recognised *only* by exit 252. A version that prints `not running` with exit 0 falls into the `err == nil && state != "running"` branch, which reports `printed "not running", neither running nor not running` and makes a firewalld-installed-but-stopped host `setup: failed` on every run. The string is the definitive signal; the exit code is a convention. No such firewalld version was observed — 252 is documented behaviour since 0.4 — so this is a robustness gap, not a reproduced regression | mutation (b): setting the existing row to `fwExit: 0` reproduces the failure message and the `failed` setup | **UNTESTED** — `TestForeignFirewall` has only the `fwExit: 252` row | tests + code (accept `state == "not running"` regardless of exit; add the row) |
| Minor | REAL | test coverage | `iptNFTTables` lists five names but only `filter`, `nat` and `security` are exercised. Dropping `mangle` or `raw` would flag ordinary iptables-nft hosts as foreign, and the suite would not notice | mutation (c): `{"filter", "nat", "security"}` → `go test ./internal/host/` green | **UNTESTED** for `mangle`, `raw` | tests (two rows in `TestNFTForeignInput`) |
| Minor | REAL | handoff-readiness | The spec is not in an archivable state: `status: implementing`, ACs unticked, archive checklist untouched, `features.json` has no trailing newline. The bodies match the code, so this is bookkeeping, not drift | `proposal.md` frontmatter + ACs; `verification.md` Archive checklist; shape tests pass, so nothing enforces the ticks | n/a | spec (status/AC ticks are normalised out of the contract digests; the archive checklist lands in `verification.md`) |
| Question | THEORETICAL | probe ordering | A failed `nft list chains` aborts the whole read, so on a ufw host whose `nft` is unusable the ufw answer (authoritative before this change) is replaced by `failed`/`unverified`. The proposal declares this ("the read is unknown"), so it is a decision and not a violation — confirm it is still the one wanted now that it can outrank a readable ufw | `readLinuxFirewall` early return; proposal Risks | `TestForeignFirewall/nft_listing_fails` (pins the unknown, not the ufw precedence) | code (only if the decision changes) |

What I could not verify, and do not claim: the firewalld hand probe on Ubuntu 24.04 (`probe739`, since deleted) is unreproducible here, so the firewalld rows rest on unit fixtures and the author's account; `make e2e-linux` needs a local VM and was not run by me or (per `verification.md`) by the implementer. No Blocker found: on every path the tests exercise the code does what the proposal says, and `linuxFirewallOK`, `policyRead` and the message builders are correctly left unable to read a foreign host as raw iptables (pinned by `TestForeignFirewall`).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC4 verified by named, passing tests; AC5 unverified and the nft format is pinned only by self-authored fixtures. |
| Verification       | C | I re-ran build/vet/lint/tests green, but one of five ACs has no evidence at all and the e2e that would prove the real-tool contract never ran. |
| Scope              | A | Diff is exactly the probe, the step wiring, its tests, the e2e case and the three declared doc rows; nothing unrelated. |
| Reliability        | B | Failed probes fall to unknown, setup never writes, no policy note or v6 scan on foreign hosts; the firewalld exit-code coupling is the one brittle spot. |
| Maintainability    | A | New file is small functions with WHY comments, CC well under 10, clear names, no dead code; `withoutProbes` is documented where it widens the call tables. |
| Handoff-readiness  | C | ADR/parity/runbook updated in-session, but the spec still reads `implementing` with ACs, closing tasks and the e2e marked undone. |

### Verdict

FAIL — one **Major / REAL** open (AC5 has no evidence; the e2e never executed) and one **Major / THEORETICAL** (no captured real `nft list chains` fixture) that the same run would close. Rubric alone is PASS WITH GAPS (two Cs, no D); `severity × reality` is the stricter path. Archive is **not** advisable in this state.

### Recommended next steps

- **Before the re-review (tests + verification, outside the contract set):** run `make e2e-linux` and paste the relevant `ok`/failure lines into `verification.md` AC5; capture one real `nft list chains` (and one real `firewall-cmd --state` failure) as a committed fixture, so the parser's format is evidence rather than an assumption. Add the three cheap test rows from the table (`not running` with exit 0; `ip mangle`; `ip raw`).
- **Contract set (only because this verdict is FAIL; a re-review follows):** tick the ACs in `proposal.md`, tick `tasks.md`'s two closing items, move f2 to `done` with its evidence in `features.json`. Checkbox ticks, the frontmatter `status:` and the harness-owned `state`/`evidence` are normalised out of the contract digests, so they do not orphan this review — but do not reword the `proposal.md`/`tasks.md` prose.
- **Then:** `dotf spec review BUG-739-foreign-firewall` again — the implementer cannot be the reviewer, and `harness/reviewer-pool.json` is the allow-list. On a clean round, tick the archive checklist and `dotf spec archive BUG-739-foreign-firewall`.
- **Disposition in `verification.md`:** the failed-probe-outranks-ufw question — apply, decline with a reason, or ticket. It is already the proposal's declared decision, so "declined, decided in the proposal" is a legitimate disposition.
