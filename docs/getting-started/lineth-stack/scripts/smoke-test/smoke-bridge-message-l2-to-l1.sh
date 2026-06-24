#!/usr/bin/env sh
# Real L2->L1 message smoke test.
#
# Sends one local L2 `sendMessage` transaction, waits until the coordinator
# finalizes/anchors the L2 message on L1, claims it with the SDK proof path,
# and verifies the L1 claim receipt emitted MessageClaimed.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="bridge-smoke-l2-to-l1"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/../lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"
STACK_DIR="$LINETH_STACK_DIR"

section() { lineth_section "$*"; }
log() { lineth_info "$*"; }
die() { lineth_die "$*"; }

lineth_banner "bridge smoke · L2 to L1 message"

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

rpc_l1() {
  method="$1"
  params="$2"
  curl -fsS "$L1_RPC_URL" \
    -H 'content-type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}"
}

psql_value() {
  docker exec postman-pg psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postman}" -At -F '|' -c "$1" \
    | tr -d '\r'
}

wait_for_postman_claim_tx() {
  message_id="$1"
  deadline="$2"

  while [ "$(date +%s)" -le "$deadline" ]; do
    claim_tx_hash="$(
      psql_value "select coalesce(claim_tx_hash,'') from message where id=$message_id and status='CLAIMED_SUCCESS';"
    )"
    if printf '%s' "$claim_tx_hash" | grep -qE '^0x[a-fA-F0-9]{64}$'; then
      printf '%s\n' "$claim_tx_hash"
      return 0
    fi
    sleep "$BRIDGE_SMOKE_POLL_SECONDS"
  done

  return 1
}

claim_l2_to_l1() {
  runtime_keys_env="$(lineth_accounts_file runtime-keys.env)"
  # shellcheck disable=SC1090
  . "$runtime_keys_env"
  l1_postman_private_key="${L1_POSTMAN_PRIVATE_KEY:-}"
  [ -n "$l1_postman_private_key" ] || die "L1_POSTMAN_PRIVATE_KEY missing from runtime-keys.env"

  docker exec -i \
    -e L1_SIGNER_PRIVATE_KEY="$l1_postman_private_key" \
    -e SMOKE_L1_CHAIN_ID="$L1_CHAIN_ID" \
    -e SMOKE_L2_CHAIN_ID="$L2_CHAIN_ID" \
    -e SMOKE_LINEA_ROLLUP_ADDRESS="$LINEA_ROLLUP" \
    -e SMOKE_L2_MESSAGE_SERVICE_ADDRESS="$L2_MESSAGE_SERVICE" \
    -e SMOKE_MESSAGE_HASH="$MESSAGE_HASH" \
    -e SMOKE_MESSAGE_SENDER="$MESSAGE_SENDER" \
    -e SMOKE_DESTINATION="$DESTINATION" \
    -e SMOKE_FEE="$MESSAGE_FEE" \
    -e SMOKE_VALUE="$MESSAGE_VALUE" \
    -e SMOKE_MESSAGE_NONCE="$MESSAGE_NONCE" \
    -e SMOKE_CALLDATA="$MESSAGE_CALLDATA" \
    -e SMOKE_SENT_BLOCK_NUMBER="$SENT_BLOCK_NUMBER" \
    postman \
    sh -lc 'cd /usr/src/app/postman && node --input-type=module' \
    < "$STACK_DIR/scripts/internal/claim-l2-to-l1.ts"
}

if ! docker info >/dev/null 2>&1; then
  die "Docker daemon is not reachable"
fi

[ -s "$(lineth_accounts_file addresses-precomputed.json)" ] || die "addresses-precomputed.json missing. Boot the stack first."
[ -s "$(lineth_accounts_file runtime-keys.env)" ] || die "runtime-keys.env missing. Boot the stack first."
[ -s "$(lineth_deployments_file addresses.json)" ] || die "addresses.json missing; deploy-contracts has not completed."

if ! docker ps --format '{{.Names}}' | grep -qx 'postman-pg'; then
  die "postman-pg is not running. Boot the stack first."
fi

