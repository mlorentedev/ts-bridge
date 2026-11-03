---
tags: [spec, verification, templates]
created: "2026-11-02"
---

# Verification - CI-563-features-json-shape

## Evidence

- [x] A check names every `features.json` whose shape differs from the scaffold -> `scripts/check-features-shape.sh`; fixtures `top-level-object`, `top-level-string`, `empty-array`, `scalar-element`, `null-element`, `nested-array-element`, `missing-key`, `extra-key`, `renamed-key`, `null-value`, `number-value`, `second-object-bad`, `invalid-json`, `empty-file` each exit 1 naming the problem; `every-bad-named` names all three files and `only-bad-named` omits the conforming one.
- [x] The current tree either passes or every reported spec is listed in the PR -> `bash scripts/check-features-shape.sh` reports 1 of 31: `specs/archive/CI-345-hygiene-guard-fixes/features.json` (a top-level object with `feature_id` and `acceptance_criteria`). Listed in the PR body.
- [x] A fixture with a non-template object makes the check fail (failing-first) -> RED against a stub that always passes: 47 FAIL lines across bash and zsh. GREEN: `test-features-shape: OK (50 cases)` under both shells.
- [x] Archived specs are read, never modified -> `archived-bad` fixture is byte-identical after the run; no file under `specs/archive/` is in the diff.

## Test status

- Test suite: `bash scripts/tests/test-features-shape.sh` -> `OK (50 cases)`; `zsh scripts/tests/test-features-shape.sh` -> `OK (50 cases)`.
- Review finding (PR-Agent): `find` errors were discarded by a process substitution, so an unreadable subtree read as a clean tree. Fixture `unreadable-dir` was RED (exit 0, wanted 2), then fixed by writing the file list to a temp file and exiting 2 when `find` fails.
- Mutants (each makes the suite exit 1): findings disabled (32 FAIL lines), JSON-validity check disabled (4), exit status flipped to 0 (36), string-type check disabled (4: `null-value`, `number-value` under both shells).
- `shellcheck` on both scripts: clean.
- No regressions in existing test suite: yes (no existing file changed outside this spec).

## Decisions made during implementation

- Shape is keys plus string type, not values: `state` and `evidence` are written by the harness, so pinning an enum would turn a harness change into a red here.
- An empty array is reported: the scaffold has one feature, and a reader gets an empty list from it.
- The real-tree step is not wired into `repo-hygiene.yml` (`.github/**` is owner-only), and when it is, CI-345 must be decided first (exempt it or normalise it): the issue forbids rewriting archived specs, so the step would be red on master until then.
- `jq` is the JSON reader. `ubuntu-latest` ships it and `scripts/check-review-published.sh` already depends on it; without it the check exits 2 instead of passing.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: nothing new; the shell-portability and structure-over-position rules are lessons 033 and 036, and this check follows them.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: a hygiene check, no architectural choice.
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: single-project guard.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-563-features-json-shape/` -> `specs/archive/CI-563-features-json-shape/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
