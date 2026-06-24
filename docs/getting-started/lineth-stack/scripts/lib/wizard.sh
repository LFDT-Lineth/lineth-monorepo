#!/usr/bin/env sh
# Guided .env setup for the Lineth quickstart. POSIX sh; safe to source.

if [ "${LINETH_WIZARD_SH_LOADED:-false}" = "true" ]; then
  return 0
fi
LINETH_WIZARD_SH_LOADED=true

LINETH_WIZARD_MANAGED_KEYS="L1_MODE L1_RPC_URL PROVER_DEV_OVERRIDE PROVER_GOMEMLIMIT"

lineth_wizard_usage() {
  cat <<'EOF'
Usage: ./scripts/start.sh --wizard [options]

Guided setup:
  --wizard, --init          configure .env
  --then-start              after writing .env, exec ./scripts/start.sh --tail
  --yes                    skip the confirmation prompt

Non-interactive setup:
  --non-interactive         never prompt; fail if a required answer is missing
  --l1-mode <local|sepolia>
  --l1-rpc-url <url>        required for Sepolia when no existing .env value exists
  --prover <dev|partial>

Environment variables for non-interactive mode:
  WIZARD_L1_MODE            local or sepolia
  WIZARD_L1_RPC_URL         Sepolia HTTPS RPC URL
  WIZARD_PROVER             dev or partial

Precedence:
  flag > WIZARD_* env var > existing .env > .env.example default
EOF
}

lineth_wizard_is_http_url() {
  case "$1" in
    http://*|https://*) return 0 ;;
    *) return 1 ;;
  esac
}

lineth_wizard_redact_value() {
  if [ -z "$1" ]; then
    printf '(empty)'
  else
    printf '<redacted>'
  fi
}

lineth_wizard_env_value() {
  wizard_env_file="$1"
  wizard_env_key="$2"
  [ -f "$wizard_env_file" ] || return 1
  awk -v key="$wizard_env_key" '
    BEGIN { found = 0; prefix = key "=" }
    index($0, prefix) == 1 {
      value = substr($0, length(prefix) + 1)
      found = 1
    }
    END {
      if (found) {
        printf "%s", value
        exit 0
      }
      exit 1
    }
  ' "$wizard_env_file"
}

lineth_wizard_bool_to_prover() {
  case "$1" in
    false) printf 'partial' ;;
    *) printf 'dev' ;;
  esac
}

lineth_wizard_prover_to_bool() {
  case "$1" in
    partial) printf 'false' ;;
    *) printf 'true' ;;
  esac
}

lineth_wizard_pick_key_value() {
  wizard_pick_flag="$1"
  wizard_pick_env_name="$2"
  wizard_pick_file="$3"
  wizard_pick_key="$4"
  wizard_pick_example="$5"
  wizard_pick_fallback="$6"

  if [ -n "$wizard_pick_flag" ]; then
    printf '%s' "$wizard_pick_flag"
    return 0
  fi

  eval "wizard_pick_env_value=\${$wizard_pick_env_name:-}"
  if [ -n "$wizard_pick_env_value" ]; then
    printf '%s' "$wizard_pick_env_value"
    return 0
  fi

  if wizard_pick_value="$(lineth_wizard_env_value "$wizard_pick_file" "$wizard_pick_key")"; then
    printf '%s' "$wizard_pick_value"
    return 0
  fi

  if wizard_pick_value="$(lineth_wizard_env_value "$wizard_pick_example" "$wizard_pick_key")"; then
    printf '%s' "$wizard_pick_value"
    return 0
  fi

  printf '%s' "$wizard_pick_fallback"
}

