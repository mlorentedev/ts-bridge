# Workflow YAML readers shared by the structural workflow guards under scripts/tests/.
# Sourced, never run. Text in, text out, no YAML library: the guards run on a bare runner.

# Comments are prose, never contract: drop full-line comments and trailing ` # ...`.
strip_comments() {
  sed -E -e '/^[[:space:]]*#/d' -e 's/[[:space:]]+#.*$//' "$1"
}

# Print the body of job $1, one step per record: every step begins with a line `@@STEP`.
# Lines are keyed by where they sit, not by what they say: a step key prints bare (`run: npm ci`),
# anything nested under one prints with its parent (`with.persist-credentials: false`,
# `env.run: npm ci`), and a job-level key prints as `job.<key>`. So `^run:` matches only a
# step's own command, never a look-alike parked under `env:` or `with:`.
job_steps() {
  awk -v job="$1" '
    /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
    in_jobs && /^[^[:space:]]/ { in_jobs = 0; in_job = 0 }
    in_jobs && /^  [^[:space:]][^:]*:[[:space:]]*$/ {
      key = $0; sub(/^  /, "", key); sub(/:[[:space:]]*$/, "", key)
      in_job = (key == job); step_indent = ""; next
    }
    in_job && !/^[[:space:]]*$/ {
      match($0, /^[[:space:]]*/); ind = RLENGTH; body = substr($0, ind + 1)
      if (body ~ /^- / && (step_indent == "" || ind == step_indent)) {
        step_indent = ind; print "@@STEP"
        body = substr(body, 3); sub(/^[[:space:]]+/, "", body); ind += 2
      }
      if (step_indent == "" || ind <= step_indent) { print "job." body; next }
      if (ind == step_indent + 2) { parent = body; sub(/:.*/, "", parent); print body; next }
      print parent "." body
    }
  '
}

# Exit 0 when a single step of the job has a line matching each ERE given.
step_has() {
  awk '
    BEGIN { for (i = 1; i < ARGC; i++) re[i] = ARGV[i]; n = ARGC - 1; ARGC = 1 }
    function flush(  i, j, hit) {
      if (!lines) return
      for (i = 1; i <= n; i++) {
        hit = 0
        for (j = 1; j <= lines; j++) if (line[j] ~ re[i]) { hit = 1; break }
        if (!hit) return
      }
      found = 1
    }
    /^@@STEP$/ { flush(); lines = 0; next }
    { line[++lines] = $0 }
    END { flush(); exit(found ? 0 : 1) }
  ' "$@"
}
