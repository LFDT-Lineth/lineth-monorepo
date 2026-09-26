# Prover I/O Drafts — Type-1 RISC-V Rollup

This directory documents the prover request/response I/O for the new RISC-V proving stack described in `../Readme.md` and holds the fully-valid example fixtures under `testdata/`. The shapes track the existing Lineth conventions (0x-prefixed hex, RLP-encoded DA blocks, version strings) and cover the public-input surface: rollup roots, filtered addresses, and program VKs are list-valued fields at the rollup / rollup-aggregation layer; l2-execution has 16 fields.

The JSON Schemas under `schemas/` are the versioned wire contract; `../proof_io_v1.py` is the codec that converts schema-valid JSON to/from the guest dataclasses (the logical model), and the `testdata/` fixtures are the language-neutral golden vectors that validate against those schemas (see `schemas/README.md`). **Guest output vs prover output:** a guest emits its public-input tuple; the `proof` bytes are attached by the zkVM/prover layer, not the guest, and are placeholders (`0x`) in the fixtures. The next proving step (or L1) consumes the prover output (guest output + `proof`).

> **JSON ≠ on-wire format.** These files describe *what* the coordinator hands to the prover at each layer (which fields exist, what they mean, how they relate). For l2-execution, `payloads[].statelessInput` is a readable JSON object mirroring the SSZ `StatelessInput` (`newPayloadRequest` + `executionWitness` + `chainConfig`). The codec (`../proof_io_v1.py`, via `stateless_input.py::encode_stateless_input_ssz`) SSZ-encodes it into the length-delimited bytes the guest reads — the prover's encode step — and the guest decodes them back with `decode_stateless_input_ssz` (backed by `remerkleable`). The readable `chainConfig` carries `{chainId, forkName}` only (the encoder reconstructs the full SSZ fork config), and `publicKeys` is omitted entirely — the codec recovers each transaction's public key from the signed transactions into the SSZ `public_keys` field. Lineth rollup-extension metadata wraps the stateless input at the proof-range layer. See §3.3 of the spec README (`../../../README.md`) for the transport details.

## Proving flow

```
l2-execution proofs -> rollup proof -> rollup-aggregation proof + emulation
```

The **rollup-aggregation proof is the final proof** of the chain — same as today.

These files are scoped to prover/guest inputs. L1 continuity anchors from the
currently finalized rollup state are checked by the contract-facing logic in
`../l1_rollup.py`; they are not rollup-aggregation guest inputs.

One request/response pair per guest layer (fully-valid fixtures in `testdata/`):

| Fixture (`testdata/`) | Layer | What it proves |
|---|---|---|
| `getZkL2ExecutionProofV1.{request,response}.json` | l2-execution proof (per block range, M ≥ 1 conflated payloads) | EVM state transition for a contiguous range of Engine API `NewPayloadRequest`s; emits the 16-field l2-execution PI tuple. |
| `getZkRollupProofV1.{request,response}.json` | rollup proof over N ≥ 1 conflations and their touched chunks | Recomputes independently length-prefixed zstd segments from `blockRlps`, walks the mixed blob/calldata chunk sequence, verifies each chunk's kind-specific binding hash, folds the dataRollingHash, and recursively verifies the l2-execution proofs whose ranges tile the combined block range. Stream offsets are canonical: 0 at every fully consumed chunk boundary and positive only inside a shared blob. The PI carries the ordered L2L1 roots and filtered-address lists. |
| `getZkRollupAggregationProofV1.{request,response}.json` | rollup-aggregation proof + emulation (the final proof, SNARK-wrapped for L1) | Recursively verifies all M rollup proofs covering the finalization range, asserts exact pairwise stream-position and execution continuity, merges the ordered L2L1 root arrays and FTX filtered-address lists into public inputs, and performs the STARK→SNARK emulation wrap in the same rollup-aggregation request. Flat (one guest invocation over all M); hierarchical aggregation is a future option. There is no separate emulation file. The response retains `l2MessagingBlocksOffsets` as top-level calldata metadata. |

## Rollup-Proof Generalization: T ≥ 1 chunks in one proof

A single rollup proof can fold `T ≥ 1` mixed blob or calldata chunks. `chunks[]` carries the ordered chunk witnesses, `conflations[]` carries the independently compressed segments, and `l2ExecutionProofs[]` carries the proofs that tile the combined block range. The public-input tuple covers the entire fold regardless of the number or kinds of chunks touched.

