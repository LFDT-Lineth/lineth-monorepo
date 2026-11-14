# Rollup Guest

The RISC-V guest program of the rollup proof (`rollup_spec/Readme.md` §2.2). It decodes the canonical `RollupProofPrivateInput` SSZ envelope (schema id `0x0004`), runs the rollup statement over it and emits the rollup public inputs (schema id `0x0005`: `keccak256(frame) || frame`, where `frame` is the schema id followed by the SSZ public inputs). The input codec is `src/rollup_ssz.zig`; the output codec and the l2-execution proofs the input embeds are the `core_serde` wire contracts of `guest_common`.

## Statement

One rollup proof covers N consecutive whole conflations, each paired with its l2-execution proof, over the chunks (blobs and calldata submissions) of the data-availability stream that carries their compressed payloads. `src/rollup.zig` runs these parts of §2.2 in this order:

- **Canonical payloads** (steps 1 and 3; `src/canonical.zig`). Every conflation's full block RLPs are truncated and encoded into the payload its zstd frame decompresses to (`lineth_da.encoding.truncation`); the block count has to match the paired proof's range, and the senders recovered from the signed transactions have to hash to the proof's `txFromsHash`.
- **Data availability** (step 2; `lineth_da.decompress`). Every chunk is bound to its `chunkHash`, every blob's EIP-4844 proof opens its commitment to the blob, the stream is parsed under the four layout rules into one segment per conflation, every segment's zstd frame decompresses with the embedded dictionary `v2025-04-21` to its conflation's canonical payload, and the dataRollingHash folds over the chunks. `endDataRollingHash` and `finalDataTailDiscardBytes` come out of it.
- **Tiling** (the witness `E₁ … Eₙ`; `src/chain.zig`). Every proof starts right after the end of the proof before it.
- **Recursion** (step 4; `src/recursion.zig`). Every l2-execution proof is verified under its program VK against the hash of its public input by the devmode system of `verifier-ray`: the proof is the image of one devmode proof, as `devmode.Encode` writes it for `devmode.GuestBase`. The verified statement is 40 cells the guest builds itself, never read from the image: the 32 bytes of the hash of the proof's public input, one cell each, then the 8 limbs of the program VK, each 4 big-endian bytes and a canonical KoalaBear element. A malformed image, a non-canonical limb and a failed verification each exit with code 11.
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
| 3 | the SSZ body is not the canonical encoding within the bounds of the wire schema |
| 4 | the request has no conflation, chunk or proof, or the conflations and proofs do not pair one to one |
| 5 | the output encoder rejected the public inputs: a list exceeds the bound of the output's wire schema |
| 6 | blocks and senders: a block is not accepted, a sender is not recoverable, a conflation's block count differs from its proof's range, the senders do not hash to `txFromsHash` |
| 7 | chunk binding: a chunk contradicts its kind, is not the canonical packing, does not hash to its `chunkHash`, or the boundary preimage does not open the parent dataRollingHash |
| 8 | a blob's EIP-4844 proof does not open its commitment |
| 9 | stream layout: a layout rule is broken, or a segment's length is missing, zero or beyond the stream |
| 10 | decompression: a segment is not one complete frame of the dictionary, or decompresses to another payload |
| 11 | recursion: a proof is rejected by the recursive verifier, or its public input is beyond a bound of its wire schema |
| 12 | block-hash chain: an end block hash or a parent hash does not match the conflations' blocks |
| 13 | tiling and continuity between consecutive proofs |
| 14 | messaging offsets: a proof's `block_count` differs from its block range, or an offset is out of order, out of range or exceeds 16 bits once rebased |
| 15 | an allocation exceeds the guest heap, whether it builds the KZG context, decodes the input, runs the statement or encodes the output |

## Allocation

The guest serves every allocation from one bump allocator over the guest heap, which keeps them until the guest exits. The same allocator backs the Constantine archive's C `malloc` through `zkvm_provide.setAllocator`, which `src/main.zig` installs before anything runs. The KZG trusted-setup context of the blob verification is built right after, before the input is read, so it takes the same heap prefix for every request. A request that needs more than the heap exits with code 16.

The recursive verifier allocates once, for the whole request: one 8-byte-aligned copy buffer, sized to the longest proof image of the input, which every proof is copied into to be relocated in place (the image sits at an arbitrary offset inside the input), and the verifier's workspace of 1,560 bytes.

## Development

```bash
make -C rollup test             # native host tests, over curated EF zkevm fixture blocks and a Linea mainnet window
make -C rollup compile          # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec             # compile, then run under the ZkC interpreter on a freshly generated sample input
make -C rollup zkc-smoke-exec   # the same run, whose output has to equal the native statement's byte for byte
```

`-Dkeccak-accel` (`KECCAK_ACCEL=` on `make`) and `-Dsecp256k1-backend` (`SECP256K1_BACKEND=`) select the keccak and secp256k1 providers of `zkvm_provide`, as in l2-execution.

### Tests

The native tests build their requests with `test/scenario.zig` from two sources of real blocks, both pinned there:

