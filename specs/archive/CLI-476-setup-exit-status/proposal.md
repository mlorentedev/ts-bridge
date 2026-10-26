---
id: "CLI-476-setup-exit-status"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#476"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, exit-status]
template_version: "1.0"
---

# CLI-476-setup-exit-status

> **Naming**: file lives at `<repo>/specs/CLI-476-setup-exit-status/proposal.md`. `CLI-476-setup-exit-status` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #476: host setup exits 0 when a step failed -->

`host setup` exits 0 even when a step reports `failed`, so `sudo ts-bridge host setup && next` cannot detect a setup that did not converge. #471 gave `host check` an exit status, but ADR-017 §2 says nothing about `setup`, and `manual` steps (which setup cannot perform) had no answer. The owner chose option B on #476 on 2026-10-22: setup's status reports its own failures, and `check` stays the convergence verdict.

## What

`host setup` exits:

| Exit | When |
|---|---|
| 0 | it ran and every step is `ok`, `changed` or `manual` |
| 2 | it ran and at least one step is `failed` |
| 1 | it could not run (not elevated, unreadable or invalid config, `host.Setup` error) |

`manual` exits 0 because setup has already printed the action that only the user can take (macOS Screen Sharing is always `manual`, so a non-zero exit would mean `host setup && ...` never chains there). The text and JSON output are unchanged; the exit status is added on top, as for `check`. ADR-017 §2 records the contract, and that `sudo ts-bridge host setup && ts-bridge host check` is the idiom for "configure, then verify".

## Out of scope

- `host check`'s exit status (unchanged: 0 all `ok`, 2 any other, 1 could not run).
- Any change to what a step reports; this spec only maps outcomes to an exit status.
- `host init` (it writes a `.env`; it has no steps).

## Risks / open questions

- A script that runs `host setup` and ignores failures sees a new non-zero exit. That is the defect being fixed; the release note must say so, as it did for `check` (#471).
- ADR-017 §2 gives setup no `unverified` outcome. If one ever appears in setup it is not `failed`, so it exits 0; the ADR row stays "n/a" and the test pins the mapping only for outcomes setup emits.

## Acceptance criteria

- [x] AC1: `host setup` exits 2 when any step is `failed`, in text and JSON mode, and still prints the full result.
- [x] AC2: `host setup` exits 0 when every step is `ok`, `changed` or `manual`.
- [x] AC3: `host setup` exits 1 when it cannot run (not elevated).
- [x] AC4: ADR-017 §2 states setup's exit status and the `setup && check` idiom.

## References

- Issue #476 and its options comment; #471 (check's exit status).
- ADR-017 §2: `docs/adr/adr-017-host-desired-state.md`.
