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

**Stream 2 — Proving (Execution & Blob).** Two leaf-level proof types are produced independently and in parallel:

- **Execution proofs** — for each contiguous range of L2 blocks (a *conflation*), a prover generates an execution proof attesting to the EVM state transition. Multiple execution proofs can be produced in parallel across different block ranges.
- **Blob proofs** — for one or more EIP-4844 blobs, a single blob proof attests to: (a) correct decompression and KZG polynomial binding for each blob, (b) the chained shnarf transition across the blobs, and (c) recursive verification of the N execution proofs whose ranges tile the combined block range of those blobs. The blob proof is the smallest unit of aggregation: it folds multiple execution proofs into one and exposes the unified 14-field public-input tuple. A single blob proof generalises across `K ≥ 1` blobs.

**Stream 3 — Aggregation.** Once all blob proofs for a target finalisation range are available, they are assembled and wrapped for L1:

1. **Aggregation proof** — a single guest invocation that recursively verifies all `M` blob proofs, asserts inter-blob-proof continuity in software, and outputs the same 14-field tuple over the full range. Flat across the `M` blob proofs (hierarchical / k-ary aggregation is a future option — see §2.5).
2. **Emulation** — the aggregation proof is wrapped in a STARK-to-SNARK step (Groth16/Plonk) and submitted to the L1 contract with the 14 public values, triggering finalisation. Performed inside the same aggregation prover request — there is no separate emulation prover invocation.

```
Exec₁ ┐
Exec₂ ┤
Exec₃ ┤
blob₁ ┼─→ Blob Proof₁ ─┐
blob₂ ┘                │
                       ├─→ Aggregation Proof ─→ Emulation ─→ L1 Finalisation
Exec₄ ┐                │
Exec₅ ┤                │
Exec₆ ┤                │
blob₃ ┼─→ Blob Proof₂ ─┘
```

Each blob proof here covers `K ≥ 1` blobs (`K = 2` in Blob Proof₁, `K = 1` in Blob Proof₂) tiled by `N` execution proofs (`N = 3` in both, illustratively). The aggregation step is flat across the `M` blob proofs; hierarchical (k-ary tree) aggregation is a future option, not part of this iteration — see §2.5.

---

## 2. Proving System

### 2.1 Execution Proof

**WE HAVEN'T CLARIFIED HOW THE GUEST PROGRAM CAN TELL WHICH TRANSACTIONS ARE TO BE REFUSED. PRESUMABLE? JUST ADDING THAT INFORMATION FOR EACH FORCED TRANSACTION IN THE PRIVATE INPUTS OF THE PROGRAM IS LIKELY ENOUGH**

The execution proof covers a contiguous range of L2 blocks and proves the EVM state transition, deposit processing, and withdrawal emission.

**Public Inputs**

| Field | Description |
|---|---|
| `parentBlockHash` | Block hash at the start of this range |
| `endBlockHash` | Block hash at the end of this range |
| `endBlockNumber` | Block number at the end of this range; required for the L1 contract to update `currentL2BlockNumber` and support liveness checks |
| `endStateRootHash` | Explicitly exposed to avoid L1 calldata hashing overhead for withdrawal resolution; the guest enforces it belongs to the header of `endBlockHash` |
| `L2L1MessagesHash` | keccak256 of the ordered list of L2→L1 withdrawal message hashes emitted in this range; the number of messages is bounded per execution proof |
| `parentL1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the start of this range; enables chaining across execution proofs and L1 continuity verification |
| `parentL1L2BridgeRollingHashMessageNumber` | Message number corresponding to `parentL1L2BridgeRollingHash` |
| `endL1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the end of this range |
| `endL1L2BridgeRollingHashMessageNumber` | Message number corresponding to `endL1L2BridgeRollingHash` |
| `dynamicChainConfigHash` | Hash of the L2 `chainID`, `coinBase`, `L2MessageServiceContract`, and `baseFee`. The first three are static configuration; `baseFee` is taken by the guest from any block's header (asserted constant across the range) and folded into the same hash. |
| `parentFtxRollingHash` | Forced-transaction rolling hash at the start of this range |
| `endFtxRollingHash` | Forced-transaction rolling hash at the end of this range |
| `lastProcessedFtxNumber` | Sequence number of the last forced transaction handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose forced transactions were refused in this range; each entry is either the sender (`fromAddress`) if refused due to a sanctioned sender, or the recipient (`toAddress`) if refused due to a sanctioned recipient; `keccak256([])` if none |
| `txFromsHash` | keccak256 of the flat ordered list of sender addresses for all transactions in this range, in block-then-transaction order |

**Private Inputs (Witness)**

