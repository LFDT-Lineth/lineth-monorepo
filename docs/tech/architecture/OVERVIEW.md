# Architecture Overview

> **Diagrams:** [System Architecture](../diagrams/system-architecture.mmd) | [Component Interactions](../diagrams/component-interactions.mmd) | [Proving Pipeline](../diagrams/proving-pipeline.mmd) | [Data Availability](../diagrams/data-availability-modes.mmd)

## System Architecture

Linea is a zkEVM Layer 2 rollup that inherits Ethereum's security through zero-knowledge proofs.

```
┌────────────────────────────────────────────────────────────────────────┐
│                              ETHEREUM L1                               │
│  ┌──────────────────┐  ┌─────────────────┐  ┌───────────────────────┐  │
│  │   LineaRollup    │  │  PlonkVerifier  │  │   TokenBridge (L1)    │  │
│  │  - Submit blobs  │  │  - Verify ZK    │  │  - Bridge ERC20       │  │
│  │  - Finalize      │  │    proofs       │  │  - L1 → L2 deposits   │  │
│  │  - Anchor msgs   │  │                 │  │  - L2 → L1 withdraws  │  │
│  └────────┬─────────┘  └────────┬────────┘  └──────────┬────────────┘  │
│           │                     │                      │               │
└───────────┼─────────────────────┼──────────────────────┼───────────────┘
            │                     │                      │
            │ Submit/Finalize     │ Verify               │ Bridge
            │                     │                      │
┌───────────┼─────────────────────┼──────────────────────┼───────────────┐
│           │                     │                      │               │
│  ┌────────▼─────────┐  ┌────────┴────────┐  ┌─────────▼─────────────┐  │
│  │   Coordinator    │  │     Prover      │  │   L2MessageService    │  │
│  │   (Kotlin)       │◄─┤     (Go)        │  │  - Send L2→L1 msgs    │  │
│  │                  │  │                 │  │  - Claim L1→L2 msgs   │  │
│  │  - Conflation    │  │  - Execution    │  │                       │  │
│  │  - Blob submit   │  │  - Compression  │  │  ┌─────────────────┐  │  │
│  │  - Finalization  │  │  - Aggregation  │  │  │ TokenBridge(L2) │  │  │
│  │  - Gas pricing   │  │                 │  │  │ - L1 mirror     │  │  │
│  └────────┬─────────┘  └────────▲────────┘  │  └─────────────────┘  │  │
│           │                     │           └───────────┬───────────┘  │
│           │                     │                       │              │
│  ┌────────▼─────────┐  ┌────────┴────────┐  ┌──────────▼───────────┐   │
│  │  Traces Node     │  │     Tracer      │  │     Sequencer        │   │
│  │  (Besu + Plugin) │──┤   (Java Plugin) │◄─┤   (Besu + Plugins)   │   │
│  │                  │  │                 │  │                      │   │
│  │  - Replay blocks │  │  - Trace EVM    │  │  - Block production  │   │
│  │  - Generate      │  │  - Generate     │  │  - TX selection      │   │
│  │    traces        │  │    matrices     │  │  - Gas estimation    │   │
│  └──────────────────┘  └─────────────────┘  │  - Bundle mgmt       │   │
│                                             │                      │   │
│                                             │  ┌─────────────────┐ │   │
│                                             │  │   Maru Engine   │ │   │
│                                             │  │  - Exec layer   │ │   │
│                                             │  └─────────────────┘ │   │
│                                             └──────────────────────┘   │
│                                                                        │
│                                   LINEA L2                             │
└────────────────────────────────────────────────────────────────────────┘
```

## Data Flow: Block → Proof → Finalization

