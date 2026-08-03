#!/bin/bash
set -e

PUBLISH_TASK="${1:?publish task name required (e.g. publish, publishToMavenLocal, or none)}"

if [ "$PUBLISH_TASK" = "publishToMavenLocal" ] || [ "$PUBLISH_TASK" = "none" ]; then
  echo "Skip injecting Cloudsmith publish to $BESU_DIR/build.gradle"
else
  echo "Injecting Cloudsmith publish to $BESU_DIR/build.gradle"
  java ./linea-besu/besu/scripts/InjectLines.java "$BESU_DIR"/build.gradle
fi

echo "Building Besu with version $RESOLVED_BESU_VERSION (distTar $PUBLISH_TASK)"
if [ "$PUBLISH_TASK" = "none" ]; then
  (cd "$BESU_DIR" && ./gradlew clean && ./gradlew -Prelease.releaseVersion="$RESOLVED_BESU_VERSION" -Pversion="$RESOLVED_BESU_VERSION" distTar)
else
  (cd "$BESU_DIR" && ./gradlew clean && ./gradlew -Prelease.releaseVersion="$RESOLVED_BESU_VERSION" -Pversion="$RESOLVED_BESU_VERSION" distTar "$PUBLISH_TASK")
fi
