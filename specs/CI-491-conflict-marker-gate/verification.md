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
- [x] Criterion 4 -> the suite ran every case under bash and zsh (16/16 ok), and
      `TCM_SHELLS="bash nosuchsh"` exits 1.
- [x] CI wiring -> `.github/workflows/repo-hygiene.yml` runs `No committed merge-conflict markers`
      and `Conflict-marker guard rejects markers, tolerates setext headings` (`features.json` f4);
      both passed in Repo hygiene run `40567222073` at `37d9133`.

## Test status

- RED: `bash scripts/tests/test-conflict-markers.sh` before the guard existed -> exit 2,
  `guard not found`.
- GREEN: `bash scripts/tests/test-conflict-markers.sh` -> exit 0, 16/16 cases (bash + zsh).
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

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-491-conflict-marker-gate/` -> `specs/archive/CI-491-conflict-marker-gate/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
