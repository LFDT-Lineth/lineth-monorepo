#!/usr/bin/env sh
# Collect a local support bundle from the Lineth quickstart runtime state.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="export-output"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"

section() { lineth_section "$*"; }

lineth_banner "support · collect quickstart report"

OUTPUT_DIR="${LINETH_OUTPUT_DIR:-$LINETH_STACK_DIR/lineth-output}"
ACCOUNTS_DIR="$LINETH_ACCOUNTS_DIR"
DEPLOYMENTS_DIR="$LINETH_DEPLOYMENTS_DIR"

latest_log_hash() {
  pattern="$1"
  docker logs --tail 4000 coordinator 2>&1 \
    | sed -nE "$pattern" \
    | tail -1 || true
}

psql_json() {
  query="$1"
  if docker ps --format '{{.Names}}' | grep -qx postman-pg; then
    docker exec postman-pg psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postman}" -At -c "$query" 2>/dev/null || printf '[]'
  else
    printf '[]'
  fi
}

if ! docker info >/dev/null 2>&1; then
  lineth_die "Docker daemon is not reachable."
fi

mkdir -p "$OUTPUT_DIR"

section "prepare output folder"
rm -f \
  "$OUTPUT_DIR/addresses-precomputed.json" \
  "$OUTPUT_DIR/addresses.json" \
  "$OUTPUT_DIR/deploy-runtime.env" \
  "$OUTPUT_DIR/deploy-timing.jsonl" \
  "$OUTPUT_DIR/links.json" \
  "$OUTPUT_DIR/finality-report.json" \
  "$OUTPUT_DIR/smoke-report.json"
rm -rf "$OUTPUT_DIR/deploy-logs"
mkdir -p "$OUTPUT_DIR/deploy-logs"
lineth_ok "clean support bundle folder ready at $OUTPUT_DIR"