- The complete set of L2 blocks in canonical RLP encoding (header + transaction list [+ withdrawals]); EIP-2718 typed transactions in full signed form. The guest recovers each transaction's sender via `secp256k1` and commits to the recovered list via `txFromsHash`.
- The stateless execution witness per block, as produced by Besu's `debug_executionWitness` (`state`, `keys`, `codes`, `headers`); the parent header within `headers` carries the state root that anchors `parentBlockHash`
- The set of L1→L2 deposit messages consumed in this range, with their message numbers and the rolling hash chain anchored at the previous finalized state
- The static chain config: `L2MessageServiceContract`, `coinBase`, `chainID`. `baseFee` is the fourth input to `dynamicChainConfigHash` but is sourced from the block header rather than this struct — see §3.2
- The forced-transaction witnesses for FTXs in the range — see §6

**What it proves:**

* **Validates the EVM state transition**: validating the state-root hash transition.

* **Enforce sequencer consensus rules**: timestamp sequentiality, base-fee, coin-base and , enforces Ethereum consensus rules (fork-choice, timestamps),

* **Extract the canonical L2L1Message** hashes from the block receipt and compute their flat-hash using keccak256.

* **Extract the L1L2RollingHash** by checking a Merkle proof on the old and the new state. Both for the `L1L2RollingHash` and the `L1L2RollingHashNumber`

* **Inspect the forced transactions**: See the corresponding section.

* **Output**: the public-inputs as computed in the above steps.

### 2.2 Blob Proof

The blob proof covers `K ≥ 1` consecutive EIP-4844 blobs and proves that, for each, the content correctly decompresses to the declared block data and satisfies the KZG polynomial binding. It is also the leaf aggregator: it recursively verifies the `N` execution proofs whose ranges tile the combined block range of the `K` blobs and chains them with software `assert_eq!` continuity checks. Its public-input tuple is identical in shape to the aggregation proof's (§2.4), so the upstream aggregation step can consume blob proofs directly.

`K = 1` is the simplest case (one blob per blob proof). `K > 1` lets the coordinator amortise recursion overhead by folding several blobs into a single proof — directly analogous to the existing M-block conflation inside an execution proof.

> *Notation used below:* a subscript `_b` indexes blobs `1..K`; a subscript `_e` indexes execution proofs `1..N`; `m_b` is the block count of blob `b`.

**Public Inputs**

The same 14-field tuple as the aggregation proof (§2.4). `parentShnarf` is the inbound shnarf before blob 1; `endShnarf` is the outbound shnarf after blob K.

The **execution proof's 15-field PI** is *input* to this guest (private witness, §2.2 step 6 recursive verification) — it is **not** the output. The 14 fields below derive from those 15 (across all N execution proofs) plus the blob content and shnarf chain:

| Execution PI (§2.1, 15 fields) | Fate at blob level |
|---|---|
| `parentBlockHash` | **Dropped** — folded into `parentShnarf` via `Hash(parentShnarf, lastBlockHash, blobHash)` (step 3) |
| `endBlockHash` | **Dropped** — folded into `endShnarf` (last block of the last blob) |
| `L2L1MessagesHash` | **Dropped** — per-execution flat hash consumed in step 8 to build `L2L1BridgeTransactionTree`, then discarded |
| `txFromsHash` | **Dropped** — consumed in step 5 to cross-check `froms_e` against blob `blockData.froms`; not propagated |
| `endBlockNumber` | Carried over from `PI_Eₙ` |
| `endStateRootHash` | Carried over from `PI_Eₙ` |
| `parentL1L2BridgeRollingHash` | Carried over from `PI_E₁` |
| `parentL1L2BridgeRollingHashMessageNumber` | Carried over from `PI_E₁` |
| `endL1L2BridgeRollingHash` | Carried over from `PI_Eₙ` |
| `endL1L2BridgeRollingHashMessageNumber` | Carried over from `PI_Eₙ` |
| `dynamicChainConfigHash` | Single shared value (step 9 asserts equality across all N) |
| `parentFtxRollingHash` | Carried over from `PI_E₁` |
| `endFtxRollingHash` | Carried over from `PI_Eₙ` |
| `lastProcessedFtxNumber` | Carried over from `PI_Eₙ` |
| `filteredAddressesHash` | Same name, content rehashed: `keccak256(addrs_E₁ ‖ … ‖ addrs_Eₙ)` (step 10) |

Plus three **new** blob-level fields: `parentShnarf` (input), `endShnarf` (computed in step 3), `L2L1BridgeTransactionTree` (computed in step 8).

**Private Inputs (Witness)**

