#!/bin/bash

# Enable fail-fast behavior
set -e

source ./versions.env

mkdir -p ./tmp
pushd ./tmp

echo "downloading besu from linea-besu-upstream: $LINEA_BESU_TAR_GZ"
wget -nv $LINEA_BESU_BASE_URL$LINEA_BESU_TAR_GZ/$LINEA_BESU_FILENAME_PREFIX-$LINEA_BESU_TAR_GZ.tar.gz
tar -xvf $LINEA_BESU_FILENAME_PREFIX-$LINEA_BESU_TAR_GZ.tar.gz
mv $LINEA_BESU_FILENAME_PREFIX-$LINEA_BESU_TAR_GZ ./besu

echo "copying the versions.env to the container as versions.txt"
cp ../versions.env ./besu/versions.txt

mkdir -p ./besu/plugins
cd ./besu/plugins

echo "downloading the plugins"
echo "getting linea_sequencer_plugin_version: $LINEA_SEQUENCER_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/linea-monorepo/releases/download/linea-sequencer-v$LINEA_SEQUENCER_PLUGIN_VERSION/linea-sequencer-v$LINEA_SEQUENCER_PLUGIN_VERSION.zip
unzip -j -o linea-sequencer-v$LINEA_SEQUENCER_PLUGIN_VERSION.zip
rm linea-sequencer-v$LINEA_SEQUENCER_PLUGIN_VERSION.zip

echo "getting linea_staterecovery_plugin_version: $LINEA_STATERECOVERY_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/linea-monorepo/releases/download/linea-staterecovery-v$LINEA_STATERECOVERY_PLUGIN_VERSION/linea-staterecovery-besu-plugin-v$LINEA_STATERECOVERY_PLUGIN_VERSION.jar

echo "getting linea_tracer_plugin_version: $LINEA_TRACER_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/linea-monorepo/releases/download/linea-tracer-$LINEA_TRACER_PLUGIN_VERSION/linea-tracer-$LINEA_TRACER_PLUGIN_VERSION.zip
unzip -j -o linea-tracer-$LINEA_TRACER_PLUGIN_VERSION.zip
rm linea-tracer-$LINEA_TRACER_PLUGIN_VERSION.zip

echo "getting shomei_plugin_version: $SHOMEI_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/besu-shomei-plugin/releases/download/v$SHOMEI_PLUGIN_VERSION/besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
unzip -j -o besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
rm besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip

popd

echo "placing the packages, config, profiles together for preparing docker image build"
cd ./linea-besu
cp -r config profiles ../tmp/besu/
