# Prover I/O Drafts — Type-1 RISC-V Rollup

This directory drafts the prover-request **logical schemas** for the new RISC-V proving stack described in `../Readme.md`. The shapes track the existing Linea conventions (0x-prefixed hex, base64-encoded blob bytes, RLP-encoded DA blocks, version strings) but cover the new public-input surface (14 fields at the rollup / rollup-aggregation layer, 15 fields at the l2-execution layer, FTX rolling hash, shnarf rebased on `lastBlockHash`).

> **JSON ≠ on-wire format.** These files describe *what* the coordinator hands to the prover at each layer (which fields exist, what they mean, how they relate). For l2-execution, `payloads[].statelessInputSsz` is the vanilla stateless guest input: SSZ-encoded `StatelessInput` bytes. The Python guest path decodes those bytes with `stateless_input.py::decode_stateless_input_ssz`, backed by `remerkleable`, into `NewPayloadRequest`, `ExecutionWitness`, stateless chain config, and optional transaction public keys. A decoded `_debugStatelessInput` object may appear in draft/debug JSON only as a review mirror; loaders must derive or validate it from `statelessInputSsz` and discard it before constructing `L2ExecutionProofPrivateInput`. Linea rollup-extension metadata wraps the stateless input at the proof-range layer. See §3.3 of `../Readme.md` for the transport details.

## Proving flow

```
l2-execution proofs -> rollup proof -> rollup-aggregation proof + emulation
```

The **rollup-aggregation proof is the final proof** of the chain — same as today.

These files are scoped to prover/guest inputs. L1 continuity anchors from the
currently finalized rollup state are checked by the contract-facing logic in
`../l1_rollup.py`; they are not rollup-aggregation guest inputs.

One request file per guest layer:

| File | Layer | What it proves |
|---|---|---|
| `getZkL2ExecutionProof.request.json` | l2-execution proof (per block range, M ≥ 1 conflated payloads) | EVM state transition for a contiguous range of Engine API `NewPayloadRequest`s; emits the 15-field l2-execution PI tuple. |
| `getZkRollupProof.request.json` | rollup proof (per K ≥ 1 blobs) | For each blob, recomputes the canonical compressed payload from `blockRlps` (truncate → RLP-encode → LZ4-compress → zero-pad to 131072 bytes), computes the KZG commitment from those bytes, checks its versioned hash against the L1-committed `blobHash`, and verifies `blobKzgProof`; there is no separately witnessed `blobKzgCommitment` or `compressedData`. Also chains the shnarf transition across all K blobs and recursively verifies the N l2-execution proofs whose ranges tile the combined block range. Emits the 14-field rollup PI tuple. |
| `getZkRollupAggregationProof.request.json` | rollup-aggregation proof + emulation (the final proof, SNARK-wrapped for L1) | Recursively verifies all M rollup proofs covering the finalization range, asserts pairwise continuity, merges the L2L1 root arrays and FTX filtered-address lists, emits the same 14-field PI tuple over the full range, and performs the STARK→SNARK emulation wrap in the same rollup-aggregation request. Flat (one guest invocation over all M); hierarchical aggregation is a future option. There is no separate emulation file. |

## Rollup-Proof Generalization: K ≥ 1 blobs in one proof

A single rollup proof can fold `K ≥ 1` blobs together. `K = 1` is the simplest case (one blob per rollup proof — the diagram from the meeting); `K > 1` lets the coordinator amortize recursion overhead by handling several blobs in a single guest invocation, exactly as today's l2-execution conflation handles `M ≥ 1` blocks in a single guest invocation. The schema is the same in either case: `blobs[]` carries the list, `l2ExecutionProofs[]` carries the (typically larger) list of l2-execution proofs that tile the combined block range, and the public-input tuple covers the entire fold. The fixture in `getZkRollupProof.request.json` uses `K = 2` to exercise this path.

## Common conventions