if ! docker ps --format '{{.Names}}' | grep -qx 'postman'; then
  die "postman is not running. Boot the stack first."
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
L1_CHAIN_ID="$(lineth_json_meta_value "$ADDR" l1ChainId)"
L2_CHAIN_ID="$(lineth_json_meta_value "$ADDR" l2ChainId)"
L1_POSTMAN_ADDRESS="$(lineth_json_section_addr "$PRE" signers l1PostmanAddress)"
L2_DEPLOYER_ADDRESS="$(lineth_json_section_addr "$PRE" signers l2DeployerAddress)"

require_address "L1 LineaRollupV8" "$LINEA_ROLLUP"
require_address "L2 L2MessageService" "$L2_MESSAGE_SERVICE"
require_address "L1 postman signer" "$L1_POSTMAN_ADDRESS"
require_address "L2 deployer" "$L2_DEPLOYER_ADDRESS"
require_uint "l1ChainId" "$L1_CHAIN_ID"
require_uint "l2ChainId" "$L2_CHAIN_ID"

HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"

L1_RPC_URL="$(lineth_l1_host_rpc_url)"
[ -n "$L1_RPC_URL" ] || die "L1_RPC_URL must be set or provided by L1_MODE=local"

RECIPIENT="${RECIPIENT:-$L1_POSTMAN_ADDRESS}"
L2_L1_MESSAGE_VALUE_WEI="${L2_L1_MESSAGE_VALUE_WEI:-0}"
L2_L1_MESSAGE_FEE_WEI="${L2_L1_MESSAGE_FEE_WEI:-0}"
CALLDATA="${CALLDATA:-0x}"
BRIDGE_SMOKE_TIMEOUT_SECONDS="${BRIDGE_SMOKE_TIMEOUT_SECONDS:-900}"
BRIDGE_SMOKE_POLL_SECONDS="${BRIDGE_SMOKE_POLL_SECONDS:-10}"
L1_RECEIPT_TIMEOUT_SECONDS="${L1_RECEIPT_TIMEOUT_SECONDS:-180}"
L2_TRAFFIC_ETH_MIN_BALANCE_WEI="${L2_TRAFFIC_ETH_MIN_BALANCE_WEI:-100000000000000000}"
L2_TRAFFIC_ETH_TOP_UP_WEI="${L2_TRAFFIC_ETH_TOP_UP_WEI:-1000000000000000000}"
L2_GAS_PRICE_WEI="${L2_GAS_PRICE_WEI:-100000000}"
FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
L2_READ_RPC_URL="${L2_READ_RPC_URL:-${L2_RPC_URL:-http://l2-node-besu:8545}}"
L2_SEND_RPC_URL="${L2_SEND_RPC_URL:-http://sequencer:8545}"
MESSAGE_CLAIMED_TOPIC="0xa4c827e719e911e8f19393ccdb85b5102f08f0910604d340ba38390b7ff2ab0e"

require_address "RECIPIENT" "$RECIPIENT"
require_uint "L2_L1_MESSAGE_VALUE_WEI" "$L2_L1_MESSAGE_VALUE_WEI"
require_uint "L2_L1_MESSAGE_FEE_WEI" "$L2_L1_MESSAGE_FEE_WEI"
require_uint "BRIDGE_SMOKE_TIMEOUT_SECONDS" "$BRIDGE_SMOKE_TIMEOUT_SECONDS"
require_uint "BRIDGE_SMOKE_POLL_SECONDS" "$BRIDGE_SMOKE_POLL_SECONDS"
require_uint "L1_RECEIPT_TIMEOUT_SECONDS" "$L1_RECEIPT_TIMEOUT_SECONDS"
require_uint "L2_TRAFFIC_ETH_MIN_BALANCE_WEI" "$L2_TRAFFIC_ETH_MIN_BALANCE_WEI"
require_uint "L2_TRAFFIC_ETH_TOP_UP_WEI" "$L2_TRAFFIC_ETH_TOP_UP_WEI"
require_uint "L2_GAS_PRICE_WEI" "$L2_GAS_PRICE_WEI"
echo "$CALLDATA" | grep -qE '^0x([a-fA-F0-9]{2})*$' || die "CALLDATA must be hex bytes"

if [ "$L2_L1_MESSAGE_VALUE_WEI" != "0" ] || [ "$L2_L1_MESSAGE_FEE_WEI" != "0" ]; then
  die "this smoke currently supports zero-value, zero-postman-fee L2->L1 messages only"
fi

