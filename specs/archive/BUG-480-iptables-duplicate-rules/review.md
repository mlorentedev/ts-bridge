---
spec: "BUG-480-iptables-duplicate-rules"
verdict: "PASS"
reviewed_sha: "38f3cd73e1b8b09bd2c425e6308cb0fa51e697ee"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-01"
---

## Adversarial review

**Scope**: `BUG-480-iptables-duplicate-rules` — `host check` reporting copies of setup's own iptables rule.
**Sources**: `specs/BUG-480-iptables-duplicate-rules/{proposal,tasks,verification,features}.md`, `git diff 41f5a0694b3c731387c55b86e4b8f7718d19aeea...HEAD` (`internal/host/linux_steps.go`, `internal/host/linux_steps_test.go`, `docs/design/host-client-parity.md`, `docs/lessons/lesson-051-2026-11-01.md`).

### Spec and task alignment

- `proposal.md` declares AC1–AC6; `tasks.md` marks every implementation and closing box `[x]`; `verification.md` maps each AC to a named test and records a mutation log. Contract digests match `dotf spec digest` (proposal `07639a…`, tasks `fc26a5…`, features `88ef5a…`), so this review is against the current contract.
- Implementation matches the proposal's *What* word for word: `readIptables` carries `copies`, `firewallCheckStep` tests `copies > 1` **after** the `ruleUnknown` and `ruleBlocked` branches, `firewallCopiesStep` reports `manual` with count + `-D` fix, `firewallSetupStep` untouched, `FirewallOK` still `state == rulePresent`.
- The one deliberate divergence from the ticket (outcome `manual` instead of `changed`, exit 0 → 2 for affected hosts) is argued in `proposal.md` §Risks and in lesson 051; it is a documented decision, not spec drift. I reproduced the exit-code consequence (below), so the documented risk is real and correctly stated.

**Independent verification I ran (not read):**

