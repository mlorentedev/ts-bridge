---
id: "CI-344-site-pr-build"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-09-29"
issue: "mlorentedev/ts-bridge#344"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-344-site-pr-build

> **Naming**: file lives at `<repo>/specs/CI-344-site-pr-build/proposal.md`. `CI-344-site-pr-build` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #344: CI: the site has no PR-time build, so a broken docs page is found by publishing it -->

The Astro/Starlight site is built only after changes reach `master`, in the
same workflow that publishes GitHub Pages. A malformed MDX page or broken site
configuration therefore reaches the default branch before any required check
compiles it; local build claims in a PR body are not an enforceable gate.

## What

A pull request that changes `site/**` or the site-build workflow receives a
read-only `site-build` check. The check installs the lockfile exactly with
`npm ci` and runs the production `npm run build` command without granting
Pages or identity-token write permissions and without deploying anything.

## Out of scope

Things this PR explicitly does NOT include. Forces a sharp boundary and prevents scope creep.

- Making the new check a required branch-protection status; that is an owner-side repository setting.
- Consolidating the duplicate `docs/` and `site/` content trees tracked by #343.
- Changing the existing push-triggered Pages deployment workflow.

## Risks / open questions

Failure modes, dependencies, and unknowns to clarify before implementation. If any item here is unresolved, do not move to `tasks.md` yet.

- A PR workflow must not inherit the Pages workflow's `pages: write` or `id-token: write` permissions.
- The workflow must trigger on its own file as well as `site/**`, otherwise the PR introducing or repairing the gate cannot prove that it runs.
- The existing setup-node npm cache is sufficient; no additional cache action is justified.

## Acceptance criteria

Observable outcomes. Each must be testable.

- [x] A pull request changing `site/**` or the site PR workflow receives a `site-build` job that checks out the proposed revision.
- [x] `site-build` uses Node 22, `npm ci`, and `npm run build` under `site/`, with checkout credentials disabled and only `contents: read`.
- [x] Pull requests cannot upload or deploy a Pages artifact; the existing push-only Pages deployment remains unchanged.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: none; this is CI verification, not a product architecture decision.
- Related patterns: repository workflow permission and SHA-pin guards in `scripts/`.
