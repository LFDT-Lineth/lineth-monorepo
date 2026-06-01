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
4. **Minimize the L1 footprint.** The L1 `LineaRollup` contract should verify as little as possible — a single proof and a small set of public values. All cryptographic complexity belongs inside the proof.

### 1.3 System Overview

The system is organized around three concurrent, independent streams that converge at finalization time.

**Stream 1 — Data Availability.** The sequencer compresses batches of L2 blocks and submits the resulting blob to L1. For each blob, the L1 contract anchors a new shnarf — a cumulative hash that chains the blob's content to the preceding history. The shnarf chain is the canonical on-chain record of submitted DA data.

**Stream 2 — Proving (l2-execution & rollup).** Two leaf-level proof types are produced independently and in parallel:

- **l2-execution proofs** — for each contiguous range of L2 blocks (a *conflation*), a prover generates an l2-execution proof attesting to the EVM state transition, L2→L1 message rolling hash checks, and forced-transaction handling. Multiple l2-execution proofs can be produced in parallel across different block ranges.
- **rollup proofs** — for one or more EIP-4844 blobs, a single rollup proof attests to: (a) for each blob, the guest computes the canonical compressed payload from the witnessed full block RLPs (truncate → RLP-encode → LZ4-compress → zero-pad to 131 072 bytes), computes the KZG commitment from those bytes, checks its versioned hash against the L1-committed `blobHash`, and verifies the KZG proof; (b) the chained shnarf transition across the blobs; and (c) recursive verification of the N l2-execution proofs whose ranges tile the combined block range of those blobs. The rollup proof is the smallest unit of aggregation: it folds multiple l2-execution proofs into one and exposes the unified 14-field public-input tuple. A single rollup proof generalizes across `K ≥ 1` blobs.

**Stream 3 — rollup-aggregation.** Once all rollup proofs for a target finalization range are available, they are assembled, aggregated, and wrapped for L1 by one rollup-aggregation prover request:

- **rollup-aggregation + emulation** — a single rollup-aggregation prover request runs one guest invocation that recursively verifies all `M` rollup proofs, asserts inter-rollup-proof continuity in software, outputs the same 14-field tuple over the full range, and then performs the STARK-to-SNARK emulation wrap (Groth16/Plonk) for L1 submission. The rollup-aggregation topology is flat across the `M` rollup proofs; hierarchical / k-ary aggregation is a future option. There is no separate emulation prover invocation.

```
l2-exec₁ ┐
l2-exec₂ ┤
l2-exec₃ ┤
blob₁    ┼─→ Rollup Proof₁ ─┐
blob₂    ┘                  │
                            ├─→ rollup-aggregation Proof + Emulation ─→ L1 Finalization
l2-exec₄ ┐                  │
l2-exec₅ ┤                  │
l2-exec₆ ┤                  │
blob₃    ┼─→ Rollup Proof₂ ─┘
```

Each rollup proof here covers `K ≥ 1` blobs (`K = 2` in Rollup Proof₁, `K = 1` in Rollup Proof₂) tiled by `N` l2-execution proofs (`N = 3` in both, illustratively). The rollup-aggregation step is flat across the `M` rollup proofs; hierarchical (k-ary tree) aggregation is a future option, not part of this iteration.

---

## 2. Proving System

The Python reference mirrors the three guest programs, with the L1 contract
checks modeled separately in `l1_rollup.py`:

| Guest program | Reference entry point | Scope |
|---|---|---|
| l2-execution | `l2_execution.py::run_l2_execution_guest` | Replays a contiguous L2 block range from canonical `blockRlp` plus `debug_executionWitness`, validates the EVM state transition, extracts bridge events, processes forced transactions, and emits the 15-field l2-execution PI. It does not read blobs, verify KZG, or recursively verify other proofs. |
| rollup | `rollup.py::run_rollup_guest` | For each of `K >= 1` consecutive blobs, recomputes the canonical compressed payload from the witnessed full block RLPs (truncate → RLP-encode → LZ4-compress → zero-pad to `BLOB_BYTES_LENGTH`), computes the KZG commitment from those bytes, checks its versioned hash against the L1-committed `blobHash`, and verifies the KZG proof. Chains the shnarf transition, recursively verifies the `N` l2-execution proofs that tile the blob range, builds L2->L1 root commitments, merges refused-address outputs, and emits the 14-field rollup PI. It does not run the EVM or perform L1 finalization checks. |
| rollup-aggregation | `rollup_aggregation.py::run_rollup_aggregation_guest` | Recursively verifies the `M` rollup proofs for a finalization range, checks proof-to-proof continuity, merges root/address commitments, and emits the final 14-field PI consumed by L1. It does not inspect raw blocks, raw blobs, or L1 storage. |

`l1_rollup.py` models the contract-facing blob anchoring and finalization checks
against L1 storage. It is intentionally not one of the RISC-V guest programs.

### 2.1 l2-execution Proof
The l2-execution proof covers a contiguous range of L2 blocks and proves the EVM state transition, deposit processing, and withdrawal emission.
The sequencer's forced-transaction handling decision is supplied per forced
transaction as `acceptance` in the private witness; the guest proves that the
declared outcome is one of the allowed outcomes in §6.5.

**Public Inputs**