- **EF zkevm fixtures** (`tests-zkevm@v21.0.5`, the shared fixtures directory that `make test` passes as `-Dzkevm-fixtures`; chain ID 1). The corpus is EVM-focused and not representative of what a rollup proves, so the tests use 12 blocks of it: one block per set of transaction types (legacy, fee-market, set-code, and their pairs), one with an access list, one with a contract creation, one with EIP-7702 authorizations, the block with the most transactions (1,020; it compresses poorly), the largest block that compresses well (103 KB of deposits), a block with a blob transaction (which the guest rejects), and ten consecutive blocks of one case. `scenario.zig` pins each as file, case and block index, and loading checks it is what it claims (accepted by its fixture, its transaction types, its chain linkage).
- **A Linea mainnet window** (`test/testdata/linea_mainnet_27434569_27434718.json.gz`; chain ID 59144): 150 consecutive blocks with 2,576 legacy and fee-market transactions and 582 KB of block RLP, in the gzip of one case in the shape of the EF blockchain tests. Its stream is 72 KB compressed, and 360 KB with the segments' frames stored, which gives the tests a stream across two full blobs. `tools/fetch_linea_blocks.zig` builds it from the public RPC: it rebuilds every header and transaction from the RPC's objects and checks them against the reported block and transaction hashes, then gives each header the two fields Amsterdam adds to the Prague header (the hash of an empty block access list and slot number 0), which the guest's header parser requires, and relinks the parent hashes; the hashes in the file are those of the reshaped headers, except the first block's parent hash. The guest's own truncation has to accept every reshaped block: it checks the transactions root and recovers the senders, which are compared with the RPC's. `make fetch-linea-blocks` reruns it.

A conflation takes its block hashes, parent hash, numbers, timestamp and senders from the fixture; its canonical payload is compressed with libzstd against the dictionary into the DA stream, which is cut into blob and calldata chunks as a plan says (shared blob boundaries included) with the commitments and proofs of Constantine, and one l2-execution proof is made per conflation. Everything is computed with the test-side encoders of `lineth_da` and put into the wire types of `rollup_ssz`; a request carries the chain ID of its source.

Every l2-execution proof of a request is a real devmode proof image for the hash of its public input and its program VK, a canonical one (`0x2a` in every byte unless a test names another). `test/devmode_prover.zig` obtains them from `verifier-ray/codegen/generate-devmode-system`, a Go program that `zig build` compiles for the host (`go` has to be on `PATH`) and runs with `-prove`: one child process per test binary, spawned on first use, reads one request per line from stdin, the 32-byte hash and the 32-byte program VK as hex separated by a space, and writes the proof image of each, behind its length as 4 big-endian bytes, to stdout. `make` generates the devmode system the guest verifies against (`verifier-ray/testdata/generated/devmode_system.zig`, a gitignored file) when it is missing. A test that edits a public input of a built request proves it again (`scenario.prove`) before running it, so that the rule under test is the one that rejects; the tests of the recursion step run the edited request as it is.

`make test` runs `test/*_test.zig`, which need no fixtures (the input frame, the exit codes and `recursion_test.zig`, which verifies devmode proofs under the statements they were and were not proved for, and rejects a changed round commitment, a truncated image and a non-canonical VK limb), and `test/reference/*_test.zig`: `rollup_test.zig` asserts the public inputs of accepted requests, each accepted EF block as a conflation of its own, the EF chain in several conflations, and the Linea window in several conflations published as calldata, as a range starting inside the first of two blobs, and as a stream across two full blobs and a third; `rejection_test.zig` breaks every rule of the statement in an otherwise valid request and asserts the error and its exit-code category, the recursive verifier's included (a changed round commitment, a proof of another conflation's public input, a non-canonical VK limb); `ef_rollup_test.zig` sweeps the curated blocks: every EF block alone and the EF chain in calldata and blob chunk layouts (one blob, a blob into calldata, a blob shared with neighbouring proofs, calldata cut at a segment boundary), and the Linea window in calldata, as a range in the first of two blobs and across full blobs. Every such request is accepted with the public inputs the fixture determines, and a copy tampered with each of six rules (the senders hash, the end block hash, the parent block hash, the chunk hash, a block RLP, the content of the first chunk) is rejected with the error of that rule. The tests that tamper with a proof image change a byte of a round commitment, located through the verifier's own types: most other bytes of an image are padding or the image's copy of the public inputs, which the verifier does not read. Without `-Dzkevm-fixtures` the `test/reference` suites are skipped with a warning.

`tools/gen_fixture.zig` writes the sample request (`zig-out/sample.request.ssz`), carrying real devmode proofs, and the output `rollup.runFramed` computes for it natively (`zig-out/sample.output.bin`): one conflation of the chain's first three blocks published in one blob; the generator reads them from the fixtures directory it is given. `make exec` runs the guest on the request under the ZkC interpreter. `make zkc-smoke-exec` does so and requires the `guest_output` zkc prints to equal the native output byte for byte; CI runs `make zkc-smoke-external ELF=...` on the prebuilt ELF.
