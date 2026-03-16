#!/bin/bash
set -e

PUBLISH_TASK="${1:?publish task name required (e.g. publish or publishToMavenLocal)}"

echo "Injecting Cloudsmith publish to $BESU_DIR/build.gradle"
java ./linea-besu/scripts/InjectLines.java "$BESU_DIR"/build.gradle

echo "Building Besu with version $RESOLVED_BESU_VERSION (distTar $PUBLISH_TASK)"
(cd "$BESU_DIR" && ./gradlew -Prelease.releaseVersion="$RESOLVED_BESU_VERSION" -Pversion="$RESOLVED_BESU_VERSION" distTar "$PUBLISH_TASK")
