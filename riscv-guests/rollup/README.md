# Rollup Guest

The RISC-V guest program of the rollup proof (`rollup_spec/Readme.md` §2.2). It decodes the canonical `RollupProofPrivateInput` SSZ envelope (schema id `0x1001`), runs the rollup statement over it and emits the rollup public inputs (schema id `0x1801`: `keccak256(frame) || frame`, where `frame` is the schema id followed by the SSZ public inputs). The input codec is `src/rollup_ssz.zig`; the output codec and the l2-execution proofs the input embeds are the `core_serde` wire contracts of `guest_common`.

## Statement

One rollup proof covers N consecutive whole conflations, each paired with its l2-execution proof, over the chunks (blobs and calldata submissions) of the data-availability stream that carries their compressed payloads. `src/rollup.zig` runs the steps of §2.2:

1. **Canonical payloads** (`src/canonical.zig`). Every conflation's full block RLPs are truncated and encoded into the payload its zstd frame decompresses to (`lineth_da.encoding.truncation`); the block count has to match the paired proof's range, and the senders recovered from the signed transactions have to hash to the proof's `txFromsHash` (step 3).
2. **Data availability** (`lineth_da.decompress`). Every chunk is bound to its `chunkHash`, every blob's EIP-4844 proof opens its commitment to the blob, the stream is parsed under the four layout rules into one segment per conflation, every segment's zstd frame decompresses with the embedded dictionary `v2025-04-21` to its conflation's canonical payload, and the dataRollingHash folds over the chunks. `endDataRollingHash` and `finalDataTailDiscardBytes` come out of it.
3. **Recursion** (`src/recursion.zig`). Every l2-execution proof is verified under its program VK against the hash of its public input. The verifier accepts every proof.
4. **Chain** (`src/chain.zig`). The proofs tile the block range; every proof's end block hash is the hash of the conflations' block at its boundary; the blocks chain by parent hash from the first proof's parent block hash over the whole range; consecutive proofs agree on block hash, chain configuration and forced-transaction state.
5. **Public inputs** (`src/rollup.zig`, `src/messaging_offsets.zig`). The L2-to-L1 messages concatenate in proof order; the filtered addresses and the program VKs are distinct and ascending; the bridge pair is the last non-zero one; the messaging-block offsets are rebased onto the first block of the proof and the block count is the length of the range.

"first"/"last" below are the first/last elements of the input's `l2_execution_proofs` list.

| `RollupPublicInput` field | Source |
| --- | --- |
| `end_block_number` | last exec proof's `public_inputs.end_block_number` |
| `end_block_timestamp` | last exec proof's `public_inputs.end_block_timestamp` |
| `end_l1_l2_bridge_rolling_hash` | `public_inputs.end_l1_l2_bridge_rolling_hash` of the last exec proof whose message number is non-zero, else zeros |
| `end_l1_l2_bridge_rolling_hash_message_number` | that exec proof's `public_inputs.end_l1_l2_bridge_rolling_hash_message_number`, else zero |
| `dynamic_chain_config_hash` | first exec proof's `public_inputs.dynamic_chain_config_hash` |
| `parent_ftx_rolling_hash` | first exec proof's `public_inputs.parent_ftx_rolling_hash` |
| `parent_ftx_number` | first exec proof's `public_inputs.parent_ftx_number` |
| `end_ftx_rolling_hash` | last exec proof's `public_inputs.end_ftx_rolling_hash` |
| `end_processed_ftx_number` | last exec proof's `public_inputs.end_processed_ftx_number` |
| `parent_data_rolling_hash` | input's `parent_data_rolling_hash` |
| `end_data_rolling_hash` | the dataRollingHash after folding the chunks (step 2) |
| `parent_block_hash` | first exec proof's `public_inputs.parent_block_hash` |
| `end_block_hash` | last exec proof's `public_inputs.end_block_hash` |
| `parent_data_tail_take_bytes` | input's `parent_data_tail_take_bytes` |
| `final_data_tail_discard_bytes` | the bytes of the stream behind the last segment (step 2) |
| `l2_l1_messages` | concatenation of every exec proof's `public_inputs.l2_l1_messages`, proof order |
| `filtered_addresses` | every exec proof's `public_inputs.filtered_addresses`, distinct and sorted ascending bytewise |
| `program_vks` | every `l2_execution_proofs[i].program_vk`, distinct and sorted ascending bytewise |
| `block_count` | `end_block_number` of the last exec proof minus `start_block_number` of the first, plus one |
| `l2_messaging_blocks_offsets` | every exec proof's offsets, each plus the proof's distance from the first block of the first proof |

