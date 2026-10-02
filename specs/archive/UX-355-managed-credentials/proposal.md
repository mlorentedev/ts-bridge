---
id: "UX-355-managed-credentials"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-09-29"
issue: "mlorentedev/ts-bridge#355"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# UX-355-managed-credentials

> **Naming**: file lives at `<repo>/specs/UX-355-managed-credentials/proposal.md`. `UX-355-managed-credentials` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #355: UX-005: Add secure credential onboarding command -->

Users with an already-created Tailscale or Headscale key must currently create
and protect a file manually, remember its path, and pass it to every command.
This blocks #183 and makes multi-tailnet profiles easy to misconfigure. ADR-015
decides that ts-bridge must own masked onboarding, per-user storage, and
profile association.

## What

`ts-bridge auth set [credential] --profile <profile>` stores an existing key
through masked input (or explicit stdin), creates and protects the per-user
credential store, and associates only the credential name with the profile.
`auth status/list/remove` manage non-secret metadata. `connect` resolves the
selected profile credential below explicit and environment sources. To preserve
ADR-014's isolated-browser boundary, `browser` accepts only an explicit
`--auth-key-file` or the selected profile credential and warns when it ignores
`TS_AUTHKEY`.

## Out of scope

Things this PR explicitly does NOT include. Forces a sharp boundary and prevents scope creep.

- Provider-backed key creation/rotation, tracked by #383.
- Native OS keychains or persistent tsnet browser login.
- Removing legacy explicit/env credential sources in this change.

## Risks / open questions

Failure modes, dependencies, and unknowns to clarify before implementation. If any item here is unresolved, do not move to `tasks.md` yet.

- Cross-platform permissions must fail loudly when they cannot be enforced.
- Profile storage remains secret-free and backward compatible.
- Stacked PRs must keep the store/profile foundation independently reviewable from the CLI integration.

## Acceptance criteria

Observable outcomes. Each must be testable.

- [x] `auth set` stores a masked/stdin key atomically with owner-only permissions, refuses silent overwrite, and never prints the key.
- [x] Profiles store only a managed credential name; old profiles remain readable and descriptors never export the reference.
- [x] `connect --profile` resolves the managed key below explicit file/inline/environment sources; `browser --profile` preserves ADR-014 by preferring `--auth-key-file`, otherwise using the managed credential and explicitly warning when `TS_AUTHKEY` is ignored.
- [x] `auth status`, `auth list`, and `auth remove` expose no secret value and handle referenced/shared credentials safely.
- [x] Windows and Unix tests cover path selection, permission hardening, invalid names, missing credentials, rollback/error paths, and output redaction.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: `docs/adr/adr-015-profile-scoped-credential-management.md`
- Related issues: #368 (profile reference), #383 (future provider rotation)
- Related patterns: `pattern-secrets-security`, `pattern-secrets-rotation`

<!-- archived 2026-10-02 — PR: https://github.com/mlorentedev/ts-bridge/pull/406 -->
