#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd "$(dirname "$0")" && pwd -P)"
REAL_STACK="$(CDPATH= cd "$SCRIPT_DIR/../../.." && pwd -P)"
TMP_DIR="$(mktemp -d)"
STACK="$TMP_DIR/stack"
ENV_FILE="$STACK/.env"
BACKUP_DIR="$STACK/artifacts/env-backups"
START_SH="$REAL_STACK/scripts/start.sh"
FAILURES=0

mkdir -p "$BACKUP_DIR"
cp "$REAL_STACK/.env.example" "$STACK/.env.example"
export LINETH_WIZARD_STACK_OVERRIDE="$STACK"

cleanup() {
  rm -f "$BACKUP_DIR"/.env.test-"$$"* "$BACKUP_DIR"/.env.noop-"$$"* \
    "$BACKUP_DIR"/.env.guard-"$$"* "$BACKUP_DIR"/.env.guard-save-"$$"* \
    "$BACKUP_DIR"/.env.devmem-"$$"*
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

pass() {
  printf '[wizard-test] OK: %s\n' "$*"
}

fail() {
  printf '[wizard-test] FAIL: %s\n' "$*" >&2
  FAILURES=$((FAILURES + 1))
}

assert_file_contains() {
  file="$1"
  needle="$2"
  label="$3"
  if grep -qF -- "$needle" "$file"; then
    pass "$label"
  else
    fail "$label"
  fi
}

assert_file_not_contains() {
  file="$1"
  needle="$2"
  label="$3"
  if grep -qF -- "$needle" "$file"; then
    fail "$label"
  else
    pass "$label"
  fi
}

run_wizard() {
  env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    "$START_SH" "$@"
}

write_managed_snapshot() {
  snapshot_file="$1"
  snapshot_out="$2"
  : > "$snapshot_out"
  for snapshot_key in L1_MODE L1_RPC_URL PROVER_DEV_OVERRIDE PROVER_GOMEMLIMIT; do
    if awk -v key="$snapshot_key" 'index($0, key "=") == 1 { found = 1 } END { exit found ? 0 : 1 }' "$snapshot_file"; then
      awk -v key="$snapshot_key" 'index($0, key "=") == 1 { value = $0 } END { print value }' "$snapshot_file" >> "$snapshot_out"
    fi
  done
}

assert_fixture() {
  fixture_name="$1"
  snapshot="$TMP_DIR/$fixture_name.actual"
  write_managed_snapshot "$ENV_FILE" "$snapshot"
  if cmp -s "$snapshot" "$SCRIPT_DIR/fixtures/$fixture_name.env"; then
    pass "$fixture_name managed-key output matches fixture file"
  else
    fail "$fixture_name managed-key output matches fixture file"
  fi
}

reset_env() {
  rm -f "$ENV_FILE"
  unset WIZARD_L1_MODE WIZARD_L1_RPC_URL WIZARD_PROVER
}

if sh -n "$REAL_STACK/scripts/start.sh" "$REAL_STACK/scripts/lib/wizard.sh"; then
  pass "start.sh and wizard.sh have valid shell syntax"
else
  fail "start.sh and wizard.sh have valid shell syntax"
fi

if command -v shellcheck >/dev/null 2>&1; then
  if shellcheck "$REAL_STACK/scripts/start.sh" "$REAL_STACK/scripts/lib/wizard.sh"; then
    pass "start.sh and wizard.sh pass shellcheck"
  else
    fail "start.sh and wizard.sh pass shellcheck"
  fi
else
  pass "shellcheck not installed; skipped shellcheck"
fi

(
  # shellcheck disable=SC1091
  . "$REAL_STACK/scripts/lib/wizard.sh"
  test_env="$TMP_DIR/set-env.env"
  expected="$TMP_DIR/set-env.expected"
  printf '# preserved comment\nA=1\nL1_RPC_URL=old\nTRAIL=ok\n' > "$test_env"
  lineth_wizard_set_env_key L1_RPC_URL 'https://example.test/a/b?x=1&y=2#frag ' "$test_env"
  lineth_wizard_set_env_key NEW_KEY 'a/b&c=d#e' "$test_env"
  printf '# preserved comment\nA=1\nL1_RPC_URL=https://example.test/a/b?x=1&y=2#frag \nTRAIL=ok\nNEW_KEY=a/b&c=d#e\n' > "$expected"
  cmp -s "$test_env" "$expected"
) && pass "lineth_wizard_set_env_key preserves comments and URL-like values literally" \
  || fail "lineth_wizard_set_env_key preserves comments and URL-like values literally"

reset_env
if run_wizard --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-local-dev.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "local dev writes L1_MODE"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=' "local dev clears L1_RPC_URL"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=true' "local dev writes dev prover mode"
  assert_fixture local-dev
else
  fail "local dev non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode local --prover partial >/tmp/lineth-wizard-local-partial.$$ 2>&1; then
  assert_fixture local-partial
else
  fail "local partial non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode sepolia --l1-rpc-url 'https://rpc.example.test/key?a=1&b=2' --prover dev >/tmp/lineth-wizard-sepolia-dev.$$ 2>&1; then
  assert_fixture sepolia-dev
else
  fail "sepolia dev non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode sepolia --l1-rpc-url 'https://rpc.example.test/key?a=1&b=2' --prover partial >/tmp/lineth-wizard-sepolia-partial.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "sepolia partial writes L1_MODE"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=https://rpc.example.test/key?a=1&b=2' "sepolia partial writes URL literally"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "sepolia partial writes partial prover mode"
  assert_file_contains "$ENV_FILE" 'PROVER_GOMEMLIMIT=24GiB' "sepolia partial pins PROVER_GOMEMLIMIT"
  assert_fixture sepolia-partial
else
  fail "sepolia partial non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode sepolia --prover dev >/tmp/lineth-wizard-missing-rpc.$$ 2>&1; then
  fail "sepolia without RPC fails"
else
  assert_file_contains /tmp/lineth-wizard-missing-rpc.$$ 'missing --l1-rpc-url / WIZARD_L1_RPC_URL' "sepolia without RPC explains missing value"
fi

reset_env
if WIZARD_L1_MODE=sepolia WIZARD_L1_RPC_URL=https://env.example.test run_wizard --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-precedence.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "flag beats WIZARD_L1_MODE env var"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=' "local flag clears env-provided RPC URL"
else
  fail "flag precedence run succeeds"
fi

reset_env
if WIZARD_L1_MODE=local WIZARD_PROVER=dev run_wizard --wizard --non-interactive >/tmp/lineth-wizard-env-only.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "WIZARD_L1_MODE env var configures non-interactive mode"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=true' "WIZARD_PROVER env var configures non-interactive mode"
else
  fail "env-only non-interactive wizard run succeeds"
fi

mkdir -p "$BACKUP_DIR"
rm -f "$BACKUP_DIR/.env.test-$$"
cat > "$ENV_FILE" <<'EOF'
# hand tuned
L1_MODE=sepolia
L1_RPC_URL=https://secret.example.test/api-key
PROVER_DEV_OVERRIDE=true
CUSTOM_HAND_ADDED=keep-me
EOF
if LINETH_WIZARD_BACKUP_TIMESTAMP="test-$$" run_wizard --wizard --non-interactive --l1-mode local --prover partial >/tmp/lineth-wizard-backup.$$ 2>&1; then
  [ -f "$BACKUP_DIR/.env.test-$$" ] && pass "overwrite creates timestamped backup" || fail "overwrite creates timestamped backup"
  assert_file_contains "$ENV_FILE" 'CUSTOM_HAND_ADDED=keep-me' "unknown .env key is preserved"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=' "local overwrite clears stale Sepolia RPC"
  assert_file_contains "$ENV_FILE" 'PROVER_GOMEMLIMIT=24GiB' "partial overwrite pins GOMEMLIMIT"
else
  fail "backup/preserve run succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=local
CUSTOM_HAND_ADDED=keep-me
L1_MODE=sepolia
L1_RPC_URL=https://rpc.example.test/key
PROVER_DEV_OVERRIDE=true
EOF
unset WIZARD_L1_MODE WIZARD_L1_RPC_URL WIZARD_PROVER
if run_wizard --wizard --non-interactive --prover dev >/tmp/lineth-wizard-duplicate-keys.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-duplicate-keys.$$ 'L1_MODE appears 2 times in .env; keeping last occurrence' "duplicate managed keys warn before normalization"
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "duplicate managed key keeps last-read value"
  [ "$(grep -c '^L1_MODE=' "$ENV_FILE")" -eq 1 ] \
    && pass "duplicate managed key is normalized to one entry" \
    || fail "duplicate managed key is normalized to one entry"
  assert_file_contains "$ENV_FILE" 'CUSTOM_HAND_ADDED=keep-me' "duplicate normalization preserves unknown keys"
else
  fail "duplicate managed-key run succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=local
L1_RPC_URL=
PROVER_DEV_OVERRIDE=false
PROVER_GOMEMLIMIT=24GiB
CUSTOM_HAND_ADDED=keep-me
EOF
rm -f "$BACKUP_DIR/.env.noop-$$"
if LINETH_WIZARD_BACKUP_TIMESTAMP="noop-$$" run_wizard --wizard --non-interactive --l1-mode local --prover partial >/tmp/lineth-wizard-noop.$$ 2>&1; then
  [ ! -f "$BACKUP_DIR/.env.noop-$$" ] && pass "no-op rerun skips backup" || fail "no-op rerun skips backup"
  assert_file_contains /tmp/lineth-wizard-noop.$$ 'no changes' "no-op rerun reports no changes"
  assert_file_not_contains /tmp/lineth-wizard-noop.$$ 'Configuration changes' "no-op rerun omits empty Configuration changes section"
else
  fail "no-op rerun succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=sepolia
L1_RPC_URL=https://secret.example.test/api-key
PROVER_DEV_OVERRIDE=true
EOF
if LINETH_WIZARD_BACKUP_TIMESTAMP="test-$$" run_wizard --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-backup-collision.$$ 2>&1; then
  [ -f "$BACKUP_DIR/.env.test-$$.1" ] && pass "backup collision keeps both backups" || fail "backup collision keeps both backups"
else
  fail "backup collision run succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=local
L1_RPC_URL=
PROVER_DEV_OVERRIDE=false
PROVER_GOMEMLIMIT=24GiB
EOF
if LINETH_WIZARD_BACKUP_TIMESTAMP="devmem-$$" run_wizard --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-dev-gomemlimit.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=true' "switching to dev writes dev override"
  assert_file_contains "$ENV_FILE" 'PROVER_GOMEMLIMIT=24GiB' "dev mode preserves existing PROVER_GOMEMLIMIT by design"
else
  fail "dev mode preserving existing PROVER_GOMEMLIMIT run succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=sepolia
L1_RPC_URL=https://secret.example.test/api-key
PROVER_DEV_OVERRIDE=true
EOF
if env LINETH_WIZARD_STATE_EXISTS=true \
  LINETH_WIZARD_SKIP_PORT_CHECK=true \
  LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
  LINETH_WIZARD_BACKUP_TIMESTAMP="guard-save-$$" \
  "$START_SH" --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-guard-save.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "save-only writes .env even with existing state and mode flip"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=' "save-only clears stale Sepolia RPC on mode flip"
  assert_file_not_contains /tmp/lineth-wizard-guard-save.$$ 'run ./scripts/reset.sh yourself' "save-only does not trigger the mode-switch guard"
else
  fail "save-only with existing state and mode flip succeeds"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=sepolia
L1_RPC_URL=https://secret.example.test/api-key
PROVER_DEV_OVERRIDE=true
EOF
if env LINETH_WIZARD_STATE_EXISTS=true \
  LINETH_WIZARD_SKIP_PORT_CHECK=true \
  LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
  LINETH_WIZARD_BACKUP_TIMESTAMP="guard-$$" \
  "$START_SH" --wizard --then-start --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-guard.$$ 2>&1; then
  fail "mode switch with existing state and start fails"
else
  assert_file_contains /tmp/lineth-wizard-guard.$$ './scripts/reset.sh' "mode-switch guard points at reset"
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "mode-switch guard leaves .env unchanged"
  [ ! -f "$BACKUP_DIR/.env.guard-$$" ] && pass "mode-switch guard skips backup" || fail "mode-switch guard skips backup"
fi

reset_env
if env LINETH_WIZARD_STATE_EXISTS=false \
  LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
  LINETH_WIZARD_PORT_CHECK_STATUS=1 \
  "$START_SH" --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-ports.$$ 2>&1; then
  [ -f "$ENV_FILE" ] && pass "save-only writes .env without checking ports" || fail "save-only writes .env without checking ports"
  assert_file_not_contains /tmp/lineth-wizard-ports.$$ 'ports are free' "save-only does not print a port check"
  assert_file_not_contains /tmp/lineth-wizard-ports.$$ 'HOST_PORT_' "save-only does not list host ports"
else
  fail "save-only with busy-port mock exits cleanly"
fi

reset_env
if env LINETH_WIZARD_STATE_EXISTS=false \
  LINETH_WIZARD_SKIP_PORT_CHECK=true \
  LINETH_WIZARD_RPC_CHECK_STATUS=1 \
  "$START_SH" --wizard --non-interactive \
    --l1-mode sepolia --l1-rpc-url 'https://rpc.example.test/key' --prover dev \
    >/tmp/lineth-wizard-rpc-fail.$$ 2>&1; then
  fail "sepolia RPC preflight failure exits non-zero"
else
  [ ! -f "$ENV_FILE" ] && pass "RPC preflight failure leaves no .env" || fail "RPC preflight failure leaves no .env"
fi

reset_env
if printf '' | env LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true "$START_SH" --wizard >/tmp/lineth-wizard-eof.$$ 2>&1; then
  fail "closed stdin exits non-zero"
else
  [ ! -f "$ENV_FILE" ] && pass "closed stdin leaves no .env" || fail "closed stdin leaves no .env"
fi

reset_env
if printf '1\n1\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true LINETH_WIZARD_SKIP_PORT_CHECK=true "$START_SH" --wizard >/tmp/lineth-wizard-numbered-local.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ 'Choose L1 mode [1]:' "L1 prompt uses numbered choice header"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ 'wizard · guided .env setup' "wizard prints the lineth banner with wizard subtitle"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '1. Local L1' "L1 prompt explains local option"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '2. Sepolia' "L1 prompt explains Sepolia option"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ 'Choose prover mode [1]:' "prover prompt uses numbered choice header"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '1. Dev proofs' "prover prompt explains dev option"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ 'Requires at least 32 GB Docker memory; 128 GB recommended.' "prover prompt explains partial prover memory needs"
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "numbered L1 choice 1 maps to local"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=true' "numbered prover choice 1 maps to dev"
else
  fail "numbered local/dev prompt run succeeds"
fi

reset_env
if printf '2\nhttps://rpc.example.test\n2\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true LINETH_WIZARD_SKIP_PORT_CHECK=true "$START_SH" --wizard >/tmp/lineth-wizard-numbered-sepolia.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "numbered L1 choice 2 maps to sepolia"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=https://rpc.example.test' "numbered sepolia flow captures RPC URL"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "numbered prover choice 2 maps to partial"
  assert_file_contains "$ENV_FILE" 'PROVER_GOMEMLIMIT=24GiB' "numbered partial prover writes memory limit"
else
  fail "numbered sepolia/partial prompt run succeeds"
fi

reset_env
if printf '\n\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true LINETH_WIZARD_SKIP_PORT_CHECK=true "$START_SH" --wizard >/tmp/lineth-wizard-numbered-defaults.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "empty L1 answer defaults to local"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=true' "empty prover answer defaults to dev"
else
  fail "numbered default prompt run succeeds"
fi

reset_env
if printf 'sepolia\nhttps://rpc.example.test\npartial\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true LINETH_WIZARD_SKIP_PORT_CHECK=true "$START_SH" --wizard >/tmp/lineth-wizard-aliases.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "text alias sepolia still works"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "text alias partial still works"
else
  fail "text alias prompt run succeeds"
fi

reset_env
if printf 'x\ny\nz\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true "$START_SH" --wizard >/tmp/lineth-wizard-numbered-invalid.$$ 2>&1; then
  fail "three invalid numbered L1 answers fail"
else
  [ ! -f "$ENV_FILE" ] && pass "three invalid numbered L1 answers leave no .env" || fail "three invalid numbered L1 answers leave no .env"
  assert_file_contains /tmp/lineth-wizard-numbered-invalid.$$ 'invalid/missing L1 mode: choose 1/local or 2/sepolia' "invalid numbered L1 answers show clear error"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=sepolia
L1_RPC_URL=https://existing.example.test/key
PROVER_DEV_OVERRIDE=true
EOF
if printf '\n\n\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true LINETH_WIZARD_SKIP_PORT_CHECK=true "$START_SH" --wizard >/tmp/lineth-wizard-keep-url.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-keep-url.$$ 'Sepolia L1 RPC URL (press Enter to keep current):' "RPC URL prompt explains Enter keeps current value"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=https://existing.example.test/key' "Enter keeps existing RPC URL"
else
  fail "keep-current RPC URL prompt run exits cleanly"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=local
L1_RPC_URL=https://old.example.test/key
PROVER_DEV_OVERRIDE=true
EOF
if printf 'sepolia\nhttps://new.example.test/key\n1\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_RPC_CHECK_STATUS=0 \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    "$START_SH" --wizard >/tmp/lineth-wizard-masked-url.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-masked-url.$$ '[1] Sepolia RPC check' "Sepolia preflight is a numbered wizard section"
  assert_file_contains /tmp/lineth-wizard-masked-url.$$ 'L1_RPC_URL' "L1_RPC_URL diff line is present"
  assert_file_not_contains /tmp/lineth-wizard-masked-url.$$ 'new.example.test' "L1_RPC_URL diff does not leak the new RPC host"
  assert_file_not_contains /tmp/lineth-wizard-masked-url.$$ 'old.example.test' "L1_RPC_URL diff does not leak the old RPC host"
  assert_file_not_contains /tmp/lineth-wizard-masked-url.$$ 'https://old.example.test/…' "L1_RPC_URL diff does not show masked host preview"
else
  fail "masked URL diff and preflight title run exits cleanly"
fi

cat > "$ENV_FILE" <<'EOF'
L1_MODE=local
L1_RPC_URL=
PROVER_DEV_OVERRIDE=true
EOF
if printf 'sepolia\nhttps://rpc.example.test\npartial\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true "$START_SH" --wizard >/tmp/lineth-wizard-confirm.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-confirm.$$ '[2] Configuration changes' "diff section uses Configuration changes label"
  assert_file_contains /tmp/lineth-wizard-confirm.$$ '[3] Next step' "existing .env shows next step as a numbered wizard section"
  assert_file_contains /tmp/lineth-wizard-confirm.$$ 'Select 1/save, 2/start, 3/clear-start, or 4/cancel [4]:' "existing .env defaults execution plan to cancel"
  assert_file_contains /tmp/lineth-wizard-confirm.$$ 'Cancel; leave .env unchanged' "execution plan includes cancel option"
  assert_file_contains /tmp/lineth-wizard-confirm.$$ 'no changes written' "default cancel leaves existing .env untouched"
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "default-No confirmation leaves .env unchanged"
  assert_file_not_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "default-No confirmation does not apply changes"
else
  fail "existing .env default-No confirmation run exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-default"
if printf '1\n1\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-default" \
    "$START_SH" --wizard >/tmp/lineth-wizard-start-default.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-start-default.$$ '[2] Next step' "fresh wizard shows next step as a numbered wizard section"
  assert_file_contains /tmp/lineth-wizard-start-default.$$ 'Select 1/save, 2/start, 3/clear-start, or 4/cancel [1]:' "wizard asks for one execution-plan choice"
  assert_file_contains /tmp/lineth-wizard-start-default.$$ '1. Save .env only' "execution plan includes save-only option"
  assert_file_contains /tmp/lineth-wizard-start-default.$$ '2. Save .env and start stack' "execution plan includes start option"
  assert_file_contains /tmp/lineth-wizard-start-default.$$ '3. Save .env, clear existing quickstart state, then start stack' "execution plan includes clean-start option"
  [ ! -f "$TMP_DIR/start-default" ] && pass "default start answer does not start stack" || fail "default start answer does not start stack"
else
  fail "default-No start prompt run exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-yes-flag"
if printf '1\n1\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-yes-flag" \
    "$START_SH" --wizard --yes >/tmp/lineth-wizard-yes-flag.$$ 2>&1; then
  assert_file_not_contains /tmp/lineth-wizard-yes-flag.$$ 'Next step' "--yes skips execution-plan prompt"
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "--yes still writes .env"
  [ ! -f "$TMP_DIR/start-yes-flag" ] && pass "--yes without --then-start does not start stack" || fail "--yes without --then-start does not start stack"
else
  fail "--yes prompt run exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-yes"
if printf '1\n1\n2\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-yes" \
    "$START_SH" --wizard >/tmp/lineth-wizard-start-yes.$$ 2>&1; then
  [ -f "$TMP_DIR/start-yes" ] && pass "yes start answer requests start handoff" || fail "yes start answer requests start handoff"
  assert_file_contains "$TMP_DIR/start-yes" '--tail' "start handoff uses --tail"
else
  fail "yes start prompt run exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-sepolia"
if printf 'sepolia\nhttps://rpc.example.test/key\n1\n2\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false \
    LINETH_WIZARD_RPC_CHECK_STATUS=0 \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-sepolia" \
    "$START_SH" --wizard >/tmp/lineth-wizard-start-sepolia.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-start-sepolia.$$ '[1] Sepolia RPC check' "sepolia start path runs the preflight section"
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "sepolia start path writes L1_MODE"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=https://rpc.example.test/key' "sepolia start path writes RPC URL"
  [ -f "$TMP_DIR/start-sepolia" ] && pass "sepolia + start handoff requests start" || fail "sepolia + start handoff requests start"
  assert_file_contains "$TMP_DIR/start-sepolia" '--tail' "sepolia start handoff uses --tail"
else
  fail "sepolia + start handoff run exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-state-no-clear" "$TMP_DIR/reset-state-no-clear"
if printf '1\n1\n2\n' \
  | env LINETH_WIZARD_STATE_EXISTS=true \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-state-no-clear" \
    LINETH_WIZARD_TEST_RESET_FILE="$TMP_DIR/reset-state-no-clear" \
    "$START_SH" --wizard >/tmp/lineth-wizard-start-state-no-clear.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-start-state-no-clear.$$ '[2] Next step' "stateful wizard still uses one execution-plan choice"
  [ -f "$TMP_DIR/start-state-no-clear" ] && pass "stateful start still starts when clear defaults to no" || fail "stateful start still starts when clear defaults to no"
  [ ! -f "$TMP_DIR/reset-state-no-clear" ] && pass "default clear answer does not reset" || fail "default clear answer does not reset"
else
  fail "stateful start prompt with default-No clear exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-state-clear" "$TMP_DIR/reset-state-clear"
if printf '1\n1\n3\n' \
  | env LINETH_WIZARD_STATE_EXISTS=true \
    LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
    LINETH_WIZARD_SKIP_PORT_CHECK=true \
    LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-state-clear" \
    LINETH_WIZARD_TEST_RESET_FILE="$TMP_DIR/reset-state-clear" \
    "$START_SH" --wizard >/tmp/lineth-wizard-start-state-clear.$$ 2>&1; then
  [ -f "$TMP_DIR/reset-state-clear" ] && pass "yes clear answer requests reset before start" || fail "yes clear answer requests reset before start"
  [ -f "$TMP_DIR/start-state-clear" ] && pass "yes clear answer still starts after reset" || fail "yes clear answer still starts after reset"
else
  fail "stateful start prompt with clear exits cleanly"
fi

reset_env
rm -f "$TMP_DIR/start-then" "$TMP_DIR/reset-then"
if env LINETH_WIZARD_STATE_EXISTS=true \
  LINETH_WIZARD_SKIP_PORT_CHECK=true \
  LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true \
  LINETH_WIZARD_TEST_START_FILE="$TMP_DIR/start-then" \
  LINETH_WIZARD_TEST_RESET_FILE="$TMP_DIR/reset-then" \
  "$START_SH" --wizard --then-start --clear-before-start --non-interactive --l1-mode local --prover dev \
    >/tmp/lineth-wizard-clear-flag.$$ 2>&1; then
  [ -f "$TMP_DIR/reset-then" ] && pass "--clear-before-start requests reset before --then-start" || fail "--clear-before-start requests reset before --then-start"
  [ -f "$TMP_DIR/start-then" ] && pass "--then-start still requests start handoff" || fail "--then-start still requests start handoff"
else
  fail "--clear-before-start with --wizard --then-start succeeds"
fi

if "$START_SH" --then-start >/tmp/lineth-wizard-then-start.$$ 2>&1; then
  fail "--then-start without wizard fails"
else
  assert_file_contains /tmp/lineth-wizard-then-start.$$ '--then-start requires --wizard' "--then-start guard message"
fi

if "$START_SH" --clear-before-start >/tmp/lineth-wizard-clear-without-wizard.$$ 2>&1; then
  fail "--clear-before-start without wizard fails"
else
  assert_file_contains /tmp/lineth-wizard-clear-without-wizard.$$ '--clear-before-start requires --wizard' "--clear-before-start without wizard guard message"
fi

if "$START_SH" --wizard --clear-before-start >/tmp/lineth-wizard-clear-without-start.$$ 2>&1; then
  fail "--clear-before-start without --then-start fails"
else
  assert_file_contains /tmp/lineth-wizard-clear-without-start.$$ '--clear-before-start requires --then-start' "--clear-before-start without --then-start guard message"
fi

if "$START_SH" --l1-mode local >/tmp/lineth-wizard-flag-without-wizard.$$ 2>&1; then
  fail "wizard-only flags without --wizard fail"
else
  assert_file_contains /tmp/lineth-wizard-flag-without-wizard.$$ '--l1-mode requires --wizard' "wizard-only flag guard message"
fi

rm -f /tmp/lineth-wizard-local-dev.$$ /tmp/lineth-wizard-local-partial.$$ /tmp/lineth-wizard-sepolia-dev.$$
rm -f /tmp/lineth-wizard-sepolia-partial.$$ /tmp/lineth-wizard-missing-rpc.$$
rm -f /tmp/lineth-wizard-precedence.$$ /tmp/lineth-wizard-env-only.$$ /tmp/lineth-wizard-backup.$$
rm -f /tmp/lineth-wizard-duplicate-keys.$$
rm -f /tmp/lineth-wizard-noop.$$ /tmp/lineth-wizard-backup-collision.$$ /tmp/lineth-wizard-dev-gomemlimit.$$
rm -f /tmp/lineth-wizard-guard.$$ /tmp/lineth-wizard-guard-save.$$ /tmp/lineth-wizard-ports.$$ /tmp/lineth-wizard-rpc-fail.$$ /tmp/lineth-wizard-eof.$$
rm -f /tmp/lineth-wizard-numbered-local.$$ /tmp/lineth-wizard-numbered-sepolia.$$
rm -f /tmp/lineth-wizard-numbered-defaults.$$ /tmp/lineth-wizard-aliases.$$
rm -f /tmp/lineth-wizard-numbered-invalid.$$
rm -f /tmp/lineth-wizard-keep-url.$$
rm -f /tmp/lineth-wizard-masked-url.$$
rm -f /tmp/lineth-wizard-confirm.$$ /tmp/lineth-wizard-start-default.$$ /tmp/lineth-wizard-yes-flag.$$
rm -f /tmp/lineth-wizard-start-yes.$$
rm -f /tmp/lineth-wizard-start-sepolia.$$
rm -f /tmp/lineth-wizard-start-state-no-clear.$$ /tmp/lineth-wizard-start-state-clear.$$ /tmp/lineth-wizard-clear-flag.$$
rm -f /tmp/lineth-wizard-then-start.$$ /tmp/lineth-wizard-clear-without-wizard.$$ /tmp/lineth-wizard-clear-without-start.$$
rm -f /tmp/lineth-wizard-flag-without-wizard.$$

if [ "$FAILURES" -ne 0 ]; then
  printf '[wizard-test] %s failure(s)\n' "$FAILURES" >&2
  exit 1
fi

printf '[wizard-test] all checks passed\n'
