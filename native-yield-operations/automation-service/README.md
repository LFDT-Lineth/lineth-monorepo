# Linea Native Yield Automation Service

## Overview

The Linea Native Yield Automation Service automates native yield operations by continuously monitoring the YieldManager contract's ossification state and executing mode-specific processors. 

The service is triggered through an event-driven mechanism that watches for `VaultsReportDataUpdated` events from the LazyOracle contract. The `waitForVaultsReportDataUpdatedEvent()` function implements a race condition between event detection and a configurable maximum wait duration — whichever occurs first triggers the operation cycle, ensuring timely execution even when events are absent.

Operation modes are selected dynamically based on the yield provider's state:
- `YIELD_REPORTING_MODE` for normal operations
- `OSSIFICATION_PENDING_MODE` during transition
- `OSSIFICATION_COMPLETE_MODE` once ossified.

## Codebase Architecture

The codebase follows a **Layered Architecture with Dependency Inversion**, incorporating concepts from Hexagonal Architecture (Ports and Adapters) and Domain-Driven Design:

- **`core/`** - Domain layer containing interfaces (ports), entities, enums, and ABIs. This layer has no dependencies on other internal layers.
- **`services/`** - Application layer containing business logic that orchestrates operations using interfaces from `core/`.
- **`clients/`** - Infrastructure layer containing adapter implementations of interfaces defined in `core/`.
- **`application/`** - Composition layer that wires dependencies and bootstraps the service.

Dependencies flow inward: `application` → `services/clients` → `core`. This ensures business logic remains independent of infrastructure concerns, making the codebase testable and maintainable.

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
└── scripts/                  # Testing and utility scripts
```

## Configuration

See the [configuration schema file](./src/application/main/config/config.schema.ts)

## Development

### Running

#### Start the docker local stack

TODO - Planning to write E2E tests with mock Lido contracts once this branch + Native Yield contracts are together

#### Run the automation service locally:

1. Create `.env` as per `.env.sample` and the [configuration schema](./src/application/main/config/config.schema.ts)

2. `pnpm --filter @consensys/linea-native-yield-automation-service exec tsx run.ts`

### Build

```bash
# Dependency on pnpm --filter @consensys/linea-shared-utils build
pnpm --filter @consensys/linea-native-yield-automation-service build
```

### Unit Test

```bash
pnpm --filter @consensys/linea-shared-utils test
```

## License

This package is licensed under the [Apache 2.0](../../LICENSE-APACHE) and the [MIT](../../LICENSE-MIT) licenses.