## Common conventions

- `programVk` — identifies the guest program/circuit the request targets; routing metadata that wraps the `proofRequest` envelope and is not a guest input.
- `proverVersion` — same string the existing prover responds with (e.g. `"4.0.0-riscv"`); carried on each response and forwarded to L1.
- `chainConfig` — the dynamic chain configuration supplied at the l2-execution range layer (`l2MessageServiceAddress`, `coinbase`, `chainID`, `baseFee`). `dynamicChainConfigHash = keccak256(uint256_be(chainID) || coinbase || l2MessageServiceAddress || uint256_be(baseFee))`, where integer fields are 32-byte big-endian values and addresses are canonical 20-byte values. `baseFee` is read from the first `NewPayloadRequest.executionPayload.baseFeePerGas` and asserted equal across the range. The range-level `chainID` intentionally duplicates the chain id decoded from each vanilla `StatelessInput`; the guest rejects the proof if any inner value differs. Rollup and rollup-aggregation proofs do not carry `chainConfig` — they inherit the hash from inner-proof PIs.
- `statelessInput` — the decoded stateless guest input as a readable JSON object (mirroring SSZ `StatelessInput`). The codec SSZ-encodes it into the length-delimited bytes `run_l2_execution_guest` reads (the prover's encode step), which the Python reference decodes with `remerkleable`, accepting the same raw/Ere-prefixed shape the underlying engine's decoder accepts. Lineth extension bytes are parsed outside this slice and must not be appended to it.
- `newPayloadRequest` — the decoded Engine API request consumed by the l2-execution guest instead of an RLP-encoded block. It contains `executionPayload`, `versionedHashes`, `parentBeaconBlockRoot`, and typed `executionRequests`.
- `executionWitness` — decoded witness byte lists (`state`, `codes`, `headers`, and optional `keys`). `headers` are RLP-encoded parent/ancestor headers ordered by block number and ending at the payload parent; the parent header hash must equal `newPayloadRequest.executionPayload.parentHash`. The canonical EIP-8025 `SszExecutionWitness` contains `state`, `codes`, and `headers`; `keys` appears only in decoded JSON/debug mirrors unless a future Lineth schema id explicitly adds it. The `state` MPT node pool must additionally include proof paths for any state the guest reads beyond block execution — at minimum, the L2MessageService's `L1L2RollingHash` and `L1L2RollingHashMessageNumber` slots at both the parent and end state roots (§2.1, §4.1), and the sender account of any Invalid FTX (§6.5).
- `publicKeys` — **not carried on the wire.** The vanilla SSZ `StatelessInput` has a `public_keys` field (uncompressed transaction public keys, ordered by `executionPayload.transactions` index), but it is derivable from the signed transactions, so the prover middleware recovers it when building the guest input (`stateless_input.py::_recover_public_keys`, via execution-specs `recover_transaction_public_key`) rather than transmitting it. The Lineth guest does not consume `public_keys` at all — it derives transaction senders with `recover_sender(chainID, tx)`; any implementation optimization that consumes `public_keys` must produce the same sender address.
- `signedTxRlp` remains the canonical bytes for forced-transaction hashing. Normal block transactions come from `newPayloadRequest.executionPayload.transactions` as canonical signed transaction byte lists; DA `blockRlps` are rollup-proof inputs only. `fromAddress` for a forced transaction is recovered from `signedTxRlp` and is not a separate witness field.
- `rollupExtension.forcedTransactions` — per-block witness array mirroring `block.py::ForcedTransactionWitness`. Empty for blocks without FTXs.
- All fixed-size byte fields are `0x`-prefixed hex in JSON; blob bytes stay base64-encoded to match `prover/backend/blobsubmission`. In the Python reference, semantic hashes use `Hash32`, fixed-width non-hash values use `Bytes32`/`BytesN`, and plain `bytes` is reserved for variable-length encodings or payloads.
- Cross-proof references use the `<startBlock>-<endBlock>-getZk<Type>.json` filename convention already used by `prover/backend`.

The `testdata/` fixtures use small but fully-valid hex (no ellipses) so the codec round-trip is byte-exact; final production test-vectors will be generated by `prover/backend/execution/testcase_gen` once the new circuits land.
