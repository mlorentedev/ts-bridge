---
spec: "HOST-007-ssh-service-linux"
verdict: "PASS"
reviewed_sha: "981e84dda284e637ac86faaee4a9396f80787cdd"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-12"
---

## Adversarial review

**Scope**: HOST-007-ssh-service-linux (Linux half of #521) — `git diff 29e3ae75d3c66e8003d0dda5d113c412abfdaed6...HEAD`, `reviewed_sha` 981e84dd.
**Sources**: `specs/HOST-007-ssh-service-linux/{proposal,tasks,verification}.md`, `features.json`, `review-request.json`;
`internal/host/{linux_ssh.go,linux_steps.go,file.go}`, `cmd/cli/{host.go,*_test.go}`, `docs/adr/adr-017-*.md`,
`docs/design/host-client-parity.md`, `docs/runbooks/*`, `docs/lessons/lesson-059-*`, `scripts/e2e/linux.sh`.

Branch commits in range (`--first-parent --no-merges`): `9cc837e docs(spec)`, `03dfb1c feat(host) converge`,
`eb9979c feat(host) check lines`, `ee42906 fix(host) ssh service read when only its firewall is unverified`,
`981e84d test(e2e)`. The range also carries other work that arrived through a merge of `origin/master`
(`1ba33a6`, `f369cfe` #764, `specs/archive/BUG-762-v4-operator-chains`); it was judged only where it
interacts — `linuxFirewallOK`→`partlyUnread` (`chainUnread`) and the `firewallSetupFor`/`firewallCheckStep`
split, both of which the ssh path now calls (checked, consistent: the ssh service's `firewall_ok` inherits
the chain-read rule exactly as rdp's does, and `sshNamed` renames a step that may come from
`ufwInactiveStep`/`foreignStep`/`withChains` without losing its outcome).

Contract integrity: `dotf spec digest HOST-007-ssh-service-linux` matches every `contract_digests` value
in `review-request.json` (`features.json 2bcac358…`, `proposal.md d94f3e75…`, `tasks.md ffa7f19c…`).
No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain.

### Evidence produced in this session

| Claim | Command | Result |
|---|---|---|
| Build | `go build ./...` | exit 0 |
| Unit + CLI tests | `go test ./internal/host/ ./cmd/cli/ -count=1 -timeout 10m` | `ok` both packages |
| Whole suite | `go test ./... -count=1 -timeout 15m` | 16 packages `ok`, no failures |
| Lint (as CI) | `golangci-lint run ./...` with `GOOS=linux`/`darwin`/`windows` (mise pin 2.14.0) | `0 issues` ×3 |
| Format / vet | `gofmt -l .` (empty), `go vet` per `GOOS=linux|darwin|windows` | clean |
| Feature replays f1–f5 | the five commands in `features.json` | all `ok` |
| AC6 red-green | reverted `sshFirewallRead`'s reclassification in a copy, re-ran `TestSetupSSHKeepsTheOperatorsOwnAllow` | red with exactly the two messages `verification.md` quotes (`ufw delete allow 22/tcp`, `iptables -D INPUT -p tcp --dport 22 -j ACCEPT` ×2), then green after restore |
| Behaviour probes | throwaway `zz_review_probe_test.go` in `internal/host` (removed; `git status` clean outside the spec folder) — operator-allow setup/check, ssh-declared-first order, socket-enabled/service-disabled | see findings 1, 2, 3, 6 |

Not reachable here, therefore **UNVERIFIED**: `make e2e-linux` (no VM in this worktree), `gosec`
(CI job only), and any real `systemd`/`ufw` host. Every real-host claim in the spec rests on the
fake `Runner`, as the proposal itself states for socket activation.

### Spec and task alignment

- **AC1** — `osCapabilities[linux]` accepts `ssh`, `windows` keeps `planned{ssh: "#521"}`
  (`internal/host/file.go`). Named tests exist and pass: `TestLoadFile_OSCapability` (linux/ssh
  accepted with no port, windows/ssh cites `#521`), `TestLoadServeFile` (linux accepted, windows
  still `#521`), `TestHostCmd_ConfigSSHOnLinux` (setup **and** check reach the runner on linux with
  `Services == [{ssh 2222}]`, windows stops at exit 1 with zero calls), `TestHostSetupCmd_ConfigOSCapability`
  moved to `vnc`. `LoadServeFile`'s own code is untouched and it already accepted `ssh` on serve-capable
  OSes (it appends `ServiceSSH`), so "unchanged" is accurate for the function; the rewritten test half
  correctly re-points the old assertion at `LoadFile`.
- **AC2/AC3/AC8** — `linuxServices` drives both halves; a host declaring nothing is still
  `[{rdp, linuxPort}]` (`TestCheckSSHDefaultStaysRDP`, `TestSetupSSHOnlyReportsNoRDPPort`).
  `readUnit`/`linuxUnit` generalize the xrdp read; step messages and names are preserved for xrdp
  (only test-fixture constants moved in `linux_steps_test.go` — no assertion was changed), and check
  still runs no mutating command (`ran` empty in every `TestCheckSSH` row). Missing server → `manual`
  with the package names and "does not install"; masked → `failed` (setup) / `manual` (check);
  alias/`indirect` → unverified; no `systemctl` → unread, never "off". Socket credit is pinned by
  `TestReadSSH` (`socket row`, `masked not rescued`).
- **AC4/AC5** — `deriveLegacy` (pre-existing, `step.go`) was the right seam: legacy keys from rdp alone,
  `firewall_ok` over every service. `TestCheckSSH` (rdp+ssh row, ssh-port-closed row) and
  `TestCheckJSON_ListsSSHBesideRDP` pin it; `serviceLines` builds one line per `services[]` entry and
  keeps the byte-identical `RDP:` line (`TestPrintCheckText_ServiceLines`, `TestPrintCheckText_Service`
  untouched and green). A no-service result still prints `RDP: no service declared`.
- **AC6** — `sshFirewallRead` reclassifies the ufw legacy allow and the iptables copies as `foreign`,
  so `legacyDeletes` sees nothing to delete; `legacySetupStep` still adds the scoped rule when it is
  absent (probed) and reports `manual`; `ufwInactiveStep` also reports `foreign`, so the inactive-ufw
  variant is covered by code even without a test. Red-green reproduced (table above).
- **AC7** — `sshPortStep`/`readSSHPorts`/`sshdPort` never write; `TestSSHPort` covers match,
  `listenaddress`-carried port, mismatch naming both ports, unreadable (`failed`/`unverified`),
  no-port answer; `TestSSHPortDefaultAndMissingReadNothing` proves port 22 and a missing server read
  nothing. `sshd -T` needs root and is reported as such (documented risk).
- **Tasks** — every implementation box is `[x]` with matching diff evidence; the two open closing
  boxes are honest (`make e2e-linux`, `features.json` replay + review). `features.json` f6 is
  `pending`, consistent with `verification.md`.
- **Docs** — ADR-017 §5 row, Amendment A note, parity doc, `host-serve.md` and the new lesson 059 all
  match the code (task list vs manual/never-deleted, no `sshd_config` write, `manual` for a missing
  server, per-service firewall rule). No speculative capability is claimed.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | tests | AC6's *"it adds the scoped rule"* half is proven by no committed test: both `TestSetupSSHKeepsTheOperatorsOwnAllow` fixtures already hold the scoped rule (`ufwSSHAllow`, `iptScopedSSH` + `ts-input`), so the test can only prove "nothing deleted" and "manual". The adding half works — a throwaway probe with `ufw show added` holding only `ufw allow 22/tcp` ran exactly `ufw allow in on tailscale0 to any port 22 proto tcp` and deleted nothing — but a regression that skipped the add would go unnoticed. | probe run; `linux_ssh_test.go:175-200` | UNTESTED (no named case for the add) | tests |
| Minor | REAL | tests | The **check** half of the same reclassification is untested: no `TestCheckSSH` row holds an operator-owned unscoped allow, so nothing pins `services[].firewall_ok == false` and the `manual` firewall step for that host. Probe: `{Kind:ssh Port:22 Enabled:true FirewallOK:false}`, step `manual … legacy unscoped rule: ufw allow 22/tcp`. | probe run | UNTESTED | tests |
| Minor | THEORETICAL | reliability | Reproduced the activation-model drift the proposal already declares: with `ssh.service` disabled+inactive and `ssh.socket` enabled+inactive, setup converges `systemctl enable --now ssh` (the *service*), leaving socket **and** service enabled at next boot instead of starting the socket. Converges and keeps the port answering, so not incorrect — but it is a behaviour the owner said was his to decide. Reproduced only through the fake runner. | probe: `SOCK runs=["systemctl enable --now ssh"] steps=["SSH server=changed", …]`; proposal Risks bullet 4 | `TestReadSSH`/`socket row` pins the read, not the convergence target | code (prefer `enable --now <unit>.socket` when the socket unit exists) or an ADR-017 line — ticket, owner's call |
| Minor | REAL | docs | `host setup --json` on an ssh-only host emits `"rdp_port": 0` (and no descriptor, correctly: `buildDescriptorLine` returns `""` at port 0). Only the **text** summary change is documented; the JSON key's new zero value on a converged host is not. | `cmd/cli/host.go:596-606`, `:531-537` | UNTESTED (no ssh-only setup-JSON assertion) | docs (ADR-017 §3 note) |
| Minor | SPECULATIVE | spec wording | AC2's *"runs no command on a converged host"* is literally false and cannot be satisfied: a converged ssh host still reads `systemctl is-enabled`/`is-active` (and `sshd -T` for a declared non-default port), exactly as the xrdp path always has. The tests prove the intended claim — no **mutating** command (`ran == nil`). | `TestSetupSSH` converged row; `linux_ssh.go:28-48,121-135` | `TestSetupSSH` | spec (contract set — **do not edit now**; carry as a follow-up wording fix or decline) |
| Minor | SPECULATIVE | spec wording | AC4 says `services[]` lists ssh *"after rdp where both are declared"*, while the What says *"in declaration order"*. A file declaring ssh first yields `[ssh, rdp]` in `services[]` and in the step list, which is the What's rule. Ambiguity, not a defect. | probe: `ORDER services=[ssh, rdp] steps=["Tailscale IP=ok" "SSH server=ok" … "xrdp service=ok" …]` | `TestCheckSSH`/rdp+ssh row (rdp-first only) | spec (contract set — carry as a follow-up, or accept declaration order as the rule) |
| Question / assumption | SPECULATIVE | resilience | `linuxServices` → `setupLinux`/`checkLinux` switch has no `default`: a declared kind with no Linux step would converge nothing and the summary would still read `HOST READY`. Unreachable today (the loader admits only `rdp`/`ssh` on linux), but it is the seam a later `vnc` row would land in. | `internal/host/linux_steps.go:47-90` | UNTESTED | code (a `default` that emits an explicit failed/unverified "no step for kind" step) — defensive, ticket only |
| Minor | REAL | docs/wording | A reclassified operator allow is described with the RDP wording: setup says *"legacy unscoped rule"* for a rule ts-bridge never wrote, and check says *"allows TCP 22 on every interface …, not only tailscale0"* even when the scoped rule is present. The proposal lists this as a known rough edge. | probe output (both messages quoted above) | UNTESTED | code (wording) — optional |
| Minor | REAL | archive readiness | `verification.md` has no `## Promotion candidates` section, which `dotf spec archive --help` names as a **third pre-flight** ("each line must be answered … No flag skips it"). Every archived spec in `specs/archive/` carries it, and this change already promoted lesson 059, so the answer is `yes: docs/lessons/lesson-059-2026-11-12.md`. Whether an absent section passes or refuses is **UNVERIFIED** (I did not run `archive`). | `dotf spec archive --help`; `grep -rn "Promotion candidates" specs/`; `specs/archive/BUG-762-v4-operator-chains/verification.md` | UNTESTED | verification.md (add the section — outside the staleness set) |
| Minor | REAL | verification artifact | No ssh behaviour has run on a real guest: `features.json` f6 is `pending` and `tasks.md` leaves `make e2e-linux` unticked. The Fedora `ssh`→`sshd` lookup, socket activation and root `sshd -T` are fake-runner only. Declared honestly, but it is the largest remaining unknown. | `features.json` f6; `scripts/e2e/linux.sh` cases named, unrun | e2e cases exist, not executed | verification.md (record the run when the lead does it — outside the staleness set) |

No Blocker and no REAL Major was found. The path that mattered most (never deleting the operator's
port-22 allow) is both correct and red-green proven; the two test gaps above are about *guarding* it,
not about it being wrong.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1–AC5, AC7, AC8 verified by named tests I ran; AC6's delete-half red-green reproduced and its add-half probe-verified, but two negative-path rows (add-half, check-half) lack committed tests |
| Verification       | B | Every criterion maps to a named, reproducible Go test, and all five feature commands replay green; real-host e2e pending and declared |
| Scope              | A | Diff matches the proposal exactly (loader flip, `linux_ssh.go`, `linuxUnit` generalization, CLI lines/summary, docs, one e2e case); merged-in #764 work is other work, not creep |
| Reliability        | B | Idempotent (converged host mutates nothing), no `sshd_config` write, foreign rules never deleted, all error paths report a fix; socket-activation convergence target is a declared, owner-deferred tradeoff |
| Maintainability    | A | `linux_ssh.go` functions are short (≤ ~20 lines), CC well under 10, one shared systemd read for xrdp and ssh, comments explain why; lint/vet clean on three GOOSes |
| Handoff-readiness  | B | ADR, parity doc, two runbooks and lesson 059 written in-session; tasks/verification honest; e2e box open and f6 `pending` |

Aggregation: no D, no C — all B or above → **PASS**. Severity axis adds nothing more severe (no Blocker, no REAL Major).

### Verdict

**PASS**

`dotf spec archive` is **advisable**: the review is fresh and passing, the reviewer is in
`harness/reviewer-pool.json`, `reviewed_sha` is the commit under review, and the three contract digests
still match `review-request.json` (verified with `dotf spec digest`, not `sha256sum`). The open e2e box
does not gate the archive; record its result in `verification.md` when the lead runs `make e2e-linux`
(that file is outside the staleness set, so the review survives it).

One precondition is **UNVERIFIED**: the archive's third pre-flight reads `verification.md`'s
`## Promotion candidates` section, which this spec does not have. Add it first (see next steps) rather
than discovering the refusal at archive time.

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** by this verdict — editing any of
them would invalidate it. Findings 5 and 6 are wording ambiguities in that set: leave them (the tests
already encode the intended reading) or carry them into a follow-up ticket; do not touch the files now.

Everything below is outside the contract set and can be applied without a re-review:

- **tests** — add the two missing rows: (a) an `ssh`+operator-allow fixture *without* the scoped rule,
  asserting `ufw allow in on tailscale0 to any port 22 proto tcp` runs and no delete does; (b) a
  `TestCheckSSH` row for the operator-owned unscoped allow, asserting `FirewallOK: false` and the
  `manual` firewall step. Disposition these in `verification.md` (applied / ticketed).
- **code (ticket, owner's call)** — the socket-activation convergence target (finding 3): when
  `<unit>.socket` exists, prefer `systemctl enable --now <unit>.socket`; otherwise record in ADR-017
  that the service is the converged unit and the drift is intended.
- **code (ticket, defensive)** — a `default` in the `linuxServices` switch emitting an explicit
  "no step for kind" outcome, so a later kind cannot converge silently (finding 7).
- **verification.md** — add the missing `## Promotion candidates` section (lesson: `yes:
  docs/lessons/lesson-059-2026-11-12.md`, written in this change; ADR: `no: ADR-017 §5 and Amendment A
  were amended in place, which is the decision's home`), then record the `make e2e-linux` result (and the Ubuntu socket-activated read, which
  the proposal says has never run on a guest); move f6 from `pending` to its real state in a way that
  does not edit `features.json` (a note in `verification.md`, or a follow-up ticket for the VM run).
- **docs (optional)** — note in ADR-017 §3 that `host setup --json` reports `rdp_port: 0` on an
  ssh-only host (finding 4), and soften the reclassified-allow wording (finding 8).
