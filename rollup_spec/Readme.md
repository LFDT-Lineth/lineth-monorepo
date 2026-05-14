# Linea Type-1 RISC-V Migration
## The Path to a Type-1 RISC-V Architecture

---

## 1. Introduction

### 1.1 Motivation

Linea currently relies on a multi-layered proving system optimized for a Type-2 zkEVM environment: bespoke arithmetic circuits for EVM execution, a custom SNARK-friendly LZSS compressor, and a dedicated pi-interconnection circuit to wire them together. While functional, this architecture accumulates significant complexity at every layer — from the proving system down to the L1 smart contracts.

The migration to a Type-1 RISC-V zkEVM, targeting the Fusaka/Glamsterdam Ethereum fork, is an opportunity to fundamentally simplify this stack. A general-purpose RISC-V virtual machine can execute standard software directly as a provable program, eliminating the need for most bespoke circuits.

### 1.2 Design Principles

The architecture is driven by four principles:

1. **Move intelligence from circuits to software.** Logic that today lives in custom Gnark/Vortex circuits moves into standard RISC-V guest programs written in Rust or C. This trades circuit complexity for software simplicity.
2. **Use industry-standard primitives.** SNARK-friendly workarounds (MiMC hashing, custom LZSS) are replaced with standard algorithms (Keccak256, LZ4/zstd, BLS12-381) that can be compiled directly into the guest.
3. **Leverage recursive proof composition for continuity.** Instead of a bespoke interconnection circuit that manually checks array mappings at the gate level, adjacent proofs are composed via recursive STARK verification with software `assert_eq!` continuity checks.
4. **Minimise the L1 footprint.** The L1 `LineaRollup` contract should verify as little as possible — a single proof and a small set of public values. All cryptographic complexity belongs inside the proof.

### 1.3 System Overview

The system is organized around three concurrent, independent streams that converge at finalization time.

**Stream 1 — Data Availability.** The sequencer compresses batches of L2 blocks and submits the resulting blob to L1. For each blob, the L1 contract anchors a new shnarf — a cumulative hash that chains the blob's content to the preceding history. The shnarf chain is the canonical on-chain record of submitted DA data.

**Stream 2 — Proving (Execution & Compression).** Two proof types are generated independently and in parallel:

- **Execution proofs** — for each range of L2 blocks, a prover generates an execution proof attesting to the EVM state transition. Multiple execution proofs can be produced in parallel across different block ranges.
- **Compression proofs** — for each submitted blob, a prover generates a compression proof attesting to correct decompression and KZG polynomial evaluation. 
    - Compared to before, the compression proof is also tasked to aggregate its related execution proofs. The compression proof thus serves as the smallest unit of aggregation. It chains the execution proofs, binds the last one to the compression proof, and produces the unified 16-field public-input tuple.

**Stream 3 — Aggregation.** Once all execution proofs and compression proofs for a target finalization range are available, they are assembled into a proof tree:

1. **Join proofs** — applied recursively, each combining two adjacent-range proofs (compression or prior join proofs) by verifying them inside the guest and asserting continuity in software. The aggregation topology is flexible: the coordinator may use a balanced tree, a sequential left-fold, or any other shape, since the only constraint is that A's end block equals B's start block.
2. **Emulation** — the root join proof is wrapped in a STARK-to-SNARK step (Groth16/Plonk) and submitted to the L1 contract with eighteen public values, triggering finalization.

```
Exec₁  ─┐
        ├─ Comp₁ ─┐
Exec₂  ─┘         ├→ Join → ... → Emulation → L1 Finalization
Exec₃  ─┐         │
        ├─ Comp₂ ─┘
Exec₄  ─┘
```

---

## 2. Proving System

### 2.1 Execution Proof

**WE HAVEN'T CLARIFIED HOW THE GUEST PROGRAM CAN TELL WHICH TRANSACTIONS ARE TO BE REFUSED. PRESUMABLE? JUST ADDING THAT INFORMATION FOR EACH FORCED TRANSACTION IN THE PRIVATE INPUTS OF THE PROGRAM IS LIKELY ENOUGH**

