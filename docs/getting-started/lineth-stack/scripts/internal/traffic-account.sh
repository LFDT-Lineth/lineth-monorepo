#!/usr/bin/env bash
set -euo pipefail

mode="${1:-}"
case "$mode" in
  ensure|require-existing) ;;
  *) echo "[traffic-account] ERROR: usage: traffic-account.sh <ensure|require-existing>" >&2; exit 1 ;;
esac

RUNTIME_KEYS_ENV="${RUNTIME_KEYS_ENV:-/accounts/runtime-keys.env}"
DEMO_TRAFFIC_ENV="${DEMO_TRAFFIC_ENV:-/accounts/demo-traffic.env}"

[[ -f "$RUNTIME_KEYS_ENV" ]] || { echo "[traffic-account] ERROR: $RUNTIME_KEYS_ENV missing; boot the stack first" >&2; exit 1; }

export RUNTIME_KEYS_ENV DEMO_TRAFFIC_ENV
export L2_READ_RPC_URL="${L2_READ_RPC_URL:-${L2_RPC_URL:-http://l2-node-besu:8545}}"
export L2_SEND_RPC_URL="${L2_SEND_RPC_URL:-http://sequencer:8545}"
export L2_GAS_PRICE_WEI="${L2_GAS_PRICE_WEI:-100000000}"
export L2_TRAFFIC_ETH_MIN_BALANCE_WEI="${L2_TRAFFIC_ETH_MIN_BALANCE_WEI:-100000000000000000}"
export L2_TRAFFIC_ETH_TOP_UP_WEI="${L2_TRAFFIC_ETH_TOP_UP_WEI:-1000000000000000000}"
export L2_TRAFFIC_ERC20_MIN_BALANCE_WEI="${L2_TRAFFIC_ERC20_MIN_BALANCE_WEI:-100}"
export L2_TRAFFIC_ERC20_TOP_UP_WEI="${L2_TRAFFIC_ERC20_TOP_UP_WEI:-10000}"
export TRAFFIC_ERC20_ADDRESS="${TRAFFIC_ERC20_ADDRESS:-}"
export L2_TRAFFIC_PRIVATE_KEY="${L2_TRAFFIC_PRIVATE_KEY:-}"
export L2_WITHDRAW_PRIVATE_KEY="${L2_WITHDRAW_PRIVATE_KEY:-}"

cd /workspace/contracts
export NODE_PATH="/workspace/node_modules:/workspace/contracts/node_modules${NODE_PATH:+:$NODE_PATH}"
export TS_NODE_TRANSPILE_ONLY=1
export TS_NODE_COMPILER_OPTIONS='{"module":"CommonJS","moduleResolution":"Node"}'

corepack enable >/dev/null 2>&1 || true
corepack prepare pnpm@10.32.1 --activate >/dev/null 2>&1 || true

pnpm -s exec ts-node /scripts/internal/traffic-account.ts "$mode"