| Field | Description |
|---|---|
| `blobContent_b` | Raw compressed blob bytes (4096 × 32-byte EIP-4844 payload) for blob `b ∈ [1, K]` |
| `blobHash_b` | The blob's versioned hash as submitted on L1 |
| `KzgProof_b` | KZG proof for blob `b` |
| `KzgY_b` | KZG evaluation claim for blob `b` |
| `blockRange_b` | The `(startBlockNumber, endBlockNumber)` pair for blob `b`'s decompression range |
| `E₁ … Eₙ` | The execution proofs, ordered by block range, tiling the combined range of all K blobs |
| `PI_E₁ … PI_Eₙ` | The public-input tuple for each execution proof |
| `L2L1MsgList_e` | Per-execution-proof L2→L1 message hash list, for `e ∈ [1, N]` |
| `froms_e` | Per-execution-proof sender address list (block-then-transaction order) — preimage of `PI_E_e.txFromsHash` |
| `addrs_e` | Per-execution-proof refused-FTX address list (§6.5) — preimage of `PI_E_e.filteredAddressesHash` |

The decompressed truncated blocks (`blockData_{b,1} … blockData_{b,m_b}`) are **computed** inside the proof by step 2, not provided as a separate witness. The proven statement is decompression: the guest attests that running `decompress_lz4` on `blobContent_b` yields the truncated blocks it then uses downstream.

**Statement (RISC-V Guest)**

For each blob `b ∈ [1, K]` in order, perform the per-blob block (steps 1–3); then perform the cross-blob recursion block (steps 4–10) once over the combined range.

1. **Schwartz–Zippel evaluation (per blob).** Derive evaluation point `X_b` from `blobHash_b` and `blobContent_b`. Check the KZG proof using `blobHash_b`, `X_b` and `KzgProof_b`. In parallel, directly check `P_b(X_b) = KzgY_b` using `blobContent_b`.

2. **Decompress and parse (per blob).** Run `decompress_lz4(blobContent_b)` and parse the result into `m_b` `TruncatedEthereumBlock` entries (`blob.py::TruncatedEthereumBlock`: `{timestamp, blockHash, prevRandao, transactions, froms}`). Assert that `m_b == blockRange_b.endBlockNumber - blockRange_b.startBlockNumber + 1`. These decompressed blocks are used directly by the steps below — there is no separate witnessed `blockData`.

3. **Chain the shnarf (per blob).** Recompute:
   ```
   shnarf_b = Hash(shnarf_{b-1}, lastBlockHash_b, blobHash_b)
   ```
   where `shnarf_0 = parentShnarf` (public input) and `lastBlockHash_b` is the `blockHash` field of the last decompressed `TruncatedEthereumBlock` of blob `b` (from step 2). After all K blobs, assert `shnarf_K == endShnarf`.

4. **Recompute the combined block-hash sequence.** Concatenate the decompressed truncated-block lists across all K blobs in canonical order and walk them, asserting parent-hash continuity at each step (the truncated form does not carry parent pointers directly; alignment is enforced via the execution-proof block-hash chain in step 7 + step 9). The first entry's hash must match the chain that descends from `parentBlockHash`; the final entry's hash must equal `lastBlockHash_K` chained into the shnarf in step 3.

5. **Verify sender addresses.** For each execution proof `Eᵢ`, assert:
   ```
   keccak256(froms_e) == PI_Eᵢ.txFromsHash
   ```
   Then assert that `froms_1 ‖ … ‖ froms_N` equals the concatenation of `froms` across all decompressed truncated blocks (step 2 output), in canonical block-then-transaction order.

6. **Verify the execution proofs.** Recursively verify each `Eᵢ` against `PI_Eᵢ`.

7. **Check execution-proof block-hash alignment.** The `parentBlockHash` of the first execution proof must equal `parentBlockHash` from the public inputs; the `endBlockHash` of the last execution proof must equal `lastBlockHash_K` from step 4; intermediate boundary points must line up with the decompressed block-hash sequence.

8. **Build the L2→L1 Merkle trees.** For each `e ∈ [1, N]`, receive the message hash list as a private witness and assert `keccak256(L2L1MsgList_e) == PI_E_e.L2L1MessagesHash`. Concatenate all N lists in order. Partition the combined list into consecutive chunks of `2^D` leaves (where D is the fixed protocol-level tree depth, currently 5). Pad the final chunk with zero-value (0x00…00) leaves to fill it. Each leaf is a 32-byte message hash; internal nodes are `keccak256(left ‖ right)`. Compute the root of each full tree and collect them into an ordered array `[root_1, …, root_T]`. Output `L2L1BridgeTransactionTree = keccak256(root_1 ‖ … ‖ root_T)` as a commitment to this ordered root list. The tree depth D is a protocol constant and is not included in the public output.

