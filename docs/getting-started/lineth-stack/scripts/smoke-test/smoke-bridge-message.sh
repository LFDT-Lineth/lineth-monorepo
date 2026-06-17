#!/usr/bin/env sh
# Real L1->L2 message smoke test.
#
# Sends one L1 `sendMessage` transaction, waits for Postman to claim it
# on the local L2, verifies the L2 claim receipt emitted MessageClaimed, and
# checks the recipient L2 balance increased by the bridged value.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="bridge-smoke"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
log() { lineth_info "$*"; }
die() { lineth_die "$*"; }

lineth_banner "bridge smoke · L1 to L2 message"

require_address() {
  label="$1"
  value="$2"
  echo "$value" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "$label missing or invalid"
}

require_uint() {
  label="$1"
  value="$2"
  case "$value" in
    ''|*[!0-9]*) die "$label must be a non-negative integer" ;;
  esac
}

rpc_l2() {
  method="$1"
  params="$2"
  curl -fsS "$L2_RPC_URL" \
    -H 'content-type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}"
}

rpc_result_hex() {
  sed -nE 's/.*"result"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' | head -1
}

cast_to_dec() {
  docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" to-dec "$1"
}

psql_value() {
  docker exec postman-pg psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postman}" -At -F '|' -c "$1" \
    | tr -d '\r'
}

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

[ -s "$(lineth_accounts_file addresses-precomputed.json)" ] || die "addresses-precomputed.json missing. Boot the stack first."
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

PRE="$(lineth_accounts_file addresses-precomputed.json)"
ADDR="$(lineth_deployments_file addresses.json)"
[ -s "$PRE" ] || die "addresses-precomputed.json missing"
[ -s "$ADDR" ] || die "addresses.json missing; deploy-contracts has not completed"

LINEA_ROLLUP="$(lineth_json_section_addr "$ADDR" l1 LineaRollupV8)"
L2_MESSAGE_SERVICE="$(lineth_json_section_addr "$ADDR" l2 L2MessageService)"
L1_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l1 TokenBridge)"
L2_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l2 TokenBridge)"
L1_DEPLOYER_ADDRESS="$(lineth_json_section_addr "$PRE" deployers l1)"

require_address "L1 LineaRollupV8" "$LINEA_ROLLUP"
require_address "L2 L2MessageService" "$L2_MESSAGE_SERVICE"
require_address "L1 deployer" "$L1_DEPLOYER_ADDRESS"

HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"

HOST_PORT_L2_RPC="$(lineth_host_port HOST_PORT_L2_RPC 8745)"
L2_RPC_URL="${L2_RPC_URL:-http://localhost:$HOST_PORT_L2_RPC}"

L1_RPC_URL="$(lineth_l1_host_rpc_url)"
L1_CONTAINER_RPC_URL="$(lineth_l1_container_rpc_url)"
L1_DEPLOYER_PRIVATE_KEY="$(lineth_l1_deployer_private_key)"
[ -n "$L1_RPC_URL" ] || die "L1_RPC_URL must be set or provided by L1_MODE=local"
[ -n "$L1_CONTAINER_RPC_URL" ] || die "container L1 RPC URL must be set or provided by L1_MODE=local"
[ -n "$L1_DEPLOYER_PRIVATE_KEY" ] || die "L1_DEPLOYER_PRIVATE_KEY must be set or provided by L1_MODE=local"

RECIPIENT="${RECIPIENT:-0x1000000000000000000000000000000000000001}"
L1_MESSAGE_VALUE_WEI="${L1_MESSAGE_VALUE_WEI:-100000000000000}"
L1_MESSAGE_FEE_WEI="${L1_MESSAGE_FEE_WEI:-0}"
CALLDATA="${CALLDATA:-0x}"
BRIDGE_SMOKE_TIMEOUT_SECONDS="${BRIDGE_SMOKE_TIMEOUT_SECONDS:-360}"
BRIDGE_SMOKE_POLL_SECONDS="${BRIDGE_SMOKE_POLL_SECONDS:-5}"
FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
MESSAGE_CLAIMED_TOPIC="0xa4c827e719e911e8f19393ccdb85b5102f08f0910604d340ba38390b7ff2ab0e"

