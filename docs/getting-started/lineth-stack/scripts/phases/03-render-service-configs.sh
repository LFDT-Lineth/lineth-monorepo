#!/bin/sh
set -eu

log() { printf '[config-render] %s\n' "$*"; }
die() { printf '[config-render] FATAL: %s\n' "$*" >&2; exit 1; }

L1_MODE="${L1_MODE:-sepolia}"
case "$L1_MODE" in
  sepolia)
    L1_RPC_URL="${L1_RPC_URL:-}"
    [ -n "$L1_RPC_URL" ] || die "L1_RPC_URL must be set or provided by L1_MODE=local"
    ;;
  local)
    L1_RPC_URL="http://l1-el-node:8545"
    ;;
  *)
    die "L1_MODE must be one of sepolia, local (got '$L1_MODE')"
    ;;
esac

LINEA_COORDINATOR_DATA_AVAILABILITY="${LINEA_COORDINATOR_DATA_AVAILABILITY:-ROLLUP}"
if [ "$LINEA_COORDINATOR_DATA_AVAILABILITY" != "ROLLUP" ]; then
  die "LINEA_COORDINATOR_DATA_AVAILABILITY=$LINEA_COORDINATOR_DATA_AVAILABILITY is not supported by this quickstart; use ROLLUP"
fi

PRECOMPUTED="${PRECOMPUTED:-/accounts/addresses-precomputed.json}"
if [ ! -f "$PRECOMPUTED" ]; then
  die "$PRECOMPUTED not found - account-setup must run first"
fi

# Extract addresses from the precomputed JSON. account-setup writes a controlled
# shape so POSIX sed -E is enough.
LINEA_ROLLUP_ADDR=$(sed -nE 's/.*"LineaRollupV8":[[:space:]]*"(0x[a-fA-F0-9]{40})".*/\1/p' "$PRECOMPUTED" | head -1)
L2_MS_ADDR=$(sed -nE 's/.*"L2MessageService":[[:space:]]*"(0x[a-fA-F0-9]{40})".*/\1/p' "$PRECOMPUTED" | head -1)

case "${L2_CHAIN_ID:-}" in
  ''|*[!0-9]*)
    die "L2_CHAIN_ID must be a decimal integer (got '${L2_CHAIN_ID:-}')"
    ;;
esac

case "${PROVER_DEV_OVERRIDE:-}" in
  true|false) ;;
  *)
    die "PROVER_DEV_OVERRIDE must be true or false (got '${PROVER_DEV_OVERRIDE:-}')"
    ;;
esac

case "${L1_DYNAMIC_GAS_PRICE_CAP_DISABLED:-}" in
  true|false) ;;
  *)
    die "L1_DYNAMIC_GAS_PRICE_CAP_DISABLED must be true or false (got '${L1_DYNAMIC_GAS_PRICE_CAP_DISABLED:-}')"
    ;;
esac

for gas_spec in \
  "L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI=${L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI:-}" \
  "L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI=${L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI:-}" \
  "L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=${L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI:-}" \
  "L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI=${L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI:-}" \
  "L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI=${L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI:-}"; do
  gas_var=${gas_spec%%=*}
  gas_value=${gas_spec#*=}
  case "$gas_value" in
    ''|*[!0-9]*)
      die "$gas_var must be a decimal wei value (got '$gas_value')"
      ;;
  esac
done

# Uncompressed secp256k1 pubkeys (128 hex chars, no 0x prefix). The L1 blob and
# finalization signers are deliberately separate senders; message anchoring signs
# L2 transactions, so it uses the L2 anchorer.
L1_BLOB_SUBMITTER_PUBKEY=$(sed -nE 's/.*"l1BlobSubmitterPubkey":[[:space:]]*"0x([a-fA-F0-9]{128})".*/\1/p' "$PRECOMPUTED" | head -1)
L1_FINALIZATION_SUBMITTER_PUBKEY=$(sed -nE 's/.*"l1FinalizationSubmitterPubkey":[[:space:]]*"0x([a-fA-F0-9]{128})".*/\1/p' "$PRECOMPUTED" | head -1)
L2_MESSAGE_ANCHORING_PUBKEY=$(sed -nE 's/.*"l2MessageAnchoringPubkey":[[:space:]]*"0x([a-fA-F0-9]{128})".*/\1/p' "$PRECOMPUTED" | head -1)

echo "$LINEA_ROLLUP_ADDR" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "LineaRollupV8 missing from $PRECOMPUTED"
echo "$L2_MS_ADDR" | grep -qE '^0x[a-fA-F0-9]{40}$' || die "L2MessageService missing from $PRECOMPUTED"
echo "$L1_BLOB_SUBMITTER_PUBKEY" | grep -qE '^[a-fA-F0-9]{128}$' || die "signers.l1BlobSubmitterPubkey missing from $PRECOMPUTED"
echo "$L1_FINALIZATION_SUBMITTER_PUBKEY" | grep -qE '^[a-fA-F0-9]{128}$' || die "signers.l1FinalizationSubmitterPubkey missing from $PRECOMPUTED"
echo "$L2_MESSAGE_ANCHORING_PUBKEY" | grep -qE '^[a-fA-F0-9]{128}$' || die "signers.l2MessageAnchoringPubkey missing from $PRECOMPUTED"

log "L1_MODE = $L1_MODE"
log "L1_RPC_URL is set (redacted)"
log "L2_CHAIN_ID = $L2_CHAIN_ID"
log "LineaRollupV8 = $LINEA_ROLLUP_ADDR"
log "L2MessageService = $L2_MS_ADDR"
log "L1 blob signer pubkey = ${L1_BLOB_SUBMITTER_PUBKEY%????????????????????????????????????????????????????????}..."
log "L1 finalization signer pubkey = ${L1_FINALIZATION_SUBMITTER_PUBKEY%????????????????????????????????????????????????????????}..."
log "L2 anchoring signer pubkey = ${L2_MESSAGE_ANCHORING_PUBKEY%????????????????????????????????????????????????????????}..."

# Deferred placeholders are seeded with zero defaults in the predeploy render.
# runtime-config-finalize writes the final coordinator config after deploy.
ZERO_HASH=0x0000000000000000000000000000000000000000000000000000000000000000

export L1_RPC_URL L2_CHAIN_ID LINEA_ROLLUP_ADDR L2_MS_ADDR
export L1_BLOB_SUBMITTER_PUBKEY L1_FINALIZATION_SUBMITTER_PUBKEY L2_MESSAGE_ANCHORING_PUBKEY
export L1_DYNAMIC_GAS_PRICE_CAP_DISABLED
export L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI
export L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI
export L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI
export PROVER_DEV_OVERRIDE PROVER_GOMEMLIMIT ZERO_HASH

/scripts/services/render-coordinator-config.sh
/scripts/services/render-maru-config.sh
/scripts/services/render-sequencer-config.sh
/scripts/services/render-l2-node-besu-config.sh
/scripts/services/render-prover-config.sh

log "service configs rendered"
