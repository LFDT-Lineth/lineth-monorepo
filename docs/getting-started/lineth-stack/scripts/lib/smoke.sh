#!/usr/bin/env sh
# Shared host-side Docker / Foundry / Postgres / claim helpers for the Lineth
# quickstart smoke-test and traffic-generation scripts.
#
# POSIX sh. This file is sourced from the host after lib/logging.sh and
# lib/runtime.sh, so it may rely on lineth_die (logging.sh) and on the artifact
# helpers exported by lineth_runtime_init (runtime.sh). It does not print output
# when sourced.

# Fail loudly when the Docker daemon is not reachable.
lineth_require_docker() {
  if ! docker info >/dev/null 2>&1; then
    lineth_die "Docker daemon is not reachable"
  fi
}

# Resolve the pinned Foundry image, honoring a caller-provided FOUNDRY_IMAGE and
# falling back to the FOUNDRY_TAG sourced from versions.env.
lineth_foundry_image() {
  printf '%s' "${FOUNDRY_IMAGE:-ghcr.io/foundry-rs/foundry:${FOUNDRY_TAG:-latest}}"
}

# Run a SQL statement against the Postman Postgres container and strip CRs.
lineth_psql_value() {
  docker exec postman-pg psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-postman}" -At -F '|' -c "$1" \
    | tr -d '\r'
}

# Derive the wallet address for a private key using the pinned Foundry image.
lineth_cast_wallet_address() {
  private_key="$1"
  docker run --rm \
    --entrypoint cast \
    "$(lineth_foundry_image)" wallet address --private-key "$private_key"
}

# Run a cast subcommand in the pinned Foundry image against a given RPC, on the
# quickstart Docker network. Usage: lineth_cast_run "<rpc_url>" <cast-args...>
lineth_cast_run() {
  rpc_url="$1"
  shift
  docker run --rm \
    --network lineth-stack_lineth \
    --entrypoint cast \
    "$(lineth_foundry_image)" "$@" --rpc-url "$rpc_url"
}

# Poll the Postman DB until the given message id has a CLAIMED_SUCCESS claim tx
# hash, or the deadline passes. Reads BRIDGE_SMOKE_POLL_SECONDS from the caller.
lineth_wait_postman_claim_tx() {
  message_id="$1"
  deadline="$2"

  while [ "$(date +%s)" -le "$deadline" ]; do
    claim_tx_hash="$(
      lineth_psql_value "select coalesce(claim_tx_hash,'') from message where id=$message_id and status='CLAIMED_SUCCESS';"
    )"
    if printf '%s' "$claim_tx_hash" | grep -qE '^0x[a-fA-F0-9]{64}$'; then
      printf '%s\n' "$claim_tx_hash"
      return 0
    fi
    # shellcheck disable=SC2154
    sleep "$BRIDGE_SMOKE_POLL_SECONDS"
  done

  return 1
}

# Submit a manual L2->L1 claim through the postman container using the shared
# claim-l2-to-l1.ts helper. Reads the message context globals set by the caller.
lineth_claim_l2_to_l1() {
  runtime_keys_env="$(lineth_accounts_file runtime-keys.env)"
  # shellcheck disable=SC1090
  . "$runtime_keys_env"
  l1_postman_private_key="${L1_POSTMAN_PRIVATE_KEY:-}"
  [ -n "$l1_postman_private_key" ] || lineth_die "L1_POSTMAN_PRIVATE_KEY missing from runtime-keys.env"

  # shellcheck disable=SC2154
  docker exec -i \
    -e L1_SIGNER_PRIVATE_KEY="$l1_postman_private_key" \
    -e SMOKE_L1_CHAIN_ID="$L1_CHAIN_ID" \
    -e SMOKE_L2_CHAIN_ID="$L2_CHAIN_ID" \
    -e SMOKE_LINETH_ROLLUP_ADDRESS="$LINETH_ROLLUP" \
    -e SMOKE_L2_MESSAGE_SERVICE_ADDRESS="$L2_MESSAGE_SERVICE" \
    -e SMOKE_MESSAGE_HASH="$MESSAGE_HASH" \
    -e SMOKE_MESSAGE_SENDER="$MESSAGE_SENDER" \
    -e SMOKE_DESTINATION="$DESTINATION" \
    -e SMOKE_FEE="$MESSAGE_FEE" \
    -e SMOKE_VALUE="$MESSAGE_VALUE" \
    -e SMOKE_MESSAGE_NONCE="$MESSAGE_NONCE" \
    -e SMOKE_CALLDATA="$MESSAGE_CALLDATA" \
    -e SMOKE_SENT_BLOCK_NUMBER="$SENT_BLOCK_NUMBER" \
    postman \
    sh -lc 'cd /usr/src/app/postman && node --input-type=module' \
    < "$LINETH_STACK_DIR/scripts/internal/claim-l2-to-l1.ts"
}
