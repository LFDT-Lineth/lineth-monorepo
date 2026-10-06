#!/usr/bin/env sh
# Stop the quickstart and remove both Docker volumes and host-backed artifacts.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
STACK_DIR="$(CDPATH= cd "$SCRIPT_DIR/.." && pwd -P)"
LINETH_LOG_CONTEXT="reset"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"

COMPOSE="docker compose --env-file versions.env --env-file .env --profile stack-partial-prover"
RESET_LOCAL_L1=false
FORGET_DEPLOYER=false
PURGE_DEPS=false

usage() {
  cat <<'EOF'
Usage: ./scripts/reset.sh [--local-l1] [--forget-deployer] [--purge-deps]

  --local-l1          also remove the quickstart local L1 data volume
  --forget-deployer  remove the generated Sepolia deployer keystore
  --purge-deps        also remove the preserved pnpm download-cache volume, so
                      the next boot re-downloads all Node dependencies
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --local-l1)
      RESET_LOCAL_L1=true
      ;;
    --forget-deployer)
      FORGET_DEPLOYER=true
      ;;
    --purge-deps)
      PURGE_DEPS=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      lineth_die "unknown argument: $1"
      ;;
  esac
  shift
done

cd "$STACK_DIR"
lineth_runtime_init "$STACK_DIR"
if [ "$(lineth_l1_mode || true)" = "local" ]; then
  RESET_LOCAL_L1=true
fi

lineth_subtitle "reset · clean quickstart state"

lineth_section "docker compose"
# Always tear down the local-l1 profile too, so stale local L1 containers are
# removed even when the current .env is Sepolia (e.g. after a mode switch).
# RESET_LOCAL_L1 only gates wiping the local L1 chain data volume below.
# shellcheck disable=SC2086
$COMPOSE --profile local-l1 down -v --remove-orphans
docker volume rm \
  lineth-stack-shared-config \
  lineth-stack-l2-genesis \
  lineth-stack-rendered-config \
  lineth-stack-postman-runtime-config >/dev/null 2>&1 || true
if [ "$RESET_LOCAL_L1" = "true" ]; then
  docker volume rm lineth-stack-local-l1-data >/dev/null 2>&1 || true
fi
# lineth-stack-contracts-pnpm-store is external in docker-compose.yml, so the
# `down -v` above leaves it alone: fresh boots reinstall node_modules from the
# lockfile but resolve package tarballs from this local cache instead of the
# npm registry. Only --purge-deps removes it.
if [ "$PURGE_DEPS" = "true" ]; then
  docker volume rm lineth-stack-contracts-pnpm-store >/dev/null 2>&1 || true
  lineth_info "removed pnpm download-cache volume; next boot re-downloads Node dependencies"
fi
if docker network inspect lineth-stack_lineth >/dev/null 2>&1; then
  lineth_warn "lineth-stack_lineth network still in use by an external container"
  lineth_info "inspect with: docker network inspect lineth-stack_lineth"
fi

lineth_section "host artifacts"
DEPLOYER_KEYSTORE_DIR="$STACK_DIR/artifacts/accounts/deployer-keystore"
PRESERVED_DEPLOYER_DIR=""
if [ "$FORGET_DEPLOYER" != "true" ] && [ -d "$DEPLOYER_KEYSTORE_DIR" ]; then
  PRESERVED_DEPLOYER_DIR="$(mktemp -d)"
  cp -a "$DEPLOYER_KEYSTORE_DIR"/. "$PRESERVED_DEPLOYER_DIR"/
fi

rm -rf \
  "$STACK_DIR/artifacts/accounts" \
  "$STACK_DIR/artifacts/genesis" \
  "$STACK_DIR/artifacts/config" \
  "$STACK_DIR/artifacts/deployments" \
  "$STACK_DIR/artifacts/reports"

if [ -n "$PRESERVED_DEPLOYER_DIR" ]; then
  mkdir -p "$DEPLOYER_KEYSTORE_DIR"
  cp -a "$PRESERVED_DEPLOYER_DIR"/. "$DEPLOYER_KEYSTORE_DIR"/
  rm -rf "$PRESERVED_DEPLOYER_DIR"
  lineth_ok "removed generated host artifacts; preserved Sepolia deployer keystore"
elif [ "$FORGET_DEPLOYER" = "true" ]; then
  lineth_ok "removed generated host artifacts, including Sepolia deployer keystore"
else
  lineth_ok "removed generated host artifacts"
fi
