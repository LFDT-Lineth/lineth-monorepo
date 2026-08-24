#!/usr/bin/env bash
set -euo pipefail

FOUNDRY_IMAGE="${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry@sha256:2dbf3d0fc58593ad9d01ef57677f93f83f4987acd295d17f303448d82e3a3ae7}"
DEPLOYER_IMAGE="${DEPLOYER_IMAGE:-consensys/lineth-contract-deployer:local}"
REPOSITORY_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
L1_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
# Deliberately not one of Anvil's funded accounts. A zero-fee L2 must accept
# deployments from this zero-balance signer.
L2_KEY="0x0000000000000000000000000000000000000000000000000000000000000001"
STATE_ROOT="0xabababababababababababababababababababababababababababababababab"

NETWORK=""
CHECKPOINT_DIR=""
L1_CONTAINER=""
L2_CONTAINER=""
DEPLOYER_CONTAINER=""

cleanup_scenario() {
  if [[ -n "$DEPLOYER_CONTAINER" ]]; then
    docker rm -f "$DEPLOYER_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [[ -n "$L1_CONTAINER" || -n "$L2_CONTAINER" ]]; then
    docker rm -f "$L1_CONTAINER" "$L2_CONTAINER" >/dev/null 2>&1 || true
  fi
  if [[ -n "$NETWORK" ]]; then
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
  fi
  if [[ -n "$CHECKPOINT_DIR" && -d "$CHECKPOINT_DIR" && "$(basename "$CHECKPOINT_DIR")" == tmp.* ]]; then
    rm -rf "$CHECKPOINT_DIR"
  fi
  NETWORK=""
  CHECKPOINT_DIR=""
  L1_CONTAINER=""
  L2_CONTAINER=""
  DEPLOYER_CONTAINER=""
}
trap cleanup_scenario EXIT

if ! docker image inspect "$DEPLOYER_IMAGE" >/dev/null 2>&1; then
  make -C "$REPOSITORY_ROOT" docker-build-contract-deployer
fi
DEPLOYER_IMAGE_DIGEST="$(docker image inspect --format '{{.Id}}' "$DEPLOYER_IMAGE")"

wait_for_rpc() {
  local container="$1"
  for _ in $(seq 1 60); do
    if docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
      block-number --rpc-url "http://${container}:8545" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "${container} RPC did not become ready" >&2
  return 1
}

start_scenario() {
  local label="$1"
  cleanup_scenario
  NETWORK="forge-deployer-${label}-$$"
  CHECKPOINT_DIR="$(mktemp -d)"
  L1_CONTAINER="forge-deployer-${label}-l1-$$"
  L2_CONTAINER="forge-deployer-${label}-l2-$$"
  DEPLOYER_CONTAINER="forge-deployer-${label}-runner-$$"

  docker network create "$NETWORK" >/dev/null
  docker run -d --name "$L1_CONTAINER" --network "$NETWORK" --entrypoint anvil "$FOUNDRY_IMAGE" \
    --host 0.0.0.0 --chain-id 31337 --base-fee 0 --gas-price 0 >/dev/null
  docker run -d --name "$L2_CONTAINER" --network "$NETWORK" --entrypoint anvil "$FOUNDRY_IMAGE" \
    --host 0.0.0.0 --chain-id 1337 --base-fee 0 --gas-price 0 >/dev/null
  wait_for_rpc "$L1_CONTAINER"
  wait_for_rpc "$L2_CONTAINER"

  L1_ADDRESS="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" wallet address "$L1_KEY")"
  L2_ADDRESS="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" wallet address "$L2_KEY")"
  INITIAL_L1_NONCE="$(get_nonce latest "$L1_CONTAINER" "$L1_ADDRESS")"
  INITIAL_L2_NONCE="$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")"
  chmod 0777 "$CHECKPOINT_DIR"
}

get_nonce() {
  local block="$1"
  local container="$2"
  local address="$3"
  docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    nonce --block "$block" --rpc-url "http://${container}:8545" "$address"
}

