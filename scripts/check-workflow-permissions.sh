#!/usr/bin/env bash
# Guard: CI workflows carry no default credential and no ambient write scope.
#
# Two rules, both measured rather than inherited:
#   1. every workflow declares a top-level `permissions:` key. Without one GitHub hands the
#      job the repository default -- read-write on every scope -- so a runner that is only
#      meant to compile tests holds a token that can rewrite the repo. Declaring the key is
#      the difference between "we chose this" and "nobody chose".
#   2. every `actions/checkout` sets `persist-credentials: false`. The default leaves the
#      run token in .git/config inside the workspace; any step after checkout -- a linter,
#      a build, a test that shells out -- can read it. Nothing in this repo pushes from a
#      workflow, so the credential is paid for and never spent.
#
# A checkout may opt out with an inline `# persist-credentials-ok: <reason>` trailer on the
# `uses:` line -- read from the raw line, since it lives in the comment the matcher below strips.
# The exemption names itself and is echoed by the guard, so a widening shows up
# in the CI log instead of arriving as a deleted check. `write-all` is refused outright: a
# declared `permissions: write-all` is the same ambient grant as no declaration at all, with a
# comment that claims otherwise.
#
# Portable: bash and zsh, GNU and BSD find (no -printf), no GNU-only awk, no regex built from
# a filename. NUL-separated file list so spaces cannot split it.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WFDIR="${1:-$ROOT/.github/workflows}"
[ -d "$WFDIR" ] || { echo "check-workflow-permissions: $WFDIR not present, nothing to check"; exit 0; }

