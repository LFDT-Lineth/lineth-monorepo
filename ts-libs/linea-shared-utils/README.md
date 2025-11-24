# Linea Shared Utils

## Overview

The linea-shared-utils package is a shared TypeScript utilities library for Linea TypeScript projects within the monorepo. This package houses shared TypeScript utilities that don't fit the SDK, which is designed for public external consumers.

The package may contain some duplication of SDK code to avoid bundling the SDK project in certain monorepo project builds (e.g., linea-native-yield-automation-service).

## Contents

- **Blockchain client adapters** - Viem-based adapters for blockchain interactions
- **Contract signing** - Web3Signer and Viem Wallet adapters for transaction signing
- **Beacon chain API client** - API client for Ethereum beacon chain interactions
- **OAuth2 authentication** - OAuth2 token management for authenticated API access
- **Prometheus metrics service** - Metrics collection with singleton pattern
- **Winston logging** - Structured logging implementation
- **Express API application framework** - Pre-configured Express server with metrics endpoint
- **Retry services** - Retry logic (including exponential backoff, and Viem transaction retries)
- **Utility functions** - Pure functions for blockchain conversions, time, math, string operations, and error handling

## Codebase Architecture

The codebase follows a **Layered Architecture with Dependency Inversion**, incorporating concepts from Hexagonal Architecture (Ports and Adapters):

- **`core/`** - Interfaces and constants. This layer has no dependencies on other internal layers.
- **`clients/`** - Infrastructure adapter implementations (blockchain, API clients) that implement interfaces from `core/`.
- **`services/`** - Service implementations for internal business logic that implement interfaces from `core/`.
- **`applications/`** - Accessory applications (e.g. Metrics HTTP endpoint using Express) that implement interfaces from `core/`.
- **`logging/`** - Logging implementations (Winston) that implement interfaces from `core/`.
- **`utils/`** - Pure standalone utility functions with no dependencies on other internal layers.

Dependencies flow inward: outer layers depend on `core/` interfaces, enabling testability and flexibility. This ensures implementations can be swapped without affecting dependent code.

## Folder Structure

```
linea-shared-utils/
├── src/
│   ├── applications/           # Accessory applications (e.g., metrics API)
│   │   └── ExpressApiApplication.ts
│   ├── clients/                # Infrastructure client adapters
│   ├── core/                   # Interfaces and constants
│   ├── logging/                # Logging implementations
│   ├── services/               # Business logic service implementations
│   ├── utils/                  # Standalone utility functions
└── scripts/                    # Testing scripts
```

## Installation and Usage

This package is part of the Linea monorepo and is typically used as a workspace dependency. It is used by other Linea services such as the `automation-service`.

Example imports:

```typescript
import { 
  ViemBlockchainClientAdapter,
  WinstonLogger,
  SingletonMetricsService,
  ExpressApiApplication,
  ExponentialBackoffRetryService
} from "@consensys/linea-shared-utils";
```

## Development

### Build

```bash
pnpm --filter @consensys/linea-shared-utils build
```

### Test

```bash
pnpm --filter @consensys/linea-shared-utils test
```

## License

This package is licensed under the [Apache 2.0](../../LICENSE-APACHE) and the [MIT](../../LICENSE-MIT) licenses.
