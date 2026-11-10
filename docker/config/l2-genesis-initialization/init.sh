#!/bin/zsh
echo "Initialization of timestamp in genesis files for Maru, Besu, and config file for coordinator"
date
cd initialization || exit
cp -T "genesis-maru.json.template" "genesis-maru.json"
cp -T "genesis-besu.json.template" "genesis-besu.json"

fork_timestamp=$(($(date +%s) + 60))

# RISC-V proves Amsterdam blocks from genesis.
if [ "$L2_GENESIS_MODE" = "riscv" ]; then
  fork_timestamp=$(date +%s)
  sed -i 's/"Osaka"/"Amsterdam"/' genesis-maru.json
  sed -i \
    -e '/"osakaTime": 0,/a\
    "amsterdamTime": 0,\
    "builderDepositRequestContractAddress": "0x0000000000000000000000000000000000009999",\
    "builderExitRequestContractAddress": "0x0000000000000000000000000000000000009999",' \
    -e '/"alloc": {/a\
    "0x0000000000000000000000000000000000009999": {"balance": "0", "code": "00"},' \
    genesis-besu.json
fi

echo "Fork Timestamp: $fork_timestamp"
sed -i "s/%FORK_TIME%/$fork_timestamp/g" genesis-maru.json
sed -i "s/%FORK_TIME%/$fork_timestamp/g" genesis-besu.json

echo $fork_timestamp > /initialization/fork-timestamp.txt
# Right now only Osaka is supported by the tracer, so no need to override forks
#cp -T "/coordinator/coordinator-config-v2.toml" "coordinator-config-v2-hardforks.toml"
#sed -i'' "s/^\(timestamp-based-hard-forks[ ]*=[ ]*\).*/\1[${fork_timestamp}]/" coordinator-config-v2-hardforks.toml
