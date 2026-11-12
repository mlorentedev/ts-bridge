---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-720-v6-jumped-chains"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-720-v6-jumped-chains

## Evidence

- [x] AC1 -> `TestIP6Exposure/an_unscoped_v6_allow_inside_ts-input` (check and setup `manual`, fix `ip6tables -D ts-input -p tcp -m tcp --dport 3389 -j ACCEPT`, `firewall_ok` false); `TestIP6Jumps/an_allow_in_ts-input`. Red at f08f7a1, green at b96a0a7.
- [x] AC2 -> `TestIP6Exposure/captured_chain` and `captured_chain,_iptables-nft` (the call list now includes `output ip6tables -S ts-input`); `TestIP6Jumps/tailscaled's_ts-input`. Fixtures copied from `scripts/e2e/capture-tsfw.sh` run at 14:13Z on 2026-11-12 (`scripts/e2e/out/tsfw-linux-20261112T141322Z.log`, tailscale 1.104.4): in `--netfilter-mode=on` with iptables, v6 `ts-input` is `-A ts-input -s fd7a:115c:a1e0::1/128 -i lo -j ACCEPT` and `-A ts-input -i tailscale0 -j ACCEPT`; under nftables mode the same with `lo+` / `tailscale0+`. Both rules are port-less, so neither is an exposure.
- [x] AC3 -> `TestIP6Jumps` rows: `a_jump_for_another_port_is_not_followed`, `a_jump_held_to_one_interface_is_not_followed`, `a_loopback_jump_is_not_followed`, `a_jump_on_every_interface_but_one_is_followed`, `two_jumps_deep`, `a_loop_ends`, `a_chain_jumped_to_twice_is_read_once`, `a_jump_through_goto`, `a_jump_for_the_port`.
- [x] AC4 -> `TestIP6Jumps/a_target_that_is_no_chain_is_skipped_without_a_note`, `verdicts_are_not_chains`, `a_chain_that_cannot_be_read_is_a_note,_INPUT_still_read`, `ufw's_chains_are_left_to_the_ufw_read`.
- [x] AC5 -> `make e2e-linux` on e9e8c7b (it carries the fix b96a0a7 and the e2e block eb0d4eb; later commits change docs and the spec only): 112 pass, 0 fail (log `scripts/e2e/out/linux-20261112T141804Z.log`). The new block creates `tsbe2e` in ip6tables with tailscaled's shape (`-i tailscale0 -j ACCEPT`) behind `-I INPUT -j tsbe2e`: check `ok`. With an unscoped 3389 allow added: check `manual` naming `ip6tables -D tsbe2e`, `firewall_ok` false, setup `manual` leaving the rule. Removed: check `ok`. Re-run on 85e640a, after the PR review fixes: 112 pass, 0 fail.

## Test status

- `go test -race ./...`: all packages pass.
- `golangci-lint run ./...` on darwin and with `GOOS=linux`: 0 issues.
- No regressions: the existing `TestIP6Exposure` rows pass with the added chain read in their call lists.

## Decisions made during implementation

- A jump's scope is judged by `ip6Narrowed` and `iptLoopback`, and its port match by `iptMatchesPort` on the rule with its `-i`/`-s`/`-d` removed (`ip6Match`). `iptMatchesPort` alone reads `! -i tailscale0` as narrowing, which would skip a jump that sends all non-tailnet traffic to the chain; the red row `a_jump_on_every_interface_but_one_is_followed` caught it.
- `ufw6-*` chains are not followed (proposal, Out of scope), so a ufw host's `ufw allow 3389` is reported once, by the ufw read, with a fix ufw keeps.
- The literal "No chain/target/match by that name" became `iptNoChain`, shared by the v4 `tsInputUnreached` and the v6 walk.

## Review dispositions

Round 1 (nan/deepseek-v4-flash, PASS, at 73bb484; this `review.md`):

- Minor REAL, misattached doc comments: applied. `ip6Match`'s comment sits on `ip6Match`, `ip6ScopeFlags` has its own, and `iptNoChain` sits above `chainMissing`'s comment, not inside it.
- Minor REAL, wrong ADR promotion reason: applied. The line now names the parity doc, which records the v6 scan; ADR-017 does not.
- Minor REAL, `tasks.md` Closing unticked: applied at archive time. Checkbox ticks are normalised out of the contract digest.
- Minor THEORETICAL, a port-less accept in a jumped chain reads `ok`: pinned and ticketed. The new row `TestIP6Jumps/a_port-less_accept_in_a_chain_is_the_operator's_posture` makes the current behaviour a stated decision, matching INPUT's row. Whether a port-less v6 accept should count as exposure, in INPUT and in chains alike, is #753.
- Minor REAL, no step-level ufw-host test: applied. `TestIP6JumpsUFWHost` runs `ufw allow 3389` with the matching `ufw6-user-input` rule through `readLinuxFirewall` and `firewallCheckStep`: check is `manual` with ufw's fix, no `ip6tables -D`, and the walk never reads the ufw6 chain. Mutation: dropping the `ufw6-` exclusion turns it red.
- Minor REAL, note text unasserted: applied. The row now asserts the note quotes the tool's stderr. Mutation: dropping `toolSaid` from the note turns it red. The error's `%v` is not asserted, because the fake's `*exec.ExitError` has no process state and prints `<nil>`; a real one prints `exit status 1`.
- Question, unmodelled jump qualifiers (`-m conntrack`, `-m mark`): the trade-off is intended and declared in the proposal (Out of scope), and inherited from `iptMatchesPort`.
- AC2 and AC5 rest on gitignored logs: the logs are local to the run's worktree (`scripts/e2e/out/` is ignored by design). The fixture lines are quoted verbatim in AC2 above, and AC5 quotes the five new PASS lines' assertions and the 112/0 count.

PR review (CodeRabbit, at 73bb484):

- Major REAL, `ufw6-*` excluded without a ufw read: applied. The walk skips a `ufw6-*` chain only where the firewall is read through ufw (`readIP6(..., ufw)` from `read.backend == backendUFW`). The proposal's Out of scope line ("ufw's v6 chains are not followed") holds for ufw hosts only; this line corrects it, and the proposal is left as reviewed. Red row first: `TestIP6Jumps/a_ufw6_chain_on_a_host_ufw_does_not_read_is_walked`.
- Minor REAL, a port jump with another qualifier was followed: applied. A port-bearing jump now passes the same unmodelled-qualifier test as a port-less one (`ip6JumpCovers`), as the proposal's Out of scope already says. Red row first: `a_port_jump_held_to_established_replies_is_not_followed`; multiport rows pin the port list.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: the walk follows the existing v4 ts-input model; the fixture capture is the repo's documented practice (capture-tsfw.sh).
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: the v6 exposure scan (#526) is documented in `docs/design/host-client-parity.md`, which this change updates, and no ADR records it; this widens what the scan reads, not a decision about the host's desired state.
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: ts-bridge-specific.

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/BUG-720-v6-jumped-chains/` -> `specs/archive/BUG-720-v6-jumped-chains/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