get_code() {
  local container="$1"
  local address="$2"
  docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    code --rpc-url "http://${container}:8545" "$address"
}

deployer_container() {
  docker run "$@" --network "$NETWORK" \
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

run_deployer() {
  deployer_container --rm
}

start_named_deployer() {
  deployer_container -d --name "$DEPLOYER_CONTAINER" >/dev/null
}

set_automine() {
  local container="$1"
  local enabled="$2"
  docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    rpc --rpc-url "http://${container}:8545" anvil_setAutomine "$enabled" >/dev/null
}

mine_one() {
  local container="$1"
  docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
    rpc --rpc-url "http://${container}:8545" anvil_mine 1 >/dev/null
}

wait_for_intent_and_pending_nonce() {
  local chain="$1"
  local container="$2"
  local signer="$3"
  local expected_pending_nonce="$4"
  local intent_address=""
  for _ in $(seq 1 200); do
    if [[ -s "$CHECKPOINT_DIR/checkpoint.json" ]]; then
      intent_address="$(
        jq -r --arg chain "$chain" \
          '[.inFlightDeployments[] | select(.chain == $chain) | .expectedAddress] | if length == 1 then .[0] else empty end' \
          "$CHECKPOINT_DIR/checkpoint.json" 2>/dev/null || true
      )"
      if [[ -n "$intent_address" ]] && \
        [[ "$(get_nonce pending "$container" "$signer")" -eq "$expected_pending_nonce" ]]; then
        printf '%s\n' "$intent_address"
        return 0
      fi
    fi
    sleep 0.1
  done
  echo "timed out waiting for ${chain} intent at pending nonce ${expected_pending_nonce}; address=${intent_address:-unknown}" >&2
  return 1
}

assert_unresolved_rerun() {
  local expected_l1_nonce="$1"
  local expected_l2_nonce="$2"
  local output status
  set +e
  output="$(run_deployer 2>&1)"
  status=$?
  set -e
  test "$status" -ne 0
  [[ "$output" == *"transaction broadcast is ambiguous and requires operator reconciliation"* ]]
  test "$(get_nonce pending "$L1_CONTAINER" "$L1_ADDRESS")" -eq "$expected_l1_nonce"
  test "$(get_nonce pending "$L2_CONTAINER" "$L2_ADDRESS")" -eq "$expected_l2_nonce"
}

verify_happy_path() {
  start_scenario happy
  run_deployer
  local l1_nonce_before l2_nonce_before
  l1_nonce_before="$(get_nonce latest "$L1_CONTAINER" "$L1_ADDRESS")"
  l2_nonce_before="$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")"

  test "$l1_nonce_before" -eq "$((INITIAL_L1_NONCE + 10))"
  test "$l2_nonce_before" -eq "$((INITIAL_L2_NONCE + 8))"
  test "$(jq '.completedSteps | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 4
  test "$(jq '.deployments | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 18
  jq -e --arg digest "$DEPLOYER_IMAGE_DIGEST" \
    '.schemaVersion == 2 and .artifactDigest == $digest and (.inFlightDeployments | length) == 0' \
    "$CHECKPOINT_DIR/checkpoint.json" >/dev/null

  while IFS=$'\t' read -r chain address; do
    local container="$L1_CONTAINER"
    if [[ "$chain" == "l2" ]]; then container="$L2_CONTAINER"; fi
    if [[ "$(get_code "$container" "$address")" == "0x" ]]; then
      echo "expected bytecode for ${chain} deployment at ${address}" >&2
      return 1
    fi
  done < <(jq -r '.expectedDeployments[] | [.chain, .expectedAddress] | @tsv' "$CHECKPOINT_DIR/checkpoint.json")

  cp "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.before-rerun.json"
  run_deployer
  test "$(get_nonce latest "$L1_CONTAINER" "$L1_ADDRESS")" -eq "$l1_nonce_before"
  test "$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")" -eq "$l2_nonce_before"
  cmp "$CHECKPOINT_DIR/checkpoint.before-rerun.json" "$CHECKPOINT_DIR/checkpoint.json"

  mv "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.saved.json"
  local checkpoint_loss_output checkpoint_loss_status
  set +e
  checkpoint_loss_output="$(run_deployer 2>&1)"
  checkpoint_loss_status=$?
  set -e
  test "$checkpoint_loss_status" -ne 0
  [[ "$checkpoint_loss_output" == *"no checkpoint and L1 signer nonce"* ]]
  test "$(get_nonce latest "$L1_CONTAINER" "$L1_ADDRESS")" -eq "$l1_nonce_before"
  test "$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")" -eq "$l2_nonce_before"
}

