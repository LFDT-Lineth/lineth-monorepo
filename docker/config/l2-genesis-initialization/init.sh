#!/bin/zsh
echo "Initialization of timestamp in genesis files for Maru, Besu, and config file for coordinator"
date
cd initialization || exit
cp -T "genesis-maru.json.template" "genesis-maru.json"
cp -T "genesis-besu.json.template" "genesis-besu.json"
cp -T "/coordinator/coordinator-config-v2.toml" "coordinator-config-v2-hardforks.toml"

shanghai_timestamp=$(($(date +%s) + 100))
echo "Shanghai Timestamp: $shanghai_timestamp"
sed -i "s/%SHANGHAI_TIME%/$shanghai_timestamp/g" genesis-maru.json
sed -i "s/%SHANGHAI_TIME%/$shanghai_timestamp/g" genesis-besu.json

cancun_timestamp=$((shanghai_timestamp + 40))
echo "Cancun Timestamp: $cancun_timestamp"
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-maru.json
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-besu.json

prague_timestamp=$((cancun_timestamp + 40))
echo "Prague Timestamp: $prague_timestamp"
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-maru.json
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-besu.json

CREATE_EMPTY_BLOCKS="${CREATE_EMPTY_BLOCKS:-false}"
sed -i "s/%CREATE_EMPTY_BLOCKS%/$CREATE_EMPTY_BLOCKS/g" genesis-besu.json

shanghai_timestamp_ms=$((shanghai_timestamp * 1000))
cancun_timestamp_ms=$((cancun_timestamp * 1000))
prague_timestamp_ms=$((prague_timestamp * 1000))
sed -i'' "s/^\(target-end-blocks[ ]*=[ ]*\).*/\1[4]/" coordinator-config-v2-hardforks.toml
sed -i'' "s/^\(timestamp-based-hard-forks[ ]*=[ ]*\).*/\1[${shanghai_timestamp_ms}, ${cancun_timestamp_ms}]/" coordinator-config-v2-hardforks.toml