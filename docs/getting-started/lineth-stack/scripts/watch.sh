#!/usr/bin/env sh
# Guided progress view for the Lineth quickstart boot/finality pipeline.
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
STACK_DIR="$(CDPATH= cd "$SCRIPT_DIR/.." && pwd -P)"
ACCOUNTS_ARTIFACTS_DIR="$STACK_DIR/artifacts/accounts"
DEPLOYMENTS_ARTIFACTS_DIR="$STACK_DIR/artifacts/deployments"
LINETH_LOG_CONTEXT="watch"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/logging.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/runtime.sh"
lineth_runtime_init "$SCRIPT_DIR"

WATCH_INTERVAL_SECONDS="${WATCH_INTERVAL_SECONDS:-10}"
WATCH_TIMEOUT_SECONDS="${WATCH_TIMEOUT_SECONDS:-1800}"
WATCH_HEARTBEAT_SECONDS="${WATCH_HEARTBEAT_SECONDS:-30}"
WATCH_SINCE="${WATCH_SINCE:-2h}"
WATCH_MODE="until-finalized"
LINETH_VERBOSE="${LINETH_VERBOSE:-false}"

usage() {
  cat <<'EOF'
Usage: ./scripts/watch.sh [--once|--tail|--verbose]

  --once   print one progress snapshot and exit
  --tail   keep printing progress snapshots until Ctrl-C
  --verbose
           include deploy transaction hashes, install details, and retry-noise detail

Environment:
  WATCH_INTERVAL_SECONDS   seconds between snapshots in follow mode (default: 10)
  WATCH_TIMEOUT_SECONDS    max seconds before watch exits in default mode (default: 1800)
  WATCH_HEARTBEAT_SECONDS  seconds between no-change heartbeats (default: 30)
  WATCH_SINCE              docker log window to inspect (default: 2h)
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --once)
      WATCH_MODE="once"
      ;;
    --tail)
      WATCH_MODE="tail"
      ;;
    --verbose)
      LINETH_VERBOSE=true
      export LINETH_VERBOSE
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

is_uint() {
  case "$1" in
    ''|*[!0-9]*) return 1 ;;
    *) return 0 ;;
  esac
}

container_state() {
  name="$1"
  state="$(docker inspect -f '{{.State.Status}} {{.State.ExitCode}}' "$name" 2>/dev/null || true)"
  if [ -n "$state" ]; then
    printf '%s\n' "$state"
  else
    printf 'missing\n'
  fi
}

shared_file_status() {
  file="$1"
  case "$file" in
    addresses-precomputed.json|runtime-keys.env|demo-traffic.env)
      path="$ACCOUNTS_ARTIFACTS_DIR/$file"
      ;;
    *)
      path="$DEPLOYMENTS_ARTIFACTS_DIR/$file"
      ;;
  esac
  if [ -f "$path" ]; then
    printf 'present'
  else
    printf 'missing'
  fi
}

latest_finalized_block() {
  docker logs --since "$WATCH_SINCE" coordinator 2>&1 \
    | sed -nE 's/.*finalization update: previousFinalizedBlock=[0-9]+ newFinalizedBlock=FinalizationUpdate\(blockNumber=([0-9]+),.*/\1/p' \
    | tail -1
}

latest_finalization_tx() {
  docker logs --since "$WATCH_SINCE" coordinator 2>&1 \
    | sed -nE 's/.*submitted aggregation=\[([0-9]+)\.\.([0-9]+)\][^ ]* transactionHash=(0x[a-fA-F0-9]{64}).*/range=[\1..\2] tx=\3/p' \
    | tail -1
}

latest_blob_tx() {
  docker logs --since "$WATCH_SINCE" coordinator 2>&1 \
    | sed -nE 's/.*blobs submitted: blobs=\[\[([0-9]+)\.\.([0-9]+)\][^ ]* transactionHash=(0x[a-fA-F0-9]{64}).*/range=[\1..\2] tx=\3/p' \
    | tail -1
}

account_setup_progress_lines() {
  if docker inspect account-setup >/dev/null 2>&1; then
    docker logs --tail 120 account-setup 2>&1 \
      | grep -E '\[account-setup\] (Preparing Node/ethers account setup runtime|Installing minimal workspace dependencies for TypeScript account setup|Using |Reused encrypted keystore|Wrote encrypted keystore|L1 deployer:|L1 deployer balance:|L1 deployer required minimum:|L1 safe listener start block|Wrote /accounts/addresses-precomputed.json|Pre-computed|Done)' \
      | awk 'length($0) > 220 { $0 = substr($0, 1, 220) "..." } { print }' \
      | tail -8
  fi
}

