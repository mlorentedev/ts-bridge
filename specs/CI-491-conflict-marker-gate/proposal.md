---
id: "CI-491-conflict-marker-gate"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#491"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-491-conflict-marker-gate

## Why

<!-- from issue #491: ci: no gate rejects committed merge-conflict markers -->

PR #485 shipped literal merge-conflict markers in `docs/design/host-client-parity.md` and
`docs/lessons/_index.md` at `6609b6e` with every check green, including `Repo hygiene`, whose
lessons guard reads the very index the markers corrupted. A `git merge` resolved one conflicted
file and `git add -A` staged the other two as content. Only PR-Agent noticed; no deterministic
gate rejects a marker line, so the next one can land the same way.

## What

A zero-toolchain guard, `scripts/check-conflict-markers.sh [repo]`, fails with `file:line` for
every tracked text file holding a line that starts with a conflict open marker (seven `<` and a
space or end of line) or close marker (seven `>`, likewise). It exits 0 on a clean tree and fails
closed (exit 2) when the target is not a Git work tree. A fixture suite,
`scripts/tests/test-conflict-markers.sh`, builds throwaway Git repositories at runtime and runs
every case under bash and zsh. `repo-hygiene.yml` runs both.

## Out of scope

- Editing `.github/workflows/repo-hygiene.yml`. That path belongs to the owner, who adds the two
  steps (guard on the real tree, fixture suite after the zsh install). Until then the gate is
  inert in CI; this PR proves it locally.
- A local pre-commit hook. The repo installs none, and CI is the gate that cannot be skipped.
- Untracked or ignored files: they are not what a commit ships.

## Risks / open questions

- **The separator line is not matched, by decision.** The issue's regex includes `=======$`,
  which also matches a Markdown setext underline of exactly seven `=`, the case the acceptance
  says must stay green. Git always writes the open and close markers around a separator (and
  around the diff3 `|||||||` base), so matching those two keeps full coverage of real conflicts
  and drops the only false-positive class. A reviewer should not reopen this without a conflict
  shape that has a separator but no open or close marker.
- **Committed fixtures would red the real tree.** The guard scans every tracked file, so a
  marker-bearing fixture under `scripts/tests/fixtures/` would fail the guard the moment it
  lands. The suite builds its repositories in a temp directory and assembles marker strings at
  runtime, so neither it nor any file in this repo contains a marker at column 0.
- **Documentation that quotes markers.** A doc that shows a conflict at column 0, even inside a
  code fence, fails the guard. That is accepted: indent the example. Today's tree has none.
- **`git grep` exit codes.** 1 means no match (OK), 0 means a match (fail), anything else is an
  error that must fail the check rather than read as clean.
- No blocking open questions remain.

## Acceptance criteria

- [ ] A tracked file containing a conflict open or close marker line fails the guard, which
      names the file and line; the same marker in an untracked file does not.
- [ ] A Markdown setext underline (three `=`, or exactly seven) does not fail the guard.
- [ ] The guard fails closed (exit 2) outside a Git work tree, and reports the current tree clean.
- [ ] Every case above passes under both bash and zsh, and the suite fails when either shell is
      missing.

## References

- Bitácora board: mlorentedev/ts-bridge#491
- Incident: PR #485 at `6609b6e`
- Sibling guards: `scripts/check-doc-authkey.sh` (CI-348), lessons 031/033 on two-shell suites