9. **Chain the execution proofs.** For each consecutive pair `(Eᵢ, Eᵢ₊₁)` assert:
   ```
   assert_eq!(PI_Eᵢ.endBlockHash,                       PI_Eᵢ₊₁.parentBlockHash)
   assert_eq!(PI_Eᵢ.endL1L2BridgeRollingHash,                  PI_Eᵢ₊₁.parentL1L2BridgeRollingHash)
   assert_eq!(PI_Eᵢ.endL1L2BridgeRollingHashMessageNumber,     PI_Eᵢ₊₁.parentL1L2BridgeRollingHashMessageNumber)
   assert_eq!(PI_Eᵢ.dynamicChainConfigHash,                 PI_Eᵢ₊₁.dynamicChainConfigHash)
   assert_eq!(PI_Eᵢ.endFtxRollingHash,                      PI_Eᵢ₊₁.parentFtxRollingHash)
   ```
   Continuity *between* blobs is implicit — the same `assert_eq!` block applies at the blob boundary because the execution proofs already tile across it.

10. **Collect forced-transaction outputs and emit PI.** For each `e ∈ [1, N]`, receive `addrs_e` as a private witness and assert `keccak256(addrs_e) == PI_E_e.filteredAddressesHash`. Concatenate all N lists in order and output `filteredAddressesHash = keccak256(addrs_1 ‖ … ‖ addrs_N)`. Take `parentFtxRollingHash` from `PI_E₁` and `endFtxRollingHash` / `lastProcessedFtxNumber` from `PI_Eₙ`. Output the 14-field public-input tuple covering the entire `K`-blob, `N`-execution range.

### 2.3 Aggregation Proof

The aggregation proof recursively verifies the `M` blob proofs covering a finalisation range and outputs a single 14-field public-input tuple over the full range. Topology is **flat**: one guest invocation consumes all `M` blob proofs at once. (Hierarchical / k-ary aggregation is a future option — see §2.5.)

**Public Inputs**

The same 14-field tuple as the blob proof (§2.2) and as the final aggregated PI (§2.4). The blob-proof and aggregation-proof PI shapes match deliberately, so an aggregation proof can also be re-aggregated by a higher-level aggregation proof if hierarchy is added later without changing the PI surface.

**Private Inputs (Witness)**

- The `M` blob proofs `B₁ … Bₘ` (or, in a hierarchical setup, prior aggregation proofs)
- Their complete 14-field public-input tuples `PI_B₁ … PI_Bₘ`
- For each `i`, the ordered L2L1 root array whose committed hash is `PI_Bᵢ.L2L1BridgeTransactionTree`
- For each `i`, the ordered filtered-address list whose committed hash is `PI_Bᵢ.filteredAddressesHash`

**Statement (RISC-V Guest)**

1. **Verify** all `M` inner proofs cryptographically against their claimed public inputs using recursive STARK verification.

2. **Assert continuity** in software, for each consecutive pair `(Bᵢ, Bᵢ₊₁)`:
   ```
   assert_eq!(PI_Bᵢ.endShnarf,                              PI_Bᵢ₊₁.parentShnarf)
   assert_eq!(PI_Bᵢ.endL1L2BridgeRollingHash,                  PI_Bᵢ₊₁.parentL1L2BridgeRollingHash)
   assert_eq!(PI_Bᵢ.endL1L2BridgeRollingHashMessageNumber,     PI_Bᵢ₊₁.parentL1L2BridgeRollingHashMessageNumber)
   assert_eq!(PI_Bᵢ.dynamicChainConfigHash,                 PI_Bᵢ₊₁.dynamicChainConfigHash)
   assert_eq!(PI_Bᵢ.endFtxRollingHash,                      PI_Bᵢ₊₁.parentFtxRollingHash)
   ```
   Block-hash continuity is implicit in the shnarf check: `PI_Bᵢ.endShnarf` encodes blob-proof i's last block hash, so the shnarf assertion subsumes a separate block-hash check.

3. **Merge the L2→L1 root lists.** Receive each blob proof's ordered root array as a private witness and verify it against its committed hash:
   ```
   for i in [1, M]: keccak256(roots_Bᵢ) == PI_Bᵢ.L2L1BridgeTransactionTree
   ```
   Concatenate all `M` arrays in order and output `L2L1BridgeTransactionTree = keccak256(roots_B₁ ‖ … ‖ roots_Bₘ)`.

4. **Merge filtered address lists.** Receive each blob proof's address list, verify it against its committed hash, concatenate all `M` lists in order, and output `filteredAddressesHash = keccak256(addrs_B₁ ‖ … ‖ addrs_Bₘ)`.

