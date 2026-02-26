# Rollup

> LineaRollup contracts, data submission, and ZK finalization — the full L1 settlement pipeline.

## Overview

Linea operates as a zk-rollup where L2 state transitions are posted to and verified on Ethereum L1. The pipeline has three phases:

1. **Submission** — Compressed L2 block data posted to L1 via EIP-4844 blobs or calldata.
2. **Shnarf chaining** — Each submission extends a rolling commitment (`shnarf`) linking all prior submissions.
3. **Finalization** — An aggregated ZK proof verifies state transitions on-chain, updating the finalized state.

Two contract variants exist:

- **LineaRollup** — Full rollup with EIP-4844 blob DA. Inherits L1 messaging, yield management, and liveness recovery.
- **Validium** — Calldata-only shnarf submission. Lighter footprint, weaker DA guarantees.

Both share `LineaRollupBase` for finalization logic, verifier management, and shnarf computation.

## Components

| Component | Path | Role |
|-----------|------|------|
| LineaRollup | `contracts/src/rollup/LineaRollup.sol` | Main L1 rollup contract |
| LineaRollupBase | `contracts/src/rollup/LineaRollupBase.sol` | Shared finalization, verifier, shnarf logic |
| Validium | `contracts/src/rollup/Validium.sol` | Calldata-only DA variant |
| LivenessRecovery | `contracts/src/rollup/LivenessRecovery.sol` | Emergency operator recovery after 6-month inactivity |
| Eip4844BlobAcceptor | `contracts/src/rollup/dataAvailability/Eip4844BlobAcceptor.sol` | EIP-4844 blob verification on-chain |
| ShnarfDataAcceptor | `contracts/src/rollup/dataAvailability/ShnarfDataAcceptor.sol` | Validium shnarf submission |
| PlonkVerifierForDataAggregation | `contracts/src/verifiers/PlonkVerifierForDataAggregation.sol` | Aggregation proof verification |
| BlobSubmissionCoordinator | `coordinator/ethereum/blob-submitter/` | Periodic blob submission to L1 |
| AggregationFinalizationCoordinator | `coordinator/ethereum/blob-submitter/` | Schedules finalization after aggregation proof |
| Blob Compressor | `jvm-libs/linea/blob-compressor/` | Go native library for block compression |
| Shnarf Calculator | `jvm-libs/linea/blob-shnarf-calculator/` | Go native library for shnarf/KZG computation |

## Inheritance

```
LineaRollup
  ├── LineaRollupBase
  │     ├── PauseManager, RateLimiter, PermissionsManager
  │     └── L1MessageService
  ├── LineaRollupYieldExtension
  ├── LivenessRecovery
  ├── Eip4844BlobAcceptor
  └── ClaimMessageV1

Validium
  ├── LineaRollupBase
  ├── LocalShnarfProvider
  └── ShnarfDataAcceptor
```

## Key State and Roles

| Variable | Type | Description |
|----------|------|-------------|
| `currentFinalizedShnarf` | `bytes32` | Latest finalized shnarf hash |
| `currentFinalizedState` | `bytes32` | Composite of last message number, rolling hash, timestamp |
| `livenessRecoveryOperator` | `address` | Emergency recovery operator address |

| Role | Purpose |
|------|---------|
| `OPERATOR_ROLE` | Submit blobs, finalize blocks |
| `VERIFIER_SETTER_ROLE` / `VERIFIER_UNSETTER_ROLE` | Manage verifier contract addresses |
| `DEFAULT_ADMIN_ROLE` | Grant/revoke roles |

---

## Data Submission

The coordinator compresses batches of L2 blocks into blobs and submits them to LineaRollup on L1.

### Submission Flow

