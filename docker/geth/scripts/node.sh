#!/usr/bin/env bash
set -Eeu

if [ $# -lt 4 ]; then
    echo "Usage: $0 networkid genesisfile gasprice gaslimit [etherbase]"
    echo "Example: $0 12345 /genesis.json 0xa 0x21312 0x6d976c9b8ceee705d4fe8699b44e5eb58242f484"
    exit
fi

source ./scripts/functions.sh
networkid=$1
genesisfile=$2
gasprice=$3
gaslimit=$4
pricelimit=$5
etherbase=${6-""}

: ${BOOTNODES:=""} # default to empty
: ${ETHSTATS_URL:=""} #default to empty

echo DATA_DIR=$DATA_DIR
echo networkid=$networkid
echo genesisfile=$genesisfile
echo gasprice=$gasprice
echo gaslimit=$gaslimit
echo pricelimit=$pricelimit
echo BOOTNODES=$BOOTNODES
echo ETHSTATS_URL=$ETHSTATS_URL
echo NETRESTRICT=$NETRESTRICT
echo etherbase=$etherbase

mkdir -p /data/traces/raw
prepareDatadir $DATA_DIR $genesisfile

#cat /jwt-secret.hex

if [ ${#BOOTNODES} -ge 1 ]; then
#    bootnode=$(getent hosts $bootnodehost | awk '{print $1}')
    echo "Starting geth connecting to $BOOTNODES"
    exec geth --datadir $DATA_DIR \
      --networkid $networkid \
      --miner.gasprice $gasprice \
      --miner.gaslimit $gaslimit \
      --bootnodes $BOOTNODES \
      --http --http.addr '0.0.0.0' --http.port 8545 --http.corsdomain '*' --http.api 'admin,eth,miner,net,web3,personal,txpool,debug' --http.vhosts="*" \
      --ws --ws.addr '0.0.0.0' --ws.port 8546 --ws.origins '*' --ws.api 'admin,eth,miner,net,web3,personal,txpool,debug' \
      --ethstats "$ETHSTATS_URL" \
      --netrestrict "$NETRESTRICT" \
      --ipcdisable \
      --verbosity 3 \
      --txpool.nolocals \
      --txpool.pricelimit $pricelimit \
      --syncmode "full" \
      --gcmode "archive"
else
    echo "Starting geth VALIDATOR with BOOTNODE enabled on port '$BOOTNODE_PORT' with '/boot.key'"
    mkdir -p $DATA_DIR/keystore
    cp /keystore/$etherbase.json $DATA_DIR/keystore
    exec geth --datadir $DATA_DIR \
     --networkid $networkid \
     --miner.etherbase $etherbase \
     --miner.gasprice $gasprice \
     --miner.gaslimit $gaslimit \
     --unlock $etherbase \
     --allow-insecure-unlock \
     --password "/dev/null" \
     --http --http.addr '0.0.0.0' --http.port 8545 --http.corsdomain '*' --http.api 'admin,eth,miner,net,web3,personal,txpool,debug' --http.vhosts="*" \
     --ws --ws.addr '0.0.0.0' --ws.port 8546 --ws.origins '*' --ws.api 'admin,eth,miner,net,web3,personal,txpool,debug' \
     --nodekey "/boot.key" \
     --port $BOOTNODE_PORT \
     --ethstats "$ETHSTATS_URL" \
     --ipcdisable \
     --netrestrict "$NETRESTRICT" \
     --verbosity 3 \
     --mine \
     --txpool.nolocals \
     --txpool.pricelimit $pricelimit \
     --syncmode "full"
fi
