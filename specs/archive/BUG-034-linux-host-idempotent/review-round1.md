---
spec: "BUG-034-linux-host-idempotent"
verdict: "FAIL"
reviewed_sha: "79492efe17caecf70de4d7cdefcb7a5107630b8b"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-22"
---

## Adversarial review

**Scope**: `BUG-034-linux-host-idempotent` (issue mlorentedev/ts-bridge#454), diff `d881023670240b0cc35716e899854e31fec669b4...HEAD` (3 commits, 12 files, +775/−99). Contract digests in `review-request.json` verified against `dotf spec digest BUG-034-linux-host-idempotent` — **identical**, so this review describes the current contract.

**Sources**: `specs/BUG-034-linux-host-idempotent/{proposal,tasks,verification}.md`, `features.json`; `git diff d8810236...HEAD` for `internal/host/{linux_steps,step}.go`, `internal/host/{linux_steps_test,steps_test}.go`, `cmd/cli/host.go`, `docs/design/host-client-parity.md`, `docs/lessons/lesson-042-2026-10-22.md`.

**Verification actually run in this session**
- `go build ./...` → exit 0; `go test -race -count=1 ./...` → all packages ok; `GOOS=linux|windows|darwin go vet ./...` → clean; `golangci-lint run` → `0 issues`.
- Mutation 1: deleted the `!r.Elevated()` gate in `readLinuxFirewall` → `TestSetupForLinux/not_root:_no_firewall_command_runs` and `TestCheckForLinux/not_root,_iptables_host` went red (4 assertions). The elevation gate is genuinely pinned.
- Real-ufw probe: `docker run --rm ubuntu:24.04` (ufw 0.36.2) → `ufw show added` prints `ufw allow 3389/tcp comment 'xrdp'` and `ufw allow 3000:3010/tcp`. Fed verbatim into `ufwAllows` via a throwaway in-package test (deleted afterwards): both return **false**. See F1.
- Not verified by me: the end-to-end container run claimed in `verification.md` (two sequential `host setup` runs, `iptables -S INPUT`), and the `docs/installed`/binary behaviour on a real Linux host — UNVERIFIED here beyond the parser probe above.

### Spec and task alignment

- All five ACs have a named test in `internal/host/linux_steps_test.go` (`TestUFWAllows`, `TestIptablesRule`, `TestXRDP`, `TestSetupForLinux`, `TestCheckForLinux`), and `features.json` verification commands resolve to those run patterns; each `features.json` entry is `state: pending` with empty `evidence`, i.e. no agent-claimed `passing` (correct per the spec's own gating rule).
- AC1's wording ("records no `run` call") is **not** what the code does on the iptables backend: the read is `iptables -C` executed through `Runner.Run`, and `TestSetupForLinux/iptables, converged` pins that `run` call. `verification.md` honestly records the exception; `proposal.md` and `tasks.md` do not (F2).
- Tasks: every Implementation box is `[x]` with diff evidence; the Closing boxes are unticked, which is correct pre-merge. `verification.md` promotion lines are all answered.
- Scope matches the proposal: no unrelated production change; the only non-`internal/host` code edit is two help-text lines. `ADR-017` needed no amendment, per the proposal.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | firewall read (`ufwAllows`) | The `ufw show added` parser only understands a bare rule body, so a legitimate inbound allow written with a **comment** or as a **port range** reads as *absent*. `ufwRuleMatchesPort` looks for a bare `<port>`, `<port>/tcp` or `... port <port>` tail and stops there; a trailing `comment '...'` makes the field slice 3 long with no `port` keyword, so it falls through to `false`. Consequence: `host check` reports `changed` ("No firewall rule found") on a host where the port **is** allowed, and `host setup` appends a second `allow` rule that the operator never asked for — the same duplicate-rule class (#454 L1) the spec exists to remove. | ufw 0.36.2 in `ubuntu:24.04`: `ufw show added` → `ufw allow 3389/tcp comment 'xrdp'`, `ufw allow 3000:3010/tcp`. In-package probe: `ufwAllows(header+"ufw allow 3389/tcp comment 'xrdp'\n", 3389) == false`, `ufwAllows(header+"ufw allow 3380:3390/tcp\n", 3389) == false`. | `TestUFWAllows` exists but has **no** case for a comment or a range → these inputs are **UNTESTED** | code (`internal/host/linux_steps.go: ufwRuleMatchesPort`) + tests (add the two verbatim lines to `TestUFWAllows`) |
| Minor | REAL | spec vs code (AC1) | `proposal.md` AC1 and `tasks.md` ("converged host makes no `run` call") state a property the implementation deliberately does not have: on the iptables backend the read is a `run`, not an `output`. Behaviour is fine (read-only `-C`), the contract text is wrong. | `TestSetupForLinux/iptables, converged` expects `run iptables -C INPUT -p tcp --dport 3389 -j ACCEPT`; `verification.md` AC1 line says "except the read-only `-C`". | `TestSetupForLinux/iptables,_converged` (pins the opposite of the AC text) | spec (`proposal.md` AC1 / `tasks.md` wording) — legal now that the verdict is FAIL; a re-review follows anyway |
| Minor | REAL | spec vs code (`services[].active`) | Proposal: "`services[].enabled` and `services[].active` come from those reads, so `active` stops being `null` on Linux". When `systemctl` is missing or answers nothing, `checkLinux` leaves `Active` nil and `Enabled` false, so `active` is still null on that path. Defensible (can't read ≠ false) but contradicts the sentence, and `enabled=false` there is a claim the read cannot support. | `TestCheckForLinux/no_systemctl,_ufw_unreadable` expects `services: [{Kind:rdp Port:3389}]` (`Active` nil, `Enabled` false) alongside an `unverified` step. | `TestCheckForLinux/no_systemctl,_ufw_unreadable` | spec (`proposal.md` "Services and legacy keys") or code (leave `Enabled` nil-able) — author's call |
| Minor | REAL | stale comment | `checkFor`'s doc comment in `internal/host/step.go` still says check "never calls Run". No longer true: the iptables read goes through `Run`. | `step.go` comment vs `TestCheckForLinux/xrdp_not_installed` (`run iptables -C ...` in the recorded calls). | `TestCheckForLinux` (behaviour correct; nothing guards the comment) | code (comment) |
| Minor | REAL | stale test fixture | `cmd/cli/host_json_contract_test.go` still builds its fixture step with the retired name `xrdp detection`. It is a serialization fixture, not a real Linux step, so nothing breaks — but the name no longer exists in production and misleads the next reader of the JSON contract. | `TestSetupJSON_StepsCarryOutcomeAndFix` uses `Name: "xrdp detection"`; production now emits `xrdp service`. | `TestSetupJSON_StepsCarryOutcomeAndFix` | tests (cosmetic) |
| Minor | THEORETICAL | xrdp detection | "Installed" is still inferred from `LookPath("xrdp")`. On Debian/Ubuntu the binary is `/usr/sbin/xrdp`, so a caller without `sbin` on `PATH` sees `xrdp is not installed` → `failed` in check rather than `unverified` (AC5's "a read that cannot be done"), which points the operator at `apt install`. Spec-consistent ("Not installed: `failed`"), so this is a probe-quality note, not a deviation. | code read of `readXRDP`; no repro (root setup has `sbin` on `PATH`) | `TestXRDP/not_installed` (asserts the current mapping, not the PATH hazard) | code (follow-up ticket, not this PR) |
| Minor | THEORETICAL | xrdp service | `readXRDP` maps only the literal word `enabled` to "enabled". `static`, `indirect` or `enabled-runtime` units all fall into "not enabled", so setup runs `systemctl enable --now` and reports `failed` for a unit that cannot be enabled. xrdp ships with `[Install]`, so this is unlikely in practice. | code read; no repro | UNTESTED (no `static` case in `TestXRDP`) | code (follow-up ticket) |

**Test deletions** (3 Linux cases removed from `internal/host/steps_test.go`, moved to `linux_steps_test.go`): each removed case has a strictly stronger successor — "linux with ufw" → `TestSetupForLinux/ufw, converged`; "linux without ufw or xrdp, Tailscale down" → `…/xrdp missing, no systemctl, no backend, Tailscale down`; "linux, firewall unreadable without root" → `TestCheckForLinux/not root, iptables host`. No seam lost a production caller. History of the removed cases (`git log -S`) not checked — UNVERIFIED, but no field of the retention bar points at a lost contract.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Happy paths and the AC-3 exact-port cases are pinned, but the read produces real false negatives (F1) that contradict AC1/AC3 for realistic ufw state. |
| Verification       | B | `verification.md` names a command per AC plus a container run; reproducible except the container transcript, which is not committed and missed F1. |
| Scope              | B | Diff matches the proposal; only leftovers are a stale comment, a stale test fixture and AC wording. |
| Reliability        | C | Error paths and elevation gating are solid (mutation-verified), but idempotence breaks on a pre-configured commented/range rule. |
| Maintainability    | B | Small functions, low complexity, parser unit-tested in a table; one false doc comment. |
| Handoff-readiness  | B | Spec triad, lesson-042 and the parity doc are updated in the diff; contract wording left behind by the implementation (F2/F3). |

Aggregation: one **REAL Major** → **FAIL** (the severity × reality path, not the rubric path). The rubric's `C`s point the same way.

### Verdict

**FAIL** — one REAL Major: the ufw read is not idempotent against a ufw rule this project's users will actually have (`ufw allow 3389/tcp comment '…'`), reproduced against verbatim ufw 0.36.2 output, and untested.

### Recommended next steps

1. **Code + tests (required to flip the verdict).** In `internal/host/linux_steps.go`, make `ufwRuleMatchesPort` tolerate a trailing `comment '…'` (truncate the field slice at `comment`) and a port range that contains the port (`a:b/tcp`), or state in `proposal.md` that ranges are excluded deliberately. Add both verbatim lines from the probe to `TestUFWAllows` so the fix is red-green. Nothing here touches the contract set, so a re-review does not have to wait for a digest change.
2. **Spec set (only because the verdict is FAIL).** AC1 in `proposal.md` and the matching `tasks.md` line overstate what the code does ("records no `run` call"); reword to "runs no mutating command" — the property `TestSetupForLinux` actually pins. Decide the `services[].active`-on-unreadable sentence the same way. These edits change the contract digests; the re-review that follows F1's fix covers them.
3. **Ticket, not this PR.** `LookPath("xrdp")` as the installed-probe (sbin off `PATH` → `failed` instead of `unverified`) and the `static`/`indirect` unit mapping.
4. **Cosmetic, any time.** `checkFor`'s "never calls Run" comment; the `xrdp detection` fixture name in `cmd/cli/host_json_contract_test.go`.

### Archive advice

`dotf spec archive` is **not** advisable in this state: the review is `FAIL`, and `proposal.md` is still `status: implementing`. Flip to PASS by fixing F1 with a named regression case, then re-running the review; items 2–4 can ride along in the same round.
