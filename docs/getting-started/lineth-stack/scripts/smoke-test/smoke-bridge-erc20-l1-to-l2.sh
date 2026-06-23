#!/usr/bin/env sh
# Real TokenBridge ERC20 L1->L2 smoke test.
#
# Mints L1 ERC20Example to the configured L1 deployer, approves the L1
# TokenBridge, calls bridgeToken(...), waits for Postman to claim the message on
# local L2, and verifies the recipient received the bridged ERC20 balance.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="erc20-bridge-l1-to-l2"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
log() { lineth_info "$*"; }
die() { lineth_die "$*"; }

lineth_banner "ERC20 bridge smoke · L1 to L2"

require_address() {
  label="$1"
  value="$2"
  echo "$value" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "$label missing or invalid"
}

require_hash() {
  label="$1"
  value="$2"
  echo "$value" | grep -qE '^0x[a-fA-F0-9]{64}$' || die "$label missing or invalid"
}

require_uint() {
  label="$1"
  value="$2"
  case "$value" in
    ''|*[!0-9]*) die "$label must be a non-negative integer" ;;
  esac
}

psql_value() {
  docker exec postman-pg psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postman}" -At -F '|' -c "$1" \
    | tr -d '\r'
}

cast_wallet_address() {
  private_key="$1"
  docker run --rm \
    --entrypoint cast \
    "$FOUNDRY_IMAGE" wallet address --private-key "$private_key"
}

cast_l2_call() {
  docker run --rm \
    --network lineth-stack_linea \
    --entrypoint cast \
    "$FOUNDRY_IMAGE" call "$@" --rpc-url "$L2_RPC_URL"
}

cast_l1_send() {
  docker run --rm \
    --network lineth-stack_linea \
    --entrypoint cast \
    -e L1_RPC_URL="$L1_RPC_URL" \
    -e L1_DEPLOYER_PRIVATE_KEY="$L1_DEPLOYER_PRIVATE_KEY" \
    "$FOUNDRY_IMAGE" send "$@" --rpc-url "$L1_RPC_URL" --private-key "$L1_DEPLOYER_PRIVATE_KEY" --json
}

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

[ -s "$(lineth_deployments_file addresses.json)" ] || die "addresses.json missing; deploy-contracts has not completed."

if ! docker ps --format '{{.Names}}' | grep -qx 'postman-pg'; then
  die "postman-pg is not running. Boot the stack first."
fi

if [ -f versions.env ]; then
  # shellcheck disable=SC1091
  . ./versions.env
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

ADDR="$(lineth_deployments_file addresses.json)"
DEMO_TRAFFIC_ENV="$(lineth_accounts_file demo-traffic.env)"
[ -s "$ADDR" ] || die "addresses.json missing; deploy-contracts has not completed"

section "ensuring L1 ERC20Example"
if ! lineth_run_stream docker compose --env-file versions.env --env-file .env --profile stack-partial-prover \
  run --rm --no-deps --entrypoint bash deploy-contracts /scripts/internal/ensure-demo-erc20.sh l1; then
  exit 1
fi
L1_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l1 TokenBridge)"
L2_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l2 TokenBridge)"
L1_ERC20="$(lineth_json_section_addr "$ADDR" l1 ERC20Example)"
L1_CHAIN_ID="$(lineth_json_meta_value "$ADDR" l1ChainId)"

require_address "L1 TokenBridge" "$L1_TOKEN_BRIDGE"
require_address "L2 TokenBridge" "$L2_TOKEN_BRIDGE"
require_address "L1 ERC20Example" "$L1_ERC20"
require_uint "l1ChainId" "$L1_CHAIN_ID"

L1_RPC_URL="$(lineth_l1_container_rpc_url)"
L1_DEPLOYER_PRIVATE_KEY="$(lineth_l1_deployer_private_key)"
[ -n "$L1_RPC_URL" ] || die "L1_RPC_URL must be set or provided by L1_MODE=local"
[ -n "$L1_DEPLOYER_PRIVATE_KEY" ] || die "L1_DEPLOYER_PRIVATE_KEY must be set or provided by L1_MODE=local"

HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"

FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
L2_RPC_URL="${L2_RPC_URL:-http://l2-node-besu:8545}"
BRIDGE_AMOUNT_WEI="${BRIDGE_AMOUNT_WEI:-1000000000000000000}"
BRIDGE_MESSAGE_FEE_WEI="${BRIDGE_MESSAGE_FEE_WEI:-10000000000000000}"
BRIDGE_SMOKE_TIMEOUT_SECONDS="${BRIDGE_SMOKE_TIMEOUT_SECONDS:-900}"
BRIDGE_SMOKE_POLL_SECONDS="${BRIDGE_SMOKE_POLL_SECONDS:-10}"

