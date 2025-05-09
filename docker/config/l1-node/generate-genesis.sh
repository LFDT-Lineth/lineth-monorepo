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

# Although this adds 0s, it is here for flexibility in the event of extension and syntax issues.
OS=$(uname);
prague_time=$(    
    if [ $OS = "Linux" ]; then
        date -d "+0 seconds" +%s;
    elif [ $OS = "Darwin" ]; then
        date -v +0S +%s;
    fi
)

sed -i -E 's/"timestamp": "[0-9]+"/"timestamp": "'"$prague_time"'"/' $modified_el_genesis_json_path
sed -i 's/\$GENESIS_TIME/'"$prague_time"'/g' $modified_cl_network_config_path