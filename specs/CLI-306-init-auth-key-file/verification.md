---
tags: [spec, verification, templates]
created: "2026-09-23"
---

# Verification - CLI-306-init-auth-key-file

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] Criterion 1 -> `TestInitAuthKeyFile` covers valid, missing, empty, and malformed key files; Windows binary smoke wrote a config from a synthetic key file and returned exit 1 for a missing file.
- [x] Criterion 2 -> `TestInitAuthKeyFile/file_takes_precedence_over_inline_key` verifies file precedence and the process-table warning.
- [x] Criterion 3 -> `TestInit_SecurityGuidance`; README, Windows deployment runbook, multi-device runbook, and site sources updated.
- [x] Criterion 4 -> full Go tests, vet, golangci-lint v2.12.2, Windows build, and Astro site build completed successfully.

## Test status

- Test suite: `go test -coverprofile=<session>/coverage.out ./...` -> green, 65.1% total statement coverage.
- Static checks: `go vet ./...` -> green; `golangci-lint run` -> `0 issues`.
- Documentation: `npm ci --prefix site --no-audit --no-fund && npm run build --prefix site` -> 5 pages built successfully.
- Manual smoke test: Windows binary exposed `init --auth-key-file`, wrote the expected synthetic key and target, and returned a clear missing-file error with exit 1.
- Corporate mesh smoke: a replacement reusable+ephemeral auth key completed a forced login from a fresh state directory and reached `READY`; `/health/ready` returned HTTP 200. The subsequent RDP dial resolved `acemagic-office` to `100.82.151.104` but port 3389 refused the connection, confirming that authentication and mesh registration work while the target RDP service remains unavailable. Real-tailnet target availability is outside this spec and tracked separately by #183.
- No regressions in existing test suite: yes.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- Reused the existing `readAuthKeyFile` implementation so `connect` and `init` share path resolution, permission warnings, newline trimming, and file errors.
- Kept secure credential onboarding outside this atomic PR; follow-up #355 tracks a masked `ts-bridge auth set`-style product flow.

## Promotion candidates

Before archiving, flag what (if anything) should be promoted to the vault. If all three are "no", archive in repo is the only persistence.

- [x] Lesson for the repo's `docs/lessons/`? No; the credential-vs-state discriminator is already reflected by the structured `bad_authkey` remediation.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? No; this extends the existing CLI contract without changing architecture.
- [x] New pattern candidate for `00_meta/patterns/`? No; no cross-project pattern was established.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CLI-306-init-auth-key-file/` -> `specs/archive/CLI-306-init-auth-key-file/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