5. **Output** the combined public inputs covering the full range: take `parentShnarf`, `parentL1L2BridgeRollingHash`, `parentL1L2BridgeRollingHashMessageNumber`, `parentFtxRollingHash`, and `dynamicChainConfigHash` from `PI_B₁`; take `endBlockNumber`, `endStateRootHash`, `endL1L2BridgeRollingHash`, `endL1L2BridgeRollingHashMessageNumber`, `endFtxRollingHash`, `lastProcessedFtxNumber`, and `endShnarf` from `PI_Bₘ`; use the merged Merkle commitment from step 3 and merged filtered-address hash from step 4.

The aggregation prover request bundles the STARK→SNARK emulation wrap (§1.3 Stream 3 sub-step 2) into the same invocation, so the response is directly L1-submittable — no separate emulation request file.

---

### 2.4 Final Aggregated Public Inputs

The aggregation proof's root exposes fourteen values to the L1 contract:

| # | Field |
|---|---|
| 1 | `endBlockNumber` |
| 2 | `endStateRootHash` |
| 3 | `L2L1BridgeTransactionTree` |
| 4 | `parentL1L2BridgeRollingHash` |
| 5 | `parentL1L2BridgeRollingHashMessageNumber` |
| 6 | `endL1L2BridgeRollingHash` |
| 7 | `endL1L2BridgeRollingHashMessageNumber` |
| 8 | `dynamicChainConfigHash` |
| 9 | `parentFtxRollingHash` |
| 10 | `endFtxRollingHash` |
| 11 | `lastProcessedFtxNumber` |
| 12 | `filteredAddressesHash` |
| 13 | `parentShnarf` |
| 14 | `endShnarf` |

Note: `parentBlockHash` and `endBlockHash` are not separate public inputs — block-hash continuity is enforced through the shnarf chain. The shnarf formula `Hash(parentShnarf, lastBlockHash, blobHash)` binds each blob's last block hash into the shnarf; the L1 contract's shnarf continuity check (`parentShnarf == currentFinalizedShnarf`) is therefore sufficient.

---

## 3. Data Availability

### 3.1 Shnarf Structure

The shnarf is a cumulative on-chain accumulator that links the canonical sequence of L2 block hashes to the EIP-4844 blobs in which their data was published.

```
endShnarf = Hash(parentShnarf, lastBlockHash, blobHash)
```

`lastBlockHash` anchors the shnarf to the execution history; `blobHash` anchors it to the DA blob. Because the KZG polynomial evaluation is proven inside the zkVM, the evaluation point `X` and claim `Y` never appear on-chain — the L1 contract only checks `blobHash` against the transaction's `VERSIONED_HASH`.

### 3.2 Blob Payload

The DA blob must contain the exact inputs required to re-execute the L2 blocks from the previous finalized state. Because the zk-proof guarantees transition validity, any data that is a deterministic *output* of execution can be stripped.

**What is included:**

