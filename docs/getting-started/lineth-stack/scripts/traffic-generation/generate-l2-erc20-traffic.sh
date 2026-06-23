#!/usr/bin/env sh
# Run continuous L2 ERC20Example transfer traffic for Blockscout/prover demos.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="l2-erc20-traffic"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
log() { lineth_info "$*"; }
die() { lineth_die "$*"; }

container_exists() {
  docker inspect "$CONTAINER_NAME" >/dev/null 2>&1
}

container_running() {
  [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null || true)" = "true" ]
}

command="${1:-start}"
CONTAINER_NAME="${TRAFFIC_CONTAINER_NAME:-linea-l2-erc20-traffic}"

lineth_banner "ERC20 traffic · start/logs/stop"

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

case "$command" in
  start|stop|status|logs) ;;
  *)
    die "usage: $0 [start|stop|status|logs]"
    ;;
esac

if [ "$command" = "status" ]; then
  if container_running; then
    docker ps --filter "name=^/${CONTAINER_NAME}$" --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' | lineth_indent
  elif container_exists; then
    log "$CONTAINER_NAME exists but is not running"
  else
    log "$CONTAINER_NAME is not running"
  fi
  exit 0
fi

if [ "$command" = "logs" ]; then
  if ! container_exists; then
    die "$CONTAINER_NAME does not exist"
  fi
  docker logs -f --tail "${TAIL:-80}" "$CONTAINER_NAME" 2>&1 | lineth_clean_prefixes
  exit 0
fi

if [ "$command" = "stop" ]; then
  if container_exists; then
    docker rm -f "$CONTAINER_NAME" >/dev/null
    log "stopped $CONTAINER_NAME"
  else
    log "$CONTAINER_NAME was not running"
  fi
  exit 0
fi

[ -s "$(lineth_accounts_file runtime-keys.env)" ] || die "runtime-keys.env missing. Boot the stack first."
[ -s "$(lineth_deployments_file addresses.json)" ] || die "addresses.json missing; deploy-contracts has not completed."

case "${AMOUNT_WEI:-1}" in
  ''|*[!0-9]*) die "AMOUNT_WEI must be a positive integer" ;;
esac
[ "${AMOUNT_WEI:-1}" -gt 0 ] || die "AMOUNT_WEI must be greater than zero"

case "${INTERVAL_SECONDS:-2}" in
  ''|*[!0-9]*) die "INTERVAL_SECONDS must be a positive integer" ;;
esac
[ "${INTERVAL_SECONDS:-2}" -gt 0 ] || die "INTERVAL_SECONDS must be greater than zero"

case "${MAX_TXS:-0}" in
  ''|*[!0-9]*) die "MAX_TXS must be a non-negative integer" ;;
esac

if container_running; then
  log "$CONTAINER_NAME is already running"
  log "tail it with: $0 logs"
  log "stop it with: $0 stop"
  exit 0
fi

if container_exists; then
  docker rm "$CONTAINER_NAME" >/dev/null
fi

if [ -f versions.env ]; then
  # shellcheck disable=SC1091
  . ./versions.env
fi

FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
L2_RPC_URL="${L2_RPC_URL:-http://sequencer:8545}"
L2_GAS_PRICE_WEI="${L2_GAS_PRICE_WEI:-100000000}"
HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"
BLOCKSCOUT_BASE_URL="${BLOCKSCOUT_BASE_URL:-http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND}"
L2_TRAFFIC_ETH_MIN_BALANCE_WEI="${L2_TRAFFIC_ETH_MIN_BALANCE_WEI:-100000000000000000}"
L2_TRAFFIC_ETH_TOP_UP_WEI="${L2_TRAFFIC_ETH_TOP_UP_WEI:-1000000000000000000}"
L2_TRAFFIC_ERC20_MIN_BALANCE_WEI="${L2_TRAFFIC_ERC20_MIN_BALANCE_WEI:-100}"
L2_TRAFFIC_ERC20_TOP_UP_WEI="${L2_TRAFFIC_ERC20_TOP_UP_WEI:-10000}"

