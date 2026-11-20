# lineth_da

Data-availability building blocks for the RISC-V guests, packaged as the `lineth_da` Zig module. `src/root.zig` is the package module, one namespace per source file; guests consume the package as a path dependency.

The `zstd_decoder` namespace decompresses frames, the `encoding` namespace holds the byte formats of the DA stream, and the `decompress` namespace verifies a stream against the chunks that carry it. `encoding` and `decompress` hash and verify blob proofs through the `zkvm_accelerators` module of `lineth_accelerators`, and `encoding` recovers transaction senders through zesu. A guest that uses `encoding` or `decompress` links the `zkvm_*` crypto providers of `lineth_accelerators` (`-Dzkvm-provide=true`), which supply sha256, keccak256, secp256k1 recovery and the verification of KZG blob proofs.

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

`zstd_encoder.Encoder` compresses payloads into DA frames on the host: the module is built for native targets and its libzstd compressor sources are linked into native builds only.

```zig
const encoder = try zstd_encoder.Encoder.init(allocator, lineth_da.dictionary_v2025_04_21); // digests the dictionary once
defer encoder.deinit();
const frame = try encoder.compress(frame_allocator, payload); // the caller owns the frame
```

The settings are fixed: level 3 in one pass, so the frame declares its content size, the dictionary ID in the header and no content checksum. libzstd's window for level 3 is at most 2^21 bytes, within the decoder's bound. `dictionary` has to be a well-formed zstd dictionary that declares an ID (`error.InvalidDictionary`) and stays alive as long as the encoder, which, like the decoder, allocates through the Zig allocator it is given. `compress` creates its own libzstd context per call and shares only the digested dictionary, which it reads alone, so any number of threads may compress with one encoder, over an allocator that serves them all.

### Builds

| Target | libzstd sources | C library |
| --- | --- | --- |
| freestanding rv64im (guest) | decoder only | none: `src/freestanding_c` supplies the two headers libzstd includes (`string.h`, `stdlib.h`); the `mem*` routines come from Zig's compiler runtime and libzstd's default allocation path reports exhaustion |
| native (tests) | decoder, and the single-threaded compressor behind `zstd_encoder` (`host/zstd_encoder.zig`) | the host libc |

The C declarations the package and the encoder use (types, constants, signatures) are translated from libzstd's own headers, `zstd.h` with `ZSTD_STATIC_LINKING_ONLY` and `zstd_errors.h`, once per target by `build.zig` and imported as `zstd_c`. The decoder and the encoder share the `ZSTD_customMem` hooks over a Zig allocator (`src/zstd_memory.zig`, imported as `zstd_memory`).

The decoder sources are compiled with `-ffunction-sections -fdata-sections`, so the linker keeps only what the guest calls, and with `-DZSTD_DISABLE_ASM`, so the portable decoder loops replace libzstd's x86-64 assembly on every target.

## `encoding`

Four parts, each its own file under `src/encoding/`.

**`truncation`** turns full Osaka or Amsterdam block RLPs into the blocks of the DA payload and serializes them.

```zig
const block = try encoding.truncation.truncateBlockRlp(output, scratch, block_rlp, chain_id);
const payload = try encoding.truncation.encodeBlocks(output, scratch, &.{block}); // the bytes a conflation's zstd frame decompresses to
```

