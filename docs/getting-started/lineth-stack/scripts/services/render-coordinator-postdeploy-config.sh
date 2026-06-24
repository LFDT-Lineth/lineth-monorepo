#!/bin/sh
set -eu

log() { printf '[runtime-config-finalize] %s\n' "$*"; }
die() { printf '[runtime-config-finalize] FATAL: %s\n' "$*" >&2; exit 1; }

DEPLOY_RUNTIME_ENV="${DEPLOY_RUNTIME_ENV:-/deployments/deploy-runtime.env}"
PREDEPLOY_CONFIG="${PREDEPLOY_CONFIG:-/rendered/coordinator-config.predeploy.toml}"
FINAL_CONFIG="${FINAL_CONFIG:-/rendered/coordinator-config.toml}"

is_hash() { printf '%s\n' "$1" | grep -qE '^0x[a-fA-F0-9]{64}$'; }
is_uint() { printf '%s\n' "$1" | grep -qE '^[0-9]+$'; }

[ -f "$DEPLOY_RUNTIME_ENV" ] || die "$DEPLOY_RUNTIME_ENV missing - deploy-contracts must run first"
[ -f "$PREDEPLOY_CONFIG" ] || die "$PREDEPLOY_CONFIG missing - config-render must run first"

GENESIS_STATE_ROOT_HASH=
GENESIS_SHNARF=
L2_MESSAGE_SERVICE_DEPLOY_BLOCK=
LINEA_ROLLUP_L1_DEPLOY_BLOCK=

while IFS= read -r line || [ -n "$line" ]; do
  case "$line" in
    ''|\#*) continue ;;
    GENESIS_STATE_ROOT_HASH=*) GENESIS_STATE_ROOT_HASH=${line#*=} ;;
    GENESIS_SHNARF=*) GENESIS_SHNARF=${line#*=} ;;
    L2_MESSAGE_SERVICE_DEPLOY_BLOCK=*) L2_MESSAGE_SERVICE_DEPLOY_BLOCK=${line#*=} ;;
    LINEA_ROLLUP_L1_DEPLOY_BLOCK=*) LINEA_ROLLUP_L1_DEPLOY_BLOCK=${line#*=} ;;
    DEPLOY_FORCED_TRANSACTION_GATEWAY=*) ;;
    *=*) die "unknown key in $DEPLOY_RUNTIME_ENV: ${line%%=*}" ;;
    *) die "malformed line in $DEPLOY_RUNTIME_ENV: $line" ;;
  esac
done < "$DEPLOY_RUNTIME_ENV"

: "${GENESIS_STATE_ROOT_HASH:?GENESIS_STATE_ROOT_HASH missing from $DEPLOY_RUNTIME_ENV}"
: "${GENESIS_SHNARF:?GENESIS_SHNARF missing from $DEPLOY_RUNTIME_ENV}"
: "${L2_MESSAGE_SERVICE_DEPLOY_BLOCK:?L2_MESSAGE_SERVICE_DEPLOY_BLOCK missing from $DEPLOY_RUNTIME_ENV}"

is_hash "$GENESIS_STATE_ROOT_HASH" || die "GENESIS_STATE_ROOT_HASH malformed"
is_hash "$GENESIS_SHNARF" || die "GENESIS_SHNARF malformed"
is_uint "$L2_MESSAGE_SERVICE_DEPLOY_BLOCK" || die "L2_MESSAGE_SERVICE_DEPLOY_BLOCK malformed"

tmp="${FINAL_CONFIG}.tmp"
awk \
  -v state_root="$GENESIS_STATE_ROOT_HASH" \
  -v shnarf="$GENESIS_SHNARF" \
  -v deploy_block="$L2_MESSAGE_SERVICE_DEPLOY_BLOCK" \
'
  /^genesis-state-root-hash[[:space:]]*=/ { sub(/".*"/, "\"" state_root "\""); }
  /^genesis-shnarf[[:space:]]*=/          { sub(/".*"/, "\"" shnarf "\""); }
  /^contract-deployment-block-number[[:space:]]*=/ { sub(/=.*/, "= " deploy_block); }
  { print }
' "$PREDEPLOY_CONFIG" > "$tmp"

if grep -qE '__[A-Z0-9_]+__' "$tmp"; then
  echo "[runtime-config-finalize] FATAL: leftover placeholder in $tmp:" >&2
  grep -nE '__[A-Z0-9_]+__' "$tmp" >&2
  exit 1
fi

mv "$tmp" "$FINAL_CONFIG"
log "wrote $FINAL_CONFIG"
