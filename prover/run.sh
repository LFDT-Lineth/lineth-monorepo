make bin/prover 

export GOMEMLIMIT=650GiB
rm -rf prover-assets/6.0.0/* && make bin/prover && bin/prover setup --config config/config-sepolia-full.toml --circuits execution-limitless

DIR=~/testing-sepolia-beta-v2-15520477/prover-execution/requests

for f in $DIR/*; do
    
    echo
    echo $f
    echo

    rm -rf /tmp/witnesses
    bin/prover prove --config config/config-sepolia-full.toml \
        --in $f \
        --out /dev/null

    exit
done