```
┌────────────────────────────────────────────────────────────────────────┐
│                          PROVING PIPELINE                              │
│                                                                        │
│  1. BLOCK CREATION          2. TRACING             3. PROOF GEN        │
│  ┌─────────────────┐        ┌─────────────────┐    ┌─────────────────┐ │
│  │   Sequencer     │        │    Tracer       │    │     Prover      │ │
│  │                 │        │                 │    │                 │ │
│  │  Transactions   │───────▶│  Execute EVM    │───▶│  Execution      │ │
│  │      ↓          │        │      ↓          │    │  Proof          │ │
│  │   L2 Block      │        │  Trace Matrices │    │      ↓          │ │
│  └─────────────────┘        │  (.lt files)    │    │  Compression    │ │
│                             └─────────────────┘    │  Proof          │ │
│                                                    │      ↓          │ │
│                                                    │  Aggregation    │ │
│                                                    │  Proof          │ │
│                                                    └────────┬────────┘ │
│                                                             │          │
│  4. CONFLATION              5. SUBMISSION          6. FINALIZATION     │
│  ┌─────────────────┐        ┌─────────────────┐    ┌─────────────────┐ │
│  │   Coordinator   │        │   Coordinator   │    │   Coordinator   │ │
│  │                 │        │                 │    │                 │ │
│  │  Group blocks   │───────▶│  Submit blob    │───▶│  Submit proof   │ │
│  │  into batches   │        │  to L1 (EIP-4844)│   │  to L1          │ │
│  │                 │        │                 │    │      ↓          │ │
│  │  Trigger limits:│        │  Calculate      │    │  Verify on-chain│ │
│  │  - Time         │        │  shnarf hash    │    │      ↓          │ │
│  │  - Data size    │        │                 │    │  State finalized│ │
│  │  - Trace lines  │        │                 │    │                 │ │
│  │  - Block count  │        │                 │    │                 │ │
│  │  - Target block │        │                 │    │                 │ │
│  │  - Timestamp HF │        │                 │    │                 │ │
│  └─────────────────┘        └─────────────────┘    └─────────────────┘ │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Cross-Chain Messaging Flow

> **Diagrams:** [L1→L2 Message Flow](../diagrams/l1-to-l2-message-flow.mmd) | [L2→L1 Message Flow](../diagrams/l2-to-l1-message-flow.mmd)

### L1 → L2 Message Flow

```
┌────────────────────────────────────────────────────────────────────────┐
│                       L1 → L2 MESSAGE FLOW                             │
│                                                                        │
│  USER/CONTRACT            L1                    L2                     │
│       │                    │                     │                     │
│       │  1. sendMessage()  │                     │                     │
│       │───────────────────▶│ LineaRollup        │                      │
│       │                    │    │                │                     │
│       │                    │    │ 2. MessageSent │                     │
│       │                    │    │    event       │                     │
│       │                    │    ▼                │                     │
│       │                    │ ┌──────────────┐    │                     │
│       │                    │ │ Coordinator  │    │                     │
│       │                    │ │ anchors hash │    │                     │
│       │                    │ └──────┬───────┘    │                     │
│       │                    │        │            │                     │
│       │                    │        │ 3. Anchor  │ L2MessageService    │
│       │                    │        └───────────▶│    │                │
│       │                    │                     │    │ 4. hash stored │
│       │                    │                     │    ▼                │
│       │                    │                     │ ┌──────────────┐    │
│       │                    │                     │ │   Claimable  │    │
│       │                    │                     │ └──────┬───────┘    │
│       │                    │                     │        │            │
│       │  5. claimMessage() │                     │        │            │
│       │  (or Postman)      │                     │◀───────┘            │
│       │─────────────────────────────────────────▶│                     │
│       │                    │                     │  6. Execute         │
│       │                    │                     │     calldata        │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### L2 → L1 Message Flow

