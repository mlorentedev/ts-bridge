---
id: lesson-023-pinning-the-linter-version-surfaces-accumulat
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, linter, golangci-lint, technical-debt, ci]
---

# Pinning the linter version surfaces accumulated deferred debt

**Context:** PR #33 pinned `golangci-lint` from `version: latest` to `v1.62.2`. Two things happened that wouldn't have happened with the floating version:
1. `gosimple S1008` fired on a pre-existing `if X { return true }; return false` block in `reconnect.go` that had survived weeks under `latest`.
2. `gocyclo` raised the bar from whatever `latest` resolved to and rejected `LoadConfig` at complexity 16 (after a new env-var validation pushed it over 15).

**Finding:** "Floating linter version" is a quiet form of technical-debt absorption — your local sees one thing, CI sees another, and rules drift in/out as upstream releases. Pinning forces every contributor (and every CI run) to see the same finding set. The PR that introduces the pin will fail CI on the existing accumulated drift — that's the *correct* failure, not a regression.

**Rule:** When introducing a linter pin, expect 1-3 immediate findings on existing code. Plan to fix them in the same PR (or a small `chore:` precursor) — they're real issues the looser version was hiding. The cost is paid once.

**Tags:** `#linter` `#golangci-lint` `#technical-debt` `#ci`
