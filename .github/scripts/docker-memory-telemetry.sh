#!/usr/bin/env bash

set -u

TELEMETRY_DIR="${DOCKER_MEMORY_TELEMETRY_DIR:-docker_memory}"
COMPOSE_FILE="${DOCKER_MEMORY_COMPOSE_FILE:-docker/compose-tracing-v2-ci-extension.yml}"
COMPOSE_PROFILES_VALUE="${COMPOSE_PROFILES:-l1,l2}"
SAMPLE_INTERVAL_SECONDS="${DOCKER_MEMORY_SAMPLE_INTERVAL_SECONDS:-2}"
PROJECT_NAME="${DOCKER_MEMORY_COMPOSE_PROJECT:-docker}"
KNOWN_CONTAINER_NAMES=(
  coordinator
  docker-l1-node-genesis-generator-1
  l1-cl-node
  l1-el-node
  l2-genesis-intialization
  l2-node-besu
  maru
  postgres
  postman
  prover-v3
  sequencer
  shomei
  shomei-frontend
  transaction-exclusion-api
  web3signer
)

timestamp() {
  date -Is 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log() {
  mkdir -p "$TELEMETRY_DIR" 2>/dev/null || true
  printf "%s %s\n" "$(timestamp)" "$*" | tee -a "$TELEMETRY_DIR/telemetry.log" >/dev/null
}

to_mib() {
  if [ -z "${1:-}" ] || [ "$1" = "0" ]; then
    printf ""
    return
  fi

  awk -v bytes="$1" 'BEGIN { printf "%.2f", bytes / 1024 / 1024 }'
}

inspect_value() {
  local container_id="$1"
  local template="$2"

  docker inspect -f "$template" "$container_id" 2>/dev/null || printf ""
}

cgroup_peak_for_container() {
  local container_id="$1"
  local pid="$2"
  local file=""

  if [ -n "$pid" ] && [ "$pid" != "0" ] && [ -r "/proc/$pid/cgroup" ]; then
    while IFS=: read -r hierarchy controllers cgroup_path; do
      if [ "$hierarchy" = "0" ]; then
        file="/sys/fs/cgroup${cgroup_path}/memory.peak"
        if [ -r "$file" ]; then
          printf "%s\t%s" "$(cat "$file")" "cgroup-v2"
          return
        fi
      elif [ "$controllers" = "memory" ] || [[ ",$controllers," == *",memory,"* ]]; then
        for file in \
          "/sys/fs/cgroup/memory${cgroup_path}/memory.max_usage_in_bytes" \
          "/sys/fs/cgroup${cgroup_path}/memory.max_usage_in_bytes"; do
          if [ -r "$file" ]; then
            printf "%s\t%s" "$(cat "$file")" "cgroup-v1"
            return
          fi
        done
      fi
    done < "/proc/$pid/cgroup"
  fi

  for file in \
    "/sys/fs/cgroup/system.slice/docker-${container_id}.scope/memory.peak" \
    "/sys/fs/cgroup/docker/${container_id}/memory.peak"; do
    if [ -r "$file" ]; then
      printf "%s\t%s" "$(cat "$file")" "cgroup-v2-fallback"
      return
    fi
  done

  printf "\t"
}

within_limit() {
  local peak_bytes="$1"
  local limit_bytes="$2"

  if [ -z "$peak_bytes" ] || [ -z "$limit_bytes" ] || [ "$limit_bytes" = "0" ]; then
    printf "unknown"
  elif [ "$peak_bytes" -le "$limit_bytes" ]; then
    printf "true"
  else
    printf "false"
  fi
}

compose_ps() {
  local output_file="$1"

  COMPOSE_PROFILES="$COMPOSE_PROFILES_VALUE" docker compose -f "$COMPOSE_FILE" ps -a --format json \
    > "$output_file" 2>> "$TELEMETRY_DIR/telemetry.log" || true
}

start() {
  mkdir -p "$TELEMETRY_DIR" || exit 0
  : > "$TELEMETRY_DIR/telemetry.log"
  log "Starting Docker memory telemetry"
  log "Compose file: $COMPOSE_FILE"
  log "Compose profiles: $COMPOSE_PROFILES_VALUE"

  COMPOSE_PROFILES="$COMPOSE_PROFILES_VALUE" docker compose -f "$COMPOSE_FILE" config \
    > "$TELEMETRY_DIR/compose-resolved.yml" 2>> "$TELEMETRY_DIR/telemetry.log" || true
  compose_ps "$TELEMETRY_DIR/compose-ps-before.jsonl"

  printf "timestamp\tname\tmem_usage\tmem_percent\n" > "$TELEMETRY_DIR/docker-stats-samples.tsv"
  (
    while true; do
      sample_timestamp="$(timestamp)"
      sample_container_ids="$(container_ids | tr '\n' ' ')"
      if [ -n "$sample_container_ids" ]; then
        docker stats --no-stream --all --format "${sample_timestamp}"'	{{.Name}}	{{.MemUsage}}	{{.MemPerc}}' \
          $sample_container_ids >> "$TELEMETRY_DIR/docker-stats-samples.tsv" 2>> "$TELEMETRY_DIR/telemetry.log" || true
      else
        printf "%s\t%s\t\t\n" "$sample_timestamp" "(no matching containers)" \
          >> "$TELEMETRY_DIR/docker-stats-samples.tsv"
      fi
      sleep "$SAMPLE_INTERVAL_SECONDS"
    done
  ) &

  printf "%s\n" "$!" > "$TELEMETRY_DIR/sampler.pid"
  log "Docker stats sampler started with pid $(cat "$TELEMETRY_DIR/sampler.pid")"
}

stop_sampler() {
  if [ ! -f "$TELEMETRY_DIR/sampler.pid" ]; then
    return
  fi

  local sampler_pid
  sampler_pid="$(cat "$TELEMETRY_DIR/sampler.pid" 2>/dev/null || true)"
  if [ -n "$sampler_pid" ] && kill -0 "$sampler_pid" 2>/dev/null; then
    kill "$sampler_pid" 2>/dev/null || true
    log "Docker stats sampler stopped"
  fi
}

container_ids() {
  local ids

  ids="$(docker ps -aq --filter "label=com.docker.compose.project=${PROJECT_NAME}" 2>/dev/null || true)"
  if [ -n "$ids" ]; then
    printf "%s\n" "$ids"
    return
  fi

  for container_name in "${KNOWN_CONTAINER_NAMES[@]}"; do
    docker ps -aq --filter "name=^/${container_name}$" 2>/dev/null || true
  done | awk 'NF && !seen[$0]++'
}

capture() {
  mkdir -p "$TELEMETRY_DIR" || exit 0
  log "Capturing Docker memory telemetry"
  stop_sampler
  compose_ps "$TELEMETRY_DIR/compose-ps-after.jsonl"

  printf "name\tservice\tid\timage\tstatus\trestart_count\toom_killed\texit_code\tpid\tlimit_bytes\tlimit_mib\treservation_bytes\treservation_mib\tcgroup_peak_bytes\tcgroup_peak_mib\tcgroup_peak_source\twithin_limit\n" \
    > "$TELEMETRY_DIR/container-memory.tsv"

  while IFS= read -r container_id; do
    [ -n "$container_id" ] || continue

    local raw_name name service image status restart_count oom_killed exit_code pid limit_bytes reservation_bytes peak_info peak_bytes peak_source

    raw_name="$(inspect_value "$container_id" '{{.Name}}')"
    name="${raw_name#/}"
    service="$(inspect_value "$container_id" '{{index .Config.Labels "com.docker.compose.service"}}')"
    image="$(inspect_value "$container_id" '{{.Config.Image}}')"
    status="$(inspect_value "$container_id" '{{.State.Status}}')"
    restart_count="$(inspect_value "$container_id" '{{.RestartCount}}')"
    oom_killed="$(inspect_value "$container_id" '{{.State.OOMKilled}}')"
    exit_code="$(inspect_value "$container_id" '{{.State.ExitCode}}')"
    pid="$(inspect_value "$container_id" '{{.State.Pid}}')"
    limit_bytes="$(inspect_value "$container_id" '{{.HostConfig.Memory}}')"
    reservation_bytes="$(inspect_value "$container_id" '{{.HostConfig.MemoryReservation}}')"
    peak_info="$(cgroup_peak_for_container "$container_id" "$pid")"
    peak_bytes="${peak_info%%$'\t'*}"
    peak_source="${peak_info#*$'\t'}"

    printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" \
      "$name" \
      "$service" \
      "$container_id" \
      "$image" \
      "$status" \
      "$restart_count" \
      "$oom_killed" \
      "$exit_code" \
      "$pid" \
      "$limit_bytes" \
      "$(to_mib "$limit_bytes")" \
      "$reservation_bytes" \
      "$(to_mib "$reservation_bytes")" \
      "$peak_bytes" \
      "$(to_mib "$peak_bytes")" \
      "$peak_source" \
      "$(within_limit "$peak_bytes" "$limit_bytes")" \
      >> "$TELEMETRY_DIR/container-memory.tsv"
  done < <(container_ids)

  log "Docker memory telemetry captured"
}

case "${1:-}" in
  start)
    start || true
    ;;
  capture)
    capture || true
    ;;
  *)
    printf "Usage: %s {start|capture}\n" "$0" >&2
    exit 0
    ;;
esac