| Field | Description |
|---|---|
| `parentBlockHash` | Block hash at the start of this range |
| `endBlockHash` | Block hash at the end of this range |
| `endBlockNumber` | Block number at the end of this range; required for the L1 contract to update `currentL2BlockNumber` and support liveness checks |
| `endBlockTimestamp` | UNIX timestamp of the last block in this range. Bubbled up unchanged through the rollup and rollup-aggregation proofs (§2.2, §2.3) and stored on L1 as `currentL2BlockTimestamp` at finalization, so L1 consumers can read the finalized L2 "wall clock". |
| `L2L1MessagesHash` | keccak256 of the ordered list of L2→L1 withdrawal message hashes emitted in this range; the number of messages is bounded per l2-execution proof |
| `parentL1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the start of this range; enables chaining across l2-execution proofs and L1 continuity verification |
| `parentL1L2BridgeRollingHashMessageNumber` | Message number corresponding to `parentL1L2BridgeRollingHash` |
| `endL1L2BridgeRollingHash` | Accumulated L1→L2 deposit rolling hash at the end of this range |
| `endL1L2BridgeRollingHashMessageNumber` | Message number corresponding to `endL1L2BridgeRollingHash` |
| `dynamicChainConfigHash` | `keccak256(uint256_be(chainID) ‖ coinBase ‖ L2MessageServiceContract ‖ uint256_be(baseFee))`, where integer fields are 32-byte big-endian values and addresses are canonical 20-byte values. `baseFee` is part of the dynamic chain configuration; a base-fee update is therefore a configuration update and a proof-range boundary. |
| `parentFtxRollingHash` | Forced-transaction rolling hash at the start of this range |
| `endFtxRollingHash` | Forced-transaction rolling hash at the end of this range |
| `lastProcessedFtxNumber` | Sequence number of the last forced transaction handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose forced transactions were refused in this range; each entry is either the recovered sender (`fromAddress = recover_sender(signedTxRlp, chainID)`) if refused due to a sanctioned sender, or the recipient (`toAddress`) if refused due to a sanctioned recipient; `keccak256([])` if none |
| `txFromsHash` | keccak256 of the flat ordered list of sender addresses for all transactions in this range, in block-then-transaction order |

**Private Inputs (Witness)**

- The complete set of L2 blocks in canonical RLP encoding (header + transaction list [+ withdrawals]); EIP-2718 typed transactions in full signed form. The guest recovers each transaction's sender via `secp256k1` and commits to the recovered list via `txFromsHash`.
- The stateless execution witness per block, as produced by Besu's `debug_executionWitness` (`state`, `keys`, `codes`, `headers`); the parent header within `headers` carries the state root that anchors `parentBlockHash`. The `state` MPT node pool must additionally include proof paths for: (a) the L2MessageService's `L1L2RollingHash` and `L1L2RollingHashMessageNumber` slots at both the parent and end state roots — read at proof-range boundaries even when no block in the range writes them; (b) the sender account of any FTX whose declared outcome is *Invalid* (§6.5), at the parent state root of the block where that FTX would have been included.
- The static chain config: `L2MessageServiceContract`, `coinBase`, `chainID`. `baseFee` — the fourth input to `dynamicChainConfigHash` — is NOT part of this struct; the guest reads it from the first block's header and asserts every subsequent block in the range carries the same value.
- The forced-transaction witnesses for FTXs in the range — see §6

**What it proves:**

* **Validates the EVM state transition**: validating the state-root hash transition.

* **Enforce sequencer consensus rules**: strictly monotonic timestamps across the range, constant `baseFee` (sourced from the first block header and asserted equal in every subsequent header), `coinbase` matching the chain configuration, and a contiguous parent-hash chain anchored at `parentBlockHash`. Standard Ethereum header validation (`validate_header`) is delegated to the state-transition primitive.

* **Extract the canonical L2L1Message** hashes from the block receipt and compute their flat-hash using keccak256.

* **Read the L1L2RollingHash boundary values.** The L2MessageService contract maintains `L1L2RollingHash` and `L1L2RollingHashMessageNumber` in its storage; they are updated as a side effect of normal EVM execution whenever an L1→L2 message is anchored. The guest reads the four boundary values (`parent`/`end` × `RollingHash`/`MessageNumber`) directly from L2 state at `parentHeader.state_root` and `endBlock.state_root` via the EVM state interface — no separate L1→L2 message witness is consumed. The L1 contract cross-checks the resulting `endL1L2BridgeRollingHash` against its authoritative `l1RollingHash[messageNumber]` chain at finalization (§5).

* **Inspect the forced transactions**: See the corresponding section.

* **Output**: the public-inputs as computed in the above steps.

### 2.2 rollup Proof

The rollup proof covers `K ≥ 1` consecutive EIP-4844 blobs and proves that, for each, the canonical compressed payload recomputed from the witnessed full block RLPs (`lz4_compress(rlp_encode(truncate(blockRlps)))`, zero-padded to the EIP-4844 blob size) is what the sequencer committed to on L1. The guest computes the KZG commitment from those padded bytes, checks `kzg_commitment_to_versioned_hash(computedCommitment) == blobHash`, and verifies `blobKzgProof` against the computed commitment. The commitment is not a witness field. The rollup proof is also the leaf aggregator: it recursively verifies the `N` l2-execution proofs whose ranges tile the combined block range of the `K` blobs and chains them with software `assert_eq!` continuity checks. Its public-input tuple is identical in shape to the rollup-aggregation proof's (§2.4), so the upstream rollup-aggregation step can consume rollup proofs directly.

`K = 1` is the simplest case (one blob per rollup proof). `K > 1` lets the coordinator amortize recursion overhead by folding several blobs into a single proof — directly analogous to the existing M-block conflation inside an l2-execution proof.

> *Notation used below:* a subscript `_b` indexes blobs `1..K`; a subscript `_e` indexes l2-execution proofs `1..N`; `m_b` is the block count of blob `b`.

**Public Inputs**

The same 14-field tuple as the rollup-aggregation proof (§2.4). `parentShnarf` is the inbound shnarf before blob 1; `endShnarf` is the outbound shnarf after blob K.

The **l2-execution proof's 15-field PI** is *input* to this guest (private witness, §2.2 step 4 recursive verification) — it is **not** the output. The 14 fields below derive from those 15 (across all N l2-execution proofs) plus the blob content and shnarf chain:

| l2-execution PI (§2.1, 15 fields) | Fate at rollup level |
|---|---|
| `parentBlockHash` | **Dropped** — folded into `parentShnarf` via `Hash(parentShnarf, lastBlockHash, blobHash)` (step 2) |
| `endBlockHash` | **Dropped** — folded into `endShnarf` (last block of the last blob) |
| `L2L1MessagesHash` | **Dropped** — per-execution flat hash consumed in step 6 to build `L2L1BridgeTransactionTree`, then discarded |
| `txFromsHash` | **Dropped** — consumed in step 3 to cross-check `froms_e` against blob `blockData.froms`; not propagated |
| `endBlockNumber` | Carried over from `PI_Eₙ` |
| `endBlockTimestamp` | Carried over from `PI_Eₙ` |
| `parentL1L2BridgeRollingHash` | Carried over from `PI_E₁` |
| `parentL1L2BridgeRollingHashMessageNumber` | Carried over from `PI_E₁` |
| `endL1L2BridgeRollingHash` | Carried over from `PI_Eₙ` |
| `endL1L2BridgeRollingHashMessageNumber` | Carried over from `PI_Eₙ` |
| `dynamicChainConfigHash` | Single shared value (step 7 asserts equality across all N) |
| `parentFtxRollingHash` | Carried over from `PI_E₁` |
| `endFtxRollingHash` | Carried over from `PI_Eₙ` |
| `lastProcessedFtxNumber` | Carried over from `PI_Eₙ` |
| `filteredAddressesHash` | Same name, content rehashed: `keccak256(addrs_E₁ ‖ … ‖ addrs_Eₙ)` (step 8) |

Plus three **new** rollup-level fields: `parentShnarf` (input), `endShnarf` (computed in step 2), `L2L1BridgeTransactionTree` (computed in step 6).

**Private Inputs (Witness)**

| Field | Description |
|---|---|
| `blobHash_b` | The blob's versioned hash as submitted on L1 — cross-checked against `kzg_commitment_to_versioned_hash(computedBlobCommitment_b)` |
| `KzgProof_b` | KZG proof for blob `b` |
| `blockRange_b` | The `(startBlockNumber, endBlockNumber)` pair for the blocks contained in blob `b` |
| `blockRlps_b` | The ordered list of canonical full block RLPs for blob `b` (`m_b` entries) — same canonical shape the l2-execution proof receives (header + tx list [+ withdrawals], EIP-2718 typed transactions in full signed form). Truncation per §3.2 happens *inside* the guest; there is no separately witnessed truncated form, and the compressed blob bytes are not witnessed either — the guest recomputes them. |
| `E₁ … Eₙ` | The l2-execution proofs, ordered by block range, tiling the combined range of all K blobs |
| `PI_E₁ … PI_Eₙ` | The public-input tuple for each l2-execution proof |
| `L2L1MsgList_e` | Per-l2-execution-proof L2→L1 message hash list, for `e ∈ [1, N]` |
| `froms_e` | Per-l2-execution-proof sender address list (block-then-transaction order) — preimage of `PI_E_e.txFromsHash` |
| `addrs_e` | Per-l2-execution-proof refused-FTX address list (§6.5) — preimage of `PI_E_e.filteredAddressesHash` |

The proven statement is **KZG verification on the canonical compressed payload computed from the canonical truncated RLP**: the guest applies the §3.2 truncation rule to each `blockRlps_b[i]` internally, RLP-encodes the truncated form, LZ4-compresses it, zero-pads to `BLOB_BYTES_LENGTH` (4096 × 32 = 131 072 bytes), computes `computedBlobCommitment_b = blob_to_kzg_commitment(paddedBytes_b)`, asserts `kzg_commitment_to_versioned_hash(computedBlobCommitment_b) == blobHash_b`, and runs `verify_blob_kzg_proof(paddedBytes_b, computedBlobCommitment_b, KzgProof_b)`. The KZG verifier accepts iff the computed bytes match what the sequencer committed to on L1 — so there is no separate byte-equality assertion against a witnessed `blobContent`; the commitment is an in-guest value, not a witness field. The intermediate truncated blocks are computed by the guest, not witnessed; their downstream consumers (block-hash boundary checks in step 5, sender-list cross-checks in step 3) read them from the in-guest computation. Authenticity of the full block RLPs is anchored by KZG (the compressed bytes are pinned to L1) plus the downstream checks — every block hash is bound to an l2-execution-proof boundary, every `froms` list to the l2-execution proof's `txFromsHash` — so the guest cannot diverge from what was actually executed.

**Statement (RISC-V Guest)**

For each blob `b ∈ [1, K]` in order, perform the per-blob block (steps 1–2); then perform the cross-blob recursion block (steps 3–8) once over the combined range.

1. **Compute and verify the blob payload (per blob).** Take the per-blob list `blockRlps_b` of `m_b` canonical full block RLPs as a private witness. For each entry: decode it, apply the §3.2 truncation rule to produce a `TruncatedEthereumBlock` (`rollup.py::TruncatedEthereumBlock`: `{timestamp, blockHash, prevRandao, transactions, froms}`) — `blockHash` is computed as `keccak256(headerRlp)` from the decoded header, `transactions` are the signature-stripped tx bytes, and `froms` are the per-tx recovered senders. RLP-encode the resulting truncated-block list in canonical order, LZ4-compress it, and zero-pad the compressed bytes to `BLOB_BYTES_LENGTH = 4096 × 32 = 131 072` bytes. Then compute `computedBlobCommitment_b = blob_to_kzg_commitment(paddedBytes_b)` inside the guest and assert `kzg_commitment_to_versioned_hash(computedBlobCommitment_b) == blobHash_b`. Finally, check that `(paddedBytes_b, computedBlobCommitment_b, KzgProof_b)` form a valid EIP-4844 blob/commitment/proof triple — the same predicate that `verify_blob_kzg_proof` computes in [consensus-specs `polynomial-commitments.md`](https://github.com/ethereum/consensus-specs/blob/master/specs/deneb/polynomial-commitments.md) and that the L1 point-evaluation precompile relies on. The Fiat-Shamir challenge `z = compute_challenge(paddedBytes_b, computedBlobCommitment_b)`, the polynomial evaluation `y = P_b(z)`, and the pairing check are entirely internal to this primitive — neither `z` nor `y` is a witness field or PI. The production guest uses a zkVM-supported KZG primitive or a deterministic linked KZG implementation with fixed trusted-setup semantics; the Python reference calls `ckzg.blob_to_kzg_commitment` and `ckzg.verify_blob_kzg_proof` directly. Also assert `m_b == blockRange_b.endBlockNumber - blockRange_b.startBlockNumber + 1`.

   These checks subsume a separate byte-equality assertion: the computed commitment must hash to L1's `blobHash`, and the KZG verifier accepts iff `paddedBytes_b` matches the bytes committed by `computedBlobCommitment_b`. Any drift between the guest-computed compressed payload and the sequencer's blob causes versioned-hash or KZG rejection. The intermediate truncated blocks are forwarded to downstream steps as in-guest values; the compressed blob bytes and computed commitment are not exposed beyond this step.

2. **Chain the shnarf (per blob).** Recompute:
   ```
   shnarf_b = Hash(shnarf_{b-1}, lastBlockHash_b, blobHash_b)
   ```
   where `shnarf_0 = parentShnarf` (public input) and `lastBlockHash_b` is the `blockHash` field of the last `TruncatedEthereumBlock` of blob `b` (from step 1). After all K blobs, assert `shnarf_K == endShnarf`.

3. **Verify sender addresses.** For each l2-execution proof `Eᵢ`, assert:
   ```
   keccak256(froms_e) == PI_Eᵢ.txFromsHash
   ```
   Then assert that `froms_1 ‖ … ‖ froms_N` equals the concatenation of `froms` across all truncated blocks (step 1 input), in canonical block-then-transaction order.

4. **Verify the l2-execution proofs.** Recursively verify each `Eᵢ` against `PI_Eᵢ`.

5. **Bind blob blocks to the l2-execution-proof chain.** Two checks together pin every block in the blob — boundary *and* intermediate — to the chain that the l2-execution proofs verified.

    a. **Boundary alignment.** For every l2-execution proof `Eᵢ`, its `endBlockHash` (PI) must equal the `blockHash` of the corresponding entry in the per-blob truncated-block list at index `Eᵢ.endBlockNumber − firstBlockNumber`. This pins the *last* block of each l2-execution proof.

    b. **Parent-hash continuity over the full range.** Decode the `header.parent_hash` of every `blockRlps_b[i]` and walk the resulting list:

       ```
       parent_hash[0]            == PI_E₁.parentBlockHash         // anchors the chain head
       parent_hash[i]            == blockHash[i-1] for i ≥ 1      // chains internal blocks
       blockHash[last]           == PI_Eₙ.endBlockHash             // already enforced by (a)
       ```

       Without (b), a malicious prover could swap a non-boundary block's header (e.g., timestamp or `prevRandao`) for a different value as long as its successor's `parent_hash` still pointed at the *original* hash, leaving the new block dangling outside the proven chain. The `from`-list cross-check (step 3) catches transaction substitutions but not header-only changes, so (b) is the load-bearing constraint for intermediate blocks.

    Adjacent l2-execution proofs already chain `endBlockHash → parentBlockHash` via step 7 below, so the head-anchor in (b) only needs to look at `PI_E₁.parentBlockHash`.

6. **Build the L2→L1 Merkle trees.** For each `e ∈ [1, N]`, receive the message hash list as a private witness and assert `keccak256(L2L1MsgList_e) == PI_E_e.L2L1MessagesHash`. Concatenate all N lists in order. Partition the combined list into consecutive chunks of `2^D` leaves (where D is the fixed protocol-level tree depth, currently 5). Pad the final chunk with zero-value (0x00…00) leaves to fill it. Each leaf is a 32-byte message hash; internal nodes are `keccak256(left ‖ right)`. Compute the root of each full tree and collect them into an ordered array `[root_1, …, root_T]`. Output `L2L1BridgeTransactionTree = keccak256(root_1 ‖ … ‖ root_T)` as a commitment to this ordered root list. The tree depth D is a protocol constant and is not included in the public output.

7. **Chain the l2-execution proofs.** For each consecutive pair `(Eᵢ, Eᵢ₊₁)` assert:
   ```
   assert_eq!(PI_Eᵢ.endBlockHash,                       PI_Eᵢ₊₁.parentBlockHash)
   assert_eq!(PI_Eᵢ.endL1L2BridgeRollingHash,                  PI_Eᵢ₊₁.parentL1L2BridgeRollingHash)
   assert_eq!(PI_Eᵢ.endL1L2BridgeRollingHashMessageNumber,     PI_Eᵢ₊₁.parentL1L2BridgeRollingHashMessageNumber)
   assert_eq!(PI_Eᵢ.dynamicChainConfigHash,                 PI_Eᵢ₊₁.dynamicChainConfigHash)
   assert_eq!(PI_Eᵢ.endFtxRollingHash,                      PI_Eᵢ₊₁.parentFtxRollingHash)
   ```
   Continuity *between* blobs is implicit — the same `assert_eq!` block applies at the blob boundary because the l2-execution proofs already tile across it.

8. **Collect forced-transaction outputs and emit PI.** For each `e ∈ [1, N]`, receive `addrs_e` as a private witness and assert `keccak256(addrs_e) == PI_E_e.filteredAddressesHash`. Concatenate all N lists in order and output `filteredAddressesHash = keccak256(addrs_1 ‖ … ‖ addrs_N)`. Take `parentFtxRollingHash` from `PI_E₁` and `endFtxRollingHash` / `lastProcessedFtxNumber` / `endBlockTimestamp` from `PI_Eₙ`. Output the 14-field public-input tuple covering the entire `K`-blob, `N`-l2-execution range.

### 2.3 rollup-aggregation Proof

The rollup-aggregation prover request recursively verifies the `M` rollup proofs covering a finalization range, outputs a single 14-field public-input tuple over the full range, and performs the emulation/SNARK wrap needed for L1 submission. The recursive rollup-aggregation topology is **flat**: one guest invocation consumes all `M` rollup proofs at once. Hierarchical / k-ary aggregation is a future option.

**Public Inputs**

The same 14-field tuple as the rollup proof (§2.2) and as the final rollup-aggregation PI (§2.4). The rollup and rollup-aggregation PI shapes match deliberately, so a rollup-aggregation proof can also be re-aggregated by a higher-level rollup-aggregation proof if hierarchy is added later without changing the PI surface.

**Private Inputs (Witness)**

- The `M` rollup proofs `B₁ … Bₘ` (or, in a hierarchical setup, prior rollup-aggregation proofs)
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
   Block-hash continuity is implicit in the shnarf check: `PI_Bᵢ.endShnarf` encodes rollup proof i's last block hash, so the shnarf assertion subsumes a separate block-hash check.

3. **Merge the L2→L1 root lists.** Receive each rollup proof's ordered root array as a private witness and verify it against its committed hash:
   ```
   for i in [1, M]: keccak256(roots_Bᵢ) == PI_Bᵢ.L2L1BridgeTransactionTree
   ```
   Concatenate all `M` arrays in order and output `L2L1BridgeTransactionTree = keccak256(roots_B₁ ‖ … ‖ roots_Bₘ)`.

4. **Merge filtered address lists.** Receive each rollup proof's address list, verify it against its committed hash, concatenate all `M` lists in order, and output `filteredAddressesHash = keccak256(addrs_B₁ ‖ … ‖ addrs_Bₘ)`.

5. **Output** the combined public inputs covering the full range: take `parentShnarf`, `parentL1L2BridgeRollingHash`, `parentL1L2BridgeRollingHashMessageNumber`, `parentFtxRollingHash`, and `dynamicChainConfigHash` from `PI_B₁`; take `endBlockNumber`, `endBlockTimestamp`, `endL1L2BridgeRollingHash`, `endL1L2BridgeRollingHashMessageNumber`, `endFtxRollingHash`, `lastProcessedFtxNumber`, and `endShnarf` from `PI_Bₘ`; use the merged Merkle commitment from step 3 and merged filtered-address hash from step 4.

The rollup-aggregation prover request includes the STARK→SNARK emulation wrap after this guest statement, so the response is directly L1-submittable — no separate emulation request file or prover invocation exists.

---

### 2.4 Final Aggregated Public Inputs

The rollup-aggregation proof's root exposes fourteen values to the L1 contract:

| # | Field |
|---|---|
| 1 | `endBlockNumber` |
| 2 | `endBlockTimestamp` |
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

### 2.5 Guest Termination Semantics

The Python reference uses `raise Exception(...)` as compact notation for proof, witness, or public-input rejection. Production Zig and Rust guests must map these failed checks to the zkVM standard failed-termination interface described in [Execution Termination Semantics](https://github.com/eth-act/zkvm-standards/blob/main/standards/standard-termination-semantics/README.md). For Linea validity proofs this spec assumes Type 1 verifier semantics: a guest failure is not an accepted proof. Type 2 proof-of-failure verification is out of scope unless a future flow explicitly needs to prove a rejected execution.

Classification:

- Guest invariant failures in `l2_execution.py`, `rollup.py`, `rollup_aggregation.py`, `block.py`, and the MPT/account/storage checks in `state_transition.py` are proof rejection points. Zig/Rust implementations should return explicit deterministic error codes and terminate as failed executions rather than relying on an uncontrolled panic path.
- Python-only stubs such as `state_transition.py::materialize_blockchain_from_execution_witness` are reference gaps, not guest semantics. They must not remain on an implementable production guest path.
- L1 finalization failures in `l1_rollup.py` model Solidity reverts, not zkVM guest termination.
- Host/environment failures in this Python reference, such as a missing trusted setup file or a host library/runtime fault, must not be collapsed into ordinary proof-invalid errors in production. The production guest should use fixed trusted-setup semantics and typed errors around KZG, MPT, compression, and recursive-verifier primitives.
- The current Python MPT helper rejects inline child nodes as "not supported in this reference". Inline MPT children are valid Ethereum trie encodings; production must either support them or document and enforce a witness-normalization rule that rejects them with a standardized failed-termination code.

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

- **Block context variables** — fields required to resolve EVM opcodes: `timestamp` (for `TIMESTAMP`) and `mixHash`/`prevrandao` (for `PREVRANDAO`). `baseFeePerGas` is not part of the truncated DA block: `BASEFEE` is resolved from the dynamic chain configuration, whose `baseFee` component is committed through `dynamicChainConfigHash`. Note: `gasLimit` and fields that are deterministic outputs of execution (such as `withdrawalsRoot`) are not included. `coinbase` (for `COINBASE`) is also supplied by the chain configuration.
- **Target block hash** — `blockHash` is included so that users can infer the return value of the keccak opcode without having access to the entire block header since a Type-1 block hash requires a perfect RLP-encoding of the full header
- **Signature-stripped transactions** — the ordered transaction list including `from` (sender address), `nonce`, gas parameters, `to`, `value`, `data`, and `accessLists`; ECDSA signatures `(v, r, s)` are omitted since the l2-execution proof guarantees they were validly signed. `from` is stored explicitly so the sender can be recovered without signature verification during re-execution. However, the transaction must figure their corresponding blob-hashes and access-lists.

**What is stripped:**

- ECDSA signatures `(v, r, s)`
- Intermediate state roots and receipt roots — these are deterministic outputs of execution, not inputs to it. Note: the current shnarf formula includes `newStateRootHash` as an explicit input; in the new design (§3.1) it is replaced by `lastBlockHash`, which is an execution input rather than an output, so no state root ever appears on-chain.
- ChainID

**Encoding and compression:** The remaining payload is compressed with a standard algorithm (LZ4 today; the framing is open to zstd in a future iteration) and zero-padded to fill the 4096 × 32-byte (`BLOB_BYTES_LENGTH = 131 072`) EIP-4844 blob field. The KZG commitment is taken over the full padded payload, so the sequencer and the rollup guest must agree byte-for-byte on the trailing zero bytes — see §2.2 step 1. Stripping the above outputs and using an unconstrained compressor significantly increases the effective throughput per blob compared to the current LZSS-based approach.

### 3.3 Prover I/O — On-Wire Format

The JSON files under `prover_inputs/` describe a logical schema. The bytes carried into the zkVM guest are binary.

**Transport.** The guest reads input bytes via the zkVM's read-input primitive (`ziskos::read_input()` on Zisk). Transport framing is an 8-byte little-endian length prefix per chunk, padded to 8-byte alignment.

**Container.** Inside the payload, the l2-execution layout is:

```
[u64 BE: block_rlp_len] [block_rlp_bytes]                — RLP-encoded block
ExecutionWitness:
  state:   [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "state"
  codes:   [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "codes"
  keys:    [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "keys"
  headers: [u64 BE: count] then [u64 BE: len][bytes]*    — debug_executionWitness field "headers"
```

Outer framing is length-prefixed binary; inner payloads are RLP. The per-FTX `signedTxRlp` payloads and the `chainConfig` fields are appended in the same `count + length-prefixed bytes` style. Rollup-proof and rollup-aggregation-proof containers follow the same convention and are pinned alongside the corresponding guest implementations.

**Debug format.** The `prover_inputs/` JSONs are the canonical schema source; the coordinator translates them to the binary container before invoking the prover. A separate supporting tool can convert JSON fixtures (e.g. `block.json` + `witness.json`) into the same binary container for local replay.

---

## 4. Bridge Mechanics

### 4.1 L1 → L2 (Deposits)

The L1→L2 bridge state is tracked via `endL1L2BridgeRollingHash` and its associated `endL1L2BridgeRollingHashMessageNumber`. These two values live in storage slots on the L2MessageService contract; they are updated by the contract's `anchorL1L2Messages` flow as a side effect of normal EVM execution. The l2-execution guest does not consume the L1→L2 message list directly — it reads the four boundary values (`parent`/`end` × `RollingHash`/`MessageNumber`) from the L2 state at `parentHeader.state_root` and `endBlock.state_root` via the EVM state interface (§2.1).

Security against L1 re-orgs is not the responsibility of the proof. The Coordinator handles this by waiting for L1 epochs to finalize before anchoring the L1 -> L2 messages — the same model as today.

Across rollup and rollup-aggregation proofs, continuity is enforced by the rollup-aggregation proof's `assert_eq!` checks on rolling hash bounds (§2.3). The `parentL1L2BridgeRollingHash` in the final public inputs allows the L1 contract to verify that the submitted proof continues exactly from the previously finalized bridge state; the new `endL1L2BridgeRollingHash` is cross-checked against L1's authoritative `l1RollingHash[messageNumber]` chain at finalization (§5).

### 4.2 L2 → L1 (Withdrawals)

The L2→L1 bridge state is tracked via `L2L1BridgeTransactionTree`, a commitment to an ordered list of fixed-depth Merkle tree roots. The message commitment is represented differently at each level of the proof tree.

**How it works across proof levels:**

- **l2-execution proof** — outputs `L2L1MessagesHash`, a flat hash of the bounded ordered list of withdrawal message hashes emitted in its range. The number of messages per l2-execution proof used to be bounded to 16 by design, keeping this commitment cheap but this requirement does not hold anymore.
- **rollup proof** — receives the per-l2-execution message hash lists as private witnesses, verifies each against the corresponding `L2L1MessagesHash`, concatenates them across all `N` l2-execution proofs, partitions the combined list into consecutive chunks of `2^D` leaves (where D is the protocol-level tree depth, currently 5), pads the last chunk with zero-value leaves, computes the root of each full tree, and outputs `L2L1BridgeTransactionTree = keccak256(root₁ ‖ … ‖ rootₖ)`. This is the single point where the flat commitment is expanded into a tree structure.
- **rollup-aggregation proof** — receives the per-rollup-proof root arrays as private witnesses, verifies each against the corresponding `L2L1BridgeTransactionTree`, concatenates the arrays in order, and outputs `keccak256(roots_B₁ ‖ … ‖ roots_Bₘ)`.

**On-chain storage and withdrawal claims.** At finalization, the submitter provides the actual root list as calldata alongside the proof. The L1 contract verifies `keccak256(roots) == L2L1BridgeTransactionTree` from the proof's public output, then stores each root via `l2MerkleRootsDepths[root] = D` exactly as today. Users claim withdrawals identically to the current flow: they provide a `merkleRoot`, `leafIndex`, and `proof[]`; the contract looks up the stored depth and verifies the sparse Merkle proof.

**Leaf position derivability from message number.** Withdrawal messages are assigned monotonically increasing message numbers. Because the finalization anchors the message number range via `parentL1L2BridgeRollingHashMessageNumber` and `endL1L2BridgeRollingHashMessageNumber`, the tree index and leaf index of any message are deterministic: for a message at offset `k` from the start of the finalization range, `treeIndex = k / 2^D` and `leafIndex = k mod 2^D`. This means a user who knows their message number can always locate their leaf without any additional on-chain data.

**`l2MessagingBlocksOffsets`.** The current system submits a compact array of uint16 block offsets alongside each finalization call. The L1 decodes these and emits `L2MessagingBlockAnchored` events, which allow off-chain indexers to map L2 blocks to their message slots. This data is **not part of the proof** — it is an unproven discoverability hint provided by the sequencer. Because leaf position is fully derivable from message number (see above), the offset list carries no security weight; a dishonest sequencer can only cause event mis-indexing, not loss of funds. The mechanism is kept unchanged in the new design.

**Comparison with the current approach:** The structure is identical to today — fixed-depth, zero-padded trees, one depth for all, roots stored per finalization in `l2MerkleRootsDepths`. The difference is that tree construction and root commitment now happen inside the RISC-V guest rather than in the bespoke pi-interconnection circuit.

---

## 5. L1 Smart Contract

The new architecture dramatically simplifies the `LineaRollup` contract.
In the Python reference, this contract-facing logic lives in `l1_rollup.py`; it is
separate from the l2-execution, rollup, and rollup-aggregation guest programs.

**What the contract does:**

1. **On blob submission:** compute `endShnarf = keccak256(parentShnarf, lastBlockHash, blobHash)` and anchor it in storage.
2. **On finalization:** verify the STARK-to-SNARK proof against the fourteen aggregated public inputs, then:
   - Assert `parentShnarf == currentFinalizedShnarf` (DA and block-hash continuity — the shnarf encodes the last block hash, so this check subsumes a separate `parentBlockHash` check)
   - Assert `parentL1L2BridgeRollingHash == currentFinalizedL1L2BridgeRollingHash` and `parentL1L2BridgeRollingHashMessageNumber == currentFinalizedL1L2BridgeRollingHashMessageNumber` (deposit bridge continuity)
   - Assert `endL1L2BridgeRollingHash == l1RollingHash[endL1L2BridgeRollingHashMessageNumber]` (deposit bridge authenticity — the proof's claimed end-of-range rolling hash must match L1's authoritative chain)
   - Assert the proof's `dynamicChainConfigHash` matches what the verifier was deployed with: `pi.dynamicChainConfigHash == IPlonkVerifier(verifier).getChainConfiguration()`. The verifier holds this digest as an immutable `bytes32` (`CHAIN_CONFIGURATION`); its preimage — the four named `ChainConfigurationParameter` entries `chainId`, `baseFee`, `coinbase`, `l2MessageServiceAddress` — is bound at verifier deploy time and emitted in the `ChainConfigurationSet` event, so the values are auditable on-chain via the deploy log + the verifier's verified constructor args. Changing any of the four values means deploying a new verifier and re-pointing the rollup at it via `setVerifierAddress`; there is no separate L1 storage slot for the chain-config preimage that could fall out of sync.
   - Verify `keccak256(submittedRoots) == L2L1BridgeTransactionTree`; store each root via `l2MerkleRootsDepths[root] = D`
   - Optionally process `l2MessagingBlocksOffsets` calldata to emit `L2MessagingBlockAnchored` discovery events (unchanged from today)
   - Update storage: `currentFinalizedLastBlockHash`, `currentFinalizedShnarf`, `currentL2BlockNumber`, `currentL2BlockTimestamp`, `currentFinalizedL1L2BridgeRollingHash`, `currentFinalizedL1L2BridgeRollingHashMessageNumber`

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

where `txHash = keccak256(signedTxRlp)` is the standard Ethereum transaction hash and `fromAddress = recover_sender(signedTxRlp, chainID)`

### 6.4 New l2-execution Public Inputs

Four FTX fields are part of the l2-execution proof public input tuple (see §2.1):

| Field | Description |
|---|---|
| `parentFtxRollingHash` | FTX rolling hash at the start of this range |
| `endFtxRollingHash` | FTX rolling hash after all FTXs handled in this range |
| `lastProcessedFtxNumber` | Sequence number of the last FTX handled in this range |
| `filteredAddressesHash` | keccak256 of the ordered list of addresses whose FTX was refused in this range; each entry is the recovered sender (`fromAddress`) for refused-from or the decoded recipient (`toAddress`) for refused-to |

These propagate through the proof tree symmetrically to the L1→L2 bridge fields: the rollup proof chains `endFtxRollingHash == parentFtxRollingHash` across consecutive l2-execution proofs (and implicitly across blob boundaries within a multi-blob rollup proof); the rollup-aggregation proof adds the same assertion across consecutive rollup proofs; the final public inputs expose all four (fields 8–11 in §2.4).

### 6.5 l2-execution Statement

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
where `txHash = keccak256(signedTxRlp)` is the standard Ethereum transaction hash of the FTX's signed RLP (as stored on L1 by `storeForcedTransaction`). The guest decodes `signedTxRlp` once, derives `fromAddress = recover_sender(signedTxRlp, chainID)`, and uses that same derived address for the rolling-hash step and any refused-from output.

**Outcome.** Each FTX carries the sequencer's declared `acceptance`, one of five variants — the cases the guest program can actually observe under RISC-V proving. The five variants and their proof-level treatment:

- *INCLUDED* — the guest asserts `txHash` appears in the declared block's transaction list (decoded from `blockRlp`).
- *Invalid sub-cases* (`BAD_NONCE` / `BAD_BALANCE`) — pre-validation must fail. The guest asserts `txHash` is NOT in the block, then reads the FTX sender's account from the L2 state at the parent state root of the block where the FTX would have been included, via the EVM state interface (a standard SLOAD/`basic`-style read against the in-process state DB backed by `debug_executionWitness.state`). It then asserts the specific failure condition:
  - `BAD_NONCE`: `account.nonce != tx.nonce`
  - `BAD_BALANCE`: `account.balance < tx.gasLimit × tx.maxFeePerGas + tx.value` (using the canonical gas-cost formula per tx type, including the blob-gas surcharge for Type-3 transactions)

  No separate per-FTX state witness is needed; the `debug_executionWitness.state` MPT node pool must include the sender account's proof path at the parent state root (§2.1).
- *Refused sub-cases* — the rollup declines for compliance reasons. No governance witness is required inside the proof; the sequencer simply declares the refusal. The L1 contract verifies a-posteriori that each bubbled-up address appears in its reference sanction list — if any entry is absent, the finalization call reverts.
  - `FILTERED_ADDRESS_FROM`: sender on the sanction list; `fromAddress` is appended to the filtered address list.
  - `FILTERED_ADDRESS_TO`: recipient on the sanction list; `toAddress` (decoded from `signedTxRlp`; rejected if the FTX is a contract-creation transaction with `to == None`) is appended instead.

After the loop the guest asserts `rollingHash == endFtxRollingHash` and outputs `filteredAddressesHash = keccak256(filtered address list)`.

### 6.6 Propagation Through the Proof Tree

- **rollup proof:** asserts `PI_Eᵢ.endFtxRollingHash == PI_Eᵢ₊₁.parentFtxRollingHash` across consecutive l2-execution proofs (§2.2 step 7); collects and concatenates filtered address lists into a single `filteredAddressesHash` (§2.2 step 8).
- **rollup-aggregation proof:** adds `assert_eq!(PI_Bᵢ.endFtxRollingHash, PI_Bᵢ₊₁.parentFtxRollingHash)` to the continuity block (§2.3 step 2); merges filtered address lists across all `M` rollup proofs by concatenation and rehashing (§2.3 step 4).
- **Final public inputs:** exposes `parentFtxRollingHash`, `endFtxRollingHash`, `lastProcessedFtxNumber`, `filteredAddressesHash` as fields 8–11 (§2.4).

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

| Component | Current (Type-2)                                                                                                      | New (Type-1 RISC-V) |
|---|-----------------------------------------------------------------------------------------------------------------------|---|
| **Shnarf formula** | `keccak256(parent, snarkHash, stateRoot, X, Y)` — 5 inputs; `snarkHash` must be computed in-circuit                   | `keccak256(parent, lastBlockHash, blobHash)` — 3 standard inputs |
| **KZG verification** | L1 contract calls `0x0A` precompile; `X` and `Y` exposed on-chain                                                     | Commitment computed and proof verified inside zkVM guest; `blobKzgCommitment`, `X`, and `Y` never appear on-chain |
| **Compression** | Custom SNARK-friendly LZSS; arithmetization-constrained compression ratio                                             | Standard LZ4/zstd compiled into RISC-V guest; unconstrained ratio |
| **Proof interconnection** | Bespoke pi-interconnection circuit in Go/Gnark; gate-level array mapping                                              | rollup proof: recursively verifies N l2-execution proofs across K ≥ 1 blobs and chains them with `assert_eq!` in the RISC-V guest. rollup-aggregation proof: flat recursion over M rollup proofs, same continuity assertions across rollup-proof boundaries |
| **l2-execution public inputs** | ~14 Type-2 parameters (timestamps, batch indices, conflation data, dynamic arrays)                                    | 15 fields — see §2.1. Drops state roots (block-hash chain anchors continuity); keeps `endBlockTimestamp`; adds FTX fields (`parent`/`endFtxRollingHash`, `lastProcessedFtxNumber`, `filteredAddressesHash`) and `txFromsHash` |
| **L2→L1 tree construction** | l2-execution proof outputs flat hash of bounded message list; pi-interconnection organizes into fixed-depth Merkle trees | Same flat hash at l2-execution level; rollup proof partitions messages into fixed-depth zero-padded trees and outputs `keccak256(roots)`; rollup-aggregation proof concatenates the per-rollup-proof root arrays and rehashes |
| **l2MessagingBlocksOffsets** | Unproven hint; L1 emits `L2MessagingBlockAnchored` events for off-chain indexing                                      | Unchanged — still an unproven hint; leaf position is fully derivable from message number, so no security impact |
| **DA payload — intermediate roots** | `blockHash`, `timestamp` and transaction RLP without signature + From                                                 | adding `prevRandao` |
| **L1 contract** | Complex: precompile calls, dynamic Type-2 input formatting, SNARK-friendly hash routing                               | Lightweight: verify proof against 13 values + roots/addresses calldata, equality checks against stored state, update storage slots |
| **Final aggregated public inputs** | 13 fields (shnarfs, timestamps, block numbers, rolling hashes ×2, Merkle roots…)                                      | 14 fields — see §2.4 |
| **rollup-proof granularity** | n/a (no rollup proof existed; compression was a separate proof per blob)                                                | Configurable: one rollup proof can cover `K ≥ 1` blobs (analogous to today's M-block conflation inside an l2-execution proof). `K = 1` is the simplest case; `K > 1` amortizes recursion overhead |