```
┌────────────────────────────────────────────────────────────────────────┐
│                       L2 → L1 MESSAGE FLOW                             │
│                                                                        │
│  USER/CONTRACT            L2                    L1                     │
│       │                    │                     │                     │
│       │  1. sendMessage()  │                     │                     │
│       │───────────────────▶│ L2MessageService   │                      │
│       │                    │    │                │                     │
│       │                    │    │ 2. MessageSent │                     │
│       │                    │    │    event       │                     │
│       │                    │    │    + rolling   │                     │
│       │                    │    │    hash update │                     │
│       │                    │    ▼                │                     │
│       │                    │ ┌──────────────┐    │                     │
│       │                    │ │ Merkle tree  │    │                     │
│       │                    │ │ updated      │    │                     │
│       │                    │ └──────────────┘    │                     │
│       │                    │        │            │                     │
│       │                    │        │ 3. Coord.  │ LineaRollup         │
│       │                    │        │ submits    │    │                │
│       │                    │        │ finalization                     │
│       │                    │        └───────────▶│    │                │
│       │                    │                     │    │ 4. Root stored │
│       │                    │                     │    ▼                │
│       │                    │                     │ ┌──────────────┐    │
│       │                    │                     │ │   Claimable  │    │
│       │                    │                     │ └──────┬───────┘    │
│       │                    │                     │        │            │
│       │  5. claimMessage-  │                     │        │            │
│       │     WithProof()    │                     │◀───────┘            │
│       │  (or Postman)      │                     │                     │
│       │─────────────────────────────────────────▶│  6. Verify Merkle   │
│       │                    │                     │     proof + execute │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Component Interactions

> **Diagram:** [Component Interactions](../diagrams/component-interactions.mmd) (Mermaid source)

```
┌────────────────────────────────────────────────────────────────────────┐
│                       COMPONENT INTERACTIONS                           │
│                                                                        │
│  USER LAYER             SDK LAYER             L1 CONTRACTS             │
│  ┌─────────────┐       ┌─────────────┐       ┌──────────────┐          │
│  │   dApps     │──────▶│  SDK Viem   │──────▶│ LineaRollup  │          │
│  │   Wallets   │──────▶│  SDK Ethers │──────▶│ PlonkVerifier│          │
│  │  Bridge UI  │       └─────────────┘       │ TokenBridge  │          │
│  └─────────────┘              │              └──────┬───────┘          │
│                               │                     │                  │
│                               ▼                     │                  │
│  SEQUENCER STACK       L2 CONTRACTS                 │                  │
│  ┌─────────────┐       ┌─────────────┐              │                  │
│  │  Sequencer  │──────▶│L2MessageSvc │◀─────────────┘                  │
│  │    Node     │       │ TokenBridge │                                 │
│  │    Maru     │       └─────────────┘                                 │
│  │   Plugins   │                                                       │
│  └──────┬──────┘                                                       │
│         │                                                              │
│  PROVING STACK         COORDINATION          STORAGE                   │
│  ┌─────────────┐       ┌─────────────┐       ┌─────────────┐           │
│  │ Traces Node │──────▶│ Coordinator │──────▶│ PostgreSQL  │           │
│  │   Tracer    │       │   Shomei    │       └─────────────┘           │
│  │   Prover    │       └─────────────┘                                 │
│  └─────────────┘                                                       │
│                                                                        │
│  CROSS-CHAIN RELAY                                                     │
│  ┌──────────────────────────────────┐                                  │
│  │  Postman (TypeScript)            │                                  │
│  │  Monitors L1 + L2, claims msgs   │                                  │
│  └──────────────────────────────────┘                                  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## ZK Proving Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                       ZK PROVING ARCHITECTURE                          │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                      TRACER (Java Plugin)                        │  │
│  │                                                                  │  │
│  │  EVM Execution ───▶ Module Tracing ───▶ Matrix Generation        │  │
│  │                                                                  │  │
│  │  Modules: HUB, ADD, MUL, MOD, MMU, MMIO, ROM, RLP, EC, BLS, etc. │  │
│  │                                                                  │  │
│  │  Output: .lt files (binary trace format)                         │  │
│  └────────────────────────────────┬─────────────────────────────────┘  │
│                                   │                                    │
│                                   ▼                                    │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                TRACER-CONSTRAINTS (Lisp/Corset)                  │  │
│  │                                                                  │  │
│  │  Constraint Definitions ───▶ go-corset ───▶ Java Code Generation │  │
│  │                                                                  │  │
│  │  hub/           ─ Central coordination constraints               │  │
│  │  alu/           ─ Arithmetic logic unit                          │  │
│  │  mmu/mmio/      ─ Memory management                              │  │
│  │  txndata/       ─ Transaction data                               │  │
│  │  rlptxn/        ─ RLP encoding                                   │  │
│  │  ecdata/blsdata ─ Precompile constraints                         │  │
│  └────────────────────────────────┬─────────────────────────────────┘  │
│                                   │                                    │
│                                   ▼                                    │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                       PROVER (Go/gnark)                          │  │
│  │                                                                  │  │
│  │  ┌───────────────┐   ┌───────────────┐   ┌───────────────┐       │  │
│  │  │   Execution   │   │  Compression  │   │  Aggregation  │       │  │
│  │  │     Prover    │   │    Prover     │   │    Prover     │       │  │
│  │  │               │   │               │   │               │       │  │
│  │  │  Batch proof  │   │  Blob proof   │   │  Combine      │       │  │
│  │  │  correctness  │   │  compression  │   │  proofs       │       │  │
│  │  └───────────────┘   └───────────────┘   └───────────────┘       │  │
│  │                                                                  │  │
│  │  Proof System: PLONK + Vortex polynomial commitments             │  │
│  │  Curves: BLS12-377, BN254, BW6-761                               │  │
│  │  Library: gnark (Consensys)                                      │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Network Topology (Local Development)

