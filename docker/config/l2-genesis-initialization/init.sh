#!/bin/zsh
echo "Initialization of timestamp in genesis files for Maru, Besu, and config file for coordinator"
date
cd initialization || exit
cp -T "genesis-maru.json.template" "genesis-maru.json"
cp -T "genesis-besu.json.template" "genesis-besu.json"
cp -T "/coordinator/coordinator-config-v2.toml" "coordinator-config-v2-hardforks.toml"

cancun_timestamp=0
echo "Cancun Timestamp: $cancun_timestamp"
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-maru.json
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-besu.json

prague_timestamp=$(($(date +%s) + 100))
echo "Prague Timestamp: $prague_timestamp"
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-maru.json
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-besu.json

prague_timestamp_ms=$((prague_timestamp * 1000))
sed -i'' "s/^\(timestamp-based-hard-forks[ ]*=[ ]*\).*/\1[${prague_timestamp_ms}]/" coordinator-config-v2-hardforks.toml
