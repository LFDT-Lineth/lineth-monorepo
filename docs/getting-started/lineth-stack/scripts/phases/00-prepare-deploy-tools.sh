#!/usr/bin/env sh
# Seed pinned Foundry binaries from ghcr.io/foundry-rs/foundry into a Docker
# volume consumed by the node-based deploy-contracts container.
set -eu

: "${FOUNDRY_TAG:?FOUNDRY_TAG must be set}"

mkdir -p /foundry/bin
for bin in forge cast anvil chisel; do
  src="$(command -v "$bin")"
  cp "$src" "/foundry/bin/$bin"
  chmod 0755 "/foundry/bin/$bin"
done

printf '[foundry-tools] seeded Foundry %s binaries into /foundry/bin\n' "$FOUNDRY_TAG"
forge --version
