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

RPC_READY_TIMEOUT_S=60
RPC_READY_POLL_INTERVAL_S=1
# The intent/pending-nonce condition below settles much faster than RPC
# readiness (it's just local checkpoint-file + nonce reads, no container
# boot), so it polls a shorter overall budget at a finer interval.
INTENT_POLL_TIMEOUT_S=20
INTENT_POLL_INTERVAL_S=0.1
INTENT_POLL_ITERATIONS=200 # INTENT_POLL_TIMEOUT_S / INTENT_POLL_INTERVAL_S

NETWORK=""
CHECKPOINT_DIR=""
CHECKPOINT_VOL=""
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
  if [[ -n "$CHECKPOINT_VOL" ]]; then
    docker volume rm "$CHECKPOINT_VOL" >/dev/null 2>&1 || true
  fi
  if [[ -n "$CHECKPOINT_DIR" && -d "$CHECKPOINT_DIR" && "$(basename "$CHECKPOINT_DIR")" == tmp.* ]]; then
    rm -rf "$CHECKPOINT_DIR"
  fi
  NETWORK=""
  CHECKPOINT_DIR=""
  CHECKPOINT_VOL=""
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
  for _ in $(seq 1 "$RPC_READY_TIMEOUT_S"); do
    if docker run --rm --network "$NETWORK" --entrypoint cast "$FOUNDRY_IMAGE" \
      block-number --rpc-url "http://${container}:8545" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$RPC_READY_POLL_INTERVAL_S"
  done
  echo "${container} RPC did not become ready" >&2
  return 1
}

