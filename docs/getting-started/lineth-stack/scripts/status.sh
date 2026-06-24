#!/usr/bin/env sh
# Print a redacted, milestone-oriented status report for the Lineth quickstart.
# Run from docs/getting-started/lineth-stack while the stack is booting.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="status"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"

ACCOUNTS_DIR="$LINETH_ACCOUNTS_DIR"
DEPLOYMENTS_DIR="$LINETH_DEPLOYMENTS_DIR"
CONFIG_DIR="$LINETH_CONFIG_DIR"
L1_MODE="$(lineth_l1_mode)"

section() { lineth_section "$*"; }

if [ "$L1_MODE" = "local" ]; then
  lineth_banner "status · local L2 + local L1 finality"
else
  lineth_banner "status · local L2 + Sepolia finality"
fi

print_deploy_progress() {
  deploy_state="$(docker inspect -f '{{.State.Status}} {{.State.ExitCode}}' deploy-contracts 2>/dev/null || true)"
  [ -n "$deploy_state" ] || return 0

  case "$deploy_state" in
    running\ *)
      lineth_info "deploy-contracts is still running; latest useful deploy log lines:"
      ;;
    created\ *)
      lineth_info "deploy-contracts is created; waiting for dependencies"
      ;;
    exited\ 0)
      lineth_info "deploy-contracts completed; waiting for addresses.json to appear"
      ;;
    exited\ *)
      lineth_error "deploy-contracts $deploy_state; latest failure context:"
      ;;
    *)
      lineth_info "deploy-contracts state: $deploy_state"
      ;;
  esac

  docker logs --tail 180 deploy-contracts 2>&1 \
      | grep -E '\[deploy-contracts\] =====|\[deploy-contracts\] (L1 deployer balance|L1 deployer required minimum|L2_GENESIS|Compiling contracts|L1_NONCE|verify |Forwarding |Captured |Funding |wrote |ERROR:|Done|addresses\.json)|^contract=.* (pending:|deployed:)|insufficient funds|Cannot fund|balance too low' \
    | tail -25 \
    | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
    | lineth_indent || true
}

check_contract_code() {
  scope="$1"
  url="$2"
  label="$3"
  address="$4"

  [ -n "$address" ] || return 0
  if [ -z "$url" ]; then
    lineth_warn "$scope code check skipped for $label: RPC unavailable"
    STATE_MISMATCH=1
    return 0
  fi

  code_resp="$(lineth_rpc_json "$url" eth_getCode "[\"$address\",\"latest\"]")"
  code="$(printf '%s\n' "$code_resp" | lineth_json_stdin_string_field result)"
  if [ -z "$code_resp" ] || [ -z "$code" ]; then
    lineth_warn "$scope $label code check unavailable at $address"
    return 0
  fi
  case "$code" in
    "0x")
      lineth_error "$scope $label has no code at $address"
      if [ "$scope" = "L1" ] && [ "${L1_MODE:-sepolia}" = "local" ]; then
        lineth_warn "local L1 artifacts point to an address without code; run ./scripts/reset.sh before rebooting"
      fi
      STATE_MISMATCH=1
      ;;
    *)
      lineth_kv "$scope $label code" "ok ($address)"
      ;;
  esac
}

if ! docker info >/dev/null 2>&1; then
  lineth_die "Docker daemon is not reachable from this shell."
fi

STATE_MISMATCH=0

section "containers"
docker ps -a --format 'table {{.Names}}\t{{.Status}}' \
  | awk 'NR == 1 || /^(l1-node-genesis-generator|l1-el-node|l1-cl-node|account-setup|config-render|postman-config-render|l2-genesis-init|sequencer|l2-node-besu|shomei|deploy-contracts|runtime-config-finalize|coordinator|prover|postman|web3signer|maru|l2-blockscout)[[:space:]]/' \
  | lineth_indent

BOOT_FAILURE=0
for init_container in account-setup config-render postman-config-render l2-genesis-init deploy-contracts runtime-config-finalize; do
  init_state="$(docker inspect -f '{{.State.Status}} {{.State.ExitCode}}' "$init_container" 2>/dev/null || true)"
  case "$init_state" in
    exited\ 0|running\ 0|created\ 0|"")
      ;;
    exited\ *)
      if [ "$BOOT_FAILURE" -eq 0 ]; then
        section "boot failure"
      fi
      BOOT_FAILURE=1
      lineth_error "$init_container $init_state"
      docker logs --tail 80 "$init_container" 2>&1 \
        | grep -E 'ERROR|Error:|error code|insufficient funds|Failed|FATAL|ADDRESS MISMATCH|balance too low|Cannot fund|service .*didn.t complete' \
        | tail -20 \
        | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
        | lineth_indent || true
      ;;
  esac
