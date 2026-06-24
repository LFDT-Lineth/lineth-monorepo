#!/usr/bin/env sh
# Check host port collisions before starting the quickstart stack.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="check-ports"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"

if [ "${LINETH_EMBEDDED:-false}" = "true" ]; then
  section() { :; }
else
  section() { lineth_section "$*"; }
fi

if [ "${LINETH_SKIP_BANNER:-false}" != "true" ]; then
  lineth_banner "check · local ports"
fi

failures=0
seen_ports=" "

env_value() {
  key="$1"
  [ -f .env ] || return 1
  sed -nE "s/^${key}=([^#[:space:]].*)$/\1/p" .env | tail -1
}

with_default() {
  value="$1"
  fallback="$2"
  if [ -n "$value" ]; then
    printf '%s' "$value"
  else
    printf '%s' "$fallback"
  fi
}

port_owner() {
  port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN 2>/dev/null | awk 'NR == 2 { print $1 " pid=" $2; exit }'
    return 0
  fi
  if command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
    printf 'listener detected'
  fi
}

port_row() {
  target="$1"
  status="$2"
  color="$3"
  detail="$4"

  if [ -n "$detail" ]; then
    printf '  %-42s %s%-5s%s %s\n' "$target" "$color" "$status" "$LINETH_RESET" "$detail"
  else
    printf '  %-42s %s%-5s%s\n' "$target" "$color" "$status" "$LINETH_RESET"
  fi
}

check_port() {
  name="$1"
  env_name="$2"
  default_port="$3"
  configured="$(env_value "$env_name" || true)"
  port="$(with_default "$configured" "$default_port")"

  case "$port" in
    ''|*[!0-9]*)
      port_row "$env_name=$port" "bad" "$LINETH_RED" "$name must be a decimal port"
      failures=$((failures + 1))
      return
      ;;
  esac

  case "$seen_ports" in
    *" $port "*)
      port_row "$env_name=$port" "dup" "$LINETH_RED" "$name duplicates another quickstart host port"
      failures=$((failures + 1))
      return
      ;;
  esac
  seen_ports="${seen_ports}${port} "

  owner="$(port_owner "$port" || true)"
  if [ -n "$owner" ]; then
    port_row "$env_name=$port" "busy" "$LINETH_RED" "$name -> $owner"
    failures=$((failures + 1))
  else
    port_row "$env_name=$port" "free" "$LINETH_GREEN" "$name"
  fi
}

section "checking expected host ports"
l1_mode="$(with_default "${L1_MODE:-$(env_value L1_MODE || true)}" sepolia)"
case "$l1_mode" in
  sepolia|local) ;;
  *)
    port_row "L1_MODE=$l1_mode" "bad" "$LINETH_RED" "L1_MODE must be one of sepolia, local"
    failures=$((failures + 1))
    ;;
esac

if [ "$l1_mode" = "local" ]; then
  check_port "Local L1 RPC HTTP" HOST_PORT_L1_RPC 8445
  check_port "Local L1 RPC WebSocket" HOST_PORT_L1_WS 8446
  check_port "Local L1 engine API" HOST_PORT_L1_ENGINE 8551
  check_port "Local L1 execution P2P" HOST_PORT_L1_P2P 30303
  check_port "Local L1 discovery" HOST_PORT_L1_DISCOVERY 9001
  check_port "Local L1 consensus P2P" HOST_PORT_L1_CL_P2P 9002
  check_port "Local L1 consensus metrics" HOST_PORT_L1_CL_METRICS 8008
  check_port "Local L1 consensus REST" HOST_PORT_L1_CL_REST 4003
fi

check_port "Sequencer RPC" HOST_PORT_L2_SEQUENCER_RPC 8645
check_port "Maru" HOST_PORT_MARU 8080
check_port "L2 RPC HTTP" HOST_PORT_L2_RPC 8745
check_port "L2 RPC WebSocket" HOST_PORT_L2_WS 8746
check_port "L2 engine API" HOST_PORT_L2_ENGINE 8748
check_port "Shomei" HOST_PORT_SHOMEI 8998
check_port "Web3signer" HOST_PORT_WEB3SIGNER 9000
check_port "Coordinator Postgres" HOST_PORT_COORDINATOR_PG 5432
check_port "Postman Postgres" HOST_PORT_POSTMAN_PG 5433
check_port "Blockscout Postgres" HOST_PORT_BLOCKSCOUT_L2_PG 5435
check_port "Coordinator observability" HOST_PORT_COORDINATOR 9545
check_port "Postman" HOST_PORT_POSTMAN 9090
check_port "Blockscout API" HOST_PORT_L2_BLOCKSCOUT 4000
check_port "Blockscout frontend" HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001

if [ "$failures" -gt 0 ]; then
  section "found $failures port issue(s)"
  lineth_info "Stop the conflicting service or override the matching HOST_PORT_* in .env."
  exit 1
fi

if [ "${LINETH_EMBEDDED:-false}" = "true" ]; then
  port_row "all checked quickstart host ports" "free" "$LINETH_GREEN" ""
else
  section "all checked quickstart host ports are free"
fi