require_uint "BRIDGE_AMOUNT_WEI" "$BRIDGE_AMOUNT_WEI"
require_uint "BRIDGE_MESSAGE_FEE_WEI" "$BRIDGE_MESSAGE_FEE_WEI"
require_uint "BRIDGE_SMOKE_TIMEOUT_SECONDS" "$BRIDGE_SMOKE_TIMEOUT_SECONDS"
require_uint "BRIDGE_SMOKE_POLL_SECONDS" "$BRIDGE_SMOKE_POLL_SECONDS"
[ "$BRIDGE_AMOUNT_WEI" -gt 0 ] || die "BRIDGE_AMOUNT_WEI must be greater than zero"

SENDER="$(cast_wallet_address "$L1_DEPLOYER_PRIVATE_KEY")"
if [ -n "${RECIPIENT:-}" ]; then
  L2_RECIPIENT="$RECIPIENT"
elif [ -s "$DEMO_TRAFFIC_ENV" ]; then
  # shellcheck disable=SC1090
  . "$DEMO_TRAFFIC_ENV"
  [ -n "${L2_TRAFFIC_PRIVATE_KEY:-}" ] || die "demo-traffic.env exists but L2_TRAFFIC_PRIVATE_KEY is missing"
  L2_RECIPIENT="$(cast_wallet_address "$L2_TRAFFIC_PRIVATE_KEY")"
else
  L2_RECIPIENT="0x1000000000000000000000000000000000000001"
fi
require_address "L2 recipient" "$L2_RECIPIENT"

START_MESSAGE_ID="$(psql_value "select coalesce(max(id),0) from message;")"
require_uint "postman max message id" "$START_MESSAGE_ID"

section "preflight"
log "L1 ERC20Example: $(lineth_l1_address_link "$L1_ERC20")"
log "L1 TokenBridge: $(lineth_l1_address_link "$L1_TOKEN_BRIDGE")"
log "L2 TokenBridge: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_TOKEN_BRIDGE"
log "sender: $SENDER"
log "recipient: $L2_RECIPIENT"
log "amountWei: $BRIDGE_AMOUNT_WEI"
log "messageFeeWei: $BRIDGE_MESSAGE_FEE_WEI"

BRIDGED_TOKEN_BEFORE="$(cast_l2_call "$L2_TOKEN_BRIDGE" 'nativeToBridgedToken(uint256,address)(address)' "$L1_CHAIN_ID" "$L1_ERC20" | tr -d '[:space:]')"
BALANCE_BEFORE=0
case "$BRIDGED_TOKEN_BEFORE" in
  0x0000000000000000000000000000000000000000|0x0000000000000000000000000000000000000111|0x0000000000000000000000000000000000000222|0x0000000000000000000000000000000000000333)
    ;;
  *)
    BALANCE_BEFORE_RAW="$(cast_l2_call "$BRIDGED_TOKEN_BEFORE" 'balanceOf(address)(uint256)' "$L2_RECIPIENT")"
    BALANCE_BEFORE="$(printf '%s\n' "$BALANCE_BEFORE_RAW" | awk '{print $1}')"
    require_uint "recipient bridged balance before" "$BALANCE_BEFORE"
    ;;
esac

section "mint and approve on L1"
MINT_RECEIPT="$(cast_l1_send "$L1_ERC20" 'mint(address,uint256)' "$SENDER" "$BRIDGE_AMOUNT_WEI")"
MINT_TX_HASH="$(printf '%s\n' "$MINT_RECEIPT" | lineth_json_stdin_string_field transactionHash)"
require_hash "mint tx hash" "$MINT_TX_HASH"
log "mintTx: $(lineth_l1_tx_link "$MINT_TX_HASH")"

APPROVE_RECEIPT="$(cast_l1_send "$L1_ERC20" 'approve(address,uint256)' "$L1_TOKEN_BRIDGE" "$BRIDGE_AMOUNT_WEI")"
APPROVE_TX_HASH="$(printf '%s\n' "$APPROVE_RECEIPT" | lineth_json_stdin_string_field transactionHash)"
require_hash "approve tx hash" "$APPROVE_TX_HASH"
log "approveTx: $(lineth_l1_tx_link "$APPROVE_TX_HASH")"

