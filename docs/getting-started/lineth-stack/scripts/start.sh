#!/usr/bin/env sh
# Thin quickstart launcher. Docker Compose remains the engine; TypeScript owns
# Sepolia/key preflight and account setup.
set -eu

SCRIPT_DIR="$(CDPATH='' cd "$(dirname "$0")" && pwd -P)"
# shellcheck disable=SC2034
LINETH_LOG_CONTEXT="start"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"

TAIL=false
PULL=true
FORCE_PULL=false
LINETH_VERBOSE="${LINETH_VERBOSE:-false}"
WIZARD=false
WIZARD_THEN_START=false
WIZARD_CLEAR_BEFORE_START=false
WIZARD_NON_INTERACTIVE=false
WIZARD_YES=false
WIZARD_FLAG_L1_MODE=""
WIZARD_FLAG_L1_RPC_URL=""
WIZARD_FLAG_PROVER=""
WIZARD_OPTION_REQUIRES=""

ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"
STACK="$ROOT/docs/getting-started/lineth-stack"

usage() {
  cat <<'EOF'
Usage: ./scripts/start.sh [--tail] [--no-pull] [--verbose]
       ./scripts/start.sh --wizard [--then-start] [--clear-before-start] [--non-interactive] [--yes]

  --tail              start the stack, then show guided deployment/finality progress
  --no-pull           skip docker compose pull
  --pull              force docker compose pull even when all pinned images are local
  --verbose           show raw preparation/pull details in the default terminal output
  --wizard, --init    configure .env with the guided setup wizard
  --then-start        after --wizard succeeds, start with ./scripts/start.sh --tail
  --clear-before-start
                      with --wizard --then-start, run ./scripts/reset.sh before starting
  --non-interactive   resolve wizard answers from flags/env/.env defaults; never prompt
  --yes               accept the wizard write without a confirmation prompt

Run ./scripts/start.sh --wizard --help for all wizard options.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --wizard|--init)
      WIZARD=true
      ;;
    --then-start)
      WIZARD_THEN_START=true
      ;;
    --clear-before-start)
      WIZARD_CLEAR_BEFORE_START=true
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---clear-before-start}"
      ;;
    --non-interactive)
      WIZARD_NON_INTERACTIVE=true
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---non-interactive}"
      ;;
    --yes)
      WIZARD_YES=true
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---yes}"
      ;;
    --l1-mode)
      shift
      [ "$#" -gt 0 ] || lineth_die "--l1-mode requires local or sepolia"
      WIZARD_FLAG_L1_MODE="$1"
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---l1-mode}"
      ;;
    --l1-rpc-url)
      shift
      [ "$#" -gt 0 ] || lineth_die "--l1-rpc-url requires a URL"
      WIZARD_FLAG_L1_RPC_URL="$1"
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---l1-rpc-url}"
      ;;
    --prover)
      shift
      [ "$#" -gt 0 ] || lineth_die "--prover requires dev or partial"
      WIZARD_FLAG_PROVER="$1"
      WIZARD_OPTION_REQUIRES="${WIZARD_OPTION_REQUIRES:---prover}"
      ;;
    --tail)
      TAIL=true
      ;;
    --no-pull)
      PULL=false
      ;;
    --pull)
      FORCE_PULL=true
      ;;
    --verbose)
      LINETH_VERBOSE=true
      export LINETH_VERBOSE
      ;;
    -h|--help)
      if [ "$WIZARD" = "true" ]; then
        # shellcheck disable=SC1091
        . "$SCRIPT_DIR/lib/wizard.sh"
        lineth_wizard_usage
      else
        usage
      fi
      exit 0
      ;;
    *)
      lineth_die "unknown argument: $1"
      ;;
  esac
  shift
done

if [ "$WIZARD_THEN_START" = "true" ] && [ "$WIZARD" != "true" ]; then
  lineth_die "--then-start requires --wizard; to just boot, run ./scripts/start.sh --tail"
fi

if [ "$WIZARD_CLEAR_BEFORE_START" = "true" ] && [ "$WIZARD" != "true" ]; then
  lineth_die "--clear-before-start requires --wizard; to just boot, run ./scripts/reset.sh && ./scripts/start.sh --tail"
fi

if [ "$WIZARD_CLEAR_BEFORE_START" = "true" ] && [ "$WIZARD_THEN_START" != "true" ]; then
  lineth_die "--clear-before-start requires --then-start"
