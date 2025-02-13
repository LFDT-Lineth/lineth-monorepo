#!/bin/bash
set -euo pipefail

genesis_time=""
current_time_delay_in_sec=""
l1_genesis=""
network_config=""
mnemonics=""
output_dir=""

usage() {
  echo "Usage: $0 --genesis-time <timestamp> --current-time-delay-in-sec <seconds to delay current timestamp if genesis-time is not given> --l1-genesis <path to l1 genesis file> --network-config <path to network config file> --mnemonics <path to mnemonics file> --output-dir <output directory>"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --genesis-time)
      genesis_time="$2"
      shift 2
      ;;
    --current-time-delay-in-sec)
      current_time_delay_in_sec="$2"
      shift 2
      ;;
    --l1-genesis)
      l1_genesis="$2"
      shift 2
      ;;
    --network-config)
      network_config="$2"
      shift 2
      ;;
    --mnemonics)
      mnemonics="$2"
      shift 2
      ;;
    --output-dir)
      output_dir="$2"
      shift 2
      ;;
    *)
      echo "Error: Unknown option: $1"
      usage
      ;;
  esac
done

if [ -z "$l1_genesis" ] || [ -z "$network_config" ] || [ -z "$mnemonics" ] || [ -z "$output_dir" ]; then
  echo "Error: Missing required argument."
  usage
fi

OS=$(uname);

if [ -z "$genesis_time" ]; then
  if [ -z "$current_time_delay_in_sec" ]; then
    current_time_delay_in_sec="3"
  fi 
  genesis_time=$(    
    if [ $OS = "Linux" ]; then
      date -d "+$current_time_delay_in_sec seconds" +%s;
    elif [ $OS = "Darwin" ]; then
      date -v +"$current_time_delay_in_sec"S +%s;
    fi
  )
fi

echo "Genesis time set to: $genesis_time"

mkdir -p $output_dir
cp $l1_genesis $output_dir/genesis.json
cp $network_config $output_dir/$(basename -- $network_config)

# sed in-place command portable with both OS 
if [ $OS = "Linux" ]; then
  sed -i -E 's/"timestamp": "[0-9]+"/"timestamp": "'"$genesis_time"'"/' $output_dir/genesis.json
  sed -i 's/\$GENESIS_TIME/'"$genesis_time"'/g' $output_dir/$(basename -- $network_config)
elif [ $OS = "Darwin" ]; then
  sed -i "" -E 's/"timestamp": "[0-9]+"/"timestamp": "'"$genesis_time"'"/' $output_dir/genesis.json
  sed -i "" 's/\$GENESIS_TIME/'"$genesis_time"'/g' $output_dir/$(basename -- $network_config)
fi

/usr/local/bin/eth2-testnet-genesis deneb --config $output_dir/$(basename -- $network_config) --mnemonics $mnemonics --tranches-dir $output_dir/tranches --state-output $output_dir/genesis.ssz --eth1-config $output_dir/genesis.json
