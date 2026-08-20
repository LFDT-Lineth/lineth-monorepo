#!/usr/bin/env bash
set -euo pipefail

FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry@sha256:2dbf3d0fc58593ad9d01ef57677f93f83f4987acd295d17f303448d82e3a3ae7}"
DEPLOYER_IMAGE="${DEPLOYER_IMAGE:-consensys/lineth-contract-deployer:local}"
REPOSITORY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
NETWORK="forge-deployer-test-$$"
CHECKPOINT_DIR="$(mktemp -d)"
L1_CONTAINER="forge-deployer-l1-$$"
L2_CONTAINER="forge-deployer-l2-$$"
L1_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
# Deliberately not one of Anvil's funded accounts. A zero-fee L2 must accept
# deployments from this zero-balance signer.
L2_KEY="0x0000000000000000000000000000000000000000000000000000000000000001"
STATE_ROOT="0xabababababababababababababababababababababababababababababababab"

cleanup() {
  docker rm -f "$L1_CONTAINER" "$L2_CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
  rm -rf "$CHECKPOINT_DIR"
}
trap cleanup EXIT

if ! docker image inspect "$DEPLOYER_IMAGE" >/dev/null 2>&1; then
  make -C "$REPOSITORY_ROOT" docker-build-contract-deployer
fi

docker network create "$NETWORK" >/dev/null
docker run -d --name "$L1_CONTAINER" --network "$NETWORK" --entrypoint anvil "$FOUNDRY_IMAGE" \
  --host 0.0.0.0 --chain-id 31337 --base-fee 0 --gas-price 0 >/dev/null
docker run -d --name "$L2_CONTAINER" --network "$NETWORK" --entrypoint anvil "$FOUNDRY_IMAGE" \
  --host 0.0.0.0 --chain-id 1337 --base-fee 0 --gas-price 0 >/dev/null

wait_for_rpc() {
  local container="$1"
  for _ in $(seq 1 30); do
    if docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
      block-number --rpc-url "http://${container}:8545" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "${container} RPC did not become ready" >&2
  return 1
}

wait_for_rpc "$L1_CONTAINER"
wait_for_rpc "$L2_CONTAINER"
L1_ADDRESS="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" wallet address "$L1_KEY")"
L2_ADDRESS="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" wallet address "$L2_KEY")"

chmod 0777 "$CHECKPOINT_DIR"
run_deployer() {
  docker run --rm --network "$NETWORK" \
    -v "$CHECKPOINT_DIR:/checkpoint" \
    -e "L1_RPC_URL=http://${L1_CONTAINER}:8545" \
    -e "L2_RPC_URL=http://${L2_CONTAINER}:8545" \
    -e "L1_DEPLOYER_PRIVATE_KEY=$L1_KEY" \
    -e "L2_DEPLOYER_PRIVATE_KEY=$L2_KEY" \
    -e "INITIAL_L2_STATE_ROOT_HASH=$STATE_ROOT" \
    -e "L1_DEPLOY_GAS_PRICE_WEI=0" \
    -e "L2_DEPLOY_GAS_PRICE_WEI=0" \
    -e "CHECKPOINT_FILE=/checkpoint/checkpoint.json" \
    "$DEPLOYER_IMAGE"
}

run_deployer
L1_NONCE_BEFORE="$(docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
  nonce --rpc-url "http://${L1_CONTAINER}:8545" "$L1_ADDRESS")"
L2_NONCE_BEFORE="$(docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
  nonce --rpc-url "http://${L2_CONTAINER}:8545" "$L2_ADDRESS")"

run_deployer
L1_NONCE_AFTER="$(docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
  nonce --rpc-url "http://${L1_CONTAINER}:8545" "$L1_ADDRESS")"
L2_NONCE_AFTER="$(docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
  nonce --rpc-url "http://${L2_CONTAINER}:8545" "$L2_ADDRESS")"

test "$L1_NONCE_BEFORE" = "$L1_NONCE_AFTER"
test "$L2_NONCE_BEFORE" = "$L2_NONCE_AFTER"
echo "two-chain deployment and zero-transaction rerun verified"