section "preflight"
log "LineaRollupV8: $(lineth_l1_address_link "$LINEA_ROLLUP")"
log "L2MessageService: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_MESSAGE_SERVICE"
log "manual L1 claim signer: $L1_POSTMAN_ADDRESS"
log "recipient: $RECIPIENT"
log "valueWei: $L2_L1_MESSAGE_VALUE_WEI"
log "postmanFeeWei: $L2_L1_MESSAGE_FEE_WEI"
log "l2GasPriceWei: $L2_GAS_PRICE_WEI"
log "l2ReadRpc: $L2_READ_RPC_URL"
log "l2SendRpc: $L2_SEND_RPC_URL"

START_MESSAGE_ID="$(psql_value "select coalesce(max(id),0) from message;")"
require_uint "postman max message id" "$START_MESSAGE_ID"

section "ensuring disposable traffic account"
TRAFFIC_ACCOUNT_OUTPUT="$(
  docker compose --env-file versions.env --env-file .env --profile stack-partial-prover \
    run --rm --no-deps \
    -v "$LINETH_ACCOUNTS_DIR:/traffic-accounts:rw" \
    -e DEMO_TRAFFIC_ENV="/traffic-accounts/demo-traffic.env" \
    -e L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}" \
    -e L2_TRAFFIC_ETH_MIN_BALANCE_WEI="$L2_TRAFFIC_ETH_MIN_BALANCE_WEI" \
    -e L2_TRAFFIC_ETH_TOP_UP_WEI="$L2_TRAFFIC_ETH_TOP_UP_WEI" \
    -e L2_READ_RPC_URL="$L2_READ_RPC_URL" \
    -e L2_SEND_RPC_URL="$L2_SEND_RPC_URL" \
    -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
    --entrypoint bash deploy-contracts /scripts/internal/traffic-account.sh ensure
)"
printf '%s\n' "$TRAFFIC_ACCOUNT_OUTPUT" | lineth_child_output
TRAFFIC_ACCOUNT_ADDRESS="$(printf '%s\n' "$TRAFFIC_ACCOUNT_OUTPUT" | sed -nE 's/^TRAFFIC_ACCOUNT_ADDRESS=(0x[a-fA-F0-9]{40})$/\1/p' | tail -1)"
require_address "traffic account helper address" "$TRAFFIC_ACCOUNT_ADDRESS"

