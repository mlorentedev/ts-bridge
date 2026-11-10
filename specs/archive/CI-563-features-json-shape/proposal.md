---
id: "CI-563-features-json-shape"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-02"
issue: "mlorentedev/ts-bridge#563"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-563-features-json-shape

> **Naming**: file lives at `<repo>/specs/CI-563-features-json-shape/proposal.md`. `CI-563-features-json-shape` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #563: CI-563: Detect non-template features.json objects in specs -->

The review tooling reads `features.json` as a JSON array of objects (`id`, `behavior`, `verification`, `state`, `evidence`), the shape `dotf spec init` scaffolds. A spec whose file has another shape is read as stale or reported falsely, and nothing in this repo says which ones they are. The check names them; it does not rewrite them (#563, split from #403).

## What

`scripts/check-features-shape.sh [specs-dir]` walks every `features.json` under `specs/` (archive included, read-only), prints one line per deviation and exits 1 when any file differs from the scaffold. `scripts/tests/test-features-shape.sh` runs a fixture suite under bash and zsh. Neither runs in CI yet: wiring them into the hygiene job is an owner follow-up (see Risks).

## Out of scope

Things this PR explicitly does NOT include. Forces a sharp boundary and prevents scope creep.

- Rewriting any `features.json`, archived or not (detect only, #563).
- Constraining the value of `state` or the content of any field: shape only.

## Risks / open questions

Failure modes, dependencies, and unknowns to clarify before implementation. If any item here is unresolved, do not move to `tasks.md` yet.

- The hygiene workflow is `.github/**`, owner-only: wiring both steps is a follow-up named in the PR, and the real-tree step stays red on `CI-345-hygiene-guard-fixes` until the owner exempts or normalises it.

## Acceptance criteria

Observable outcomes. Each must be testable.

- [ ] A check names every `features.json` whose shape differs from the scaffold.
- [ ] The current tree either passes or every reported spec is listed in the PR.
- [ ] A fixture with a non-template object makes the check fail (failing-first).
- [ ] Archived specs are read, never modified.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: `<repo>/docs/adr/adr-XXX.md` (if any)
- Related patterns: `00_meta/patterns/<pattern>.md` (if any)