fi

if [ "$WIZARD" != "true" ] && [ -n "$WIZARD_OPTION_REQUIRES" ]; then
  lineth_die "$WIZARD_OPTION_REQUIRES requires --wizard; to just boot, run ./scripts/start.sh --tail"
fi

if [ "$WIZARD" = "true" ]; then
  if [ -n "${LINETH_WIZARD_STACK_OVERRIDE:-}" ]; then
    STACK="$LINETH_WIZARD_STACK_OVERRIDE"
  fi

  # shellcheck disable=SC1091
  . "$SCRIPT_DIR/lib/wizard.sh"
  WIZARD_RESULT_FILE="$(mktemp)"
  if ! LINETH_WIZARD_FLAG_L1_MODE="$WIZARD_FLAG_L1_MODE" \
    LINETH_WIZARD_FLAG_L1_RPC_URL="$WIZARD_FLAG_L1_RPC_URL" \
    LINETH_WIZARD_FLAG_PROVER="$WIZARD_FLAG_PROVER" \
    LINETH_WIZARD_NON_INTERACTIVE="$WIZARD_NON_INTERACTIVE" \
    LINETH_WIZARD_YES="$WIZARD_YES" \
    LINETH_WIZARD_THEN_START="$WIZARD_THEN_START" \
    LINETH_WIZARD_CLEAR_BEFORE_START="$WIZARD_CLEAR_BEFORE_START" \
    LINETH_WIZARD_RESULT_FILE="$WIZARD_RESULT_FILE" \
    lineth_wizard_main "$STACK" "$SCRIPT_DIR"; then
    rm -f "$WIZARD_RESULT_FILE"
    exit 1
  fi

  WIZARD_REQUESTED_START="$WIZARD_THEN_START"
  WIZARD_REQUESTED_CLEAR_BEFORE_START="$WIZARD_CLEAR_BEFORE_START"
  if [ -f "$WIZARD_RESULT_FILE" ]; then
    while IFS='=' read -r wizard_result_key wizard_result_value; do
      case "$wizard_result_key" in
        START)
          WIZARD_REQUESTED_START="$wizard_result_value"
          ;;
        CLEAR_BEFORE_START)
          WIZARD_REQUESTED_CLEAR_BEFORE_START="$wizard_result_value"
          ;;
      esac
    done < "$WIZARD_RESULT_FILE"
  fi
  rm -f "$WIZARD_RESULT_FILE"

  if [ "$WIZARD_REQUESTED_START" != "true" ]; then
    exit 0
  fi

  if [ ! -f "$STACK/.env" ]; then
    lineth_die "wizard did not write .env; nothing to start. Re-run ./scripts/start.sh --wizard"
  fi

  if [ "$WIZARD_REQUESTED_CLEAR_BEFORE_START" = "true" ]; then
    if [ -n "${LINETH_WIZARD_TEST_RESET_FILE:-}" ]; then
      printf '%s\n' "$SCRIPT_DIR/reset.sh" > "$LINETH_WIZARD_TEST_RESET_FILE"
    else
      "$SCRIPT_DIR/reset.sh"
    fi
  fi

  if [ -n "${LINETH_WIZARD_TEST_START_FILE:-}" ]; then
    {
      printf '%s\n' "$SCRIPT_DIR/start.sh"
      printf '%s\n' "--tail"
      [ "$PULL" = "true" ] || printf '%s\n' "--no-pull"
      [ "$LINETH_VERBOSE" = "true" ] && printf '%s\n' "--verbose"
    } > "$LINETH_WIZARD_TEST_START_FILE"
    exit 0
  fi

  wizard_start_args=""
  if [ "$PULL" = "false" ]; then
    wizard_start_args="$wizard_start_args --no-pull"
  fi
  if [ "$FORCE_PULL" = "true" ]; then
    wizard_start_args="$wizard_start_args --pull"
  fi
  if [ "$LINETH_VERBOSE" = "true" ]; then
    wizard_start_args="$wizard_start_args --verbose"
  fi
  # shellcheck disable=SC2086
  exec "$SCRIPT_DIR/start.sh" --tail $wizard_start_args
fi

