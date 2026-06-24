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

usage() {
  cat <<'EOF'
Usage: ./scripts/reset.sh [--local-l1] [--forget-deployer]

  --local-l1          also remove the quickstart local L1 data volume
  --forget-deployer  remove the generated Sepolia deployer keystore
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

lineth_banner "reset · clean quickstart state"

lineth_section "docker compose"
if [ "$RESET_LOCAL_L1" = "true" ]; then
  COMPOSE="$COMPOSE --profile local-l1"
fi
# shellcheck disable=SC2086
$COMPOSE down -v --remove-orphans
docker volume rm \
  lineth-stack-shared-config \
  lineth-stack-l2-genesis \
  lineth-stack-rendered-config \
  lineth-stack-postman-runtime-config >/dev/null 2>&1 || true
if [ "$RESET_LOCAL_L1" = "true" ]; then
  docker volume rm lineth-stack-local-l1-data >/dev/null 2>&1 || true
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