done

section "deploy milestones"
if [ ! -d "$ACCOUNTS_DIR" ] && [ ! -d "$DEPLOYMENTS_DIR" ]; then
  lineth_warn "host artifacts missing: run ./scripts/bootstrap-artifacts.sh before boot"
else
  ADDRESSES_JSON_PRESENT=0
  if lineth_artifact_exists addresses.json; then
    ADDRESSES_JSON_PRESENT=1
  fi

  if [ "$ADDRESSES_JSON_PRESENT" -ne 1 ]; then
    print_deploy_progress
  fi

  {
    if [ -f "$ACCOUNTS_DIR/addresses-precomputed.json" ]; then
      echo "addresses-precomputed.json: present"
    else
      echo "addresses-precomputed.json: missing"
    fi

    if [ -f "$DEPLOYMENTS_DIR/addresses.json" ]; then
      echo "addresses.json: present"
      sed -nE "s/.*\"LineaRollupV8\":[[:space:]]*\"(0x[a-fA-F0-9]{40})\".*/L1 LineaRollupV8: \1/p" "$DEPLOYMENTS_DIR/addresses.json" | head -1
      sed -nE "s/.*\"TokenBridge\":[[:space:]]*\"(0x[a-fA-F0-9]{40})\".*/TokenBridge: \1/p" "$DEPLOYMENTS_DIR/addresses.json" | head -2
      sed -nE "s/.*\"ERC20Example\":[[:space:]]*\"(0x[a-fA-F0-9]{40})\".*/ERC20Example: \1/p" "$DEPLOYMENTS_DIR/addresses.json" | head -2
    else
      echo "addresses.json: missing"
    fi

    for f in "$DEPLOYMENTS_DIR"/deploy-logs/*.log; do
      [ -f "$f" ] || continue
      name="${f##*/}"
      count=$(grep -c "^contract=.* deployed:" "$f" || true)
      if [ "$count" -gt 0 ]; then
        echo "$name: $count contract deploy log(s)"
        grep -E "^contract=.* deployed:|\[deploy-contracts\] verify .*: OK|ERROR|ADDRESS MISMATCH" "$f" | tail -12
      else
        echo "$name: present, no deploy marker yet"
      fi
    done
  } | lineth_indent
fi

section "chain / prover config"
HOST_PORT_L2_RPC="$(lineth_host_port HOST_PORT_L2_RPC 8745)"
L2_RPC_URL="${L2_RPC_URL:-http://localhost:$HOST_PORT_L2_RPC}"
chain_response="$(lineth_rpc_json "$L2_RPC_URL" eth_chainId '[]')"
local_l2_chain_hex="$(printf '%s\n' "$chain_response" | lineth_json_stdin_string_field result)"
local_l2_chain_dec=""
if [ -n "$chain_response" ]; then
  lineth_kv "l2 rpc eth_chainId" "$chain_response"
  if [ -n "$local_l2_chain_hex" ]; then
    local_l2_chain_dec="$(lineth_hex_to_dec_small "$local_l2_chain_hex")"
  fi
else
  lineth_warn "l2 rpc eth_chainId unavailable at $L2_RPC_URL"
fi

local_l2_latest_block_dec=""
block_response="$(lineth_rpc_json "$L2_RPC_URL" eth_blockNumber '[]')"
local_l2_latest_block_hex="$(printf '%s\n' "$block_response" | lineth_json_stdin_string_field result)"
if [ -n "$local_l2_latest_block_hex" ] && [ "$local_l2_latest_block_hex" != "0x" ]; then
  local_l2_latest_block_dec="$(lineth_hex_to_dec_small "$local_l2_latest_block_hex")"
  lineth_kv "l2 rpc latest block" "$local_l2_latest_block_dec ($local_l2_latest_block_hex)"
else
  lineth_warn "l2 rpc latest block unavailable at $L2_RPC_URL"
fi

if [ -f "$DEPLOYMENTS_DIR/addresses.json" ]; then
  sed -nE 's/.*"l2ChainId":[[:space:]]*"?([0-9]+)"?.*/addresses.json l2ChainId: \1/p' "$DEPLOYMENTS_DIR/addresses.json" \
    | head -1 \
    | lineth_indent
fi