require_address "RECIPIENT" "$RECIPIENT"
require_uint "L1_MESSAGE_VALUE_WEI" "$L1_MESSAGE_VALUE_WEI"
require_uint "L1_MESSAGE_FEE_WEI" "$L1_MESSAGE_FEE_WEI"
require_uint "BRIDGE_SMOKE_TIMEOUT_SECONDS" "$BRIDGE_SMOKE_TIMEOUT_SECONDS"
require_uint "BRIDGE_SMOKE_POLL_SECONDS" "$BRIDGE_SMOKE_POLL_SECONDS"
echo "$CALLDATA" | grep -qE '^0x([a-fA-F0-9]{2})*$' || die "CALLDATA must be hex bytes"

TOTAL_VALUE_WEI=$((L1_MESSAGE_VALUE_WEI + L1_MESSAGE_FEE_WEI))

section "preflight"
log "LineaRollupV8: $(lineth_l1_address_link "$LINEA_ROLLUP")"
log "L2MessageService: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_MESSAGE_SERVICE"
[ -n "$L1_TOKEN_BRIDGE" ] && log "L1 TokenBridge: $(lineth_l1_address_link "$L1_TOKEN_BRIDGE")"
[ -n "$L2_TOKEN_BRIDGE" ] && log "L2 TokenBridge: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_TOKEN_BRIDGE"
log "sender: $L1_DEPLOYER_ADDRESS"
log "recipient: $RECIPIENT"
log "valueWei: $L1_MESSAGE_VALUE_WEI"
log "feeWei: $L1_MESSAGE_FEE_WEI"

START_MESSAGE_ID="$(psql_value "select coalesce(max(id),0) from message;")"
require_uint "postman max message id" "$START_MESSAGE_ID"

BALANCE_BEFORE_HEX="$(rpc_l2 eth_getBalance "[\"$RECIPIENT\",\"latest\"]" | rpc_result_hex)"
[ -n "$BALANCE_BEFORE_HEX" ] || die "could not read recipient L2 balance before send"
BALANCE_BEFORE_DEC="$(cast_to_dec "$BALANCE_BEFORE_HEX")"

section "send L1 message"
SEND_RECEIPT="$(
  docker run --rm \
    --entrypoint sh \
    --network lineth-stack_linea \
    -e L1_RPC_URL="$L1_CONTAINER_RPC_URL" \
    -e L1_DEPLOYER_PRIVATE_KEY="$L1_DEPLOYER_PRIVATE_KEY" \
    -e LINEA_ROLLUP="$LINEA_ROLLUP" \
    -e RECIPIENT="$RECIPIENT" \
    -e L1_MESSAGE_FEE_WEI="$L1_MESSAGE_FEE_WEI" \
    -e TOTAL_VALUE_WEI="$TOTAL_VALUE_WEI" \
    -e CALLDATA="$CALLDATA" \
    "$FOUNDRY_IMAGE" \
    -lc 'cast send "$LINEA_ROLLUP" "sendMessage(address,uint256,bytes)" "$RECIPIENT" "$L1_MESSAGE_FEE_WEI" "$CALLDATA" --value "$TOTAL_VALUE_WEI" --rpc-url "$L1_RPC_URL" --private-key "$L1_DEPLOYER_PRIVATE_KEY" --json'
)"

L1_TX_HASH="$(printf '%s\n' "$SEND_RECEIPT" | lineth_json_stdin_string_field transactionHash)"
echo "$L1_TX_HASH" | grep -qE '^0x[a-fA-F0-9]{64}$' || {
  printf '%s\n' "$SEND_RECEIPT" >&2
  die "cast send did not return a transactionHash"
}
log "l1Tx: $L1_TX_HASH"
log "l1TxLink: $(lineth_l1_tx_link "$L1_TX_HASH")"

section "wait for Postman claim"
DEADLINE=$(( $(date +%s) + BRIDGE_SMOKE_TIMEOUT_SECONDS ))
ROW=""
while [ "$(date +%s)" -le "$DEADLINE" ]; do
  ROW="$(psql_value "select id,status,message_hash,coalesce(claim_tx_hash,''),message_sender,destination,value,message_nonce,coalesce(compressed_transaction_size::text,''),coalesce(is_for_sponsorship::text,'') from message where id > $START_MESSAGE_ID and direction='L1_TO_L2' and lower(destination)=lower('$RECIPIENT') and value='$L1_MESSAGE_VALUE_WEI' order by id desc limit 1;")"
  if [ -n "$ROW" ]; then
    STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
    CLAIM_TX_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 4)"
    if [ "$STATUS" = "CLAIMED_SUCCESS" ] && echo "$CLAIM_TX_HASH" | grep -qE '^0x[a-fA-F0-9]{64}$'; then
      break
    fi
    case "$STATUS" in
      NON_EXECUTABLE|CLAIMED_REVERTED|ZERO_FEE|FEE_UNDERPRICED|NEEDS_MANUAL_INTERVENTION)
        printf '%s\n' "$ROW" >&2
        die "postman moved message to terminal/problem status: $STATUS"
        ;;
    esac
  fi
  sleep "$BRIDGE_SMOKE_POLL_SECONDS"