start_scenario() {
  local label="$1"
  cleanup_scenario
  # Reset any bootstrap inputs a prior scenario exported so they never leak.
  unset BOOTSTRAP_MANIFEST_FILE BOOTSTRAP_SCRIPTS_DIR
  NETWORK="forge-deployer-${label}-$$"
  CHECKPOINT_DIR="$(mktemp -d)"
  CHECKPOINT_VOL="forge-deployer-${label}-ckpt-$$"
  L1_CONTAINER="forge-deployer-${label}-l1-$$"
  L2_CONTAINER="forge-deployer-${label}-l2-$$"
  DEPLOYER_CONTAINER="forge-deployer-${label}-runner-$$"
  docker volume create "$CHECKPOINT_VOL" >/dev/null

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

# The deployer reads/writes its checkpoint in a NAMED volume, not a bind mount.
# On the Kubernetes self-hosted runners the Docker daemon runs rootless / in a
# sidecar with its own user namespace, so a bind-mounted host dir is not writable
# by the image's non-root `node` user (neither `chmod 0777` nor `--user "$(id
# -u)"` survives the uid-remap). A named volume is container-owned, so `node`
# can write it regardless of the host daemon's mapping. The published image still
# runs as the non-root `node` user; this changes only how the test shares state.
#
# The host never touches the volume's filesystem directly (checkpoint files are
# written 0o600). Instead the host keeps a staging dir (CHECKPOINT_DIR) and syncs
# it with the volume via tar streams through the daemon, which runs as root and
# is therefore immune to both the uid-remap and the 0o600 mode.

# push_checkpoint: replace the volume's contents with the host staging dir. The
# volume is cleared first so an empty staging dir reproduces checkpoint loss.
# The volume root is chmod 0777 so `node` can create checkpoint.json.tmp, and the
# staged files are made world-readable so `node` can read bootstrap fixtures.
push_checkpoint() {
  tar -C "$CHECKPOINT_DIR" -cf - . | docker run --rm -i \
    -v "$CHECKPOINT_VOL:/checkpoint" \
    --entrypoint sh "$DEPLOYER_IMAGE" -c \
    'find /checkpoint -mindepth 1 -delete 2>/dev/null || true; tar -C /checkpoint -xf -; chmod 0777 /checkpoint; chmod -R a+rX /checkpoint'
}

# pull_checkpoint: copy the volume's contents into the host staging dir. The tar
# stream is extracted on the HOST with ownership and permissions stripped, so the
# staged copies are host-owned and readable (not root-owned 0o600) and jq/cmp/cp
# work. Tolerates an empty volume. Used after each run and on each poll iteration.
pull_checkpoint() {
  docker run --rm -v "$CHECKPOINT_VOL:/checkpoint:ro" \
    --entrypoint sh "$DEPLOYER_IMAGE" -c 'tar -C /checkpoint -cf - .' \
    | tar -C "$CHECKPOINT_DIR" --no-same-owner --no-same-permissions -xf - 2>/dev/null || true
}

deployer_container() {
  docker run "$@" --network "$NETWORK" \
    -v "$CHECKPOINT_VOL:/checkpoint" \
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
    ${BOOTSTRAP_MANIFEST_FILE:+-e "BOOTSTRAP_MANIFEST_FILE=$BOOTSTRAP_MANIFEST_FILE"} \
    ${BOOTSTRAP_SCRIPTS_DIR:+-e "BOOTSTRAP_SCRIPTS_DIR=$BOOTSTRAP_SCRIPTS_DIR"} \
    "$DEPLOYER_IMAGE"
}

run_deployer() {
  push_checkpoint
  deployer_container --rm
  pull_checkpoint
}

start_named_deployer() {
  push_checkpoint
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
  for _ in $(seq 1 "$INTENT_POLL_ITERATIONS"); do
    # The detached deployer writes the checkpoint live in the volume; refresh the
    # host staging copy each poll. Writes are atomic (tmp + rename), so a pull
    # snapshots either the old or the new complete file, never a partial one.
    pull_checkpoint
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
    sleep "$INTENT_POLL_INTERVAL_S"
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
  # Anvil preinstalls the deterministic proxy factory in genesis, so the runner
  # adopts it as recovered and the funding send is skipped: only the 8 contract
  # deployments consume L2 deployer nonces here.
  test "$l2_nonce_before" -eq "$((INITIAL_L2_NONCE + 8))"
  test "$(jq '.completedSteps | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 5
  # Total deployment-key count from buildAddressPlan(); kept as a literal since
  # this shell test can't import the TS plan. Update alongside
  # test/checkpoint.test.ts (which derives it dynamically) if the plan changes.
  test "$(jq '.deployments | length' "$CHECKPOINT_DIR/checkpoint.json")" -eq 19
  jq -e --arg digest "$DEPLOYER_IMAGE_DIGEST" \
    '.schemaVersion == 3 and .artifactDigest == $digest and (.inFlightDeployments | length) == 0' \
    "$CHECKPOINT_DIR/checkpoint.json" >/dev/null

  # The deterministic deployment proxy factory must be installed on L2, and the
  # checkpoint records it as recovered (adopted from the preinstalled code).
  test "$(get_code "$L2_CONTAINER" "0x4e59b44847b379578588920cA78FbF26c0B4956C")" != "0x"
  jq -e '.deployments["l2-deterministic-proxy.factory"].recovered == true' \
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
  # Snapshot the crashed deployer's final checkpoint state from the volume; the
  # last pull in the poll loop predates the kill.
  pull_checkpoint

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
  # Snapshot the crashed deployer's final checkpoint state from the volume; the
  # last pull in the poll loop predates the kill.
  pull_checkpoint

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

# Exercises the optional custom bootstrap phase: a sign funding item, a keyless
# presigned deploy, and an operator script, all on L2, then asserts the rerun
# skips every item and advances no further nonce.
verify_custom_bootstrap() {
  start_scenario bootstrap

  # Operator-supplied bootstrap inputs are real files shipped under
  # test/fixtures/bootstrap/ and staged into the checkpoint staging dir exactly as
  # an operator would mount them; push_checkpoint seeds them into the volume.
  local fixtures_dir
  fixtures_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../fixtures/bootstrap" && pwd)"
  # Bytecode/salt/factory used by the fixture manifest and its create2 script.
  local simple_initcode="0x6001600c60003960016000f300"
  local create2_salt="0x00000000000000000000000000000000000000000000000000000000000000cd"
  local proxy_factory="0x4e59b44847b379578588920cA78FbF26c0B4956C"

  mkdir -p "$CHECKPOINT_DIR/bootstrap"
  cp "$fixtures_dir/manifest.json" "$CHECKPOINT_DIR/manifest.json"
  cp "$fixtures_dir/deploy-via-create2.js" "$CHECKPOINT_DIR/bootstrap/deploy-via-create2.js"

  BOOTSTRAP_MANIFEST_FILE="/checkpoint/manifest.json"
  BOOTSTRAP_SCRIPTS_DIR="/checkpoint/bootstrap"

  # The bootstrap phase runs sign items first (nonce N, N+1), then the script's
  # create2 call (nonce N+2), where N = INITIAL_L2_NONCE + 8 (the 8 core L2
  # deploys; the Anvil-preinstalled det-proxy consumes no nonce).
  local bare_create_nonce=$((INITIAL_L2_NONCE + 9))

  run_deployer
  local l2_nonce_after_first
  l2_nonce_after_first="$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")"
  # 8 core L2 deploys + 2 sign items + 1 create2 script call.
  test "$l2_nonce_after_first" -eq "$((INITIAL_L2_NONCE + 11))"

  # The bare create landed at the deployer's CREATE address and the create2
  # script landed at the deterministic CREATE2 address.
  local bare_create_addr create2_addr
  bare_create_addr="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" compute-address --nonce "$bare_create_nonce" "$L2_ADDRESS" | awk '{print $NF}')"
  create2_addr="$(docker run --rm --entrypoint cast "$FOUNDRY_IMAGE" compute-address --salt "$create2_salt" --init-code "$simple_initcode" "$proxy_factory" | awk '{print $NF}')"
  test "$(get_code "$L2_CONTAINER" "$bare_create_addr")" != "0x"
  test "$(get_code "$L2_CONTAINER" "$create2_addr")" != "0x"

  jq -e '.bootstrap["bootstrap.fund-target"].kind == "sign"' "$CHECKPOINT_DIR/checkpoint.json" >/dev/null
  jq -e '.bootstrap["bootstrap.bare-create"].kind == "sign"' "$CHECKPOINT_DIR/checkpoint.json" >/dev/null
  jq -e --arg addr "$bare_create_addr" \
    '.bootstrap["bootstrap.bare-create"].address | ascii_downcase == ($addr | ascii_downcase)' \
    "$CHECKPOINT_DIR/checkpoint.json" >/dev/null
  jq -e '.bootstrap["bootstrap.create2-deploy"].kind == "script"' "$CHECKPOINT_DIR/checkpoint.json" >/dev/null

  # A rerun with the same manifest completes no additional items and advances no
  # further nonce: completed bootstrap items are skipped.
  cp "$CHECKPOINT_DIR/checkpoint.json" "$CHECKPOINT_DIR/checkpoint.before-rerun.json"
  run_deployer
  test "$(get_nonce latest "$L2_CONTAINER" "$L2_ADDRESS")" -eq "$l2_nonce_after_first"
  cmp "$CHECKPOINT_DIR/checkpoint.before-rerun.json" "$CHECKPOINT_DIR/checkpoint.json"

  unset BOOTSTRAP_MANIFEST_FILE BOOTSTRAP_SCRIPTS_DIR
}

verify_happy_path
verify_l1_broadcast_crash
verify_l2_broadcast_crash
verify_custom_bootstrap
cleanup_scenario

echo "two-chain deployment, zero-transaction rerun, checkpoint-loss refusal, L1/L2 crash refusal, and custom bootstrap verified"
