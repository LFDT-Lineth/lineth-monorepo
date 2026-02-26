# Coordinator

> Kotlin service that orchestrates the Linea rollup proving and submission pipeline.

> **Diagram:** [Coordinator Architecture](../diagrams/coordinator-architecture.mmd) (Mermaid source)

## Overview

The Coordinator is the central orchestration service responsible for:
- Conflating L2 blocks into batches
- Requesting ZK proofs from the prover
- Submitting blob data to L1 (EIP-4844)
- Finalizing batches with aggregated proofs
- Managing gas pricing for L1 submissions
- Anchoring L1→L2 messages

## Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                            COORDINATOR                                 │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                        ConflationApp                             │  │
│  │                                                                  │  │
│  │  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐   │  │
│  │  │   Block     │───▶│ Conflation  │───▶│  ZK Proof Creation  │   │  │
│  │  │  Monitor    │    │   Service   │    │    Coordinator      │   │  │
│  │  └─────────────┘    └─────────────┘    └─────────────────────┘   │  │
│  │                                                 │                │  │
│  │                                                 ▼                │  │
│  │  ┌────────────────────────────────────────────────────────────┐  │  │
│  │  │                BlobCompressionProofCoordinator             │  │  │
│  │  └────────────────────────────────────────────────────────────┘  │  │
│  │                                  │                               │  │
│  │                                  ▼                               │  │
│  │  ┌────────────────────────────────────────────────────────────┐  │  │
│  │  │             ProofAggregationCoordinatorService             │  │  │
│  │  └────────────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       L1DependentApp                             │  │
│  │                                                                  │  │
│  │  ┌─────────────────────┐    ┌─────────────────────────────────┐  │  │
│  │  │   BlobSubmission    │    │     Aggregation Finalization    │  │  │
│  │  │    Coordinator      │    │         Coordinator             │  │  │
│  │  └─────────────────────┘    └─────────────────────────────────┘  │  │
│  │                                                                  │  │
│  │  ┌─────────────────────┐    ┌─────────────────────────────────┐  │  │
│  │  │   Finalization      │    │      Message Anchoring          │  │  │
│  │  │     Monitor         │    │         Service                 │  │  │
│  │  └─────────────────────┘    └─────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
coordinator/
├── app/                    # Application layer
│   └── src/main/kotlin/
│       └── net/consensys/zkevm/coordinator/app/
│           ├── CoordinatorApp.kt           # Main orchestrator
│           ├── CoordinatorAppMain.kt       # Entry point
│           ├── CoordinatorAppCli.kt        # CLI interface
│           ├── L1DependentApp.kt           # L1 submission services
│           ├── conflation/
│           │   └── ConflationApp.kt        # Proof pipeline
│           └── config/                     # Configuration parsing
│
├── core/                   # Business logic
│   └── src/main/kotlin/
│       └── net/consensys/zkevm/
│           ├── ethereum/coordination/
│           │   ├── conflation/             # Block conflation
│           │   ├── blob/                   # Blob compression
│           │   ├── aggregation/            # Proof aggregation
│           │   ├── proofcreation/          # Proof requests
│           │   └── blockcreation/          # Block monitoring
│           └── persistence/                # Persistence interfaces
│
├── clients/                # External service clients
│   ├── prover-client/      # File-based prover communication
│   ├── smart-contract-client/  # L1 contract interactions
│   ├── shomei-client/      # State manager client
│   ├── traces-generator-api-client/
│   └── web3signer-client/  # Transaction signing
│
├── ethereum/               # Ethereum integration
│   ├── blob-submitter/     # EIP-4844 blob submission
│   ├── finalization-monitor/
│   ├── gas-pricing/        # Dynamic gas pricing
│   ├── message-anchoring/
│   └── common/
│
└── persistence/            # Data storage
    ├── batch/              # Batch persistence
    ├── blob/               # Blob persistence
    ├── aggregation/        # Aggregation persistence
    └── db-common/          # Flyway migrations
```

## Data Flow

### 1. Block Conflation

```
L2 Blocks ───▶ BlockCreationMonitor ───▶ ConflationService
                     │                          │
                     │                          ▼
              Monitor new blocks         Apply conflation triggers:
              from L2 node               - Time limit (6s default)
                                         - Trace line count limit
                                         - Data size limit
                                         - Max blocks limit
                                                │
                                                ▼
                                         Create Batch
```

### 2. Proof Generation

```
Batch ───▶ ZkProofCreationCoordinator ───▶ Execution Proof Request
                                                    │
                                                    ▼
                                           Write to prover/requests/
                                                    │
                                                    ▼
                                           Prover processes (Go)
                                                    │
                                                    ▼
                                           Read from prover/responses/
                                                    │
                                                    ▼
         BlobCompressionProofCoordinator ◀──────────┘
                     │
                     ▼
         Compression Proof Request ───▶ Prover ───▶ Blob Compression Proof
                     │
                     ▼
         ProofAggregationCoordinator ◀─────────────────────────────────────┘
                     │
                     ▼
         Aggregation Proof Request ───▶ Prover ───▶ Aggregated Proof
