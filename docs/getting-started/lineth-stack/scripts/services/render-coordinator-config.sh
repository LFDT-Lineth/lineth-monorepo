#!/bin/sh
set -eu

log() { printf '[render-coordinator-config] %s\n' "$*"; }
die() { printf '[render-coordinator-config] FATAL: %s\n' "$*" >&2; exit 1; }

src="${COORDINATOR_TEMPLATE:-/templates/coordinator-config.toml.template}"
dst="${COORDINATOR_PREDEPLOY_CONFIG:-/rendered/coordinator/coordinator-config.predeploy.toml}"
tmp="${dst}.tmp"
zero_hash="${ZERO_HASH:-0x0000000000000000000000000000000000000000000000000000000000000000}"

mkdir -p "$(dirname "$dst")"
sed \
  -e "s|__L1_RPC_URL__|$L1_RPC_URL|g" \
  -e "s|__L2_CHAIN_ID__|$L2_CHAIN_ID|g" \
  -e "s|__LINEA_ROLLUP_ADDRESS__|$LINEA_ROLLUP_ADDR|g" \
  -e "s|__L2_MESSAGE_SERVICE_ADDRESS__|$L2_MS_ADDR|g" \
  -e "s|__L1_BLOB_SUBMITTER_PUBKEY_NO_PREFIX__|$L1_BLOB_SUBMITTER_PUBKEY|g" \
  -e "s|__L1_FINALIZATION_SUBMITTER_PUBKEY_NO_PREFIX__|$L1_FINALIZATION_SUBMITTER_PUBKEY|g" \
  -e "s|__L2_MESSAGE_ANCHORING_PUBKEY_NO_PREFIX__|$L2_MESSAGE_ANCHORING_PUBKEY|g" \
  -e "s|__L1_DYNAMIC_GAS_PRICE_CAP_DISABLED__|$L1_DYNAMIC_GAS_PRICE_CAP_DISABLED|g" \
  -e "s|__L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI__|$L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI|g" \
  -e "s|__L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI__|$L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI|g" \
  -e "s|__L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI__|$L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI|g" \
  -e "s|__L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI__|$L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI|g" \
  -e "s|__L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI__|$L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI|g" \
  -e "s|__GENESIS_STATE_ROOT_HASH__|$zero_hash|g" \
  -e "s|__GENESIS_SHNARF__|$zero_hash|g" \
  -e "s|__LINEA_ROLLUP_DEPLOY_BLOCK__|0|g" \
  "$src" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[render-coordinator-config] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$dst"
log "rendered $src -> $dst"
