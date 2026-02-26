# Linea Monorepo Documentation

> Comprehensive technical documentation for the Linea zkEVM rollup stack.

> **See also:** [All Mermaid Diagrams](./diagrams/README.md) — Index of all architecture and flow diagrams

## Quick Navigation

| Section | Description |
|---------|-------------|
| [Architecture Overview](./architecture/OVERVIEW.md) | High-level system architecture and data flows |
| [External Dependencies](./architecture/EXTERNAL-DEPENDENCIES.md) | Maru, Linea Besu, and other external components |
| [Component Guide](./components/README.md) | Detailed documentation for each component |
| [Development Guide](./development/README.md) | Local setup, building, and testing |
| [Operations Guide](./operations/README.md) | Deployment, monitoring, and operational tools |

## Repository Structure

```
linea-monorepo/
├── Kotlin/Java (Gradle)
│   ├── coordinator/          # Orchestration service
│   ├── jvm-libs/             # Shared JVM libraries
│   ├── besu-plugins/         # Besu plugin extensions
│   ├── tracer/               # EVM trace generation
│   ├── transaction-exclusion-api/
│   └── testing-tools/
│
├── Go
│   └── prover/               # ZK proof generation
│
├── Solidity
│   ├── contracts/            # Core protocol contracts
│   └── contracts-tge/        # Token generation event contracts
│
├── TypeScript
│   ├── sdk/                  # Developer SDK (viem/ethers)
│   ├── bridge-ui/            # Bridge frontend (Next.js)
│   ├── postman/              # Message relay service
│   ├── e2e/                  # End-to-end tests
│   ├── ts-libs/              # Shared TS libraries
│   ├── operations/           # CLI operational tools
│   └── native-yield-operations/
│
├── Rust
│   └── corset/               # Constraint compiler
│
├── Lisp/zkasm
│   └── tracer-constraints/   # ZK constraint definitions
│
└── Infrastructure
    ├── docker/               # Docker compose configurations
    └── config/               # Service configurations
```

## Tech Stack Summary

| Layer | Technology | Purpose |
|-------|------------|---------|
| L1 Contracts | Solidity | Rollup, messaging, bridging, verification |
| L2 Contracts | Solidity | Message service, token bridge |
| Sequencer | Besu + Plugins | Block production, transaction processing |
| Execution Engine | Maru *(external)* | L2 block production via Engine API |
| Tracer | Java | EVM execution trace generation |
| Constraints | Lisp/Corset | ZK circuit constraint definitions |
| Prover | Go/gnark | ZK proof generation (PLONK + Vortex) |
| Coordinator | Kotlin | Orchestration, L1 submission |
| SDK | TypeScript | Developer integration library |
| Bridge UI | Next.js/React | User-facing bridge interface |
| Postman | TypeScript | Automated message claiming |

> **Note**: Components marked *(external)* are not part of this repository. See [External Dependencies](./architecture/EXTERNAL-DEPENDENCIES.md) for details.

## Key Dependencies

> **Diagram:** [System Dependencies](./diagrams/system-dependencies.mmd) (Mermaid source)

```
┌────────────────────────────────────────────────────────────────────────┐
│                        SYSTEM DEPENDENCIES                             │
│                                                                        │
│  EXTERNAL                LAYER 2                 PROVING PIPELINE      │
│  ┌─────────────┐        ┌───────────────┐       ┌─────────────┐        │
│  │ Users/dApps │        │   Sequencer   │       │   Tracer    │        │
│  │             │        │ (Besu+Plugins)│       │(Java Plugin)│        │
│  └──────┬──────┘        └───────┬───────┘       └──────┬──────┘        │
│         │                       │                      │               │
│         ▼                       ▼                      ▼               │
│  ┌─────────────┐        ┌─────────────┐         ┌─────────────┐        │
│  │  Bridge UI  │        │    Maru     │         │   Prover    │        │
│  │  (Next.js)  │        │   Engine    │         │  (Go/gnark) │        │
│  └──────┬──────┘        └─────────────┘         └──────┬──────┘        │
│         │                                              │               │
│         ▼                                              ▼               │
│  ┌─────────────┐                                ┌─────────────┐        │
│  │    SDK      │────────────────────────────────│ Coordinator │        │
│  │ (TypeScript)│                                │  (Kotlin)   │        │
│  └──────┬──────┘                                └──────┬──────┘        │
│         │                                              │               │
│         ▼                                              ▼               │
│  ┌──────────────────────────────────────────────────────────────┐      │
│  │                       Ethereum L1                            │      │
│  └──────────────────────────────────────────────────────────────┘      │
│                                                                        │
│  CROSS-CHAIN: Postman (TypeScript) ←→ L1 ←→ L2                         │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Prerequisites: Node.js v22+, Docker v24+, pnpm v10+, Make, JDK 21

# 1. Install dependencies
make pnpm-install

# 2. Start the full stack
make start-env-with-tracing-v2

# 3. Run end-to-end tests
cd e2e && pnpm run test:e2e:local

# 4. Stop and clean
make clean-environment
```

## Documentation Conventions

- **Must** = Mandatory requirement (blocking)
- **Should** = Recommended practice
- Code examples use syntax highlighting
- Diagrams use Mermaid or ASCII art
- Cross-references link to relevant sections