```mermaid
sequenceDiagram
    participant Coord as Coordinator
    participant Comp as BlobCompressor
    participant Shnarf as ShnarfCalculator
    participant L1 as LineaRollup (L1)

    Coord->>Comp: Compress batches into blob
    Comp-->>Coord: compressedData (≤127KB)
    Coord->>Shnarf: calculateShnarf(compressedData, stateRoots, prevShnarf)
    Shnarf-->>Coord: commitment, kzgProofs, expectedShnarf
    Coord->>L1: submitBlobs(blobSubmissionData[], parentShnarf, finalBlobShnarf)
    L1->>L1: Point evaluation precompile verify per blob
    L1->>L1: Chain shnarf, store, emit DataSubmittedV3
```

### Contract Interface

```solidity
function submitBlobs(
    BlobSubmission[] calldata _blobSubmissions,
    bytes32 _parentShnarf,
    bytes32 _finalBlobShnarf
) external;

struct BlobSubmission {
    uint256 dataEvaluationClaim;
    bytes kzgCommitment;
    bytes kzgProof;
    bytes32 finalStateRootHash;
    bytes32 snarkHash;
}
```

Point evaluation uses `POINT_EVALUATION_PRECOMPILE_ADDRESS` (0x0a) with `BLS_CURVE_MODULUS` field validation and `POINT_EVALUATION_FIELD_ELEMENTS_LENGTH` (4096).

### Shnarf Computation

The shnarf is a rolling commitment linking consecutive submissions:

```
shnarf = keccak256(parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim)
```

### Blob Structure

Blobs pack arbitrary data into BLS12-381 scalar field elements (254 usable bits per 32-byte chunk):

```
Header:
  version      uint16   // currently 0xffff
  dictChecksum [32]byte
  nbBatches    uint16
  batchNbBytes []uint24 // per-batch size

Payload:
  compress header (version uint16, bypassed uint8)
  RLP-encoded blocks (boundary from header)
```

Padding uses `0xFF000000...` to fill to exactly 127 KB (130,047 bytes usable).

### Coordinator Blob Grouping

`BlobsGrouperForSubmission` chunks blobs into groups of up to 6 for multi-blob transactions. `L1ShnarfBasedAlreadySubmittedBlobsFilter` skips blobs already on-chain. `ContractUpgradeSubmissionLatchFilter` blocks submission during contract upgrades.

---

## Finalization

Finalization posts an aggregated ZK proof to LineaRollup, proving that a range of L2 blocks were executed correctly.

### Finalization Flow

```mermaid
sequenceDiagram
    participant Prover
    participant Coord as Coordinator
    participant L1 as LineaRollup (L1)
    participant Verifier as PlonkVerifier

    Prover-->>Coord: Aggregation proof response (file system)
    Coord->>L1: finalizeBlocks(aggregatedProof, proofType, finalizationData)
    L1->>L1: Validate shnarf continuity
    L1->>L1: Validate rolling hash (L1→L2 message integrity)
    L1->>L1: Anchor L2→L1 Merkle roots
    L1->>Verifier: _verifyProof(publicInput, proof)
    Verifier-->>L1: proof valid
    L1->>L1: Update currentFinalizedShnarf, currentFinalizedState
    L1->>L1: Emit DataFinalizedV3
```

### Contract Interface

```solidity
function finalizeBlocks(
    bytes calldata _aggregatedProof,
    uint256 _proofType,
    FinalizationDataV3 calldata _finalizationData
) public whenTypeAndGeneralNotPaused(PauseType.FINALIZATION) onlyRole(OPERATOR_ROLE);

struct FinalizationDataV3 {
    bytes32 parentStateRootHash;
    uint256 endBlockNumber;
    ShnarfData shnarfData;
    uint256 lastFinalizedTimestamp;
    uint256 finalTimestamp;
    bytes32 lastFinalizedL1RollingHash;
    bytes32 l1RollingHash;
    uint256 lastFinalizedL1RollingHashMessageNumber;
    uint256 l1RollingHashMessageNumber;
    uint256 l2MerkleTreesDepth;
    bytes32[] l2MerkleRoots;
    bytes l2MessagingBlocksOffsets;
}
```

### Public Input Computation

The verifier receives a single `uint256` public input derived from `_computePublicInput` in `LineaRollupBase`:

