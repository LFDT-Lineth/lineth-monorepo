#!/usr/bin/env bash
set -euo pipefail

lane="${1:-}"
case "$lane" in
  l1|l2) ;;
  *) echo "[demo-erc20] ERROR: usage: ensure-demo-erc20.sh <l1|l2>" >&2; exit 1 ;;
esac

RUNTIME_KEYS_ENV="${RUNTIME_KEYS_ENV:-/accounts/runtime-keys.env}"
ADDRESSES_PATH="${ADDRESSES_PATH:-/deployments/addresses.json}"

[[ -f "$ADDRESSES_PATH" ]] || { echo "[demo-erc20] ERROR: $ADDRESSES_PATH missing; boot the stack first" >&2; exit 1; }
[[ -f "$RUNTIME_KEYS_ENV" ]] || { echo "[demo-erc20] ERROR: $RUNTIME_KEYS_ENV missing; boot the stack first" >&2; exit 1; }

# shellcheck disable=SC1090
source "$RUNTIME_KEYS_ENV"

export ADDRESSES_PATH
export L2_RPC_URL="${L2_RPC_URL:-http://l2-node-besu:8545}"
export LINETH_STACK_DIR="${LINETH_STACK_DIR:-/workspace/docs/getting-started/lineth-stack}"
export LINETH_ACCOUNTS_DIR="${LINETH_ACCOUNTS_DIR:-/accounts}"
L1_MODE="${L1_MODE:-sepolia}"
case "$L1_MODE" in
  local)
    L1_RPC_URL="http://l1-el-node:8545"
    ;;
  sepolia)
    : "${L1_RPC_URL:?L1_RPC_URL must be set or provided by L1_MODE=local}"
    ;;
  *)
    echo "[demo-erc20] ERROR: L1_MODE must be one of sepolia, local (got '$L1_MODE')" >&2
    exit 1
    ;;
esac
: "${L2_DEPLOYER_PRIVATE_KEY:?L2_DEPLOYER_PRIVATE_KEY missing from $RUNTIME_KEYS_ENV}"
export L1_MODE L1_RPC_URL L2_RPC_URL L2_DEPLOYER_PRIVATE_KEY ADDRESSES_PATH
export L1_DEPLOYER_KEYSTORE_PATH L1_DEPLOYER_KEYSTORE_PASSWORD L1_DEPLOYER_KEYSTORE_PASSWORD_FILE

cd /workspace/contracts
export NODE_PATH="/workspace/node_modules:/workspace/contracts/node_modules${NODE_PATH:+:$NODE_PATH}"
export TS_NODE_TRANSPILE_ONLY=1
export TS_NODE_COMPILER_OPTIONS='{"module":"CommonJS","moduleResolution":"Node"}'

corepack enable >/dev/null 2>&1 || true
corepack prepare pnpm@10.32.1 --activate >/dev/null 2>&1 || true

pnpm -s exec ts-node /scripts/internal/ensure-demo-erc20.ts "$lane"
