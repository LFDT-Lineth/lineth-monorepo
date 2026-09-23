#!/bin/sh
set -eu

cd "${L2_GENESIS_DIRECTORY:-/initialization}"
mode=${L2_GENESIS_MODE:-zkevm}
case "$mode" in
  zkevm|riscv) ;;
  *) echo "Unknown L2_GENESIS_MODE: $mode" >&2; exit 1 ;;
esac

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
    genesis-besu.json.template > genesis-besu.json
  sed 's/"Osaka"/"Amsterdam"/' genesis-maru.json.template > genesis-maru.json
else
  sed "s/%FORK_TIME%/$fork_timestamp/g" genesis-besu.json.template > genesis-besu.json
  sed "s/%FORK_TIME%/$fork_timestamp/g" genesis-maru.json.template > genesis-maru.json
fi
printf '%s\n' "$fork_timestamp" > fork-timestamp.txt
