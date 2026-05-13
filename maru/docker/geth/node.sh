#!/usr/bin/env bash
set -Eeu

if [ $# -lt 1 ]; then
    echo "Usage: $0 genesisfile"
    echo Example: "$0" /genesis.json
    exit
fi

genesisfile=$1

function prepareDatadir {
  datadir=$1
  genesisfile=$2
  if [ ! -d "$datadir"/geth ]; then
    echo -e "\n\n----------> A new data directory '$datadir' will be created!"
    geth --datadir "$datadir" init "$genesisfile"
    echo -e "----------> A new data directory '$datadir' created!\n\n"
  else
    echo -e "\n\n----------> Data directory '$datadir' already exists! Contents:"
    find "$datadir"
    echo -e "\n\n"
  fi
}

echo genesisfile="$genesisfile"

prepareDatadir "/data" "$genesisfile"

echo "Starting geth"
exec geth --config=/scripts/config.toml \
 --authrpc.jwtsecret /jwt \
 --bootnodes "$BOOTNODES" \
 --log.vmodule eth/*=5