deployed_links_lines() {
  [ -f "$DEPLOYMENTS_ARTIFACTS_DIR/addresses.json" ] || return 0
  host_port="${HOST_PORT_L2_BLOCKSCOUT_FRONTEND:-4001}"
  l1_rollup="$(lineth_json_section_addr "$DEPLOYMENTS_ARTIFACTS_DIR/addresses.json" l1 LineaRollupV8)"
  l1_bridge="$(lineth_json_section_addr "$DEPLOYMENTS_ARTIFACTS_DIR/addresses.json" l1 TokenBridge)"
  l2_ms="$(lineth_json_section_addr "$DEPLOYMENTS_ARTIFACTS_DIR/addresses.json" l2 L2MessageService)"
  l2_bridge="$(lineth_json_section_addr "$DEPLOYMENTS_ARTIFACTS_DIR/addresses.json" l2 TokenBridge)"
  [ -n "$l1_rollup" ] && echo "L1 LineaRollupV8 $(lineth_l1_address_link "$l1_rollup")"
  [ -n "$l1_bridge" ] && echo "L1 TokenBridge $(lineth_l1_address_link "$l1_bridge")"
  [ -n "$l2_ms" ] && echo "L2 MessageService http://localhost:$host_port/address/$l2_ms"
  [ -n "$l2_bridge" ] && echo "L2 TokenBridge http://localhost:$host_port/address/$l2_bridge"
}

deploy_addresses_ready() {
  docker inspect deploy-contracts >/dev/null 2>&1 || return 1
  docker logs --tail 500 deploy-contracts 2>&1 \
    | grep -Eq '\[deploy-contracts\] ===== Aggregate addresses|wrote /deployments/addresses.json'
}

deploy_progress_lines() {
  if docker inspect deploy-contracts >/dev/null 2>&1; then
    docker logs --tail 500 deploy-contracts 2>&1 \
      | tr '\r' '\n' \
      | awk '
          function emit(kind, value) {
            if (value != "") {
              print kind ": " value
            }
          }

          /\[deploy-contracts\] ===== / {
            line = $0
            sub(/^.*\[deploy-contracts\] ===== /, "", line)
            sub(/ =====.*$/, "", line)
            emit("phase", line)
            next
          }

          /\[deploy-contracts\] Enabling corepack \+ pnpm/ {
            activity = "Corepack/pnpm"
            emit("detail", "Enabling corepack + pnpm")
            next
          }

          /\[deploy-contracts\] Installing Foundry/ {
            activity = "Foundry installer"
            emit("detail", "Installing Foundry")
            next
          }

          /\[deploy-contracts\] Installing workspace dependencies / {
            activity = "pnpm install"
            line = $0
            sub(/^.*\[deploy-contracts\] /, "", line)
            emit("detail", line)
            next
          }

          /\[deploy-contracts\] Compiling contracts / {
            activity = "Hardhat compile"
            line = $0
            sub(/^.*\[deploy-contracts\] /, "", line)
            emit("detail", line)
            next
          }

          /\[deploy-contracts\] (L1 RPC reachable|L2 RPC reachable|Waiting for Shomei |L2_GENESIS_STATE_ROOT |Funding |.* balance after funding: |wrote \/deployments\/addresses.json|wrote \/deployments\/deploy-runtime.env|Done)/ {
            line = $0
            sub(/^.*\[deploy-contracts\] /, "", line)
            emit("detail", line)
            next
          }

          /\[deploy-contracts\] Step 1: .*present .*skipping deploy/ {
            emit("reuse", "Step 1 reused: L1 Verifier + LineaRollup")
            next
          }

          /\[deploy-contracts\] Step 2: .*present .*skipping deploy/ {
            emit("reuse", "Step 2 reused: L2 MessageService")
            next
          }

          /\[deploy-contracts\] Step 3: .*present .*skipping deploy/ {
            emit("reuse", "Step 3 reused: L1 TokenBridge")
            next
          }

          /\[deploy-contracts\] Step 4: .*present .*skipping deploy/ {
            emit("reuse", "Step 4 reused: L2 TokenBridge")
            next
          }

          /\[fund-runtime-accounts\] / {
            line = $0
            sub(/^.*\[fund-runtime-accounts\] /, "", line)
            if (line ~ /(skipped:|no top-ups needed|funding .*: (value=|tx=|confirmed block=|finalBalance=)|ERROR:|Done\.)/) {
              emit("funding", line)
            }
            next
          }

          /! Corepack is about to download / {
            line = $0
            sub(/^.*! Corepack/, "! Corepack", line)
            activity = "pnpm download"
            emit("activity", line)
            next
          }

          /foundryup: / {
            line = $0
            sub(/^.*foundryup: /, "foundryup: ", line)
            if (line ~ /downloading forge, cast, anvil, and chisel/) {
              activity = "Foundry binaries"
            } else if (line ~ /downloading attestation artifact/) {
              activity = "Foundry attestation"
            } else if (line ~ /downloading manpages/) {
              activity = "Foundry manpages"
            } else if (line ~ /installing foundry/) {
              activity = "Foundry release"
            } else if (line ~ /checking if forge, cast, anvil, and chisel/) {
              activity = "Foundry cache check"
            } else if (line ~ /fetching latest release tag|resolved release tag/) {
              activity = "Foundry release lookup"
            }
            gsub(/attestation artifact/, "Foundry attestation", line)
            emit("activity", line)
            next
          }

          /forge Version: / {
            line = $0
            sub(/^.*forge Version: /, "forge Version: ", line)
            emit("detail", line)
            next
          }

          /Solidity .* is not fully supported yet/ {
            next
          }

          /Nothing to compile/ {
            emit("detail", "Nothing to compile")
            next
          }

          /No need to generate any newer typings/ {
            emit("detail", "No need to generate any newer typings")
            next
          }

          /^contract=/ {
            if ($0 ~ / pending: transactionHash=/) {
              name = $0
              sub(/^contract=/, "", name)
              sub(/ pending:.*/, "", name)

              hash = $0
              sub(/^.*transactionHash=/, "", hash)
              sub(/ .*/, "", hash)

              nonce = $0
              sub(/^.* nonce=/, "", nonce)
              sub(/ .*/, "", nonce)

              if (hash ~ /^0x[a-fA-F0-9]{64}$/ && nonce ~ /^[0-9]+$/) {
                emit("pending", name " nonce=" nonce " tx=" hash)
              }
              next
            }

            name = $0
            sub(/^contract=/, "", name)
            sub(/ deployed:.*/, "", name)

            address = $0
            sub(/^.*address=/, "", address)
            sub(/ .*/, "", address)

            block = $0
            sub(/^.*blockNumber=/, "", block)
            sub(/ .*/, "", block)

            if (address ~ /^0x[a-fA-F0-9]{40}$/ && block ~ /^[0-9]+$/) {
              emit("contract", name " block=" block " address=" address)
            }
            next
          }

          /\[deploy-contracts\] verify .*: OK/ {
            line = $0
            sub(/^.*\[deploy-contracts\] /, "", line)
            print line
            next
          }

          /\[deploy-contracts\] ERROR:/ {
            line = $0
            sub(/^.*\[deploy-contracts\] /, "", line)
            print line
            next
          }

          /insufficient funds for gas \* price \+ value:/ {
            print $0
            next
          }

          $NF ~ /^[0-9]+\.[0-9]%$/ {
            label = activity
            if (label == "") {
              label = "download"
            }
            emit("progress", label " " $NF)
            next
          }
        ' \
      | tail -18
  fi
}

