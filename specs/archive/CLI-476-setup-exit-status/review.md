---
spec: "CLI-476-setup-exit-status"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "85df6084b70f745da3c59cf87de3abb89cae45c1"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `CLI-476-setup-exit-status` — `host setup` exit status (issue mlorentedev/ts-bridge#476, PR #490).
**Sources**: `specs/CLI-476-setup-exit-status/{proposal,tasks,verification}.md` + `features.json`; PR #490 (`gh pr view 490`); diff `git diff 8ca1783e8f4b63cb5cc261147ff7814b216d4367...HEAD`; implementation commit `8ca1783` (`git show`), because the resolved base excludes it (finding F8).

### Spec and task alignment

- **Notice on scope**: the launcher-resolved base commit `8ca1783` *is* the implementation commit, so `git diff base...HEAD` is 176 lines of spec markdown and **no code**. I did not substitute a base; I read the implementation from `git show 8ca1783` and reviewed the branch's real change (5 files, +105/−11). Everything below is verified against the working tree at `85df608`, which equals the PR head committed by the launcher.
- **AC1** (failed step → exit 2, text and JSON, result still printed): met. `setupConvergence` returns `&ExitError{Code: exitNotConverged}` only for `OutcomeFailed`, in both output modes, after the result is written. Named subtests `TestHostSetupCmd_ExitStatus/a_failed_step,_text` and `/a_failed_step,_json` were red-green verified (see Evidence).
- **AC2** (ok/changed/manual → exit 0): met. Subtests `ok,_changed_and_manual,_text` and `,_json`; a mutation that widens the mapping to "any non-ok" turns them red (`exit 2, want 0`).
- **AC3** (not elevated → exit 1): met. `runHostSetup` returns a plain error before any step, `ExitCode` maps a non-`ExitError` to 1, and subtest `not elevated` asserts it. The stdout half of that assertion is vacuous (F9).
- **AC4** (ADR states it): met in the tree — `docs/adr/adr-017-host-desired-state.md` §2 carries the amendment (mapping + `setup && check` idiom); `verification.md` and the site CLI reference agree with the code. The *artifacts that gate* AC4 are weak (F2, F3).
- **Non-goals respected**: `checkConvergence` behaviour is unchanged (only its doc comment moved), no step's reported outcome changed, `host init` untouched. `host check`'s help/documentation still matches its own contract.
- **Decision fidelity**: the implementation matches the owner's option B on #476 (`2` on `failed`; `0` on `ok`/`changed`/`manual`; `1` otherwise). One part of B's wording is not carried into the ADR or a test: `unverified` (F6).
- **Contract digests**: `dotf spec digest CLI-476-setup-exit-status` equals `review-request.json` `contract_digests` for all three files — this review is not stale at `85df608`.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec folder.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | CLI help / docs | `ts-bridge host setup --help` says nothing about the new exit status, while `host check --help` documents its own (`check` got that block in #471). The contract is discoverable only in the site reference, not where a user asks the binary. | `/tmp/tsb476 host setup --help \| grep -ci "exit status"` → `0`; same for `host check --help` → `1` | UNTESTED | code (`cmd/cli/host.go` `Long`) |
| Minor | REAL | verification artifact | `features.json` `CLI-476-setup-exit-status-f4`'s verification command passes on the **unfixed base**, so it cannot detect the AC4 amendment it certifies. tasks.md ticks "non-vacuous verification command" as satisfied. | `git show 8ca1783^:docs/adr/adr-017-host-desired-state.md \| grep -n 'setup.*exit'` → exit `0` (matches pre-existing line 41, "`host setup` configures the machine and exits"); at HEAD → exit `0` | UNTESTED (the command is the gate) | spec (`features.json` — contract set) |
| Minor | REAL | spec bookkeeping | tasks.md ticks `[x] CLI reference and parity doc updated`, but the parity doc was not touched and contains no exit-status text. | `git show --stat 8ca1783` → 5 files, none is `docs/design/host-client-parity.md`; `grep -ci "exit status" docs/design/host-client-parity.md` → `0` | UNTESTED | spec (`tasks.md` tick; digests normalise ticks) |
| Minor | REAL | spec bookkeeping | tasks.md leaves `[ ] PR opened referencing this spec folder` unticked although PR #490 is open at exactly this HEAD and names `specs/CLI-476-setup-exit-status/`. | `gh pr view 490 --json headRefOid,body` → head `85df6084…`, body line "Spec: `specs/CLI-476-setup-exit-status/`" | UNTESTED | spec (`tasks.md`) |
| Minor | REAL | ADR accuracy | ADR-017 §2 justifies `manual`→0 with "on macOS, where Screen Sharing is always `manual`", but macOS `setup` returns early ("not applicable") and `setupFor("darwin")` produces zero steps; no production step emits `manual` at all today. The rationale describes the post-#459 state as if it were current. | `internal/host/step.go:65-66` (`case "darwin": return SetupResult{RDPPort: defaultRDPPort}`); `runHostSetup` early return; `grep -rn OutcomeManual internal/host` → only the constant declaration | UNTESTED | docs (`docs/adr/adr-017-host-desired-state.md`; not in the contract set) |
| Minor | THEORETICAL | exit contract completeness | The owner's option B names `unverified` explicitly ("exit 0 for `manual` and `unverified`"); the ADR's 0 clause lists only ok/changed/manual, the outcome table marks it `n/a` for setup, and no test pins `unverified`→0. Harmless today (setup cannot emit it), but a step that ever does lands on an undocumented, unpinned 0 while `check` would exit 2 for the same state. | #476 comment ("(B) … Exit 0 for `manual` and `unverified`"); ADR §2 table row `unverified \| n/a`; `grep -rn OutcomeUnverified internal/host` → `xrdpCheckStep`, firewall check only | UNTESTED | docs + tests |
| Minor | THEORETICAL | OS source of truth | `hostGOOS` in `cmd/cli` now shadows `runtime.GOOS` inside `host.Setup`/`host.Check`. Tests replace `hostSetup` wholesale, so the two dispatches are never exercised together under a non-host GOOS; a future test that sets `hostGOOS` alone would diverge silently. Production is consistent (both `runtime.GOOS`). | `cmd/cli/host.go` var block (`hostGOOS = runtime.GOOS`); `internal/host/host.go:32` (`setupFor(runtime.GOOS, …)`) | UNTESTED | code (or a comment pinning the invariant) |
| Minor | REAL | reviewable diff scope | The launcher-resolved base is the implementation commit, so the stated review scope contains none of the code the spec is about; a reviewer obeying "do not substitute the base" would certify a change it never read. | `git diff --stat 8ca1783...HEAD` → 4 spec `.md`/`.json` files; `git show --stat 8ca1783` → `cmd/cli/*`, ADR, CLI reference | n/a (process) | process (review launcher; outside this repo) |
| Minor | REAL | test quality | The `not elevated` case asserts `strings.Contains(out, "")`, which is always true — the case proves only the exit code, not that nothing was printed. | `cmd/cli/exit_code_test.go`, `{"not elevated", false, converged, nil, 1, ""}` + the shared `Contains` assertion | UNTESTED (the assertion itself) | tests |
| Question | REAL | contract on unsupported OS | On macOS the binary prints "not applicable" and exits **0** having configured nothing, which the new ADR reads as a vacuous "every step" success. Intentional per the PR body (`setup && check`), but a script cannot distinguish "setup not applicable" from "setup converged". Confirm the 0 is deliberate rather than an unexamined early return. | `/tmp/tsb476 host setup; echo $?` → `Host setup is not applicable on macOS.` / `0` (also with `--json`) | UNTESTED | docs (or spec, if the owner wants 1 here) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every acceptance criterion holds and I reproduced the mapping both ways; branches for `host.Setup` returning an error (→1) and `unverified` (→0) have no test. |
| Verification       | C | AC1–AC3 are proven red-green by named tests; AC4's harness-facing `verification` command passes on the unfixed base, so the recorded gate for it verifies nothing. |
| Scope              | A | Diff is exactly the proposal: command mapping, injectable seams, ADR §2, CLI reference — no creep, no dependency change. |
| Reliability        | B | Error ordering is deliberate (JSON write error → 1 before the convergence verdict) and commented; the real `host.Setup`/`check` dispatch is never exercised under an injected GOOS. |
| Maintainability    | B | Small functions, `exitNotConverged` named once, comments explain *why*; two OS sources of truth and package-level mutable seams are the only smell (the seams mirror the existing `check` pattern). |
| Handoff-readiness  | B | ADR amendment + CLI reference + PR `## Knowledge` section present; tasks bookkeeping is inaccurate (two false ticks, one missing) and no lesson was written (defensible: lesson 040 covers the OS-as-input point). |

Aggregation: one **C**, no **D**, no Blocker and no REAL Major → **PASS WITH GAPS**.

### Verdict

**PASS WITH GAPS**

The behavioural change is correct, minimal, and provably tested: a failed step exits 2 in text and JSON with the result still printed, ok/changed/manual exit 0, and "could not run" exits 1 — each demonstrated by mutating the mapping and watching the named subtests fail. The gaps are in the artifacts around AC4 and in the spec's own bookkeeping, not in the code.

`dotf spec archive` is **advisable** in this state (fresh, passing review, matching contract digests). Its own gate run was not observed — UNVERIFIED, out of scope by instruction.

### Recommended next steps

**Free to apply now — outside the contract set, no re-review needed:**

- Add the `Exit status:` block to `host setup`'s `Long` help, mirroring `host check` (finding 1). One paragraph in `cmd/cli/host.go`; no test pins help text either way.
- Reword the ADR-017 §2 sentence so the macOS justification is stated as the declared contract (C8/C10, #459) rather than a current code path, and add `unverified` to the setup exit mapping as option B wrote it (findings 5 and 6) — or state explicitly, as `proposal.md`'s Risks already do, that setup cannot emit it so no mapping is due.
- Tighten the `not elevated` assertion to the empty string's *intent* (e.g. assert `out.Len() == 0` or a non-empty stderr marker) (finding 9).
- Record each disposition in `verification.md` (excluded from the staleness check).

**Contract set — closed by this verdict; do not "just fix it":**

- `features.json` f4's `verification` (finding 2) is the one change that would make the AC4 gate real (e.g. grep for the amendment's own wording, or `git diff` the ADR section). It is a contract-file edit: applying it **invalidates this review** and requires a new round. Either accept it as tracked here + a follow-up ticket, or edit and re-review — do not edit it and keep this verdict.
- The two false ticks and the missing PR tick in `tasks.md` are checkbox-only, which the contract digests normalise away (confirmed: the current digest matches `review-request.json`); ticking them is safe, but re-run `dotf spec digest CLI-476-setup-exit-status` after the edit if the tool disagrees.

**Process (finding 8, outside the repo):** the launcher resolved this spec's base to its implementation commit, handing the reviewer a diff of markdown while the spec is about code. Worth fixing where the base is resolved, so the next round's stated scope actually contains the change.

### Evidence (commands run in this session, on a clean tree)

```
go build ./...                                   exit 0
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   all exit 0
go test -race -count=1 ./...                     all ok (cmd/cli 2.2s)
mise exec -- golangci-lint run                   0 issues
go test -count=1 ./cmd/cli -run TestHostSetupCmd_ExitStatus
  clean: PASS
  mutation A (setupConvergence returns nil):  FAIL a_failed_step_{text,json} "exit 0, want 2"
  mutation B (any non-ok → exit 2):           FAIL ok,_changed_and_manual_{text,json} "exit 2, want 0"
  (file restored with cp backup; `git status --porcelain cmd/cli` clean afterwards)
dotf spec digest CLI-476-setup-exit-status        equals review-request.json contract_digests
features.json verification commands f1–f4        all exit 0 at HEAD; f4 also exit 0 at 8ca1783^
/tmp/tsb476 host setup --help \| grep -ci "exit status"     0   (host check --help: 1)
```

Not run, stated as such: `scripts/tests/smoke.bats` (needs the Linux CI runner for the non-root setup case; the diff does not touch it, and this macOS host cannot execute the Linux refusal path), and the archive gate itself (out of scope).