section "bridge L1 ERC20Example to L2"
BRIDGE_RECEIPT="$(cast_l1_send "$L1_TOKEN_BRIDGE" 'bridgeToken(address,uint256,address)' "$L1_ERC20" "$BRIDGE_AMOUNT_WEI" "$L2_RECIPIENT" --value "$BRIDGE_MESSAGE_FEE_WEI")"
BRIDGE_TX_HASH="$(printf '%s\n' "$BRIDGE_RECEIPT" | lineth_json_stdin_string_field transactionHash)"
require_hash "bridge tx hash" "$BRIDGE_TX_HASH"
log "bridgeTx: $(lineth_l1_tx_link "$BRIDGE_TX_HASH")"

section "wait for Postman L2 claim"
DEADLINE=$(( $(date +%s) + BRIDGE_SMOKE_TIMEOUT_SECONDS ))
ROW=""
while [ "$(date +%s)" -le "$DEADLINE" ]; do
  ROW="$(psql_value "select id,status,message_hash,coalesce(claim_tx_hash,''),message_sender,destination,value from message where id > $START_MESSAGE_ID and direction='L1_TO_L2' and lower(message_sender)=lower('$L1_TOKEN_BRIDGE') and lower(destination)=lower('$L2_TOKEN_BRIDGE') order by id desc limit 1;")"
  if [ -n "$ROW" ]; then
    STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
    CLAIM_TX_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 4)"
    log "postmanStatus: $STATUS"
    if [ "$STATUS" = "CLAIMED_SUCCESS" ] && echo "$CLAIM_TX_HASH" | grep -qE '^0x[a-fA-F0-9]{64}$'; then
      break
    fi
    case "$STATUS" in
      NON_EXECUTABLE|CLAIMED_REVERTED|ZERO_FEE|FEE_UNDERPRICED|NEEDS_MANUAL_INTERVENTION)
        printf '%s\n' "$ROW" >&2
        die "postman moved ERC20 bridge message to terminal/problem status: $STATUS"
        ;;
    esac
  else
    log "postmanStatus: pending"
  fi
  sleep "$BRIDGE_SMOKE_POLL_SECONDS"
done

[ -n "$ROW" ] || die "timed out waiting for postman to ingest ERC20 bridge message"
STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
CLAIM_TX_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 4)"
[ "$STATUS" = "CLAIMED_SUCCESS" ] || {
  printf '%s\n' "$ROW" >&2
  die "timed out waiting for CLAIMED_SUCCESS; last status=$STATUS"
}
require_hash "L2 claim tx hash" "$CLAIM_TX_HASH"
log "l2ClaimTx: $CLAIM_TX_HASH"
log "l2ClaimExplorer: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/tx/$CLAIM_TX_HASH"

section "verify bridged ERC20 balance"
BRIDGED_TOKEN="$(cast_l2_call "$L2_TOKEN_BRIDGE" 'nativeToBridgedToken(uint256,address)(address)' "$L1_CHAIN_ID" "$L1_ERC20" | tr -d '[:space:]')"
require_address "L2 bridged token" "$BRIDGED_TOKEN"
case "$BRIDGED_TOKEN" in
  0x0000000000000000000000000000000000000000|0x0000000000000000000000000000000000000111|0x0000000000000000000000000000000000000222|0x0000000000000000000000000000000000000333)
    die "L2 TokenBridge mapping did not resolve to a bridged token: $BRIDGED_TOKEN"
    ;;
esac

BALANCE_AFTER_RAW="$(cast_l2_call "$BRIDGED_TOKEN" 'balanceOf(address)(uint256)' "$L2_RECIPIENT")"
BALANCE_AFTER="$(printf '%s\n' "$BALANCE_AFTER_RAW" | awk '{print $1}')"
require_uint "recipient bridged balance after" "$BALANCE_AFTER"

DELTA=$((BALANCE_AFTER - BALANCE_BEFORE))
[ "$DELTA" -ge "$BRIDGE_AMOUNT_WEI" ] || {
  log "balanceBefore: $BALANCE_BEFORE"
  log "balanceAfter: $BALANCE_AFTER"
  die "recipient bridged ERC20 balance did not increase by at least $BRIDGE_AMOUNT_WEI"
}

log "l2BridgedToken: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/token/$BRIDGED_TOKEN"
log "recipientBalanceBeforeWei: $BALANCE_BEFORE"
log "recipientBalanceAfterWei: $BALANCE_AFTER"
log "recipientBalanceDeltaWei: $DELTA"
log "OK: ERC20 TokenBridge L1->L2 transfer verified"
