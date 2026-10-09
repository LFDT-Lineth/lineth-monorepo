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
  echo "Please provide a valid file path for the besu distribution tar gzip file in env LOCAL_BESU_ZIP_PATH"
  exit 1
fi

if [ -z "$LOCAL_SEQUENCER_DIST_FOLDER" ]; then
  echo "Please provide a valid path for the sequencer plugin distribution folder in env LOCAL_SEQUENCER_DIST_FOLDER"
  exit 1
fi

if [ -z "$LOCAL_TRACER_DIST_FOLDER" ]; then
  echo "Please provide a valid path for the tracer plugin distribution folder in env LOCAL_TRACER_DIST_FOLDER"
  exit 1
fi

echo "using local besu tar.gz: $LOCAL_BESU_ZIP_PATH"
cp $LOCAL_BESU_ZIP_PATH .
tar -xvf $(basename "$LOCAL_BESU_ZIP_PATH")
mv besu-$BESU_VERSION ./besu

# tar preserves the archive's mtimes, so every extracted file lands with the same
# timestamp (often epoch 0 for Gradle distTar output). BuildKit's local-context
# content dedup keys blobs on (path, mtime, size); a script whose content changed
# but whose size is identical to the previous build (e.g. bin/besu-untuned) is then
# NOT re-transferred, and the stale blob is baked into the image even with
# --no-cache. Touch every extracted file so mtimes differ across builds and the
# context differ picks up real content changes.
# Without this line, subsequent image build with besuCommit updated might lead
# to runtime java.lang.ClassNotFoundException
find ./besu -exec touch {} +

echo "copying the versions.env to the container as versions.txt"
cp ../versions.env ./besu/versions.txt

mkdir -p ./besu/plugins
cd ./besu/plugins

echo "using JAR files under local sequencer distribution folder: $LOCAL_SEQUENCER_DIST_FOLDER"
cp -a $LOCAL_SEQUENCER_DIST_FOLDER/. .

echo "using JAR files under local tracer distribution folder: $LOCAL_TRACER_DIST_FOLDER"
cp -a $LOCAL_TRACER_DIST_FOLDER/. .

## Temporarily disabled the fetch of staterecovery plugin
## as the plugin needed to release with besu >= 26.8.1 with Vertx v5
# echo "getting linea_staterecovery_plugin_version: $LINEA_STATERECOVERY_PLUGIN_VERSION"
# wget -nv https://github.com/LFDT-Lineth/lineth-monorepo/releases/download/linea-staterecovery-v$LINEA_STATERECOVERY_PLUGIN_VERSION/linea-staterecovery-besu-plugin-v$LINEA_STATERECOVERY_PLUGIN_VERSION.jar

echo "getting shomei_plugin_version: $SHOMEI_PLUGIN_VERSION"
wget -nv https://github.com/Consensys/besu-shomei-plugin/releases/download/v$SHOMEI_PLUGIN_VERSION/besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
unzip -j -o besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip
rm besu-shomei-plugin-v$SHOMEI_PLUGIN_VERSION.zip

popd

echo "placing the packages, config, profiles together for preparing docker image build"
cd ./linea-besu
cp -r config profiles ../tmp/besu/
