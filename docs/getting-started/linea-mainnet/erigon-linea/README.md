# Linea-patched Erigon image

This folder contains the build recipe for the **Linea-patched Erigon** image referenced by the
Linea getting-started compose files. Stock Erigon does not sync current Linea from scratch (see
the Erigon page in the Linea docs); this image applies the three patches needed to make it work.

## What's different vs upstream `erigontech/erigon:v3.3.0`

The Dockerfile clones upstream `erigontech/erigon` tag `v3.3.0` and applies 3 tiny patches + builds without Silkworm:

- `sentryMcDisableBlockDownload = false` (enables EL-driven bootstrap from genesis; fixes the block-0 hang in [erigontech/erigon#22081](https://github.com/erigontech/erigon/issues/22081))
- Return `nil` instead of error on empty code for **EIP-7002** (Linea has empty withdrawal contract; fixes [erigontech/erigon#18160](https://github.com/erigontech/erigon/issues/18160))
- Return `nil` instead of error on empty code for **EIP-7251** (Linea has empty consolidation contract)
- Build with `BUILD_TAGS=nosilkworm`

See `Dockerfile` for the exact patch lines.

## Build locally

From the repo root:

```bash
cd docs/getting-started/linea-mainnet/erigon-linea
make build
```

This builds `linea-erigon:v3.3.0-linea-patched` and loads it into your local Docker daemon. The
getting-started compose files reference this image name, so after building you can start the
node directly:

```bash
cd docs/getting-started/linea-mainnet
docker compose up erigon-init erigon-node maru-node
```

### Apple Silicon (arm64)

The default build target is `linux/amd64` (the platform most node operators run). On an
Apple Silicon Mac, build for arm64 so the image runs natively:

```bash
make build PLATFORM=linux/arm64
```