deploy_summary() {
  state="$(container_state deploy-contracts)"
  precomputed_status="$(shared_file_status addresses-precomputed.json)"
  addresses_status="$(shared_file_status addresses.json)"

  if [ "$precomputed_status" != "present" ]; then
    lineth_kv "generated files" "missing addresses-precomputed.json"
    lineth_kv "l2-genesis-init" "$(container_state l2-genesis-init)"
    lineth_kv "config-render" "$(container_state config-render)"
  fi

  case "$state" in
    "missing")
      lineth_warn "deploy-contracts container not found"
      ;;
    "created "*)
      if [ "$precomputed_status" = "present" ]; then
        lineth_kv "deploy-contracts" "waiting for sequencer/shomei/web3signer"
      else
        lineth_kv "deploy-contracts" "waiting for generated files"
      fi
      ;;
    "running "*)
      lineth_kv "deploy-contracts" "running"
      ;;
    "exited 0")
      lineth_kv "deploy-contracts" "exited 0"
      ;;
    "exited "*)
      lineth_error "deploy-contracts $state"
      docker logs --tail 80 deploy-contracts 2>&1 \
        | grep -E 'ERROR|Error:|error code|insufficient funds|Failed|FATAL|ADDRESS MISMATCH|balance too low|Cannot fund' \
        | tail -12 \
        | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
        | lineth_indent || true
      ;;
    *)
      lineth_kv "deploy-contracts" "$state"
      ;;
  esac

  lineth_kv "addresses-precomputed.json" "$precomputed_status"
  lineth_kv "addresses.json" "$addresses_status"

  if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
    deploy_progress_lines | lineth_indent
    return
  fi

  phase="$(last_deploy_phase || true)"
  [ -n "$phase" ] && lineth_kv "latest deploy step" "$phase"

  reuse="$(deploy_reuse_lines || true)"
  if [ -n "$reuse" ]; then
    printf '%s\n' "$reuse" | while IFS= read -r line; do
      [ -n "$line" ] && lineth_kv "deploy reuse" "$line"
    done
  fi

  funding_done="$(deploy_progress_lines | sed -n 's/^funding: //p' | grep -E '^(no top-ups needed|Done\.)$' | tail -1 || true)"
  [ -n "$funding_done" ] && lineth_kv "runtime funding" "$funding_done"

  deploy_progress_lines \
    | sed -n 's/^detail: //p' \
    | grep -E 'wrote /deployments/(addresses\.json|deploy-runtime\.env)|^Done\.' \
    | tail -3 \
    | while IFS= read -r line; do
      [ -n "$line" ] && lineth_kv "deploy output" "$line"
    done
}

