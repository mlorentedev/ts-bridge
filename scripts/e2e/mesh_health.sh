#!/bin/sh
# Polls host serve's /health/ready inside the Linux mesh guest (HOST-011). Stdin carries the
# wanted answer and a budget in seconds: "503 40" waits for a 503 naming
# control_plane_unreachable, "200 30" for a 200. Exits 0 once seen, 1 when the budget runs out.
set -u
read -r want budget
start=$(date +%s)
while [ $(( $(date +%s) - start )) -le "$budget" ]; do
  got=$(python3 -c 'import urllib.request, urllib.error
try:
    r = urllib.request.urlopen("http://127.0.0.1:9090/health/ready", timeout=3)
    print(r.status, r.read().decode().strip())
except urllib.error.HTTPError as e:
    print(e.code, e.read().decode().strip())
except Exception as e:
    print("ERR", e)')
  case "$want:$got" in
    503:503*control_plane_unreachable* | 200:200*)
      echo "health: $got after $(( $(date +%s) - start ))s"
      exit 0
      ;;
  esac
  sleep 2
done
echo "health: still '$got' after ${budget}s, wanted $want"
exit 1