lineth_wizard_resolve_defaults() {
  WIZARD_ENV_FILE="$LINETH_WIZARD_STACK/.env"
  WIZARD_EXAMPLE_FILE="$LINETH_WIZARD_STACK/.env.example"

  WIZARD_L1_MODE_RESOLVED="$(lineth_wizard_pick_key_value \
    "${LINETH_WIZARD_FLAG_L1_MODE:-}" WIZARD_L1_MODE "$WIZARD_ENV_FILE" L1_MODE "$WIZARD_EXAMPLE_FILE" sepolia)"
  WIZARD_L1_RPC_URL_RESOLVED="$(lineth_wizard_pick_key_value \
    "${LINETH_WIZARD_FLAG_L1_RPC_URL:-}" WIZARD_L1_RPC_URL "$WIZARD_ENV_FILE" L1_RPC_URL "$WIZARD_EXAMPLE_FILE" "")"

  if [ -n "${LINETH_WIZARD_FLAG_PROVER:-}" ]; then
    WIZARD_PROVER_RESOLVED="$LINETH_WIZARD_FLAG_PROVER"
  elif [ -n "${WIZARD_PROVER:-}" ]; then
    WIZARD_PROVER_RESOLVED="$WIZARD_PROVER"
  elif wizard_prover_bool="$(lineth_wizard_env_value "$WIZARD_ENV_FILE" PROVER_DEV_OVERRIDE)"; then
    WIZARD_PROVER_RESOLVED="$(lineth_wizard_bool_to_prover "$wizard_prover_bool")"
  elif wizard_prover_bool="$(lineth_wizard_env_value "$WIZARD_EXAMPLE_FILE" PROVER_DEV_OVERRIDE)"; then
    WIZARD_PROVER_RESOLVED="$(lineth_wizard_bool_to_prover "$wizard_prover_bool")"
  else
    WIZARD_PROVER_RESOLVED="dev"
  fi
}

lineth_wizard_validate_resolved() {
  case "$WIZARD_L1_MODE_RESOLVED" in
    local|sepolia) ;;
    *) lineth_die "invalid/missing L1 mode: expected local or sepolia" ;;
  esac

  case "$WIZARD_PROVER_RESOLVED" in
    dev|partial) ;;
    *) lineth_die "invalid/missing prover mode: expected dev or partial" ;;
  esac

  if [ "$WIZARD_L1_MODE_RESOLVED" = "sepolia" ]; then
    if [ -z "$WIZARD_L1_RPC_URL_RESOLVED" ]; then
      lineth_die "missing --l1-rpc-url / WIZARD_L1_RPC_URL for Sepolia mode"
    fi
    if ! lineth_wizard_is_http_url "$WIZARD_L1_RPC_URL_RESOLVED"; then
      lineth_die "invalid/missing L1 RPC URL: expected http(s)://..."
    fi
  fi
}

lineth_wizard_prompt_l1_mode() {
  wizard_prompt_default="$1"
  wizard_prompt_attempt=1

  while [ "$wizard_prompt_attempt" -le 3 ]; do
    if [ "$wizard_prompt_default" = "sepolia" ]; then
      wizard_prompt_default_number=2
    else
      wizard_prompt_default_number=1
    fi

    cat <<EOF
Choose L1 mode [$wizard_prompt_default_number]:

  1. Local L1
     Recommended for demos/dev. No Sepolia RPC, no real ETH.

  2. Sepolia
     Public finality. Needs a Sepolia RPC URL and funded deployer.

EOF
    printf 'Select 1/local or 2/sepolia: '
    if ! IFS= read -r wizard_prompt_answer; then
      lineth_die "invalid/missing L1 mode: choose 1/local or 2/sepolia"
    fi
    if [ -z "$wizard_prompt_answer" ]; then
      wizard_prompt_answer="$wizard_prompt_default"
    fi
    case "$wizard_prompt_answer" in
      1|local|Local|LOCAL)
        WIZARD_PROMPT_RESULT="local"
        return 0
        ;;
      2|sepolia|Sepolia|SEPOLIA)
        WIZARD_PROMPT_RESULT="sepolia"
        return 0
        ;;
    esac
    lineth_warn "invalid/missing L1 mode: choose 1/local or 2/sepolia"
    wizard_prompt_attempt=$((wizard_prompt_attempt + 1))
  done

  lineth_die "invalid/missing L1 mode: choose 1/local or 2/sepolia"
}