PROVER_CONFIG="$CONFIG_DIR/prover/prover-config-partial.toml"
if [ -f "$PROVER_CONFIG" ]; then
  awk '
    /^\[[^]]+\]/ {
      section = $0
      gsub(/^\[/, "", section)
      gsub(/\]$/, "", section)
      next
    }
    /^prover_mode[[:space:]]*=/ ||
    /^is_allowed_circuit_id[[:space:]]*=/ ||
    /^chain_id[[:space:]]*=/ {
      printf "%s.%s\n", section, $0
    }
  ' "$PROVER_CONFIG" | lineth_indent
else
  lineth_warn "rendered prover config missing"
fi

section "state mismatch guardrails"
# ADDRESSES_L2_CHAIN_ID and L2_DEPLOY_MAX_BLOCK are only assigned inside the
# guards below, so they need empty defaults for the set -u reads that follow.
ADDRESSES_L2_CHAIN_ID=""
L2_DEPLOY_MAX_BLOCK=""
L2_MESSAGE_SERVICE_ADDRESS="$(lineth_artifact_section_addr addresses.json l2 L2MessageService)"
L2_TOKEN_BRIDGE_ADDRESS="$(lineth_artifact_section_addr addresses.json l2 TokenBridge)"
L2_ERC20_ADDRESS="$(lineth_artifact_section_addr addresses.json l2 ERC20Example)"
L1_TOKEN_BRIDGE_ADDRESS="$(lineth_artifact_section_addr addresses.json l1 TokenBridge)"
L1_ERC20_ADDRESS="$(lineth_artifact_section_addr addresses.json l1 ERC20Example)"
if [ -f "$DEPLOYMENTS_DIR/addresses.json" ]; then
  ADDRESSES_L2_CHAIN_ID="$(sed -nE 's/.*"l2ChainId"[[:space:]]*:[[:space:]]*"?([0-9]+)"?.*/\1/p' "$DEPLOYMENTS_DIR/addresses.json" | head -1)"
fi
if [ -d "$DEPLOYMENTS_DIR/deploy-logs" ]; then
  L2_DEPLOY_MAX_BLOCK="$(
    for f in "$DEPLOYMENTS_DIR"/deploy-logs/*.log; do
      [ -f "$f" ] || continue
      awk '/contract=.* deployed:.*chainId=1337/ {
        if (match($0, /blockNumber=[0-9]+/)) {
          v=substr($0, RSTART + 12, RLENGTH - 12)
          if (v > max) max = v
        }
      } END { if (max > 0) print max }' "$f"
    done | awk 'BEGIN { max=0 } { if ($1 > max) max=$1 } END { if (max > 0) print max }'
  )"
fi

if [ -n "$ADDRESSES_L2_CHAIN_ID" ] && [ -n "$local_l2_chain_dec" ]; then
  if [ "$ADDRESSES_L2_CHAIN_ID" = "$local_l2_chain_dec" ]; then
    lineth_kv "addresses.json vs local chainId" "ok ($ADDRESSES_L2_CHAIN_ID)"
  else
    lineth_error "addresses.json l2ChainId=$ADDRESSES_L2_CHAIN_ID but local eth_chainId=$local_l2_chain_dec"
    STATE_MISMATCH=1
  fi
else
  lineth_info "addresses.json/local chainId comparison unavailable"
fi

if [ -n "$L2_DEPLOY_MAX_BLOCK" ] && lineth_is_uint "$L2_DEPLOY_MAX_BLOCK" && lineth_is_uint "$local_l2_latest_block_dec"; then
  if [ "$L2_DEPLOY_MAX_BLOCK" -le "$local_l2_latest_block_dec" ]; then
    lineth_kv "L2 deploy-log block floor" "ok (max deployed block $L2_DEPLOY_MAX_BLOCK <= local latest $local_l2_latest_block_dec)"
  else
    lineth_error "deploy logs reference L2 block $L2_DEPLOY_MAX_BLOCK but local latest block is $local_l2_latest_block_dec"
    STATE_MISMATCH=1
  fi
else
  lineth_info "L2 deploy-log block floor unavailable"
fi

check_contract_code "L2" "$L2_RPC_URL" "L2MessageService" "$L2_MESSAGE_SERVICE_ADDRESS"
check_contract_code "L2" "$L2_RPC_URL" "TokenBridge" "$L2_TOKEN_BRIDGE_ADDRESS"
check_contract_code "L2" "$L2_RPC_URL" "ERC20Example" "$L2_ERC20_ADDRESS"

section "l1 data availability vs finalization"
lineth_kv "L1 mode" "$L1_MODE"
LINEA_ROLLUP_ADDRESS="$(lineth_artifact_section_addr addresses.json l1 LineaRollupV8)"

if [ -n "$LINEA_ROLLUP_ADDRESS" ]; then
  lineth_kv "LineaRollupV8" "$LINEA_ROLLUP_ADDRESS"
else
  lineth_warn "LineaRollupV8 unavailable until addresses.json exists"
fi

latest_blob_tx=""
latest_finalization_tx=""
latest_finalization_window=""
if docker ps -a --format '{{.Names}}' | grep -qx coordinator; then
  latest_blob_tx="$(docker logs --tail 4000 coordinator 2>&1 \
    | sed -nE 's/.*blobs submitted:.*transactionHash=(0x[a-fA-F0-9]{64}).*/\1/p' \
    | tail -1 || true)"
  latest_finalization_tx="$(docker logs --tail 4000 coordinator 2>&1 \
    | sed -nE 's/.*submitted aggregation=[^ ]+ transactionHash=(0x[a-fA-F0-9]{64}).*/\1/p' \
    | tail -1 || true)"
  latest_finalization_window="$(docker logs --tail 4000 coordinator 2>&1 \
    | sed -nE 's/.*submitted aggregation=([^ ]+) transactionHash=0x[a-fA-F0-9]{64}.*/\1/p' \
    | tail -1 || true)"
