#!/bin/sh
set -eu

fork_timestamp=$(date +%s)
mkdir -p /initialization
sed \
  -e "s/%FORK_TIME%/$fork_timestamp/g" \
  -e 's/"gasLimit": "0x77359400"/"gasLimit": "0x1c9c380"/' \
  -e '/"osakaTime": 0,/a\
    "amsterdamTime": 0,\
    "builderDepositRequestContractAddress": "0x0000000000000000000000000000000000009999",\
    "builderExitRequestContractAddress": "0x0000000000000000000000000000000000009999",' \
  -e '/"alloc": {/a\
    "0x0000000000000000000000000000000000009999": {"balance": "0", "code": "00"},' \
  /templates/genesis-besu.json.template > /initialization/genesis-besu.json
sed 's/"Osaka"/"Amsterdam"/' \
  /templates/genesis-maru.json.template > /initialization/genesis-maru.json
printf '%s\n' "$fork_timestamp" > /initialization/fork-timestamp.txt
