#!/bin/sh
# Polls host serve's /health/ready inside the Linux mesh guest (HOST-011). Stdin carries the
# wanted answer and a budget in seconds: "503 40" waits for a 503 naming
# control_plane_unreachable, "200 30" for a 200. Exits 0 once seen inside the budget, 1 when the
# budget runs out. Polls every second with a 1 s request timeout, so the probe adds at most ~2 s to
# what it measures, and an answer that arrives after the deadline does not count. HEALTH_URL
# overrides the endpoint for scripts/tests/test-e2e-probe.sh.
set -u
read -r want budget
url="${HEALTH_URL:-http://127.0.0.1:9090/health/ready}"
export url
start=$(date +%s)
while [ $(( $(date +%s) - start )) -le "$budget" ]; do
  got=$(python3 -c 'import os, urllib.request, urllib.error
try:
    r = urllib.request.urlopen(os.environ["url"], timeout=1)
    print(r.status, r.read().decode().strip())
except urllib.error.HTTPError as e:
    print(e.code, e.read().decode().strip())
except Exception as e:
    print("ERR", e)')
  took=$(( $(date +%s) - start ))
  case "$want:$got" in
    503:503*control_plane_unreachable* | 200:200*)
      if [ "$took" -le "$budget" ]; then
        echo "health: $got after ${took}s"
        exit 0
      fi
      ;;
  esac
  sleep 1
done
echo "health: still '$got' after ${budget}s, wanted $want"
exit 1
