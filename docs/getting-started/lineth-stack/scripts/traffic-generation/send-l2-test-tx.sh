#!/usr/bin/env sh
# Send one or more tiny L2 ETH transfers to make the local L2 and Blockscout move.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="l2-test-tx"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
die() { lineth_die "$*"; }

lineth_banner "L2 ETH transfer · local sequencer"

case "${COUNT:-1}" in
  ''|*[!0-9]*) die "COUNT must be a positive integer" ;;
esac
[ "${COUNT:-1}" -gt 0 ] || die "COUNT must be greater than zero"

case "${VALUE_WEI:-1}" in
  ''|*[!0-9]*) die "VALUE_WEI must be a non-negative integer" ;;
esac

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

[ -s "$(lineth_accounts_file runtime-keys.env)" ] || die "runtime-keys.env missing. Boot the stack first."
[ -s "$(lineth_accounts_file addresses-precomputed.json)" ] || die "addresses-precomputed.json missing. Boot the stack first."

if [ -f versions.env ]; then
  # shellcheck disable=SC1091
  . ./versions.env
fi

FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
L2_RPC_URL="${L2_RPC_URL:-http://sequencer:8545}"
L2_GAS_PRICE_WEI="${L2_GAS_PRICE_WEI:-100000000}"
HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"
BLOCKSCOUT_BASE_URL="${BLOCKSCOUT_BASE_URL:-http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND}"

case "$L2_GAS_PRICE_WEI" in
  ''|*[!0-9]*) die "L2_GAS_PRICE_WEI must be a decimal wei value" ;;
esac

section "sending L2 ETH transfer(s)"
if ! lineth_run_stream docker run --rm \
  --user 0:0 \
  --entrypoint sh \
  --network lineth-stack_linea \
  -v "$LINETH_ACCOUNTS_DIR:/accounts:ro" \
  -e COUNT="${COUNT:-1}" \
  -e VALUE_WEI="${VALUE_WEI:-1}" \
  -e TO="${TO:-}" \
  -e L2_RPC_URL="$L2_RPC_URL" \
  -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
  -e BLOCKSCOUT_BASE_URL="$BLOCKSCOUT_BASE_URL" \
  "$FOUNDRY_IMAGE" \
  -lc '
    set -eu

    [ -f /accounts/runtime-keys.env ] || { echo "[l2-test-tx] ERROR: /accounts/runtime-keys.env missing" >&2; exit 1; }
    [ -f /accounts/addresses-precomputed.json ] || { echo "[l2-test-tx] ERROR: /accounts/addresses-precomputed.json missing" >&2; exit 1; }

    . /accounts/runtime-keys.env
    : "${L2_DEPLOYER_PRIVATE_KEY:?L2_DEPLOYER_PRIVATE_KEY missing from runtime-keys.env}"
    case "$L2_GAS_PRICE_WEI" in
      ""|*[!0-9]*) echo "[l2-test-tx] ERROR: L2_GAS_PRICE_WEI must be a decimal wei value" >&2; exit 1 ;;
    esac

    if [ -n "${TO:-}" ]; then
      recipient="$TO"
    else
      recipient=$(sed -nE "s/.*\"l2PostmanAddress\"[[:space:]]*:[[:space:]]*\"(0x[a-fA-F0-9]{40})\".*/\1/p" /accounts/addresses-precomputed.json | head -1)
    fi
    echo "$recipient" | grep -qE "^0x[a-fA-F0-9]{40}$" || { echo "[l2-test-tx] ERROR: recipient address invalid: $recipient" >&2; exit 1; }
    printf "[l2-test-tx] l2GasPriceWei=%s\n" "$L2_GAS_PRICE_WEI"

    i=1
    while [ "$i" -le "$COUNT" ]; do
      receipt=$(cast send "$recipient" \
        --value "$VALUE_WEI" \
        --private-key "$L2_DEPLOYER_PRIVATE_KEY" \
        --legacy \
        --gas-price "$L2_GAS_PRICE_WEI" \
        --rpc-url "$L2_RPC_URL" \
        --json)

      tx_hash=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"transactionHash\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" | head -1)
      block_number=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"blockNumber\"[[:space:]]*:[[:space:]]*\"?([^\",}]+)\"?.*/\1/p" | head -1)
      [ -n "$tx_hash" ] || { echo "[l2-test-tx] ERROR: cast receipt did not include transactionHash" >&2; printf "%s\n" "$receipt" >&2; exit 1; }
      [ -n "$block_number" ] || block_number="unknown"

      printf "[l2-test-tx] %s/%s tx=%s block=%s to=%s valueWei=%s\n" "$i" "$COUNT" "$tx_hash" "$block_number" "$recipient" "$VALUE_WEI"
      printf "[l2-test-tx] blockscout=%s/tx/%s\n" "$BLOCKSCOUT_BASE_URL" "$tx_hash"
      i=$((i + 1))
    done

    current_block=$(cast block-number --rpc-url "$L2_RPC_URL")
    printf "[l2-test-tx] currentL2Block=%s\n" "$current_block"
  '
then
  exit 1
fi

section "links"
lineth_kv "Blockscout UI" "$BLOCKSCOUT_BASE_URL"
