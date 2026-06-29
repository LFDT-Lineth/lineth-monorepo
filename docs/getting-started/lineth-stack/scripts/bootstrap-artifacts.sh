#!/usr/bin/env sh
# Prepare host-backed quickstart generated files before Docker Compose creates runtime containers.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
STACK_DIR="$(CDPATH= cd "$SCRIPT_DIR/.." && pwd -P)"
LINETH_LOG_CONTEXT="bootstrap"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"

COMPOSE="docker compose --env-file versions.env --env-file .env --profile bootstrap"

cd "$STACK_DIR"
BUSYBOX_TAG="${BUSYBOX_TAG:-$(sed -n 's/^BUSYBOX_TAG=//p' versions.env | tail -n 1)}"
BUSYBOX_TAG="${BUSYBOX_TAG:-1.36.1}"

if [ "${LINETH_SKIP_BANNER:-false}" != "true" ]; then
  lineth_banner "prepare · generated files"
fi

lineth_section "Create generated-file folders"
mkdir -p \
  "$STACK_DIR/artifacts/accounts/deployer-keystore" \
  "$STACK_DIR/artifacts/accounts/runtime-keystores" \
  "$STACK_DIR/artifacts/accounts/web3signer-keys" \
  "$STACK_DIR/artifacts/genesis" \
  "$STACK_DIR/artifacts/config/coordinator" \
  "$STACK_DIR/artifacts/config/maru" \
  "$STACK_DIR/artifacts/config/sequencer" \
  "$STACK_DIR/artifacts/config/l2-node-besu" \
  "$STACK_DIR/artifacts/config/prover" \
  "$STACK_DIR/artifacts/config/postman" \
  "$STACK_DIR/artifacts/deployments/deploy-logs" \
  "$STACK_DIR/artifacts/reports"

# Compose parses env_file paths before it can run the renderer. Keep a harmless
# placeholder so compose config/pull/run work from a clean checkout.
if [ ! -f "$STACK_DIR/artifacts/config/postman/postman.env" ]; then
  : > "$STACK_DIR/artifacts/config/postman/postman.env"
fi
lineth_ok "generated-file folders ready under $STACK_DIR/artifacts"

volume_device() {
  docker volume inspect -f '{{ index .Options "device" }}' "$1" 2>/dev/null || true
}

volume_exists() {
  docker volume inspect "$1" >/dev/null 2>&1
}

artifact_dir_has_content() {
  dir="$1"
  [ -n "$(find "$dir" -mindepth 1 -print -quit 2>/dev/null || true)" ]
}

copy_if_present() {
  src="$1"
  dst="$2"
  [ -e "$src" ] || return 0
  if [ -d "$src" ]; then
    mkdir -p "$dst"
    cp -a "$src"/. "$dst"/
  else
    mkdir -p "$(dirname "$dst")"
    cp -a "$src" "$dst"
  fi
}

remove_stopped_volume_users() {
  volume="$1"
  running_ids="$(docker ps -q --filter "volume=$volume" 2>/dev/null || true)"
  if [ -n "$running_ids" ]; then
    lineth_die "$volume is used by running containers. Stop the stack first, then rerun ./scripts/bootstrap-artifacts.sh"
  fi

  stopped_ids="$(docker ps -aq --filter "volume=$volume" 2>/dev/null || true)"
  [ -n "$stopped_ids" ] || return 0
  for id in $stopped_ids; do
    docker rm "$id" >/dev/null
  done
  lineth_info "removed stopped containers still attached to $volume"
}

migrate_legacy_volume() {
  volume="$1"
  target_dir="$2"

  volume_exists "$volume" || return 0
  existing_device="$(volume_device "$volume")"
  [ "$existing_device" = "$target_dir" ] && return 0

  lineth_warn "$volume exists with old Docker-volume backing; migrating to $target_dir"
  if ! artifact_dir_has_content "$target_dir"; then
    docker run --rm \
      -v "$volume:/from:ro" \
      -v "$target_dir:/to:rw" \
      "busybox:${BUSYBOX_TAG}" sh -eu -c 'cp -a /from/. /to/'
    lineth_info "copied existing $volume contents into $target_dir"
  else
    lineth_info "$target_dir already has content; not copying old $volume contents"
  fi

  remove_stopped_volume_users "$volume"
  if ! docker volume rm "$volume" >/dev/null 2>&1; then
    lineth_die "$volume is still in use. Stop the stack first, then rerun ./scripts/bootstrap-artifacts.sh"
  fi
  lineth_ok "removed legacy $volume so Compose can recreate it as host-backed"
}