section "copy deploy facts"
cp "$ACCOUNTS_DIR/addresses-precomputed.json" "$OUTPUT_DIR/addresses-precomputed.json" 2>/dev/null || true
cp "$DEPLOYMENTS_DIR/addresses.json" "$OUTPUT_DIR/addresses.json" 2>/dev/null || true
cp "$DEPLOYMENTS_DIR/deploy-runtime.env" "$OUTPUT_DIR/deploy-runtime.env" 2>/dev/null || true
cp "$DEPLOYMENTS_DIR/deploy-timing.jsonl" "$OUTPUT_DIR/deploy-timing.jsonl" 2>/dev/null || true
cp "$DEPLOYMENTS_DIR"/deploy-logs/*.log "$OUTPUT_DIR/deploy-logs/" 2>/dev/null || true
if [ -s "$OUTPUT_DIR/addresses.json" ]; then
  lineth_ok "copied deploy facts into $OUTPUT_DIR"
else
  lineth_warn "addresses.json missing; boot the stack before exporting deployed addresses"
fi

PRE="$OUTPUT_DIR/addresses-precomputed.json"
ADDR="$OUTPUT_DIR/addresses.json"

HOST_PORT_L2_RPC="$(lineth_host_port HOST_PORT_L2_RPC 8745)"
HOST_PORT_L2_BLOCKSCOUT="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT 4000)"
HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"
HOST_PORT_POSTMAN="$(lineth_host_port HOST_PORT_POSTMAN 9090)"
HOST_PORT_COORDINATOR="$(lineth_host_port HOST_PORT_COORDINATOR 9545)"
L2_RPC_URL="${L2_RPC_URL:-http://localhost:$HOST_PORT_L2_RPC}"
L1_MODE="$(lineth_l1_mode)"
L1_RPC_URL="$(lineth_l1_host_rpc_url)"

LINEA_ROLLUP="$(lineth_json_section_addr "$ADDR" l1 LineaRollupV8 || true)"
L1_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l1 TokenBridge || true)"
L1_ERC20="$(lineth_json_section_addr "$ADDR" l1 ERC20Example || true)"
L2_MESSAGE_SERVICE="$(lineth_json_section_addr "$ADDR" l2 L2MessageService || true)"
L2_TOKEN_BRIDGE="$(lineth_json_section_addr "$ADDR" l2 TokenBridge || true)"
L2_ERC20="$(lineth_json_section_addr "$ADDR" l2 ERC20Example || true)"

section "write links.json"
cat > "$OUTPUT_DIR/links.json" <<EOF
{
  "local": {
    "l1Mode": $(lineth_json_value "$L1_MODE"),
    "l1Rpc": $(lineth_json_value "$L1_RPC_URL"),
    "l2Rpc": "http://localhost:$HOST_PORT_L2_RPC",
    "l2BlockscoutUi": "http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND",
    "l2BlockscoutApi": "http://localhost:$HOST_PORT_L2_BLOCKSCOUT",
    "postmanApi": "http://localhost:$HOST_PORT_POSTMAN",
    "coordinatorObservability": "http://localhost:$HOST_PORT_COORDINATOR"
  },
  "l1": {
    "LineaRollupV8": $(lineth_json_value "$(lineth_l1_address_link "$LINEA_ROLLUP")"),
    "TokenBridge": $(lineth_json_value "$(lineth_l1_address_link "$L1_TOKEN_BRIDGE")"),
    "ERC20Example": $(lineth_json_value "$(lineth_l1_address_link "$L1_ERC20")")
  },
  "l2": {
    "L2MessageService": $(lineth_json_value "${L2_MESSAGE_SERVICE:+http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_MESSAGE_SERVICE}"),
    "TokenBridge": $(lineth_json_value "${L2_TOKEN_BRIDGE:+http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_TOKEN_BRIDGE}"),
    "ERC20Example": $(lineth_json_value "${L2_ERC20:+http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$L2_ERC20}")
  }
}
EOF
lineth_ok "$OUTPUT_DIR/links.json"

section "write finality-report.json"
local_chain_resp="$(lineth_rpc_json "$L2_RPC_URL" eth_chainId '[]')"
local_chain_hex="$(printf '%s\n' "$local_chain_resp" | lineth_json_stdin_string_field result)"
local_block_resp="$(lineth_rpc_json "$L2_RPC_URL" eth_blockNumber '[]')"
local_block_hex="$(printf '%s\n' "$local_block_resp" | lineth_json_stdin_string_field result)"
if [ -n "$local_block_hex" ]; then
  local_block_dec="$(lineth_hex_to_dec_small "$local_block_hex")"
else
  local_block_dec=""
fi
addresses_chain_id="$(lineth_json_root_value "$ADDR" l2ChainId || true)"

latest_blob_tx=""
latest_finalization_tx=""
latest_finalization_window=""
if docker ps -a --format '{{.Names}}' | grep -qx coordinator; then
  latest_blob_tx="$(latest_log_hash 's/.*blobs submitted:.*transactionHash=(0x[a-fA-F0-9]{64}).*/\1/p')"
  latest_finalization_tx="$(latest_log_hash 's/.*submitted aggregation=[^ ]+ transactionHash=(0x[a-fA-F0-9]{64}).*/\1/p')"
  latest_finalization_window="$(latest_log_hash 's/.*submitted aggregation=([^ ]+) transactionHash=0x[a-fA-F0-9]{64}.*/\1/p')"
fi

rollup_block_hex=""
rollup_block_dec=""
finalization_selector=""
finalization_receipt_status=""
data_finalized="false"
state_updated="false"
state_mismatch="false"

if [ -n "$L1_RPC_URL" ] && [ -n "$LINEA_ROLLUP" ]; then
  rollup_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_call "[{\"to\":\"$LINEA_ROLLUP\",\"data\":\"0x695378f5\"},\"latest\"]")"
  rollup_block_hex="$(printf '%s\n' "$rollup_resp" | lineth_json_stdin_string_field result)"
  if [ -n "$rollup_block_hex" ]; then
    rollup_block_dec="$(lineth_hex_to_dec_small "$rollup_block_hex")"
  else
    rollup_block_dec=""
  fi

  if [ -n "$latest_finalization_tx" ]; then
    tx_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_getTransactionByHash "[\"$latest_finalization_tx\"]")"
    finalization_input="$(printf '%s\n' "$tx_resp" | lineth_json_stdin_string_field input)"
    finalization_selector="$(printf '%.10s' "$finalization_input")"

    receipt_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_getTransactionReceipt "[\"$latest_finalization_tx\"]")"
    finalization_receipt_status="$(printf '%s\n' "$receipt_resp" | lineth_json_stdin_string_field status)"
    case "$receipt_resp" in
      *a0262dc79e4ccb71ceac8574ae906311ae338aa4a2044fd4ec4b99fad5ab60cb*) data_finalized="true" ;;
    esac
    case "$receipt_resp" in
      *32e016ccc5c33419c35caa94023fdeb75143da613fb2ac738ab736404c09fc5d*) state_updated="true" ;;
    esac
  fi