lineth_wizard_prompt_prover_mode() {
  wizard_prompt_default="$1"
  wizard_prompt_attempt=1

  while [ "$wizard_prompt_attempt" -le 3 ]; do
    if [ "$wizard_prompt_default" = "partial" ]; then
      wizard_prompt_default_number=2
    else
      wizard_prompt_default_number=1
    fi

    cat <<EOF
Choose prover mode [$wizard_prompt_default_number]:

  1. Dev proofs
     Recommended. Fast dummy proofs for laptops and demos.

  2. Partial prover
     Real partial proving. Needs much more Docker memory.

EOF
    printf 'Select 1/dev or 2/partial: '
    if ! IFS= read -r wizard_prompt_answer; then
      lineth_die "invalid/missing prover mode: choose 1/dev or 2/partial"
    fi
    if [ -z "$wizard_prompt_answer" ]; then
      wizard_prompt_answer="$wizard_prompt_default"
    fi
    case "$wizard_prompt_answer" in
      1|dev|Dev|DEV)
        WIZARD_PROMPT_RESULT="dev"
        return 0
        ;;
      2|partial|Partial|PARTIAL)
        WIZARD_PROMPT_RESULT="partial"
        return 0
        ;;
    esac
    lineth_warn "invalid/missing prover mode: choose 1/dev or 2/partial"
    wizard_prompt_attempt=$((wizard_prompt_attempt + 1))
  done

  lineth_die "invalid/missing prover mode: choose 1/dev or 2/partial"
}

lineth_wizard_prompt_url() {
  wizard_url_default="$1"
  wizard_url_attempt=1

  while [ "$wizard_url_attempt" -le 3 ]; do
    if [ -n "$wizard_url_default" ]; then
      printf 'Sepolia L1 RPC URL [keep current]: '
    else
      printf 'Sepolia L1 RPC URL: '
    fi
    if ! IFS= read -r wizard_url_answer; then
      lineth_die "missing --l1-rpc-url / WIZARD_L1_RPC_URL for Sepolia mode"
    fi
    if [ -z "$wizard_url_answer" ]; then
      wizard_url_answer="$wizard_url_default"
    fi
    if lineth_wizard_is_http_url "$wizard_url_answer"; then
      WIZARD_PROMPT_RESULT="$wizard_url_answer"
      return 0
    fi
    lineth_warn "invalid/missing L1 RPC URL: expected http(s)://..."
    wizard_url_attempt=$((wizard_url_attempt + 1))
  done

  lineth_die "invalid/missing L1 RPC URL: expected http(s)://..."
}

lineth_wizard_prompt_confirm() {
  wizard_confirm_default="$1"
  if [ "$wizard_confirm_default" = "yes" ]; then
    wizard_confirm_label="[Y/n]"
  else
    wizard_confirm_label="[y/N]"
  fi

  printf 'Write .env now? %s ' "$wizard_confirm_label"
  if ! IFS= read -r wizard_confirm_answer; then
    lineth_die "missing confirmation answer"
  fi

  case "$wizard_confirm_answer" in
    '')
      [ "$wizard_confirm_default" = "yes" ]
      ;;
    y|Y|yes|YES)
      return 0
      ;;
    n|N|no|NO)
      return 1
      ;;
    *)
      lineth_die "invalid confirmation answer"
      ;;
  esac
}