The execution proof covers a contiguous range of L2 blocks and proves the EVM state transition, deposit processing, and withdrawal emission.

**Public Inputs**

| Field | Description |
|---|---|
| `prevLastBlockHash` | Block hash at the start of this range |
| `newLastBlockHash` | Block hash at the end of this range |
| `newLastBlockNumber` | Block number at the end of this range; required for the L1 contract to update `currentL2BlockNumber` and support liveness checks |
| `finalStateRootHash` | Explicitly exposed to avoid L1 calldata hashing overhead for withdrawal resolution; the guest enforces it belongs to the header of `newLastBlockHash` |
| `L2L1MessagesHash` | keccak256 of the ordered list of L2→L1 withdrawal message hashes emitted in this range; the number of messages is bounded per execution proof |
| `prevL1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the start of this range; enables chaining across execution proofs and L1 continuity verification |
| `prevL1L2BridgeRollingHashMessageNumber` | Message number corresponding to `prevL1L2BridgeRollingHash` |
| `L1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the end of this range |
| `L1L2BridgeRollingHashMessageNumber` | Message number corresponding to `L1L2BridgeRollingHash` |
| `dynamicChainConfig` | Hash of the L2 `chainID, baseFee, coinBase` and `L2MessageServiceContract` |
| `prevFtxRollingHash` | Forced-transaction rolling hash at the start of this range |
| `newFtxRollingHash` | Forced-transaction rolling hash at the end of this range |
| `lastProcessedFtxNumber` | Sequence number of the last forced transaction handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose forced transactions were refused in this range; each entry is either the sender (`fromAddress`) if refused due to a sanctioned sender, or the recipient (`toAddress`) if refused due to a sanctioned recipient; `keccak256([])` if none |
| `txFromsHash` | keccak256 of the flat ordered list of sender addresses for all transactions in this range, in block-then-transaction order |

**Private Inputs (Witness)**

- The complete set of L2 block headers (all fields required for full RLP encoding: `parentHash`, `stateRoot`, `txsRoot`, `receiptsRoot`, `logsBloom`, `number`, `gasLimit`, `gasUsed`, `timestamp`, `extraData`, `mixHash`/`prevrandao`, `nonce`, `baseFeePerGas`, and any fork-relevant additions from Fusaka/Glamsterdam)
- The signature-stripped ordered transaction list per block
- The set of L1→L2 deposit messages consumed in this range, with their message numbers and the rolling hash chain anchored at the previous finalized state
- The ordered list of L2→L1 withdrawal message hashes emitted in this range (pre-image of `L2L1MessagesHash`)
- The dynamic chain config: `L2MessageServiceContract, coinBase, baseFee, chainID`

**What it proves:** 

* **Validates the EVM state transition**: validating the state-root hash transition.

* **Enforce sequencer consensus rules**: timestamp sequentiality, base-fee, coin-base and , enforces Ethereum consensus rules (fork-choice, timestamps), 

* **Extract the canonical L2L1Message** hashes from the block receipt and compute their flat-hash using keccak256.

* **Extract the L1L2RollingHash** by checking a Merkle proof on the old and the new state. Both for the `L1L2RollingHash` and the `L1L2RollingHashNumber`

* **Inspect the forced transactions**: See the corresponding section.

* **Output**: the public-inputs as computed in the above steps.

### 2.2 Compression Proof

The compression proof covers a single EIP-4844 blob and proves that its content correctly decompresses to the declared block data and satisfies the KZG polynomial binding. It also serves as the aggregating proof for the execution proofs covering the same block range. Importantly, its public-input tuple

**Public Inputs**

*See the Join public-inputs*

**Private Inputs (Witness)**

| Field | Description |
|---|---|
| `blobContent` | The raw compressed blob bytes (4096 × 32-byte EIP-4844 payload) |
| `blobHash` | The blobHash of the blob as submitted on L1 |
| `KzgProof` | The KZG proof for the blob |
| `KzgY` | The KZG proof evaluation claim |
| `blockData₁ … blockDatam` | The block data for all related blocks |
| `E₁ … Eₙ` | The executions proofs, ordered by block range |
| `PI_E₁ … PI_Eₙ` | The public-input tuple for the execution proofs |
| `L2L1MsgList₁ … L2L1MsgListₙ ` | The list of the L2L1 message hashes for each execution proof |
| `froms_E₁ … froms_Eₙ` | The sender address lists for each execution proof, in block-then-transaction order |

