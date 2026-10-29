---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - CI-491-conflict-marker-gate

## Evidence

- [x] Criterion 1 -> `scripts/tests/test-conflict-markers.sh` cases `tracked-conflict` (exit 1,
      names `docs/index.md:2:`), `bare-open-marker` (exit 1, an open marker with no label) and
      `untracked-conflict` (exit 0).
- [x] Criterion 2 -> cases `setext-short` (`===`) and `setext-seven` (exactly seven `=`), both
      exit 0; `indented-example` shows an indented quoted conflict stays green.
- [x] Criterion 3 -> case `not-a-repo` (exit 2, `not a Git work tree`); the real tree reports
      `check-conflict-markers: OK (513 tracked files)` at `89b8e61`.
- [x] Criterion 4 -> the suite ran every case under bash and zsh (26/26 ok: 13 cases x 2 shells,
      Repo hygiene run `40573251031` at `96c9760`), and
      `TCM_SHELLS="bash nosuchsh"` exits 1.
- [x] CI wiring -> `.github/workflows/repo-hygiene.yml` runs `No committed merge-conflict markers`
      and `Conflict-marker guard rejects markers, tolerates setext headings` (`features.json` f4);
      both passed in Repo hygiene run `40567222073` at `37d9133`.

## Test status

- RED: `bash scripts/tests/test-conflict-markers.sh` before the guard existed -> exit 2,
  `guard not found`.
- GREEN: `bash scripts/tests/test-conflict-markers.sh` -> exit 0, 16/16 checks (8 cases, bash + zsh)
  at `068bf95`. The current suite has 13 cases: 26/26 in Repo hygiene run `40573251031` at
  `96c9760`. The local counts below are bash-only runs from the review follow-ups.
- Mutants, each run with `TCM_SHELLS=bash`, all redden the suite (exit 1): separator `=======`
  added back to the regex; `git grep` exit 1 no longer treated as OK; the end-of-line form of the
  marker dropped; findings exiting 0.
- Real tree: `bash scripts/check-conflict-markers.sh` -> `OK (513 tracked files)` at `89b8e61`;
  `bash scripts/check-lessons.sh` -> `OK (46 lessons)`.
- `shellcheck scripts/check-conflict-markers.sh scripts/tests/test-conflict-markers.sh` -> exit 0.
  The hygiene job's shellcheck covers only workflow-embedded shell, so this ran locally.
- No regressions: no Go code changed.
- PR review follow-up (CodeRabbit on #495): three new cases, `long-markers` (32-character runs
  under `conflict-marker-size=32`), `diff3-base` (a `|||||||` base line) and `nested/sub` (a
  `[repo]` inside the work tree, marker elsewhere), each failed against the previous guard with
  `exit 0, wanted 1` under `TCM_SHELLS=bash`. After the fix the suite passes 11/11 under bash.
  The real tree reports `OK (539 tracked files)`, also when run from `docs/`, and `shellcheck` on
  both scripts exits 0. zsh is not installed on this Windows workstation, so CI's Repo hygiene
  run covers the zsh half.
- PR-Agent on `b54fa99`: a bare repository or a path inside `.git` answers
  `--is-inside-work-tree` with `false` and exit 0, so the guard reached `--show-toplevel` and exited
  128. New cases `bare-repo` and `dotgit/.git` failed with `exit 128, wanted 2` (bare repos allowed
  via `GIT_CONFIG_*` `safe.bareRepository=all`, since this workstation sets `explicit`). The guard
  now tests the printed answer. 13/13 pass under bash with and without that override.

## Independent review round 1 - 2026-10-27