done

[ -n "$ROW" ] || die "timed out waiting for postman to ingest the L1 MessageSent event"

MESSAGE_ID="$(printf '%s' "$ROW" | cut -d '|' -f 1)"
STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
MESSAGE_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 3)"
CLAIM_TX_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 4)"
MESSAGE_SENDER="$(printf '%s' "$ROW" | cut -d '|' -f 5)"
MESSAGE_NONCE="$(printf '%s' "$ROW" | cut -d '|' -f 8)"
COMPRESSED_TX_SIZE="$(printf '%s' "$ROW" | cut -d '|' -f 9)"
IS_FOR_SPONSORSHIP="$(printf '%s' "$ROW" | cut -d '|' -f 10)"

[ "$STATUS" = "CLAIMED_SUCCESS" ] || {
  printf '%s\n' "$ROW" >&2
  die "timed out waiting for CLAIMED_SUCCESS; last status=$STATUS"
}

log "messageId: $MESSAGE_ID"
log "messageHash: $MESSAGE_HASH"
log "messageSender: $MESSAGE_SENDER"
log "messageNonce: $MESSAGE_NONCE"
log "sponsoredByPostman: $IS_FOR_SPONSORSHIP"
log "compressedTxSize: $COMPRESSED_TX_SIZE"
log "l2ClaimTx: $CLAIM_TX_HASH"
log "l2ClaimExplorer: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/tx/$CLAIM_TX_HASH"

section "verify L2 receipt"
CLAIM_RECEIPT="$(rpc_l2 eth_getTransactionReceipt "[\"$CLAIM_TX_HASH\"]")"
printf '%s\n' "$CLAIM_RECEIPT" | grep -q '"status":"0x1"' || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L2 claim receipt missing or failed"
}
printf '%s\n' "$CLAIM_RECEIPT" | grep -qi "$MESSAGE_CLAIMED_TOPIC" || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L2 claim receipt did not emit MessageClaimed"
}
printf '%s\n' "$CLAIM_RECEIPT" | grep -qi "$MESSAGE_HASH" || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L2 claim receipt MessageClaimed hash does not match postman message hash"
}

CLAIM_BLOCK_HEX="$(printf '%s\n' "$CLAIM_RECEIPT" | lineth_json_stdin_string_field blockNumber)"
CLAIM_BLOCK_DEC="$(cast_to_dec "$CLAIM_BLOCK_HEX")"
log "l2ClaimBlock: $CLAIM_BLOCK_DEC"

BALANCE_AFTER_HEX="$(rpc_l2 eth_getBalance "[\"$RECIPIENT\",\"latest\"]" | rpc_result_hex)"
[ -n "$BALANCE_AFTER_HEX" ] || die "could not read recipient L2 balance after claim"
BALANCE_AFTER_DEC="$(cast_to_dec "$BALANCE_AFTER_HEX")"
BALANCE_DELTA=$((BALANCE_AFTER_DEC - BALANCE_BEFORE_DEC))

if [ "$BALANCE_DELTA" -lt "$L1_MESSAGE_VALUE_WEI" ]; then
  die "recipient L2 balance delta $BALANCE_DELTA is lower than expected $L1_MESSAGE_VALUE_WEI"
fi

section "success"
log "recipientBalanceBeforeWei: $BALANCE_BEFORE_DEC"
log "recipientBalanceAfterWei: $BALANCE_AFTER_DEC"
log "recipientBalanceDeltaWei: $BALANCE_DELTA"
log "recipientExplorer: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$RECIPIENT"
log "L1 tx: $(lineth_l1_tx_link "$L1_TX_HASH")"
log "Local L2 claim tx: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/tx/$CLAIM_TX_HASH"