- `proverVersion` — same string the existing prover responds with (e.g. `"4.0.0-riscv"`); coordinator forwards to L1.
- `chainConfig` — the dynamic chain configuration supplied at the l2-execution range layer (`l2MessageServiceAddress`, `coinbase`, `chainID`, `baseFee`). `dynamicChainConfigHash = keccak256(uint256_be(chainID) || coinbase || l2MessageServiceAddress || uint256_be(baseFee))`, where integer fields are 32-byte big-endian values and addresses are canonical 20-byte values. `baseFee` is read from the first `NewPayloadRequest.executionPayload.baseFeePerGas` and asserted equal across the range. The range-level `chainID` intentionally duplicates the chain id decoded from each vanilla `StatelessInput`; the guest rejects the proof if any inner value differs. Rollup and rollup-aggregation proofs do not carry `chainConfig` — they inherit the hash from inner-proof PIs.
- `statelessInputSsz` — the length-delimited vanilla stateless-input SSZ bytes consumed by the l2-execution guest. The Python reference uses `remerkleable` for container decoding and accepts the same raw/Ere-prefixed shape that the underlying engine's decoder accepts. This is the only `StatelessInput` form accepted by `run_l2_execution_guest`; Linea extension bytes are parsed outside this slice and must not be appended to it.
- `newPayloadRequest` — the decoded Engine API request consumed by the l2-execution guest instead of an RLP-encoded block. It contains `executionPayload`, `versionedHashes`, `parentBeaconBlockRoot`, and typed `executionRequests`.
- `executionWitness` — decoded witness byte lists (`state`, `codes`, `headers`, and optional `keys`). `headers` are RLP-encoded parent/ancestor headers ordered by block number and ending at the payload parent; the parent header hash must equal `newPayloadRequest.executionPayload.parentHash`. The canonical EIP-8025 `SszExecutionWitness` contains `state`, `codes`, and `headers`; `keys` appears only in decoded JSON/debug mirrors unless a future Linea schema id explicitly adds it. The `state` MPT node pool must additionally include proof paths for any state the guest reads beyond block execution — at minimum, the L2MessageService's `L1L2RollingHash` and `L1L2RollingHashMessageNumber` slots at both the parent and end state roots (§2.1, §4.1), and the sender account of any Invalid FTX (§6.5).
- `publicKeys` — optional transaction public keys carried by the vanilla stateless input, ordered by `executionPayload.transactions` index. They are separate from `executionWitness.keys`, which are state-access/debug hints. The Linea logical spec and Python reference derive transaction senders with execution-specs `recover_sender(chainID, tx)`. `publicKeys` is not a witness override; any implementation optimization that consumes it must produce the same accepted/rejected transaction result and sender address as `recover_sender(chainID, tx)`.
- `signedTxRlp` remains the canonical bytes for forced-transaction hashing. Normal block transactions come from `newPayloadRequest.executionPayload.transactions` as canonical signed transaction byte lists; DA `blockRlps` are rollup-proof inputs only. `fromAddress` for a forced transaction is recovered from `signedTxRlp` and is not a separate witness field.
- `rollupExtension.forcedTransactions` — per-block witness array mirroring `block.py::ForcedTransactionWitness`. Empty for blocks without FTXs.
- All fixed-size byte fields are `0x`-prefixed hex in JSON; blob bytes stay base64-encoded to match `prover/backend/blobsubmission`. In the Python reference, semantic hashes use `Hash32`, fixed-width non-hash values use `Bytes32`/`BytesN`, and plain `bytes` is reserved for variable-length encodings or payloads.
- Cross-proof references use the `<startBlock>-<endBlock>-getZk<Type>.json` filename convention already used by `prover/backend`.

The placeholder values (`0xdeadbeef…`, `0xaaaa…`) are fixtures; final test-vectors will be generated by `prover/backend/execution/testcase_gen` once the new circuits land.