verify_l1_broadcast_crash() {
  start_scenario l1-crash
  set_automine "$L1_CONTAINER" false
  start_named_deployer

  local expected_l1_pending=$((INITIAL_L1_NONCE + 1))
  local intent_address
  intent_address="$(wait_for_intent_and_pending_nonce l1 "$L1_CONTAINER" "$L1_ADDRESS" "$expected_l1_pending")"
  docker kill "$DEPLOYER_CONTAINER" >/dev/null
  docker rm "$DEPLOYER_CONTAINER" >/dev/null
  DEPLOYER_CONTAINER=""

  cp "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.after-crash.json"
  assert_unresolved_rerun "$expected_l1_pending" "$INITIAL_L2_NONCE"
  cmp "$CHECKPOINT_DIR/checkpoint.after-crash.json" "$CHECKPOINT_DIR/checkpoint.json"

  mine_one "$L1_CONTAINER"
  test "$(get_code "$L1_CONTAINER" "$intent_address")" != "0x"
  assert_unresolved_rerun "$expected_l1_pending" "$INITIAL_L2_NONCE"
  cmp "$CHECKPOINT_DIR/checkpoint.after-crash.json" "$CHECKPOINT_DIR/checkpoint.json"
}

verify_l2_broadcast_crash() {
  start_scenario l2-crash
  set_automine "$L2_CONTAINER" false
  start_named_deployer

  local expected_l1_prefix=$((INITIAL_L1_NONCE + 5))
  local expected_l2_pending=$((INITIAL_L2_NONCE + 1))
  local intent_address
  intent_address="$(wait_for_intent_and_pending_nonce l2 "$L2_CONTAINER" "$L2_ADDRESS" "$expected_l2_pending")"
  docker kill "$DEPLOYER_CONTAINER" >/dev/null
  docker rm "$DEPLOYER_CONTAINER" >/dev/null
  DEPLOYER_CONTAINER=""

  test "$(get_nonce latest "$L1_CONTAINER" "$L1_ADDRESS")" -eq "$expected_l1_prefix"
  jq -e '.completedSteps == ["l1-rollup"] and (.deployments | length) == 5' \
    "$CHECKPOINT_DIR/checkpoint.json" >/dev/null
  local l1_prefix
  l1_prefix="$(jq -c '{completedSteps, deployments: [.deployments | to_entries[] | select(.value.chainId == "31337")]}' "$CHECKPOINT_DIR/checkpoint.json")"

  cp "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.after-crash.json"
  assert_unresolved_rerun "$expected_l1_prefix" "$expected_l2_pending"
  cmp "$CHECKPOINT_DIR/checkpoint.after-crash.json" "$CHECKPOINT_DIR/checkpoint.json"

  mine_one "$L2_CONTAINER"
  test "$(get_code "$L2_CONTAINER" "$intent_address")" != "0x"
  assert_unresolved_rerun "$expected_l1_prefix" "$expected_l2_pending"
  cmp "$CHECKPOINT_DIR/checkpoint.after-crash.json" "$CHECKPOINT_DIR/checkpoint.json"
  test "$(jq -c '{completedSteps, deployments: [.deployments | to_entries[] | select(.value.chainId == "31337")]}' "$CHECKPOINT_DIR/checkpoint.json")" = "$l1_prefix"
}

verify_happy_path
verify_l1_broadcast_crash
verify_l2_broadcast_crash
cleanup_scenario

echo "two-chain deployment, zero-transaction rerun, checkpoint-loss refusal, and L1/L2 crash refusal verified"
