#!/bin/sh
set -eu

log() { printf '[postman-config-render] %s\n' "$*"; }
die() { printf '[postman-config-render] FATAL: %s\n' "$*" >&2; exit 1; }

PRECOMPUTED="${PRECOMPUTED:-/accounts/addresses-precomputed.json}"
POSTMAN_ENV="${POSTMAN_ENV:-/postman-runtime/postman.env}"
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

is_addr() { printf '%s\n' "$1" | grep -qE '^0x[a-fA-F0-9]{40}$'; }
is_pubkey() { printf '%s\n' "$1" | grep -qE '^0x[a-fA-F0-9]{128}$'; }
is_uint() { printf '%s\n' "$1" | grep -qE '^[0-9]+$'; }

[ -f "$PRECOMPUTED" ] || die "$PRECOMPUTED missing - account-setup must run first"
json_address() {
  key="$1"
  sed -nE "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"(0x[a-fA-F0-9]{40})\".*/\1/p" "$PRECOMPUTED" | head -1
}

json_uint() {
  key="$1"
  sed -nE "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"?([0-9]+)\"?.*/\1/p" "$PRECOMPUTED" | head -1
}

L1_CONTRACT_ADDRESS=$(json_address "LineaRollupV8")
L2_CONTRACT_ADDRESS=$(json_address "L2MessageService")
L1_LISTENER_INITIAL_FROM_BLOCK=$(json_uint "l1PostmanListenerStartBlock")
L1_WEB3_SIGNER_PUBLIC_KEY=$(sed -nE 's/.*"l1PostmanPubkey":[[:space:]]*"(0x[a-fA-F0-9]{128})".*/\1/p' "$PRECOMPUTED" | head -1)
L2_WEB3_SIGNER_PUBLIC_KEY=$(sed -nE 's/.*"l2PostmanPubkey":[[:space:]]*"(0x[a-fA-F0-9]{128})".*/\1/p' "$PRECOMPUTED" | head -1)

is_addr "$L1_CONTRACT_ADDRESS" || die "L1_CONTRACT_ADDRESS missing from $PRECOMPUTED"
is_addr "$L2_CONTRACT_ADDRESS" || die "L2_CONTRACT_ADDRESS missing from $PRECOMPUTED"
is_uint "$L1_LISTENER_INITIAL_FROM_BLOCK" || die "l1PostmanListenerStartBlock missing from $PRECOMPUTED"
is_pubkey "$L1_WEB3_SIGNER_PUBLIC_KEY" || die "l1PostmanPubkey missing from $PRECOMPUTED"
is_pubkey "$L2_WEB3_SIGNER_PUBLIC_KEY" || die "l2PostmanPubkey missing from $PRECOMPUTED"

mkdir -p "$(dirname "$POSTMAN_ENV")"
tmp="${POSTMAN_ENV}.tmp"
umask 077
{
  printf "L1_RPC_URL='%s'\n" "$L1_RPC_URL"
  printf "L1_CONTRACT_ADDRESS='%s'\n" "$L1_CONTRACT_ADDRESS"
  printf "L2_CONTRACT_ADDRESS='%s'\n" "$L2_CONTRACT_ADDRESS"
  printf "L1_LISTENER_INITIAL_FROM_BLOCK='%s'\n" "$L1_LISTENER_INITIAL_FROM_BLOCK"
  printf "L1_SIGNER_TYPE='web3signer'\n"
  printf "L2_SIGNER_TYPE='web3signer'\n"
  printf "L1_WEB3_SIGNER_ENDPOINT='https://web3signer:9000'\n"
  printf "L2_WEB3_SIGNER_ENDPOINT='https://web3signer:9000'\n"
  printf "L1_WEB3_SIGNER_PUBLIC_KEY='%s'\n" "$L1_WEB3_SIGNER_PUBLIC_KEY"
  printf "L2_WEB3_SIGNER_PUBLIC_KEY='%s'\n" "$L2_WEB3_SIGNER_PUBLIC_KEY"
  printf "L1_WEB3_SIGNER_TLS_KEYSTORE_PATH='/tls-files/postman-client-keystore.p12'\n"
  printf "L1_WEB3_SIGNER_TLS_KEYSTORE_PASSWORD='changeit'\n"
  printf "L1_WEB3_SIGNER_TLS_TRUSTSTORE_PATH='/tls-files/web3signer-truststore.p12'\n"
  printf "L1_WEB3_SIGNER_TLS_TRUSTSTORE_PASSWORD='changeit'\n"
  printf "L2_WEB3_SIGNER_TLS_KEYSTORE_PATH='/tls-files/postman-client-keystore.p12'\n"
  printf "L2_WEB3_SIGNER_TLS_KEYSTORE_PASSWORD='changeit'\n"
  printf "L2_WEB3_SIGNER_TLS_TRUSTSTORE_PATH='/tls-files/web3signer-truststore.p12'\n"
  printf "L2_WEB3_SIGNER_TLS_TRUSTSTORE_PASSWORD='changeit'\n"
} > "$tmp"
mv "$tmp" "$POSTMAN_ENV"
chmod 0644 "$POSTMAN_ENV" || die "failed to chmod $POSTMAN_ENV"

log "wrote $POSTMAN_ENV with precomputed contract addresses and Web3Signer Postman signer config"
