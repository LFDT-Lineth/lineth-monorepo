#!/bin/zsh
echo "Initialization of timestamp in genesis files for Maru, Besu, and Geth."
date
cd initialization || exit
cp -T "genesis-maru.json.template" "genesis-maru.json"
cp -T "genesis-besu.json.template" "genesis-besu.json"
cp -T "genesis-geth.json.template" "genesis-geth.json"

shanghai_timestamp=$(($(date +%s) + 100))
echo "Shanghai Timestamp: $shanghai_timestamp"
sed -i "s/%SHANGHAI_TIME%/$shanghai_timestamp/g" genesis-maru.json
sed -i "s/%SHANGHAI_TIME%/$shanghai_timestamp/g" genesis-besu.json
sed -i "s/%SHANGHAI_TIME%/$shanghai_timestamp/g" genesis-geth.json

cancun_timestamp=$((shanghai_timestamp + 40))
echo "Cancun Timestamp: $cancun_timestamp"
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-maru.json
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-besu.json
sed -i "s/%CANCUN_TIME%/$cancun_timestamp/g" genesis-geth.json

prague_timestamp=$((cancun_timestamp + 40))
echo "Prague Timestamp: $prague_timestamp"
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-maru.json
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-besu.json
sed -i "s/%PRAGUE_TIME%/$prague_timestamp/g" genesis-geth.json

CREATE_EMPTY_BLOCKS="${CREATE_EMPTY_BLOCKS:-false}"
sed -i "s/%CREATE_EMPTY_BLOCKS%/$CREATE_EMPTY_BLOCKS/g" genesis-besu.json
