#!/bin/sh
set -eu

# Both execution modes use the same genesis location. Initialization must never
# replace an existing chain when Compose recreates this one-shot container.
cd "${L2_GENESIS_DIRECTORY:-/initialization}"
mode=${L2_GENESIS_MODE:-zkevm}
case "$mode" in
  zkevm|riscv) ;;
  *) echo "Unknown L2_GENESIS_MODE: $mode" >&2; exit 1 ;;
esac

if [ -e genesis-besu.json ] || [ -e genesis-maru.json ] || [ -e fork-timestamp.txt ]; then
  for file in genesis-besu.json genesis-maru.json fork-timestamp.txt; do
    if [ ! -s "$file" ]; then
      echo "Incomplete L2 genesis: $file is missing or empty; refusing to overwrite existing state" >&2
      exit 1
    fi
  done
  if { [ "$mode" = riscv ] && ! grep -Eq '"amsterdamTime"[[:space:]]*:[[:space:]]*0[[:space:]]*[,}]' genesis-besu.json; } ||
     { [ "$mode" = zkevm ] && grep -Eq '"amsterdamTime"[[:space:]]*:[[:space:]]*0[[:space:]]*[,}]' genesis-besu.json; }; then
    echo "Existing L2 genesis does not match the requested smoke mode ($mode)." >&2
    echo "A zkEVM/RISC-V transition requires fork and storage migration; do not reset a chain under test." >&2
    exit 1
  fi
  echo "Reusing existing L2 genesis"
  exit 0
fi

# Generate off to the side so a failed template read cannot publish a truncated
# genesis. An interrupted publish is detected as incomplete on the next run.
staging=$(mktemp -d .genesis.XXXXXX)
trap 'rm -rf "$staging"' EXIT HUP INT TERM

fork_timestamp=$(($(date +%s) + 60))
if [ "$mode" = riscv ]; then
  fork_timestamp=$(date +%s)
  sed \
    -e "s/%FORK_TIME%/$fork_timestamp/g" \
    -e 's/"gasLimit": "0x77359400"/"gasLimit": "0x1c9c380"/' \
    -e '/"osakaTime": 0,/a\
    "amsterdamTime": 0,\
    "builderDepositRequestContractAddress": "0x0000000000000000000000000000000000009999",\
    "builderExitRequestContractAddress": "0x0000000000000000000000000000000000009999",' \
    -e '/"alloc": {/a\
    "0x0000000000000000000000000000000000009999": {"balance": "0", "code": "00"},' \
    genesis-besu.json.template > "$staging/genesis-besu.json"
  sed 's/"Osaka"/"Amsterdam"/' genesis-maru.json.template > "$staging/genesis-maru.json"
else
  sed "s/%FORK_TIME%/$fork_timestamp/g" genesis-besu.json.template > "$staging/genesis-besu.json"
  sed "s/%FORK_TIME%/$fork_timestamp/g" genesis-maru.json.template > "$staging/genesis-maru.json"
fi
printf '%s\n' "$fork_timestamp" > "$staging/fork-timestamp.txt"
for file in genesis-besu.json genesis-maru.json fork-timestamp.txt; do
  test -s "$staging/$file"
done
for file in genesis-besu.json genesis-maru.json fork-timestamp.txt; do
  mv "$staging/$file" "$file"
done