lineth_wizard_collect_interactive() {
  if [ -z "${LINETH_WIZARD_FLAG_L1_MODE:-}" ] && [ -z "${WIZARD_L1_MODE:-}" ]; then
    wizard_l1_prompt_default="$WIZARD_L1_MODE_RESOLVED"
    if [ ! -f "$WIZARD_ENV_FILE" ]; then
      wizard_l1_prompt_default="local"
    fi
    lineth_wizard_prompt_l1_mode "$wizard_l1_prompt_default"
    WIZARD_L1_MODE_RESOLVED="$WIZARD_PROMPT_RESULT"
  fi

  if [ "$WIZARD_L1_MODE_RESOLVED" = "sepolia" ] \
    && [ -z "${LINETH_WIZARD_FLAG_L1_RPC_URL:-}" ] \
    && [ -z "${WIZARD_L1_RPC_URL:-}" ]; then
    lineth_wizard_prompt_url "$WIZARD_L1_RPC_URL_RESOLVED"
    WIZARD_L1_RPC_URL_RESOLVED="$WIZARD_PROMPT_RESULT"
  fi

  if [ -z "${LINETH_WIZARD_FLAG_PROVER:-}" ] && [ -z "${WIZARD_PROVER:-}" ]; then
    wizard_prover_prompt_default="$WIZARD_PROVER_RESOLVED"
    if [ ! -f "$WIZARD_ENV_FILE" ]; then
      wizard_prover_prompt_default="dev"
    fi
    lineth_wizard_prompt_prover_mode "$wizard_prover_prompt_default"
    WIZARD_PROVER_RESOLVED="$WIZARD_PROMPT_RESULT"
  fi
}

lineth_wizard_set_env_key() {
  wizard_set_key="$1"
  wizard_set_value="$2"
  wizard_set_file="$3"
  wizard_set_tmp="${wizard_set_file}.$$.$(date '+%Y%m%d%H%M%S').tmp"

  # Values go through awk -v, which interprets backslash escapes; the
  # managed values (URLs, true/false, sizes) never contain backslashes.
  awk -v key="$wizard_set_key" -v value="$wizard_set_value" '
    BEGIN { replaced = 0; prefix = key "=" }
    index($0, prefix) == 1 {
      if (!replaced) {
        print key "=" value
        replaced = 1
      }
      next
    }
    { print }
    END {
      if (!replaced) {
        print key "=" value
      }
    }
  ' "$wizard_set_file" > "$wizard_set_tmp"
  mv "$wizard_set_tmp" "$wizard_set_file"
}

lineth_wizard_dedupe_managed_keys() {
  wizard_dedupe_file="$1"

  for wizard_dedupe_key in $LINETH_WIZARD_MANAGED_KEYS; do
    wizard_dedupe_count="$(awk -v key="$wizard_dedupe_key" '
      BEGIN { count = 0; prefix = key "=" }
      index($0, prefix) == 1 { count += 1 }
      END { print count }
    ' "$wizard_dedupe_file")"

    [ "$wizard_dedupe_count" -gt 1 ] || continue
    lineth_warn "$wizard_dedupe_key appears $wizard_dedupe_count times in .env; keeping last occurrence"

    wizard_dedupe_tmp="${wizard_dedupe_file}.$$.$wizard_dedupe_key.dedupe.tmp"
    awk -v key="$wizard_dedupe_key" -v count="$wizard_dedupe_count" '
      BEGIN { seen = 0; prefix = key "=" }
      index($0, prefix) == 1 {
        seen += 1
        if (seen < count) {
          next
        }
      }
      { print }
    ' "$wizard_dedupe_file" > "$wizard_dedupe_tmp"
    mv "$wizard_dedupe_tmp" "$wizard_dedupe_file"
  done
}

lineth_wizard_build_env_file() {
  wizard_build_target="$1"
  wizard_build_base="$WIZARD_EXAMPLE_FILE"
  [ -f "$WIZARD_ENV_FILE" ] && wizard_build_base="$WIZARD_ENV_FILE"
  [ -f "$wizard_build_base" ] || lineth_die ".env.example not found"

  cp "$wizard_build_base" "$wizard_build_target"
  lineth_wizard_dedupe_managed_keys "$wizard_build_target"

  lineth_wizard_set_env_key L1_MODE "$WIZARD_L1_MODE_RESOLVED" "$wizard_build_target"
  if [ "$WIZARD_L1_MODE_RESOLVED" = "local" ]; then
    lineth_wizard_set_env_key L1_RPC_URL "" "$wizard_build_target"
  else
    lineth_wizard_set_env_key L1_RPC_URL "$WIZARD_L1_RPC_URL_RESOLVED" "$wizard_build_target"
  fi

  lineth_wizard_set_env_key PROVER_DEV_OVERRIDE "$(lineth_wizard_prover_to_bool "$WIZARD_PROVER_RESOLVED")" "$wizard_build_target"
  if [ "$WIZARD_PROVER_RESOLVED" = "partial" ]; then
    lineth_wizard_set_env_key PROVER_GOMEMLIMIT "24GiB" "$wizard_build_target"
  fi
}

