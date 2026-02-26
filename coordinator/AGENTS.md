# AGENTS.md — coordinator

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

Kotlin-based orchestration service for the Linea protocol. Manages proof submission, blob submission, finalization monitoring, message anchoring, and gas pricing. Built on Vert.x with Picocli CLI, Hoplite config, and Jackson serialization.

## How to Run

```bash
# Build
./gradlew :coordinator:app:build


# Run locally (requires L1/L2 nodes + PostgreSQL)
./gradlew :coordinator:app:run

# Build Docker image
docker buildx build --file coordinator/Dockerfile \
  --build-context libs=./coordinator/app/build/install/coordinator/lib \
  --tag consensys/linea-coordinator:local .

# Run as part of full stack
make start-env-with-tracing-v2
```

### Test

```bash
# Unit tests
./gradlew :coordinator:app:test

# Integration tests (requires PostgreSQL via Docker)
./gradlew :coordinator:app:integrationTest

# Integration tests including all dependencies
./gradlew :coordinator:app:integrationTestAllNeeded
```

### Lint and Format

```bash
./gradlew spotlessCheck    # Check Kotlin/Java formatting
./gradlew spotlessApply    # Auto-fix formatting
```

## Kotlin-Specific Conventions

- **Kotlin version:** 2.3.0
- **Formatter:** ktlint via Spotless (disabled rules: discouraged-comment-location, property-naming, function-naming, function-signature)
- **Build plugin:** `net.consensys.zkevm.kotlin-application-conventions`
- **Main class:** `net.consensys.zkevm.coordinator.app.CoordinatorAppMain`
- **Warnings as errors** unless `LINEA_DEV_ALLOW_WARNINGS` is set

### Directory Structure

```
coordinator/
├── app/          Main application entry point
├── core/         Core business logic
├── clients/      Client implementations (prover, smart-contract, web3signer, traces-generator)
├── ethereum/     Ethereum modules (gas-pricing, blob-submitter, finalization-monitor, message-anchoring)
├── persistence/  Database persistence (aggregation, batch, blob, feehistory, db-common)
└── utilities/    Shared utilities
```

### Configuration Files

- `config/coordinator/coordinator-config-v2.toml` — Main configuration
- `config/coordinator/vertx-options.json` — Vert.x runtime options
- `config/coordinator/log4j2-dev.xml` — Log4j2 logging config
- `config/common/traces-limits-vV.toml` — Trace limits
- `config/common/smart-contract-errors.toml` — Smart contract error mappings

### Key Dependencies

Vert.x, Jackson, Hoplite (config), Picocli (CLI), Web3J, jvm-libs workspace projects.
check versions in `gradle/libs.versions.toml`

## Kotlin-Specific Safety Rules

- Override `toString()` on config classes to redact secrets
- on toString() use `TypeArray.encodeHex()` over contentString
- Integration tests require `localStackPostgresDbOnly` Docker Compose profile
- Test timeout: 5 minutes per test
- Parallel test execution with max(runtime.processors, 9) forks

## Agent Rules (Overrides)

- Run `./gradlew spotlessCheck` before proposing Kotlin changes
- Integration tests are Docker-dependent — note this in test instructions
- Configuration changes to TOML files affect runtime behavior across environments
