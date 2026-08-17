#!/usr/bin/env bash
set -euo pipefail

bash /generate-genesis.sh
/usr/local/bin/eth-genesis-state-generator devnet \
  --config /data/l1-node-config/network-config.yml ${L1_GENESIS_TIME:+--timestamp ${L1_GENESIS_TIME:-} } \
  --mnemonics /config/mnemonics.yaml \
  --state-output /data/l1-node-config/genesis.ssz \
  --eth1-config /data/l1-node-config/genesis.json