lineth_wizard_print_summary() {
  lineth_section "Wizard summary"
  lineth_kv "L1 mode" "$WIZARD_L1_MODE_RESOLVED"
  if [ "$WIZARD_L1_MODE_RESOLVED" = "sepolia" ]; then
    lineth_kv "L1 RPC URL" "$(lineth_wizard_redact_value "$WIZARD_L1_RPC_URL_RESOLVED")"
  else
    lineth_kv "L1 RPC URL" "(cleared for local mode)"
  fi
  lineth_kv "prover" "$WIZARD_PROVER_RESOLVED"
  if [ "$WIZARD_PROVER_RESOLVED" = "partial" ]; then
    lineth_kv "PROVER_GOMEMLIMIT" "24GiB"
  fi
}

lineth_wizard_diff_value() {
  wizard_diff_key="$1"
  wizard_diff_old="$2"
  wizard_diff_new="$3"
  [ "$wizard_diff_old" = "$wizard_diff_new" ] && return 0

  if [ "$wizard_diff_key" = "L1_RPC_URL" ]; then
    wizard_diff_old="$(lineth_wizard_redact_value "$wizard_diff_old")"
    wizard_diff_new="$(lineth_wizard_redact_value "$wizard_diff_new")"
  elif [ -z "$wizard_diff_old" ]; then
    wizard_diff_old="(empty)"
  elif [ -z "$wizard_diff_new" ]; then
    wizard_diff_new="(empty)"
  fi
  lineth_kv "$wizard_diff_key" "$wizard_diff_old -> $wizard_diff_new"
}

lineth_wizard_print_diff() {
  [ -f "$WIZARD_ENV_FILE" ] || return 0
  lineth_section "Managed-key changes"
  for wizard_diff_key in $LINETH_WIZARD_MANAGED_KEYS; do
    wizard_diff_old="$(lineth_wizard_env_value "$WIZARD_ENV_FILE" "$wizard_diff_key" || true)"
    wizard_diff_new="$(lineth_wizard_env_value "$WIZARD_CANDIDATE_ENV" "$wizard_diff_key" || true)"
    lineth_wizard_diff_value "$wizard_diff_key" "$wizard_diff_old" "$wizard_diff_new"
  done
}

lineth_wizard_state_exists() {
  if [ -n "${LINETH_WIZARD_STATE_EXISTS:-}" ]; then
    [ "$LINETH_WIZARD_STATE_EXISTS" = "true" ]
    return $?
  fi
  command -v docker >/dev/null 2>&1 || return 1
  if docker ps -a --format '{{.Names}}' 2>/dev/null \
    | grep -Eq '^(l1-el-node|l1-cl-node|l2-node-besu|coordinator|postman|prover|web3signer|shomei|maru|blockscout)($|-)'; then
    return 0
  fi
  docker volume ls --format '{{.Name}}' 2>/dev/null | grep -Eq '^lineth-stack-' && return 0
  return 1
}

lineth_wizard_existing_mode_flip() {
  [ -f "$WIZARD_ENV_FILE" ] || return 1
  wizard_flip_old_l1="$(lineth_wizard_env_value "$WIZARD_ENV_FILE" L1_MODE || true)"
  wizard_flip_new_l1="$(lineth_wizard_env_value "$WIZARD_CANDIDATE_ENV" L1_MODE || true)"
  wizard_flip_old_prover="$(lineth_wizard_env_value "$WIZARD_ENV_FILE" PROVER_DEV_OVERRIDE || true)"
  wizard_flip_new_prover="$(lineth_wizard_env_value "$WIZARD_CANDIDATE_ENV" PROVER_DEV_OVERRIDE || true)"

  [ -n "$wizard_flip_old_l1" ] && [ "$wizard_flip_old_l1" != "$wizard_flip_new_l1" ] && return 0
  [ -n "$wizard_flip_old_prover" ] && [ "$wizard_flip_old_prover" != "$wizard_flip_new_prover" ] && return 0
  return 1
}

