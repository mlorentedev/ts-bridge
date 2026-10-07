---
id: "CI-348-doc-auth-key-guard"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-09-30"
issue: "mlorentedev/ts-bridge#348"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-348-doc-auth-key-guard

## Why

<!-- from issue #348: CI: guard that no connect example in docs passes the auth key inline -->

SEC-212 established that command examples must not place auth keys in the process table, but its
documentation invariant is enforced only by a manual grep in an archived verification file. That
guidance has already drifted twice, so a future documentation edit can silently reintroduce
`connect --auth-key <value>` unless the repository turns the rule into a required hygiene check.

## What

Add a zero-toolchain documentation guard and fixture-driven test suite. The guard scans `README.md`
and Markdown below `docs/`, fails with file and line details when a `ts-bridge` command passes an
inline `--auth-key` value outside `init`, and permits an `init` exception only when a process-list
or process-table warning is within three lines. `repo-hygiene.yml` runs both the guard and its
tests on every pull request and push to `master`.

## Out of scope

- Editing README, runbooks, auth-key guidance, or site content; the current in-scope tree is clean.
- Scanning `site/src/content/docs/`. Issue #343 still owns the source-of-truth decision, and the
  site currently contains deliberately warned inline quick-try examples. Including it would
  require changing content touched by PR #398 and would no longer be an atomic guard-only change.
- Removing the backward-compatible `--auth-key` flag or changing credential precedence.

## Risks / open questions

- A text guard must distinguish command examples from prose that merely names `--auth-key`; fixtures
  cover both so the guard does not make ordinary security explanations fail.
- The original issue predates #306. `init --auth-key-file` now exists and the real tree has no
  inline `init` examples, but the specified warning-gated exception remains covered as a regression
  contract rather than being exercised by current documentation.
- **Threat model (added after three review rounds, 2026-10-06).** The guard is a regression lint
  for examples maintainers write, the SEC-212 contract kept true by CI. It is not a defence against
  documentation authored to evade it. So it classifies by *region*, not by recognising the
  command: every logical command in a fenced or indented code block is checked, and in prose only
  an inline code span that names `ts-bridge` is. A wrapper, prompt, quote or subshell therefore
  cannot hide a key in a code block. What stays out of scope: inline keys in prose outside a code
  span, and evasions that have neither an occurrence in the tree nor a plausible documentation use.
  Findings of that kind are tracked as gaps, not blockers.
- No blocking open questions remain.

## Acceptance criteria

- [ ] `scripts/check-doc-authkey.sh` reports the current `README.md` and `docs/` tree clean while
      ignoring prose that only discusses the deprecated flag.
- [ ] Fixture tests make an inline `connect --auth-key <value>` example fail, allow a warned
      inline `init` example, and reject an unwarned inline `init` example under bash and zsh.
- [ ] `repo-hygiene.yml` runs the guard and fixture suite on every pull request and push to
      `master`, and the existing hygiene, Go build, test, vet, and lint gates remain green.

## References

- Bitácora board: issue #348 (see `issue:` frontmatter).
- Originating contract and finding:
  `specs/archive/SEC-212-authkey-hardening/{proposal,review,verification}.md`.
- Related ADRs: `docs/adr/adr-013-cli-tests-in-go.md` (this is a repository hygiene guard, not
  new CLI behavior) and `docs/adr/adr-015-profile-scoped-credential-management.md` (inline key
  compatibility is deprecated but retained).
- Existing guard pattern: `scripts/check-workflow-permissions.sh` plus
  `scripts/tests/test-workflow-permissions.sh`, wired through `.github/workflows/repo-hygiene.yml`.