```
keccak256(
    lastFinalizedShnarf, finalShnarf,
    lastFinalizedTimestamp, finalTimestamp,
    lastFinalizedBlockNumber, endBlockNumber,
    lastFinalizedL1RollingHash, l1RollingHash,
    lastFinalizedL1RollingHashMessageNumber, l1RollingHashMessageNumber,
    l2MerkleTreesDepth, keccak256(l2MerkleRoots),
    verifierChainConfiguration
) mod MODULO_R
```

`MODULO_R` is the BN254 scalar field order. `verifierChainConfiguration` is obtained from the verifier contract via `IPlonkVerifier.getChainConfiguration()`. The Merkle roots array is pre-hashed before inclusion.

### Rolling Hash Validation

Finalization verifies that the L1→L2 rolling hash stored on L2 matches the expected value on L1, guaranteeing no censored or fabricated messages and preserved ordering.

### Proof System

The aggregation proof recursively verifies N execution proofs and M compression proofs:

- **BLS12-377** — Execution and compression proofs
- **BW6** — Intermediate recursion (2-chain with BLS12-377)
- **BN254** — Final proof, verifiable on Ethereum via `ecPairing` precompile

### Errors

| Error | Condition |
|-------|-----------|
| `FinalShnarfNotSubmitted` | Finalization references a shnarf not yet submitted |
| `FinalizationStateIncorrect` | Parent state does not match current finalized state |
| `FinalizationInTheFuture` | Final timestamp exceeds current block timestamp |
| `InvalidProof` | Proof verification failed |
| `StartingRootHashDoesNotMatch` | Parent state root mismatch |

---

## Liveness Recovery

If no finalization occurs for `SIX_MONTHS_IN_SECONDS` (182 days, due to Solidity integer division `365 / 2`), any caller can invoke `setLivenessRecoveryOperator()` to grant `OPERATOR_ROLE` to the pre-configured recovery operator. This prevents permanent lock-up of bridged funds.

## Events

| Event | Description |
|-------|-------------|
| `DataSubmittedV3` | Blob data accepted on L1 |
| `DataFinalizedV3` | Blocks finalized with proof |
| `VerifierAddressChanged` | Verifier contract updated |

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `contracts/test/hardhat/rollup/LineaRollup.ts` | Hardhat | Initialization, roles, pause, operator actions |
| `contracts/test/hardhat/rollup/LineaRollup/BlobSubmission.ts` | Hardhat | EIP-4844 blob submission, KZG proofs, calldata |
| `contracts/test/hardhat/rollup/LineaRollup/Finalization.ts` | Hardhat | Proof verification, state updates, error cases |
| `contracts/test/hardhat/rollup/Validium.ts` | Hardhat | Validium-specific behavior |
| `contracts/test/hardhat/verifiers/PlonkVerifierForDataAggregation.ts` | Hardhat | Verifier contract correctness |
| `contracts/test/foundry/LineaRollup.t.sol` | Foundry | Shnarf calculation, `_calculateY` |
| `e2e/src/submission-finalization.spec.ts` | Jest | End-to-end submission and finalization |
| `e2e/src/restart.spec.ts` | Jest | Finalization resumes after coordinator restart |

## Related Documentation

- [Architecture: Coordinator](../architecture-description.md#coordinator)
- [Architecture: Blob Compressor](../architecture-description.md#blob-compressor)
- [Architecture: Provers](../architecture-description.md#provers)
- [Tech: Contracts Component](../tech/components/contracts.md) — Contract addresses, directory structure, deployment details
- [Workflow: LineaRollup](../../contracts/docs/workflows/LineaRollup.md)
- [Workflow: Blob Submission and Finalization](../../contracts/docs/workflows/operations/blobSubmissionAndFinalization.md)
- [Official docs: Smart Contracts](https://docs.linea.build/protocol/architecture/smart-contracts)
- [Official docs: Transaction Lifecycle](https://docs.linea.build/technology/transaction-lifecycle)
