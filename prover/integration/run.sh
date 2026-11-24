#!/bin/bash
set -eu

echo "Running the integration tests in $1 mode"

# Ensure $0 is either dev-mode or full-mode
if [ "$1" == "dev-mode" ]; then
    CONFIG_FILE="config/config-integration-development.toml"
else
    if [ "$1" == "full-mode" ]; then
        CONFIG_FILE="config/config-integration-full.toml"
    else
        echo "Usage: run.sh <dev-mode|full-mode>"
        exit 1
    fi
fi

LOCAL_DIR=./integration/${1}/tmp

# The script is meant to be run from the prover folder and not the current
# folder.

TESTDATA_DIR=../testdata/prover-v2

PROVER_AGGREG=prover-aggregation
PROVER_COMPRESS=prover-compression
PROVER_EXEC=prover-execution
RESPONSES=responses
REQUESTS=requests

TD_AGGREG_REQ_DIR=${TESTDATA_DIR}/${PROVER_AGGREG}/${REQUESTS}
TD_AGGREG_RES_DIR=${TESTDATA_DIR}/${PROVER_AGGREG}/${RESPONSES}
TD_COMPRESS_REQ_DIR=${TESTDATA_DIR}/${PROVER_COMPRESS}/${REQUESTS}
TD_COMPRESS_RES_DIR=${TESTDATA_DIR}/${PROVER_COMPRESS}/${RESPONSES}
TD_EXEC_REQ_DIR=${TESTDATA_DIR}/${PROVER_EXEC}/${REQUESTS}
TD_EXEC_RES_DIR=${TESTDATA_DIR}/${PROVER_EXEC}/${RESPONSES}

LOC_AGGREG_REQ_DIR=${LOCAL_DIR}/${PROVER_AGGREG}/${REQUESTS}
LOC_AGGREG_RES_DIR=${LOCAL_DIR}/${PROVER_AGGREG}/${RESPONSES}
LOC_COMPRESS_REQ_DIR=${LOCAL_DIR}/${PROVER_COMPRESS}/${REQUESTS}
LOC_COMPRESS_RES_DIR=${LOCAL_DIR}/${PROVER_COMPRESS}/${RESPONSES}
LOC_EXEC_REQ_DIR=${LOCAL_DIR}/${PROVER_EXEC}/${REQUESTS}
LOC_EXEC_RES_DIR=${LOCAL_DIR}/${PROVER_EXEC}/${RESPONSES}

# Compiles the testcase generator
echo "copying the request files from ${TESTDATA_DIR} into ${LOCAL_DIR}"

# Tear down the data folder to ensure a fresh restart every time. Also cleans
# the prover samples.
# Attempt to remove the directory without sudo
if rm -rf ${LOCAL_DIR}/* 2>/dev/null; then
    echo "Directory removed successfully without sudo"
else
    # If the removal fails due to lack of permissions, try with sudo
    if sudo rm -rf ${LOCAL_DIR}/*; then
        echo "Directory removed successfully with sudo"
    fi
fi

mkdir -p ${LOC_AGGREG_REQ_DIR} ${LOC_COMPRESS_REQ_DIR} ${LOC_EXEC_REQ_DIR}
mkdir -p ${LOC_AGGREG_RES_DIR} ${LOC_COMPRESS_RES_DIR} ${LOC_EXEC_RES_DIR}
cp -rf ${TD_AGGREG_REQ_DIR}/* ${LOC_AGGREG_REQ_DIR}/
cp -rf ${TD_COMPRESS_REQ_DIR}/* ${LOC_COMPRESS_REQ_DIR}/
cp -rf ${TD_EXEC_REQ_DIR}/* ${LOC_EXEC_REQ_DIR}/

mkdir -p ${LOCAL_DIR}/traces/conflated/
cp -rf ${TESTDATA_DIR}/conflated/* ${LOCAL_DIR}/traces/conflated/

# Running setup
if [ "$1" == "dev-mode" ]; then
    echo "no need to run setup -- everything is dummy"
else
    echo "running the setup for needed circuits"
    # TODO @gbotrel: we use dummy circuit here while waiting for execution to be functional end-to-end
    make bin/prover
    bin/prover setup --dict ./lib/compressor/compressor_dict.bin --force --config ${CONFIG_FILE} --circuits execution,aggregation,emulation --assets-dir ./prover-assets
fi

# Refresh, Build and run the docker in the background
echo "starting the environment"

docker compose -f compose-integration.yml kill -s SIGINT &>/dev/null
docker compose -f compose-integration.yml down &>/dev/null
docker compose -f compose-integration.yml build
docker compose -f compose-integration.yml run --rm -v ${LOCAL_DIR}:/shared/ -e CONFIG_FILE="/opt/linea/prover/${CONFIG_FILE}" prover &>${LOCAL_DIR}/prover.log &

echo "waiting for the prover to process the requests..."

# The commands takes a long time to run, so we can expect the container
# deployment to be ready once the above command exits. Now, we shall wait a few
# minutes to leave time for the prover to be finished.
./integration/wait-empty-dir.sh ${LOC_AGGREG_REQ_DIR}

echo "Done waiting for the prover to finish, killing the container and cleaning the resources..."

# At this point the controller in the prover service is still running and we
# need to kill it.
docker compose -f compose-integration.yml kill -s SIGINT
sleep 2
docker compose -f compose-integration.yml down

echo "Updating the testdata, with the generated prover's responses"

if [ "$1" == "dev-mode" ]; then
    # TODO @gbotrel @AlexandreBelling when / what do we copy exactly for test data?
    cp -rf ${LOC_AGGREG_RES_DIR}/* ${TD_AGGREG_RES_DIR}
    cp -rf ${LOC_COMPRESS_RES_DIR}/* ${TD_COMPRESS_RES_DIR}/
    cp -rf ${LOC_EXEC_RES_DIR}/* ${TD_EXEC_RES_DIR}/
fi
