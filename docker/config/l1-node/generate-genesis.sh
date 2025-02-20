# Script to modify config files to enable Prague/Electra to run for the local stack
# Runs on the 'l1-node-genesis-generator' Docker image entrypoint

original_el_genesis_json_path="/config/genesis.json"
original_cl_network_config_path="/config/network-config.yml"
output_dir="/data/l1-node-config"
modified_el_genesis_json_path=$output_dir/$(basename -- $original_el_genesis_json_path)
modified_cl_network_config_path=$output_dir/$(basename -- $original_cl_network_config_path)

mkdir -p $output_dir
cp $original_el_genesis_json_path $modified_el_genesis_json_path
cp $original_cl_network_config_path $modified_cl_network_config_path

# Early exit if $PRAGUE feature flag is off
if [ -z "$PRAGUE" ]; then
  exit 0
fi

OS=$(uname);
prague_time=$(    
    if [ $OS = "Linux" ]; then
        date -d "+32 seconds" +%s;
    elif [ $OS = "Darwin" ]; then
        date -v +32S +%s;
    fi
)

# Add Prague config to modified Besu config
jq --argjson prague_time "$prague_time" '.config.pragueTime = $prague_time' $original_el_genesis_json_path > $modified_el_genesis_json_path

# Add Electra config to modified Teku config
cat <<EOF >> $modified_cl_network_config_path
ELECTRA_FORK_VERSION: 0x60000038
ELECTRA_FORK_EPOCH: 1
MAX_BLOBS_PER_BLOCK_ELECTRA: 9
MIN_PER_EPOCH_CHURN_LIMIT_ELECTRA: 128000000000
BLOB_SIDECAR_SUBNET_COUNT_ELECTRA: 9
MAX_REQUEST_BLOB_SIDECARS_ELECTRA: 1152
EOF