| Claim | How I checked | Result |
|---|---|---|
| Suite green | `go test -race -count=1 ./...` | all 12 packages `ok` |
| Build | `go build ./...`; `GOOS=linux GOARCH=arm64 go build ./cmd/ts-bridge/` | exit 0 |
| Vet, three OSes | `GOOS=linux\|windows\|darwin go vet ./...` | clean |
| Lint | `golangci-lint run ./internal/host/...` (2.14.0, the `mise.toml` pin) | `0 issues` |
| Formatting of the diff | `gofmt -l internal/host` | empty (`cmd/cli/status*.go` are unformatted **at the base commit too** — pre-existing, not this change's) |
| AC1/AC2/AC5 end to end | `ubuntu:24.04` + `NET_ADMIN`, two copies appended with setup's own command line, then `host check` / `host setup` / `iptables -D` | `check` = `manual`, message "holds 2 copies of -A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT", fix "`iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` 1 time(s)", `firewall_ok=true`, exit 2; `setup` = `ok` and both copies still present; after one `-D`, `check` = `ok` |
| AC6 printed form | appended the rule twice with `iptables -A INPUT -p tcp --dport 3389 -j ACCEPT` in the container and read `-S INPUT` | identical `-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT` lines; **also true under `iptables-legacy`** (checked both binaries) |
| Fix command validity | ran exactly `iptables -D INPUT -p tcp --dport 3389 -j ACCEPT` on the two-copy ruleset | `DELETE_OK`, one copy left |
| Red first (M1) | stubbed `iptSetupCopies` to return 0 | 11 `--- FAIL` lines: the two copy check rows plus seven `TestIptablesSetupCopies` rows — matches `verification.md` |
| "Only the exact form counts" (M3) | replaced equality with a loose `--dport <port> … -j ACCEPT` match | `TestCheckForLinux/iptables, one copy beside a scoped allow`, `TestIptablesSetupCopies/a scoped allow is not a copy`, `…/setup's own command line is not the printed form` fail |

Both mutants were reverted; `git status` is clean apart from this review and the launcher's own `review-request.json` / transcript.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|---|---|---|---|---|---|---|
| Minor | REAL | UX / text report | On a copies host the text report says `Firewall: ✓ OK` in the summary block and `! Firewall configuration (manual)` in the same block. Both are intended (`firewall_ok` is deliberately true), but a reader sees a green row and a warning about the same subsystem with no wording linking them. | My container run output: `Firewall: ✓ OK` immediately followed by `! Firewall configuration (manual): run `iptables -D …``. `cmd/cli/host.go:494` prints `statusStr(r.FirewallOK)`; `:495-499` prints non-`ok` steps. First step where these two disagree (BUG-481's blocked step clears `FirewallOK`, so it never showed before). | UNTESTED for the text wording; `TestHostCheckCmd_ExitStatus` covers JSON/exit only | code (text report wording) or declared acceptable — deferrable |
| Minor | THEORETICAL | Coverage gap | On a host that ran pre-#477 setup **and later installed ufw**, `readLinuxFirewall` prefers ufw, so the accumulated iptables copies are never counted and the drift stays silent — the exact failure #480 names. The proposal's out-of-scope reason ("ufw never piles up copies") addresses ufw piling up, not copies that predate ufw. | Code read: `internal/host/linux_steps.go:168-181` returns the ufw read before `readIptables`; no test covers "ufw installed + iptables copies". Not reproduced (needs a pre-#477 host then a ufw install). | UNTESTED | spec (widen or re-justify the ufw non-goal) — deferred to a follow-up ticket; do **not** edit `proposal.md` under this verdict |
| Minor | REAL | Handoff | `features.json` still has `state: "pending"` and `evidence: ""` for all four entries although all four verification commands pass (I re-ran each: f1–f4 OK). Machine-readable evidence is missing while `verification.md` claims it. | `specs/BUG-480-iptables-duplicate-rules/features.json`; commands re-run → all pass. `state`/`evidence` are outside the digest normalisation, so recording them does not stale this review — confirm with `dotf spec digest` afterwards. | n/a (artifact state) | spec (run `dotf spec audit`) |
| Minor | THEORETICAL | Maintainability | `iptSetupPrinted` derives the printed form as `slices.Insert(rule, slices.Index(rule, protoTCP)+1, "-m", "tcp")`. If `iptablesAppend` ever loses `"tcp"` from its own args, `Index` returns `-1` and the insert lands at position 0, silently producing a garbage form (count always 0) instead of failing loudly. | `internal/host/linux_steps.go:237-240`. Behaviour is currently correct (`Index` → 3). Guarded only indirectly: changing the proto would fail `TestIptablesSetupCopies/{one,two,three…}`, so the failure is caught, but the failure mode is a silent zero rather than a diagnostic. | `TestIptablesSetupCopies` (indirect guard) | code (look up the flag value, not the bare word) — optional |
| Minor | REAL | Spec hygiene | `proposal.md`'s six AC checkboxes are still `[ ]` while every closing box in `tasks.md` is `[x]` and `verification.md` records each AC. Cosmetic, and `proposal.md` is contract-set: editing it under this verdict would stale the review, so this is recorded for the next round only. | `proposal.md` §Acceptance criteria vs `tasks.md` §Closing | n/a | spec — **do not** apply now; note for a future round |

No Blocker and no REAL Major was found. Nothing in the diff touches injection surface, secrets, auth, blocking I/O or unbounded work: the only new code is a field-count comparison and two `fmt.Sprintf`s; complexity is 2–3 per new function (well under 10), all functions < 40 lines, no dead code.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | Every AC independently verified, including an end-to-end run and two mutants proving the negative paths are enforced. |
| Verification       | A | Evidence is reproducible: I re-ran the suite, vet, lint, the four `features.json` commands and the container scenario; all match `verification.md` (the M1 failure count matches exactly). |
| Scope              | A | Diff is the change plus its lesson, parity table and spec triad; no unrelated edits (`cmd/cli/status*.go` formatting is pre-existing at the base). |
| Reliability        | B | Check path stays read-only, setup path untouched and idempotent; the ufw-preferred backend leaves one silent path for pre-existing copies. |
| Maintainability    | B | Small, named, table-tested functions with a WHY comment; `iptSetupPrinted`'s index arithmetic is one silent-zero hazard. |
| Handoff-readiness  | B | Lesson 051 + parity row capture the decision; `features.json` evidence left `pending`. |

### Verdict

PASS

### Recommended next steps

Everything below is **outside** the contract set (`proposal.md`, `tasks.md`, `features.json` are contract-set only for `proposal`/`tasks`; `features.json` `state`/`evidence` are excluded from the digest). Disposition each in `verification.md` as applied, ticketed, or declined with a reason — do not edit `proposal.md` or `tasks.md` under this verdict, or the review becomes stale.

- **Text report (Minor, REAL)**: either phrase the summary row so `Firewall: ✓ OK` cannot read as "no firewall action needed" (e.g. derive it from the step outcome, or add "port allowed; 1 copy drift — see below"), or record it as an accepted trade-off justified by `firewall_ok`'s meaning. If changed, add a text-mode assertion to `TestHostCheckCmd_ExitStatus`.
- **ufw-preferred backend (Minor, THEORETICAL)**: file a follow-up ticket for "copies accumulated before ufw was installed are invisible", or extend `proposal.md`'s ufw non-goal in a later round. Not blocking.
- **`features.json` evidence (Minor)**: run `dotf spec audit BUG-480-iptables-duplicate-rules` (or the supported evidence path) so the four entries leave `pending`; re-check `dotf spec digest` afterwards to confirm the review stays fresh.
- **`iptSetupPrinted` (Minor, optional)**: derive the insertion from the `-p` flag's value (`iptFlag(rule, "-p")`) rather than `slices.Index(rule, "tcp")`, so a future protocol change fails a test rather than counting nothing.
- `dotf spec archive` is **advisable** once the two artifact-state items above are dispositioned; nothing in this review blocks it. I could not exercise the archive gate itself (`dotf spec archive` has no `--dry-run` and this review was not to be applied): gate behaviour beyond what you observe is **UNVERIFIED** by me.