```
┌────────────────────────────────────────────────────────────────────────┐
│                      DOCKER NETWORK TOPOLOGY                           │
│                                                                        │
│  L1 NETWORK (10.10.10.0/24)           LINEA NETWORK (11.11.11.0/24)    │
│  ┌─────────────────────────┐          ┌────────────────────────────┐   │
│  │                         │          │                            │   │
│  │  l1-el-node  .201       │          │  sequencer      .101       │   │
│  │  (Besu)                 │          │  (Linea-Besu)              │   │
│  │  :8445 HTTP             │          │  :8545 HTTP                │   │
│  │  :8446 WS               │          │  :8546 WS                  │   │
│  │                         │          │                            │   │
│  │  l1-cl-node  .202       │          │  maru           .210       │   │
│  │  (Teku)                 │          │  (Maru Engine)             │   │
│  │  :4003 REST             │          │  :8080                     │   │
│  │                         │          │                            │   │
│  │  blobscan-api .203      │          │  traces-node    .115       │   │
│  │  :4001                  │          │  :8745 HTTP                │   │
│  │                         │          │                            │   │
│  │  coordinator  .106      │◄────────▶│  coordinator    .106       │   │
│  │  (connected to both)    │          │  :9545                     │   │
│  │                         │          │                            │   │
│  │  postman      .222      │◄────────▶│  postman        .222       │   │
│  │  (connected to both)    │          │  :9090                     │   │
│  │                         │          │                            │   │
│  └─────────────────────────┘          │  prover-v3      .109       │   │
│                                       │  (ZK Prover)               │   │
│                                       │                            │   │
│                                       │  shomei         .114       │   │
│                                       │  :8998                     │   │
│                                       │                            │   │
│                                       │  zkbesu-shomei  .113       │   │
│                                       │  :8945 HTTP                │   │
│                                       │                            │   │
│                                       │  l2-node-besu   .119       │   │
│                                       │  :9045 HTTP                │   │
│                                       │                            │   │
│                                       └────────────────────────────┘   │
│                                                                        │
│  SHARED SERVICES                                                       │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │  postgres :5432    │    redis :6379    │    grafana :3001        │  │
│  │  prometheus :9091  │    loki :3100     │    blockscout L1 :4001  │  │
│  │                                        │    blockscout L2 :4000  │  │
│  └──────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Data Availability Modes

Linea supports two data availability modes:

### Rollup Mode (Default)
- Data submitted via EIP-4844 blobs
- Full data availability on L1
- Higher cost, higher security

### Validium Mode
- Data stored off-chain
- Only commitments on L1
- Lower cost, trust assumptions

```
┌────────────────────────────────────────────────────────────────────────┐
│                       DATA AVAILABILITY MODES                          │
│                                                                        │
│  ROLLUP MODE                        VALIDIUM MODE                      │
│  ┌─────────────────────┐            ┌─────────────────────┐            │
│  │ L2 Blocks           │            │ L2 Blocks           │            │
│  │      │              │            │      │              │            │
│  │      ▼              │            │      ▼              │            │
│  │ Compress to Blob    │            │ Compress to Blob    │            │
│  │      │              │            │      │              │            │
│  │      ▼              │            │      ▼              │            │
│  │ Submit EIP-4844     │            │ Store Off-chain     │            │
│  │ Blob to L1          │            │ (DAC/external)      │            │
│  │      │              │            │      │              │            │
│  │      ▼              │            │      ▼              │            │
│  │ Full data on L1     │            │ Commitment on L1    │            │
│  └─────────────────────┘            └─────────────────────┘            │
│                                                                        │
│  + Higher security                  + Lower cost                       │
│  + No trust assumptions             - Requires data committee          │
│  - Higher L1 gas cost               - Trust assumptions                │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## State Recovery

