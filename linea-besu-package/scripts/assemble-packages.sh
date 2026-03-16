#!/bin/bash

# Enable fail-fast behavior
set -e

source ./versions.env

mkdir -p ./tmp
pushd ./tmp

echo "BESU_VERSION=$BESU_VERSION"
echo "LOCAL_BESU_ZIP_PATH=$LOCAL_BESU_ZIP_PATH"
echo "LOCAL_SEQUENCER_DIST_FOLDER: $LOCAL_SEQUENCER_DIST_FOLDER"
echo "LOCAL_TRACER_DIST_FOLDER: $LOCAL_TRACER_DIST_FOLDER"

if [ -z "$BESU_VERSION" ]; then
  echo "Please provide besu version in env BESU_VERSION"
  exit 1
fi

if [ -z "$LOCAL_BESU_ZIP_PATH" ] || [ ! -f "$LOCAL_BESU_ZIP_PATH" ]; then
  echo "Please provide an valid file path for the besu distribution tar gzip file in env LOCAL_BESU_ZIP_PATH"
  exit 1
fi

if [ -z "$LOCAL_SEQUENCER_DIST_FOLDER" ]; then
  echo "Please provide an valid path for the sequencer plugin distribution folder in env LOCAL_SEQUENCER_DIST_FOLDER"
  exit 1
fi

if [ -z "$LOCAL_TRACER_DIST_FOLDER" ]; then
  echo "Please provide an valid path for the tracer plugin distribution folder in env LOCAL_TRACER_DIST_FOLDER"
  exit 1
fi

echo "using local besu tar.gz: $LOCAL_BESU_ZIP_PATH"
cp $LOCAL_BESU_ZIP_PATH .
tar -xvf $(basename "$LOCAL_BESU_ZIP_PATH")
mv besu-$BESU_VERSION ./besu

echo "copying the versions.env to the container as versions.txt"
cp ../versions.env ./besu/versions.txt

mkdir -p ./besu/plugins
cd ./besu/plugins

echo "using JAR files under local sequencer distribution folder: $LOCAL_SEQUENCER_DIST_FOLDER"
cp -a $LOCAL_SEQUENCER_DIST_FOLDER/. .

echo "using JAR files under local tracer distribution folder: $LOCAL_TRACER_DIST_FOLDER"
cp -a $LOCAL_TRACER_DIST_FOLDER/. .

echo "getting linea_staterecovery_plugin_version: $LINEA_STATERECOVERY_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/linea-monorepo/releases/download/linea-staterecovery-v$LINEA_STATERECOVERY_PLUGIN_VERSION/linea-staterecovery-besu-plugin-v$LINEA_STATERECOVERY_PLUGIN_VERSION.jar

echo "getting shomei_plugin_version: $SHOMEI_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/besu-shomei-plugin/releases/download/v$SHOMEI_PLUGIN_VERSION/besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
unzip -j -o besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
rm besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip

popd

echo "placing the packages, config, profiles together for preparing docker image build"
cd ./linea-besu
cp -r config profiles ../tmp/besu/