```

### 3. L1 Submission

```
Blob + Compression Proof ───▶ BlobSubmissionCoordinator
                                        │
                                        ▼
                              Submit EIP-4844 blob to L1
                              (LineaRollup.submitBlobs)
                                        │
                                        ▼
Aggregated Proof ───▶ AggregationFinalizationCoordinator
                                        │
                                        ▼
                              Submit finalization to L1
                              (LineaRollup.finalizeBlocks)
                                        │
                                        ▼
                              FinalizationMonitor tracks
                              L1 finalization status
```

## Configuration

Main configuration file: `coordinator-config-v2.toml`

### Key Configuration Sections

```toml
[default]
l1-endpoint = "http://l1-el-node:8545"
l2-endpoint = "http://sequencer:8545"

[conflation]
conflation-deadline = "PT6S"        # 6 seconds
blocks-limit = 200
traces-limits = "traces-limits-v2.toml"

[prover]
request-directory = "/shared/prover-execution/requests"
response-directory = "/shared/prover-execution/responses"
polling-interval = "PT1S"

[l1-submission.blob]
gas-limit = 5000000
max-fee-per-gas-cap = "100gwei"

[l1-submission.aggregation]
gas-limit = 10000000
target-end-block-confirmation-delay = "PT12S"

[gas-pricing]
type = "dynamic"  # or "static"
time-of-day-multipliers = "gas-price-cap-time-of-day-multipliers.toml"

[database]
host = "postgres"
port = 5432
name = "coordinator"
```

## Prover Communication

The coordinator uses **file-based communication** with the prover:

```
/shared/
├── prover-execution/
│   ├── requests/           # Coordinator writes here
│   │   └── {start}-{end}-{contentHash}-getZkProof.json
│   └── responses/          # Prover writes here
│       └── {start}-{end}-{contentHash}-getZkProof.json
├── prover-compression/
│   ├── requests/
│   └── responses/
└── prover-aggregation/
    ├── requests/
    └── responses/
```

### Request Flow

1. Coordinator writes JSON request to `requests/`
2. Prover controller watches directory
3. Prover moves request to `.inprogress`
4. Prover generates proof
5. Prover writes response to `responses/`
6. Prover moves request to `done/`
7. Coordinator polls and reads response

## Database Schema

```sql
-- Batches
CREATE TABLE batches (
    start_block_number BIGINT,
    end_block_number BIGINT,
    status VARCHAR(50),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

-- Blobs
CREATE TABLE blobs (
    start_block_number BIGINT,
    end_block_number BIGINT,
    blob_hash VARCHAR(66),
    compression_proof BYTEA,
    status VARCHAR(50)
);

-- Aggregations
CREATE TABLE aggregations (
    start_block_number BIGINT,
    end_block_number BIGINT,
    aggregation_proof BYTEA,
    status VARCHAR(50)
);
```

## Building

```bash
# Build JAR
./gradlew :coordinator:app:build

# Build Docker image
./gradlew coordinator:app:installDist
docker buildx build \
  --file coordinator/Dockerfile \
  --build-context libs=./coordinator/app/build/install/coordinator/lib \
  --tag consensys/linea-coordinator:local .
```

## Running

```bash
# With full stack
COORDINATOR_TAG=local make start-env-with-tracing-v2

# Standalone (requires dependencies)
java -jar coordinator/app/build/libs/coordinator.jar \
  --traces-limits-v2 config/common/traces-limits-v2.toml \
  --smart-contract-errors config/common/smart-contract-errors.toml \
  config/coordinator/coordinator-config-v2.toml
```

## API Endpoints

- `GET /` - Health check
- `GET /config` - Current configuration
- `GET /metrics` - Prometheus metrics

## Metrics

Key metrics exposed:

- `coordinator_blocks_conflated_total`
- `coordinator_batches_created_total`
- `coordinator_proofs_generated_total`
- `coordinator_blobs_submitted_total`
- `coordinator_finalizations_submitted_total`
- `coordinator_gas_price_gwei`
- `coordinator_l1_submission_latency_seconds`

## Dependencies

- **L1 Node**: Submit blobs and finalizations
- **L2 Node (Sequencer)**: Monitor new blocks
- **Traces Node**: Fetch execution traces
- **Prover**: Generate ZK proofs
- **Shomei**: State verification
- **PostgreSQL**: Persistence
- **Web3Signer** (optional): External signing

## Related Documentation

- [Feature: Coordinator](../../features/coordinator.md) — Business-level overview, conflation pipeline, flow diagrams, and interface reference
