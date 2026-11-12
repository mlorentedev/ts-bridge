---
tags: [spec, verification]
created: "2026-11-11"
id: "BUG-691-permissive-input-policy"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-691-permissive-input-policy

## Evidence

- [x] AC1 (ACCEPT, rule present → manual naming the policy and the DROP fix, `firewall_ok` false) -> commit `81f6a80`, test `TestPermissiveInputPolicy/ACCEPT,_rule_present`; e2e "permissive policy: check says manual, naming the policy" and "check does not report firewall_ok".
- [x] AC2 (ACCEPT, rule absent → setup appends the scoped rule, reports manual, runs no `iptables -P`) -> `TestPermissiveInputPolicy/ACCEPT,_rule_absent...` asserts the append and that no call contains ` -P `; e2e "setup writes the scoped rule, reports manual, leaves the policy".
- [x] AC3 (DROP, ufw, unverified/failed keep their outcomes) -> DROP rows and the unfollowable-jump row of `TestPermissiveInputPolicy`; `TestFirewallStepsTailscaledModes` and `TestIP6Exposure` replayed green on DROP fixtures. The ufw backend never reads `iptables -S`, so `inputOpen` is never set there (structural, not a table row).
- [x] AC4 (e2e) -> `make e2e-linux` at `c2f25a5`: 94 pass, 0 fail, including the five new permissive/closed-policy cases and every existing iptables case under DROP.

## Test status

- `go test -race ./...` at `c2f25a5`: all packages ok.
- `golangci-lint run`: 0 issues on darwin and with `GOOS=linux`.
- `gosec`: not installed locally; the CI `security` job runs it.
- Mutation check: dropping `withPolicy` from setup, from check, or the `inputOpen` term from `linuxFirewallOK` each fails `TestPermissiveInputPolicy`.
- No regressions: the shared step-test fixtures moved from `-P INPUT ACCEPT` to `-P INPUT DROP` (28 cases assumed a closed policy implicitly); their expected outcomes are unchanged.

## Decisions made during implementation

- The outcome (`manual`, never `-P`) was the owner's decision on #691; setup still writes the scoped rule so the host is correct the moment the operator closes the policy.
- The e2e guest closes its policy with loopback, ESTABLISHED/RELATED and port 22 first, so the harness keeps its SSH channel. `iptablesDecide` does not count those rules for 3389 (conntrack is portless and unmodeled; port 22 is a different port).
- The IPv6 policy is out of scope: #733.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the fix applies a decision taken on #691; no new failure mode was learned.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: it extends ADR-017's `manual` model without changing it.
- [x] New pattern candidate for `00_meta/patterns/`? no: specific to this firewall read.

## Spec review round 1 (nan/deepseek-v4-flash, PASS-WITH-GAPS at afe45f6)

| Finding | Disposition |
|---|---|
| F1 Minor REAL: a multi-line `--comment` can forge `-P INPUT ACCEPT` on a DROP host | Applied. Red row `DROP, a multi-line comment cannot forge the policy line` in `32df2cc`; `iptPolicyAccept` reads the first line only in `955cefb` |
| F2 Major THEORETICAL: ufw with an incoming default of allow leaves the same blindness | Ticketed #735 (the proposal's out-of-scope assumption is not guaranteed) |
| F3 Minor REAL: ADR-017 Amendment B still says `check` does not report the policy | Applied in `f1170c0` |
| F4 Minor REAL: no runbook mentions the Linux firewall outcome | Applied in `f1170c0`: a troubleshooting row in `docs/runbooks/guide-rdp-host-setup.md` |
| F5 Minor REAL: `host check` exit 2 on default-ACCEPT hosts is not stated where operators meet it | Applied in `f1170c0`, in the same runbook row. No `cmd/cli` test: `checkConvergence` already maps any non-ok step to exit 2, and its own tests cover that |
| F6 Minor THEORETICAL: no ACCEPT row combines with the legacy/foreign paths | Applied for the legacy path: row `ACCEPT, setup's legacy rule...` in `32df2cc`. A foreign allow is already `manual` and is not promoted, so it adds no new path |
| AC4: the e2e log is not committed | Counts are recorded under Evidence. The log lives in the untracked `scripts/e2e/out/` |
