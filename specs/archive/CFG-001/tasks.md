---
tags: [spec, tasks]
created: "2026-06-26"
---

# Tasks - CFG-001

> TDD order. One task = one focused commit. Tick as you go.
> Building on: `internal/profile/store.go` (Store.Import), `internal/config/statedir.go` (ProfileStorePath), `cmd/cli/connect.go` (--profile + applyProfile) — all shipped in CFG-002 (#228).

## Setup

- [ ] Branch created from main: `feat/CFG-001-config-profiles-model`
- [ ] `proposal.md` complete, all acceptance criteria testable, no MUST RESOLVE open
- [ ] ADR-012 skeleton drafted before any code touches `cmd/cli/init.go`

## 1. ADR-012 — author architectural decision

- [ ] Create `docs/adr/adr-012-config-profiles-model.md` (status: accepted)
- [ ] Document: `os.UserConfigDir()` rejected in favor of existing `ProfileStorePath()` (LOCALAPPDATA/XDG_STATE_HOME path — already live in CFG-002); rationale for reusing same store
- [ ] Mark `docs/adr/adr-002-single-binary-no-config.md` as `status: deprecated` in frontmatter

## 2. Store.Set — write profile from raw params (no Descriptor)

- [ ] Test: `Store.Set("work", "host:3389", "")` writes `profiles["work"].target = "host:3389"` and no `control_url` key
- [ ] Test: `Store.Set("kubelab", "host:3389", "https://vpn.kubelab.live")` writes both fields
- [ ] Test: `Store.Set` on existing name overwrites cleanly (idempotent)
- [ ] Implement `func (s *Store) Set(name, target, controlURL string) error` in `internal/profile/store.go`

## 3. `ts-bridge init --profile <name>` — manual profile creation

- [ ] Test: `init --profile work --target host:3389` writes profile to store; no `.env` file created
- [ ] Test: `init --profile work --target host:3389 --overwrite` updates existing profile
- [ ] Test: `init --profile work --target host:3389` without `--overwrite` fails with clear error when profile exists
- [ ] Test: `init --profile work --target host:3389 --control-url https://vpn.kubelab.live` writes `control_url`
- [ ] Implement: add `--profile` flag to `initCmd` in `cmd/cli/init.go`; when flag is set, call `Store.Set` instead of writing `.env`
- [ ] Test: `init` without `--profile` still writes `.env` — regression test unchanged

## 4. Secrets guard

- [ ] Test: `Store.Set` returns error if `target` contains `tskey-` or `hskey-` (guard against accidental secret in target field)
- [ ] Implement guard in `Store.Set`

## Closing

- [ ] Every acceptance criterion from `proposal.md` covered by ≥1 test
- [ ] `features.json` populated with verification commands for each criterion
- [ ] `go build ./...` clean
- [ ] Lint: no new goconst / staticcheck violations (string literals extracted as consts if repeated ≥3 times)
- [ ] `verification.md` filled in
- [ ] PR opened with `Closes #185`

## Machine-readable features

Sibling `features.json` to be added once tasks are frozen.
