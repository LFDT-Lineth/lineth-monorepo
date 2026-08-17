#!/bin/sh
set -eu

log() { printf '[render-prover-config] %s\n' "$*"; }
die() { printf '[render-prover-config] FATAL: %s\n' "$*" >&2; exit 1; }

src="${PROVER_TEMPLATE:-/templates/prover-config-partial.toml.template}"
dst="${PROVER_CONFIG:-/rendered/prover/prover-config-partial.toml}"
tmp="${dst}.tmp"

mkdir -p "$(dirname "$dst")"
sed \
  -e "s|__L2_CHAIN_ID__|$L2_CHAIN_ID|g" \
  -e "s|__L2_MESSAGE_SERVICE_ADDRESS__|$L2_MS_ADDR|g" \
  "$src" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[render-prover-config] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$dst"
log "rendered $src -> $dst"

if [ "$PROVER_DEV_OVERRIDE" = "true" ]; then
  log "PROVER_DEV_OVERRIDE=true -> patching $dst to all-dev mode"
  sed -i \
    -e '/^\[execution\]/,/^\[/ { s/^prover_mode = "partial"/prover_mode = "dev"/; }' \
    -e '/^\[invalidity\]/,/^\[/ { s/^prover_mode = "partial"/prover_mode = "dev"/; }' \
    -e '/^\[execution\]/,/^\[/ { /^conflated_traces_dir = /d; /^ignore_compatibility_check = /d; /^serialization = /d; }' \
    -e 's/^is_allowed_circuit_id = 483$/is_allowed_circuit_id = 963/' \
    "$dst"
  dev_count=$(grep -cE '^prover_mode = "dev"$' "$dst" || true)
  if [ "$dev_count" != "4" ] || ! grep -qE '^is_allowed_circuit_id = 963$' "$dst" \
    || ! grep -qE "^chain_id = $L2_CHAIN_ID$" "$dst"; then
    echo "[render-prover-config] FATAL: PROVER_DEV_OVERRIDE patch failed - got $dev_count dev modes, expected 4" >&2
    grep -E '^prover_mode|^is_allowed_circuit_id|^chain_id' "$dst" >&2
    exit 1
  fi
  log "dev override applied: 4x prover_mode=dev, is_allowed_circuit_id=963"
else
  if [ -z "${PROVER_GOMEMLIMIT:-}" ]; then
    die "PROVER_GOMEMLIMIT must be set explicitly when PROVER_DEV_OVERRIDE=false"
  fi
  log "PROVER_DEV_OVERRIDE=false -> keeping upstream partial defaults in rendered config"
  partial_count=$(grep -cE '^prover_mode = "partial"$' "$dst" || true)
  dev_count=$(grep -cE '^prover_mode = "dev"$' "$dst" || true)
  if [ "$partial_count" != "2" ] || [ "$dev_count" != "2" ] \
    || ! grep -qE '^is_allowed_circuit_id = 483$' "$dst" \
    || ! grep -qE "^chain_id = $L2_CHAIN_ID$" "$dst"; then
    echo "[render-prover-config] FATAL: partial prover render mismatch - got partial=$partial_count dev=$dev_count" >&2
    grep -E '^prover_mode|^is_allowed_circuit_id|^chain_id' "$dst" >&2
    exit 1
  fi
  log "partial render verified: 2x partial, 2x dev, is_allowed_circuit_id=483, GOMEMLIMIT=$PROVER_GOMEMLIMIT"
fi
