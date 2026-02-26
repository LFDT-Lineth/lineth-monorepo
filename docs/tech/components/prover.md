# Prover

> Go-based ZK proof generation system using PLONK with Vortex polynomial commitments.

> **Diagrams:** [Prover Architecture](../diagrams/prover-architecture.mmd) | [Controller Workflow](../diagrams/controller-workflow.mmd) | [Proof System](../diagrams/proof-system.mmd) | [ZK Architecture](../diagrams/zk-proving-architecture.mmd)

## Overview

The Prover generates zero-knowledge proofs for:
- **Execution proofs**: Prove correct batch execution
- **Compression proofs**: Prove blob compression validity
- **Aggregation proofs**: Combine multiple proofs for efficient verification

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                              PROVER                                    │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                        cmd/controller                            │  │
│  │                                                                  │  │
│  │  File-system watcher ───▶ Job Queue ───▶ Proof Execution         │  │
│  │                                                                  │  │
│  └─────────────────────────────────┬────────────────────────────────┘  │
│                                    │                                   │
│  ┌─────────────────────────────────▼────────────────────────────────┐  │
│  │                          cmd/prover                              │  │
│  │                                                                  │  │
│  │  setup: Generate proving/verifying keys                          │  │
│  │  prove: Process proof request                                    │  │
│  │                                                                  │  │
│  └─────────────────────────────────┬────────────────────────────────┘  │
│                                    │                                   │
│  ┌─────────────────────────────────▼────────────────────────────────┐  │
│  │                          circuits/                               │  │
│  │                                                                  │  │
│  │  ┌─────────────┐   ┌─────────────┐   ┌─────────────────────────┐ │  │
│  │  │  execution  │   │   blob      │   │      aggregation        │ │  │
│  │  │  circuit    │   │ decompres.  │   │        circuit          │ │  │
│  │  │             │   │  circuit    │   │                         │ │  │
│  │  │ Batch exec  │   │             │   │ Combines execution +    │ │  │
│  │  │ correctness │   │ Compression │   │ compression proofs      │ │  │
│  │  │             │   │ validity    │   │                         │ │  │
│  │  └─────────────┘   └─────────────┘   └─────────────────────────┘ │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                         protocol/                                │  │
│  │                                                                  │  │
│  │  ┌───────────────────────┐    ┌────────────────────────────────┐ │  │
│  │  │    Wizard IOP         │    │      Vortex Commitments        │ │  │
│  │  │  (Interactive Oracle  │    │  (Polynomial commitment        │ │  │
│  │  │   Proof framework)    │    │   scheme)                      │ │  │
│  │  └───────────────────────┘    └────────────────────────────────┘ │  │
│  │                                                                  │  │
│  │  ┌───────────────────────┐    ┌────────────────────────────────┐ │  │
│  │  │    Arcane Compiler    │    │      PLONK Circuit             │ │  │
│  │  │  (PLONK-in-Wizard)    │    │      Compiler                  │ │  │
│  │  └───────────────────────┘    └────────────────────────────────┘ │  │
│  │                                                                  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
prover/
├── cmd/
│   ├── controller/         # File-system based job controller
│   │   ├── main.go         # Entry point
│   │   └── controller/     # Controller logic
│   │       ├── controller.go
│   │       ├── executor.go
│   │       └── command.go
│   └── prover/             # CLI proof generation
│       └── main.go         # setup/prove commands
│
├── circuits/               # Circuit definitions
│   ├── execution/          # Batch execution circuit
│   ├── blobdecompression/  # Blob compression circuit
│   ├── aggregation/        # Proof aggregation circuit
│   ├── pi-interconnection/ # Public input circuits
│   └── emulation/          # Circuit emulation
│
├── backend/                # Request/response handling
│   ├── execution/          # Execution proof backend
│   ├── blobdecompression/  # Blob proof backend
│   ├── aggregation/        # Aggregation proof backend
│   ├── blobsubmission/     # Blob submission backend
│   └── files/              # File I/O utilities
│
├── zkevm/                  # Core zkEVM logic
│   ├── arithmetization/    # EVM trace arithmetization
│   └── prover/             # Module provers (ECDSA, Keccak, etc.)
│
├── protocol/               # Proof system implementation
│   ├── compiler/           # Arcane compiler
│   │   └── vortex/         # Vortex-specific compilation
│   ├── wizard/             # Wizard IOP framework
│   └── ...                 # Many other protocol packages
│
└── crypto/                 # Cryptographic primitives
    ├── vortex/             # Vortex polynomial commitments
    ├── mimc/               # MiMC hash function
    └── state-management/   # Sparse Merkle Tree operations
```

## Proof Types

### 1. Execution Proof

Proves correct execution of L2 transactions within a batch.

**Inputs:**
- Trace matrices from tracer
- State roots
- Transaction data

**Outputs:**
- Execution proof
- Public inputs (state roots, block hashes)

### 2. Blob Compression Proof

Proves the compressed blob data matches the original batch data.

**Inputs:**
- Batch data
- Compressed blob
- Shnarf (rolling hash)

**Outputs:**
- Compression proof
- Blob commitment

### 3. Aggregation Proof

Combines execution and compression proofs for efficient L1 verification.

**Inputs:**
- Execution proof(s)
- Compression proof(s)

**Outputs:**
- Aggregated proof
- Public inputs for verifier contract

## Proof System Details

### PLONK + Vortex

```
┌────────────────────────────────────────────────────────────────────────┐
│                          PROOF SYSTEM                                  │
│                                                                        │
│  Arithmetization                   Polynomial IOP                      │
│  ┌─────────────┐                   ┌─────────────────────────┐         │
│  │   Trace     │                   │      Wizard IOP         │         │
│  │  Matrices   │──────────────────▶│                         │         │
│  │             │                   │  - Column definitions   │         │
│  │  From       │                   │  - Constraint system    │         │
│  │  tracer     │                   │  - Permutation args     │         │
│  └─────────────┘                   └────────────┬────────────┘         │
│                                                 │                      │
│                                                 ▼                      │
│                                    ┌─────────────────────────┐         │
│                                    │    Arcane Compiler      │         │
│                                    │                         │         │
│                                    │  PLONK-in-Wizard        │         │
│                                    │  compilation            │         │
│                                    └────────────┬────────────┘         │
│                                                 │                      │
│                                                 ▼                      │
│  Commitments                       ┌─────────────────────────┐         │
│  ┌─────────────┐                   │    Vortex Scheme        │         │
│  │  Polynomial │◀──────────────────│                         │         │
│  │ Commitments │                   │  - Reed-Solomon based   │         │
│  │             │                   │  - Efficient for large  │         │
│  │  BLS12-377  │                   │    circuits             │         │
│  │  BN254      │                   │                         │         │
│  │  BW6-761    │                   └─────────────────────────┘         │
│  └─────────────┘                                                       │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### Supported Curves

