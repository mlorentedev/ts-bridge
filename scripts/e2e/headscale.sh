#!/usr/bin/env bash
# A local Headscale for the mesh scenarios (QA-607, #607). Sourced by run.sh, which defines
# HERE; nothing runs on source. See specs/QA-607-headscale-e2e.
#
# The control plane is a container labelled tsb-e2e=headscale, published on every interface
# of the host, with its embedded DERP on the same port and no TLS. Everything it writes lives
# under $HS_DIR, so hs_down leaves nothing behind.

HS_DIR="${HS_DIR:-${HERE:?}/out/headscale}"
HS_NAME=tsb-e2e-headscale
HS_LABEL=tsb-e2e=headscale
HS_IMAGE="${HS_IMAGE:-headscale/headscale:v0.29.7@sha256:11f81665e842809db9fe3006a38c56a66e968baa5abd4bac498e86445572b970}"
HS_USER=e2e
HS_PORT=8080

# hs_host_ip prints the IPv4 address of the default-route interface: the one address the macOS
# client and both guests can use, since the DERP map carries a single host name.
hs_host_ip() {
  local ifc ip
  ifc="$(route -n get default 2>/dev/null | awk '$1 == "interface:" {print $2}')"
  [ -n "$ifc" ] || { echo "e2e: no default route: connect the host to a network" >&2; return 1; }
  ip="$(ipconfig getifaddr "$ifc" 2>/dev/null)" || ip=""
  [ -n "$ip" ] || { echo "e2e: no IPv4 address on $ifc, the default-route interface" >&2; return 1; }
  echo "$ip"
}

# hs_render_config IP prints the Headscale config advertising http://IP:8080.
hs_render_config() {
  sed "s/@HOST_IP@/$1/" "${HERE:?}/headscale/config.yaml.tmpl"
}

# hs_premise_ok [MODULE_DIR] checks that the pinned tailscale.com still registers the knob
# that lets DERP run over plain HTTP. If an upgrade drops it, say so rather than let every
# case time out (E3).
hs_premise_ok() {
  local dir="${1:-}" src
  [ -n "$dir" ] || dir="$(cd "${ROOT:?}" && go list -m -f '{{.Dir}}' tailscale.com)" || return 1
  src="$dir/derp/derphttp/derphttp_client.go"
  grep -q 'RegisterBool("TS_DEBUG_USE_DERP_HTTP")' "$src" 2>/dev/null && return 0
  echo "e2e: tailscale.com no longer registers TS_DEBUG_USE_DERP_HTTP in $src:" \
    "the mesh scenarios need DERP over plain HTTP; the fallback is TLS with a local CA" >&2
  return 1
}

hs_cli() { docker exec "$HS_NAME" /ko-app/headscale "$@"; }

# hs_up starts the control plane and creates its one user. A second call changes nothing.
hs_up() {
  local ip
  ip="$(hs_host_ip)" || return 1
  [ -z "$(docker ps -q --filter "label=$HS_LABEL")" ] || return 0
  docker rm -f "$HS_NAME" >/dev/null 2>&1 || true # a stopped leftover keeps the name
  mkdir -p "$HS_DIR/conf" "$HS_DIR/data"
  hs_render_config "$ip" >"$HS_DIR/conf/config.yaml"
  cp "${HERE:?}/headscale/policy.hujson" "$HS_DIR/conf/policy.hujson"
  docker run -d --name "$HS_NAME" --label "$HS_LABEL" -p "$HS_PORT:8080" -p 3478:3478/udp \
    -v "$HS_DIR/conf:/etc/headscale:ro" -v "$HS_DIR/data:/var/lib/headscale" "$HS_IMAGE" serve >/dev/null
  for _ in $(seq 1 30); do
    hs_cli health >/dev/null 2>&1 && { hs_create_user; return; }
    sleep 1
  done
  docker logs --tail 20 "$HS_NAME" >&2 || true
  echo "e2e: headscale did not become healthy in 30s" >&2
  return 1
}

# hs_down removes every Headscale container and everything it wrote. It is safe to repeat.
hs_down() {
  local ids
  if command -v docker >/dev/null; then
    ids="$(docker ps -aq --filter "label=$HS_LABEL")" || { echo "e2e: could not list headscale containers" >&2; return 1; }
    # shellcheck disable=SC2086 # one word per container id
    [ -z "$ids" ] || docker rm -f $ids >/dev/null || { echo "e2e: could not remove the headscale container" >&2; return 1; }
  fi
  rm -rf "$HS_DIR"
}

hs_create_user() { hs_cli users create "$HS_USER" >/dev/null; }

# hs_user_id prints the numeric id preauthkeys want; users list reports it under "id".
hs_user_id() {
  hs_cli users list -n "$HS_USER" -o json | python3 -c 'import json,sys; print(json.load(sys.stdin)[0]["id"])'
}

# hs_mint_key [TAG] prints a single-use pre-auth key valid for one hour, tagged TAG if given.
# It goes to the caller's stdout only: never argv, never a log.
hs_mint_key() {
  local args=(--expiration 1h -u "$(hs_user_id)" -o json)
  [ -z "${1:-}" ] || args+=(--tags "$1")
  hs_cli preauthkeys create "${args[@]}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["key"])'
}

hs_mint_api_key() { hs_cli apikeys create --expiration 24h; }
hs_nodes_json() { hs_cli nodes list -o json; }

# hs_guest_reaches OS SESSION fetches the health endpoint from inside the guest: the guests
# sit behind NAT and the DERP map names the host's LAN address, so this is the hairpin (E2).
hs_guest_reaches() {
  local os="$1" session="$2" url
  url="http://$(hs_host_ip):$HS_PORT/health"
  if [ "$os" = windows ]; then
    dotf vm windows exec --session "$session" -- powershell -NoProfile -Command \
      "(Invoke-WebRequest -UseBasicParsing -TimeoutSec 5 $url).StatusCode" </dev/null >/dev/null 2>&1
  else
    dotf vm "$os" exec --session "$session" -- curl -fsS --max-time 5 "$url" </dev/null >/dev/null 2>&1
  fi || { echo "guest $os cannot reach the control plane at $url" >&2; return 1; }
}
