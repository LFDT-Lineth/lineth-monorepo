#!/usr/bin/env sh
# Thin quickstart launcher. Docker Compose remains the engine; TypeScript owns
# Sepolia/key preflight and account setup.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="start"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"

TAIL=false
PULL=true
LINETH_VERBOSE="${LINETH_VERBOSE:-false}"

usage() {
  cat <<'EOF'
Usage: ./scripts/start.sh [--tail] [--no-pull] [--verbose]

  --tail     start the stack, then show the guided deployment/finality progress
  --no-pull  skip docker compose pull
  --verbose  show raw preparation/pull details in the default terminal output
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --tail)
      TAIL=true
      ;;
    --no-pull)
      PULL=false
      ;;
    --verbose)
      LINETH_VERBOSE=true
      export LINETH_VERBOSE
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      lineth_die "unknown argument: $1"
      ;;
  esac
  shift
done

ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
STACK="$ROOT/docs/getting-started/lineth-stack"
COMPOSE="docker compose --env-file versions.env --env-file .env --profile stack-partial-prover"
lineth_runtime_init "$STACK"
L1_MODE="$(lineth_l1_mode)"
case "$L1_MODE" in
  sepolia|local) ;;
  *) lineth_die "L1_MODE must be one of sepolia, local (got '$L1_MODE')" ;;
esac

show_compose_failure() {
  log_file="$1"
  lineth_error "Docker startup failed; raw output: $log_file"
  if [ -f "$log_file" ]; then
    tail -40 "$log_file" | lineth_indent
  fi
}

run_ts_preflight() {
  if [ ! -x "$ROOT/node_modules/.bin/ts-node" ]; then
    lineth_warn "host L1 check skipped; run pnpm install for earlier balance/gas checks"
    lineth_info "account generation still runs the same checks before runtime containers are created"
    return 0
  fi

  tmp="$(mktemp)"
  if (
    cd "$ROOT/contracts"
    export NODE_PATH="$ROOT/node_modules:$ROOT/contracts/node_modules${NODE_PATH:+:$NODE_PATH}"
    LINETH_STACK_DIR="$STACK" \
    TS_NODE_TRANSPILE_ONLY=1 \
    TS_NODE_COMPILER_OPTIONS='{"module":"CommonJS","moduleResolution":"Node"}' \
      pnpm -s exec ts-node "$STACK/scripts/internal/quickstart-preflight.ts"
  ) > "$tmp" 2>&1; then
    status=0
  else
    status=$?
  fi
  lineth_child_output < "$tmp"
  rm -f "$tmp"
  return "$status"
}

wait_for_local_l1() {
  lineth_kv "mode" "local L1"
  lineth_kv "start" "docker compose --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node"
  # shellcheck disable=SC2086
  local_l1_log="/tmp/lineth-local-l1.$$.$(date '+%Y%m%d%H%M%S').log"
  if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
    COMPOSE_PROGRESS=plain $COMPOSE --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node
  elif ! COMPOSE_PROGRESS=plain $COMPOSE --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node > "$local_l1_log" 2>&1; then
    lineth_error "local L1 Docker startup failed; raw output: $local_l1_log"
    tail -40 "$local_l1_log" | lineth_indent
    exit 1
  else
    lineth_info "raw local L1 Docker output: $local_l1_log"
  fi

  wait_for_container_health() {
    container="$1"
    label="$2"
    i=0
    while [ "$i" -lt 180 ]; do
      health="$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)"
      if [ "$health" = "healthy" ]; then
        lineth_ok "$label is healthy"
        return 0
      fi
      sleep 1
      i=$((i + 1))
    done

    lineth_error "$label did not become healthy"
    docker logs --tail 80 "$container" 2>&1 | lineth_indent || true
    exit 1
  }

  local_l1_block_number() {
    block_response="$(lineth_rpc_json "$(lineth_l1_host_rpc_url)" eth_blockNumber '[]')"
    block_hex="$(printf '%s\n' "$block_response" | lineth_json_stdin_string_field result)"
    [ -n "$block_hex" ] || return 1
    lineth_hex_to_dec_small "$block_hex"
  }

  wait_for_l1_block_advance() {
    baseline=""
    i=0
    while [ "$i" -lt 120 ]; do
      baseline="$(local_l1_block_number || true)"
      if lineth_is_uint "$baseline"; then
        break
      fi
      sleep 1
      i=$((i + 1))
    done
    if ! lineth_is_uint "$baseline"; then
      lineth_error "local L1 RPC did not return eth_blockNumber at $(lineth_l1_host_rpc_url)"
      docker logs --tail 80 l1-el-node 2>&1 | lineth_indent || true
      exit 1
    fi

    i=0
    while [ "$i" -lt 180 ]; do
      current="$(local_l1_block_number || true)"
      if lineth_is_uint "$current" && [ "$current" -gt "$baseline" ]; then
        lineth_ok "local L1 block production advanced from $baseline to $current"
        return 0
      fi
      sleep 1
      i=$((i + 1))
    done

    lineth_error "local L1 eth_blockNumber did not advance from $baseline"
    docker logs --tail 80 l1-el-node 2>&1 | lineth_indent || true
    docker logs --tail 80 l1-cl-node 2>&1 | lineth_indent || true
    exit 1
  }

  wait_for_container_health l1-el-node "local L1 execution node"
  wait_for_container_health l1-cl-node "local L1 consensus node"
  wait_for_l1_block_advance

  lineth_kv "rpc" "$(lineth_l1_host_rpc_url)"
}