migrate_legacy_shared_dir() {
  from_dir="$1"
  [ -d "$from_dir" ] || return 0

  copy_if_present "$from_dir/addresses-precomputed.json" "$STACK_DIR/artifacts/accounts/addresses-precomputed.json"
  copy_if_present "$from_dir/runtime-keys.env" "$STACK_DIR/artifacts/accounts/runtime-keys.env"
  copy_if_present "$from_dir/demo-traffic.env" "$STACK_DIR/artifacts/accounts/demo-traffic.env"
  copy_if_present "$from_dir/runtime-keystores" "$STACK_DIR/artifacts/accounts/runtime-keystores"
  copy_if_present "$from_dir/web3signer-keys" "$STACK_DIR/artifacts/accounts/web3signer-keys"

  copy_if_present "$from_dir/addresses.json" "$STACK_DIR/artifacts/deployments/addresses.json"
  copy_if_present "$from_dir/deploy-runtime.env" "$STACK_DIR/artifacts/deployments/deploy-runtime.env"
  copy_if_present "$from_dir/deploy-timing.jsonl" "$STACK_DIR/artifacts/deployments/deploy-timing.jsonl"
  copy_if_present "$from_dir/deploy-logs" "$STACK_DIR/artifacts/deployments/deploy-logs"
}

migrate_legacy_rendered_dir() {
  from_dir="$1"
  [ -d "$from_dir" ] || return 0

  copy_if_present "$from_dir/coordinator-config.predeploy.toml" "$STACK_DIR/artifacts/config/coordinator/coordinator-config.predeploy.toml"
  copy_if_present "$from_dir/coordinator-config.toml" "$STACK_DIR/artifacts/config/coordinator/coordinator-config.toml"
  copy_if_present "$from_dir/maru-config.toml" "$STACK_DIR/artifacts/config/maru/config.toml"
  copy_if_present "$from_dir/sequencer.config.toml" "$STACK_DIR/artifacts/config/sequencer/sequencer.config.toml"
  copy_if_present "$from_dir/l2-node-besu.config.toml" "$STACK_DIR/artifacts/config/l2-node-besu/l2-node-besu.config.toml"
  copy_if_present "$from_dir/prover-config-partial.toml" "$STACK_DIR/artifacts/config/prover/prover-config-partial.toml"
}

migrate_legacy_volume_to_temp() {
  volume="$1"
  handler="$2"

  volume_exists "$volume" || return 0
  tmp_dir="$(mktemp -d)"
  docker run --rm \
    -v "$volume:/from:ro" \
    -v "$tmp_dir:/to:rw" \
    "busybox:${BUSYBOX_TAG}" sh -eu -c 'cp -a /from/. /to/ 2>/dev/null || true'
  "$handler" "$tmp_dir"
  rm -rf "$tmp_dir"
  remove_stopped_volume_users "$volume"
  docker volume rm "$volume" >/dev/null 2>&1 || true
}

lineth_section "Clean old Docker-volume state"
migrate_legacy_shared_dir "$STACK_DIR/artifacts/shared"
migrate_legacy_rendered_dir "$STACK_DIR/artifacts/config/rendered"
migrate_legacy_volume_to_temp lineth-stack-shared-config migrate_legacy_shared_dir
migrate_legacy_volume lineth-stack-l2-genesis "$STACK_DIR/artifacts/genesis"
migrate_legacy_volume_to_temp lineth-stack-rendered-config migrate_legacy_rendered_dir
docker volume rm lineth-stack-postman-runtime-config >/dev/null 2>&1 || true
rm -rf "$STACK_DIR/artifacts/shared" "$STACK_DIR/artifacts/config/rendered"
lineth_ok "generated files ready"

lineth_section "Generate runtime wallets and keystores"
lineth_info "generating/reusing runtime wallets, encrypted keystores, and Web3Signer key files"
# shellcheck disable=SC2086
COMPOSE_PROGRESS=plain $COMPOSE run --rm --no-deps account-setup

lineth_section "Render Postman Web3Signer config"
lineth_info "rendering generated Postman env before runtime containers are created"
# shellcheck disable=SC2086
COMPOSE_PROGRESS=plain $COMPOSE run --rm --no-deps postman-config-render

POSTMAN_ENV="$STACK_DIR/artifacts/config/postman/postman.env"
[ -s "$POSTMAN_ENV" ] || lineth_die "$POSTMAN_ENV missing or empty"

if grep -q 'SIGNER_PRIVATE_KEY' "$POSTMAN_ENV"; then
  lineth_die "$POSTMAN_ENV must not contain raw Postman private keys"
fi

grep -q "L1_SIGNER_TYPE='web3signer'" "$POSTMAN_ENV" \
  || lineth_die "$POSTMAN_ENV missing L1 Web3Signer config"
grep -q "L2_SIGNER_TYPE='web3signer'" "$POSTMAN_ENV" \
  || lineth_die "$POSTMAN_ENV missing L2 Web3Signer config"
grep -q "L1_WEB3_SIGNER_PUBLIC_KEY='0x" "$POSTMAN_ENV" \
  || lineth_die "$POSTMAN_ENV missing L1 Web3Signer public key"
grep -q "L2_WEB3_SIGNER_PUBLIC_KEY='0x" "$POSTMAN_ENV" \
  || lineth_die "$POSTMAN_ENV missing L2 Web3Signer public key"

chmod 0644 "$POSTMAN_ENV" || true
lineth_ok "generated $POSTMAN_ENV"