## Exit codes

A rejected request exits with the category of the step that rejects it (`src/guest_errors.zig`); success exits 0. A category never changes its number when its errors change.

| Code | Category |
| --- | --- |
| 1 | unknown |
| 2 | the schema id of the frame is missing or wrong |
| 3 | the SSZ body is not the canonical encoding |
| 4 | a list or byte field exceeds its bound |
| 5 | the request has no conflation or proof, or they do not pair one to one |
| 6 | the output encoder rejected the public inputs: a list exceeds the bound of the output's wire schema |
| 7 | blocks and senders: a block is not accepted, a sender is not recoverable, the block count differs from the proof's range, the senders do not hash to `txFromsHash` |
| 8 | chunk binding: a chunk contradicts its kind, is not the canonical packing, does not hash to its `chunkHash`, or the boundary preimage does not open the parent dataRollingHash |
| 9 | a blob's EIP-4844 proof does not open its commitment |
| 10 | stream layout: a layout rule is broken, or a segment's length is missing, zero or beyond the stream |
| 11 | decompression: a segment is not one complete frame of the dictionary, or decompresses to another payload |
| 12 | recursion: a proof is rejected by the recursive verifier |
| 13 | block-hash chain: an end block hash or a parent hash does not match the conflations' blocks |
| 14 | tiling and continuity between consecutive proofs |
| 15 | messaging offsets: the block count differs from the range, or an offset is out of order, out of range or exceeds 16 bits once rebased |
| 16 | an allocation exceeds the guest heap, whether it builds the KZG context, decodes the input, runs the statement or encodes the output |

## Allocation

The guest serves every allocation from one bump allocator over the guest heap, which keeps them until the guest exits. The same allocator backs the Constantine archive's C `malloc` through `zkvm_provide.setAllocator`, which `src/main.zig` installs before anything runs. The KZG trusted-setup context of the blob verification is built right after, before the input is read, so it takes the same heap prefix for every request. A request that needs more than the heap exits with code 16.

## Development

```bash
make -C rollup test             # native host unit tests
make -C rollup reference-test   # the statement over consecutive blocks of the EF zkevm fixtures
make -C rollup compile          # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec             # compile, then run under the ZkC interpreter on a freshly generated sample input
```

`-Dkeccak-accel` (`KECCAK_ACCEL=` on `make`) and `-Dsecp256k1-backend` (`SECP256K1_BACKEND=`) select the keccak and secp256k1 providers of `zkvm_provide`, as in l2-execution.

### Tests

The native tests (`test/`) build real requests with `test/scenario.zig`: conflations of random signed blocks, their canonical payloads compressed with libzstd against the dictionary, the stream cut into blob and calldata chunks as a plan says (shared blob boundaries included) with the commitments and proofs of c-kzg-4844, and one l2-execution proof per conflation. Everything is computed with the test-side encoders of `lineth_da`, without the rollup package. `rollup_test.zig` asserts the public inputs of accepted requests; `rejection_test.zig` breaks every rule of the statement in an otherwise valid request and asserts the error and its exit-code category.

`make reference-test` (`test/reference/`) builds requests from the Amsterdam blocks of the Ethereum Foundation's zkevm fixtures (`tests-zkevm@v21.0.5`, a lazy dependency of 3.4 GB): the consecutive valid blocks of each fixture are grouped into conflations, whose block hashes, parent hashes and senders come from the fixture, and published as calldata and blob chunks, alone and sharing a blob with neighbouring proofs. Every request is accepted with the public inputs the fixture determines, and a tampered copy of each is rejected with the error of the tampered rule.

`make exec` runs the guest on the request `tools/gen_fixture.zig` generates (`zig-out/sample.request.ssz`): one conflation of three signed blocks published in one blob, checked by the statement natively before it is written.
