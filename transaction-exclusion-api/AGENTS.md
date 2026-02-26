# AGENTS.md — transaction-exclusion-api

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

Kotlin-based JSON-RPC API for tracking and querying rejected transactions in the Linea network. Provides `linea_saveRejectedTransactionV1` and `linea_getTransactionExclusionStatusV1` endpoints. Built on Vert.x with PostgreSQL persistence.

## How to Run

```bash
# Build
./gradlew :transaction-exclusion-api:app:build

# Build distribution
./gradlew :transaction-exclusion-api:app:installDist

# Run locally (requires PostgreSQL)
./gradlew :transaction-exclusion-api:app:run

# Unit tests
./gradlew :transaction-exclusion-api:app:test
./gradlew :transaction-exclusion-api:core:test

# Integration tests (requires PostgreSQL via Docker)
./gradlew :transaction-exclusion-api:app:integrationTest

# Integration tests including all dependencies
./gradlew :transaction-exclusion-api:app:integrationTestAllNeeded

# Lint and format
./gradlew spotlessCheck
./gradlew spotlessApply
```

## API-Specific Conventions

- **Build plugin:** `net.consensys.zkevm.kotlin-application-conventions`
- **Main class:** `net.consensys.linea.transactionexclusion.app.TransactionExclusionAppMain`
- **API protocol:** JSON-RPC 2.0
- **Test HTTP assertions:** REST-assured + JSON Unit

### Directory Structure

```
transaction-exclusion-api/
├── app/          Main application entry point
├── core/         Core business logic
└── persistence/  Database persistence (rejectedtransaction)
```

### Configuration Files

- `config/transaction-exclusion-api/transaction-exclusion-app-docker.config.toml` — Docker config
- `config/transaction-exclusion-api/transaction-exclusion-app-local-dev.config.overrides.toml` — Local overrides
- `config/transaction-exclusion-api/vertx.json` — Vert.x options
- `config/transaction-exclusion-api/log4j2-dev.xml` — Logging

## API-Specific Safety Rules

- API method names follow versioned convention (V1 suffix) per versioning rules
- Integration tests require `localStackPostgresDbOnly` Docker Compose profile
- Database schema changes require migration testing with Docker PostgreSQL

## Agent Rules (Overrides)

- Run integration tests for any persistence layer changes
- API endpoint changes must maintain backward compatibility (versioned methods)
- See [README.md](README.md) for API method documentation
