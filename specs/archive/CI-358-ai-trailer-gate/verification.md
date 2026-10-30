---
tags: [spec, verification, templates]
created: "2026-10-28"
---

# Verification - CI-358-ai-trailer-gate

## Evidence

- [x] Criterion 1 -> `bash scripts/tests/test-ai-trailers.sh` cases `copilot`, `claude-key-case` (`Co-Authored-By`), `among-humans` (human plus Cursor in one block), `earlier-in-range` (trailer in the first of two commits) and `folded-value` (value continued on an indented line) all exit 1. Against real history, `bash scripts/check-ai-trailers.sh 813d898~5 813d898` lists five commits with `Co-authored-by: Copilot`, including `fd6e4e1` (#496), and exits 1.
- [x] Criterion 2 -> cases `no-trailer`, `human`, `human-name-contains-agent` (`Claudette Geminiani`), `prose-mention` (the trailer text in a body paragraph, not the trailer block) and `before-base` (trailer only below `base`) exit 0.
- [x] Criterion 3 -> the suite's unresolvable-ref case exits 2; `features.json` f3 also runs it against the real tree.
- [x] Criterion 4 -> `repo-hygiene.yml` checks out full history, runs the guard on `pull_request` with `BASE_SHA`/`HEAD_SHA` passed through `env:` (no expression in the shell), and runs the suite with `ATG_SHELLS='bash zsh'` after the existing zsh install step. `features.json` f4 asserts the step, its `if:` and the two-shell suite run; it exits 0 on this tree and 1 with the `if:` line removed. The step's live result is this PR's own `hygiene` run.

## Test status

- `bash scripts/tests/test-ai-trailers.sh` -> `test-ai-trailers: OK (bash)`. Before the guard existed the suite exited 2 (`guard not found`).
- Mutants, each killed by the suite: dropping `cursor` from the agent list (`among-humans`), reading `%B` instead of the parsed trailers (`copilot`, `claude-key-case`), exiting 0 on an unreadable range (`unresolvable ref`), dropping `unfold` (`folded-value`), and matching agent names as substrings instead of whole words (`human-name-contains-agent`). The last two came from CodeRabbit on #510; each new case failed before its fix.
- `shellcheck scripts/check-ai-trailers.sh scripts/tests/test-ai-trailers.sh` -> clean.
- Sibling hygiene suites on this branch: `test-site-pr-workflow`, `test-go-toolchain`, `check-actions-pinned`, `check-workflow-permissions` pass. `test-workflow-permissions` fails only on `[zsh] not installed`, identically on `master` (this workstation has no zsh); every bash case passes.
- The zsh half runs in CI only; zsh is absent on this workstation.

## Independent review round 1 - 2026-10-29

Reviewer `nan/deepseek-v4-flash` returned **PASS WITH GAPS** at `0b86357` (`review.md` with
`review-request.json`). No Blocker, no REAL Major. The contract set is closed by this verdict, so
every disposition below lands outside it.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| Major (THEORETICAL): the denylist omits harnesses in use (OpenCode, Antigravity, Hermes, ...) | Applied (code + tests). | `AI_AGENTS` gains `opencode`, `antigravity`, `qwen`. `cline`, `goose`, `amp` and `hermes` are also human names (PR-Agent on #517: `Sam Cline` would be rejected), so by owner decision they match only beside `agent`, `bot` or `ai` (`AI_QUALIFIED`): `Cline Bot` and `Hermes-Agent` fail, `Sam Cline`, `Lucy Goose`, `Amp Patel` and `Hermes Conrad` pass. The suite builds one repository per agent value and per human value, so dropping any single name, or the qualifier, fails a case. The human cases failed before the qualifier existed. The list stays a denylist, as `proposal.md` Risks declares. |
| Minor: AC1's "names the commit and trailer" is untested | Applied (tests). | The suite asserts the `copilot` report contains `<sha>: Co-authored-by: Copilot <...>`. Replacing the report line with a fixed string fails it. |
| Minor: the usage path (wrong arity → exit 2) is untested | Applied (tests). | One-argument call expects exit 2; turning the usage `exit 2` into `exit 0` fails it. |
| Minor (THEORETICAL): `2>&1` folds git warnings into the parsed stream | Applied (code). | Stderr goes to a temp file and is printed only when `git log` fails. |
| Minor (SPECULATIVE): a PR runs its own copy of the workflow and guard | Ticketed: #516. | A posture shared by every hygiene guard, filed once for all of them. |
| Minor (THEORETICAL): lines Git does not parse as trailers pass | Recorded; declined. | Text after the trailer block, or a trailer with no blank line before it, is not a trailer to Git, and the reviewer refuted the attribution impact: GitHub only credits co-authors from the trailer block. The guard follows Git's parser by design. |

`bash scripts/tests/test-ai-trailers.sh` → `OK (bash)`; `shellcheck` on both scripts → clean.

## Decisions made during implementation

- The suite defaults to `bash` and takes extra shells from `ATG_SHELLS`, so every `features.json` command runs on a workstation without zsh; CI's step names both shells and fails if either is missing (the CI-491 round-1 Major).
- Git's `%(trailers:key=Co-authored-by,valueonly)` parses the trailer block, rather than a regex over the message, so body prose can never match and the key is case-insensitive; `unfold` joins folded values, and agent names match as whole words so a human named Claudette passes.
- Push runs skip the guard: `master` already holds trailered commits that cannot be rewritten, and a red push run would only report them forever.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: the rule already lives in the doctrine; the gate is its enforcement, recorded here
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: a hygiene guard, not an architectural change
- [x] New pattern candidate for `00_meta/patterns/`? no: one repo; other repos would reuse the script, not a new pattern

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/CI-358-ai-trailer-gate/` -> `specs/archive/CI-358-ai-trailer-gate/`
- [x] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018): #506 closed by #510
- [x] Promotions above executed (if any)