COMPOSE="docker compose --env-file versions.env --env-file .env --profile stack-partial-prover"
lineth_runtime_init "$STACK"
L1_MODE="$(lineth_l1_mode)"
case "$L1_MODE" in
  sepolia|local) ;;
  *) lineth_die "L1_MODE must be one of sepolia, local (got '$L1_MODE')" ;;
esac

show_compose_failure() {
  log_file="$1"
  lineth_error "Docker startup failed; raw output: $log_file"
  if [ -f "$log_file" ]; then
    tail -40 "$log_file" | lineth_indent
  fi
}

first_major_minor_version() {
  printf '%s\n' "$1" | awk 'match($0, /[0-9]+[.][0-9]+/) {
    split(substr($0, RSTART, RLENGTH), parts, ".")
    print parts[1] " " parts[2]
  }'
}

version_at_least() {
  version_parts="$(first_major_minor_version "$1")"
  [ -n "$version_parts" ] || return 1

  version_major="${version_parts%% *}"
  version_minor="${version_parts#* }"
  minimum_major="$2"
  minimum_minor="$3"

  [ "$version_major" -gt "$minimum_major" ] && return 0
  [ "$version_major" -eq "$minimum_major" ] && [ "$version_minor" -ge "$minimum_minor" ]
}

check_docker_preflight() {
  if ! command -v docker >/dev/null 2>&1; then
    lineth_die "Docker v24+ is required; install Docker Desktop or Docker Engine and retry"
  fi

  docker_version="$(docker --version 2>/dev/null || true)"
  if ! version_at_least "$docker_version" 24 0; then
    lineth_die "Docker v24+ is required (detected: ${docker_version:-unknown})"
  fi
  lineth_kv "docker" "$docker_version"

  compose_version="$(docker compose version 2>/dev/null || true)"
  if [ -z "$compose_version" ]; then
    lineth_die "Docker Compose v2.19+ is required; install/enable the Docker Compose plugin and retry"
  fi
  if ! version_at_least "$compose_version" 2 19; then
    lineth_die "Docker Compose v2.19+ is required (detected: $compose_version)"
  fi
  lineth_kv "compose" "$compose_version"

  if ! docker ps >/dev/null 2>&1; then
    lineth_die "Docker engine is not reachable; start Docker Desktop or the Docker daemon and retry"
  fi
  lineth_ok "Docker engine is reachable"

  docker_disk_tmp="$(mktemp)"
  if docker system df > "$docker_disk_tmp" 2>&1; then
    lineth_info "Docker disk usage:"
    lineth_child_output < "$docker_disk_tmp"
  else
    lineth_warn "Docker disk usage unavailable:"
    lineth_child_output < "$docker_disk_tmp"
  fi
  rm -f "$docker_disk_tmp"
}

run_ts_preflight() {
  if ! lineth_ts_node_available "$ROOT"; then
    lineth_warn "host L1 check skipped; run pnpm install for earlier balance/gas checks"
    lineth_info "account generation still runs the same checks before runtime containers are created"
    return 0
  fi

  tmp="$(mktemp)"
  if (
    export LINETH_STACK_DIR="$STACK"
    lineth_run_ts_node "$ROOT" "$STACK/scripts/internal/quickstart-preflight.ts"
  ) > "$tmp" 2>&1; then
    status=0
  else
    status=$?
  fi
  lineth_child_output < "$tmp"
  rm -f "$tmp"
  return "$status"
}

