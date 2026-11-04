---
tags: [spec, verification]
created: "2026-11-04"
id: "HOST-455-windows-host-state"
type: spec-verification
status: active
owner: manu
---

# Verification - HOST-455-windows-host-state

## Evidence

- [ ] AC1 → PR 1, test `TestWindowsUnattendedStep`. It has five cases:
  - already unattended;
  - Headscale prefs without `ForceDaemon`;
  - unparseable prefs;
  - unreadable prefs;
  - a failing `set`.

  The test asserts that no call starts with `run tailscale up`. The golden Windows setup sequence
  (`windowsSetupCalls`) now starts with `output tailscale debug prefs`, then
  `run tailscale set --unattended`. Mutant checks: dropping `&& prefs.ForceDaemon` fails two
  subtests (`headscale node, not unattended`, `set fails`). Trusting a failed read's stdout
  (`out, _ :=`) fails `prefs unreadable`, whose runner exits 1 while printing ForceDaemon true.
- [ ] AC2 → PR 2, test `TestWindowsRDPDenyState`. Six (policy, local) cases, run through both
  `rdpSetupStep` and `checkWindows`: local allows, local denies, nothing set, policy denies over a
  local allow, policy allows over a local deny, and a failed read. The test asserts the outcome,
  whether setup wrote, `services[].enabled`, and that a `manual` fix names the policy key.
  Mutants: ignoring the policy value fails 2 subtests; treating an absent value as allowed fails 1.
- [ ] AC3 → PR 3, test `TestWindowsFirewallRule`. Eight cases, run through both setup and check:
  converged, absent, other port, disabled, a Block rule, an Outbound rule, a duplicated rule, and a
  failed read. Setup recreates only when the rule is not converged; check reports `firewall_ok`
  false in every non-converged case. Mutants: dropping the port comparison fails 1 subtest;
  dropping the `count == 1` test fails 1.
- [ ] AC4 → every test above drives the `Runner` seam (`recordingRunner`) with no build tag, so it
  runs on Linux, macOS and in `test-windows`. `GOOS=windows go vet ./...` is clean.

## Test status

- PR 1: `go test -race ./...` exit 0. `golangci-lint run ./...` reports 0 issues. `gosec ./...`
  exit 0. `GOOS=windows go vet ./...` exit 0.
- Not observed on a live host. Fixtures pin the output shapes of `tailscale debug prefs` and of the
  PowerShell reads; nobody has captured them from a Windows machine. **Live acceptance is Manu's**:
  run `ts-bridge host setup` twice on a Headscale-joined Windows host. The second run must report
  `Tailscale unattended mode=ok` (and RDP and firewall `ok` once PR 2 and PR 3 land).

## Decisions made during implementation

- W1 reads `tailscale debug prefs` (a debug subcommand) and fails open toward the `set`: only a
  positive parse of `ForceDaemon: true` skips the write. Format drift then costs one redundant
  idempotent command, never a skipped step (lesson-053).
- W3 also checks the rule's action (`Allow`) and direction (`Inbound`), beyond the AC's port and
  state: a same-named Block rule would otherwise read as converged.
- W3 on an unreadable rule: setup recreates it, because the recreate is idempotent. Check reports
  `failed`, because it cannot say what setup would do.
- `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"` now runs only when the named rule is
  recreated. The named rule opens the declared port, which is all the step promises; on a converged
  host the built-in group is left as the user set it.
- A port read of 0 (unknown) skips the firewall port comparison in check, as before, rather than
  reporting every rule as drifted.
- `host check` still does not report unattended mode; see Out of scope in `proposal.md`.
- `site/src/content/docs/getting-started.md` still says `tailscale up --unattended`. `site/**` is
  outside this change's paths, so a follow-up issue tracks it.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-053-2026-11-04.md
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: applies ADR-017's read-then-act step contract, decides nothing new
- [ ] New pattern candidate for `00_meta/patterns/`? no: one occurrence, ts-bridge only

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/HOST-455-windows-host-state/` -> `specs/archive/HOST-455-windows-host-state/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
