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
    "$BACKUP_DIR"/.env.guard-"$$"* "$BACKUP_DIR"/.env.devmem-"$$"*
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

assert_golden() {
  golden_name="$1"
  snapshot="$TMP_DIR/$golden_name.actual"
  write_managed_snapshot "$ENV_FILE" "$snapshot"
  if cmp -s "$snapshot" "$SCRIPT_DIR/golden/$golden_name.env"; then
    pass "$golden_name managed-key output matches golden file"
  else
    fail "$golden_name managed-key output matches golden file"
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
  assert_golden local-dev
else
  fail "local dev non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode local --prover partial >/tmp/lineth-wizard-local-partial.$$ 2>&1; then
  assert_golden local-partial
else
  fail "local partial non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode sepolia --l1-rpc-url 'https://rpc.example.test/key?a=1&b=2' --prover dev >/tmp/lineth-wizard-sepolia-dev.$$ 2>&1; then
  assert_golden sepolia-dev
else
  fail "sepolia dev non-interactive wizard succeeds"
fi

reset_env
if run_wizard --wizard --non-interactive --l1-mode sepolia --l1-rpc-url 'https://rpc.example.test/key?a=1&b=2' --prover partial >/tmp/lineth-wizard-sepolia-partial.$$ 2>&1; then
  assert_file_contains "$ENV_FILE" 'L1_MODE=sepolia' "sepolia partial writes L1_MODE"
  assert_file_contains "$ENV_FILE" 'L1_RPC_URL=https://rpc.example.test/key?a=1&b=2' "sepolia partial writes URL literally"
  assert_file_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "sepolia partial writes partial prover mode"
  assert_file_contains "$ENV_FILE" 'PROVER_GOMEMLIMIT=24GiB' "sepolia partial pins PROVER_GOMEMLIMIT"
  assert_golden sepolia-partial
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

rm -f "$BACKUP_DIR/.env.noop-$$"
if LINETH_WIZARD_BACKUP_TIMESTAMP="noop-$$" run_wizard --wizard --non-interactive --l1-mode local --prover partial >/tmp/lineth-wizard-noop.$$ 2>&1; then
  [ ! -f "$BACKUP_DIR/.env.noop-$$" ] && pass "no-op rerun skips backup" || fail "no-op rerun skips backup"
  assert_file_contains /tmp/lineth-wizard-noop.$$ 'no changes' "no-op rerun reports no changes"
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
  LINETH_WIZARD_BACKUP_TIMESTAMP="guard-$$" \
  "$START_SH" --wizard --non-interactive --l1-mode local --prover dev >/tmp/lineth-wizard-guard.$$ 2>&1; then
  fail "mode switch with existing state fails"
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
  fail "busy-port simulation exits non-zero"
else
  [ -f "$ENV_FILE" ] && pass "busy-port simulation writes .env before failing" || fail "busy-port simulation writes .env before failing"
  assert_file_contains /tmp/lineth-wizard-ports.$$ 'HOST_PORT_*' "busy-port simulation explains HOST_PORT edits"
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
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '1. Local L1' "L1 prompt explains local option"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '2. Sepolia' "L1 prompt explains Sepolia option"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ 'Choose prover mode [1]:' "prover prompt uses numbered choice header"
  assert_file_contains /tmp/lineth-wizard-numbered-local.$$ '1. Dev proofs' "prover prompt explains dev option"
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
L1_MODE=local
L1_RPC_URL=
PROVER_DEV_OVERRIDE=true
EOF
if printf 'sepolia\nhttps://rpc.example.test\npartial\n\n' \
  | env LINETH_WIZARD_STATE_EXISTS=false LINETH_WIZARD_SKIP_RPC_PREFLIGHT=true "$START_SH" --wizard >/tmp/lineth-wizard-confirm.$$ 2>&1; then
  assert_file_contains /tmp/lineth-wizard-confirm.$$ 'no changes written' "existing .env default confirmation is No"
  assert_file_contains "$ENV_FILE" 'L1_MODE=local' "default-No confirmation leaves .env unchanged"
  assert_file_not_contains "$ENV_FILE" 'PROVER_DEV_OVERRIDE=false' "default-No confirmation does not apply changes"
else
  fail "existing .env default-No confirmation run exits cleanly"
fi

if "$START_SH" --then-start >/tmp/lineth-wizard-then-start.$$ 2>&1; then
  fail "--then-start without wizard fails"
else
  assert_file_contains /tmp/lineth-wizard-then-start.$$ '--then-start requires --wizard' "--then-start guard message"
fi

if "$START_SH" --l1-mode local >/tmp/lineth-wizard-flag-without-wizard.$$ 2>&1; then
  fail "wizard-only flags without --wizard fail"
else
  assert_file_contains /tmp/lineth-wizard-flag-without-wizard.$$ '--l1-mode requires --wizard' "wizard-only flag guard message"
fi

rm -f /tmp/lineth-wizard-local-dev.$$ /tmp/lineth-wizard-local-partial.$$ /tmp/lineth-wizard-sepolia-dev.$$
rm -f /tmp/lineth-wizard-sepolia-partial.$$ /tmp/lineth-wizard-missing-rpc.$$
rm -f /tmp/lineth-wizard-precedence.$$ /tmp/lineth-wizard-env-only.$$ /tmp/lineth-wizard-backup.$$
rm -f /tmp/lineth-wizard-noop.$$ /tmp/lineth-wizard-backup-collision.$$ /tmp/lineth-wizard-dev-gomemlimit.$$
rm -f /tmp/lineth-wizard-guard.$$ /tmp/lineth-wizard-ports.$$ /tmp/lineth-wizard-rpc-fail.$$ /tmp/lineth-wizard-eof.$$
rm -f /tmp/lineth-wizard-numbered-local.$$ /tmp/lineth-wizard-numbered-sepolia.$$
rm -f /tmp/lineth-wizard-numbered-defaults.$$ /tmp/lineth-wizard-aliases.$$
rm -f /tmp/lineth-wizard-numbered-invalid.$$
rm -f /tmp/lineth-wizard-confirm.$$ /tmp/lineth-wizard-then-start.$$ /tmp/lineth-wizard-flag-without-wizard.$$

if [ "$FAILURES" -ne 0 ]; then
  printf '[wizard-test] %s failure(s)\n' "$FAILURES" >&2
  exit 1
fi

printf '[wizard-test] all checks passed\n'