section "send L2 message"
SEND_OUTPUT="$(
  docker run --rm \
    --user 0:0 \
    --entrypoint sh \
    --network lineth-stack_linea \
    -v "$LINETH_ACCOUNTS_DIR:/accounts:ro" \
    -e RECIPIENT="$RECIPIENT" \
    -e L2_MESSAGE_SERVICE="$L2_MESSAGE_SERVICE" \
    -e L2_L1_MESSAGE_FEE_WEI="$L2_L1_MESSAGE_FEE_WEI" \
    -e L2_L1_MESSAGE_VALUE_WEI="$L2_L1_MESSAGE_VALUE_WEI" \
    -e CALLDATA="$CALLDATA" \
    -e L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}" \
    -e L2_TRAFFIC_ETH_MIN_BALANCE_WEI="$L2_TRAFFIC_ETH_MIN_BALANCE_WEI" \
    -e L2_TRAFFIC_ETH_TOP_UP_WEI="$L2_TRAFFIC_ETH_TOP_UP_WEI" \
    -e L2_READ_RPC_URL="$L2_READ_RPC_URL" \
    -e L2_SEND_RPC_URL="$L2_SEND_RPC_URL" \
    -e L2_GAS_PRICE_WEI="$L2_GAS_PRICE_WEI" \
    "$FOUNDRY_IMAGE" \
    -lc '
      set -eu

      [ -f /accounts/runtime-keys.env ] || { echo "[bridge-smoke-l2-to-l1] ERROR: /accounts/runtime-keys.env missing" >&2; exit 1; }
      DEMO_TRAFFIC_ENV="/accounts/demo-traffic.env"

      is_privkey() { printf "%s\n" "$1" | grep -qE "^0x[a-fA-F0-9]{64}$"; }
      is_uint() { printf "%s\n" "$1" | grep -qE "^[0-9]+$"; }
      is_uint "$L2_GAS_PRICE_WEI" || { echo "[bridge-smoke-l2-to-l1] ERROR: L2_GAS_PRICE_WEI must be a decimal wei value" >&2; exit 1; }
      to_dec() {
        case "$1" in
          0x*) cast --to-dec "$1" ;;
          *) printf "%s\n" "$1" ;;
        esac
      }

      if [ -n "${L2_TRAFFIC_PRIVATE_KEY:-}" ]; then
        traffic_key="$L2_TRAFFIC_PRIVATE_KEY"
        echo "[bridge-smoke-l2-to-l1] using L2_TRAFFIC_PRIVATE_KEY from environment"
      elif [ -f "$DEMO_TRAFFIC_ENV" ]; then
        . "$DEMO_TRAFFIC_ENV"
        traffic_key="${L2_TRAFFIC_PRIVATE_KEY:-}"
        echo "[bridge-smoke-l2-to-l1] reusing disposable traffic account from $DEMO_TRAFFIC_ENV"
      else
        echo "[bridge-smoke-l2-to-l1] ERROR: no disposable traffic account found after traffic-account helper" >&2
        exit 1
      fi
      is_privkey "$traffic_key" || { echo "[bridge-smoke-l2-to-l1] ERROR: L2 traffic private key malformed" >&2; exit 1; }

      sender=$(cast wallet address --private-key "$traffic_key")

      minimum_fee_raw=$(cast call "$L2_MESSAGE_SERVICE" "minimumFeeInWei()(uint256)" --rpc-url "$L2_READ_RPC_URL" | awk "NF {print \$1; exit}")
      minimum_fee=$(to_dec "$minimum_fee_raw")
      is_uint "$minimum_fee" || { echo "[bridge-smoke-l2-to-l1] ERROR: could not read minimumFeeInWei" >&2; exit 1; }

      send_fee=$((minimum_fee + L2_L1_MESSAGE_FEE_WEI))
      total_value=$((L2_L1_MESSAGE_VALUE_WEI + send_fee))
      echo "[bridge-smoke-l2-to-l1] l2MinimumFeeInWei=$minimum_fee"
      echo "[bridge-smoke-l2-to-l1] l2SendFeeWei=$send_fee"
      echo "[bridge-smoke-l2-to-l1] l2TxValueWei=$total_value"

      receipt=$(cast send "$L2_MESSAGE_SERVICE" "sendMessage(address,uint256,bytes)" "$RECIPIENT" "$send_fee" "$CALLDATA" \
        --value "$total_value" \
        --private-key "$traffic_key" \
        --legacy \
        --gas-price "$L2_GAS_PRICE_WEI" \
        --rpc-url "$L2_SEND_RPC_URL" \
        --json)

      tx_hash=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"transactionHash\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" | head -1)
      block_number=$(printf "%s\n" "$receipt" | sed -nE "s/.*\"blockNumber\"[[:space:]]*:[[:space:]]*\"?([^\",}]+)\"?.*/\1/p" | head -1)
      echo "$tx_hash" | grep -qE "^0x[a-fA-F0-9]{64}$" || { echo "[bridge-smoke-l2-to-l1] ERROR: cast receipt did not include transactionHash" >&2; printf "%s\n" "$receipt" >&2; exit 1; }
      [ -n "$block_number" ] || block_number="unknown"

      printf "[bridge-smoke-l2-to-l1] sender=%s\n" "$sender"
      printf "[bridge-smoke-l2-to-l1] l2Tx=%s\n" "$tx_hash"
      printf "[bridge-smoke-l2-to-l1] l2Block=%s\n" "$block_number"
    '
)"
printf '%s\n' "$SEND_OUTPUT" | lineth_child_output

L2_SENDER="$(printf '%s\n' "$SEND_OUTPUT" | sed -nE 's/.*sender=(0x[a-fA-F0-9]{40}).*/\1/p' | tail -1)"
L2_TX_HASH="$(printf '%s\n' "$SEND_OUTPUT" | sed -nE 's/.*l2Tx=(0x[a-fA-F0-9]{64}).*/\1/p' | tail -1)"
L2_TX_BLOCK="$(printf '%s\n' "$SEND_OUTPUT" | sed -nE 's/.*l2Block=([^[:space:]]+).*/\1/p' | tail -1)"
require_address "L2 sender" "$L2_SENDER"
require_hash "L2 tx hash" "$L2_TX_HASH"

