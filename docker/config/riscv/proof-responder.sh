#!/bin/sh
set -eu

requests_dir=${RISCV_EXECUTION_REQUESTS_DIR:-/data/prover/riscv/execution/requests}
responses_dir=${RISCV_EXECUTION_RESPONSES_DIR:-/data/prover/riscv/execution/responses}
program_vk=${R5_L2_EXECUTION_PROGRAM_VK:?R5_L2_EXECUTION_PROGRAM_VK is required}
zero_hash=0x0000000000000000000000000000000000000000000000000000000000000000

mkdir -p "$responses_dir"

while true; do
  for request_file in "$requests_dir"/*-getZkL2ExecutionProof.json; do
    [ -f "$request_file" ] || continue

    file_name=${request_file##*/}
    start_block=${file_name%%-*}
    remainder=${file_name#*-}
    end_block=${remainder%%-*}
    response_file="$responses_dir/$file_name"

    [ -f "$response_file" ] && continue

    temporary_file="$response_file.tmp"
    cat >"$temporary_file" <<EOF
{
  "proverVersion": "riscv-local-dev",
  "startBlockNumber": $start_block,
  "proof": "0x00",
  "publicInputs": {
    "parentBlockHash": "$zero_hash",
    "endBlockHash": "$zero_hash",
    "endBlockNumber": $end_block,
    "endBlockTimestamp": 0,
    "l2L1MessagesHash": "$zero_hash",
    "parentL1L2BridgeRollingHash": "$zero_hash",
    "parentL1L2BridgeRollingHashMessageNumber": 0,
    "endL1L2BridgeRollingHash": "$zero_hash",
    "endL1L2BridgeRollingHashMessageNumber": 0,
    "dynamicChainConfigHash": "$zero_hash",
    "parentFtxRollingHash": "$zero_hash",
    "parentFtxNumber": 0,
    "endFtxRollingHash": "$zero_hash",
    "endProcessedFtxNumber": 0,
    "filteredAddressesHash": "$zero_hash",
    "txFromsHash": "$zero_hash"
  },
  "l2L1Messages": [],
  "txFroms": [],
  "filteredAddresses": [],
  "programVk": "$program_vk"
}
EOF
    mv "$temporary_file" "$response_file"
    echo "Created development proof response: $file_name"
  done

  sleep 1
done
