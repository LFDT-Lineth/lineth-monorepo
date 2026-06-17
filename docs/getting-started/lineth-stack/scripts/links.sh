#!/usr/bin/env sh
# Print local quickstart links and L1/L2 explorer URLs from host-backed runtime artifacts.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
LINETH_LOG_CONTEXT="links"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"

section() { lineth_section "$*"; }

lineth_banner "links · local services + explorers"

HOST_PORT_L2_RPC="$(lineth_host_port HOST_PORT_L2_RPC 8745)"
HOST_PORT_L2_BLOCKSCOUT="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT 4000)"
HOST_PORT_L2_BLOCKSCOUT_FRONTEND="$(lineth_host_port HOST_PORT_L2_BLOCKSCOUT_FRONTEND 4001)"
HOST_PORT_POSTMAN="$(lineth_host_port HOST_PORT_POSTMAN 9090)"
HOST_PORT_COORDINATOR="$(lineth_host_port HOST_PORT_COORDINATOR 9545)"

PRE="$(lineth_accounts_file addresses-precomputed.json)"
ADDR="$(lineth_deployments_file addresses.json)"

print_l1() {
  label="$1"
  addr="$2"
  [ -n "$addr" ] || return 0
  lineth_kv "$label" "$(lineth_l1_address_link "$addr")"
}

print_l2() {
  label="$1"
  addr="$2"
  [ -n "$addr" ] || return 0
  lineth_kv "$label" "http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND/address/$addr"
}

section "local services"
if [ "$(lineth_l1_mode)" = "local" ]; then
  lineth_kv "L1 RPC" "$(lineth_l1_host_rpc_url)"
fi
lineth_kv "L2 RPC" "http://localhost:$HOST_PORT_L2_RPC"
lineth_kv "L2 Blockscout UI" "http://localhost:$HOST_PORT_L2_BLOCKSCOUT_FRONTEND"
lineth_kv "L2 Blockscout API" "http://localhost:$HOST_PORT_L2_BLOCKSCOUT"
lineth_kv "Postman API" "http://localhost:$HOST_PORT_POSTMAN"
lineth_kv "Coordinator observability" "http://localhost:$HOST_PORT_COORDINATOR"

if [ -s "$PRE" ]; then
  section "pre-boot contract links"
  print_l1 "LineaRollupV8" "$(lineth_json_section_addr "$PRE" l1 LineaRollupV8)"
  print_l2 "L2MessageService" "$(lineth_json_section_addr "$PRE" l2 L2MessageService)"

  section "runtime signer addresses"
  lineth_kv "L1 blob submitter" "$(lineth_json_section_addr "$PRE" signers l1BlobSubmitterAddress)"
  lineth_kv "L1 finalization" "$(lineth_json_section_addr "$PRE" signers l1FinalizationSubmitterAddress)"
  lineth_kv "L1 postman" "$(lineth_json_section_addr "$PRE" signers l1PostmanAddress)"
  lineth_kv "L2 deployer" "$(lineth_json_section_addr "$PRE" signers l2DeployerAddress)"
  lineth_kv "L2 anchorer" "$(lineth_json_section_addr "$PRE" signers l2MessageAnchoringAddress)"
  lineth_kv "L2 postman" "$(lineth_json_section_addr "$PRE" signers l2PostmanAddress)"
else
  section "pre-boot contract links"
  lineth_warn "addresses-precomputed.json missing"
fi

if [ -s "$ADDR" ]; then
  section "deployed L1 contracts"
  print_l1 "LineaRollupV8" "$(lineth_json_section_addr "$ADDR" l1 LineaRollupV8)"
  print_l1 "TokenBridge" "$(lineth_json_section_addr "$ADDR" l1 TokenBridge)"
  print_l1 "ERC20Example" "$(lineth_json_section_addr "$ADDR" l1 ERC20Example)"
  print_l1 "ForcedTransactionGateway" "$(lineth_json_section_addr "$ADDR" l1 ForcedTransactionGateway)"

  section "deployed L2 contracts"
  print_l2 "L2MessageService" "$(lineth_json_section_addr "$ADDR" l2 L2MessageService)"
  print_l2 "TokenBridge" "$(lineth_json_section_addr "$ADDR" l2 TokenBridge)"
  print_l2 "ERC20Example" "$(lineth_json_section_addr "$ADDR" l2 ERC20Example)"
else
  section "deployed contracts"
  lineth_warn "addresses.json missing; deploy-contracts has not completed yet"
fi