# awk emits findings, then one machine line: "SUMMARY <checkouts> <ok> <bad> <optout> <writeall> <notop>"
scan() {
  awk '
    function indent(s) { if (match(s, /[^ ]/)) return RSTART - 1; return 999 }
    # Column of the key on a line, past any list dash: the parent a block scalar is nested under.
    function keycol(s) { if (match(s, /[^ \t-]/)) return RSTART - 1; return 999 }
    function report_wa() { wa++; printf "  write-all  %-28s line %-4d ambient write scope declared\n", FILENAME, FNR }
    function begin_step(col) { instep = 1; sindent = col; ck = 0; persist = ""; optout = "" }
    # A step is judged once it has ended, from everything it contained: the checkout and its
    # persist-credentials may appear in either order.
    function flush() {
      if (!instep) return
      instep = 0
      if (!ck) return
      checkouts++
      if (optout != "") { opted++; printf "  opt-out    %-28s line %-4d %s\n", FILENAME, plineno, optout; return }
      if (persist == "")      { bad++; printf "  no-except  %-28s line %-4d checkout persists credentials\n", FILENAME, plineno }
      # YAML booleans are case-insensitive: False and FALSE are the same input as false.
      else if (tolower(persist) != "false") { bad++; printf "  enabled    %-28s line %-4d persist-credentials: %s\n", FILENAME, plineno, persist }
      else ok++
    }
    # A permissions value need not share the key line. `permissions: >-` (a block scalar) and a
    # bare `permissions:` followed by an indented plain scalar both evaluate to the string on the
    # next line, so `write-all` written that way used to pass with OK (CI-322 adversarial review).
    # The value must BE write-all -- optionally quoted, anchored or tagged -- not merely mention
    # it: `MSG: "never grant contents: write-all"` is a string, and matching it was a false red.
    function scan_permissions(code, n) {
      if (code ~ R_WA) report_wa()
      if (permblk >= 0) {
        if (n <= permblk) permblk = -1
        else if (code ~ R_WAV) report_wa()
      }
      if (code ~ R_PKEY) permblk = keycol(code)
    }
    # Steps are delimited by their list dash, never by the `uses:` line. The first version took
    # the step indent from `uses:`, which is the dash column only when `uses:` comes first; with
    # `- name:` first, `with:` sat at the same indent, the step closed early and a correct
    # checkout was reported as persisting credentials (CI-322 adversarial review, Blocker).
    function scan_step(line, code, n) {
      if (instep && n <= sindent) flush()
      if (!instep && code ~ /^[ \t]*-([ \t]|$)/) begin_step(n)
      # A checkout outside any list item is not valid Actions YAML, but it is still counted:
      # a guard that reports 0 checkouts for a file that has one is announcing a blind spot.
      if (!instep && match(code, R_CK)) begin_step(n - 1)
      if (!instep) return
      if (match(code, R_CK)) {
        ck = 1; plineno = FNR
        if (match(line, /#.*persist-credentials-ok:[ \t]*[^ \t]+/)) {
          optout = substr(line, RSTART, RLENGTH)
          sub(/^.*persist-credentials-ok:[ \t]*/, "", optout)
        }
      }
      if (match(code, R_PC)) {
        persist = substr(code, RSTART, RLENGTH)
        sub("persist-credentials:[ \\t]*", "", persist)
        gsub(("^" QC "+|" QC "+$"), "", persist)        # `"false"` and false are the same input
      }
    }
    BEGIN {
      instep = 0; permblk = -1; blk = -1
      # A YAML scalar may be quoted, and GitHub evaluates it the same way. A matcher that only
      # knows the unquoted spelling is not a rule about permissions, it is a rule about the
      # handful of spellings this file happens to recognise -- `permissions: "write-all"` and
      # `uses: "actions/checkout@<sha>"` both walked straight through the first version (found by
      # CodeRabbit on #336, reproduced before fixed). One shared class, used by every value match.
      QC = "[\"\047]"                    # a quote character, single or double
      QO = QC "?"                        # optional opening quote
      PROPS = "([&!][^ \t]*[ \t]+)?([&!][^ \t]*[ \t]+)?"   # an anchor and/or a tag before a value
      WAVAL = PROPS QO "write-all" QO "[ \t]*$"
      R_WA   = "^[ \t]*(-[ \t]+)?[A-Za-z0-9_-]+:[ \t]*" WAVAL   # key: write-all, the whole value
      R_WAV  = "^[ \t]*" WAVAL                                   # write-all on its own line
      R_PKEY = "(^|[ \t])permissions:[ \t]*" PROPS "([>|][-+0-9]*)?[ \t]*$"
      R_BLK  = ":[ \t]*" PROPS "[>|][-+0-9]*[ \t]*$"             # any key opening a block scalar
      R_CK   = "uses:[ \t]*" QO "actions/checkout@"
      # `,` and `}` end a value in flow style: `{persist-credentials: false}}` is false.
      R_PC   = "persist-credentials:[ \t]*" QO "[^ \t#\",}\047]+"
    }
    {
      line = $0
      sub(/\r$/, "", line)               # CRLF: `false\r` is still false
      if (FNR == 1) { top = 0 }
      # A YAML comment is not structure. It grants nothing, checks out nothing, and must not be
      # scanned as if it did: a commented-out checkout step used to be counted as a real one, so
      # the guard reported `no-except` on a repository that was entirely clean -- a false red on
      # the job that gates every PR, which is precisely how a useful check gets deleted.
      if (line ~ /^[ \t]*#/) next
      if (line ~ /^[ \t]*$/) next                       # blanks never close a step
      n = indent(line)
      # The body of a block scalar (`run: |`, a multi-line `with:` value) is text, not structure:
      # a `- uses: actions/checkout@` line inside a script is not a step. Skipped like a comment.
      if (blk >= 0) { if (n > blk) next; blk = -1 }
      code = line; cpos = index(code, " #")             # trailing comment: strip for matching,
      if (cpos > 0) code = substr(code, 1, cpos - 1)    # keep it below for the opt-out trailer
      if (code ~ /^permissions:/) top = 1
      scan_permissions(code, n)
      scan_step(line, code, n)
      if (code ~ R_BLK && code !~ R_PKEY) blk = keycol(code)
    }
    END {
      flush()
      if (top == 0) { notop++; printf "  no-perms   %-28s line 1    no top-level permissions: key\n", FILENAME }
      printf "SUMMARY %d %d %d %d %d %d\n", checkouts + 0, ok + 0, bad + 0, opted + 0, wa + 0, notop + 0
    }
  ' "$1"
}

rc=0; files=0; tot_checkouts=0; tot_ok=0; tot_opt=0; tot_bad=0
while IFS= read -r -d '' f; do
  files=$((files + 1))
  out="$(scan "$f")"
  sum="$(printf '%s\n' "$out" | grep '^SUMMARY ' | tail -1)"
  # Validate by shape, then split with `read` -- NOT `set -- $sum`: zsh does not word-split an
  # unquoted expansion, so the same script that works in bash hands `set` one argument and the
  # guard reports "cannot parse" on a clean repo. A guard that fails differently per shell is a
  # guard that gets deleted, so both shells are the contract here.
  case "$sum" in
    'SUMMARY '*[0-9]' '*[0-9]' '*[0-9]' '*[0-9]' '*[0-9]' '*[0-9]) ;;
    *) echo "check-workflow-permissions: cannot parse guard output for $f (got: ${sum:-<none>})" >&2; exit 2 ;;
  esac
  IFS=' ' read -r _lbl s_checkouts s_ok s_bad s_opt s_wa s_notop <<EOF
$sum
EOF
  tot_checkouts=$((tot_checkouts + s_checkouts)); tot_ok=$((tot_ok + s_ok)); tot_bad=$((tot_bad + s_bad)); tot_opt=$((tot_opt + s_opt))
  # An opt-out is a disclosure, not a finding: it prints whether or not the run is otherwise
  # clean, so the count of persisted credentials can only ever move in front of a reader.
  printf '%s\n' "$out" | grep '^  opt-out' || true
  if [ "$s_bad" -gt 0 ] || [ "$s_wa" -gt 0 ] || [ "$s_notop" -gt 0 ]; then
    printf '%s\n' "$out" | grep -v '^SUMMARY ' | grep -v '^  opt-out' || true
    rc=1
  fi
done < <(find "$WFDIR" -maxdepth 1 -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)

if [ "$rc" -eq 0 ]; then
  echo "check-workflow-permissions: OK ($files workflows, $tot_checkouts checkouts, $tot_ok credential-less, $tot_opt opted out with a reason)"
  if [ "$tot_opt" -gt 0 ]; then echo "check-workflow-permissions: NOTE $tot_opt checkout(s) persist credentials deliberately -- the reasons are above"; fi
else
  echo "check-workflow-permissions: findings above -- declare a top-level permissions: key and set persist-credentials: false (or opt out by naming a reason)"
fi
exit "$rc"
