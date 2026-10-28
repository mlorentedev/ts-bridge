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

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-358-ai-trailer-gate/` -> `specs/archive/CI-358-ai-trailer-gate/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
