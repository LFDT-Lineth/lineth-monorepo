# AGENTS.md — tracer

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

EVM trace generation system for Linea's ZK proving pipeline. Implements the constraint system (arithmetization) that translates EVM execution into algebraic constraints. Includes Besu plugins for runtime tracing, reference test infrastructure, and Go Corset integration for constraint compilation.

## How to Run

```bash
# Build
./gradlew :tracer:arithmetization:build

# Unit tests
./gradlew :tracer:arithmetization:test

# Fast replay tests (integration)
./gradlew :tracer:arithmetization:fastReplayTests

# Nightly tests
./gradlew :tracer:arithmetization:nightlyTests
./gradlew :tracer:arithmetization:nightlyReplayTests
./gradlew :tracer:arithmetization:nightlyModexpTests

# Weekly tests
./gradlew :tracer:arithmetization:weeklyTests

# Besu node tests
./gradlew :tracer:arithmetization:besuNodeTests

# PRC call tests
./gradlew :tracer:arithmetization:prcCallTests

# Ethereum reference tests
./gradlew :tracer:reference-tests:referenceExecutionSpecBlockchainTests

# Lint and format
./gradlew spotlessCheck
./gradlew spotlessApply

# License header check
./gradlew :tracer:arithmetization:checkSpdxHeader

# Sonarqube analysis
./gradlew :tracer:sonarqube -Dtests=Unit
```

## Tracer-Specific Conventions

- **Languages:** Java (primary), with Go Corset for constraint compilation
- **Build plugins:** `net.consensys.besu-plugin-library` + `net.consensys.besu-plugin-distribution`
- **Corset integration:** `gradle/corset.gradle` compiles Go Corset into Java sources at `build/generated/gocorset/main/java`
- **Lombok:** Enabled via `lombok.config`
- **Javadoc:** Strict mode with `-Xdoclint:all,-missing` and `-Xwerror` for HTML5

### Directory Structure

```
tracer/
├── arithmetization/    Main constraint system implementation
├── plugins/            Besu plugin distributions (tracer + Shomei)
├── reference-tests/    Ethereum execution spec reference tests
├── testing/            Test utilities and helpers
├── gradle/             Custom Gradle scripts (tests, corset, lint)
├── scripts/            Build and deployment scripts
└── docs/               Documentation
```

### Test Types and Tags

| Test Type | Command | Timeout | Tag |
|-----------|---------|---------|-----|
| Unit | `test` | 15 min | — |
| Fast replay | `fastReplayTests` | 30 min | `replay` |
| Nightly | `nightlyTests` | 60 min | `nightly` |
| Nightly replay | `nightlyReplayTests` | 60 min | `nightly`, `replay` |
| Nightly modexp | `nightlyModexpTests` | 60 min | `nightly-modexp` |
| Weekly | `weeklyTests` | 60 min | `weekly` |
| PRC calls | `prcCallTests` | 60 min | `prc-calltests` |
| Besu node | `besuNodeTests` | 20 min | — |
| Reference blockchain | `referenceExecutionSpecBlockchainTests` | — | `BlockchainReferenceTest` |

### JVM Configuration

- Test memory: 256m min, 8g max (local), 32g max (CI)
- Max parallel forks: 1 (most tests), 3 (Besu node tests)
- JUnit5 parallelism configurable via `JUNIT_TESTS_PARALLELISM` env var

## Tracer-Specific Safety Rules

- Corset constraint changes affect ZK proof validity — verify with replay tests
- Reference tests validate Ethereum spec compliance — do not skip
- SPDX license headers required on all source files (`checkSpdxHeader` task)
- Plugin dependencies (tracer plugin, Shomei) are auto-downloaded and extracted during build

## Agent Rules (Overrides)

- Always run `./gradlew :tracer:arithmetization:test` for unit test changes
- For constraint changes, also run `./gradlew :tracer:arithmetization:fastReplayTests`
- Check SPDX headers: `./gradlew :tracer:arithmetization:checkSpdxHeader`
- Reference the tracer's own [README.md](README.md), [SETUP.md](SETUP.md), and [PLUGINS.md](PLUGINS.md) for detailed docs