- **Block context variables** — fields required to resolve EVM opcodes: `timestamp` (for `TIMESTAMP`), `mixHash`/`prevrandao` (for `PREVRANDAO`), `baseFeePerGas` (for `BASEFEE`; the guest takes it from any block's header, asserts the value is constant across the range, and folds it into `dynamicChainConfigHash` as the fourth preimage component). Note: `gasLimit` and fields that are deterministic outputs of execution (such as `withdrawalsRoot`) are not included. `coinbase` (for `COINBASE`) is a static configuration field of the rollup.
- **Target block hash** — `blockHash` is included so that users can infer the return value of the keccak opcode without having access to the entire block header since a Type-1 block hash requires a perfect RLP-encoding of the full header
- **Signature-stripped transactions** — the ordered transaction list including `from` (sender address), `nonce`, gas parameters, `to`, `value`, `data`, and `accessLists`; ECDSA signatures `(v, r, s)` are omitted since the execution proof guarantees they were validly signed. `from` is stored explicitly so the sender can be recovered without signature verification during re-execution. However, the transaction must figure their corresponding blob-hashes and access-lists.

**What is stripped:**

- ECDSA signatures `(v, r, s)`
- Intermediate state roots and receipt roots — these are deterministic outputs of execution, not inputs to it. Note: the current shnarf formula includes `newStateRootHash` as an explicit input; in the new design (§3.1) it is replaced by `lastBlockHash`, which is an execution input rather than an output, so no state root ever appears on-chain.
- ChainID

**Encoding and compression:** The remaining payload is compressed with a standard algorithm (LZ4 or zstd) and packed into the 4096 × 32-byte EIP-4844 blob field. Stripping the above outputs and using an unconstrained compressor significantly increases the effective throughput per blob compared to the current LZSS-based approach.

### 3.3 Prover I/O — On-Wire Format

The JSON files under `prover_inputs/` describe a logical schema. The bytes carried into the zkVM guest are binary.

**Transport.** The guest reads input bytes via the zkVM's read-input primitive (`ziskos::read_input()` on Zisk). Transport framing is an 8-byte little-endian length prefix per chunk, padded to 8-byte alignment.

**Container.** Inside the payload, the execution-proof layout is:

```
[u64 BE: block_rlp_len] [block_rlp_bytes]                — RLP-encoded block
ExecutionWitness:
  state:   [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "state"
  codes:   [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "codes"
  keys:    [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "keys"
  headers: [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "headers"
```

Outer framing is length-prefixed binary; inner payloads are RLP. The per-FTX `signedTxRlp` / `stateWitness` payloads, the `l1L2Messages` array, and the `chainConfig` fields are appended in the same `count + length-prefixed bytes` style. Blob-proof and aggregation-proof containers follow the same convention and are pinned alongside the corresponding guest implementations.

**Debug format.** The `prover_inputs/` JSONs are the canonical schema source; the coordinator translates them to the binary container before invoking the prover. A separate supporting tool can convert JSON fixtures (e.g. `block.json` + `witness.json`) into the same binary container for local replay.

---

## 4. Bridge Mechanics

### 4.1 L1 → L2 (Deposits)

The L1→L2 bridge state is tracked via `endL1L2BridgeRollingHash` and its associated `endL1L2BridgeRollingHashMessageNumber`. The execution proof guest consumes L1 deposit messages (per §2.1) and advances the rolling hash across its range.

Security against L1 re-orgs is not the responsibility of the proof. The Coordinator handles this by waiting for L1 epochs to finalize before anchoring the L1 -> L2 messages — the same model as today.

Across blob and aggregation proofs, continuity is enforced by the aggregation proof's `assert_eq!` checks on rolling hash bounds (§2.3). The `parentL1L2BridgeRollingHash` in the final public inputs allows the L1 contract to verify that the submitted proof continues exactly from the previously finalized bridge state; the new `endL1L2BridgeRollingHash` is cross-checked against L1's authoritative `l1RollingHash[messageNumber]` chain at finalization (§5).

### 4.2 L2 → L1 (Withdrawals)

The L2→L1 bridge state is tracked via `L2L1BridgeTransactionTree`, a commitment to an ordered list of fixed-depth Merkle tree roots. The message commitment is represented differently at each level of the proof tree.

**How it works across proof levels:**

- **Execution proof** — outputs `L2L1MessagesHash`, a flat hash of the bounded ordered list of withdrawal message hashes emitted in its range. The number of messages per execution proof used to be bounded to 16 by design, keeping this commitment cheap but this requirement does not hold anymore.
- **Blob proof** — receives the per-execution message hash lists as private witnesses, verifies each against the corresponding `L2L1MessagesHash`, concatenates them across all `N` execution proofs, partitions the combined list into consecutive chunks of `2^D` leaves (where D is the protocol-level tree depth, currently 5), pads the last chunk with zero-value leaves, computes the root of each full tree, and outputs `L2L1BridgeTransactionTree = keccak256(root₁ ‖ … ‖ rootₖ)`. This is the single point where the flat commitment is expanded into a tree structure.
- **Aggregation proof** — receives the per-blob-proof root arrays as private witnesses, verifies each against the corresponding `L2L1BridgeTransactionTree`, concatenates the arrays in order, and outputs `keccak256(roots_B₁ ‖ … ‖ roots_Bₘ)`.

**On-chain storage and withdrawal claims.** At finalization, the submitter provides the actual root list as calldata alongside the proof. The L1 contract verifies `keccak256(roots) == L2L1BridgeTransactionTree` from the proof's public output, then stores each root via `l2MerkleRootsDepths[root] = D` exactly as today. Users claim withdrawals identically to the current flow: they provide a `merkleRoot`, `leafIndex`, and `proof[]`; the contract looks up the stored depth and verifies the sparse Merkle proof.

**Leaf position derivability from message number.** Withdrawal messages are assigned monotonically increasing message numbers. Because the finalization anchors the message number range via `parentL1L2BridgeRollingHashMessageNumber` and `endL1L2BridgeRollingHashMessageNumber`, the tree index and leaf index of any message are deterministic: for a message at offset `k` from the start of the finalization range, `treeIndex = k / 2^D` and `leafIndex = k mod 2^D`. This means a user who knows their message number can always locate their leaf without any additional on-chain data.

**`l2MessagingBlocksOffsets`.** The current system submits a compact array of uint16 block offsets alongside each finalization call. The L1 decodes these and emits `L2MessagingBlockAnchored` events, which allow off-chain indexers to map L2 blocks to their message slots. This data is **not part of the proof** — it is an unproven discoverability hint provided by the sequencer. Because leaf position is fully derivable from message number (see above), the offset list carries no security weight; a dishonest sequencer can only cause event mis-indexing, not loss of funds. The mechanism is kept unchanged in the new design.

**Comparison with the current approach:** The structure is identical to today — fixed-depth, zero-padded trees, one depth for all, roots stored per finalization in `l2MerkleRootsDepths`. The difference is that tree construction and root commitment now happen inside the RISC-V guest rather than in the bespoke pi-interconnection circuit.

---

## 5. L1 Smart Contract

The new architecture dramatically simplifies the `LineaRollup` contract.

**What the contract does:**

1. **On blob submission:** compute `endShnarf = keccak256(parentShnarf, lastBlockHash, blobHash)` and anchor it in storage.
2. **On finalization:** verify the STARK-to-SNARK proof against the fourteen aggregated public inputs, then:
   - Assert `parentShnarf == currentFinalizedShnarf` (DA and block-hash continuity — the shnarf encodes the last block hash, so this check subsumes a separate `parentBlockHash` check)
   - Assert `parentL1L2BridgeRollingHash == currentFinalizedL1L2BridgeRollingHash` and `parentL1L2BridgeRollingHashMessageNumber == currentFinalizedL1L2BridgeRollingHashMessageNumber` (deposit bridge continuity)
   - Assert `endL1L2BridgeRollingHash == l1RollingHash[endL1L2BridgeRollingHashMessageNumber]` (deposit bridge authenticity — the proof's claimed end-of-range rolling hash must match L1's authoritative chain)
   - Assert `chainID`, `coinbase`, the L2 message-service address, and `baseFee` match the contract's registered chain configuration by checking the dynamic-chain config hash
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

Four fields are added to the execution proof (11 → 15 total; see §2.1):

| Field | Description |
|---|---|
| `parentFtxRollingHash` | FTX rolling hash at the start of this range |
| `endFtxRollingHash` | FTX rolling hash after all FTXs handled in this range |
| `lastProcessedFtxNumber` | Sequence number of the last FTX handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose FTX was refused in this range; each entry is `fromAddress` (refused-from) or `toAddress` (refused-to) depending on which party is sanctioned |

These propagate through the proof tree symmetrically to the L1→L2 bridge fields: the blob proof chains `endFtxRollingHash == parentFtxRollingHash` across consecutive execution proofs (and implicitly across blob boundaries within a multi-blob blob proof); the aggregation proof adds the same assertion across consecutive blob proofs; the final public inputs expose all four (fields 9–12 in §2.4).

### 6.5 Execution Proof Statement

The guest processes FTXs in ascending `ftxNumber` order after completing normal block execution. For each FTX in the range:

**Deadline constraint.** Assert:
```
ftx.deadlineBlockNumber >= prevLastBlockNumber
```
A FTX whose deadline falls before the start of this range was already expired; it must have been handled in a prior range. If it wasn't, finalization of the prior range would have been blocked.

**Authenticity.** Re-derive the rolling hash step and assert it matches the L1-stored value:
```
keccak256(rollingHash ‖ txHash ‖ deadlineBlockNumber ‖ fromAddress) == ftxRollingHash[ftx.number]
```
where `txHash = keccak256(signedTxRlp)` is the standard Ethereum transaction hash of the FTX's signed RLP (as stored on L1 by `storeForcedTransaction`). The guest also asserts `fromAddress == recover_sender(signedTxRlp, chainID)` to bind the witness fields to the canonical signed transaction.

**Outcome:**
- *Included* — the guest asserts `txHash` appears in the declared block's transaction list (decoded from `blockRlp`).
- *Invalid* — pre-validation fails; the guest reads `tx.nonce`, `tx.value`, `tx.gasLimit`, `tx.maxFeePerGas` from `signedTxRlp` and the relevant account state from the EVM state trie (via `stateWitness`, anchored against the parent state root) and asserts the failure condition:
  - Bad nonce: `account.nonce != tx.nonce`
  - Bad balance: `account.balance < tx.gasLimit × tx.maxFeePerGas + tx.value`
  - Similar checks for other pre-validation failures.
  No separate invalidity proof type is needed; this is proven inline.
- *Refused* — the rollup declines for compliance reasons. No governance witness is required inside the proof; the sequencer simply declares the refusal. The L1 contract verifies a posteriori that each refused address appears in its reference sanction list — if any entry is absent, the finalization call reverts. Two refusal modes are supported:
  - *Refused-from*: the sender is sanctioned; `fromAddress` is appended to the filtered address list.
  - *Refused-to*: the recipient is sanctioned; `toAddress` (decoded from `signedTxRlp`; rejected if the FTX is a contract-creation transaction with `to == None`) is appended instead.

After the loop the guest asserts `rollingHash == endFtxRollingHash` and outputs `filteredAddressesHash = keccak256(filtered address list)`.

### 6.6 Propagation Through the Proof Tree

- **Blob Proof:** asserts `PI_Eᵢ.endFtxRollingHash == PI_Eᵢ₊₁.parentFtxRollingHash` across consecutive execution proofs (§2.2 step 9); collects and concatenates filtered address lists into a single `filteredAddressesHash` (§2.2 step 10).
- **Aggregation Proof:** adds `assert_eq!(PI_Bᵢ.endFtxRollingHash, PI_Bᵢ₊₁.parentFtxRollingHash)` to the continuity block (§2.3 step 2); merges filtered address lists across all `M` blob proofs by concatenation and rehashing (§2.3 step 4).
- **Final public inputs:** exposes `parentFtxRollingHash`, `endFtxRollingHash`, `lastProcessedFtxNumber`, `filteredAddressesHash` as fields 9–12 (§2.4).

### 6.7 L1 Contract Changes

**New storage slots:** `currentFinalizedFtxRollingHash`, `currentFinalizedLastProcessedFtxNumber`.

**On finalization, add:**
- Assert `parentFtxRollingHash == currentFinalizedFtxRollingHash` (continuity).
- Assert `endFtxRollingHash == ftxRollingHash[lastProcessedFtxNumber]` (authenticity against L1-stored per-FTX hash).
- Verify `keccak256(submittedFilteredAddresses) == filteredAddressesHash`; for each entry, assert the address is on the sanction list — revert if any is absent — then emit `ForcedTransactionRefused(address)` per entry.
- Deadline check: revert if any FTX K with `ftxDeadline[K] <= endBlockNumber` has K > `lastProcessedFtxNumber`.
- Update `currentFinalizedFtxRollingHash` and `currentFinalizedLastProcessedFtxNumber`.

**Rolling hash migration:** existing FTX submissions used MiMC; at upgrade, any already-submitted FTXs must either be re-hashed or the contract must support both hash functions during a transition window.

---

## 7. What Changes from Today

| Component | Current (Type-2) | New (Type-1 RISC-V) |
|---|---|---|
| **Shnarf formula** | `keccak256(parent, snarkHash, stateRoot, X, Y)` — 5 inputs; `snarkHash` must be computed in-circuit | `keccak256(parent, lastBlockHash, blobHash)` — 3 standard inputs |
| **KZG verification** | L1 contract calls `0x0A` precompile; `X` and `Y` exposed on-chain | Proven inside zkVM guest; `X` and `Y` never appear on-chain |
| **Compression** | Custom SNARK-friendly LZSS; arithmetization-constrained compression ratio | Standard LZ4/zstd compiled into RISC-V guest; unconstrained ratio |
| **Proof interconnection** | Bespoke pi-interconnection circuit in Go/Gnark; gate-level array mapping | Blob proof: recursively verifies N execution proofs across K ≥ 1 blobs and chains them with `assert_eq!` in the RISC-V guest. Aggregation proof: flat recursion over M blob proofs, same continuity assertions across blob-proof boundaries |
| **Execution public inputs** | ~14 Type-2 parameters (timestamps, batch indices, conflation data, dynamic arrays) | +`initial`/`endStateRootHash` - `initialBlockNumber` - `initial/finalTimestamp` - `initialStateRootHash` + "ForcedTxData" |
| **L2→L1 tree construction** | Execution proof outputs flat hash of bounded message list; pi-interconnection organises into fixed-depth Merkle trees | Same flat hash at execution level; blob proof partitions messages into fixed-depth zero-padded trees and outputs `keccak256(roots)`; aggregation proof concatenates the per-blob-proof root arrays and rehashes |
| **l2MessagingBlocksOffsets** | Unproven hint; L1 emits `L2MessagingBlockAnchored` events for off-chain indexing | Unchanged — still an unproven hint; leaf position is fully derivable from message number, so no security impact |
| **DA payload — intermediate roots** | `blockHash`, `timestamp` and transaction RLP without signature + From | adding `prevRandao` |
| **L1 contract** | Complex: precompile calls, dynamic Type-2 input formatting, SNARK-friendly hash routing | Lightweight: verify proof against 14 values + roots/addresses calldata, equality checks against stored state, update storage slots |
| **Final aggregated public inputs** | 13 fields (shnarfs, timestamps, block numbers, rolling hashes ×2, Merkle roots…) | 14 fields — see §2.4 |
| **Blob-proof granularity** | n/a (no blob proof existed; compression was a separate proof per blob) | Configurable: one blob proof can cover `K ≥ 1` blobs (analogous to today's M-block conflation inside an execution proof). `K = 1` is the simplest case; `K > 1` amortises recursion overhead |