**Statement (RISC-V Guest)**

1. **Schwarz-Zipfel evaluation.** Derive evaluation point `X` from `blobHash` and the `blobContent`. Check the KZG proof using `blobHash`, `X` and the `KzgProof`. In parallel, directy check `P(X) = KzgY` using the blobContent data.

2. **Decompress and parse** `blobContent` and assert the result is consistent with `blockData₁ … blockDatam`. E.G. we should find consistent values modulo the stripped down fields (see the data-availability) section.

3. **Verify sender addresses.** For each execution proof `Eᵢ`, assert:
   ```
   keccak256(froms_Eᵢ) == PI_Eᵢ.txFromsHash
   ```
   Then assert that `froms_E₁ ‖ … ‖ froms_Eₙ == blockData₁.froms ‖ … ‖ blockDatam.froms`.

4. **Recompute the last block hashes** using the `prevLastBlockHash` as a basis and check all the blocks are all in sequence and that we obtain the same `lastBlockHash` value as what we have in the public-inputs.

5. **Assert the shnarf.** Using `blobHash` as a public input, assert:
   ```
   Hash(prevShnarf, lastBlockHash, blobHash) == newShnarf
   ```
    Using the Keccak hash function.

6. **Verify the execution proofs** using proof composition with their respective public inputs tuples.

7. **Check the execution proofs blockHashes** all line-up from the list of block hashes computed from **§4**. The `prevLastBlockHash` (and resp. the `lastBlockHash`) of the first (resp. last) execution proof must match with what we found in **§4**.

9. **Build the L2→L1 Merkle trees.** For each `i`, receive the message hash list as a private witness and assert `keccak256(messages_i) == PI_Eᵢ.L2L1MessagesHash`. Concatenate all N lists in order. Partition the combined list into consecutive chunks of `2^D` leaves (where D is the fixed protocol-level tree depth, currently 5). Pad the final chunk with zero-value (0x00…00) leaves to fill it. Each leaf is a 32-byte message hash; internal nodes are `keccak256(left ‖ right)`. Compute the root of each full tree and collect them into an ordered array `[root₁, …, rootₖ]`. Output `L2L1BridgeTransactionTree = keccak256(root₁ ‖ … ‖ rootₖ)` as a commitment to this ordered root list. The tree depth D is a protocol constant and is not included in the public output.

10. **Collect forced-transaction outputs.** Concatenate the filtered address lists from all N execution proofs in order and output `filteredAddressesHash = keccak256(concatenated list)`. Take `prevFtxRollingHash` from `PI_E₁` and `newFtxRollingHash` / `lastProcessedFtxNumber` from `PI_Eₙ`.

11. **Chain execution proofs.** For each consecutive pair assert:
   ```
   assert_eq!(PI_Eᵢ.newLastBlockHash,                       PI_Eᵢ₊₁.prevLastBlockHash)
   assert_eq!(PI_Eᵢ.L1L2BridgeRollingHash,                  PI_Eᵢ₊₁.prevL1L2BridgeRollingHash)
   assert_eq!(PI_Eᵢ.L1L2BridgeRollingHashMessageNumber,     PI_Eᵢ₊₁.prevL1L2BridgeRollingHashMessageNumber)
   assert_eq!(PI_Eᵢ.dynamicChainConfig,                     PI_Eᵢ₊₁.dynamicChainConfig)
   assert_eq!(PI_Eᵢ.newFtxRollingHash,                      PI_Eᵢ₊₁.prevFtxRollingHash)
   ```

12. **Output** the tuple of public inputs.

### 2.3 Join Proof (Recursive Aggregation)

A Join proof C combines two adjacent-range proofs A and B, where A covers blocks `[m, k]` and B covers blocks `[k+1, n]`.

**Private Inputs (Witness)**