pipeline_events() {
  docker logs --since "$WATCH_SINCE" coordinator 2>&1 \
    | sed -nE '
        s/^time=([^ ]+) .*message=Started :\).*/\1 coordinator ready/p
        s/^time=([^ ]+) .*message=new batch: batch=\[([0-9]+)\.\.([0-9]+)\].*/\1 batch             [\2..\3]/p
        s/^time=([^ ]+) .*message=new blob: blob=\[([0-9]+)\.\.([0-9]+)\].*/\1 blob candidate    [\2..\3]/p
        s/^time=([^ ]+) .*message=execution proof request generated: proofIndex=ExecutionProofIndex\(startBlockNumber=([0-9]+), endBlockNumber=([0-9]+).*/\1 proof request     [\2..\3]/p
        s/^time=([^ ]+) .*message=execution proof generated: batch=Batch\(startBlockNumber=([0-9]+), endBlockNumber=([0-9]+)\).*/\1 execution proof   [\2..\3]/p
        s/^time=([^ ]+) .*message=blob compression proof generated: blob=\[([0-9]+)\.\.([0-9]+)\].*/\1 compression proof [\2..\3]/p
        s/^time=([^ ]+) .*message=aggregation proof generated: aggregation=\[([0-9]+)\.\.([0-9]+)\].*/\1 aggregation proof [\2..\3]/p
        s/^time=([^ ]+) .*message=eth_call for blob submission failed: blob=.*\[([0-9]+)\.\.([0-9]+)\].*reason: '\''max fee per gas less than block base fee'\''.*logger=BlobSubmissionCoordinator.*/\1 blob gas cap     [\2..\3] max fee < base fee/p
        s/^time=([^ ]+) .*message=blobs submitted: blobs=\[\[([0-9]+)\.\.([0-9]+)\][^ ]* transactionHash=(0x[a-fA-F0-9]{64}).*/\1 blob submitted    [\2..\3] tx=\4/p
        s/^time=([^ ]+) .*message=submitted aggregation=\[([0-9]+)\.\.([0-9]+)\][^ ]* transactionHash=(0x[a-fA-F0-9]{64}).*/\1 finalize tx       [\2..\3] tx=\4/p
        s/^time=([^ ]+) .*message=finalization update: previousFinalizedBlock=([0-9]+) newFinalizedBlock=FinalizationUpdate\(blockNumber=([0-9]+),.*/\1 finalized        L2 \2 -> \3/p
      ' \
    | tail -45
}

pipeline_timeline() {
  if ! docker inspect coordinator >/dev/null 2>&1; then
    lineth_warn "coordinator container not found"
    return
  fi
  coordinator_state="$(container_state coordinator)"
  case "$coordinator_state" in
    running\ *) ;;
    *)
      lineth_info "coordinator not running yet; waiting for deploy-contracts and runtime-config-finalize"
      return
      ;;
  esac

  events="$(pipeline_events)"

  if [ -n "$events" ]; then
    if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
      printf '%s\n' "$events" | lineth_indent
    else
      lineth_kv "latest event" "$(printf '%s\n' "$events" | tail -1)"
    fi
  else
    lineth_info "no coordinator pipeline events yet"
  fi
}

retry_summary() {
  if ! docker inspect coordinator >/dev/null 2>&1; then
    lineth_info "coordinator not created yet"
    return
  fi
  case "$(container_state coordinator)" in
    running\ *) ;;
    *)
      lineth_info "coordinator not running yet"
      return
      ;;
  esac

  retry_noise="$(retry_noise_line || true)"
  if [ -n "$retry_noise" ]; then
    lineth_kv "coordinator retry noise" "$retry_noise"
    lineth_info "retry noise is only a blocker if finalized L2 block stops advancing"
  else
    lineth_info "no coordinator retry noise seen in the selected log window"
  fi
}

retry_noise_line() {
  if ! docker inspect coordinator >/dev/null 2>&1; then
    return 0
  fi
  case "$(container_state coordinator)" in
    running\ *) ;;
    *) return 0 ;;
  esac

  retry_logs="$(docker logs --since "$WATCH_SINCE" coordinator 2>&1 || true)"
  root_mismatch_count="$(printf '%s\n' "$retry_logs" | grep -c 'StartingRootHashDoesNotMatch' || true)"
  already_known_count="$(printf '%s\n' "$retry_logs" | grep -c 'already known' || true)"
  nonce_low_count="$(printf '%s\n' "$retry_logs" | grep -c 'nonce too low' || true)"
  replacement_count="$(printf '%s\n' "$retry_logs" | grep -c 'replacement transaction underpriced' || true)"
  shnarf_count="$(printf '%s\n' "$retry_logs" | grep -c 'ShnarfAlreadySubmitted' || true)"
  insufficient_funds_count="$(printf '%s\n' "$retry_logs" | grep -c 'insufficient funds' || true)"
  fee_history_count="$(printf '%s\n' "$retry_logs" | grep -c 'Not enough fee history data' || true)"
  total_count=$((root_mismatch_count + already_known_count + nonce_low_count + replacement_count + shnarf_count + insufficient_funds_count + fee_history_count))

  if [ "$total_count" -gt 0 ]; then
    printf 'root=%s already-known=%s nonce-low=%s replacement=%s shnarf-already=%s insufficient-funds=%s fee-history=%s' \
      "$root_mismatch_count" \
      "$already_known_count" \
      "$nonce_low_count" \
      "$replacement_count" \
      "$shnarf_count" \
      "$insufficient_funds_count" \
      "$fee_history_count"
  fi
}

