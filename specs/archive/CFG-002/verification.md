---
tags: [spec, verification, templates]
created: "2026-06-23"
---

# Verification - CFG-002

## Evidence

- [x] `host setup`/`host check` emit a descriptor with the detected port → `TestBuildDescriptorLine_SaaSControl` / `TestBuildDescriptorLine_HeadscaleControl` / `TestBuildDescriptorLine_EmptyIP_ReturnsEmpty` / `TestBuildDescriptorLine_ZeroPort_ReturnsEmpty` (commit `a7b6b4b`)
- [x] Importing a descriptor yields a profile whose target uses the descriptor's port; `connect --profile` dials it → `TestRunImport_WritesProfile` / `TestApplyProfile_ResolvesTargetFromStore` (commit `da510cf`)
- [x] Descriptor round-trips losslessly incl. version; malformed input errors without panic → `TestDescriptorRoundTrip*` / `TestParse_Malformed*` / `TestParse_AbsentVersionDefaultsToOne` / `TestString_OmitsVersionWhenOne` (commits `dd4c09d`, `d729b49`)
- [x] Backward compat: `TS_TARGET=host:port` works unchanged with no profile → `TestBackwardCompat_ExistingTargetWithoutProfile` / `TestApplyProfile_EmptyName_NoOp` (commit `520ace8`)
- [x] Cross-platform emission: `buildDescriptorLine` is OS-agnostic (pure Go, no syscalls) → verified by tests running on Windows CI + Linux CI (commit `a7b6b4b`)
- [x] Control-plane parity: `cp=<Headscale URL>` descriptor imports and connects identically to `cp=saas` → `TestControlPlaneParity_HeadscaleDescriptorRoundTrip` (commit `520ace8`)
- [x] Idempotent import: same descriptor imported twice → single, uncorrupted profile → `TestStore_Import_Idempotent` (commit `d729b49`)
- [x] `.env.example`, `README`, `docs/` document import flow and `discover --port` behavior → commit `be79aa1`

## Test status

- Test suite: `go test ./...` → all 10 packages pass, 0 failures
- No regressions in existing test suite: yes

## Decisions made during implementation

- `buildDescriptorLine(ip, port, controlURL)` placed in `cmd/cli/host.go` (same package as discover.go), not in `internal/profile` — avoids a cmd → internal import cycle; reused by both `host` and `discover` output functions.
- `defaultProfileStorePath` is a package-level `var` (not a const) so tests can inject a temp path without touching the real user store.
- `applyProfile` only fills `cfg.Target`/`cfg.ControlURL` when they are empty — explicit `--target`/`TS_TARGET` always wins, maintaining full backward compatibility.
- `discover.updateEnvFile` now takes `controlURL string` so the printed descriptor reflects the active control plane (Headscale or SaaS), not always SaaS.

## Promotion candidates

- [ ] Lesson for `docs/lessons.md`? No — this is a clean feature addition with no surprises.
- [ ] ADR-worthy decision? No — `buildDescriptorLine` placement is minor; the tsb:// scheme design is already in ADR-011.
- [ ] New pattern candidate? No — single project.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CFG-002/` -> `specs/archive/CFG-002/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