cd "$STACK"

if [ "$L1_MODE" = "local" ]; then
  lineth_banner "start · local services + local L1 finality"
else
  lineth_banner "start · local services + Sepolia finality"
fi

lineth_section "Check ports"
lineth_run_stream env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$SCRIPT_DIR/check-ports.sh"

lineth_section "Check L1 network"
if [ "$L1_MODE" = "local" ]; then
  wait_for_local_l1
else
  lineth_kv "mode" "Sepolia"
fi

run_ts_preflight

lineth_section "Generate accounts and configs"
bootstrap_log="/tmp/lineth-bootstrap.$$.$(date '+%Y%m%d%H%M%S').log"
if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
  lineth_run_stream env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$SCRIPT_DIR/bootstrap-artifacts.sh"
else
  if env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$SCRIPT_DIR/bootstrap-artifacts.sh" > "$bootstrap_log" 2>&1; then
    lineth_kv "accounts" "runtime wallets and encrypted keystores ready"
    lineth_kv "web3signer" "runtime key files ready"
    lineth_kv "configs" "Postman Web3Signer config ready"
    lineth_info "raw generation output: $bootstrap_log"
  else
    lineth_error "account/config generation failed; raw output: $bootstrap_log"
    tail -80 "$bootstrap_log" | lineth_indent
    exit 1
  fi
fi

if [ "$PULL" = "true" ]; then
  lineth_section "Pull Docker images"
  lineth_info "checking/pulling Docker images; this can take a while on a slow connection"
  pull_log="/tmp/lineth-docker-pull.$$.$(date '+%Y%m%d%H%M%S').log"
  # shellcheck disable=SC2086
  if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
    set +e
    COMPOSE_PROGRESS=plain $COMPOSE pull
    pull_status=$?
    set -e
  elif COMPOSE_PROGRESS=plain $COMPOSE pull > "$pull_log" 2>&1; then
    pull_status=0
  else
    pull_status=$?
  fi
  if [ "$pull_status" -ne 0 ]; then
    lineth_error "Docker image pull failed. This is usually a Docker Hub/network issue."
    [ "${LINETH_VERBOSE:-false}" = "true" ] || tail -60 "$pull_log" | lineth_indent
    lineth_info "retry the same command, or use ./scripts/start.sh --tail --no-pull if the images are already local"
    exit 1
  fi
  if [ "${LINETH_VERBOSE:-false}" != "true" ]; then
    lineth_ok "Docker images are available"
    lineth_info "raw Docker pull output: $pull_log"
  fi
fi

lineth_section "Start services"
if [ "$TAIL" = "true" ]; then
  compose_log="/tmp/lineth-compose-up.$$.$(date '+%Y%m%d%H%M%S').log"
  lineth_kv "docker" "docker compose up -d"
  lineth_info "raw Docker service output: $compose_log"
  lineth_info "deployment and finality progress will stream below"
  # shellcheck disable=SC2086
  COMPOSE_PROGRESS=plain $COMPOSE up -d > "$compose_log" 2>&1 &
  compose_pid=$!

  interrupt_message="Stack startup is still running. Reattach with ./scripts/watch.sh or inspect raw logs with docker compose logs."
  trap 'lineth_info "$interrupt_message"; exit 130' INT
  i=0
  while [ "$i" -lt 60 ]; do
    if [ -n "$($COMPOSE ps -q l2-genesis-init 2>/dev/null || true)" ] \
      || [ -n "$($COMPOSE ps -q deploy-contracts 2>/dev/null || true)" ]; then
      break
    fi
    if ! kill -0 "$compose_pid" 2>/dev/null; then
      break
    fi
    sleep 1
    i=$((i + 1))
  done

  if ! kill -0 "$compose_pid" 2>/dev/null; then
    set +e
    wait "$compose_pid"
    status=$?
    set -e
    if [ "$status" -eq 0 ]; then
      if ! LINETH_SECTION_INDEX="$LINETH_SECTION_INDEX" LINETH_SUPPRESS_BANNER=1 "$SCRIPT_DIR/watch.sh"; then
        exit 1
      fi
      exit 0
    fi
    show_compose_failure "$compose_log"
    exit "$status"
  fi

  if ! LINETH_SECTION_INDEX="$LINETH_SECTION_INDEX" LINETH_SUPPRESS_BANNER=1 "$SCRIPT_DIR/watch.sh"; then
    wait "$compose_pid" >/dev/null 2>&1 || true
    exit 1
  fi
  set +e
  wait "$compose_pid"
  status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    show_compose_failure "$compose_log"
    exit "$status"
  fi
else
  lineth_kv "docker" "docker compose up -d"
  # shellcheck disable=SC2086
  COMPOSE_PROGRESS=plain $COMPOSE up -d
  lineth_info "stack start requested; use ./scripts/watch.sh for deploy/proof/finality progress"
fi