- The full proofs A and B
- Their complete public input tuples `PI_A` and `PI_B`
- The ordered leaf sets (message hash lists) whose Merkle roots are `PI_A.L2L1BridgeTransactionTree` and `PI_B.L2L1BridgeTransactionTree` respectively

**Statement (RISC-V Guest)**

1. **Verify** both inner proofs cryptographically against their claimed public inputs using recursive STARK verification.

2. **Assert continuity** in software:
   ```
   assert_eq!(PI_A.newShnarf,                              PI_B.prevShnarf)
   assert_eq!(PI_A.L1L2BridgeRollingHash,                  PI_B.prevL1L2BridgeRollingHash)
   assert_eq!(PI_A.L1L2BridgeRollingHashMessageNumber,     PI_B.prevL1L2BridgeRollingHashMessageNumber)
   assert_eq!(PI_A.chainID,                                PI_B.chainID)
   assert_eq!(PI_A.coinbase,                               PI_B.coinbase)
   assert_eq!(PI_A.baseFee,                                PI_B.baseFee)
   assert_eq!(PI_A.newFtxRollingHash,                      PI_B.prevFtxRollingHash)
   ```
   Block-hash continuity is implicit in the shnarf check: `PI_A.newShnarf` encodes A's last block hash, and asserting `PI_A.newShnarf == PI_B.prevShnarf` is sufficient.

3. **Merge the L2→L1 root lists.** Receive the ordered root arrays `roots_A` and `roots_B` as private witnesses. Verify each against its committed hash:
   ```
   keccak256(roots_A) == PI_A.L2L1BridgeTransactionTree
   keccak256(roots_B) == PI_B.L2L1BridgeTransactionTree
   ```
   Concatenate the two arrays in order and output `L2L1BridgeTransactionTree = keccak256(roots_A ‖ roots_B)`.

4. **Merge filtered address lists.** Receive the two address lists as private witnesses, verify each against its committed hash, concatenate in order, and output `filteredAddressesHash = keccak256(addrs_A ‖ addrs_B)`.

5. **Output** the combined public inputs covering the full range `[m, n]`: take `prevL1L2BridgeRollingHash`, `prevL1L2BridgeRollingHashMessageNumber`, `prevFtxRollingHash`, `prevShnarf`, `chainID`, `coinbase`, and `baseFee` from `PI_A`; take `newLastBlockNumber`, `finalStateRootHash`, `L1L2BridgeRollingHash`, `L1L2BridgeRollingHashMessageNumber`, `newFtxRollingHash`, `lastProcessedFtxNumber`, and `newShnarf` from `PI_B`; use the merged Merkle commitment from step 3 and merged filtered addresses from step 4.

This join is applied recursively — in any chosen topology — until a single root proof covers the entire finalization range, which is then wrapped in a STARK-to-SNARK step for L1 verification.

---

### 2.4 Final Aggregated Public Inputs

After all joins, the root proof exposes fourteen values to the L1 contract:

| # | Field |
|---|---|
| 1 | `newLastBlockNumber` |
| 2 | `finalStateRootHash` |
| 3 | `L2L1BridgeTransactionTree` |
| 4 | `prevL1L2BridgeRollingHash` |
| 5 | `prevL1L2BridgeRollingHashMessageNumber` |
| 6 | `L1L2BridgeRollingHash` |
| 7 | `L1L2BridgeRollingHashMessageNumber` |
| 8 | `dynamicChainConfigHash` |
| 9 | `prevFtxRollingHash` |
| 10 | `newFtxRollingHash` |
| 11 | `lastProcessedFtxNumber` |
| 12 | `filteredAddressesHash` |
| 13 | `prevShnarf` |
| 14 | `newShnarf` |

Note: `prevLastBlockHash` and `newLastBlockHash` are not separate public inputs — block-hash continuity is enforced through the shnarf chain. The shnarf formula `Hash(prevShnarf, lastBlockHash, blobHash)` binds each blob's last block hash into the shnarf; the L1 contract's shnarf continuity check (`prevShnarf == currentFinalizedShnarf`) is therefore sufficient.

---

## 3. Data Availability

### 3.1 Shnarf Structure