- A `TruncatedBlock` has the timestamp, the block hash (keccak256 of the header RLP), the parent hash, the prevRandao, the transactions without signature and chain ID, and the senders. The legacy form is `[nonce, gasPrice, gas, to, value, data]`; a typed transaction keeps its type byte and its field list without chain ID, yParity, r and s (an EIP-7702 authorization list keeps the signatures of its tuples).
- Senders are recovered with zesu's `recoverSender` and the `chain_id` argument, which is positive, under the rules every fork since Homestead shares: a signature's `s` is at most half the curve order (EIP-2). A legacy transaction's `v` is 27 or 28 (an unprotected signature) or 35 or 36 above twice the chain ID (an EIP-155 signature, which commits to the chain ID, the argument).
- The block is canonical RLP, `[header, transactions, ommers, withdrawals]`, with all the header fields of Osaka (21, ending with the requests hash) or of Amsterdam (23, which adds the block access list hash and the slot number), and ends with the block. The block's timestamp is a 64-bit integer; an ommer is a header whose timestamp is a 256-bit integer. The transactions are the ones the header's transactions root commits to: the trie over the wire form of the entries of the transaction list, key `RLP(index)`. A transaction is a legacy list or an access-list, fee-market or set-code envelope (types 1, 2 and 4), so a blob transaction (type 3) is rejected. The blocks are ones an l2-execution proof executed, so their transactions are valid Ethereum transactions and their fields are read as decoded. Every length an RLP item declares is checked against the bytes that remain before the item is decoded, to the depth of a transaction. Anything else is `error.MalformedBlock`; a sender that cannot be recovered, any other `v` of a legacy transaction, or a chain ID of zero is `error.SenderRecoveryFailed`; an allocator that refuses a request, in any step, is `error.OutOfMemory`.
- `truncateBlockRlp` and `encodeBlocks` allocate their result from `output`; `scratch` takes the decoded transactions and the encodings of the nested lists and can be reset between calls. Both are arenas: a rejection leaves its allocations behind.

**`blob_payload`** packs a payload, a 0xff terminal and zero padding into the 4096 elements of a blob, 254 bits per element (`pack`), and unpacks a blob back (`unpack`). A blob unpacks only when it is the canonical packing of its payload: every element is below 2^254 and the last nonzero byte of the stream is the terminal.

**`rolling_hash`** is the dataRollingHash fold, `keccak256(parent ‖ chunkHash)`, with the opening of the boundary preimage for a range that starts inside a chunk.

**`blob_kzg`** verifies EIP-4844 blob proofs without recomputing the commitment: the versioned hash of a commitment (`versionedHash`) and the proofs of a batch of blobs against their commitments (`verifyBatch`), by Constantine's `verify_blob_kzg_proof_batch` with the consensus specs' Fiat-Shamir batch weights, behind the `lineth_zkvm_kzg_verify_blob_batch` accelerator. The `zkvm_provide` providers define that accelerator and own the guest's one KZG context, which holds the embedded verification-only trusted setup. A blob element of at least the scalar field's order, an invalid commitment and an invalid proof do not verify; an empty batch verifies.

## `decompress`

`decompress.verifyDaChunks` verifies the DA stream of a rollup proof against the chunks that carry it. The stream is the segments `[4-byte big-endian nonzero length][zstd frame]`, one per conflation.

```zig
const verified = try lineth_da.decompress.verifyDaChunks(
    allocator, decoder, chunks, canonical_payloads,
    parent_data_rolling_hash, parent_data_tail_take_bytes, boundary_prev_data_rolling_hash,
);
```