finality_summary() {
  case "$(container_state coordinator)" in
    running\ *) ;;
    *)
      lineth_info "finality starts after deploy-contracts and runtime-config-finalize complete"
      return
      ;;
  esac

  finalized="$(latest_finalized_block || true)"
  blob="$(latest_blob_tx || true)"
  finalization="$(latest_finalization_tx || true)"

  if [ -n "$blob" ]; then
    lineth_kv "latest blob tx" "$blob"
  else
    lineth_kv "latest blob tx" "none seen yet"
  fi

  if [ -n "$finalization" ]; then
    lineth_kv "latest finalization tx" "$finalization"
  else
    lineth_kv "latest finalization tx" "none seen yet"
  fi

  if [ -n "$finalized" ]; then
    lineth_kv "finalized L2 block" "$finalized"
  else
    lineth_kv "finalized L2 block" "0 or unavailable"
  fi
}

snapshot() {
  LINETH_SECTION_INDEX=0
  lineth_info "snapshot $(date '+%H:%M:%S')"

  lineth_section "Deploy contracts"
  deploy_summary

  lineth_section "Wait for finality"
  pipeline_timeline

  if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
    lineth_section "Coordinator retry noise"
    retry_summary
  fi

  lineth_section "Finality status"
  finality_summary

  lineth_section "Result"
  deploy_state="$(container_state deploy-contracts)"
  precomputed_status="$(shared_file_status addresses-precomputed.json)"
  addresses_status="$(shared_file_status addresses.json)"
  case "$deploy_state" in
    exited\ 0) ;;
    exited\ *)
      lineth_error "deploy failed; fund/fix the reported issue, then reset with: docker compose --env-file versions.env --env-file .env --profile stack-partial-prover down -v --remove-orphans"
      return 0
      ;;
    running\ *)
      lineth_info "deploy-contracts is running; keep this watcher open"
      return 0
      ;;
    created\ *)
      if [ "$precomputed_status" = "present" ]; then
        lineth_info "waiting for deploy-contracts dependencies to become healthy"
      else
        lineth_info "waiting for generated files to include addresses-precomputed.json"
      fi
      return 0
      ;;
    missing)
      lineth_info "Start services: waiting for quickstart containers"
      return 0
      ;;
    *)
      lineth_info "Start services: waiting for deploy-contracts"
      return 0
      ;;
  esac

  if [ "$addresses_status" != "present" ]; then
    lineth_info "deploy completed; waiting for addresses.json and runtime finalization"
    return 0
  fi

  finalized="$(latest_finalized_block || true)"
  if [ -n "$finalized" ] && is_uint "$finalized" && [ "$finalized" -gt 0 ]; then
    lineth_ok "first L1 finalization observed"
  else
    lineth_info "waiting for first L1 finalization; keep this watcher running"
  fi
}

elapsed_label() {
  elapsed="$1"
  minutes=$((elapsed / 60))
  seconds=$((elapsed % 60))
  if [ "$minutes" -gt 0 ]; then
    printf '%dm%02ds' "$minutes" "$seconds"
  else
    printf '%ds' "$seconds"
  fi
}

last_deploy_phase() {
  if docker inspect deploy-contracts >/dev/null 2>&1; then
    docker logs --since "$WATCH_SINCE" deploy-contracts 2>&1 \
      | tr '\r' '\n' \
      | sed -nE 's/.*\[deploy-contracts\] ===== ([^=].*[^[:space:]]) =====.*/\1/p' \
      | tail -1
  fi
}

last_deploy_contract() {
  if docker inspect deploy-contracts >/dev/null 2>&1; then
    docker logs --since "$WATCH_SINCE" deploy-contracts 2>&1 \
      | tr '\r' '\n' \
      | sed -nE 's/^contract=([^ ]+) deployed: address=(0x[a-fA-F0-9]{40}).*blockNumber=([0-9]+).*/\1 block=\3 address=\2/p' \
      | tail -1
  fi
}

last_deploy_pending() {
  deploy_progress_lines | sed -n 's/^pending: //p' | tail -1
}

deploy_reuse_lines() {
  deploy_progress_lines | sed -n 's/^reuse: //p'
}

last_deploy_detail() {
  deploy_progress_lines | sed -n 's/^detail: //p' | tail -1
}

last_deploy_activity() {
  deploy_progress_lines | sed -n 's/^activity: //p' | tail -1
}

last_deploy_progress() {
  deploy_progress_lines | sed -n 's/^progress: //p' | tail -1
}

