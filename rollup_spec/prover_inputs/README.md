# Prover I/O Drafts — Type-1 RISC-V Rollup

This directory drafts the prover-request **logical schemas** for the new RISC-V proving stack described in `../Readme.md`. The shapes track the existing Linea conventions (0x-prefixed hex, base64-encoded blob bytes, RLP-encoded blocks, version strings) but cover the new public-input surface (14 fields at the rollup / rollup-aggregation layer, 15 fields at the l2-execution layer, FTX rolling hash, shnarf rebased on `lastBlockHash`).

> **JSON ≠ on-wire format.** These files describe *what* the coordinator hands to the prover at each layer (which fields exist, what they mean, how they relate). The bytes actually carried into the zkVM guest are **binary** — a length-prefixed container holding RLP-encoded blocks and the `debug_executionWitness` payload verbatim. See §3.3 of `../Readme.md` for the on-wire format spec. The JSON form here is the canonical source of truth for the schema, and is also accepted directly by the guest in a `--json` debug mode for fixture loading.

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
| `getZkL2ExecutionProof.request.json` | l2-execution proof (per block range, M ≥ 1 conflated blocks) | EVM state transition for a contiguous range of L2 blocks; emits the 15-field l2-execution PI tuple. |
| `getZkRollupProof.request.json` | rollup proof (per K ≥ 1 blobs) | For each blob, recomputes the canonical compressed payload from `blockRlps` (truncate → RLP-encode → LZ4-compress → zero-pad to 131072 bytes), computes the KZG commitment from those bytes, checks its versioned hash against the L1-committed `blobHash`, and verifies `blobKzgProof`; there is no separately witnessed `blobKzgCommitment` or `compressedData`. Also chains the shnarf transition across all K blobs and recursively verifies the N l2-execution proofs whose ranges tile the combined block range. Emits the 14-field rollup PI tuple. |
| `getZkRollupAggregationProof.request.json` | rollup-aggregation proof + emulation (the final proof, SNARK-wrapped for L1) | Recursively verifies all M rollup proofs covering the finalization range, asserts pairwise continuity, merges the L2L1 root arrays and FTX filtered-address lists, emits the same 14-field PI tuple over the full range, and performs the STARK→SNARK emulation wrap in the same rollup-aggregation request. Flat (one guest invocation over all M); hierarchical aggregation is a future option. There is no separate emulation file. |

## Rollup-Proof Generalization: K ≥ 1 blobs in one proof

A single rollup proof can fold `K ≥ 1` blobs together. `K = 1` is the simplest case (one blob per rollup proof — the diagram from the meeting); `K > 1` lets the coordinator amortize recursion overhead by handling several blobs in a single guest invocation, exactly as today's l2-execution conflation handles `M ≥ 1` blocks in a single guest invocation. The schema is the same in either case: `blobs[]` carries the list, `l2ExecutionProofs[]` carries the (typically larger) list of l2-execution proofs that tile the combined block range, and the public-input tuple covers the entire fold. The fixture in `getZkRollupProof.request.json` uses `K = 2` to exercise this path.

## Common conventions

- `proverVersion` — same string the existing prover responds with (e.g. `"4.0.0-riscv"`); coordinator forwards to L1.
- `chainConfig` — the dynamic chain configuration supplied at the l2-execution layer (`l2MessageServiceContract`, `coinbase`, `chainID`, `baseFee`). `dynamicChainConfigHash = keccak256(uint256_be(chainID) || coinbase || L2MessageServiceContract || uint256_be(baseFee))`, where integer fields are 32-byte big-endian values and addresses are canonical 20-byte values. Rollup and rollup-aggregation proofs do not carry `chainConfig` — they inherit the hash from inner-proof PIs.
- `executionWitness` — Besu `debug_executionWitness` payload (stateless witness: account/storage trie proofs, contract code, code hashes, recent headers). The first block's witness must include exactly one parent header whose block hash equals the first block's `parentHash`; there is no separate `parentBlockChain` input. The `state` MPT node pool must additionally include proof paths for any state the guest reads beyond block execution — at minimum, the L2MessageService's `L1L2RollingHash` and `L1L2RollingHashMessageNumber` slots at both the parent and end state roots (§2.1, §4.1), and the sender account of any Invalid FTX (§6.5).
- `blockRlp` and `signedTxRlp` are the canonical bytes for block and forced-transaction hashing. The Python reference decodes execution views from those bytes internally; decoded objects are not prover inputs and should not be re-encoded just to compute hashes. `fromAddress` for a forced transaction is recovered from `signedTxRlp` and is not a separate witness field.
- `forcedTransactions` — per-block witness array mirroring `block.py::ForcedTransactionWitness`. Empty for blocks without FTXs.
- All fixed-size byte fields are `0x`-prefixed hex in JSON; blob bytes stay base64-encoded to match `prover/backend/blobsubmission`. In the Python reference, semantic hashes use `Hash32`, fixed-width non-hash values use `Bytes32`/`BytesN`, and plain `bytes` is reserved for variable-length encodings or payloads.
- Cross-proof references use the `<startBlock>-<endBlock>-getZk<Type>.json` filename convention already used by `prover/backend`.

The placeholder values (`0xdeadbeef…`, `0xaaaa…`) are fixtures; final test-vectors will be generated by `prover/backend/execution/testcase_gen` once the new circuits land.
