#!/bin/bash
set -e

error_handler() {
    echo "Error occurred in script at line: $1"
    exit 1
}

trap 'error_handler $LINENO' ERR

npx ts-node ./src/scripts/build.ts
cp -R src/compressor/lib/ dist/lib
rm -rf ./dist/scripts

echo "Build script executed successfully."