The shnarf is a cumulative on-chain accumulator that links the canonical sequence of L2 block hashes to the EIP-4844 blobs in which their data was published.

```
newShnarf = Hash(prevShnarf, lastBlockHash, blobHash)
```

`lastBlockHash` anchors the shnarf to the execution history; `blobHash` anchors it to the DA blob. Because the KZG polynomial evaluation is proven inside the zkVM, the evaluation point `X` and claim `Y` never appear on-chain — the L1 contract only checks `blobHash` against the transaction's `VERSIONED_HASH`.

### 3.2 Blob Payload

The DA blob must contain the exact inputs required to re-execute the L2 blocks from the previous finalized state. Because the zk-proof guarantees transition validity, any data that is a deterministic *output* of execution can be stripped.

**What is included:**

- **Block context variables** — fields required to resolve EVM opcodes: `timestamp` (for `TIMESTAMP`), `mixHash`/`prevrandao` (for `PREVRANDAO`). Note: `gasLimit` and fields that are deterministic outputs of execution (such as `withdrawalsRoot`) are not included. `coinbase` (for `COINBASE`), `baseFee` are static configuration fields of the rollup.
- **Target block hash** — `blockHash` is included so that users can infer the return value of the keccak opcode without having access to the entire block header since a Type-1 block hash requires a perfect RLP-encoding of the full header
- **Signature-stripped transactions** — the ordered transaction list including `from` (sender address), `nonce`, gas parameters, `to`, `value`, `data`, and `accessLists`; ECDSA signatures `(v, r, s)` are omitted since the execution proof guarantees they were validly signed. `from` is stored explicitly so the sender can be recovered without signature verification during re-execution. However, the transaction must figure their corresponding blob-hashes and access-lists.

**What is stripped:**

- ECDSA signatures `(v, r, s)`
- Intermediate state roots and receipt roots — these are deterministic outputs of execution, not inputs to it. Note: the current shnarf formula includes `newStateRootHash` as an explicit input; in the new design (§3.1) it is replaced by `lastBlockHash`, which is an execution input rather than an output, so no state root ever appears on-chain.
- ChainID

**Encoding and compression:** The remaining payload is compressed with a standard algorithm (LZ4 or zstd) and packed into the 4096 × 32-byte EIP-4844 blob field. Stripping the above outputs and using an unconstrained compressor significantly increases the effective throughput per blob compared to the current LZSS-based approach.

---

## 4. Bridge Mechanics

### 4.1 L1 → L2 (Deposits)

The L1→L2 bridge state is tracked via `L1L2BridgeRollingHash` and its associated `L1L2BridgeRollingHashMessageNumber`. The execution proof guest consumes L1 deposit messages and advances the rolling hash across its range.

Security against L1 re-orgs is not the responsibility of the proof. The bridge contract handles this by waiting for L1 epochs to finalize before making deposit messages available to the sequencer — the same model as today.

Across joined proofs, continuity is enforced by the Join proof's `assert_eq!` checks on rolling hash bounds (§2.4). The `prevL1L2BridgeRollingHash` in the final public inputs allows the L1 contract to verify that the submitted proof continues exactly from the previously finalized bridge state, closing the continuity chain at the contract level.

### 4.2 L2 → L1 (Withdrawals)

The L2→L1 bridge state is tracked via `L2L1BridgeTransactionTree`, a commitment to an ordered list of fixed-depth Merkle tree roots. The message commitment is represented differently at each level of the proof tree.

**How it works across proof levels:**

- **Execution proof** — outputs `L2L1MessagesHash`, a flat hash of the bounded ordered list of withdrawal message hashes emitted in its range. The number of messages per execution proof used to be bounded to 16 by design, keeping this commitment cheap but this requirement does not hold anymore.
- **Compression proof** — receives the message lists as a private witness, verifies it against `L2L1MessagesHash`, partitions the combined message list into consecutive chunks of `2^D` leaves (where D is the protocol-level tree depth, currently 5), pads the last chunk with zero-value leaves, computes the root of each full tree, and outputs `L2L1BridgeTransactionTree = keccak256(root₁ ‖ … ‖ rootₖ)`. This is the single point where the flat commitment is expanded into a tree structure.
- **Join proof** — receives the two flat message arrays as private witnesses and the roots as public witness, verifies each array against its root, concatenates the arrays, and reconstructs the Merkle tree root-hash list before returning `keccak256(roots_A ‖ roots_B)`.