wait_for_local_l1() {
  lineth_kv "mode" "local L1"
  lineth_kv "start" "docker compose --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node"
  # shellcheck disable=SC2086
  local_l1_log="/tmp/lineth-local-l1.$$.$(date '+%Y%m%d%H%M%S').log"
  if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
    COMPOSE_PROGRESS=plain $COMPOSE --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node
  elif ! COMPOSE_PROGRESS=plain $COMPOSE --profile local-l1 up -d l1-node-genesis-generator l1-el-node l1-cl-node > "$local_l1_log" 2>&1; then
    lineth_error "local L1 Docker startup failed; raw output: $local_l1_log"
    tail -40 "$local_l1_log" | lineth_indent
    exit 1
  else
    lineth_info "raw local L1 Docker output: $local_l1_log"
  fi

  wait_for_container_health() {
    container="$1"
    label="$2"
    i=0
    while [ "$i" -lt 180 ]; do
      health="$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)"
      if [ "$health" = "healthy" ]; then
        lineth_ok "$label is healthy"
        return 0
      fi
      sleep 1
      i=$((i + 1))
    done

    lineth_error "$label did not become healthy"
    docker logs --tail 80 "$container" 2>&1 | lineth_indent || true
    exit 1
  }

  local_l1_block_number() {
    block_response="$(lineth_rpc_json "$(lineth_l1_host_rpc_url)" eth_blockNumber '[]')"
    block_hex="$(printf '%s\n' "$block_response" | lineth_json_stdin_string_field result)"
    [ -n "$block_hex" ] || return 1
    lineth_hex_to_dec_small "$block_hex"
  }

  wait_for_l1_block_advance() {
    baseline=""
    i=0
    while [ "$i" -lt 120 ]; do
      baseline="$(local_l1_block_number || true)"
      if lineth_is_uint "$baseline"; then
        break
      fi
      sleep 1
      i=$((i + 1))
    done
    if ! lineth_is_uint "$baseline"; then
      lineth_error "local L1 RPC did not return eth_blockNumber at $(lineth_l1_host_rpc_url)"
      docker logs --tail 80 l1-el-node 2>&1 | lineth_indent || true
      exit 1
    fi

    i=0
    while [ "$i" -lt 180 ]; do
      current="$(local_l1_block_number || true)"
      if lineth_is_uint "$current" && [ "$current" -gt "$baseline" ]; then
        lineth_ok "local L1 block production advanced from $baseline to $current"
        return 0
      fi
      sleep 1
      i=$((i + 1))
    done

    lineth_error "local L1 eth_blockNumber did not advance from $baseline"
    docker logs --tail 80 l1-el-node 2>&1 | lineth_indent || true
    docker logs --tail 80 l1-cl-node 2>&1 | lineth_indent || true
    exit 1
  }

  wait_for_container_health l1-el-node "local L1 execution node"
  wait_for_container_health l1-cl-node "local L1 consensus node"
  wait_for_l1_block_advance

  lineth_kv "rpc" "$(lineth_l1_host_rpc_url)"
}

cd "$STACK"

if [ "$L1_MODE" = "local" ]; then
  lineth_banner "start · local services + local L1 finality"
else
  lineth_banner "start · local services + Sepolia finality"
fi

lineth_section "Check Docker"
check_docker_preflight

lineth_section "Check ports"
lineth_run_stream env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$SCRIPT_DIR/check-ports.sh"

lineth_section "Check L1 network"
if [ "$L1_MODE" = "local" ]; then
  wait_for_local_l1
fi

run_ts_preflight

lineth_section "Generate accounts and configs"
bootstrap_log="/tmp/lineth-bootstrap.$$.$(date '+%Y%m%d%H%M%S').log"
# Stream bootstrap progress live instead of buffering it into a temp file; the
# noisy dependency install is already reduced to single progress lines inside
# phases/01-generate-accounts.sh. A status file carries the child exit code
# across the pipe because POSIX sh has no pipefail.
bootstrap_status_file="$bootstrap_log.status"
{
  env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$SCRIPT_DIR/bootstrap-artifacts.sh" 2>&1 \
    || printf '%s\n' "$?" > "$bootstrap_status_file"
} | tee "$bootstrap_log" | lineth_child_output
if [ -s "$bootstrap_status_file" ]; then
  rm -f "$bootstrap_status_file"
  lineth_error "account/config generation failed; raw output: $bootstrap_log"
  exit 1
fi
rm -f "$bootstrap_status_file"
lineth_info "raw generation output: $bootstrap_log"

# Print image references required by the compose profile that are not in the
# local Docker image store. Prints a sentinel line when the list itself cannot
# be resolved, so callers treat "unknown" as "missing".
missing_required_images() {
  required_images="$($COMPOSE config --images 2>/dev/null | sort -u)"
  if [ -z "$required_images" ]; then
    printf '%s' "(could not list required images via docker compose config --images)"
    return
  fi
  missing=""
  for required_image in $required_images; do
    docker image inspect "$required_image" >/dev/null 2>&1 \
      || missing="$missing $required_image"
  done
  printf '%s' "$missing"
}