case "$L2_GAS_PRICE_WEI" in
  ''|*[!0-9]*) die "L2_GAS_PRICE_WEI must be a decimal wei value" ;;
esac

section "ensuring L2 ERC20Example"
if ! lineth_run_stream docker compose --env-file versions.env --env-file .env --profile stack-partial-prover \
  run --rm --no-deps --entrypoint bash deploy-contracts /scripts/internal/ensure-demo-erc20.sh l2; then
  exit 1
fi
ERC20="$(lineth_json_section_addr "$(lineth_deployments_file addresses.json)" l2 ERC20Example)"
echo "$ERC20" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "L2 ERC20Example missing from addresses.json"

section "ensuring disposable traffic account"
TRAFFIC_ACCOUNT_OUTPUT="$(
  docker compose --env-file versions.env --env-file .env --profile stack-partial-prover \
    run --rm --no-deps \
    -v "$LINETH_ACCOUNTS_DIR:/traffic-accounts:rw" \
    -e DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env" \
    -e TRAFFIC_ERC20_ADDRESS="$ERC20" \
    -e L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}" \
    -e L2_TRAFFIC_ETH_MIN_BALANCE_WEI="$L2_TRAFFIC_ETH_MIN_BALANCE_WEI" \
    -e L2_TRAFFIC_ETH_TOP_UP_WEI="$L2_TRAFFIC_ETH_TOP_UP_WEI" \
    -e L2_TRAFFIC_ERC20_MIN_BALANCE_WEI="$L2_TRAFFIC_ERC20_MIN_BALANCE_WEI" \
    -e L2_TRAFFIC_ERC20_TOP_UP_WEI="$L2_TRAFFIC_ERC20_TOP_UP_WEI" \
    -e L2_READ_RPC_URL="$L2_RPC_URL" \
    -e L2_SEND_RPC_URL="$L2_RPC_URL" \
    -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
    --entrypoint bash deploy-contracts /scripts/internal/traffic-account.sh ensure
)"
printf '%s\n' "$TRAFFIC_ACCOUNT_OUTPUT" | lineth_child_output
TRAFFIC_ACCOUNT_ADDRESS="$(printf '%s\n' "$TRAFFIC_ACCOUNT_OUTPUT" | sed -nE 's/^TRAFFIC_ACCOUNT_ADDRESS=(0x[a-fA-F0-9]{40})$/\1/p' | tail -1)"
echo "$TRAFFIC_ACCOUNT_ADDRESS" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "traffic account helper did not return a valid address"