fi

if [ -n "$latest_blob_tx" ]; then
  lineth_kv "latest blob tx (DA only)" "$latest_blob_tx"
else
  lineth_info "latest blob tx (DA only): none seen in coordinator logs yet"
fi

if [ -n "$latest_finalization_tx" ]; then
  if [ -n "$latest_finalization_window" ]; then
    lineth_kv "latest finalization tx" "$latest_finalization_tx aggregation=$latest_finalization_window"
  else
    lineth_kv "latest finalization tx" "$latest_finalization_tx"
  fi
else
  lineth_info "latest finalization tx: none seen in coordinator logs yet"
fi

L1_RPC_URL="$(lineth_l1_host_rpc_url)"
if [ "$L1_MODE" = "local" ] && [ -n "$L1_RPC_URL" ]; then
  l1_chain_response="$(lineth_rpc_json "$L1_RPC_URL" eth_chainId '[]')"
  l1_chain_hex="$(printf '%s\n' "$l1_chain_response" | lineth_json_stdin_string_field result)"
  if [ -n "$l1_chain_hex" ]; then
    lineth_kv "local L1 eth_chainId" "$(lineth_hex_to_dec_small "$l1_chain_hex") ($l1_chain_hex)"
  else
    lineth_warn "local L1 eth_chainId unavailable at $L1_RPC_URL"
  fi
fi
if [ -n "$L1_RPC_URL" ] && [ -n "$LINEA_ROLLUP_ADDRESS" ]; then
  check_contract_code "L1" "$L1_RPC_URL" "LineaRollupV8" "$LINEA_ROLLUP_ADDRESS"
  check_contract_code "L1" "$L1_RPC_URL" "TokenBridge" "$L1_TOKEN_BRIDGE_ADDRESS"
  check_contract_code "L1" "$L1_RPC_URL" "ERC20Example" "$L1_ERC20_ADDRESS"

  l2_finalized_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_call "[{\"to\":\"$LINEA_ROLLUP_ADDRESS\",\"data\":\"0x695378f5\"},\"latest\"]")"
  l2_finalized_hex="$(printf '%s\n' "$l2_finalized_resp" | lineth_json_stdin_string_field result)"
  if [ -n "$l2_finalized_hex" ] && [ "$l2_finalized_hex" != "0x" ]; then
    rollup_current_l2_block_dec="$(lineth_hex_to_dec_small "$l2_finalized_hex")"
    lineth_kv "rollup currentL2BlockNumber" "$rollup_current_l2_block_dec ($l2_finalized_hex)"
    if lineth_is_uint "$rollup_current_l2_block_dec" \
      && lineth_is_uint "$local_l2_latest_block_dec" \
      && [ "$rollup_current_l2_block_dec" -gt "$local_l2_latest_block_dec" ]; then
      lineth_warn "rollup finalized block is ahead of local L2 latest block."
      lineth_warn "local chain state does not match the preserved L1 rollup state; run ./scripts/reset.sh for a clean boot."
      STATE_MISMATCH=1
    fi
  else
    lineth_warn "rollup currentL2BlockNumber unavailable"
  fi

  if [ -n "$latest_finalization_tx" ]; then
    finalization_tx_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_getTransactionByHash "[\"$latest_finalization_tx\"]")"
    finalization_input="$(printf '%s\n' "$finalization_tx_resp" | lineth_json_stdin_string_field input)"
    finalization_selector="$(printf '%.10s' "$finalization_input")"
    if [ "$finalization_selector" = "0x755bc62f" ]; then
      lineth_kv "latest finalization method" "finalizeBlocks(bytes,uint256,tuple) selector=$finalization_selector"
    elif [ -n "$finalization_selector" ]; then
      lineth_warn "latest finalization method unexpected selector=$finalization_selector"
    else
      lineth_warn "latest finalization method unavailable"
    fi

    finalization_receipt_resp="$(lineth_rpc_json "$L1_RPC_URL" eth_getTransactionReceipt "[\"$latest_finalization_tx\"]")"
    finalization_status="$(printf '%s\n' "$finalization_receipt_resp" | lineth_json_stdin_string_field status)"
    case "$finalization_receipt_resp" in
      *a0262dc79e4ccb71ceac8574ae906311ae338aa4a2044fd4ec4b99fad5ab60cb*) data_finalized="yes" ;;
      *) data_finalized="no" ;;
    esac
    case "$finalization_receipt_resp" in
      *32e016ccc5c33419c35caa94023fdeb75143da613fb2ac738ab736404c09fc5d*) state_updated="yes" ;;
      *) state_updated="no" ;;
    esac
    if [ -n "$finalization_status" ]; then
      lineth_kv "latest finalization receipt" "status=$finalization_status DataFinalizedV3=$data_finalized FinalizedStateUpdated=$state_updated"
    else
      lineth_warn "latest finalization receipt unavailable"
    fi
  fi
