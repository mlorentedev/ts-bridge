---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-736-negated-portless-exposure"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-736-negated-portless-exposure

## Evidence

- [x] AC1 (v4 negated, `-p tcp -j ACCEPT`, `-j ACCEPT` → `manual`, rule named, `firewall_ok` false, SSH caution in the fix, nothing deleted) -> commits `18b6154` (red: 9 failures on the old code), `8ece357`; tests `TestExposureShapes`, `TestReadIptablesUnscoped/a_port-less_accept_is_foreign`, `.../an_accept_for_every_port_but_SSH_is_foreign`, `TestSetupForLinux/iptables,_an_accept_for_every_port:...(#736)` (appends the scoped rule, deletes nothing), `TestCheckForLinux/iptables,_an_accept_for_every_port_but_SSH:_manual_(#736)` (`FirewallOK` false).
- [x] AC2 (v6 same shapes) -> `TestExposureShapes` (both reads per row), `TestIP6Exposure/a_port-less_v6_accept`, `.../a_v6_accept_for_every_port_but_SSH` (fix names the `ip6tables -D` and the SSH caution).
- [x] AC3 (not reported: `! --dport 3389`, `! --dports 3000:4000`, mixed negated dport + positive sport, `--sport` only, DROP, UDP, `-i lo`, `-i eth0`, `-i tailscale0`, `-s` prefix, conntrack-only, a rule outside INPUT) -> the `false` rows of `TestExposureShapes`; every pre-existing step row unchanged.
- [x] AC4 (e2e) -> `make e2e-linux` at `f8ac37b`: 102 pass, 0 fail, including the eight new #736 cases (v4 `! --dport 22`, v6 `-p tcp -j ACCEPT`: manual, not `firewall_ok`, left in place, ok once removed).

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run`: 0 issues on darwin and with `GOOS=linux`.
- `gosec`: not installed locally; the CI `security` job runs it.
- No regressions: the two rows that pinned the old "operator's posture" reading were changed on purpose (they are the bug).

## Decisions made during implementation

- `iptablesDecide` is unchanged: it already counts a port-less ACCEPT as admitting the port; only the exposure reads were blind.
- v4 scope: `iptHasScope` treats any `!` as a scope, which hid `! --dport 22`. The exposure read uses `iptScopedAllow`, where a `!` before a port flag negates the port, not the scope. The deny path keeps `iptHasScope`.
- Source ports: clients connect from ephemeral ports, so beside a negated destination port a source-port match does not narrow (`! --dport 22 --sport 1024:65535` is reported, review F2). With no destination match, only an all-negated source match (`! --sport 22`) is reported; `--sport 22` alone is not.
- A portless unscoped allow no longer counts as the scoped rule, so `setup` appends `-i tailscale0` beside it (review F4; proposal and ADR-017 say so).
- Removing a rule with no destination port can close SSH, so both fixes (v4 `foreignFix`, v6 `withIP6`) name the loopback, replies and SSH rules to add first; the list is shared with #691's policy fix as `keepOpenFirst`.
- Residual: a port-less allow under a negated interface (`! -i tailscale0 -j ACCEPT`) is still not reported, because `iptPortlessCovers` treats any non-loopback negation as an unmodelled narrowing (#743).

## Spec review round 1 (nan/deepseek-v4-flash, FAIL at d167344)

- F1 (Major, REAL, ufw lines given the iptables SSH caution): applied. `iptAdmitsOthers` takes only `-A` lines; `TestWideAllowFix` pins the whole fix for `ufw allow 3389`, `ufw allow 22,3389/tcp`, a port-naming iptables rule, a port-less one, and both v6 shapes. Red at `11c2906` before the fix.
- F2 (Major, REAL, `! --dport 22 --sport 1024:65535` missed): applied, see Decisions. The `TestExposureShapes` row now expects it reported; `--dport 22 ! --sport 22` added as not reported.
- F3 (Major, REAL, empty `verification.md`, unticked tasks): the review read `d167344`, one commit before this file was filled at `cdf6ed0`; `tasks.md` boxes now ticked.
- F4 (Minor, REAL, the `scoped` flip undocumented): applied in `proposal.md` and ADR-017.
- F5 (Minor, REAL, substring fix assertions): applied by `TestWideAllowFix`, which compares whole strings.
- F6 (Minor, SPECULATIVE, `! --sport` over-report): declined, accepted in the proposal; an extra `manual` on a very unusual rule.
- Question (AC4 evidence): recorded above, `make e2e-linux` at `f8ac37b`, 102/0.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: lesson-055 already records the gap and is updated to say it is closed.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017's text on port-less allows is amended in place (reported `manual`, never deleted).
- [x] New pattern candidate for `00_meta/patterns/`? no: specific to this firewall read.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-736-negated-portless-exposure/` -> `specs/archive/BUG-736-negated-portless-exposure/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
