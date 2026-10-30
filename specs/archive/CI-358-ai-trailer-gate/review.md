---
spec: "CI-358-ai-trailer-gate"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "0b863571b1c38dbd188024e74d9845e0a17279b8"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-29"
---

## Adversarial review

**Scope**: `CI-358-ai-trailer-gate` — commit 1553b0d (PR #510, squash-merged to `master`), reviewed at `HEAD` = 0b86357. `git diff 1553b0d HEAD -- scripts/check-ai-trailers.sh scripts/tests/test-ai-trailers.sh specs/CI-358-ai-trailer-gate/ .github/workflows/repo-hygiene.yml` is empty, so the merged state is the state reviewed. Contract digests in `review-request.json` equal `dotf spec digest CI-358-ai-trailer-gate`.
**Sources**: `specs/CI-358-ai-trailer-gate/{proposal,tasks,verification}.md`, `features.json`; diff `933f904..1553b0d` (7 files); `.github/workflows/repo-hygiene.yml`; live CI job 123029800243 (PR #510, `hygiene`).

Read-only review. No tracked file was edited. All mutation experiments ran on throwaway copies under `/tmp/ci358adv`; golangci-lint was not run locally (owner prohibition) and was not needed — this change is shell + YAML.

### Spec and task alignment

- **AC1** ("fails the guard (exit 1) with the commit and trailer, whatever the key's case and whatever other co-authors share the block") — met and reproduced: `claude-key-case` and `among-humans` exit 1, and `bash scripts/check-ai-trailers.sh 813d898~5 813d898` prints one `813d898: Co-authored-by: Copilot <…>` line for each of five commits including `fd6e4e1` (#496) and exits 1. The reporting half of the criterion has **no covering test** (finding 2).
- **AC2** (human co-author, prose mention, trailer only before `base` → exit 0) — met: cases `human`, `human-name-contains-agent`, `prose-mention`, `before-base` all pass; `before-base` moves the `base` tag past the trailered commit, so the exclusive boundary is exercised, not assumed.
- **AC3** (fail closed, exit 2, on a range it cannot resolve) — met: the suite's unresolvable-ref case and `features.json` f3 both exit 2; I re-ran f3 and got 2. A missing repo path also exits 2.
- **AC4** (`Repo hygiene` runs the guard on every PR plus the fixture suite under bash and zsh) — met, and confirmed independently of the spec's own claim: job 123029800243 step 10 ran `bash scripts/check-ai-trailers.sh "$BASE_SHA" "$HEAD_SHA"` → `check-ai-trailers: OK (2 commits)`, step 11 ran `ATG_SHELLS='bash zsh'` → `test-ai-trailers: OK (bash zsh)`. Branch protection on `master` lists required contexts `["test","lint","security","hygiene"]`, so the guard is merge-blocking, not advisory.
- `tasks.md`: every box is backed by diff evidence (guard, suite, workflow, features.json, verification). No `[x]` without evidence.
- Spec drift: none found. `verification.md` claims reproduce exactly, including the mutation battery — I re-ran it and all four claimed mutants are killed (`drop-cursor` → `among-humans`, `drop-unfold` → `folded-value`, `%B`-instead-of-parsed-trailers → `copilot`, `substring` → `human-name-contains-agent`, `exit-0-on-bad-range` → `unresolvable ref`).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | scope / denylist | `AI_AGENTS` omits harnesses that are already in this workstation's agent fleet, so a trailer naming them passes. `proposal.md` declares the denylist risk, but "a new agent name" is not hypothetical here: `AGENTS.md` (workspace doctrine) names OpenCode, Antigravity and Hermes as supported agents, and none of the three is in the list. | Reproduced: throwaway repos with `Co-authored-by: OpenCode <bot@example.com>` (also `Antigravity`, `Hermes`, `Cline`, `Goose`, `Qwen`, `Amp`) all exit 0 with `check-ai-trailers: OK (1 commits)`. | UNTESTED — no case covers an agent name outside the list, so the denylist's own failure mode has no witness | code (one-word additions to `AI_AGENTS`) + tests (a case per added name) |
| Minor | REAL | tests | AC1's clause "with the commit and trailer" is untested: the suite compares exit codes only and discards the guard's output. The guard does print `<sha>: Co-authored-by: <value>` (observed above), but nothing pins it. | Mutant on a throwaway copy — `print $1 ": Co-authored-by: " values[i]` → `print "AI trailer found"` — leaves the suite green (`exit=0`). No case has ever seen the report line. | UNTESTED | tests (assert the output names the commit and the trailer for one case) |
| Minor | REAL | tests | The fail-closed **usage** path (`#` outside 2..3 → exit 2) is untested; only the unresolvable-ref path is. | Mutant — first `exit 2` → `exit 0` — survives the suite (`exit=0`). | UNTESTED | tests |
| Minor | THEORETICAL | resilience | `2>&1` sits *inside* the `log="$(git log …)"` substitution, so stderr is captured into the stream that is then parsed as commit lines. A git warning on a successful run inflates the `OK (N commits)` count, and a tab-bearing warning line could print a phantom finding. | Code read (`scripts/check-ai-trailers.sh` line 23). No warning observed on this tree or in CI, so no repro. | UNTESTED | code (capture stderr to its own file/variable) |
| Minor | SPECULATIVE | gate integrity | The gate is skipped on `push` (declared) **and** runs from the workflow file of the PR's own head, which is how `pull_request` events resolve their workflow. A PR that weakens `check-ai-trailers.sh` or deletes the step reports `hygiene` green. | Behaviour follows from the `pull_request` event model, not from this diff; every sibling guard shares it. Not observed. `proposal.md` "Out of scope" sells CI as "the gate that cannot be skipped", which is true of local hooks, not of a PR editing its own workflow. | UNTESTED | vault/docs — open a ticket; the contract set is closed under this verdict |
| Minor | THEORETICAL | message shape | A `Co-authored-by:` line that git does not classify as a trailer is not flagged: text after the trailer block (`prose-after-trailer`, exits 0), or a trailer with no blank line before it (`subject\nCo-authored-by: …`, exits 0). | Reproduced on throwaway repos. **Attribution impact refuted**: GitHub documents trailers "at the end of the commit message", and independent reports ([update-flake-lock#294](https://github.com/DeterminateSystems/update-flake-lock/issues/294), [Mergify #4636](https://github.com/Mergifyio/Mergify/discussions/4636)) confirm nothing may follow them, so these shapes credit nobody. The residual is textual: the doctrine's literal "no AI attribution in history" is unmet for them. | UNTESTED | spec (already declared: `prose-mention` is a required pass) — record the residual in `verification.md` |
| — | refuted | message shape | Hypothesis: `git cherry-pick -x` appends `(cherry picked from commit …)` after the trailers, unhitching them from the trailer block and sneaking an attributed commit past the guard. **Refuted**: `git cherry-pick -x` places the line with no blank line after the trailer, git still parses the block, and the guard exits 1. | Throwaway repro: `03d0978: Co-authored-by: Copilot <…>`, exit 1. | n/a | none |

No Blocker. No REAL Major. The one Major is THEORETICAL and already declared as an accepted risk in `proposal.md`, so it is tracked, not blocking.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four criteria met and independently reproduced; negative-path coverage is strong (`among-humans`, `folded-value`, `human-name-contains-agent`, `before-base`), but the denylist's own failure mode and the reporting clause are unwitnessed. |
| Verification       | B | Every claim in `verification.md` reproduces, mutation battery included, and the live CI log corroborates the CI-only zsh leg — but AC1's reporting half and the usage path have no test, and zsh cannot run on this workstation. |
| Scope              | A | Exactly the 7 files the proposal implies, no creep; the `fetch-depth: 0` addition is attributed in a comment and costs 32 s in the live run. |
| Reliability        | A | Fail-closed everywhere I could exercise it (unresolvable ref, missing repo, empty range), read-only and idempotent, no partial-failure window; the stderr-bleed nit is a diagnostic inaccuracy, not an error-handling gap. |
| Maintainability    | A | 43-line guard, one denylist variable, awk kept flat, no function over 40 lines, comments explain *why* (`unfold`, whole-word matching, parser-not-regex). |
| Handoff-readiness  | B | Spec and verification are current and the three promotion candidates are answered with reasons; the denylist residuals and the two untested clauses are recorded nowhere the next agent would look. |

### Verdict

**PASS WITH GAPS** — no Blocker, no REAL Major, no rubric dimension at C or D; one open THEORETICAL Major plus four Minor findings, one hypothesis refuted.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** under this verdict: editing any of them stales the review and the archive gate will refuse it. Everything below is outside that set, so it can be applied now, or carried into a follow-up ticket.

- **tests** — add to `scripts/tests/test-ai-trailers.sh`: (a) for one hit case, assert the guard's output names the commit and the trailer value (findings 2); (b) a wrong-arity case expecting exit 2 (finding 3); (c) a negative case for an agent name deliberately left out of `AI_AGENTS`, so the denylist's failure mode is visible rather than silent (finding 1).
- **code** — in `scripts/check-ai-trailers.sh`, stop folding stderr into the parsed stream: capture it to its own variable and print it only on failure (finding 4). Add the in-use harness names (OpenCode, Antigravity, Hermes — at minimum) to `AI_AGENTS` in the same change (finding 1).
- **verification.md** — record the two residuals as decisions: the message shapes git does not parse as trailers (finding 6, with the refutation above), and that `hygiene` guards the PR's own copy of the workflow, which is a repo-wide posture and not this gate's property (finding 5). `verification.md` is excluded from the staleness check, so this does not cost a review round.
- **ticket** — file the finding-5 posture (a PR can weaken the guard that checks it) once, for all hygiene guards, rather than per spec.
- **archive** — `dotf spec archive CI-358-ai-trailer-gate` is advisable in this state: the review is fresh against the recorded contract digests, the promotion candidates are answered, and the change is already on `master` (1553b0d) with a `hygiene`-blocking live run. No action is required to flip this verdict; the gaps are tracked, not blocking.