progress_phase() {
  genesis_state="$(container_state l2-genesis-init)"
  render_state="$(container_state config-render)"
  deploy_state="$(container_state deploy-contracts)"
  runtime_state="$(container_state runtime-config-finalize)"
  coordinator_state="$(container_state coordinator)"
  prover_state="$(container_state prover)"
  postman_state="$(container_state postman)"
  precomputed_status="$(shared_file_status addresses-precomputed.json)"
  addresses_status="$(shared_file_status addresses.json)"

  if [ "$precomputed_status" != "present" ]; then
    printf 'preparing generated files: waiting for addresses-precomputed.json'
    return
  fi

  case "$genesis_state" in
    exited\ [!0]*)
      printf 'l2-genesis-init failed'
      return
      ;;
    running\ *|created\ *)
      printf 'preparing generated L2 genesis: l2-genesis-init %s' "$genesis_state"
      return
      ;;
  esac

  case "$render_state" in
    exited\ [!0]*)
      printf 'config-render failed'
      return
      ;;
    running\ *|created\ *)
      printf 'rendering generated service config: config-render %s' "$render_state"
      return
      ;;
  esac

  case "$deploy_state" in
    missing)
      printf 'Start services: waiting for deploy-contracts'
      return
      ;;
    exited\ [!0]*)
      printf 'deploy-contracts failed'
      return
      ;;
    created\ *)
      printf 'Start services: waiting for sequencer, Shomei, and Web3Signer health'
      return
      ;;
    running\ *)
      phase="$(last_deploy_phase || true)"
      [ -n "$phase" ] || phase="${last_seen_deploy_phase:-starting}"
      printf 'Deploy contracts: %s' "$phase"
      return
      ;;
  esac

  if [ "$addresses_status" != "present" ]; then
    printf 'Deploy contracts: waiting for addresses.json'
    return
  fi

  case "$runtime_state" in
    missing)
      printf 'Deploy contracts: waiting for coordinator postdeploy config'
      return
      ;;
    exited\ [!0]*)
      printf 'runtime-config-finalize failed'
      return
      ;;
    running\ *|created\ *)
      printf 'Deploy contracts: rendering coordinator postdeploy config %s' "$runtime_state"
      return
      ;;
  esac

  case "$coordinator_state/$prover_state/$postman_state" in
    running\ */running\ */running\ *) ;;
    *)
      printf 'Wait for finality: starting coordinator/prover/postman coordinator=%s prover=%s postman=%s' "$coordinator_state" "$prover_state" "$postman_state"
      return
      ;;
  esac

  finalized="$(latest_finalized_block || true)"
  if [ -n "$finalized" ] && is_uint "$finalized" && [ "$finalized" -gt 0 ]; then
    printf 'Wait for finality: finalized L2 block %s' "$finalized"
    return
  fi

  printf 'Wait for finality: waiting for first finalization'
}

progress_section_for_phase() {
  case "$1" in
    Deploy\ contracts:*)
      printf 'Deploy contracts'
      ;;
    Wait\ for\ finality:*)
      printf 'Wait for finality'
      ;;
  esac
}

failure_tail() {
  name="$1"
  docker logs --tail 100 "$name" 2>&1 \
    | grep -E 'ERROR|Error:|error code|insufficient funds|Failed|FATAL|ADDRESS MISMATCH|balance too low|Cannot fund|max fee per gas less than block base fee' \
    | tail -12 \
    | awk 'length($0) > 260 { $0 = substr($0, 1, 260) "..." } { print }' \
    | lineth_indent || true
}

check_terminal_failure() {
  for name in l2-genesis-init config-render deploy-contracts runtime-config-finalize coordinator prover postman; do
    state="$(container_state "$name")"
    case "$state" in
      exited\ 0|running\ *|created\ *|missing)
        ;;
      exited\ *)
        lineth_error "$name $state"
        failure_tail "$name"
        return 1
        ;;
    esac
  done
  return 0
}