**On-chain storage and withdrawal claims.** At finalization, the submitter provides the actual root list as calldata alongside the proof. The L1 contract verifies `keccak256(roots) == L2L1BridgeTransactionTree` from the proof's public output, then stores each root via `l2MerkleRootsDepths[root] = D` exactly as today. Users claim withdrawals identically to the current flow: they provide a `merkleRoot`, `leafIndex`, and `proof[]`; the contract looks up the stored depth and verifies the sparse Merkle proof.

**Leaf position derivability from message number.** Withdrawal messages are assigned monotonically increasing message numbers. Because the finalization anchors the message number range via `prevL1L2BridgeRollingHashMessageNumber` and `L1L2BridgeRollingHashMessageNumber`, the tree index and leaf index of any message are deterministic: for a message at offset `k` from the start of the finalization range, `treeIndex = k / 2^D` and `leafIndex = k mod 2^D`. This means a user who knows their message number can always locate their leaf without any additional on-chain data.

**`l2MessagingBlocksOffsets`.** The current system submits a compact array of uint16 block offsets alongside each finalization call. The L1 decodes these and emits `L2MessagingBlockAnchored` events, which allow off-chain indexers to map L2 blocks to their message slots. This data is **not part of the proof** — it is an unproven discoverability hint provided by the sequencer. Because leaf position is fully derivable from message number (see above), the offset list carries no security weight; a dishonest sequencer can only cause event mis-indexing, not loss of funds. The mechanism is kept unchanged in the new design.

**Comparison with the current approach:** The structure is identical to today — fixed-depth, zero-padded trees, one depth for all, roots stored per finalization in `l2MerkleRootsDepths`. The difference is that tree construction and root commitment now happen inside the RISC-V guest rather than in the bespoke pi-interconnection circuit.

---

## 5. L1 Smart Contract

The new architecture dramatically simplifies the `LineaRollup` contract.

**What the contract does:**

1. **On blob submission:** compute `newShnarf = keccak256(prevShnarf, lastBlockHash, blobHash)` and anchor it in storage.
2. **On finalization:** verify the STARK-to-SNARK proof against the fourteen aggregated public inputs, then:
   - Assert `prevShnarf == currentFinalizedShnarf` (DA and block-hash continuity — the shnarf encodes the last block hash, so this check subsumes a separate `prevLastBlockHash` check)
   - Assert `prevL1L2BridgeRollingHash == currentFinalizedL1L2BridgeRollingHash` and `prevL1L2BridgeRollingHashMessageNumber == currentFinalizedL1L2BridgeRollingHashMessageNumber` (deposit bridge continuity)
   - Assert `chainID`, `coinbase`, and `baseFee` match the contract's registered chain configuration by checking the dynamic-chain config hash
   - Verify `keccak256(submittedRoots) == L2L1BridgeTransactionTree`; store each root via `l2MerkleRootsDepths[root] = D`
   - Optionally process `l2MessagingBlocksOffsets` calldata to emit `L2MessagingBlockAnchored` discovery events (unchanged from today)
   - Update storage: `currentFinalizedLastBlockHash`, `currentFinalizedShnarf`, `currentL2BlockNumber`, `currentFinalizedL1L2BridgeRollingHash`, `currentFinalizedL1L2BridgeRollingHashMessageNumber`

**What is removed:**

- The call to the `0x0A` point evaluation precompile — the KZG binding is now guaranteed by the proof
- All Type-2 conflation metadata processing (timestamps, batch indices, dynamic array unpacking)
- SNARK-friendly hash routing

The result is a contract that takes fourteen standard `bytes32`/`uint256` values plus a roots array, runs a small set of equality checks against stored state, updates storage slots, and delegates to a generated verifier. Hundreds of lines of bespoke parsing logic are permanently deleted.

---

