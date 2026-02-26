# External Dependencies

> **Diagrams:** [Sequencer Architecture](../diagrams/sequencer-architecture.mmd) | [State Recovery Flow](../diagrams/state-recovery-flow.mmd)

This document describes external components that are used by the Linea stack but are not part of this monorepo.

## Maru Engine

**Repository**: [Consensys/maru](https://github.com/Consensys/maru) (private/separate repo)

### Overview

Maru is the execution engine for the Linea sequencer. It implements the Engine API and serves as the execution layer client, working alongside Besu (the consensus layer client) in a similar pattern to Ethereum's post-merge architecture.

### Role in the Stack

```
┌────────────────────────────────────────────────────────────────────────┐
│                         SEQUENCER ARCHITECTURE                         │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    Linea Besu (Consensus Layer)                  │  │
│  │                                                                  │  │
│  │  - P2P networking                                                │  │
│  │  - Block gossip and sync                                         │  │
│  │  - Transaction pool management                                   │  │
│  │  - Linea-specific plugins (transaction selection, validation)    │  │
│  │                                                                  │  │
│  └────────────────────────────────┬─────────────────────────────────┘  │
│                                   │                                    │
│                            Engine API                                  │
│                     (newPayload, forkchoiceUpdated)                    │
│                                   │                                    │
│  ┌────────────────────────────────▼─────────────────────────────────┐  │
│  │                      Maru Engine (Execution Layer)               │  │
│  │                                                                  │  │
│  │  - Block production (payload building)                           │  │
│  │  - Transaction execution                                         │  │
│  │  - State management                                              │  │
│  │  - EVM execution                                                 │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### Key Responsibilities

| Responsibility | Description |
|----------------|-------------|
| **Payload Building** | Constructs new L2 blocks with transactions from the pool |
| **Transaction Execution** | Executes transactions and updates state |
| **State Management** | Maintains the L2 world state |
| **Engine API** | Implements the standard Engine API for consensus layer communication |

### Configuration

Maru is configured via environment variables and config files in the Docker stack:

```bash
# Docker compose service
services:
  maru:
    image: consensys/linea-maru:${MARU_TAG:-latest}
    ports:
      - "8080:8080"      # Engine API
    environment:
      - MARU_ENGINE_API_PORT=8080
      - MARU_LOG_LEVEL=INFO
```

### Local Development

When running the local stack, Maru is started automatically:

```bash
# Starts all L2 services including Maru
make start-env-with-tracing-v2

# Check Maru status
docker logs maru

# Maru is accessible at
# - Internal (Docker): http://maru:8080
# - External: http://localhost:8080 (if port-mapped)
```

### Interaction Points

1. **Sequencer (Besu)**: Communicates via Engine API for block production
2. **Coordinator**: Reads finalized blocks for conflation
3. **Traces Node**: Replays blocks using Maru-produced state

## Linea Besu

**Repository**: [Consensys/linea-besu](https://github.com/Consensys/linea-besu)

A fork of Hyperledger Besu with Linea-specific modifications for the sequencer and node infrastructure.

### Modifications from Upstream Besu

- Linea-specific consensus rules
- Integration with Maru via Engine API
- Support for Linea transaction types
- Optimizations for L2 workloads

### Usage in the Stack

| Role | Description |
|------|-------------|
| **Sequencer** | Primary block producer with Linea plugins |
| **Traces Node** | Replays blocks for trace generation |
| **L2 Node** | Standard node for RPC and sync |
| **zkBesu-Shomei** | State proof generation node |

## Shomei

**Repository**: Part of [Consensys/linea-monorepo](https://github.com/Consensys/linea-monorepo) or separate

Shomei is the state manager that provides Merkle proofs for L2 state.

### Role

- Maintains sparse Merkle tree of L2 state
- Generates inclusion/exclusion proofs
- Used for L2→L1 message claims
- Supports state recovery verification

### Endpoints

```bash
# Shomei RPC
http://localhost:8998

# Key methods
shomei_getProof        # Get Merkle proof for account/storage
shomei_getStateRoot    # Get state root at block
```

## go-corset

**Repository**: [Consensys/go-corset](https://github.com/Consensys/go-corset)

Compiler and validator for zkEVM constraint definitions.

### Role

- Compiles `.lisp` constraint files to binary format
- Generates Java trace interfaces
- Validates trace files against constraints

### Usage

```bash
# Compile constraints
go-corset compile -o zkevm.bin tracer-constraints/

# Generate Java code
go-corset generate -o Trace.java zkevm.bin

# Validate a trace file
go-corset check --bin zkevm.bin trace.lt
```

## gnark

**Repository**: [Consensys/gnark](https://github.com/Consensys/gnark)

Zero-knowledge proof library used by the prover.

### Features Used

- PLONK proving system
- BLS12-377, BN254, BW6-761 curves
- Vortex polynomial commitments

## External Services (Local Development)

### BlobScan

Blob data indexer for EIP-4844 blobs.

- **Local**: http://localhost:4001
- Used by state recovery to fetch historical blob data

### Blockscout

Block explorer for debugging.

- **L1 Explorer**: http://localhost:4001
- **L2 Explorer**: http://localhost:4000

## Version Compatibility

| Component | Minimum Version | Notes |
|-----------|-----------------|-------|
| Maru | Latest | Check `MARU_TAG` in docker-compose |
| Linea Besu | 24.x | Must match tracer version |
| go-corset | Latest | For constraint compilation |
| gnark | 0.9+ | For prover |

## Troubleshooting

### Maru Not Starting

```bash
# Check logs
docker logs maru

# Common issues:
# - Port 8080 in use
# - Engine API connection refused (check Besu config)
# - Insufficient memory
```

### Engine API Connection Issues

```bash
# Verify Besu can reach Maru
docker exec sequencer curl http://maru:8080

# Check Engine API authentication (if enabled)
# Verify JWT secret matches between Besu and Maru
```

### Version Mismatch

Ensure all components are compatible:

```bash
# Check versions
docker exec maru --version
docker exec sequencer besu --version

# Use matching tags
export MARU_TAG=v1.0.0
export BESU_PACKAGE_TAG=v24.0.0
make start-env-with-tracing-v2
```