progress_stream() {
  start_ts="$(date +%s)"
  last_phase=""
  last_seen_deploy_phase=""
  last_deploy_contract=""
  last_deploy_pending=""
  printed_deploy_reuse_lines=""
  last_deploy_detail=""
  last_deploy_activity=""
  last_deploy_progress=""
  last_account_setup_detail=""
  existing_addresses_notice_printed=""
  deployed_links_printed=""
  last_pipeline_event=""
  last_blob=""
  last_finalization=""
  last_finalized=""
  last_retry_noise=""
  first_finalized_seen=""
  last_heartbeat_ts="$start_ts"
  last_progress_section=""

  while :; do
    now_ts="$(date +%s)"
    elapsed="$(elapsed_label "$((now_ts - start_ts))")"
    changed=0

    phase="$(progress_phase)"
    progress_section="$(progress_section_for_phase "$phase")"
    if [ -n "$progress_section" ] && [ "$progress_section" != "$last_progress_section" ]; then
      lineth_section "$progress_section"
      last_progress_section="$progress_section"
      changed=1
    fi
    if [ "$phase" != "$last_phase" ]; then
      lineth_info "$phase ($elapsed)"
      last_phase="$phase"
      changed=1
    fi

    if ! check_terminal_failure; then
      exit 1
    fi

    account_setup_detail="$(account_setup_progress_lines | tail -1 || true)"
    if [ -n "$account_setup_detail" ] && [ "$account_setup_detail" != "$last_account_setup_detail" ]; then
      lineth_kv "account setup" "$account_setup_detail"
      last_account_setup_detail="$account_setup_detail"
      changed=1
    fi

    if [ -z "$existing_addresses_notice_printed" ] \
      && [ "$(shared_file_status addresses.json)" = "present" ] \
      && ! deploy_addresses_ready; then
      lineth_info "existing deployment addresses found; waiting for deploy-contracts to verify/reuse them"
      existing_addresses_notice_printed=1
      changed=1
    fi

    deploy_phase="$(last_deploy_phase || true)"
    if [ -n "$deploy_phase" ] && [ "$deploy_phase" != "$last_seen_deploy_phase" ]; then
      lineth_kv "Deploy step" "$deploy_phase"
      last_seen_deploy_phase="$deploy_phase"
      changed=1
    fi

    deploy_contract="$(last_deploy_contract || true)"
    if [ "${LINETH_VERBOSE:-false}" = "true" ] \
      && [ -n "$deploy_contract" ] \
      && [ "$deploy_contract" != "$last_deploy_contract" ]; then
      lineth_kv "Deployed contract" "$deploy_contract"
      last_deploy_contract="$deploy_contract"
      changed=1
    fi

    deploy_pending="$(last_deploy_pending || true)"
    if [ "${LINETH_VERBOSE:-false}" = "true" ] \
      && [ -n "$deploy_pending" ] \
      && [ "$deploy_pending" != "$last_deploy_pending" ]; then
      lineth_kv "Pending deploy tx" "$deploy_pending"
      last_deploy_pending="$deploy_pending"
      changed=1
    fi

    deploy_reuse_lines="$(deploy_reuse_lines || true)"
    if [ -n "$deploy_reuse_lines" ]; then
      printf '%s\n' "$deploy_reuse_lines" | while IFS= read -r deploy_reuse; do
        [ -n "$deploy_reuse" ] || continue
        if ! printf '%s\n' "$printed_deploy_reuse_lines" | grep -Fxq "$deploy_reuse"; then
          lineth_kv "Deploy reuse" "$deploy_reuse"
        fi
      done
      new_reuse_lines="$(printf '%s\n' "$deploy_reuse_lines" | while IFS= read -r deploy_reuse; do
        [ -n "$deploy_reuse" ] || continue
        if ! printf '%s\n' "$printed_deploy_reuse_lines" | grep -Fxq "$deploy_reuse"; then
          printf '%s\n' "$deploy_reuse"
        fi
      done)"
      if [ -n "$new_reuse_lines" ]; then
        printed_deploy_reuse_lines="${printed_deploy_reuse_lines}${printed_deploy_reuse_lines:+
}$new_reuse_lines"
        changed=1
      fi
    fi

    deploy_detail="$(last_deploy_detail || true)"
    if [ -n "$deploy_detail" ] && [ "$deploy_detail" != "$last_deploy_detail" ]; then
      case "$deploy_detail" in
        wrote\ /deployments/addresses.json*|wrote\ /deployments/deploy-runtime.env*|Done*)
          lineth_kv "Deploy detail" "$deploy_detail"
          changed=1
          ;;
        *)
          if [ "${LINETH_VERBOSE:-false}" = "true" ]; then
            lineth_kv "Deploy detail" "$deploy_detail"
            changed=1
          fi
          ;;
      esac
      last_deploy_detail="$deploy_detail"
    fi

    deploy_activity="$(last_deploy_activity || true)"
    if [ "${LINETH_VERBOSE:-false}" = "true" ] \
      && [ -n "$deploy_activity" ] \
      && [ "$deploy_activity" != "$last_deploy_activity" ]; then
      lineth_kv "Deploy activity" "$deploy_activity"
      last_deploy_activity="$deploy_activity"
      changed=1
    fi

    deploy_progress="$(last_deploy_progress || true)"
    if [ "${LINETH_VERBOSE:-false}" = "true" ] \
      && [ -n "$deploy_progress" ] \
      && [ "$deploy_progress" != "$last_deploy_progress" ]; then
      lineth_kv "Deploy progress" "$deploy_progress"
      last_deploy_progress="$deploy_progress"
      changed=1
    fi

    if [ -z "$deployed_links_printed" ] \
      && [ "$(shared_file_status addresses.json)" = "present" ] \
      && deploy_addresses_ready; then
      links="$(deployed_links_lines || true)"
      if [ -n "$links" ]; then
        if [ "$last_progress_section" != "Show links" ]; then
          lineth_section "Show links"
          last_progress_section="Show links"
        fi
        lineth_kv "links" "deployment addresses are ready"
        printf '%s\n' "$links" | lineth_indent
        deployed_links_printed=1
        changed=1
      fi
    fi

    case "$(container_state coordinator)" in
      running\ *)
      if [ "$last_progress_section" != "Wait for finality" ]; then
        lineth_section "Wait for finality"
        last_progress_section="Wait for finality"
        changed=1
      fi

      pipeline_event="$(pipeline_events | tail -1 || true)"
      if [ -n "$pipeline_event" ] && [ "$pipeline_event" != "$last_pipeline_event" ]; then
        lineth_kv "Finality pipeline" "$pipeline_event"
        last_pipeline_event="$pipeline_event"
        changed=1
      fi

      blob="$(latest_blob_tx || true)"
      if [ -n "$blob" ] && [ "$blob" != "$last_blob" ]; then
        lineth_kv "Blob tx" "$blob"
        last_blob="$blob"
        changed=1
      fi

      finalization="$(latest_finalization_tx || true)"
      if [ -n "$finalization" ] && [ "$finalization" != "$last_finalization" ]; then
        lineth_kv "Finalization tx" "$finalization"
        last_finalization="$finalization"
        changed=1
      fi

      finalized="$(latest_finalized_block || true)"
      if [ -n "$finalized" ] && [ "$finalized" != "$last_finalized" ]; then
        if [ "$WATCH_MODE" != "until-finalized" ] && [ -z "$first_finalized_seen" ] && is_uint "$finalized" && [ "$finalized" -gt 0 ]; then
          blob="$(latest_blob_tx || true)"
          finalization="$(latest_finalization_tx || true)"
          [ -n "$blob" ] && lineth_kv "Blob tx" "$blob"
          [ -n "$finalization" ] && lineth_kv "Finalization tx" "$finalization"
          lineth_ok "first L1 finalization observed at L2 block $finalized"
          first_finalized_seen=1
        fi
        lineth_kv "Finalized L2 block" "$finalized"
        last_finalized="$finalized"
        changed=1
      fi

      retry_noise="$(retry_noise_line || true)"
      if [ "${LINETH_VERBOSE:-false}" = "true" ] \
        && [ -n "$retry_noise" ] \
        && [ "$retry_noise" != "$last_retry_noise" ]; then
        lineth_kv "Coordinator retry noise" "$retry_noise"
        lineth_info "retry noise is only a blocker if finalized L2 block stops advancing"
        last_retry_noise="$retry_noise"
        changed=1
      fi
      ;;
    esac

    finalized="$(latest_finalized_block || true)"
    if [ "$WATCH_MODE" = "until-finalized" ] && [ -n "$finalized" ] && is_uint "$finalized" && [ "$finalized" -gt 0 ]; then
      blob="$(latest_blob_tx || true)"
      finalization="$(latest_finalization_tx || true)"
      if [ "$last_progress_section" != "Result" ]; then
        lineth_section "Result"
        last_progress_section="Result"
      fi
      lineth_kv "finality" "first L1 finality observed"
      [ -n "$blob" ] && lineth_kv "Blob tx" "$blob"
      [ -n "$finalization" ] && lineth_kv "Finalization tx" "$finalization"
      lineth_kv "Finalized L2 block" "$finalized"
      lineth_ok "first L1 finalization observed at L2 block $finalized"
      lineth_info "next: ./scripts/status.sh or ./scripts/links.sh"
      lineth_info "support bundle, only if you need to share/debug the run: ./scripts/export-output.sh"
      exit 0
    fi

    if [ "$changed" -eq 1 ]; then
      last_heartbeat_ts="$now_ts"
    elif [ $((now_ts - last_heartbeat_ts)) -ge "$WATCH_HEARTBEAT_SECONDS" ]; then
      case "$phase" in
        Deploy\ contracts:*)
          if [ -n "$last_deploy_activity" ]; then
            lineth_info "still $phase ($(elapsed_label "$((now_ts - start_ts))")); last deploy activity: $last_deploy_activity"
          elif [ -n "$last_deploy_detail" ]; then
            lineth_info "still $phase ($(elapsed_label "$((now_ts - start_ts))")); last deploy detail: $last_deploy_detail"
          else
            lineth_info "still $phase ($(elapsed_label "$((now_ts - start_ts))"))"
          fi
          ;;
        *)
          if [ -n "$last_pipeline_event" ]; then
            lineth_info "still $phase ($(elapsed_label "$((now_ts - start_ts))")); last pipeline event: $last_pipeline_event"
          else
            lineth_info "still $phase ($(elapsed_label "$((now_ts - start_ts))"))"
          fi
          ;;
      esac
      last_heartbeat_ts="$now_ts"
    fi

    elapsed_total=$((now_ts - start_ts))
    if [ "$WATCH_MODE" = "until-finalized" ] && [ "$elapsed_total" -ge "$WATCH_TIMEOUT_SECONDS" ]; then
      lineth_warn "watch timed out before first finalization; inspect ./scripts/status.sh and coordinator/prover logs"
      exit 1
    fi

    sleep "$WATCH_INTERVAL_SECONDS"
  done
}

if ! docker info >/dev/null 2>&1; then
  lineth_die "Docker daemon is not reachable from this shell."
fi

case "$WATCH_INTERVAL_SECONDS" in
  ''|*[!0-9]*) lineth_die "WATCH_INTERVAL_SECONDS must be a positive integer" ;;
esac
case "$WATCH_TIMEOUT_SECONDS" in
  ''|*[!0-9]*) lineth_die "WATCH_TIMEOUT_SECONDS must be a positive integer" ;;
esac
case "$WATCH_HEARTBEAT_SECONDS" in
  ''|*[!0-9]*) lineth_die "WATCH_HEARTBEAT_SECONDS must be a positive integer" ;;
esac

if [ "${LINETH_SUPPRESS_BANNER:-0}" != "1" ]; then
  lineth_banner "watch · deploy + proof/finality timeline"
fi

if [ "$WATCH_MODE" = "once" ]; then
  snapshot
  exit 0
fi

progress_stream