log "l2TxExplorer: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/tx/$L2_TX_HASH"

section "wait for L1 finality/anchoring"
DEADLINE=$(( $(date +%s) + BRIDGE_SMOKE_TIMEOUT_SECONDS ))
ROW=""
READY=0
while [ "$(date +%s)" -le "$DEADLINE" ]; do
  ROW="$(psql_value "select id,status,message_hash,message_sender,destination,fee,value,message_nonce,calldata,sent_block_number,coalesce(claim_tx_hash,'') from message where id > $START_MESSAGE_ID and direction='L2_TO_L1' and lower(message_sender)=lower('$L2_SENDER') and lower(destination)=lower('$RECIPIENT') and fee='$L2_L1_MESSAGE_FEE_WEI' and value='$L2_L1_MESSAGE_VALUE_WEI' order by id desc limit 1;")"
  if [ -n "$ROW" ]; then
    STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
    case "$STATUS" in
      ANCHORED|ZERO_FEE|CLAIMED_SUCCESS)
        READY=1
        break
        ;;
      PENDING)
        log "Postman already submitted an L1 claim; waiting for CLAIMED_SUCCESS"
        ;;
      NON_EXECUTABLE|CLAIMED_REVERTED|FEE_UNDERPRICED|NEEDS_MANUAL_INTERVENTION)
        printf '%s\n' "$ROW" >&2
        die "postman moved message to terminal/problem status: $STATUS"
        ;;
    esac
  fi
  sleep "$BRIDGE_SMOKE_POLL_SECONDS"
done

[ -n "$ROW" ] || die "timed out waiting for postman to ingest the L2 MessageSent event"
[ "$READY" -eq 1 ] || {
  printf '%s\n' "$ROW" >&2
  die "timed out waiting for the L2->L1 message to become claimable"
}

MESSAGE_ID="$(printf '%s' "$ROW" | cut -d '|' -f 1)"
STATUS="$(printf '%s' "$ROW" | cut -d '|' -f 2)"
MESSAGE_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 3)"
MESSAGE_SENDER="$(printf '%s' "$ROW" | cut -d '|' -f 4)"
DESTINATION="$(printf '%s' "$ROW" | cut -d '|' -f 5)"
MESSAGE_FEE="$(printf '%s' "$ROW" | cut -d '|' -f 6)"
MESSAGE_VALUE="$(printf '%s' "$ROW" | cut -d '|' -f 7)"
MESSAGE_NONCE="$(printf '%s' "$ROW" | cut -d '|' -f 8)"
MESSAGE_CALLDATA="$(printf '%s' "$ROW" | cut -d '|' -f 9)"
SENT_BLOCK_NUMBER="$(printf '%s' "$ROW" | cut -d '|' -f 10)"
CLAIM_TX_HASH="$(printf '%s' "$ROW" | cut -d '|' -f 11)"

require_hash "messageHash" "$MESSAGE_HASH"
require_address "messageSender" "$MESSAGE_SENDER"
require_address "destination" "$DESTINATION"
require_uint "message fee" "$MESSAGE_FEE"
require_uint "message value" "$MESSAGE_VALUE"
require_uint "message nonce" "$MESSAGE_NONCE"
require_uint "sent block number" "$SENT_BLOCK_NUMBER"
echo "$MESSAGE_CALLDATA" | grep -qE '^0x([a-fA-F0-9]{2})*$' || die "message calldata malformed"

log "messageId: $MESSAGE_ID"
log "postmanStatus: $STATUS"
log "messageHash: $MESSAGE_HASH"
log "messageSender: $MESSAGE_SENDER"
log "messageNonce: $MESSAGE_NONCE"
log "sentBlockNumber: $SENT_BLOCK_NUMBER"

