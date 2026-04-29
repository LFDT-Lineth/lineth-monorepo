# Linea Forced Transactions Architecture

This document provides a comprehensive view of the Forced Transaction system in the Linea rollup, including contract architecture, data flows, and the processing guarantee mechanism.

## Table of Contents

1. [System Overview](#system-overview)
2. [Contract Architecture](#contract-architecture)
3. [Core Components](#core-components)
4. [Forced Transaction Submission Flow](#forced-transaction-submission-flow)
5. [Rolling Hash: Proving Integrity](#rolling-hash-proving-integrity)
6. [Finalization & Processing Guarantees](#finalization--processing-guarantees)
7. [Important: Processing vs Successful Execution](#important-processing-vs-successful-execution)
8. [Sourcing the Last Finalized State](#sourcing-the-last-finalized-state)
9. [Security Mechanisms](#security-mechanisms)

---

## System Overview

The Forced Transaction system provides **processing guarantees** for Linea L2. It allows users to submit transactions directly to L1 that **must** be processed (attempted) by the sequencer **by** a specified block deadline. The sequencer will typically process the transaction well before the deadline - the deadline represents the **latest acceptable block**, not the target block. If the sequencer fails to process a forced transaction by its deadline, finalization will revert.

**Important:** A processing guarantee means the transaction will be **attempted** by the deadline. It does NOT guarantee successful execution - the transaction may still fail on L2 due to invalid nonce, insufficient gas, insufficient balance. See [Processing vs Successful Execution](#important-processing-vs-successful-execution) for details.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         FORCED TRANSACTION FLOW                             │
└─────────────────────────────────────────────────────────────────────────────┘

   USER              L1 CONTRACTS         COORDINATOR    L2 SEQUENCER    PROVER
  ┌─────┐           ┌─────────────┐      ┌───────────┐  ┌───────────┐  ┌────────┐
  │     │           │             │      │           │  │           │  │        │
  │  1. │ Sign tx   │             │      │           │  │           │  │        │
  │     │ (EIP-1559)│             │      │           │  │           │  │        │
  │     │           │             │      │           │  │           │  │        │
  │  2. │ Query     │  Gateway    │      │  Listens  │  │           │  │        │
  │     │ state     │      +      │      │  for L1   │  │           │  │        │
  │     │<─────────>│  Rollup     │      │  events   │  │           │  │        │
  │     │           │      +      │      │           │  │           │  │        │
  │  3. │ Submit    │  Filter     │      │           │  │           │  │        │
  │     │ + fee     │             │      │           │  │           │  │        │
  │     │──────────>│             │      │           │  │           │  │        │
  │     │           │             │      │           │  │           │  │        │
  │     │           │ 4. Store tx │      │           │  │           │  │        │
  │     │           │ 5. Emit     │      │           │  │           │  │        │
  │     │<──────────│     event   │      │           │  │           │  │        │
  │     │           │─────────────┼─────>│           │  │           │  │        │
  │     │           │             │      │  6. Forward  │           │  │        │
  │     │           │             │      │  tx to    │  │           │  │        │
  │     │           │             │      │  sequencer│  │           │  │        │
  │     │           │             │      │──────────>│  │           │  │        │
  │     │           │             │      │           │  │ 7. Process│  │        │
  │     │           │             │      │           │  │ BY deadline  │        │
  │     │           │             │      │           │  │──────────>│  │        │
  │     │           │             │      │           │  │ 8. L2 state  │        │
  │     │           │             │      │           │  │           │  │9.Prove │
  │     │           │             │      │           │  │           │  │off-chain
  │     │           │ 10. Submit  │      │           │  │           │  │        │
  │     │           │ proof +     │      │           │  │           │  │        │
  │     │           │ finalization│<─────┼───────────┼──┼───────────┼──│        │
  │     │           │             │      │           │  │           │  │        │
  └─────┘           └─────────────┘      └───────────┘  └───────────┘  └────────┘
```

> **Diagram:** For a detailed Mermaid version, see [diagrams/forced-transactions/system-overview.mmd](./diagrams/forced-transactions/system-overview.mmd)

### The Coordinator's Role

The **Coordinator** is an off-chain service that bridges L1 and L2:

1. **Listens** to `ForcedTransactionAdded` events emitted by LineaRollup on L1
2. **Extracts** the RLP-encoded signed transaction from the event
3. **Submits** the transaction to the Sequencer for processing on L2 via `linea_sendForcedRawTransaction` JSON-RPC including `forcedTransactionNumber`

The Coordinator ensures that forced transactions registered on L1 are actually delivered to the Sequencer. Without this component, the Sequencer would have no way of knowing about forced transactions.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         COORDINATOR RESPONSIBILITIES                        │
└─────────────────────────────────────────────────────────────────────────────┘

  L1 LineaRollup                    Coordinator                    L2 Sequencer
  ───────────────                   ───────────                    ────────────
        │                                │                              │
        │  ForcedTransactionAdded        │                              │
        │  event emitted                 │                              │
        │───────────────────────────────>│                              │
        │                                │                              │
        │  Event contains:               │  Extract & forward:          │
        │  - forcedTransactionNumber     │  - rlpEncodedSignedTx        │
        │  - from (signer)               │  - deadline info             │
        │  - blockNumberDeadline         │  - forcedTransaction         │
        │  - rollingHash                 │     Number                   │
        │  - rlpEncodedSignedTransaction │─────────────────────────────>│
        │                                │                              │
        │                                │                              │  Process tx
        │                                │                              │  by deadline
```

### The Sequencer's Role
The **Sequencer** is responsible for block production:

1. **Receives** the forced transaction from the Coordinator
2. **Evaluates** the forced transaction by forced transaction number and before the deadline

---

## Contract Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    CONTRACT & SERVICE RELATIONSHIPS                         │
└─────────────────────────────────────────────────────────────────────────────┘

  ┌──────────────────────────────────────┐
  │   ForcedTransactionGateway           │
  │   ────────────────────────────────── │
  │   - LINEA_ROLLUP                     │
  │   - DESTINATION_CHAIN_ID             │
  │   - L2_BLOCK_BUFFER                  │
  │   - L2_BLOCK_DURATION_SECONDS        │
  │   - BLOCK_NUMBER_DEADLINE_BUFFER     │
  │   - MAX_GAS_LIMIT                    │
  │   - MAX_INPUT_LENGTH_LIMIT           │
  │   - ADDRESS_FILTER                   │
  │   ────────────────────────────────── │
  │   + submitForcedTransaction          │
  │   + toggleUseAddressFilter           │
  └──────────────────┬───────────────────┘
                     │ calls
                     ▼
  ┌─────────────────────────────┐      ┌─────────────────────────────┐
  │        LineaRollup          │      │       AddressFilter         │
  │   ───────────────────────── │      │   ───────────────────────── │
  │   - currentFinalizedState   │      │   - filteredAddresses       │
  │   - nextForcedTxNumber      │◄────►│   ───────────────────────── │
  │   - forcedTxL2BlockNumbers  │      │   + addressIsFiltered       │
  │   - forcedTxRollingHashes   │      │   + setFilteredStatus       │
  │   - forcedTxFeeInWei        │      └─────────────────────────────┘
  │   ───────────────────────── │
  │   + storeForcedTransaction  │      ┌─────────────────────────────┐
  │   + finalizeBlocks          │      │    Libraries (Mimc,         │
  │   + getRequiredFields       │◄────►│    FinalizedStateHashing)   │
  └──────────────┬──────────────┘      └─────────────────────────────┘
                 │
                 │ emits ForcedTransactionAdded
                 ▼
  ┌─────────────────────────────┐      ┌─────────────────────────────┐
  │    Coordinator (off-chain)  │      │    Sequencer (L2 node)      │
  │   ───────────────────────── │      │   ───────────────────────── │
  │   Listens for events        │─────>│   Receives forced txs       │
  │   Extracts signed tx        │      │   Processes forced txs      │
  │   Submits to Sequencer      │      │   Must process BY deadline  │
  └─────────────────────────────┘      └──────────────┬──────────────┘
                                                      │
                                                      │ provides L2 state
                                                      ▼
                                       ┌─────────────────────────────┐
                                       │     Prover (off-chain)      │
                                       │   ───────────────────────── │
                                       │   Generates ZK proof        │
                                       │   Computes rolling hash     │
                                       │   in proof circuit          │
                                       └──────────────┬──────────────┘
                                                      │
                                                      │ submits proof + finalization
                                                      ▼
                                              ┌───────────────┐
                                              │ LineaRollup   │
                                              │ finalizeBlocks│
                                              └───────────────┘
```

> **Diagram:** For a detailed Mermaid class diagram, see [diagrams/forced-transactions/contract-architecture.mmd](./diagrams/forced-transactions/contract-architecture.mmd)

---

## Core Components

### 1. ForcedTransactionGateway

The user-facing contract that validates and submits forced transactions.

| Parameter | Description |
|-----------|-------------|
| `LINEA_ROLLUP` | Reference to the LineaRollup contract |
| `DESTINATION_CHAIN_ID` | L2 chain ID for RLP encoding |
| `L2_BLOCK_BUFFER` | Buffer added to deadline calculation (in L2 blocks) |
| `L2_BLOCK_DURATION_SECONDS` | L2 block time in seconds, used to convert elapsed time to L2 blocks in deadline calculation |
| `BLOCK_NUMBER_DEADLINE_BUFFER` | Buffer added when deadline would not be strictly increasing (ensures monotonic deadlines) |
| `MAX_GAS_LIMIT` | Maximum allowed gas limit per forced tx |
| `MAX_INPUT_LENGTH_LIMIT` | Maximum calldata length |
| `ADDRESS_FILTER` | Contract for address filtering |

### 2. LineaRollup

The main rollup contract that stores forced transactions and enforces processing during finalization.

| Storage | Description |
|---------|-------------|
| `currentFinalizedState` | Hash of the last finalized state components |
| `nextForcedTransactionNumber` | Counter for forced transactions (starts at 1) |
| `forcedTransactionL2BlockNumbers` | Maps tx number → L2 block deadline (must be processed BY this block) |
| `forcedTransactionRollingHashes` | Maps tx number → MiMC rolling hash |
| `forcedTransactionFeeInWei` | Required fee to submit a forced tx |
| `addressFilter` | Address filter contract instance |

### 3. AddressFilter

Maintains a list of filtered addresses that cannot participate in forced transactions. This is used for various purposes such as filtering EVM precompiles as destination addresses.

```
┌─────────────────────────────────────────────────────────┐
│                   ADDRESS FILTER FLOW                   │
└─────────────────────────────────────────────────────────┘

                    ┌──────────────────┐
    Admin ─────────>│ setFilteredStatus│
                    └────────┬─────────┘
                             │ updates
                             ▼
                    ┌──────────────────┐
                    │ filteredAddresses│
                    │     mapping      │
                    └────────┬─────────┘
                             │ checked by
                             ▼
   ┌─────────┐      ┌──────────────────┐      ┌─────────────┐
   │ Gateway │─────>│ addressIsFiltered│─────>│ true: FILTER│
   │         │      │   (sender, to)   │      │ false: ALLOW│
   └─────────┘      └──────────────────┘      └─────────────┘
```

### 4. Coordinator (Off-Chain Service)

The Coordinator is an off-chain service that bridges L1 events to the L2 Sequencer:

| Responsibility | Description |
|----------------|-------------|
| **Event Listening** | Monitors L1 for `ForcedTransactionAdded` events |
| **Transaction Extraction** | Extracts the RLP-encoded signed transaction from event data |
| **Sequencer Submission** | Forwards the transaction to the Sequencer for L2 processing via `linea_sendForcedRawTransaction` JSON-RPC including `forcedTransactionNumber` |

Without the Coordinator, the Sequencer would have no way of knowing about forced transactions submitted on L1.

---

## Forced Transaction Submission Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SUBMISSION PROCESS                                 │
└─────────────────────────────────────────────────────────────────────────────┘

PHASE 1: DATA GATHERING
─────────────────────────────────────────────────────────────────────────────
User queries L1 for:
  • FinalizedStateUpdated events → timestamp, messageNumber, forcedTxNumber
  • rollingHashes[messageNumber] → messageRollingHash
  • forcedTransactionRollingHashes[forcedTxNumber] → forcedTxRollingHash

PHASE 2: TRANSACTION PREPARATION
─────────────────────────────────────────────────────────────────────────────
User constructs:
  • LastFinalizedState struct with the 5 components above
  • Signs an EIP-1559 transaction destined for L2

PHASE 3: VALIDATION (in Gateway)
─────────────────────────────────────────────────────────────────────────────
  ┌─ Gas limit checks (21000 <= gasLimit <= MAX_GAS_LIMIT)
  ├─ Calldata length check (input.length <= MAX_INPUT_LENGTH_LIMIT)
  ├─ Fee parameter checks (maxFeePerGas, maxPriorityFeePerGas > 0)
  ├─ Fee ordering check (maxPriorityFeePerGas <= maxFeePerGas)
  ├─ Signature parity check (yParity <= 1)
  ├─ One-tx-per-block check (block.number > lastSubmissionBlock)
  ├─ msg.value == forcedTransactionFeeInWei
  ├─ LastFinalizedState hash matches currentFinalizedState
  ├─ Signature recovery (ECDSA.recover) succeeds
  └─ Address filter checks pass for both sender and recipient (if enabled)

PHASE 4: STORAGE & CHAINING
─────────────────────────────────────────────────────────────────────────────
  1. Calculate blockNumberDeadline (the LATEST block by which tx must be processed):
     deadline = finalizedL2Block
                + (block.timestamp - lastFinalized.timestamp) / L2_BLOCK_DURATION_SECONDS
                + L2_BLOCK_BUFFER

     Note: The division converts elapsed wall-clock time into L2 blocks.
           The sequencer will typically process the tx much sooner than this.
           This is a "must be done BY" constraint, not a target block.

     If deadline would not be strictly increasing:
     if (deadline <= previousForcedTransactionBlockDeadline)
       deadline = previousForcedTransactionBlockDeadline + BLOCK_NUMBER_DEADLINE_BUFFER

     This ensures deadlines are monotonically increasing across forced transactions.

  2. Compute new rolling hash via MiMC:
     newRollingHash = MiMC(prevRollingHash, txHashMSB, txHashLSB, deadline, signer)

  3. Store in LineaRollup:
     forcedTransactionRollingHashes[nextNumber] = newRollingHash
     forcedTransactionL2BlockNumbers[nextNumber] = deadline
     nextForcedTransactionNumber++

  4. Emit ForcedTransactionAdded event with RLP-encoded signed tx
```

> **Diagram:** For a detailed Mermaid sequence diagram, see [diagrams/forced-transactions/submission-flow.mmd](./diagrams/forced-transactions/submission-flow.mmd)

---

## Rolling Hash: Proving Integrity

The forced transaction system uses a **MiMC-based rolling hash** to create a cryptographic chain. This is critical for proving that the exact set of forced transactions was processed - **no exclusions and no unauthorized additions**.

### Purpose of the Rolling Hash

The rolling hash serves as a **commitment to the complete ordered sequence** of forced transactions. When the L2 prover generates a finalization proof, it must demonstrate that:

1. All forced transactions up to `finalForcedTransactionNumber` were processed
2. The computed rolling hash matches the stored `forcedTransactionRollingHashes[finalForcedTransactionNumber]`
3. No transactions in the sequence were skipped, reordered, or fabricated

The rolling hash provides **tamper-proof integrity**:
- **No exclusions**: If any forced transaction is skipped, the rolling hash won't match
- **No additions**: Fabricated transactions not registered on L1 cannot produce a valid rolling hash
- **No reordering**: The chain structure ensures transactions are processed in the correct order

### Rolling Hash Chain Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         ROLLING HASH CHAIN                                  │
└─────────────────────────────────────────────────────────────────────────────┘

   Tx #0 (init)        Tx #1                Tx #2                Tx #N
  ┌──────────┐      ┌──────────┐         ┌──────────┐         ┌──────────┐
  │  0x00..0 │─────>│  Hash 1  │────────>│  Hash 2  │─ ... ──>│  Hash N  │
  │  (zero)  │      │          │         │          │         │          │
  └──────────┘      └──────────┘         └──────────┘         └──────────┘
                          ▲                    ▲                    ▲
                          │                    │                    │
                    ┌─────┴─────┐        ┌─────┴─────┐        ┌─────┴─────┐
                    │ MiMC of:  │        │ MiMC of:  │        │ MiMC of:  │
                    │ - prevHash│        │ - prevHash│        │ - prevHash│
                    │ - txMSB   │        │ - txMSB   │        │ - txMSB   │
                    │ - txLSB   │        │ - txLSB   │        │ - txLSB   │
                    │ - deadline│        │ - deadline│        │ - deadline│
                    │ - signer  │        │ - signer  │        │ - signer  │
                    └───────────┘        └───────────┘        └───────────┘
```

### How MiMC Hashing Works

```solidity
// 1. Hash the unsigned EIP-1559 transaction fields
bytes32 hashedPayload = keccak256(abi.encodePacked(hex"02", rlpEncode(txFields)));

// 2. Split into MSB and LSB (MiMC operates on smaller field elements)
bytes32 hashedPayloadMsb = hashedPayload >> 128;
bytes32 hashedPayloadLsb = hashedPayload & 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF;

// 3. Compute rolling hash using SNARK-friendly MiMC
bytes32 newRollingHash = Mimc.hash(abi.encode(
    previousForcedTransactionRollingHash,
    hashedPayloadMsb,
    hashedPayloadLsb,
    blockNumberDeadline,
    signer
));
```

The MiMC hash function is specifically chosen because it is **SNARK-friendly**, meaning it can be efficiently verified inside the ZK proof.

---

## Finalization & Processing Guarantees

Finalization is a two-phase process:

1. **Off-chain Proving**: The Prover generates a ZK proof covering the forced transactions and computes the rolling hash within the proof circuit
2. **On-chain Verification**: The proof and finalization data are submitted to L1, where the rollup contract verifies the proof and checks that all forced transactions with passed deadlines were processed

### Understanding the Deadline

The `blockNumberDeadline` is the **latest L2 block by which** the transaction must be processed - it is NOT a specific target block:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      DEADLINE SEMANTICS                                     │
└─────────────────────────────────────────────────────────────────────────────┘

  Deadline = Block 1000 means:
  
    Block 1    Block 500    Block 1000    Block 1001
      │           │             │             │
      ▼           ▼             ▼             ▼
  ────┬───────────┬──────────────┬────────────┬─────────────────────────────
      │           │              │            │
      │◄────────────────────────►│            │
      │   VALID: Tx can be       │            │
      │   processed anywhere     │            │
      │   in this range          │            │
      │                          │            │
                                 │            │
                            Last valid    TOO LATE
                              block      (finalization
                                           will fail)
```

The sequencer will typically process forced transactions **well before** the deadline. The buffer (e.g., 3 days worth of blocks) exists to account for:
- Time between L1 submission and sequencer awareness
- Network latency and reorg handling
- Operational flexibility for the sequencer

### Processing Check Logic

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   FINALIZATION PROCESSING CHECK                             │
└─────────────────────────────────────────────────────────────────────────────┘

  OFF-CHAIN (Prover)                        ON-CHAIN (LineaRollup)
  ──────────────────                        ─────────────────────
        │                                          │
  ┌─────┴─────┐                                    │
  │ Generate  │                                    │
  │ ZK proof  │                                    │
  │ including │                                    │
  │ forced txs│                                    │
  └─────┬─────┘                                    │
        │                                          │
  ┌─────┴─────┐                          ┌─────────┴─────────┐
  │ Compute   │                          │  finalizeBlocks   │
  │ rolling   │    Submit proof +        │  called with      │
  │ hash in   │    finalization data     │  proof + data     │
  │ circuit   │─────────────────────────>│                   │
  └───────────┘                          └─────────┬─────────┘
                                                   │
                                                   ▼
                                         ┌─────────────────────┐
                                         │ Validate last       │
                                         │ finalized state     │
                                         └─────────┬───────────┘
                                                   │
                                    No ┌───────────┴───────────┐ Yes
                                 ┌─────┤   State matches?      ├─────┐
                                 │     └───────────────────────┘     │
                                 ▼                                   ▼
                          ┌──────────┐                    ┌──────────────────┐
                          │  REVERT  │                    │ Check forced tx  │
                          │          │                    │ deadlines        │
                          └──────────┘                    └────────┬─────────┘
                                                                   │
                                                    ┌──────────────┴──────────────┐
                                                    │                             │
                                   Has any deadline passed that                   │
                                   wasn't processed before deadline?              │
                                                    │                             │
                                           Yes      │      No                     │
                                      ┌─────────────┴─────────────┐               │
                                      ▼                           ▼               │
                               ┌──────────────┐          ┌────────────────┐       │
                               │    REVERT:   │          │  Verify ZK     │       │
                               │ Missing Tx!  │          │  proof         │       │
                               └──────────────┘          └────────┬───────┘       │
                                                                  │               │
                                                         ┌────────┴───────┐       │
                                                         │ Verify rolling │       │
                                                         │ hash matches   │       │
                                                         └────────┬───────┘       │
                                                                  │               │
                                                         ┌────────┴───────┐       │
                                                         │ Update state & │       │
                                                         │ emit events    │       │
                                                         └────────────────┘       │
```

### Rolling Hash Existence Check (from LineaRollup)

Before the censorship check, finalization verifies that the rolling hash exists for the claimed final forced transaction:

```solidity
bytes32 finalForcedTransactionRollingHash = forcedTransactionRollingHashes[
    _finalizationData.finalForcedTransactionNumber
];

// If a non-zero forced tx number is claimed but has no rolling hash, revert
if (
    _finalizationData.finalForcedTransactionNumber > 0 &&
    finalForcedTransactionRollingHash == EMPTY_HASH
) {
    revert MissingRollingHashForForcedTransactionNumber(
        _finalizationData.finalForcedTransactionNumber
    );
}
```

### The Censorship Resistance Check (from LineaRollup)

```solidity
// Get the NEXT forced transaction number after the one being finalized
uint256 nextFinalizationStartingForcedTxNumber = forcedTransactionL2BlockNumbers[
    _finalizationData.finalForcedTransactionNumber + 1
];

// If it exists AND its deadline has passed (deadline <= endBlockNumber),
// the sequencer MUST have processed it by now
if (
    nextFinalizationStartingForcedTxNumber > 0 &&
    nextFinalizationStartingForcedTxNumber <= _finalizationData.endBlockNumber
) {
    revert FinalizationDataMissingForcedTransaction(
        _finalizationData.finalForcedTransactionNumber + 1
    );
}
```

### Example Scenario

```
Forced Tx #5 submitted with deadline = Block 1000
(meaning: must be processed BY block 1000, could be processed anytime before)

Operator attempts to finalize blocks up to Block 1500
Operator claims: "I processed forced txs up to #4"

Check: Does forced tx #5 exist? YES
Check: Is its deadline (1000) <= endBlockNumber (1500)? YES
       → The deadline has PASSED

Result: REVERT - Tx #5 should have been processed somewhere in blocks 1-1000
```

The transaction could have been processed at **any block from 1 to 1000**. The deadline is not a specific target block - it's the **latest acceptable block** by which processing must occur.

> **Diagram:** For a detailed Mermaid flowchart, see [diagrams/forced-transactions/finalization-flow.mmd](./diagrams/forced-transactions/finalization-flow.mmd)

---

## Important: Processing vs Successful Execution

**A forced transaction being "processed" does NOT guarantee successful execution on L2.**

The forced transaction system guarantees that the sequencer will **attempt to execute** the transaction before the deadline. However, the transaction itself may fail on L2 for various reasons:

### Reasons a Forced Transaction May Fail on L2

| Failure Reason | Description |
|----------------|-------------|
| **Invalid Nonce** | The signer's nonce on L2 has changed since the transaction was signed. Another transaction may have executed first. |
| **Insufficient Gas** | The `gasLimit` specified is too low for the transaction to complete. |
| **Insufficient Balance** | The signer doesn't have enough ETH on L2 to cover `value + (gasLimit * maxFeePerGas)`. |
| **Contract Revert** | The target contract's logic reverted the call (require failed, custom error, etc.). |
| **Out of Gas During Execution** | Complex computation exhausted the gas limit mid-execution. |
| **Invalid Signature** | Edge cases where the signature is technically valid but doesn't match L2 state. |

### What "Processing" Actually Means

```
┌─────────────────────────────────────────────────────────────────────────────┐
│               PROCESSING vs SUCCESSFUL EXECUTION                            │
└─────────────────────────────────────────────────────────────────────────────┘

  L1 Guarantee:               L2 Reality:
  ─────────────               ───────────
  "Transaction WILL           "Transaction was ATTEMPTED"
   be processed by            
   block deadline"            Could result in:
                               ├─ Successful execution ✓
                               ├─ Reverted execution ✗
                               ├─ Out of gas ✗
                               └─ Nonce error ✗

  The Flow:
  ─────────
  L1 Event ──> Coordinator ──> Sequencer ──> L2 Block ──> Prover ──> L1 Finalization
                    │              │                         │              │
             Listens &      Must process            Generate proof    Submit proof
             forwards       BY deadline              (off-chain)      to verify

  All execution outcomes (success or failure) count as "processed"
  from the forced tx perspective. The prover generates a proof that 
  the tx was ATTEMPTED (off-chain), then submits it to L1 for verification.
```

### User Recommendations

1. **Check your nonce** - Query your L2 account nonce right before signing
2. **Use sufficient gas** - Estimate gas on L2 and add a buffer
3. **Ensure adequate balance** - Have enough L2 ETH for worst-case gas cost
4. **Test your calldata** - If calling a contract, simulate the call first
5. **Consider timing** - The 3-day buffer means L2 state may change significantly

---

## Sourcing the Last Finalized State

Users must provide the `LastFinalizedState` struct when submitting forced transactions. Here's how to obtain each component:

### Data Source Map

```
┌─────────────────────────────────────────────────────────────────────────────┐
│              LAST FINALIZED STATE COMPONENTS                                │
└─────────────────────────────────────────────────────────────────────────────┘

  LastFinalizedState {
    timestamp ──────────────────────┐
    messageNumber ──────────────────┼──> From FinalizedStateUpdated event
    forcedTransactionNumber ────────┘
    
    messageRollingHash ─────────────────> lineaRollup.rollingHashes(messageNumber)
    
    forcedTransactionRollingHash ───────> lineaRollup.forcedTransactionRollingHashes(
                                              forcedTransactionNumber
                                          )
  }
```

### Step-by-Step Data Retrieval

```typescript
// 1. Get the latest FinalizedStateUpdated event
const events = await lineaRollup.queryFilter(
    lineaRollup.filters.FinalizedStateUpdated()
);
const latestEvent = events[events.length - 1];

// 2. Extract values from the event
const { 
    blockNumber,              // L2 block number (indexed)
    timestamp,                // Finalization timestamp
    messageNumber,            // L1->L2 message number
    forcedTransactionNumber   // Last finalized forced tx number
} = latestEvent.args;

// 3. Query rolling hashes from contract state
const messageRollingHash = await lineaRollup.rollingHashes(messageNumber);
const forcedTransactionRollingHash = await lineaRollup
    .forcedTransactionRollingHashes(forcedTransactionNumber);

// 4. Construct the LastFinalizedState
const lastFinalizedState = {
    timestamp,
    messageNumber,
    messageRollingHash,
    forcedTransactionNumber,
    forcedTransactionRollingHash
};

// 5. OPTIONAL: Verify it matches the contract's stored hash
const expectedHash = keccak256(abi.encode(
    messageNumber,
    messageRollingHash,
    forcedTransactionNumber,
    forcedTransactionRollingHash,
    timestamp
));
const storedHash = await lineaRollup.currentFinalizedState();
assert(expectedHash === storedHash, "State mismatch - data may be stale");
```

### Using an Indexer

For production applications, consider using an indexer service that:
- Continuously indexes `FinalizedStateUpdated` events
- Pre-fetches the rolling hashes
- Provides an API endpoint returning the complete `LastFinalizedState`

---

## Security Mechanisms

### 1. Address Filtering

The system maintains a list of filtered addresses that cannot participate in forced transactions. The filter is used for various purposes, including:

- **EVM Precompiles**: Filtered by default to prevent unexpected behavior (all precompile addresses supported by the current L2 EVM fork)
- **Admin-configurable addresses**: Additional addresses can be added to or removed from the filter as needed

```
  Filtered Address Categories:
  ────────────────────────────
  ├─ EVM Precompiles (all supported by current L2 EVM fork)
  └─ Admin-configurable addresses
  
  Filter checks both:
  ├─ Transaction signer (from)
  └─ Transaction recipient (to)
```

### 2. Transaction Validation

| Check | Error | Purpose |
|-------|-------|---------|
| `gasLimit < 21000` | `GasLimitTooLow` | Minimum viable gas |
| `gasLimit > MAX_GAS_LIMIT` | `MaxGasLimitExceeded` | Prevent DoS |
| `input.length > MAX_INPUT_LENGTH_LIMIT` | `CalldataInputLengthLimitExceeded` | Prevent DoS |
| `maxPriorityFeePerGas == 0` | `GasFeeParametersContainZero` | Valid EIP-1559 |
| `maxFeePerGas == 0` | `GasFeeParametersContainZero` | Valid EIP-1559 |
| `maxPriorityFeePerGas > maxFeePerGas` | `MaxPriorityFeePerGasHigherThanMaxFee` | Valid EIP-1559 |
| `yParity > 1` | `YParityGreaterThanOne` | Valid signature |
| Duplicate tx in same L1 block | `ForcedTransactionAlreadySubmittedInBlock` | Rate limiting |
| `msg.value != forcedTransactionFee` | `ForcedTransactionFeeNotMet` | Correct fee |
| State hash mismatch | `FinalizationStateIncorrect` | Fresh state |
| Invalid signature recovery | `ECDSAInvalidSignature` (OpenZeppelin) | Valid signature |
| Sender or recipient filtered | `AddressIsFiltered` | Address filtering |

### 3. Role-Based Access Control

```
  Role                              Protected Functions
  ────                              ────────────────────
  DEFAULT_ADMIN_ROLE          ───>  toggleUseAddressFilter (Gateway)
  
  FORCED_TRANSACTION_SENDER_ROLE ─> storeForcedTransaction (Rollup)
                                    (Granted to Gateway contract)
  
  FORCED_TRANSACTION_FEE_SETTER_ROLE
                               ──> setForcedTransactionFee (Rollup)
  
  SET_ADDRESS_FILTER_ROLE ────────> setAddressFilter (Rollup)
```

---

## Appendix: Key Data Structures

### LastFinalizedState

```solidity
struct LastFinalizedState {
    uint256 timestamp;                    // Last finalized L2 block timestamp
    uint256 messageNumber;                // L1→L2 message number at finalization
    bytes32 messageRollingHash;           // Rolling hash of L1→L2 messages
    uint256 forcedTransactionNumber;      // Last finalized forced tx number
    bytes32 forcedTransactionRollingHash; // Rolling hash of forced transactions
}
```

### Eip1559Transaction

```solidity
struct Eip1559Transaction {
    uint256 nonce;
    uint256 maxPriorityFeePerGas;
    uint256 maxFeePerGas;
    uint256 gasLimit;
    address to;
    uint256 value;
    bytes input;
    AccessList[] accessList;
    uint8 yParity;
    uint256 r;
    uint256 s;
}
```

### Finalized State Hash Computation

```solidity
// From FinalizedStateHashing library
function _computeLastFinalizedState(
    uint256 _messageNumber,
    bytes32 _messageRollingHash,
    uint256 _forcedTransactionNumber,
    bytes32 _forcedTransactionRollingHash,
    uint256 _timestamp
) internal pure returns (bytes32) {
    return keccak256(abi.encode(
        _messageNumber,
        _messageRollingHash,
        _forcedTransactionNumber,
        _forcedTransactionRollingHash,
        _timestamp
    ));
}
```

---

## Related Diagrams

All Mermaid diagrams are stored separately for easy rendering:

| Diagram | Path |
|---------|------|
| System Overview | [diagrams/forced-transactions/system-overview.mmd](./diagrams/forced-transactions/system-overview.mmd) |
| Contract Architecture | [diagrams/forced-transactions/contract-architecture.mmd](./diagrams/forced-transactions/contract-architecture.mmd) |
| Submission Flow | [diagrams/forced-transactions/submission-flow.mmd](./diagrams/forced-transactions/submission-flow.mmd) |
| Finalization Flow | [diagrams/forced-transactions/finalization-flow.mmd](./diagrams/forced-transactions/finalization-flow.mmd) |
| Complete End-to-End Flow | [diagrams/forced-transactions/complete-flow.mmd](./diagrams/forced-transactions/complete-flow.mmd) |

To render these diagrams:
- Use [Mermaid Live Editor](https://mermaid.live)
- Install a Mermaid plugin for your IDE/viewer
- Use GitHub's native Mermaid rendering (paste content in a `.md` file with ` ```mermaid ` code blocks)
