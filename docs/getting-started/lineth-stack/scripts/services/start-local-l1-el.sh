#!/usr/bin/env bash
set -euo pipefail

rm -fr /opt/besu/plugins
exec /opt/besu/bin/besu-untuned --config-file=/config/config.toml
