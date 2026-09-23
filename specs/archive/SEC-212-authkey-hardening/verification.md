---
id: "SEC-212-authkey-hardening"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#212"
tags: [spec, verification, security, hardening]
template_version: "1.0"
---

# Verification - SEC-212: Auth Key Security Hardening

## Evidence Checklist

- [x] `go test -race ./...` passing.
- [x] `golangci-lint run` clean (0 issues).
- [x] `gosec ./...` clean (0 issues).
- [x] Verification of `ts-bridge init --help` and generated configs security notes.
- [x] Documentation updated across README.md, .env.example, runbooks, and security audit.

## Follow-up evidence (issue #305, 2026-08-31)

The remainder PR #298 left uncommitted was recovered from the dead branch's stash, then
re-verified against a binary built from `master` rather than trusted on the strength of the diff:

- `go build ./...` clean; `go test ./...` green across all 10 packages
  (`cmd/cli`, `internal/{config,config/envfile,discover,health,host,logging,profile,proxy,telemetry}`).
- Every flag named in the touched runbooks was executed against `master`:
  - `ts-bridge init --auth-key-file /tmp/k --target 100.0.0.1:22` → `unknown flag: --auth-key-file`,
    exit 1. `init` has **no** key-file flag, so the runbook shows the masked interactive prompt
    instead of the example the original draft contained.
  - `ts-bridge connect --auth-key-file /tmp/k --target 100.0.0.1:22` →
    `read auth key file: stat auth key file: stat /tmp/k: no such file or directory`, proving the
    documented path is the supported one.
- Gap surfaced by that check filed as #306; the Linux runbook deferral and its reason are recorded
  in `tasks.md` and filed as #307.
- Reviewer output on #310 was dispositioned under its `## Review triage` comment: five findings
  applied (duplicated code fence, a "non-interactive" heading over a prompting example,
  POSIX-only syntax in an "All platforms" block, AC3 ticked while deferred, an unfinished
  verification sentence) and one Major declined as refuted by the code.

## Adversarial review round 1 (FAIL, 2026-09-21) — dispositions

| Finding | Disposition |
|---|---|
| **Major**: AC3 required every runbook to use `--auth-key-file`, but `guide-deployment-linux.md` was skipped | **Contract rescoped**, as the review's first recommendation proposed. The Linux runbook has no CLI auth-key example to convert: it uses a `0600` `EnvironmentFile`. Converting it needs a restored systemd unit with `ExecStart --auth-key-file` (#307). AC3 now names that exclusion and the `init` exception (#306) |
| Minor: `TestInit_SecurityGuidance` did not check the `--auth-key` flag's process-list warning | **Applied**: the test now asserts that `Flags().Lookup("auth-key").Usage` contains `visible in process list`. Mutation check: removing the warning from `init.go` makes the test fail (`init_test.go:276`) |
| Minor: the multi-device quick setup passes `Get-Content` (and `$(cat …)`) to `--auth-key` | **Declined for this spec**: `init` registers no `--auth-key-file`, so an unattended `init` has no key-file form yet. The example is labelled as process-table-visible and points to `connect --auth-key-file` for launch. Fixed by #306 |

Evidence for the rescoped AC3, from `grep -rnE -- '--auth-key[ =]' docs README.md .env.example` excluding
`auth-key-file`: exactly two hits, `guide-multi-device-operations.md:67` (POSIX) and `:75`
(PowerShell), both unattended `init` calls under the process-table warning. No `connect`
example uses inline `--auth-key`.

## Adversarial review round 2 (PASS-WITH-GAPS, 2026-09-22) — dispositions

Reviewer `nan/deepseek-v4-flash`, `reviewed_sha` `00c71b9`. Every finding sat outside the contract
set (`proposal.md`, `tasks.md`, `features.json`), so dispositioning them here keeps the verdict fresh.

| Finding | Disposition |
|---|---|
| F1 (Minor): `init --help` said "consider using `--auth-key-file`", a flag `init` does not register | **Applied.** The security note now names `ts-bridge connect --auth-key-file /path/to/key`, states that `init` has no key-file flag, and warns that inline `--auth-key` is visible in the process list. The examples block says the non-interactive forms expose the key. `TestInit_SecurityGuidance` asserts the `connect` form and refuses the old wording. It failed before the fix |
| F2 (Minor): `discover` takes an inline `--auth-key` with no process-list warning | **Applied (warning) + ticketed (key file).** `discover`'s usage now carries the warning. The new `TestAuthKeyFlags_WarnProcessList` walks the whole command tree, so any command registering `--auth-key` must carry it (it went red on `discover` before the fix). The key-file flag for `discover` was folded into #306 |
| F3 (Minor): the `printNextSteps` tip and the YAML-mode `.env` sidecar note were unpinned | **Applied.** `TestPrintNextSteps_SecurityTip` captures stdout; `TestWriteYAMLConfig_CreatesYamlAndEnv` asserts the sidecar note |
| F4 (Minor): AC3's invariant is a manual grep, not a guard | **Ticketed: #348** (`check-doc-authkey.sh` in `repo-hygiene.yml`) |
| F5 (Minor): the Linux runbook's `cp scripts/host/ts-bridge.service` step is dangling | **Already recorded in #307**, whose body cites that exact line and its deletion in #156 |
| F6 (Question): no `features.json` | **Left absent, deliberately.** Backfilling adds a fourth contract file, which makes this review stale by construction and forces another round for no behaviour change |

Mutation evidence for this round. Each new assertion failed once the string it guards was removed:

```
[next-steps-tip]     removing the connect --auth-key-file tip          -> FAIL
[yaml-sidecar-note]  removing the .env "Secure alternative" line       -> FAIL
[help-connect-line]  removing "ts-bridge connect --auth-key-file" help -> FAIL
[discover-warning]   discover usage without the warning                -> FAIL (before the fix)
```

`go test -race ./...` green across all packages; `golangci-lint run ./...` 0 issues after
`golangci-lint cache clean` (the reviewer hit a stale cache shared with a sibling worktree).
