# AGENTS.md — besu-plugins

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

Besu blockchain client plugins for Linea: the sequencer plugin (transaction ordering, profitability, tracing integration), finalized-tag-updater, and state-recovery modules. Built as Gradle distributions that extend Hyperledger Besu.

## How to Run

```bash
# Build sequencer plugin
./gradlew :besu-plugins:linea-sequencer:build

# Build distribution ZIP
./gradlew :besu-plugins:linea-sequencer:distZip

# Unit tests
./gradlew :besu-plugins:linea-sequencer:test

# Acceptance tests (large runner, requires Docker)
./gradlew :besu-plugins:linea-sequencer:acceptance-tests:acceptanceTests

# Lint and format
./gradlew spotlessCheck
./gradlew spotlessApply

# License check
./gradlew :besu-plugins:linea-sequencer:checkLicense
```

## Plugin-Specific Conventions

- **Build plugin:** `net.consensys.besu-plugin-library` + `net.consensys.besu-plugin-distribution`
- **Target Besu version:** Defined in `gradle/libs.versions.toml` (Besu catalog)
- **Distribution format:** ZIP (no JAR artifact)
- **License management:** `org.gradle.license-report` with allowlist at `gradle/allowed-licenses.json`

### Directory Structure

```
besu-plugins/
├── linea-sequencer/          Main sequencer plugin
│   ├── sequencer/            Core sequencer implementation
│   ├── acceptance-tests/     Acceptance tests (Web3j, REST-assured, Wiremock)
│   ├── docs/                 Plugin documentation
│   └── README.md
├── finalized-tag-updater/    Finalized tag management
└── state-recovery/           State recovery
    ├── appcore/              Application core
    ├── besu-plugin/          Besu plugin integration
    ├── clients/              Client implementations
    └── test-cases/           Test infrastructure
```

### Acceptance Tests

- Use Web3j contract wrappers generated from Solidity 0.8.19
- Max parallel forks: CI = runtime.processors, local = 3
- Parallel execution disabled by default
- Includes REST-assured for HTTP testing and Wiremock for mocking

## Plugin-Specific Safety Rules

- Plugin changes affect the Besu node runtime — test with acceptance tests before merging
- License compliance: all dependencies must match `gradle/allowed-licenses.json`
- Sequencer plugin has its own release workflow (`.github/workflows/linea-sequencer-plugin-release.yml`)

## Agent Rules (Overrides)

- Always run acceptance tests for sequencer changes: `./gradlew :besu-plugins:linea-sequencer:acceptance-tests:acceptanceTests`
- Check license compliance: `./gradlew :besu-plugins:linea-sequencer:checkLicense`
- Plugin distribution changes may require corresponding linea-besu-package updates