## 6. Escape Hatch (Forced Transaction Inclusion)

### 6.1 Overview

The escape hatch lets a user submit a transaction directly to the L1 contract if they fear the L2 sequencer is censoring them. Once accepted, a deadline block is set. The rollup cannot finalize past that deadline unless it proves the forced transaction (FTX) has been handled — either executed (successfully or with a pre-validation failure) or explicitly refused for compliance reasons.

### 6.2 On-Chain Submission (unchanged)

The user calls `storeForcedTransaction(rlpEncodedSignedTx)`, paying a fee. The L1 assigns a monotonically increasing `forcedTransactionNumber`, records `deadlineBlockNumber`, and updates its running keccak256 FTX rolling hash (replacing MiMC — see §7.3). The hash is computed over all submissions in order; the per-FTX value is stored and used by the proof to assert authenticity.

### 6.3 Rolling Hash

MiMC is replaced with keccak256, consistent with the rest of the new design. The formula matches the fields used in the current circuit (transaction hash, deadline, sender), all committed per FTX:

```
ftxRollingHash_n = keccak256(ftxRollingHash_{n-1} ‖ txHash ‖ deadlineBlockNumber ‖ fromAddress)
```

where `txHash = keccak256(ftxRlp)` is the standard Ethereum transaction hash.

### 6.4 New Execution Proof Public Inputs

Four fields are added to the execution proof (12 → 16 total; see §2.1):

| Field | Description |
|---|---|
| `prevFtxRollingHash` | FTX rolling hash at the start of this range |
| `newFtxRollingHash` | FTX rolling hash after all FTXs handled in this range |
| `lastProcessedFtxNumber` | Sequence number of the last FTX handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose FTX was refused in this range; each entry is `fromAddress` (refused-from) or `toAddress` (refused-to) depending on which party is sanctioned |

These propagate through the proof tree symmetrically to the L1→L2 bridge fields: the compression chains `newFtxRollingHash == prevFtxRollingHash` across consecutive execution proofs; the join proof adds the same assertion across compressions/joins; the final public inputs expose all four (fields 13–16 in §2.5).

### 6.5 Execution Proof Statement

The guest processes FTXs in ascending `ftxNumber` order after completing normal block execution. For each FTX in the range:

**Deadline constraint.** Assert:
```
ftx.deadlineBlockNumber >= prevLastBlockNumber
```
A FTX whose deadline falls before the start of this range was already expired; it must have been handled in a prior range. If it wasn't, finalization of the prior range would have been blocked.

**Authenticity.** Re-derive the rolling hash step and assert it matches the L1-stored value:
```
keccak256(rollingHash, txHash, ftx.deadlineBlockNumber, ftx.fromAddress) == ftxRollingHash[ftx.number]
```

**Outcome:**
- *Included* — the guest asserts the transaction hash appears in the declared block's transaction list (directly observable from the execution witness).
- *Invalid* — pre-validation fails; the guest reads the relevant account state from the EVM state trie (already in the witness) and asserts the failure condition:
  - Bad nonce: `account.nonce != tx.nonce`
  - Bad balance: `account.balance < tx.gasLimit × tx.maxFeePerGas + tx.value`
  - Similar checks for other pre-validation failures.
  No separate invalidity proof type is needed; this is proven inline.
- *Refused* — the rollup declines for compliance reasons. No governance witness is required inside the proof; the sequencer simply declares the refusal. The L1 contract verifies a posteriori that each refused address appears in its reference sanction list — if any entry is absent, the finalization call reverts. Two refusal modes are supported:
  - *Refused-from*: the sender is sanctioned; `fromAddress` is appended to the filtered address list.
  - *Refused-to*: the recipient is sanctioned; `toAddress` is appended to the filtered address list instead.

After the loop the guest asserts `rollingHash == newFtxRollingHash` and outputs `filteredAddressesHash = keccak256(filtered address list)`.

**Deadline sweep.** After the loop, assert that every FTX with `deadlineBlockNumber <= newLastBlockNumber` has been covered:
```
for each pending FTX K where ftxDeadline[K] <= newLastBlockNumber:
    assert!(K <= lastProcessedFtxNumber)
```

