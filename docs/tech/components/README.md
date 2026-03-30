# Component Guide

> **Diagram:** [Component Dependency Graph](../diagrams/component-dependency-graph.mmd) (Mermaid source)

This section provides detailed documentation for each component in the Linea monorepo.

## Component Index

| Component | Language | Path | Description |
|-----------|----------|------|-------------|
| [Coordinator](./coordinator.md) | Kotlin | `coordinator/` | Orchestration service |
| [Prover](./prover.md) | Go | `prover/` | ZK proof generation |
| [Contracts](./contracts.md) | Solidity | `contracts/` | Smart contracts |
| [Tracer](./tracer.md) | Java | `tracer/` | EVM trace generation |
| [Besu Plugins](./besu-plugins.md) | Kotlin/Java | `besu-plugins/` | Sequencer extensions |
| [SDK](./sdk.md) | TypeScript | `sdk/` | Developer SDK |
| [Postman](./postman.md) | TypeScript | `postman/` | Message relay |
| [Bridge UI](./bridge-ui.md) | TypeScript | `bridge-ui/` | Web interface |
| [E2E Tests](./e2e.md) | TypeScript | `e2e/` | End-to-end tests |
| [Tracer Constraints](./tracer-constraints.md) | Lisp | `tracer-constraints/` | ZK constraints |
| [Corset](./corset.md) | Rust | `corset/` | Constraint compiler (DSL → binary + Java interfaces) |

## Component Dependency Graph

```
                                    ┌─────────────────┐
                                    │   bridge-ui     │
                                    │   (Next.js)     │
                                    └────────┬────────┘
                                             │
                                             ▼
                              ┌──────────────────────────────┐
                              │            sdk               │
                              │  ┌────────┬────────┬───────┐ │
                              │  │  core  │  viem  │ ethers│ │
                              │  └────────┴────────┴───────┘ │
                              └──────────────┬───────────────┘
                                             │
                    ┌────────────────────────┼────────────────────────┐
                    │                        │                        │
                    ▼                        ▼                        ▼
        ┌───────────────────┐    ┌───────────────────┐    ┌───────────────────┐
        │    contracts      │    │     postman       │    │      e2e          │
        │    (L1 + L2)      │    │  (message relay)  │    │    (testing)      │
        └─────────┬─────────┘    └─────────┬─────────┘    └───────────────────┘
                  │                        │
                  │              ┌─────────┴─────────┐
                  │              │                   │
                  ▼              ▼                   ▼
        ┌───────────────────────────────┐  ┌───────────────────┐
        │        coordinator            │  │    ts-libs        │
        │         (Kotlin)              │  │  shared utilities │
        └───────────────┬───────────────┘  └───────────────────┘
                        │
        ┌───────────────┼───────────────┐
        │               │               │
        ▼               ▼               ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐
│   prover    │ │  jvm-libs   │ │    besu-plugins     │
│    (Go)     │ │   shared    │ │  sequencer + state  │
└──────┬──────┘ └─────────────┘ │      recovery       │
       │                        └──────────┬──────────┘
       │                                   │
       │        ┌──────────────────────────┘
       │        │
       ▼        ▼
┌─────────────────────┐    ┌─────────────────────┐
│      tracer         │    │ tracer-constraints  │
│   (Java Plugin)     │◄───│      (Lisp)         │
└─────────────────────┘    └─────────────────────┘
```

## Build Dependencies

### Gradle Projects (JVM)

```
root build.gradle
├── coordinator/
│   ├── app (main application)
│   ├── core (business logic)
│   ├── clients/ (external service clients)
│   ├── ethereum/ (L1 interactions)
│   └── persistence/ (database)
├── jvm-libs/
│   ├── generic/ (shared utilities)
│   └── linea/ (Linea-specific libs)
├── besu-plugins/
│   ├── linea-sequencer/ (sequencer plugins)
│   └── state-recovery/ (recovery plugin)
├── tracer/
│   ├── arithmetization (trace generation)
│   └── plugins (Besu integration)
└── testing-tools/
```

### pnpm Workspace (TypeScript)

```
root package.json
├── contracts/ (Hardhat + TypeScript tests)
├── sdk/
│   ├── sdk-core (shared types)
│   ├── sdk-viem (Viem-based)
│   └── sdk-ethers (Ethers-based)
├── postman/ (message relay service)
├── bridge-ui/ (Next.js frontend)
├── e2e/ (end-to-end tests)
├── ts-libs/
│   ├── linea-shared-utils
│   ├── linea-native-libs
│   └── eslint-config
├── operations/ (CLI tools)
└── native-yield-operations/
    └── automation-service/
```

## Language-Specific Notes

### Kotlin/Java (Gradle)

- Java 21 required
- Gradle 8.5+ for building
- Run tests: `./gradlew test`
- Build all: `./gradlew build`
- Code style: ktlint for Kotlin, Google Java Format for Java

### Go

- Go 1.24.6 required
- Build: `go build ./...`
- Test: `go test ./...`
- Uses gnark library for ZK circuits

### TypeScript

- Node.js >= 22.22.2 required
- pnpm >= 10.32.1 for package management
- Build: `pnpm run build`
- Test: `pnpm run test`
- Lint: `pnpm run lint`

### Solidity

- Hardhat for development
- Protocol contracts: Solidity **0.8.33** (see [`contracts/AGENTS.md`](../../../contracts/AGENTS.md))
- Besu plugin acceptance tests use Web3j wrappers generated from a separate Solidity version for fixtures (see [`besu-plugins/AGENTS.md`](../../../besu-plugins/AGENTS.md))
- OpenZeppelin upgradeable contracts
- Compile: `pnpm -F contracts run compile`

### Lisp (Corset)

- Custom DSL for ZK constraints
- Compiled via `go-corset`
- Generates Java interfaces for tracer