else
  lineth_info "Sepolia rollup state check skipped: L1_RPC_URL or LineaRollupV8 unavailable"
fi

if [ "$STATE_MISMATCH" -ne 0 ]; then
  section "state mismatch action"
  lineth_error "Local L2 state and preserved Sepolia artifact state do not belong together."
  lineth_info "Stop debugging this boot; reset with:"
  lineth_info "./scripts/reset.sh"
elif [ -n "$LINEA_ROLLUP_ADDRESS" ] || [ -n "$ADDRESSES_L2_CHAIN_ID" ]; then
  section "state mismatch action"
  lineth_ok "no local/L1 state mismatch detected"
else
  section "state mismatch action"
  lineth_info "guardrails pending until addresses.json exists"
fi

section "coordinator"
if docker ps --format '{{.Names}}' | grep -qx coordinator; then
  docker exec coordinator sh -eu -c '
    if grep -qi ":2549" /proc/net/tcp /proc/net/tcp6 2>/dev/null; then echo "observability 9545: listening"; else echo "observability 9545: not listening"; fi
    if grep -qi ":254A" /proc/net/tcp /proc/net/tcp6 2>/dev/null; then echo "json-rpc 9546: listening"; else echo "json-rpc 9546: not listening"; fi
    for lane in execution compression invalidity aggregation; do
      req_dir="/data/prover/v3/$lane/requests"
      resp_dir="/data/prover/v3/$lane/responses"
      if [ -d "$req_dir" ]; then
        requests=$(find "$req_dir" -type f 2>/dev/null | wc -l | tr -d " ")
        failed=$(find "$req_dir" -type f -name "*.failure.*" 2>/dev/null | wc -l | tr -d " ")
        inprogress=$(find "$req_dir" -type f -name "*.inprogress.*" 2>/dev/null | wc -l | tr -d " ")
      else
        requests="missing"; failed="?"; inprogress="?"
      fi
      if [ -d "$resp_dir" ]; then
        responses=$(find "$resp_dir" -type f 2>/dev/null | wc -l | tr -d " ")
      else
        responses="missing"
      fi
      echo "prover $lane: requests=$requests responses=$responses failed=$failed inprogress=$inprogress"
    done
  ' | lineth_indent
  docker logs --tail 200 coordinator 2>&1 \
    | grep -E 'Rollup finalized block updated|execution proof request generated|blob compression proof generated|blobs to submit|blob submission failed|max fee per gas less than block base fee|aggregation proof|submitted|finalized|ERROR|WARN' \
    | tail -25 \
    | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
    | lineth_indent || true
else
  lineth_warn "coordinator not running"
fi

section "prover"
if docker ps --format '{{.Names}}' | grep -qx prover; then
  docker stats --no-stream --format 'prover resources: mem={{.MemUsage}} cpu={{.CPUPerc}}' prover 2>/dev/null \
    | lineth_indent || true
  docker logs --tail 120 prover 2>&1 \
    | grep -E 'ERROR|WARN|Running the|Chain config|IsAllowedCircuitID|proof|request|response|done|completed|failed' \
    | tail -25 \
    | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
    | lineth_indent || true
else
  lineth_warn "prover not running"
fi
