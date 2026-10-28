---
id: "CI-358-ai-trailer-gate"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-10-28"
issue: "mlorentedev/ts-bridge#506"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-358-ai-trailer-gate

## Why

<!-- from issue #506: CI-358: reject AI-attribution trailers in pull request commits -->

The doctrine forbids AI attribution in git history, yet agent harnesses append a
`Co-authored-by:` trailer naming the agent by default and nothing in this repo catches it.
`fd6e4e1` (#496) and `aa3e8c6` (#497) reached `master` with `Co-authored-by: Copilot`, and the
no-rewrite rule makes them permanent; the BUG-030 adversarial review asked twice for a
prevention gate.

## What

A zero-toolchain guard, `scripts/check-ai-trailers.sh <base> <head> [repo]`, lists every commit
in `base..head` whose `Co-authored-by` trailer value (unfolded) names an AI agent as a whole word (Copilot, Claude/Anthropic,
ChatGPT/OpenAI, Gemini, Codex, Cursor, Devin, Aider, Windsurf, Codeium, Tabnine, CodeWhisperer;
case-insensitive) and exits 1. Git's own trailer parser decides what is a trailer, so a mention
in body prose is not one and the key matches in any case. It exits 0 on a clean range and fails
closed (exit 2) on a range it cannot read. `repo-hygiene.yml` runs it on pull requests over the
PR's `base.sha..head.sha`, and runs its fixture suite, `scripts/tests/test-ai-trailers.sh`, which
builds throwaway repositories and runs every case under each shell in `ATG_SHELLS` (default
`bash`; CI passes `bash zsh`).

## Out of scope

- Rewriting history already on `master` (`fd6e4e1`, `aa3e8c6` and earlier): the gate reads only
  a pull request's own commits, and push runs skip it.
- Other attribution forms (`Generated with ...` lines, PR or issue bodies): the issue scopes the
  gate to commit trailers.
- A local commit hook: CI is the gate that cannot be skipped.

## Risks / open questions

- **Agent list is a denylist.** A new agent name passes until it is added; the list sits in one
  variable so the fix is a one-word change.
- **Squash merges.** The squash commit is written at merge time from the PR title and body, after
  the gate has run; a trailer typed into the merge dialog is not checked. The PR's commits, which
  are what GitHub folds into that message by default, are.
- **Full-history checkout.** `fetch-depth: 0` is needed for `base..head`; the repository is small,
  so the cost is seconds.
- No blocking open questions remain.

## Acceptance criteria

- [x] A commit in the range whose `Co-authored-by` trailer names an AI agent fails the guard
      (exit 1) with the commit and trailer, whatever the key's case and whatever other
      co-authors share the block.
- [x] A human co-author, a mention in body prose, and a trailer only in commits before `base`
      do not fail the guard.
- [x] The guard fails closed (exit 2) on a range it cannot resolve.
- [x] `Repo hygiene` runs the guard on every pull request and its fixture suite under bash and
      zsh.

## References

- Bitácora board: mlorentedev/ts-bridge#506
- Evidence: BUG-030 adversarial review rounds 1-2 (`specs/BUG-030-profile-target-source/`)
- Sibling guard: `scripts/check-conflict-markers.sh` (CI-491); CI-491's round-1 review on
  shell-dependent `features.json` commands (hence `ATG_SHELLS` defaulting to bash)
