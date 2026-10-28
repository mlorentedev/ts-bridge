---
tags: [spec, tasks, templates]
created: "2026-10-28"
---

# Tasks - CI-358-ai-trailer-gate

## Setup

- [x] Branch created from master: `feat/ci358-ai-trailer-gate`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC1] [AC2] [AC3] Fixture suite `scripts/tests/test-ai-trailers.sh` (ten ranges plus an unresolvable ref)
- [x] [AC1] [AC2] [AC3] Guard `scripts/check-ai-trailers.sh` makes it pass
- [x] [AC4] `repo-hygiene.yml`: full-history checkout, guard on pull requests, suite under bash and zsh

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [x] Lint passes (`shellcheck` on both scripts)
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [x] PR opened referencing this spec folder (#510)
