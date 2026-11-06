# Rollup Guest

The RISC-V guest program of the rollup proof (`rollup_spec/Readme.md` §2.2). It decodes the canonical `RollupProofPrivateInput` SSZ envelope (schema id `0x1001`), runs the rollup statement over it and emits the rollup public inputs (schema id `0x1801`: `keccak256(frame) || frame`, where `frame` is the schema id followed by the SSZ public inputs). The input codec is `src/rollup_ssz.zig`; the output codec and the l2-execution proofs the input embeds are the `core_serde` wire contracts of `guest_common`.

## Statement

One rollup proof covers N consecutive whole conflations, each paired with its l2-execution proof, over the chunks (blobs and calldata submissions) of the data-availability stream that carries their compressed payloads. `src/rollup.zig` runs these parts of §2.2 in this order:

- **Canonical payloads** (steps 1 and 3; `src/canonical.zig`). Every conflation's full block RLPs are truncated and encoded into the payload its zstd frame decompresses to (`lineth_da.encoding.truncation`); the block count has to match the paired proof's range, and the senders recovered from the signed transactions have to hash to the proof's `txFromsHash`.
- **Data availability** (step 2; `lineth_da.decompress`). Every chunk is bound to its `chunkHash`, every blob's EIP-4844 proof opens its commitment to the blob, the stream is parsed under the four layout rules into one segment per conflation, every segment's zstd frame decompresses with the embedded dictionary `v2025-04-21` to its conflation's canonical payload, and the dataRollingHash folds over the chunks. `endDataRollingHash` and `finalDataTailDiscardBytes` come out of it.
- **Tiling** (the witness `E₁ … Eₙ`; `src/chain.zig`). Every proof starts right after the end of the proof before it.
- **Recursion** (step 4; `src/recursion.zig`). Every l2-execution proof is verified under its program VK against the hash of its public input. The verifier accepts every proof.
- **Chain** (steps 5 and 7; `src/chain.zig`). Every proof's end block hash is the hash of the conflations' block at its boundary; the blocks chain by parent hash from the first proof's parent block hash over the whole range; consecutive proofs agree on block hash, chain configuration and forced-transaction state.
- **Public inputs** (steps 6, 8 and 9; `src/rollup.zig`, `src/messaging_offsets.zig`). The L2-to-L1 messages concatenate in proof order; the filtered addresses and the program VKs are distinct and ascending; the bridge pair is the last non-zero one; the messaging-block offsets are rebased onto the first block of the proof and the block count is the length of the range.

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
| 5 | the request has no conflation, chunk or proof, or the conflations and proofs do not pair one to one |
| 6 | the output encoder rejected the public inputs: a list exceeds the bound of the output's wire schema |
| 7 | blocks and senders: a block is not accepted, a sender is not recoverable, a conflation's block count differs from its proof's range, the senders do not hash to `txFromsHash` |
| 8 | chunk binding: a chunk contradicts its kind, is not the canonical packing, does not hash to its `chunkHash`, or the boundary preimage does not open the parent dataRollingHash |
| 9 | a blob's EIP-4844 proof does not open its commitment |
| 10 | stream layout: a layout rule is broken, or a segment's length is missing, zero or beyond the stream |
| 11 | decompression: a segment is not one complete frame of the dictionary, or decompresses to another payload |
| 12 | recursion: a proof is rejected by the recursive verifier, or its public input is beyond a bound of its wire schema |
| 13 | block-hash chain: an end block hash or a parent hash does not match the conflations' blocks |
| 14 | tiling and continuity between consecutive proofs |
| 15 | messaging offsets: a proof's `block_count` differs from its block range, or an offset is out of order, out of range or exceeds 16 bits once rebased |
| 16 | an allocation exceeds the guest heap, whether it builds the KZG context, decodes the input, runs the statement or encodes the output |

## Allocation

The guest serves every allocation from one bump allocator over the guest heap, which keeps them until the guest exits. The same allocator backs the Constantine archive's C `malloc` through `zkvm_provide.setAllocator`, which `src/main.zig` installs before anything runs. The KZG trusted-setup context of the blob verification is built right after, before the input is read, so it takes the same heap prefix for every request. A request that needs more than the heap exits with code 16.

## Development

```bash
make -C rollup test             # native host tests, over the blocks of the EF zkevm fixtures
make -C rollup compile          # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec             # compile, then run under the ZkC interpreter on a freshly generated sample input
make -C rollup zkc-smoke-exec   # the same run, whose output has to equal the native statement's byte for byte
```

`-Dkeccak-accel` (`KECCAK_ACCEL=` on `make`) and `-Dsecp256k1-backend` (`SECP256K1_BACKEND=`) select the keccak and secp256k1 providers of `zkvm_provide`, as in l2-execution.

### Tests

The native tests build their requests with `test/scenario.zig` from a fixed selection of blocks of the Ethereum Foundation's zkevm fixtures (`tests-zkevm@v21.0.5`, the shared fixtures directory that `make test` passes as `-Dzkevm-fixtures`): the ten consecutive blocks of one case, each with an EIP-155 legacy transaction; a block with a blob transaction; and a block with a 161 KB set-code transaction, whose frame is longer than 20 KB and shorter than a blob. A conflation takes its block hashes, parent hash, numbers, timestamp and senders from the fixture; its canonical payload is compressed with libzstd against the dictionary into the DA stream, which is cut into blob and calldata chunks as a plan says (shared blob boundaries included) with the commitments and proofs of Constantine, and one l2-execution proof is made per conflation. Everything is computed with the test-side encoders of `lineth_da` and put into the wire types of `rollup_ssz`.

`make test` runs `test/*_test.zig`, which need no fixtures (the wire format and the exit codes), and `test/reference/*_test.zig`: `rollup_test.zig` asserts the public inputs of accepted requests; `rejection_test.zig` breaks every rule of the statement in an otherwise valid request and asserts the error and its exit-code category; `ef_rollup_test.zig` builds requests from the Amsterdam blocks of the whole corpus: each run of consecutive valid blocks of a fixture, which a block with a blob transaction ends, is grouped into conflations and published as calldata and blob chunks, alone and sharing a blob with neighbouring proofs. Every such request is accepted with the public inputs the fixture determines, and a tampered copy of each is rejected with the error of the tampered rule. Without `-Dzkevm-fixtures` the `test/reference` suites are skipped with a warning.

`tools/gen_fixture.zig` writes the sample request (`zig-out/sample.request.ssz`) and the output `rollup.runFramed` computes for it natively (`zig-out/sample.output.bin`): one conflation of the chain's first three blocks published in one blob; the generator reads them from the fixtures directory it is given. `make exec` runs the guest on the request under the ZkC interpreter. `make zkc-smoke-exec` does so and requires the `guest_output` zkc prints to equal the native output byte for byte; CI runs `make zkc-smoke-external ELF=...` on the prebuilt ELF.
