#!/usr/bin/env sh
#
# 01-generate-accounts.sh — thin launcher for the TypeScript account/key setup.
#
# Shell stays as Docker glue. Wallet generation, encrypted ethers keystores,
# L1 RPC checks, Sepolia gas-cap checks, and address precomputation live in
# account-setup.ts.
#
set -eu

ts() { date -u '+%Y-%m-%dT%H:%M:%SZ'; }
log() { printf "[account-setup] %s timestamp=%s\n" "$*" "$(ts)"; }
die() { printf "[account-setup] ERROR: %s timestamp=%s\n" "$*" "$(ts)" >&2; exit 1; }

WORKSPACE_DIR="${WORKSPACE_DIR:-/workspace}"
CONTRACTS_DIR="${CONTRACTS_DIR:-$WORKSPACE_DIR/contracts}"

[ -d "$WORKSPACE_DIR" ] || die "workspace dir not mounted at $WORKSPACE_DIR"
[ -d "$CONTRACTS_DIR" ] || die "contracts dir not mounted at $CONTRACTS_DIR"

log "Preparing Node/ethers account setup runtime"
corepack enable >/dev/null 2>&1 || true
corepack prepare pnpm@10.32.1 --activate >/dev/null 2>&1 || true

if [ ! -x "$WORKSPACE_DIR/node_modules/.bin/ts-node" ] \
  || [ ! -d "$WORKSPACE_DIR/node_modules/.pnpm" ] \
  || [ ! -e "$CONTRACTS_DIR/node_modules/ethers" ]; then
  log "Installing minimal workspace dependencies for TypeScript account setup"
  (
    cd "$WORKSPACE_DIR"
    HUSKY=0 CI=true pnpm install --filter linea-monorepo --filter contracts... --no-frozen-lockfile --prefer-offline
  )
fi

cd "$CONTRACTS_DIR"
export NODE_PATH="$WORKSPACE_DIR/node_modules:$CONTRACTS_DIR/node_modules${NODE_PATH:+:$NODE_PATH}"
TS_NODE_TRANSPILE_ONLY=1 \
TS_NODE_COMPILER_OPTIONS='{"module":"CommonJS","moduleResolution":"Node"}' \
  pnpm -s exec ts-node /scripts/internal/account-setup.ts
