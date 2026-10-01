---
spec: "CLI-306-init-auth-key-file"
verdict: "PASS"
reviewed_sha: "b9d37ff75e3ed6c645d5529ed2db808b955e18c7"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-01"
---

## Adversarial review

**Scope**: `CLI-306-init-auth-key-file` — implementation PR #356 (squash commit `9d256e9`), reviewed at HEAD `b9d37ff`.
**Sources**: `specs/CLI-306-init-auth-key-file/{proposal,tasks,verification}.md` + `features.json` + `review-request.json`; `git show 9d256e9` (13 files, +415/−57; production **+35/−18 in `cmd/cli/init.go`**, +4 in `cmd/cli/connect.go`, 165 lines of tests, docs and spec artifacts); the code as it stands at `b9d37ff`. Commands executed in this session are quoted inline.

### Spec and task alignment

- All four acceptance criteria are ticked, and each tick is backed by something I re-ran rather than by assertion (see findings/evidence). No `[x]` is unsupported.
- `tasks.md` gating rule is not violated: `features.json` carries `"state": "pending"` with empty `evidence` on all four entries — the conservative state, not `passing` with empty evidence. I treat this as harness-owned and **recommend no edit** (editing it would stale this review for no verification gain).
- Promotion section: all three candidates answer `no: <reason>`, which passes the archive pre-flight's `promotionAnswer`/`judgePromotion` shape (reason non-empty, no placeholder).
- Tag pre-flight: no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` in any non-review-state spec file. The literal occurs inside `review-transcript.jsonl`, which `IsReviewState` skips; I confirmed this empirically with a scratch repo (`dotf spec archive TEST-001-x` → *"no review.md in the spec folder"*, i.e. the tag check had already passed, not refused).
- Contract-stability check (this review's own freshness): `tasks.md` and `proposal.md` reproduce the digests recorded in `review-request.json` exactly under the launcher's documented normalization (checkbox ticks, status, CRLF folded). `features.json` did not reproduce under any model of the JSON normalization I tried — raised as a Question below, with the content-stability evidence that makes it non-blocking.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | scope / spec drift | `cmd/cli/connect.go` changed although `proposal.md` "Out of scope" excludes changes to `connect`: the shared `readAuthKeyFile` now rejects embedded line breaks, so `connect` and `browser` gained a new failure mode. Mitigating: AC1 asks for malformed files to fail clearly, and the proposal's own risk item asks to reuse that helper; the change is a strict hardening (a valid key has no line break). What is missing is the record, not the code — `verification.md` "Decisions made during implementation" does not mention it. | `git show 9d256e9 -- cmd/cli/connect.go` (+4 lines: `strings` import + `ContainsAny(key, "\r\n")` guard); proposal.md *Out of scope* bullet | `TestInitAuthKeyFile/embedded_newline_returns_clear_error` (helper-level, driven through `init`; no `connect`-path case) | spec (verification.md disposition — **not** proposal.md) or a follow-up ticket |
| Minor | REAL | docs / tests | AC3 claims README, runbook and site docs describe the workflow, and those files were updated by hand — but no test pins them, while this repo already pins docs this way (`TestBootstrapSSHDocumentation`, `TestManagedCredentialDocumentation` assert required strings per file). The stale "init has no `--auth-key-file`" wording was removed by hand and can return unflagged; only `cmd.Long` is guarded. | `cmd/cli/bootstrap_docs_test.go`, `cmd/cli/managed_credential_docs_test.go` (the established pattern); `git show 9d256e9 -- README.md docs/runbooks site/src/content/docs` | UNTESTED (`TestInit_SecurityGuidance` covers only `cmd.Long`) | tests (extend a docs test with `init --auth-key-file` across README/runbook/site) |
| Minor | REAL | UX / behavior | `init --auth-key-file <valid>` **without** `--target` reads the file, then falls into the interactive wizard, which prompts for the key again and overwrites the file value — the credential file is silently discarded. The flag's own help text says "(secure non-interactive mode)", so the fallback contradicts it. No criterion covers this path. | code read: `runInit` → `isInteractive := f.AuthKey == "" \|\| f.Target == ""` → `collectInteractiveInputs` reassigns `f.AuthKey` | UNTESTED | code + tests (follow-up ticket) |
| Minor | REAL | error messages | Double-wrapped prefix on failure: `read auth key file: read auth key file: read <path>: Incorrect function.` — `readAuthKeyFile` already contextualizes, and `runInit` wraps it again. | observed: `/tmp/tsb.exe init --auth-key-file <directory> --target 100.64.0.1:3389` → exit 1 with that text | UNTESTED | code |
| Minor | THEORETICAL | tests | The table asserts the process-table warning is **present** when both flags are given, but never that it is **absent** when only `--auth-key-file` is used; an unconditional warning would pass every test in the suite. | `cmd/cli/init_test.go:403` — `if tt.wantWarning && strings.Contains(stderr, ...)`, with `wantWarning` false and unasserted in the file-only case | UNTESTED | tests |
| Minor | REAL | verification claim | Criterion 4's "full Go test suite … green" did not reproduce on my first full run: `TestStartEnforcesReadyTimeoutDuringDial` failed at `206.9478ms` against a 200 ms wall-clock budget under concurrent load; it passed 3/3 in isolation. Unrelated to this diff (`internal/bootstrapssh`, from CLI-363), but the claim is flake-by-construction rather than green. | `go test ./...` → `--- FAIL: TestStartEnforcesReadyTimeoutDuringDial ... readness timeout took 206.9478ms, want <= 200ms`; then `go test ./internal/bootstrapssh -run TestStartEnforcesReadyTimeoutDuringDial -count=1` ×3 → `ok` | `TestStartEnforcesReadyTimeoutDuringDial` | tests (bootstrapssh — separate ticket, outside this spec) |
| Minor | REAL | tooling (out of repo) | `dotf spec archive` reports `status: archived` but leaves `status: verifying` in the archived `proposal.md` on a CRLF checkout — `setStatus` matches `line == "---"` against raw content, and every line on this Windows checkout ends `\r`. Not a defect of this change, but it is what this spec's own archive will produce. | reproduced twice: CRLF copy (`crlf pairs: 50`) → archived `status: verifying`; LF copy (identical otherwise, digests unchanged) → `status: archived`; both runs printed `status: archived`. Gate digests are unaffected (`normaliseContract` folds CRLF *before* `setStatus`). | UNTESTED (in `cli/internal/spec`, dotfiles repo) | out-of-repo tooling (dotfiles `cli/internal/spec/archive.go` + a CRLF test case) |
| Question | SPECULATIVE | tooling | `review-request.json`'s `features.json` digest (`2d209271…`) is not reproducible from the on-disk file under any normalization I modelled (delete/zero of `state`+`evidence`, sorted or original key order, compact or indented, CRLF folded), while `proposal.md` and `tasks.md` reproduce byte-exactly. The launcher and `dotf spec archive` are the same binary, and the folder is unmodified since `9d256e9`, so freshness should hold — but I could not verify the digest equality independently. | recorded `2d20927178ce7366b08c7d70285b8e4a11c4c32196d8626a9132db8e09d6040f` vs. computed `cbe25bcb…` (delete), `b84cd2b8…` (zero); `git status --porcelain -- specs/CLI-306-init-auth-key-file/` → empty | UNTESTED | vault / out-of-repo tooling (dotfiles `ContractDigests`); no action required for this spec |

No Blocker and no REAL Major was found. Specifically refuted: file-over-inline precedence (end-to-end: `TS_AUTHKEY=tskey-auth-FILEKEY`, inline key count `0` in the generated `.env`, warning still on stderr); missing/empty/whitespace-path/malformed/embedded-newline/profile cases all exit 1 with an actionable message; the key is never echoed to stdout (`printNextSteps` prints paths only) and never written to YAML (it goes to `.env`, mode 0600); the docs now state honestly that `init` writes `TS_AUTHKEY` to the generated `.env` rather than overselling "secure".

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every criterion met and negative paths covered (empty/missing/malformed/newline/profile); AC3 is verified by inspection rather than by a test, and two behavior paths (interactive discard, warning-absent) have no case. |
| Verification       | B | Named tests + reproducible commands, and I re-ran them; the "full suite green" line did not reproduce once (unrelated flake) and the promotion evidence is manual. |
| Scope              | B | 35 executable production lines, tests/docs/spec alongside; one undeclared side-change to the shared key-file reader that also alters `connect`/`browser`. |
| Reliability        | B | Error paths thorough, overwrite protection deliberate and idempotent; YAML mode writes two files non-atomically (pre-existing) and one error message is double-prefixed. |
| Maintainability    | A | Small named helpers, `runInit` complexity ≈8 (gocyclo at threshold 10 reports nothing for `init.go`), no dead code, no test deletions, comments explain why. |
| Handoff-readiness  | A | Spec artifacts filled and honest, decisions and promotion answers recorded, no unresolved draft tags; `features.json` evidence left empty is harness-owned, not a gap in the handoff. |

Aggregation: no D and no C → **PASS**; the minors above are tracked, not blocking.

### Verdict

PASS

### Recommended next steps

On a PASS the contract set (`proposal.md`, `tasks.md`, `features.json`) is closed — editing any of them invalidates this verdict, so none of the below asks for that. Record dispositions in `verification.md` (excluded from the staleness check) or carry them into a follow-up ticket:

- Record the `connect.go` helper hardening under `verification.md` "Decisions made during implementation", and file the follow-up ticket if the shared-reader hardening is meant to be a `connect` change in its own right (finding 1).
- Ticket: add `init --auth-key-file` to the docs-pinning test pattern so README/runbook/site cannot silently regress (finding 2).
- Ticket: decide whether `init --auth-key-file` without `--target` should fail fast instead of discarding the file into the wizard (finding 3), and unwrap the duplicated `read auth key file:` prefix (finding 4).
- Ticket: assert the warning is absent when only `--auth-key-file` is passed (finding 5).
- Separate ticket (pre-existing, not this spec): `TestStartEnforcesReadyTimeoutDuringDial`'s 200 ms wall-clock budget is load-sensitive (finding 6), and note that `golangci-lint run ./...` in this repo replayed 5 cached gosec hits from an out-of-tree sibling worktree until `golangci-lint cache clean` was run.
- File the CRLF status-rewrite defect against the dotfiles repo (`dotf spec archive` claims a rewrite it did not perform); until it is fixed, a Windows CRLF archive records the wrong lifecycle status even though the gate's verdict is correct. The same drill showed my review.md clears all four gate checks in a scratch repo (`[OK] Archived … promotions: every candidate in verification.md is answered`, exit 0) — no gate finding depends on this.
- Advisable to archive once the dispositions above are written: the review gate's four checks (presence, provenance, freshness by content digest, reviewer pool) are satisfied by this file, the tag pre-flight is clean, and the promotion answers pass.