State recovery allows rebuilding L2 state from L1 data:

```
┌────────────────────────────────────────────────────────────────────────┐
│                       STATE RECOVERY FLOW                              │
│                                                                        │
│  1. Monitor L1          2. Fetch Blobs        3. Decompress & Import   │
│  ┌─────────────┐        ┌─────────────┐       ┌─────────────┐          │
│  │ State       │        │ BlobScan    │       │ zkBesu      │          │
│  │ Recovery    │───────▶│ Client      │──────▶│ + Shomei    │          │
│  │ Plugin      │        │             │       │             │          │
│  │             │        │ Fetch blob  │       │ Decompress  │          │
│  │ Watch       │        │ data from   │       │ blob data   │          │
│  │ LineaRollup │        │ L1/BlobScan │       │             │          │
│  │ events      │        │             │       │ Rebuild     │          │
│  │             │        │             │       │ blocks      │          │
│  └─────────────┘        └─────────────┘       │             │          │
│                                               │ Verify      │          │
│                                               │ state root  │          │
│                                               └─────────────┘          │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

## Key Terminology

Several terms appear throughout the diagrams and docs with distinct meanings:

### "Finalization"

| Context | Meaning |
|---------|---------|
| **Coordinator: "Finalization"** | The coordinator submitting an aggregated ZK proof to `LineaRollup.finalizeBlocksWithProof()` on L1 |
| **L1 contract: "Finalize"** | `LineaRollup` verifying the proof on-chain via `PlonkVerifier` and updating `currentFinalizedShnarf` |
| **L2 block tag: "finalized"** | The Ethereum `finalized` block tag on L2, updated after L1 finalization is confirmed |

### "Anchoring"

| Context | Meaning |
|---------|---------|
| **Coordinator: "Anchor msgs"** | The coordinator calling `L2MessageService.anchorL1L2MessageHashes()` on L2 to store L1→L2 message hashes, making them claimable on L2 |
| **LineaRollup: "Anchor msgs"** | `LineaRollup` storing L2→L1 Merkle roots (`l2MerkleRoots`) during finalization, enabling L2 messages to be claimed on L1 |

These two anchoring operations flow in opposite directions and should not be confused.

## External Dependencies

Some components in the architecture diagrams above are not part of this repository:

- **Maru Engine**: Execution layer for the sequencer (Engine API)
- **Linea Besu**: Modified Besu fork for L2 consensus
- **go-corset**: Constraint compiler for ZK circuits
- **gnark**: ZK proof library

See [External Dependencies](./EXTERNAL-DEPENDENCIES.md) for detailed documentation on these components.

## Next Steps

- [External Dependencies](./EXTERNAL-DEPENDENCIES.md) - Maru, Linea Besu, and other external components
- [Component Details](../components/README.md) - Deep dive into each component
- [Development Guide](../development/README.md) - Build and run locally
- [Operations Guide](../operations/README.md) - Deployment and monitoring