if [ "$PULL" = "true" ]; then
  lineth_section "Pull Docker images"
  missing_images="$(missing_required_images)"
  if [ -z "$missing_images" ] && [ "${FORCE_PULL:-false}" != "true" ]; then
    # All images in versions.env are pinned, so a registry round-trip adds
    # nothing when every required image is already local — and it makes boot
    # time hostage to registry latency/outages.
    lineth_ok "all pinned images already local; skipping registry pull"
    lineth_info "force a registry refresh with ./scripts/start.sh --pull"
  else
    lineth_info "checking/pulling Docker images; this can take a while on a slow connection"
    pull_log="/tmp/lineth-docker-pull.$$.$(date '+%Y%m%d%H%M%S').log"
    # shellcheck disable=SC2086
    if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
      set +e
      COMPOSE_PROGRESS=plain $COMPOSE pull
      pull_status=$?
      set -e
    elif COMPOSE_PROGRESS=plain $COMPOSE pull > "$pull_log" 2>&1; then
      pull_status=0
    else
      pull_status=$?
    fi
    if [ "$pull_status" -ne 0 ]; then
      # Registry hiccups (e.g. transient Docker Hub 5xx on the auth token
      # endpoint) must not abort a boot that can run entirely from local
      # images. Only fail hard when a required image is actually missing.
      missing_images="$(missing_required_images)"
      if [ -z "$missing_images" ]; then
        lineth_warn "image pull/refresh failed (likely a transient registry issue), but all required images are already local; continuing"
        lineth_info "raw Docker pull output: $pull_log"
      else
        lineth_error "Docker image pull failed. This is usually a Docker Hub/network issue."
        [ "${LINETH_VERBOSE:-false}" = "true" ] || tail -60 "$pull_log" | lineth_indent
        lineth_info "missing images:$missing_images"
        lineth_info "retry the same command, or use ./scripts/start.sh --tail --no-pull if the images are already local"
        exit 1
      fi
    elif [ "${LINETH_VERBOSE:-false}" != "true" ]; then
      lineth_ok "Docker images are available"
      lineth_info "raw Docker pull output: $pull_log"
    fi
  fi
fi

lineth_section "Start services"
if [ "$TAIL" = "true" ]; then
  compose_log="/tmp/lineth-compose-up.$$.$(date '+%Y%m%d%H%M%S').log"
  lineth_kv "docker" "docker compose up -d"
  lineth_info "raw Docker service output: $compose_log"
  lineth_info "deployment and finality progress will stream below"
  # shellcheck disable=SC2086
  COMPOSE_PROGRESS=plain $COMPOSE up -d > "$compose_log" 2>&1 &
  compose_pid=$!

  interrupt_message="Stack startup is still running. Reattach with ./scripts/watch.sh or inspect raw logs with docker compose logs."
  trap 'lineth_info "$interrupt_message"; exit 130' INT
  i=0
  while [ "$i" -lt 60 ]; do
    if [ -n "$($COMPOSE ps -q l2-genesis-init 2>/dev/null || true)" ] \
      || [ -n "$($COMPOSE ps -q deploy-contracts 2>/dev/null || true)" ]; then
      break
    fi
    if ! kill -0 "$compose_pid" 2>/dev/null; then
      break
    fi
    sleep 1
    i=$((i + 1))
  done

  if ! kill -0 "$compose_pid" 2>/dev/null; then
    set +e
    wait "$compose_pid"
    status=$?
    set -e
    if [ "$status" -eq 0 ]; then
      if ! LINETH_SECTION_INDEX="$LINETH_SECTION_INDEX" LINETH_SUPPRESS_BANNER=1 "$SCRIPT_DIR/watch.sh"; then
        exit 1
      fi
      exit 0
    fi
    show_compose_failure "$compose_log"
    exit "$status"
  fi

  if ! LINETH_SECTION_INDEX="$LINETH_SECTION_INDEX" LINETH_SUPPRESS_BANNER=1 "$SCRIPT_DIR/watch.sh"; then
    wait "$compose_pid" >/dev/null 2>&1 || true
    exit 1
  fi
  set +e
  wait "$compose_pid"
  status=$?
  set -e
  if [ "$status" -ne 0 ]; then
    show_compose_failure "$compose_log"
    exit "$status"
  fi
else
  lineth_kv "docker" "docker compose up -d"
  # shellcheck disable=SC2086
  COMPOSE_PROGRESS=plain $COMPOSE up -d
  lineth_info "stack start requested; use ./scripts/watch.sh for deploy/proof/finality progress"
fi
