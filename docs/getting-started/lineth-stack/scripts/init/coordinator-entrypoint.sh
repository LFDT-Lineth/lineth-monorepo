#!/bin/sh
set -eu

raw_da_mode="${LINEA_COORDINATOR_DATA_AVAILABILITY:-}"
override_da_mode="${config__override__l1_submission__data_availability:-}"

if [ -n "$raw_da_mode" ] && [ "$raw_da_mode" != "ROLLUP" ]; then
  echo "[coordinator] FATAL: data availability mode '$raw_da_mode' is not supported by this quickstart; use ROLLUP" >&2
  exit 1
fi

if [ -n "$override_da_mode" ] && [ "$override_da_mode" != "ROLLUP" ]; then
  echo "[coordinator] FATAL: data availability mode '$override_da_mode' is not supported by this quickstart; use ROLLUP" >&2
  exit 1
fi

exec java \
  -Dvertx.configurationFile=/var/lib/coordinator/vertx-options.json \
  -Dlog4j2.configurationFile=/var/lib/coordinator/log4j2-dev.xml \
  -jar libs/coordinator.jar \
  --traces-limits-v5 /opt/consensys/linea/coordinator/config/traces-limits-v5.toml \
  --smart-contract-errors /opt/consensys/linea/coordinator/config/smart-contract-errors.toml \
  --gas-price-cap-time-of-day-multipliers /opt/consensys/linea/coordinator/config/gas-price-cap-time-of-day-multipliers.toml \
  /rendered/coordinator-config.toml