section "starting continuous ERC20Example traffic"
container_id="$(docker run -d \
  --name "$CONTAINER_NAME" \
  --user 0:0 \
  --entrypoint sh \
  --network lineth-stack_linea \
  -v "$LINETH_ACCOUNTS_DIR:/accounts:ro" \
  -v "$LINETH_DEPLOYMENTS_DIR:/deployments:ro" \
  -e AMOUNT_WEI="${AMOUNT_WEI:-1}" \
  -e INTERVAL_SECONDS="${INTERVAL_SECONDS:-2}" \
  -e MAX_TXS="${MAX_TXS:-0}" \
  -e TO="${TO:-}" \
  -e ERC20="$ERC20" \
  -e L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}" \
  -e L2_RPC_URL="$L2_RPC_URL" \
  -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
  -e BLOCKSCOUT_BASE_URL="$BLOCKSCOUT_BASE_URL" \
  "$FOUNDRY_IMAGE" \
  -lc '
    set -eu

    [ -f /accounts/runtime-keys.env ] || { echo "[l2-erc20-traffic] ERROR: /accounts/runtime-keys.env missing" >&2; exit 1; }
    [ -f /deployments/addresses.json ] || { echo "[l2-erc20-traffic] ERROR: /deployments/addresses.json missing; deploy-contracts has not completed" >&2; exit 1; }

    DEMO_TRAFFIC_ENV="/accounts/demo-traffic.env"

    is_privkey() { printf "%s\n" "$1" | grep -qE "^0x[a-fA-F0-9]{64}$"; }
    is_uint() { printf "%s\n" "$1" | grep -qE "^[0-9]+$"; }

    is_uint "$L2_GAS_PRICE_WEI" || { echo "[l2-erc20-traffic] ERROR: L2_GAS_PRICE_WEI must be a decimal wei value" >&2; exit 1; }

    if [ -n "${L2_TRAFFIC_PRIVATE_KEY:-}" ]; then
      traffic_key="$L2_TRAFFIC_PRIVATE_KEY"
      echo "[l2-erc20-traffic] using L2_TRAFFIC_PRIVATE_KEY from environment"
    elif [ -f "$DEMO_TRAFFIC_ENV" ]; then
      . "$DEMO_TRAFFIC_ENV"
      traffic_key="${L2_TRAFFIC_PRIVATE_KEY:-}"
      echo "[l2-erc20-traffic] reusing disposable traffic account from $DEMO_TRAFFIC_ENV"
    else
      echo "[l2-erc20-traffic] ERROR: no disposable traffic account found after traffic-account helper" >&2
      exit 1
    fi
    is_privkey "$traffic_key" || { echo "[l2-erc20-traffic] ERROR: L2 traffic private key malformed" >&2; exit 1; }

    erc20="$ERC20"
    echo "$erc20" | grep -qE "^0x[a-fA-F0-9]{40}$" || { echo "[l2-erc20-traffic] ERROR: ERC20 invalid: $erc20" >&2; exit 1; }

    if [ -n "${TO:-}" ]; then
      recipient="$TO"
    else
      recipient="0x1000000000000000000000000000000000000001"
    fi
    echo "$recipient" | grep -qE "^0x[a-fA-F0-9]{40}$" || { echo "[l2-erc20-traffic] ERROR: recipient address invalid: $recipient" >&2; exit 1; }

    sender=$(cast wallet address --private-key "$traffic_key")

    echo "[l2-erc20-traffic] start token=$erc20 from=$sender to=$recipient amountWei=$AMOUNT_WEI interval=${INTERVAL_SECONDS}s maxTxs=$MAX_TXS gasPriceWei=$L2_GAS_PRICE_WEI"
    echo "[l2-erc20-traffic] blockscout token=$BLOCKSCOUT_BASE_URL/token/$erc20"

    i=1
    while [ "$MAX_TXS" -eq 0 ] || [ "$i" -le "$MAX_TXS" ]; do
      receipt=$(cast send "$erc20" "transfer(address,uint256)" "$recipient" "$AMOUNT_WEI" \
        --private-key "$traffic_key" \
        --legacy \
        --gas-price "$L2_GAS_PRICE_WEI" \
        --rpc-url "$L2_RPC_URL" \
        --json)

      tx_hash=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"transactionHash\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" | head -1)
      block_number=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"blockNumber\"[[:space:]]*:[[:space:]]*\"?([^\",}]+)\"?.*/\1/p" | head -1)
      [ -n "$tx_hash" ] || { echo "[l2-erc20-traffic] ERROR: cast receipt did not include transactionHash" >&2; printf "%s\n" "$receipt" >&2; exit 1; }
      [ -n "$block_number" ] || block_number="unknown"

      echo "[l2-erc20-traffic] $i tx=$tx_hash block=$block_number"
      echo "[l2-erc20-traffic] blockscout=$BLOCKSCOUT_BASE_URL/tx/$tx_hash"
      i=$((i + 1))
      sleep "$INTERVAL_SECONDS"
    done

    echo "[l2-erc20-traffic] completed maxTxs=$MAX_TXS"
  '
)"

lineth_kv "container id" "$container_id"
log "started $CONTAINER_NAME"
log "Blockscout UI: $BLOCKSCOUT_BASE_URL"
log "tail it with: $0 logs"
log "stop it with: $0 stop"
