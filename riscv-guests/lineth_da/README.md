# lineth_da

Data-availability building blocks for the RISC-V guests, packaged as the `lineth_da` Zig module. `src/root.zig` is the package module, one namespace per source file; guests consume the package as a path dependency.

The `zstd_decoder` namespace decompresses frames, the `encoding` namespace holds the byte formats of the DA stream, the `decompress` namespace verifies a stream against the chunks that carry it. A guest that uses `encoding` or `decompress` links the `zkvm_*` crypto providers of `lineth_accelerators` (`-Dzkvm-provide=true`), which supply sha256, keccak256, secp256k1 recovery and the KZG point evaluation.

## `zstd_decoder`

Decompression of the DA frame against a dictionary, over upstream libzstd v1.5.6 (the unmodified release tarball pinned in `build.zig.zon`).

```zig
const decoder = try lineth_da.zstd_decoder.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21);
defer decoder.deinit();
try decoder.decompress(frame, out_buffer); // out_buffer.len is the content size the frame declares
```

- **One frame form.** A DA frame has exactly one form: its header declares the content size, equal to the length of the payload (`out.len`), and the ID of the dictionary; it carries no content checksum; it declares a window of at most `zstd_decoder.max_window_size` (2^27 + 1 bytes, libzstd's streaming-decoder default, so standard libzstd decoders decode the published data); it is complete and ends the input. `decompress` reads the header with `ZSTD_getFrameHeader`, checks these rules, and then decodes the frame in one pass straight into `out` with the dictionary.
- **Dictionary required.** `Decoder.init` takes the dictionary as runtime bytes and references them for the decoder's lifetime. Only a well-formed zstd dictionary that declares an ID is accepted. `lineth_da.dictionary_v2025_04_21` embeds `dictionaries/v2025-04-21.bin`; a host can pass bytes loaded from anywhere else.
- **Categorized errors.** `zstd_decoder.InitError` (`OutOfMemory`, `InvalidDictionary`) names why a decoder cannot be created; `zstd_decoder.FrameError` names why a frame is rejected: `TruncatedFrame` (the input ends early), `TrailingBytes` (bytes after the frame), `MalformedFrame` (not a zstd frame, a skippable frame, or corrupt content), `MissingContentSize`, `ContentSizeMismatch` (the declared content size is not `out.len`), `MissingDictionaryId`, `DictionaryMismatch` (another dictionary's ID), `ChecksumPresent` and `WindowTooLarge`.
- **Allocator.** libzstd allocates through `ZSTD_customMem` backed by the Zig allocator passed to `Decoder.init`: the decoder state and the dictionary when the decoder is created. Decompression allocates nothing, and a decoder is reusable across frames.

## `zstd_encoder`

`zstd_encoder.Encoder` compresses payloads into DA frames on the host: the module is built for native targets and its libzstd compressor sources are linked into native tests and tools only.

```zig
const encoder = try zstd_encoder.Encoder.init(allocator, lineth_da.dictionary_v2025_04_21); // digests the dictionary once
defer encoder.deinit();
const frame = try encoder.compress(frame_allocator, payload); // the caller owns the frame
```

The settings are fixed: level 3 in one pass, so the frame declares its content size, the dictionary ID in the header and no content checksum. libzstd's window for level 3 is at most 2^21 bytes, within the decoder's bound. `dictionary` has to be a well-formed zstd dictionary that declares an ID (`error.InvalidDictionary`) and stays alive as long as the encoder, which, like the decoder, allocates through the Zig allocator it is given.

### Builds

| Target | libzstd sources | C library |
| --- | --- | --- |
| freestanding rv64im (guest) | decoder only | none: `src/freestanding_c` supplies the two headers libzstd includes (`string.h`, `stdlib.h`); the `mem*` routines come from Zig's compiler runtime and libzstd's default allocation path reports exhaustion |
| native (tests, tools) | decoder, and the single-threaded compressor behind `zstd_encoder` (`src/zstd_encoder.zig`) | the host libc |

The C declarations the package and the encoder use (types, constants, signatures) are translated from libzstd's own headers, `zstd.h` with `ZSTD_STATIC_LINKING_ONLY` and `zstd_errors.h`, once per target by `build.zig` and imported as `zstd_c`. The decoder and the encoder share the `ZSTD_customMem` hooks over a Zig allocator (`src/zstd_memory.zig`, imported as `zstd_memory`).

The decoder sources are compiled with `-ffunction-sections -fdata-sections`, so the linker keeps only what the guest calls, and with `-DZSTD_DISABLE_ASM`, so the portable decoder loops replace libzstd's x86-64 assembly on every target.

## `encoding`

Four parts, each its own file under `src/encoding/`.

**`truncation`** turns full Amsterdam block RLPs into the blocks of the DA payload and serializes them.

```zig
const block = try encoding.truncation.truncateBlockRlp(output, scratch, block_rlp, chain_id);
const payload = try encoding.truncation.encodeBlocks(output, scratch, &.{block}); // the bytes a conflation's zstd frame decompresses to
```

- A `TruncatedBlock` has the timestamp, the block hash (keccak256 of the header RLP), the parent hash, the prevRandao, the transactions without signature and chain ID, and the senders. The legacy form is `[nonce, gasPrice, gas, to, value, data]`; a typed transaction keeps its type byte and its field list without chain ID, yParity, r and s (an EIP-7702 authorization list keeps the signatures of its tuples).
- Senders are recovered with zesu's `recoverSender` under the Amsterdam rules and the `chain_id` argument, which is positive. A legacy transaction's `v` is 27 or 28 (an unprotected signature) or 35 or 36 above twice the chain ID (an EIP-155 signature, which commits to the chain ID, the argument).
- The block is canonical RLP, `[header, transactions, ommers, withdrawals]`, with all 23 Amsterdam header fields, and ends with the block. The block's timestamp is a 64-bit integer; an ommer is a header whose timestamp is a 256-bit integer. The transactions are the ones the header's transactions root commits to: the trie over the wire form of the entries of the transaction list, key `RLP(index)`. A transaction is a legacy list or an access-list, fee-market or set-code envelope (types 1, 2 and 4), so a blob transaction (type 3) is rejected. The blocks are ones an l2-execution proof executed, so their transactions are valid Ethereum transactions and their fields are read as decoded. Every length an RLP item declares is checked against the bytes that remain before the item is decoded, to the depth of a transaction. Anything else is `error.MalformedBlock`; a sender that cannot be recovered, any other `v` of a legacy transaction, or a chain ID of zero is `error.SenderRecoveryFailed`; an allocator that refuses a request, in any step, is `error.OutOfMemory`.
- `truncateBlockRlp` and `encodeBlocks` allocate their result from `output`; `scratch` takes the decoded transactions and the encodings of the nested lists and can be reset between calls. Both are arenas: a rejection leaves its allocations behind.

**`blob_payload`** packs a payload, a 0xff terminal and zero padding into the 4096 elements of a blob, 254 bits per element (`pack`), and unpacks a blob back (`unpack`). A blob unpacks only when it is the canonical packing of its payload: every element is below 2^254 and the last nonzero byte of the stream is the terminal.

**`rolling_hash`** is the dataRollingHash fold, `keccak256(parent ‖ chunkHash)`, with the opening of the boundary preimage for a range that starts inside a chunk.

**`blob_kzg`** verifies an EIP-4844 blob proof without recomputing the commitment: the versioned hash of the commitment (`versionedHash`), the Fiat-Shamir challenge `z` (`computeChallenge`), the blob polynomial's value `y` at `z` by the barycentric formula over the bit-reversed 4096th roots of unity and the point-evaluation check of (commitment, `z`, `y`, proof) (`verify`). The field arithmetic is Constantine's BLS12-381 scalar field: `ctt_bls12_381_fr_*`, called through `src/encoding/scalar_field.zig` and defined by the archive of the `zkvm_provide` providers, which every consumer links for sha256 and the point evaluation.

## `decompress`

`decompress.verifyDaChunks` verifies the DA stream of a rollup proof against the chunks that carry it. The stream is the segments `[4-byte big-endian nonzero length][zstd frame]`, one per conflation.

```zig
const verified = try lineth_da.decompress.verifyDaChunks(
    allocator, decoder, chunks, canonical_payloads,
    parent_data_rolling_hash, parent_data_tail_take_bytes, boundary_prev_data_rolling_hash,
);
```

- **Chunk binding.** A `Chunk` is a blob (131072 physical bytes, a 48-byte commitment and a 48-byte proof, no calldata) or a calldata submission (no blob, commitment or proof). A blob is the canonical packing of its payload and the versioned hash of its commitment is the `chunk_hash`; calldata hashes to the `chunk_hash` with keccak256.
- **Layout.** The chunk payloads concatenate into the stream; a positive `parent_data_tail_take_bytes` makes the first blob contribute only the last that many bytes of its payload. A blob followed by another blob holds a full payload (130047 bytes); a positive tail count needs a first blob with a longer payload; every chunk owns at least one byte inside the parsed segments; every calldata chunk ends at a segment boundary. `final_data_tail_discard_bytes` is the number of stream bytes after the last segment.
- **Segments.** One segment per entry of `canonical_payloads`: a nonzero length that stays inside the stream, then one frame in the DA form whose content size is the length of that payload and that decompresses with the decoder's dictionary to exactly those bytes.
- **dataRollingHash.** `end_data_rolling_hash` folds the chunk hashes onto `parent_data_rolling_hash`; a positive tail count starts the range inside the first chunk, which the parent hash includes, so `boundary_prev_data_rolling_hash` has to open it.
- **KZG.** Once every chunk is bound, each blob's EIP-4844 proof is verified against the blob and its commitment (`blob_kzg.verify`), in chunk order and before the layout is parsed: a blob is bound to its commitment, and so to the data L1 anchors.
- **Categorized errors.** `decompress.Error` names the rejection. Chunk binding: `NoChunks`, `MalformedChunk`, `NonCanonicalBlob`, `ChunkHashMismatch`, `BlobKzgMismatch`, `BoundaryPreimageMismatch`. Layout: `PartialBlobBeforeBlob`, `InvalidTailTake`, `MissingSegmentLength`, `InvalidSegmentLength`, `ChunkOwnsNoBytes`, `CalldataEndsMidSegment`. Decompression: `InvalidFrame` (the segment is not one frame in the DA form for its payload: any `zstd_decoder.FrameError`), `PayloadMismatch` (the frame decodes to other bytes of the payload's length). And `OutOfMemory`.
- **Allocator.** The working buffers (the joined stream, one unpacked blob, the decompressed segment) are released before `verifyDaChunks` returns.

## Dictionary

`dictionaries/v2025-04-21.bin` is a byte copy of `jvm-libs/linea/blob-compressor/src/main/resources/compressor-dictionaries/v2025-04-21.bin` (64 KiB, zstd dictionary format).

## Development

```bash
make -C lineth_da test      # native unit tests
make -C lineth_da compile   # statically-linked rv64im smoke ELFs at zig-out/bin/{zstd,blob,truncation,decompress}_smoke
make -C lineth_da exec      # compress a sample conflation payload with the dictionary, then decompress it under the ZkC interpreter
make -C lineth_da exec SMOKE=blob       # verify the proof of a blob of the official reference vectors under the ZkC interpreter
make -C lineth_da exec SMOKE=decompress # verify a one-blob DA stream of the sample payload under the ZkC interpreter
```

The smoke guests (`test/smoke/`) run one part of the package on the guest target; linking one proves its freestanding object needs no symbol the guest does not define. `zig build -Dsmoke=<name>` links one of them.

- `zstd_smoke` reads the payload's length (4 big-endian bytes) and a frame with `read_input`, decompresses the frame against the embedded dictionary and publishes the decompressed bytes through `write_output`; it exits with 0 on success, 2 for an input without a length and with 3 to 13 for the `zstd_decoder.Error` category. `compress_frame` (`tools/`) produces its input.
- `blob_smoke` reads the 48-byte commitment, the 48-byte proof and the 131072-byte blob, verifies the proof and publishes the versioned hash; it exits with 0 on success, 2 for an input of the wrong size and 3 for a proof that does not verify. `blob_sample <input-out>` (`tools/`) produces its input: a case of the official EIP-4844 reference vectors (c-kzg-4844 v2.1.5) whose proof verifies.
- `decompress_smoke` reads the parent dataRollingHash, one conflation's canonical payload and one chunk (a blob or calldata), verifies the stream and publishes the end dataRollingHash and the unconsumed tail count; it exits with 0 on success, 2 for malformed input and with 3 to 10 for the `decompress.Error` category. `da_sample <input-out> [blob|calldata [<payload-in>]]` (`tools/`) produces its input: the payload compressed against the dictionary, published in one chunk. A blob chunk carries the commitment and the blob proof that Constantine computes, so the run verifies the blob's proof.
- `truncation_smoke` reads a chain ID as 8 big-endian bytes and a full block's RLP, truncates the block, round-trips the encoded payload through the blob packing and publishes the block hash, the parent hash and the payload; it exits with 0 on success and with 2 to 6 for its failure categories.

`compress_frame <input-out> <fixtures-dir> [<payload-in>]` and `da_sample <input-out> <fixtures-dir> [blob|calldata [<payload-in>]]` take the canonical payload of a conflation of three selected blocks (a legacy, a fee-market and a set-code transaction) of the EF zkevm fixtures in `fixtures-dir`, the directory that holds `blockchain_tests/`, unless a `payload-in` file gives it; `make exec` passes the shared fixtures directory.

`truncate_block <chain-id> <block-rlp-file>...` (`tools/`) prints the truncation of full blocks as `ok <timestamp> <block-hash> <parent-hash> <prev-randao> <payload>` lines.

The native tests and tools are built with `-Dhost-tools=true`, which `make test` passes: the tests read the official EIP-4844 reference vectors (`tests/` of c-kzg-4844 v2.1.5, a lazy dependency), compute blob commitments and proofs with Constantine's KZG prover, truncate every valid block of the Ethereum Foundation's zkevm fixtures (`tests-zkevm@v21.0.5`, the shared fixtures directory that `make test` passes as `-Dzkevm-fixtures`) and compare the result with the hash, parent hash, prevRandao, timestamp, senders and stripped transactions the fixtures list, and each case's encoded payload with the specified layout built from the same facts; edit the bytes of selected fixture blocks one place at a time and check what `truncation` rejects; compress, decompress and verify the canonical payloads of a fixed selection of fixture blocks in the suites of `test/reference/` (which run when `-Dzkevm-fixtures` is given and are skipped, with a warning, otherwise), and link the host build of the crypto providers, which builds the Constantine archive with the full trusted setup embedded. The selection (`test/support/ef_conflations.zig`) names every block by fixture file, case and position, so each machine reads the same blocks: a legacy, an access-list, a fee-market and a set-code transaction, a block with 32 KiB of calldata, one with twenty transactions, an empty block, a block whose access list has 1397 entries, a block of 257 KiB and a block with a blob transaction; `test/reference/ef_selection_test.zig` checks that the blocks have what they are picked for. A package that depends on `lineth_da` builds the guest side only.
