---
spec: "BUG-739-foreign-firewall"
verdict: "PASS"
reviewed_sha: "67fe2b28a66dc7c171f86f5f9a32c847f4332869"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: `BUG-739-foreign-firewall` — the whole range `git diff 5651418ede6c6bcd6fdd9e77778e401e2df2ab41...HEAD` (10 branch commits, 4 the spec's own; 13 files, ~550 insertions). This is round 2, read against the whole change rather than against round 1's fixes.
**Sources**: `specs/BUG-739-foreign-firewall/{proposal,tasks,verification,features.json}`, `review-request.json` (contract digests re-checked with `dotf spec digest BUG-739-foreign-firewall` — all three match, so this review describes the recorded contract), `review-round1.md`, `internal/host/linux_foreign.go`, `internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `internal/host/steps_test.go`, `scripts/e2e/linux.sh`, `docs/{adr,design,runbooks}`.

### Spec and task alignment

- Every task in `tasks.md` is ticked and has diff evidence behind it: the failing-then-passing table (`TestForeignFirewall`, 10 rows), the probe in `readForeignFirewall` ahead of the backend choice, the nft rows plus `TestNFTForeignInput` (11 rows), the fixture plumbing (`withoutProbes` + the widened `TestIP6Exposure` call list), the e2e block, and the three declared doc surfaces (ADR-017 Amendment B, the parity table, two runbook rows). No tick is unbacked.
- The design in the code is the design in the proposal: the probe runs first and returns early, so neither v4 nor v6 policy is read and no rule is written; `check` is `unverified`, `setup` is `manual`, and both name the firewall with its own commands. A failed probe leaves `read.other` empty and falls to the pre-existing unknown path (`check` `unverified`, `setup` `failed`, no write), which is AC3. `README`/ADR outcome semantics agree: ADR-017's table gives `setup` no `unverified`, so `manual` is the correct outcome for the found case, not a shortcut.
- `linuxFirewallOK` is false on a foreign read for the right reason — `state` stays at its zero value (`ruleAbsent`), `policyRead` is false with an empty backend — and `checkLinux` reads the firewall once and feeds the same value to the step and to `Services[].FirewallOK`. Nothing downstream of the step can read a foreign host as raw iptables.
- **Commands I ran fresh in this session** (all green, full output read): `go build ./...`; `go vet ./...`; `GOOS=linux|windows|darwin go vet ./internal/host/`; `gofmt -l .` (empty); `golangci-lint run ./...` → `0 issues`; `go test -race -count=1 ./...` → 19 packages ok; `bash -n scripts/e2e/linux.sh`; `scripts/tests/test-features-shape.sh` and `test-features-selectors.sh` → both exit 0. Same 1.8 MB transcript file aside, the working tree is clean.
- **Mutation probes** (each applied, observed, reverted; `git status` clean afterwards):
  - caught — hook parsing (`f[i+1] == "input"` → `"inputt"`): `TestForeignFirewall`, `TestNFTForeignInput` fail.
  - caught — dropping `mangle` from `iptNFTTables` (round 1's surviving mutation): `TestNFTForeignInput` now fails, so the round-1 fix is real.
  - caught — removing the `r.Elevated()` gate on the probe: `TestIP6Exposure` fails.
  - caught — `setup` outcome `Manual` → `Unverified`: `TestForeignFirewall` fails.
  - caught — dropping the `state == "not running"` words branch (exit-252 only, round 1's finding 3): `TestForeignFirewall` fails.
  - **survived** — `checkLinux`'s `readLinuxFirewall` → `readLinuxFirewallV4`: the entire `internal/host` suite stays green. See finding 3.
- **AC5 is not reproducible from here** (`make e2e-linux` needs a local VM through `dotf vm`, and this worktree holds none). Its evidence is the recorded run in `verification.md`/`features.json`: 107 pass / 0 fail on `999c0a7`, including the four native-nft cases. I am judging that record, and finding 1 is the one place its wording does not hold — the nft behaviour itself is exercised by the same four e2e assertions on a real `nft` ruleset, which is the strongest available proof that the parser matches the tool's real output (round 1's Major-THEORETICAL).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | verification provenance | `verification.md` says "The later commits on the branch change docs and the spec only" — but `144d34f` (`fix(host): read firewalld's not running from its words, whatever the exit`) changes production code in `internal/host/linux_foreign.go` *after* the AC5 e2e run at `999c0a7`. The nft evidence still applies (the guest has no `firewall-cmd`, so the changed switch never executes there), but the sentence is false about the history it claims | `git log --oneline 999c0a7..HEAD` (144d34f, 45429f1 post-date the run); `git show --stat 144d34f` (2 lines, `linux_foreign.go`) | n/a — evidence claim, not a behaviour | spec (`verification.md`, outside the contract set) |
| Minor | REAL | spec vs code (test guard) | The proposal's Risks promise "A fixture that does not answer it [the new `nft list chains`] shows up as a failing test, never as a silent pass". What shipped: `recordingRunner.Output` returns empty stdout and `nil` for an unanswered command, and `readNFTForeign` reads an empty listing as "no tables", so a fixture that puts `nft` on PATH and never answers silently falls through to the iptables read. `verification.md` states this behaviour correctly, which means the two spec files now contradict each other | `internal/host/steps_test.go:74–85` (no `output` key → empty, nil); `readNFTForeign`'s empty-listing branch; `verification.md` "Fixture with no answer" | UNTESTED — no test asserts that every fixture with `nft` on PATH answers the listing | tests (add the guard) + spec wording via a `verification.md` disposition |
| Minor | REAL | test coverage (check wiring) | No named Go test drives `checkFor`/`checkLinux` with a foreign firewall: `TestForeignFirewall` calls the builders directly. Bypassing the probe in `checkLinux` leaves the whole suite green, so `host check`'s `firewall_ok: false` and its `unverified` step on a foreign host rest entirely on the e2e, which by design never runs in CI | mutation run: `go test -count=1 ./internal/host/` → `ok` (0.186s) with `readLinuxFirewallV4` substituted in `checkLinux` | UNTESTED at the `checkFor` level; the only coverage is the e2e case "native nft table: check does not report firewall_ok" | tests (a `checkFor` row) |
| Minor | THEORETICAL | docs (runbook) | The #691 troubleshooting row gained "On a host filtered by firewalld or nftables instead, the report is a false alarm: confirm that firewall closes the port (#739)". After this change such a host fires the foreign path first, so that row's own message no longer reaches it in the ordinary case, and calling the new report a "false alarm" reads against the `unverified`/`manual` report the same commit introduces | diff of `docs/runbooks/guide-rdp-host-setup.md`; the early return in `readLinuxFirewall` | n/a | docs |
| Minor | REAL | docs (parity table) | A sentence was appended with no full stop: "…while ufw is disabled (#735) Where firewalld runs or a native nftables table hooks input…" | `git diff` on `docs/design/host-client-parity.md` | n/a | docs |
| Minor | SPECULATIVE | detection coverage | Detection is binary-gated: a host whose native nftables ruleset was loaded without `nft` on root's PATH is still read as raw iptables — the #739 defect, silently. The proposal declares the absent-binary skip as intended, so this is a known limit rather than drift; surfaced so the limit is a decision on record | code read of `readNFTForeign`'s `LookPath` | UNTESTED (accepted limit) | code (only if the limit is revisited) |
| Question | THEORETICAL | probe precedence | A failed `nft list chains` outranks a readable ufw/firewalld. Round 1 raised it and `verification.md` declined it with a reason ("an unusable `nft` on root means the ruleset that filters the host is unknown"), which the proposal's Risks declare. Recorded, not open — no action expected | `verification.md` Review dispositions; `TestForeignFirewall/nft_listing_fails` pins the unknown, not the precedence | `TestForeignFirewall/nft_listing_fails` | — (decision stands) |

What I could not verify, and do not claim: the firewalld hand probe on the deleted `probe739` guest is unreproducible here, so the firewalld rows rest on named unit tests and the author's recorded account; AC5's e2e run is likewise taken from the record, not re-run by me. No Blocker and no REAL Major: on every path the tests and mutations exercise, the code does what the proposal says, and the destructive paths (write, policy read, v6 scan) are all correctly withheld.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC4 verified by named tests I ran and by six mutations (five caught); AC5 rests on a recorded e2e run I cannot reproduce, and the check-side wiring is only covered there. |
| Verification       | B | Build, vet (three OSes), gofmt, lint, the full race suite and the shell/features checks all reproduced green; the one blemish is the AC5 provenance sentence (finding 1). |
| Scope              | A | The diff is exactly the probe, its step wiring, its tests, the e2e case and the three declared doc surfaces — nothing else. |
| Reliability        | B | Foreign hosts never get a write, a policy note or a v6 scan; failed probes degrade to the unknown path; the binary-gated detection and the ufw-outrank decision are stated limits, not surprises. |
| Maintainability    | A | 136-line new file of small single-purpose functions (all well under 40 lines, CC far below 10), names that say what they mean, WHY-comments on each non-obvious branch, no dead code. |
| Handoff-readiness  | B | ADR/parity/runbook updated in-session and the promotion questions answered; the two spec-accuracy nits (findings 1 and 2) and the two doc nits are what keep this off an A. |

### Verdict

PASS — no Blocker, no REAL Major, every open finding is a Minor (one SPECULATIVE) or an already-dispositioned Question, and the rubric has no C or D.

### Recommended next steps

Every item below is **outside the contract set** (`proposal.md`, `tasks.md`, `features.json` stay untouched; their digests already match `review-request.json`), so this verdict stands while they are applied:

- **`verification.md`** — correct the AC5 provenance sentence (finding 1): either state that the run covers `999c0a7` and explain why `144d34f` cannot affect the native-nft path (the guest has no `firewall-cmd`), or re-run `make e2e-linux` at HEAD and paste the summary. Then record a disposition for finding 2 — the proposal's "a fixture that does not answer shows up as a failing test" is not what shipped.
- **Tests** — a `checkFor`-level foreign row so `host check`'s `firewall_ok: false` and `unverified` step are pinned in CI (finding 3, the mutation that survived); and a guard that every fixture putting `nft` (or `firewall-cmd`) on PATH answers the new probes, which is the safety net the proposal promised (finding 2).
- **Docs** — the missing full stop in `docs/design/host-client-parity.md`; and decide the fate of the "false alarm" sentence added to the #691 row of `docs/runbooks/guide-rdp-host-setup.md` (keep, reword, or drop as superseded by #739).
- **Then archive**: tick the archive checklist in `verification.md` and run `dotf spec archive BUG-739-foreign-firewall`. The review gate is satisfied by this file; no further review round is needed for the contract as recorded.