| Curve | Use Case |
|-------|----------|
| BLS12-377 | Inner proofs |
| BN254 | Ethereum verification |
| BW6-761 | Curve cycling |

## File-System Interface

The prover uses file-based communication with the coordinator:

```
/shared/
├── prover-execution/
│   ├── requests/
│   │   └── {start}-{end}-{contentHash}-getZkProof.json
│   ├── responses/
│   │   └── {start}-{end}-{contentHash}-getZkProof.json
│   └── done/
│       └── {start}-{end}-{contentHash}-getZkProof.json.success
│
├── prover-compression/
│   ├── requests/
│   │   └── {start}-{end}-{contentHash}-getZkBlobCompressionProof.json
│   └── responses/
│       └── {start}-{end}-{contentHash}-getZkBlobCompressionProof.json
│
└── prover-aggregation/
    ├── requests/
    │   └── {start}-{end}-{contentHash}-getZkAggregatedProof.json
    └── responses/
        └── {start}-{end}-{contentHash}-getZkAggregatedProof.json
```

### Request Format (Execution)

```json
{
  "zkStateMerkleProof": [...],
  "blocksData": [...],
  "tracesEngineVersion": "0.2.0",
  "stateManagerVersion": "1.2.0",
  "proverVersion": "0.2.0",
  "executionProofs": {
    "startBlockNumber": 1,
    "endBlockNumber": 10
  }
}
```

### Response Format

```json
{
  "proof": "0x...",
  "publicInput": "0x...",
  "startBlockNumber": 1,
  "endBlockNumber": 10,
  "executionProofs": {
    "proof": "0x...",
    "publicInputs": [...]
  }
}
```

## Controller Workflow

```
┌───────────────────────────────────────────────────────────────────────┐
│                      CONTROLLER WORKFLOW                              │
│                                                                       │
│  1. Watch                  2. Process                3. Complete      │
│  ┌─────────────┐           ┌─────────────┐          ┌─────────────┐   │
│  │  Scan       │           │   Move to   │          │   Write     │   │
│  │  requests/  │──────────▶│ .inprogress │─────────▶│  response   │   │
│  │  directory  │           │             │          │             │   │
│  └─────────────┘           │   Execute   │          │   Move to   │   │
│        │                   │   prover    │          │   done/     │   │
│        │                   └─────────────┘          └─────────────┘   │
│        │                         │                                    │
│        │                         ▼                                    │
│        │                   ┌─────────────┐                            │
│        │                   │   On Error  │                            │
│        │                   │   Retry or  │                            │
│        │                   │   Mark fail │                            │
│        └───────────────────┴─────────────┘                            │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘
```

## Building

```bash
cd prover

# Build prover binary
go build -o prover ./cmd/prover

# Build controller binary
go build -o controller ./cmd/controller

# Run tests
go test ./...
```

## Running

### Setup Phase (Generate Keys)

```bash
./prover setup \
  --circuit execution \
  --output-dir /data/keys
```

### Prove Phase

```bash
./prover prove \
  --circuit execution \
  --request /shared/prover-execution/requests/1-10.json \
  --output /shared/prover-execution/responses/1-10.json \
  --keys-dir /data/keys
```

### Controller Mode

```bash
./controller \
  --config /config/prover-config.toml \
  --watch-dir /shared
```

## Configuration

```toml
[prover]
# Circuit type
circuit = "execution"

# Key paths
keys-dir = "/data/keys"

# Performance tuning
num-workers = 4
memory-limit = "64GB"

[controller]
# Directory watching
request-dirs = [
  "/shared/prover-execution/requests",
  "/shared/prover-compression/requests",
  "/shared/prover-aggregation/requests"
]

# Polling interval
poll-interval = "1s"

# Retry configuration
max-retries = 3
retry-delay = "10s"
```

## Dependencies

- **gnark**: ZK-SNARK library (github.com/consensys/gnark)
- **gnark-crypto**: Cryptographic primitives
- **Tracer output**: `.lt` trace files from Java tracer
- **Coordinator**: File-based request/response interface

## Performance Considerations

- **Memory**: Execution proofs require significant RAM (16GB+)
- **Parallelism**: Multiple proof types can run concurrently
- **Storage**: Trace files can be large (GBs for complex batches)
- **Time**: Proof generation is CPU-intensive (minutes to hours)

## Metrics

Key metrics exposed:

- `prover_proof_generation_time_seconds`
- `prover_proofs_generated_total`
- `prover_memory_usage_bytes`
- `prover_circuit_constraints_total`

## Related Documentation

- [Feature: Prover](../../features/prover.md) — Business-level overview, proving stages, key interfaces, and configuration reference