lineth_wizard_guard_mode_switch() {
  if lineth_wizard_existing_mode_flip && lineth_wizard_state_exists; then
    lineth_error "existing stack state was detected and this changes L1/prover mode"
    lineth_info "run ./scripts/reset.sh yourself first, then rerun the wizard"
    return 1
  fi
  return 0
}

lineth_wizard_backup_env() {
  [ -f "$WIZARD_ENV_FILE" ] || return 0
  wizard_backup_dir="$LINETH_WIZARD_STACK/artifacts/env-backups"
  mkdir -p "$wizard_backup_dir"
  wizard_backup_timestamp="${LINETH_WIZARD_BACKUP_TIMESTAMP:-$(date '+%Y%m%d%H%M%S')}"
  wizard_backup_path="$wizard_backup_dir/.env.$wizard_backup_timestamp"
  wizard_backup_suffix=1
  while [ -e "$wizard_backup_path" ]; do
    wizard_backup_path="$wizard_backup_dir/.env.$wizard_backup_timestamp.$wizard_backup_suffix"
    wizard_backup_suffix=$((wizard_backup_suffix + 1))
  done
  cp "$WIZARD_ENV_FILE" "$wizard_backup_path"
  lineth_info "backup written: artifacts/env-backups/$(basename "$wizard_backup_path")"
}

lineth_wizard_check_rpc_deep() {
  [ "$WIZARD_L1_MODE_RESOLVED" = "sepolia" ] || return 0
  [ "${LINETH_WIZARD_SKIP_RPC_PREFLIGHT:-false}" = "true" ] && return 0
  if [ -n "${LINETH_WIZARD_RPC_CHECK_STATUS:-}" ]; then
    return "$LINETH_WIZARD_RPC_CHECK_STATUS"
  fi
  [ -n "${ROOT:-}" ] || return 0
  if ! lineth_ts_node_available "$ROOT"; then
    lineth_warn "host Sepolia RPC check skipped; run pnpm install for earlier chain checks"
    return 0
  fi

  lineth_info "Checking RPC..."
  wizard_rpc_tmp="$(mktemp)"
  if (
    export LINETH_STACK_DIR="$LINETH_WIZARD_STACK"
    export LINETH_PREFLIGHT_RPC_ONLY=true
    export L1_MODE="$WIZARD_L1_MODE_RESOLVED"
    export L1_RPC_URL="$WIZARD_L1_RPC_URL_RESOLVED"
    lineth_run_ts_node "$ROOT" "$LINETH_WIZARD_STACK/scripts/internal/quickstart-preflight.ts"
  ) > "$wizard_rpc_tmp" 2>&1; then
    lineth_child_output < "$wizard_rpc_tmp"
    rm -f "$wizard_rpc_tmp"
    lineth_ok "Sepolia RPC preflight completed"
    return 0
  fi

  lineth_child_output < "$wizard_rpc_tmp"
  rm -f "$wizard_rpc_tmp"
  return 1
}

lineth_wizard_rpc_was_supplied() {
  [ -n "${LINETH_WIZARD_FLAG_L1_RPC_URL:-}" ] || [ -n "${WIZARD_L1_RPC_URL:-}" ]
}

