#!/usr/bin/env sh
# Transfer a tiny amount of the deployed L2 ERC20Example token.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="l2-erc20-transfer"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
die() { lineth_die "$*"; }

lineth_banner "L2 ERC20Example transfer · demo account"

case "${AMOUNT_WEI:-1}" in
  ''|*[!0-9]*) die "AMOUNT_WEI must be a positive integer" ;;
esac
[ "${AMOUNT_WEI:-1}" -gt 0 ] || die "AMOUNT_WEI must be greater than zero"

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

[ -s "$(lineth_accounts_file runtime-keys.env)" ] || die "runtime-keys.env missing. Boot the stack first."
[ -s "$(lineth_deployments_file addresses.json)" ] || die "addresses.json missing; deploy-contracts has not completed."

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

section "sending L2 ERC20Example transfer"
if ! lineth_run_stream docker run --rm \
  --user 0:0 \
  --entrypoint sh \
  --network lineth-stack_linea \
  -v "$LINETH_ACCOUNTS_DIR:/accounts:ro" \
  -v "$LINETH_DEPLOYMENTS_DIR:/deployments:ro" \
  -e AMOUNT_WEI="${AMOUNT_WEI:-1}" \
  -e TO="${TO:-}" \
  -e ERC20="$ERC20" \
  -e L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}" \
  -e L2_RPC_URL="$L2_RPC_URL" \
  -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
  -e BLOCKSCOUT_BASE_URL="$BLOCKSCOUT_BASE_URL" \
  "$FOUNDRY_IMAGE" \
  -lc '
    set -eu

    [ -f /accounts/runtime-keys.env ] || { echo "[l2-erc20-transfer] ERROR: /accounts/runtime-keys.env missing" >&2; exit 1; }
    [ -f /deployments/addresses.json ] || { echo "[l2-erc20-transfer] ERROR: /deployments/addresses.json missing; deploy-contracts has not completed" >&2; exit 1; }

    DEMO_TRAFFIC_ENV="/accounts/demo-traffic.env"

    is_privkey() { printf "%s\n" "$1" | grep -qE "^0x[a-fA-F0-9]{64}$"; }
    is_uint() { printf "%s\n" "$1" | grep -qE "^[0-9]+$"; }

    is_uint "$L2_GAS_PRICE_WEI" || { echo "[l2-erc20-transfer] ERROR: L2_GAS_PRICE_WEI must be a decimal wei value" >&2; exit 1; }

    if [ -n "${L2_TRAFFIC_PRIVATE_KEY:-}" ]; then
      traffic_key="$L2_TRAFFIC_PRIVATE_KEY"
      echo "[l2-erc20-transfer] using L2_TRAFFIC_PRIVATE_KEY from environment"
    elif [ -f "$DEMO_TRAFFIC_ENV" ]; then
      . "$DEMO_TRAFFIC_ENV"
      traffic_key="${L2_TRAFFIC_PRIVATE_KEY:-}"
      echo "[l2-erc20-transfer] reusing disposable traffic account from $DEMO_TRAFFIC_ENV"
    else
      echo "[l2-erc20-transfer] ERROR: no disposable traffic account found after traffic-account helper" >&2
      exit 1
    fi
    is_privkey "$traffic_key" || { echo "[l2-erc20-transfer] ERROR: L2 traffic private key malformed" >&2; exit 1; }

    erc20="$ERC20"
    echo "$erc20" | grep -qE "^0x[a-fA-F0-9]{40}$" || { echo "[l2-erc20-transfer] ERROR: ERC20 invalid: $erc20" >&2; exit 1; }

    if [ -n "${TO:-}" ]; then
      recipient="$TO"
    else
      recipient="0x1000000000000000000000000000000000000001"
    fi
    echo "$recipient" | grep -qE "^0x[a-fA-F0-9]{40}$" || { echo "[l2-erc20-transfer] ERROR: recipient address invalid: $recipient" >&2; exit 1; }

    sender=$(cast wallet address --private-key "$traffic_key")
    printf "[l2-erc20-transfer] l2GasPriceWei=%s\n" "$L2_GAS_PRICE_WEI"

    receipt=$(cast send "$erc20" "transfer(address,uint256)" "$recipient" "$AMOUNT_WEI" \
      --private-key "$traffic_key" \
      --legacy \
      --gas-price "$L2_GAS_PRICE_WEI" \
      --rpc-url "$L2_RPC_URL" \
      --json)

    tx_hash=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"transactionHash\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" | head -1)
    block_number=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"blockNumber\"[[:space:]]*:[[:space:]]*\"?([^\",}]+)\"?.*/\1/p" | head -1)
    [ -n "$tx_hash" ] || { echo "[l2-erc20-transfer] ERROR: cast receipt did not include transactionHash" >&2; printf "%s\n" "$receipt" >&2; exit 1; }
    [ -n "$block_number" ] || block_number="unknown"

    printf "[l2-erc20-transfer] token=%s\n" "$erc20"
    printf "[l2-erc20-transfer] trafficAccount=%s\n" "$sender"
    printf "[l2-erc20-transfer] from=%s to=%s amountWei=%s\n" "$sender" "$recipient" "$AMOUNT_WEI"
    printf "[l2-erc20-transfer] tx=%s block=%s\n" "$tx_hash" "$block_number"
    printf "[l2-erc20-transfer] blockscout=%s/tx/%s\n" "$BLOCKSCOUT_BASE_URL" "$tx_hash"
    printf "[l2-erc20-transfer] tokenUrl=%s/token/%s\n" "$BLOCKSCOUT_BASE_URL" "$erc20"
  '
then
  exit 1
fi

section "links"
lineth_kv "Blockscout UI" "$BLOCKSCOUT_BASE_URL"
