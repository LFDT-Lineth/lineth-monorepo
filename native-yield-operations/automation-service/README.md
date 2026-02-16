# Linea Native Yield Automation Service

## Overview

Automates native yield operations on Linea by monitoring the YieldManager contract and executing the appropriate operation mode (yield reporting, ossification pending, or ossification complete). See [docs/architecture.md](./docs/architecture.md) for design details.

## Folder Structure

```
automation-service/
├── src/
│   ├── application/          # Application bootstrap and configuration
│   │   ├── main/             # Service entry point and config loading
│   │   └── metrics/          # Metrics service and updaters
│   ├── clients/              # External service clients
│   │   ├── contracts/        # Smart contract clients (LazyOracle, YieldManager, VaultHub, etc.)
│   ├── core/                 # Interfaces
│   │   ├── abis/             # Contract ABIs
│   │   ├── clients/          # Client interfaces
│   │   ├── entities/         # Domain entities and data models
│   │   ├── enums/            # Enums
│   │   ├── metrics/          # Metrics interfaces and types
│   │   └── services/         # Service interfaces
│   ├── services/             # Business logic services
│   │   ├── operation-mode-processors/  # Mode-specific processors
│   │   └── OperationModeSelector.ts    # Mode selection orchestrator
│   └── utils/                # Utility functions
└── scripts/                  # Manual integration runner scripts
```

## Configuration

See the [configuration schema file](./src/application/main/config/config.schema.ts) for all available options and their defaults.

## Development

### Running locally

1. Create `.env` as per `.env.sample` and the [configuration schema](./src/application/main/config/config.schema.ts)

2. `pnpm --filter @consensys/linea-native-yield-automation-service exec tsx run.ts`

### Build

```bash
# Dependency on pnpm --filter @consensys/linea-shared-utils build
pnpm --filter @consensys/linea-native-yield-automation-service build
```

### Unit tests

```bash
pnpm --filter @consensys/linea-native-yield-automation-service test
```

### Manual integration scripts

The `scripts/` directory contains manual runners for exercising individual clients against a live environment. Each script requires environment variables - see the file header for usage.

| Script | Client under test |
|--------|-------------------|
| `test-consensys-staking-graphql-client.ts` | `ConsensysStakingApiClient` (validator data via GraphQL) |
| `test-lazy-oracle-contract-client.ts` | `LazyOracleContractClient` (event watching, oracle reads) |
| `test-lido-accounting-report-client.ts` | `LidoAccountingReportClient` (report fetching/submission) |
| `test-yield-manager-contract-client.ts` | `YieldManagerContractClient` (rebalance, withdraw, state queries) |

Example:

```bash
RPC_URL=https://0xrpc.io/hoodi \
PRIVATE_KEY=0xabc123... \
YIELD_MANAGER_ADDRESS=0x... \
pnpm --filter @consensys/linea-native-yield-automation-service exec tsx scripts/test-yield-manager-contract-client.ts
```

## License

This package is licensed under the [Apache 2.0](../../LICENSE-APACHE) and the [MIT](../../LICENSE-MIT) licenses.