fi

case "$rollup_block_dec:$local_block_dec" in
  *[!0-9]*:*) ;;
  *:*[!0-9]*) ;;
  :*) ;;
  *:) ;;
  *)
    if [ "$rollup_block_dec" -gt "$local_block_dec" ]; then
      state_mismatch="true"
    fi
    ;;
esac

cat > "$OUTPUT_DIR/finality-report.json" <<EOF
{
  "localL2": {
    "rpcUrl": $(lineth_json_value "$L2_RPC_URL"),
    "chainIdHex": $(lineth_json_value "$local_chain_hex"),
    "latestBlockHex": $(lineth_json_value "$local_block_hex"),
    "latestBlockNumber": $(lineth_json_value "$local_block_dec")
  },
  "addresses": {
    "l2ChainId": $(lineth_json_value "$addresses_chain_id"),
    "lineaRollupV8": $(lineth_json_value "$LINEA_ROLLUP")
  },
  "l1": {
    "mode": $(lineth_json_value "$L1_MODE"),
    "rpcUrl": $(lineth_json_value "$L1_RPC_URL"),
    "latestBlobTxHash": $(lineth_json_value "$latest_blob_tx"),
    "latestFinalizationTxHash": $(lineth_json_value "$latest_finalization_tx"),
    "latestFinalizationWindow": $(lineth_json_value "$latest_finalization_window"),
    "rollupCurrentL2BlockHex": $(lineth_json_value "$rollup_block_hex"),
    "rollupCurrentL2BlockNumber": $(lineth_json_value "$rollup_block_dec"),
    "latestFinalizationSelector": $(lineth_json_value "$finalization_selector"),
    "latestFinalizationReceiptStatus": $(lineth_json_value "$finalization_receipt_status"),
    "dataFinalizedV3": $(lineth_json_bool "$data_finalized"),
    "finalizedStateUpdated": $(lineth_json_bool "$state_updated")
  },
  "stateMismatch": $(lineth_json_bool "$state_mismatch")
}
EOF
lineth_ok "$OUTPUT_DIR/finality-report.json"

section "write smoke-report.json"
POSTMAN_SUMMARY="$(psql_json "select coalesce(json_agg(row_to_json(t))::text,'[]') from (select direction,status,count(*)::int as count from message group by direction,status order by direction,status) t;")"
POSTMAN_LATEST="$(psql_json "select coalesce(json_agg(row_to_json(t))::text,'[]') from (select id,direction,status,message_hash,coalesce(claim_tx_hash,'') as claim_tx_hash,message_sender,destination,value,message_nonce from message order by id desc limit 10) t;")"
GENERATED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

cat > "$OUTPUT_DIR/smoke-report.json" <<EOF
{
  "generatedAt": $(lineth_json_value "$GENERATED_AT"),
  "source": "postman database",
  "postmanMessageSummary": $POSTMAN_SUMMARY,
  "latestPostmanMessages": $POSTMAN_LATEST,
  "smokeCommands": [
    "./scripts/smoke-test/smoke-bridge-message.sh",
    "./scripts/smoke-test/smoke-bridge-message-l2-to-l1.sh",
    "./scripts/smoke-test/smoke-bridge-erc20-l1-to-l2.sh",
    "./scripts/smoke-test/smoke-bridge-erc20-l2-to-l1.sh"
  ]
}
EOF
lineth_ok "$OUTPUT_DIR/smoke-report.json"

section "done"
lineth_kv "support bundle" "$OUTPUT_DIR"
lineth_info "use this only when sharing run evidence or debugging; it is not required for normal demos"
