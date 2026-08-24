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
DEPLOYER_IMAGE_DIGEST="$(docker image inspect --format '{{.Id}}' "$DEPLOYER_IMAGE")"

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

get_nonce() {
  local container="$1"
  local address="$2"
  docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    nonce --rpc-url "http://${container}:8545" "$address"
}

INITIAL_L1_NONCE="$(get_nonce "$L1_CONTAINER" "$L1_ADDRESS")"
INITIAL_L2_NONCE="$(get_nonce "$L2_CONTAINER" "$L2_ADDRESS")"

chmod 0777 "$CHECKPOINT_DIR"
run_deployer() {
  docker run --rm --network "$NETWORK" \
    -v "$CHECKPOINT_DIR:/checkpoint" \
    -e "L1_RPC_URL=http://${L1_CONTAINER}:8545" \
    -e "L2_RPC_URL=http://${L2_CONTAINER}:8545" \
    -e "L1_DEPLOYER_PRIVATE_KEY=$L1_KEY" \
    -e "L2_DEPLOYER_PRIVATE_KEY=$L2_KEY" \
    -e "L1_STARTING_NONCE=$INITIAL_L1_NONCE" \
    -e "L2_STARTING_NONCE=$INITIAL_L2_NONCE" \
    -e "INITIAL_L2_STATE_ROOT_HASH=$STATE_ROOT" \
    -e "DEPLOYER_IMAGE_DIGEST=$DEPLOYER_IMAGE_DIGEST" \
    -e "L1_DEPLOY_GAS_PRICE_WEI=0" \
    -e "L2_DEPLOY_GAS_PRICE_WEI=0" \
    -e "CHECKPOINT_FILE=/checkpoint/checkpoint.json" \
    "$DEPLOYER_IMAGE"
}

run_deployer
L1_NONCE_BEFORE="$(get_nonce "$L1_CONTAINER" "$L1_ADDRESS")"
L2_NONCE_BEFORE="$(get_nonce "$L2_CONTAINER" "$L2_ADDRESS")"

test "$L1_NONCE_BEFORE" -eq "$((INITIAL_L1_NONCE + 10))"
test "$L2_NONCE_BEFORE" -eq "$((INITIAL_L2_NONCE + 8))"
test "$(jq '.completedSteps | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 4
test "$(jq '.deployments | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 18
test "$(jq '.schemaVersion' "$CHECKPOINT_DIR/checkpoint.json")" -eq 2
test "$(jq -r '.artifactDigest' "$CHECKPOINT_DIR/checkpoint.json")" = "$DEPLOYER_IMAGE_DIGEST"
test "$(jq '.inFlightDeployments | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 0

while IFS=$'\t' read -r chain address; do
  if [[ "$chain" == "l1" ]]; then
    container="$L1_CONTAINER"
  else
    container="$L2_CONTAINER"
  fi
  code="$(docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    code --rpc-url "http://${container}:8545" "$address")"
  if [[ "$code" == "0x" ]]; then
    echo "expected bytecode for ${chain} deployment at ${address}" >&2
    exit 1
  fi
done < <(jq -r '.expectedDeployments[] | [.chain, .expectedAddress] | @tsv' "$CHECKPOINT_DIR/checkpoint.json")

cp "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.before-rerun.json"

run_deployer
L1_NONCE_AFTER="$(get_nonce "$L1_CONTAINER" "$L1_ADDRESS")"
L2_NONCE_AFTER="$(get_nonce "$L2_CONTAINER" "$L2_ADDRESS")"

test "$L1_NONCE_BEFORE" = "$L1_NONCE_AFTER"
test "$L2_NONCE_BEFORE" = "$L2_NONCE_AFTER"
cmp "$CHECKPOINT_DIR/checkpoint.before-rerun.json" "$CHECKPOINT_DIR/checkpoint.json"

mv "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.saved.json"
set +e
CHECKPOINT_LOSS_OUTPUT="$(run_deployer 2>&1)"
CHECKPOINT_LOSS_STATUS=$?
set -e
test "$CHECKPOINT_LOSS_STATUS" -ne 0
[[ "$CHECKPOINT_LOSS_OUTPUT" == *"no checkpoint and L1 signer nonce"* ]]
test "$(get_nonce "$L1_CONTAINER" "$L1_ADDRESS")" = "$L1_NONCE_AFTER"
test "$(get_nonce "$L2_CONTAINER" "$L2_ADDRESS")" = "$L2_NONCE_AFTER"
mv "$CHECKPOINT_DIR/checkpoint.saved.json" "$CHECKPOINT_DIR/checkpoint.json"

echo "two-chain deployment, zero-transaction rerun, and checkpoint-loss refusal verified"