Reviewer `nan/deepseek-v4-flash` returned **FAIL** at `813d898`; the signed report is kept as
`review-round1.md` with its matching `review-request-round1.json`. No Blocker; the guard itself
was judged sound. The contract-set edits below change the digests, so a fresh review follows.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| Major: `features.json` f1-f3 pin `TCM_SHELLS='bash zsh'` and cannot pass on a workstation without zsh | Applied (contract). | f1-f3 run with `TCM_SHELLS=bash` and exit 0 here. AC4 moved to its own feature, f5: the missing-shell failure runs locally (`TCM_SHELLS='bash ci491-missing-shell'` must fail), and the zsh half is stated as CI-only, asserted by the zsh install preceding the suite's default (bash + zsh) run in `repo-hygiene.yml`. AC4's text records the same split. |
| Minor: f4 only proves the paths are mentioned | Applied (contract). | f4 asserts both exact `run:` lines inside steps carrying no `if:`, and no job-level `if:`. It exits 0 on this tree and 1 with `if: false` added to the guard step; f5 exits 1 with the zsh install removed. |
| Minor: `git grep -I` skips binary files silently | Applied (code + tests). | The guard uses `-a`. New cases `binary-attribute` (`*.md binary`) and `nul-byte` (NUL on line 1, marker on line 3) failed with `exit 0, wanted 1` before the change and pass after; restoring `-I` fails both. The real tree stays `OK (541 tracked files)`. |
| Minor: `bare-repo` exercises only the ambient `safe.bareRepository` branch | Applied (tests). | The case runs under `GIT_CONFIG_*` `safe.bareRepository=explicit` and `=all` explicitly. Mutating the guard to trust `rev-parse`'s exit code fails `bare-repo[all]` (and `dotgit/.git`) with `exit 128, wanted 2`. |
| Minor: `lesson-048` names only the open and close markers | Ticketed: #507. | Docs-only; together with the `security-audit.md` required-check reconciliation. |
| Minor: spec drift (PR box, `status: implementing`) | Applied. | `tasks.md` PR box ticked (#495, #501); `proposal.md` is `verifying`. |
| Minor (THEORETICAL): `\|{7,}` in an ERE | Applied. | Written as a bracketed pipe class, `[\|]{7,}`; `diff3-base` still fails as required. |

`shellcheck` on both scripts exits 0; the suite passes 16/16 checks under `TCM_SHELLS=bash` (15 cases, `bare-repo` run under both settings).

CodeRabbit on #511 then tightened the contract and named one limit:

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| f4 records a step unconditional before reading an `if:` placed after `run:` | Applied (contract). | f4 decides per step, at the step's end. Exits 0 here; 1 with `if: github.event_name == 'push'` after the guard's `run:`. |
| f5 passes when the suite step sets `TCM_SHELLS: bash` in `env:` | Applied (contract). | f5 fails on any non-comment `TCM_SHELLS` in the workflow (step, job or workflow `env:`). Exits 0 here; 1 with that `env:` added. |
| `git grep -a` misses markers in a `working-tree-encoding=UTF-16LE` file | Skipped; recorded as a limit (proposal Risks). | No tracked file declares `working-tree-encoding`, and CI scans a fresh checkout with no unstaged content. An encoding-aware scan is a second behaviour, to be added if such a file is ever tracked. |
| Pipe splits table cells in `review-round1.md` and here | Applied here; skipped in `review-round1.md`. | The signed reviewer report stays verbatim. |

## Independent review round 2 - 2026-10-29

Reviewer `nan/deepseek-v4-flash` returned **PASS WITH GAPS** at `7bc98be` (#511 merged) against
the current contract digests (`review.md` with `review-request.json`). No Blocker, no REAL Major.
The contract set is closed by this verdict, so every disposition below lands outside it.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| Major (THEORETICAL): the fail-closed `git grep` error branch has no test | Applied (tests). | New case `corrupt-index` (garbage in `.git/index`) expects exit 2 and `git grep failed`. Mutating the guard's `*)` arm so an error falls through to exit 0 fails it. |
| Minor: an unlabelled marker is missed under CRLF (`<<<<<<<\r`) | Applied (code + tests). | The marker now ends at `[[:space:]]` or the line end. New case `crlf-bare-marker` (`*.txt text eol=crlf`, CRLF bytes) failed with `exit 0, wanted 1` before and passes after; restoring `( \|$)` fails it again. |
| Minor (SPECULATIVE): tab-separated label missed | Applied with the CRLF fix. | New case `tab-label` failed before, passes after. A marker with no separator at all (`<<<<<<<HEAD`) stays unmatched: Git never writes it. |
| Minor: f4 accepts `continue-on-error` on a guarded step | Ticketed: #514. | Fixing it means editing `features.json`, which would stale this verdict. |
| Minor: `build()` exceeds the 40-line / CC-10 rule | Exemption recorded. | A comment above `build()` states why a flat fixture table stays in one function. |
| Minor: `dotf spec audit` reports the spec as a zombie (#491 closed) | Cleared by archiving in this PR. | — |

The suite passes 19/19 checks under `TCM_SHELLS=bash`, three consecutive runs; the real tree
reports `OK (553 tracked files)`; `shellcheck` on both scripts exits 0.

## Decisions made during implementation

- The separator is not matched (see proposal Risks): the issue's `=======$` would red a
  seven-`=` setext underline, the acceptance's own negative case.
- Fixtures are built at runtime, never committed: the guard scans every tracked file, so a
  committed marker fixture would fail the real tree.
- The workflow steps were first proposed in the PR body; the owner then authorized adding them
  in this PR (`37d9133`).
- The issue's `:!*.md.orig` exclusion is not carried: no `.orig` file is tracked, and a tracked
  backup holding markers would itself be a shipped defect.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-048-2026-10-22.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: a hygiene guard following the existing guard pattern, no architectural choice
- [x] New pattern candidate for `00_meta/patterns/`? no: the cross-project form (gate on marker lines) is already the generic hygiene-guard pattern; one project so far

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/CI-491-conflict-marker-gate/` -> `specs/archive/CI-491-conflict-marker-gate/`
- [x] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018): #491 closed by #495
- [x] Promotions above executed (if any)