- **Chunk binding.** A `Chunk` is the witness struct of the rollup input, declared with the `guest_common.ssz` views (`.bytes` of its byte lists, `.value` of `is_calldata`) so the decoded witness is the value verified. It is a blob (131072 physical bytes, a 48-byte commitment and a 48-byte proof, no calldata) or a calldata submission (no blob, commitment or proof). A blob is the canonical packing of its payload and the versioned hash of its commitment is the `chunk_hash`; calldata hashes to the `chunk_hash` with keccak256.
- **Layout.** The chunk payloads concatenate into the stream; a positive `parent_data_tail_take_bytes` makes the first blob contribute only the last that many bytes of its payload. A blob followed by another blob holds a full payload (130047 bytes); a positive tail count needs a first blob with a longer payload; every chunk owns at least one byte inside the parsed segments; every calldata chunk ends at a segment boundary. `final_data_tail_discard_bytes` is the number of stream bytes after the last segment.
- **Segments.** One segment per entry of `canonical_payloads`: a nonzero length that stays inside the stream, then one frame in the DA form whose content size is the length of that payload and that decompresses with the decoder's dictionary to exactly those bytes.
- **dataRollingHash.** `end_data_rolling_hash` folds the chunk hashes onto `parent_data_rolling_hash`; a positive tail count starts the range inside the first chunk, which the parent hash includes, so `boundary_prev_data_rolling_hash` has to open it.
- **KZG.** Once every chunk is bound, the blobs' EIP-4844 proofs are verified against the blobs and their commitments as one batch (`blob_kzg.verifyBatch`), in chunk order and before the layout is parsed: a blob is bound to its commitment, and so to the data L1 anchors.
- **Categorized errors.** `decompress.Error` names the rejection. Chunk binding: `NoChunks`, `MalformedChunk`, `NonCanonicalBlob`, `ChunkHashMismatch`, `BlobKzgMismatch`, `BoundaryPreimageMismatch`. Layout: `PartialBlobBeforeBlob`, `InvalidTailTake`, `MissingSegmentLength`, `InvalidSegmentLength`, `ChunkOwnsNoBytes`, `CalldataEndsMidSegment`. Decompression: `InvalidFrame` (the segment is not one frame in the DA form for its payload: any `zstd_decoder.FrameError`), `PayloadMismatch` (the frame decodes to other bytes of the payload's length). And `OutOfMemory`.
- **Allocator.** The working buffers (the joined stream, one unpacked blob, the decompressed segment) are released before `verifyDaChunks` returns.

## Host library

`host/` is the host layer over the package, built as the shared library `lineth_da` and declared in `include/lineth_da.h`: the conflation compressor and the DA stream a host runs ahead of the guests, with the same truncation, encoder, blob packing and hashes as the guests, so the blobs it seals are the ones `decompress.verifyDaChunks` accepts. `host/` is the producer side of the DA format whose consumer side is `decompress`, so a release of the library pairs with a rollup guest built from the same commit; `lineth_da_abi_version` returns the version of the C ABI the library was built with, which changes with the structs, the status enum or the signatures. `host/root.zig` is the module, one namespace per file.

```zig
const compress = try compressor.Compressor.init(allocator, dictionary, chain_id); // copies the dictionary
defer compress.deinit();
const frame = try compress.compress(block_rlps, null); // the caller owns the frame

var s = try stream.Stream.open(allocator, &.fresh(data_rolling_hash));
defer s.deinit();
const sealed = try s.append(frame, end_block); // the blobs this append sealed; `end_block` is the conflation's last block
defer stream.freeSealed(allocator, sealed);
```

- **`compressor`** truncates the full block RLPs of one conflation under the chain ID, encodes them as the canonical payload and compresses that into one frame. It decodes what it compresses; the numbers of the blocks, their links and the chain ID of their transactions are the caller's to check, and the caller labels the frame with the last block of the conflation when it appends it to a stream. It fails for no blocks, for a block that truncation cannot decode (`InvalidBlockRlp`, with the index of the block and `MalformedBlock` or `SenderRecoveryFailed` as the reason), for bytes that are not a zstd dictionary declaring an ID (`InvalidDictionary`) and for a refused allocation. Any number of threads may `compress` at once on one compressor: the dictionary is read-only and every call has its own arenas and libzstd context. The host layer builds zesu with its keccak memo, an unsynchronized global cache that only pays off inside a zkVM, compiled out.
- **`stream`** appends the segments `[4-byte big-endian length][frame]` of the conflations in order, packs them into blobs of 130047 payload bytes and seals each with its KZG commitment and proof, the versioned hash (`chunk_hash`) and the data rolling hash. A frame is opaque bytes of at least one byte, short enough for its length prefix. A blob is sealed lazily: a full blob stays open until the next segment needs room or `flush`. A sealed blob lists the cuts in it, each the end of a conflation's segment with the payload bytes that follow it (`unused_tail_bytes`). A stream opens at a `Position`, the end of the last proven rollup; inside a shared blob it checks that the blob is the canonical packing of a payload longer than the tail and folds from the parent hash to the data rolling hash, commits to it once, and requires the appended bytes to reproduce its tail (`SeededBlobMismatch`) before a `flush` is allowed. An append stages everything fallible before it changes the stream, so a failed append leaves the stream as it was. One thread at a time uses a stream; streams are independent.
- **`kzg_prover`** commits to blobs and proves them by Constantine's prover, called through the declarations that `lineth_accelerators` translates from Constantine's headers (`constantine_c`), over one embedded trusted-setup context, built on first use under a mutex and shared by every thread afterwards.
- **`c_api`** exports the functions of the header. A call returns a status, `OK` to `INTERNAL` in the order of `lineth_da_status`, and writes a NUL-terminated message into the buffer the call is given, cut to its size. Results are copied out: the library owns a `lineth_da_compressed` or `lineth_da_sealed_blobs` until its `_free` call, keeps no pointer of the caller past a call, and the handles of a compressor and a stream end with `lineth_da_compressor_free` and `lineth_da_stream_close`.

### Builds

`make host-lib` builds `zig-out/lib/liblineth_da.so` (`.dylib` on macOS) and installs `zig-out/include/lineth_da.h`: ReleaseSafe, stripped, for the machine that builds it, with a baseline CPU model, so the library does not depend on the CPU features of the machine, and on Linux glibc 2.39. libzstd is compiled with hidden visibility, and on Linux lld applies the version script `host/lineth_da.map`, which exports the `lineth_da_*` functions and nothing else. The Constantine archive is built by the host's Nim and clang, so a Linux library needs the glibc of the machine that built it.

| Make target | Result |
| --- | --- |
| `host-lib` | the library and the header |
| `host-lib-check` | on Linux, fails unless `nm -D` lists `lineth_da_*` symbols alone and the newest `GLIBC_x.y` of `objdump -T` is at most 2.39; elsewhere skipped |

## Dictionary

`dictionaries/v2025-04-21.bin` is a byte copy of `jvm-libs/linea/blob-compressor/src/main/resources/compressor-dictionaries/v2025-04-21.bin` (64 KiB, zstd dictionary format).

## Development

```bash
make -C lineth_da test      # native unit tests, and the shared library of the host layer built and checked
make -C lineth_da host-lib  # the shared library of the host layer
```

The native tests are built with `-Dhost-tools=true`, which `make test` passes: the tests read the official EIP-4844 reference vectors (`tests/` of c-kzg-4844 v2.1.5, a lazy dependency), compute blob commitments and proofs with Constantine's KZG prover, truncate every valid block of the Ethereum Foundation's zkevm fixtures (`tests-zkevm@v21.0.5`, the shared fixtures directory that `make test` passes as `-Dzkevm-fixtures`) and compare the result with the hash, parent hash, prevRandao, timestamp, senders and stripped transactions the fixtures list, and each case's encoded payload with the specified layout built from the same facts; edit the bytes of selected fixture blocks one place at a time and check what `truncation` rejects; compress, decompress and verify the canonical payloads of a fixed selection of fixture blocks in the suites of `test/reference/` (which run when `-Dzkevm-fixtures` is given and are skipped, with a warning, otherwise), and link the host build of the crypto providers, which builds the Constantine archive with the full trusted setup embedded. The selection (`test/support/ef_conflations.zig`) names every block by fixture file, case and position, so each machine reads the same blocks: a legacy, an access-list, a fee-market and a set-code transaction, a block with 32 KiB of calldata, one with twenty transactions, an empty block, a block whose access list has 1397 entries, a block of 257 KiB and a block with a blob transaction; `test/reference/ef_selection_test.zig` checks that the blocks have what they are picked for. A package that depends on `lineth_da` builds the guest side only, unless it passes `-Dhost-tools=true`, which also exposes the modules the tests of a consumer build real DA streams with: `support` (the byte-level pieces of a DA stream and the blob commitments and proofs of Constantine), `zstd_encoder` (the package's frame encoder) and `ef_blocks` (the cases of the EF zkevm fixtures with the facts the fixtures list for the blocks they accept, hex-decoded). The three are compiled at the `-Doptimize` the consumer passes, where ReleaseSmall becomes ReleaseSafe. `test/host/` tests the host layer, one subject per file: the compressor, the stream and its resumes, the guest's acceptance of what the host emits, the C ABI against its header, threads, and the KZG prover against the reference vectors. The tests that need real blocks take them from the selection and are skipped without `-Dzkevm-fixtures`; the others, which feed the stream synthetic frames and random payloads, run without it. `test/reference/` checks the package against the facts of the EF corpus.