### 6.6 Propagation Through the Proof Tree

- **Compression:** asserts `PI_Eᵢ.newFtxRollingHash == PI_Eᵢ₊₁.prevFtxRollingHash` across consecutive execution proofs (step 2); collects and concatenates filtered address lists into a single `filteredAddressesHash` (step 5).
- **Join Proof:** adds `assert_eq!(PI_A.newFtxRollingHash, PI_B.prevFtxRollingHash)` to the continuity block (step 2); merges filtered address lists by concatenation and rehashing (step 4).
- **Final public inputs:** exposes `prevFtxRollingHash`, `newFtxRollingHash`, `lastProcessedFtxNumber`, `filteredAddressesHash` as fields 13–16.

### 6.7 L1 Contract Changes

**New storage slots:** `currentFinalizedFtxRollingHash`, `currentFinalizedLastProcessedFtxNumber`.

**On finalization, add:**
- Assert `prevFtxRollingHash == currentFinalizedFtxRollingHash` (continuity).
- Assert `newFtxRollingHash == ftxRollingHash[lastProcessedFtxNumber]` (authenticity against L1-stored per-FTX hash).
- Verify `keccak256(submittedFilteredAddresses) == filteredAddressesHash`; for each entry, assert the address is on the sanction list — revert if any is absent — then emit `ForcedTransactionRefused(address)` per entry.
- Deadline check: revert if any FTX K with `ftxDeadline[K] <= newLastBlockNumber` has K > `lastProcessedFtxNumber`.
- Update `currentFinalizedFtxRollingHash` and `currentFinalizedLastProcessedFtxNumber`.

**Rolling hash migration:** existing FTX submissions used MiMC; at upgrade, any already-submitted FTXs must either be re-hashed or the contract must support both hash functions during a transition window.

---

## 7. What Changes from Today

| Component | Current (Type-2) | New (Type-1 RISC-V) |
|---|---|---|
| **Shnarf formula** | `keccak256(parent, snarkHash, stateRoot, X, Y)` — 5 inputs; `snarkHash` must be computed in-circuit | `keccak256(parent, lastBlockHash, blobHash)` — 3 standard inputs |
| **KZG verification** | L1 contract calls `0x0A` precompile; `X` and `Y` exposed on-chain | Proven inside zkVM guest; `X` and `Y` never appear on-chain |
| **Compression** | Custom SNARK-friendly LZSS; arithmetization-constrained compression ratio | Standard LZ4/zstd compiled into RISC-V guest; unconstrained ratio |
| **Proof interconnection** | Bespoke pi-interconnection circuit in Go/Gnark; gate-level array mapping | Compression: chains N execution proofs and binds the last one to the compression proof with `assert_eq!` in the RISC-V guest |
| **Execution public inputs** | ~14 Type-2 parameters (timestamps, batch indices, conflation data, dynamic arrays) | +`initial`/`finalStateRootHash` - `initialBlockNumber` - `initial/finalTimestamp` - `initialStateRootHash` + "ForcedTxData" |
| **L2→L1 tree construction** | Execution proof outputs flat hash of bounded message list; pi-interconnection organises into fixed-depth Merkle trees | Same flat hash at execution level; Compression proof partitions messages into fixed-depth zero-padded trees and outputs `keccak256(roots)`; Join proof concatenates root arrays and rehashes |
| **l2MessagingBlocksOffsets** | Unproven hint; L1 emits `L2MessagingBlockAnchored` events for off-chain indexing | Unchanged — still an unproven hint; leaf position is fully derivable from message number, so no security impact |
| **DA payload — intermediate roots** | `blockHash`, `timestamp` and transaction RLP without signature + From | adding `prevRandao` |
| **L1 contract** | Complex: precompile calls, dynamic Type-2 input formatting, SNARK-friendly hash routing | Lightweight: verify proof against 18 values + roots/addresses calldata, equality checks against stored state, update storage slots |
| **Final aggregated public inputs** | 13 fields (shnarfs, timestamps, block numbers, rolling hashes ×2, Merkle roots…) | See corresponding table |
