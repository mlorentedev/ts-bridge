#!/usr/bin/env bash
# Tests for scripts/e2e/probe.py, the protocol probe of the mesh scenarios (QA-607, #607).
#
# The probe's verdict is what a mesh case reports, so each case is a fake server that does one
# thing: confirm, send garbage, close at once, or never answer. `confirm` and `banner` are the
# controls that stop a probe which fails everything; a probe that waits for a silent server
# would end in `timeout`'s 124, not the 1 asserted here.
#
# The same holds for scripts/e2e/mesh_health.sh, the /health/ready poll of the mesh outage case
# (HOST-011): a fake answers one fixed status and body, and the poll must take only the answer it
# was asked for, inside its budget.
#
# Run: bash scripts/tests/test-e2e-probe.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROBE="$ROOT/scripts/e2e/probe.py"
WORK="$(mktemp -d)"
pids=()
trap 'kill "${pids[@]}" 2>/dev/null || true; rm -rf "$WORK"' EXIT
fails=0

cat >"$WORK/fake.py" <<'PY'
import socket, sys, time
mode = sys.argv[1]
s = socket.socket()
s.bind(("127.0.0.1", 0))
s.listen(1)
print(s.getsockname()[1], flush=True)
c, _ = s.accept()
if mode == "confirm":
    c.recv(64)
    c.sendall(bytes.fromhex("0300000b06d00000123400"))
elif mode == "garbage":
    c.sendall(b"HTTP/1.1 400 Bad Request\r\n\r\n")
elif mode == "banner":
    c.sendall(b"SSH-2.0-OpenSSH_9.6\r\n")
elif mode == "ssh1":
    c.sendall(b"SSH-1.99-Old\r\n")
elif mode == "close":
    c.close()
elif mode == "silent":
    time.sleep(10)
PY

check() { # check <case> <probe> <fake mode> <want exit>
  local name="$1" kind="$2" mode="$3" want="$4" port got=0
  python3 -I "$WORK/fake.py" "$mode" >"$WORK/port" &
  pids+=("$!")
  for _ in $(seq 1 100); do [ -s "$WORK/port" ] && break; sleep 0.05; done
  if [ ! -s "$WORK/port" ]; then echo "FAIL $name: the fake server printed no port in 5s"; fails=$((fails + 1)); return; fi
  port="$(cat "$WORK/port")"
  rm "$WORK/port"
  timeout 5 python3 -I "$PROBE" "$kind" "127.0.0.1:$port" >/dev/null 2>&1 || got=$?
  if [ "$got" = "$want" ]; then echo "ok   $name"; else echo "FAIL $name: want exit $want, got $got"; fails=$((fails + 1)); fi
}

check x224-confirm x224 confirm 0
check x224-garbage x224 garbage 1
check x224-closed-at-once x224 close 1
check x224-silent x224 silent 1
check ssh-banner ssh banner 0
check ssh-garbage ssh garbage 1
check ssh-other-version ssh ssh1 1
check ssh-silent ssh silent 1

got=0
timeout 5 python3 -I "$PROBE" x224 127.0.0.1:1 >/dev/null 2>&1 || got=$?
if [ "$got" = 1 ]; then echo "ok   x224-refused"; else echo "FAIL x224-refused: want exit 1, got $got"; fails=$((fails + 1)); fi

# mesh_health.sh: <case> <fake status> <fake body> <stdin "want budget"> <want exit>
cat >"$WORK/health.py" <<'PY'
import http.server, sys
code, body = int(sys.argv[1]), sys.argv[2].encode()
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(code); self.end_headers(); self.wfile.write(body)
    def log_message(self, *a): pass
s = http.server.HTTPServer(("127.0.0.1", 0), H)
print(s.server_port, flush=True)
s.serve_forever()
PY
health() {
  local name="$1" code="$2" body="$3" input="$4" want="$5" port got=0
  python3 -I "$WORK/health.py" "$code" "$body" >"$WORK/hport" &
  pids+=("$!")
  for _ in $(seq 1 100); do [ -s "$WORK/hport" ] && break; sleep 0.05; done
  port="$(cat "$WORK/hport")"
  rm "$WORK/hport"
  echo "$input" | HEALTH_URL="http://127.0.0.1:$port/health/ready" timeout 10 sh "$ROOT/scripts/e2e/mesh_health.sh" >/dev/null 2>&1 || got=$?
  if [ "$got" = "$want" ]; then echo "ok   $name"; else echo "FAIL $name: want exit $want, got $got"; fails=$((fails + 1)); fi
}
down='{"reason":"control_plane_unreachable","status":"not_ready"}'
health health-503-named 503 "$down" "503 3" 0
health health-503-other-reason 503 '{"reason":"bootstrap_down"}' "503 1" 1
health health-200-while-waiting-503 200 '{"status":"ok"}' "503 1" 1
health health-200 200 '{"status":"ok"}' "200 3" 0
health health-503-while-waiting-200 503 "$down" "200 1" 1

[ "$fails" -eq 0 ] || { echo "test-e2e-probe: $fails case(s) failed"; exit 1; }
echo "test-e2e-probe: all cases passed"