lineth_wizard_check_rpc_with_retries() {
  wizard_rpc_attempt=1
  while ! lineth_wizard_check_rpc_deep; do
    if [ "${LINETH_WIZARD_NON_INTERACTIVE:-false}" = "true" ] \
      || [ "$WIZARD_L1_MODE_RESOLVED" != "sepolia" ] \
      || lineth_wizard_rpc_was_supplied; then
      return 1
    fi
    if [ "$wizard_rpc_attempt" -ge 3 ]; then
      return 1
    fi
    lineth_warn "Sepolia RPC preflight failed; enter a different RPC URL"
    lineth_wizard_prompt_url "$WIZARD_L1_RPC_URL_RESOLVED"
    WIZARD_L1_RPC_URL_RESOLVED="$WIZARD_PROMPT_RESULT"
    lineth_wizard_validate_resolved
    wizard_rpc_attempt=$((wizard_rpc_attempt + 1))
  done
  return 0
}

lineth_wizard_check_ports_after_write() {
  [ "${LINETH_WIZARD_THEN_START:-false}" = "true" ] && return 0
  [ "${LINETH_WIZARD_SKIP_PORT_CHECK:-false}" = "true" ] && return 0

  if [ -n "${LINETH_WIZARD_PORT_CHECK_STATUS:-}" ]; then
    if [ "$LINETH_WIZARD_PORT_CHECK_STATUS" = "0" ]; then
      lineth_ok "ports are free"
      return 0
    fi
    lineth_error "port check failed"
    lineth_info "Edit the matching HOST_PORT_* values in .env, then rerun the wizard."
    return 1
  fi

  if env LINETH_EMBEDDED=true LINETH_SKIP_BANNER=true "$LINETH_WIZARD_SCRIPT_DIR/check-ports.sh"; then
    lineth_ok "ports are free"
    return 0
  fi
  lineth_info "Edit the matching HOST_PORT_* values in .env, then rerun the wizard."
  return 1
}

lineth_wizard_cleanup() {
  [ -n "${WIZARD_CANDIDATE_ENV:-}" ] && rm -f "$WIZARD_CANDIDATE_ENV"
}

lineth_wizard_cancelled() {
  lineth_wizard_cleanup
  lineth_info "cancelled"
  exit 130
}

lineth_wizard_main() {
  LINETH_WIZARD_STACK="$1"
  LINETH_WIZARD_SCRIPT_DIR="$2"
  WIZARD_CANDIDATE_ENV=""
  trap lineth_wizard_cancelled INT TERM

  lineth_wizard_resolve_defaults
  if [ "${LINETH_WIZARD_NON_INTERACTIVE:-false}" != "true" ]; then
    lineth_wizard_collect_interactive
  fi
  lineth_wizard_validate_resolved
  if ! lineth_wizard_check_rpc_with_retries; then
    lineth_wizard_cleanup
    return 1
  fi

  WIZARD_CANDIDATE_ENV="$LINETH_WIZARD_STACK/.env.$$.$(date '+%Y%m%d%H%M%S').tmp"
  lineth_wizard_build_env_file "$WIZARD_CANDIDATE_ENV"

  lineth_wizard_print_summary
  lineth_wizard_print_diff

  if [ -f "$WIZARD_ENV_FILE" ] && cmp -s "$WIZARD_ENV_FILE" "$WIZARD_CANDIDATE_ENV"; then
    lineth_info "no changes"
    lineth_wizard_cleanup
    return 0
  fi

  if [ "${LINETH_WIZARD_NON_INTERACTIVE:-false}" != "true" ] && [ "${LINETH_WIZARD_YES:-false}" != "true" ]; then
    if [ -f "$WIZARD_ENV_FILE" ]; then
      wizard_confirm_default="no"
    else
      wizard_confirm_default="yes"
    fi
    if ! lineth_wizard_prompt_confirm "$wizard_confirm_default"; then
      lineth_info "no changes written"
      lineth_wizard_cleanup
      return 0
    fi
  fi

  lineth_wizard_guard_mode_switch || {
    lineth_wizard_cleanup
    return 1
  }

  lineth_wizard_backup_env
  mv "$WIZARD_CANDIDATE_ENV" "$WIZARD_ENV_FILE"
  WIZARD_CANDIDATE_ENV=""
  lineth_ok ".env written"
  lineth_wizard_check_ports_after_write
}