if [ "$STATUS" != "CLAIMED_SUCCESS" ]; then
  section "claim on L1"
  CLAIM_OUTPUT=""
  if ! CLAIM_OUTPUT="$(claim_l2_to_l1)"; then
    printf '%s\n' "$CLAIM_OUTPUT" >&2
    die "L1 SDK claim failed"
  fi

  CLAIM_TX_HASH="$(printf '%s\n' "$CLAIM_OUTPUT" | lineth_json_stdin_string_field claimTxHash)"
  PROOF_ROOT="$(printf '%s\n' "$CLAIM_OUTPUT" | lineth_json_stdin_string_field proofRoot)"
  PROOF_LEAF_INDEX="$(printf '%s\n' "$CLAIM_OUTPUT" | lineth_json_stdin_number_field proofLeafIndex)"
  PROOF_LENGTH="$(printf '%s\n' "$CLAIM_OUTPUT" | lineth_json_stdin_number_field proofLength)"
  CLAIMANT="$(printf '%s\n' "$CLAIM_OUTPUT" | lineth_json_stdin_string_field claimant)"
  require_hash "claim tx hash" "$CLAIM_TX_HASH"
  require_hash "proof root" "$PROOF_ROOT"
  require_uint "proof leaf index" "$PROOF_LEAF_INDEX"
  require_uint "proof length" "$PROOF_LENGTH"
  require_address "claimant" "$CLAIMANT"

  log "claimant: $CLAIMANT"
  log "proofRoot: $PROOF_ROOT"
  log "proofLeafIndex: $PROOF_LEAF_INDEX"
  log "proofLength: $PROOF_LENGTH"
  log "l1ClaimTx: $CLAIM_TX_HASH"
fi

require_hash "L1 claim tx" "$CLAIM_TX_HASH"
log "l1ClaimTxLink: $(lineth_l1_tx_link "$CLAIM_TX_HASH")"

section "verify L1 receipt"
RECEIPT_DEADLINE=$(( $(date +%s) + L1_RECEIPT_TIMEOUT_SECONDS ))
CLAIM_RECEIPT=""
while [ "$(date +%s)" -le "$RECEIPT_DEADLINE" ]; do
  CLAIM_RECEIPT="$(rpc_l1 eth_getTransactionReceipt "[\"$CLAIM_TX_HASH\"]")"
  if printf '%s\n' "$CLAIM_RECEIPT" | grep -q '"result":[[:space:]]*{'; then
    break
  fi
  sleep "$BRIDGE_SMOKE_POLL_SECONDS"
done

if ! printf '%s\n' "$CLAIM_RECEIPT" | grep -q '"result":[[:space:]]*{'; then
  POSTMAN_CLAIM_TX_HASH=""
  if POSTMAN_CLAIM_TX_HASH="$(wait_for_postman_claim_tx "$MESSAGE_ID" "$RECEIPT_DEADLINE")" \
    && [ "$POSTMAN_CLAIM_TX_HASH" != "$CLAIM_TX_HASH" ]; then
    log "postman claimed while manual claim raced: $POSTMAN_CLAIM_TX_HASH"
    CLAIM_TX_HASH="$POSTMAN_CLAIM_TX_HASH"
    log "l1ClaimTxLink: $(lineth_l1_tx_link "$CLAIM_TX_HASH")"
    CLAIM_RECEIPT=""
    while [ "$(date +%s)" -le "$RECEIPT_DEADLINE" ]; do
      CLAIM_RECEIPT="$(rpc_l1 eth_getTransactionReceipt "[\"$CLAIM_TX_HASH\"]")"
      if printf '%s\n' "$CLAIM_RECEIPT" | grep -q '"result":[[:space:]]*{'; then
        break
      fi
      sleep "$BRIDGE_SMOKE_POLL_SECONDS"
    done
  fi
fi

printf '%s\n' "$CLAIM_RECEIPT" | grep -q '"status":"0x1"' || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L1 claim receipt missing or failed"
}
printf '%s\n' "$CLAIM_RECEIPT" | grep -qi "$MESSAGE_CLAIMED_TOPIC" || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L1 claim receipt did not emit MessageClaimed"
}
printf '%s\n' "$CLAIM_RECEIPT" | grep -qi "$MESSAGE_HASH" || {
  printf '%s\n' "$CLAIM_RECEIPT" >&2
  die "L1 claim receipt MessageClaimed hash does not match postman message hash"
}

section "success"
log "L2 message tx: http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/tx/$L2_TX_HASH"
[ -n "$L2_TX_BLOCK" ] && log "L2 message block: $L2_TX_BLOCK"
log "L1 claim tx: $(lineth_l1_tx_link "$CLAIM_TX_HASH")"
log "LineaRollupV8: $(lineth_l1_address_link "$LINEA_ROLLUP")"
