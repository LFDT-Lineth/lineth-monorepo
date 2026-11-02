# lineth_da

Data-availability building blocks for the RISC-V guests, packaged as the `lineth_da` Zig module. `src/root.zig` is the package module, one namespace per source file; guests consume the package as a path dependency.

The `zstd` namespace decompresses frames, the `encoding` namespace holds the byte formats of the DA stream, the `decompress` namespace verifies a stream against the chunks that carry it. A guest that uses `encoding` or `decompress` links the `zkvm_*` crypto providers of `lineth_accelerators` (`-Dzkvm-provide=true`), which supply sha256, keccak256, secp256k1 recovery and the KZG point evaluation.

## `zstd`

Decompression of single zstd frames against a dictionary, over upstream libzstd v1.5.6 (the unmodified release tarball pinned in `build.zig.zon`).

```zig
const decoder = try lineth_da.zstd.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21);
defer decoder.deinit();
const bytes = try decoder.decompress(frame, out_buffer); // out_buffer.len bounds the output
```

- **Dictionary required.** `Decoder.init` takes the dictionary as runtime bytes and references them for the decoder's lifetime. Only a well-formed zstd dictionary is accepted. `lineth_da.dictionary_v2025_04_21` embeds `dictionaries/v2025-04-21.bin`; a host can pass bytes loaded from anywhere else.
- **One complete frame.** `decompress` accepts exactly one frame: an input that ends early, an input with bytes after the frame, and output beyond `out.len` are rejected. A frame that declares a dictionary ID other than the dictionary's own is rejected; a frame that declares none decodes against the supplied dictionary. A frame declaring a window beyond `zstd.max_window_size` (2^27 + 1 bytes, libzstd's streaming-decoder default) is rejected.
- **Streaming decoder.** The frame goes through `ZSTD_decompressStream` with `out` as its history (`ZSTD_d_stableOutBuffer`). A frame that declares its content size decodes in one pass. A frame without content size decodes block by block, each block holds at most `min(window, 128 KiB)` bytes, and the window covers the decompressed content: a frame whose window is smaller than its content is rejected, so every decoder keeps all the output a match can reach.
- **Categorized errors.** `zstd.Error` names the rejection: `OutOfMemory`, `InvalidDictionary`, `DictionaryMismatch`, `TruncatedFrame`, `TrailingBytes`, `MalformedFrame`, `OutputTooLarge`.
- **Allocator.** libzstd allocates through `ZSTD_customMem` backed by the Zig allocator passed to `Decoder.init`: the decoder state and the dictionary when the decoder is created, and one input buffer of at most 128 KiB when the first frame without content size is decoded, which the decoder keeps. A decoder is reusable across frames, including after a rejected one.

### Builds

| Target | libzstd sources | C library |
| --- | --- | --- |
| freestanding rv64im (guest) | decoder only | none: `src/freestanding_c` supplies the two headers libzstd includes (`string.h`, `stdlib.h`); the `mem*` routines come from Zig's compiler runtime and libzstd's default allocation path reports exhaustion |
| native (tests, tools) | decoder, and the single-threaded compressor behind `test/zstd_compressor.zig` | the host libc |

The decoder sources are compiled with `-ffunction-sections -fdata-sections`, so the linker keeps only what the guest calls, and with `-DZSTD_DISABLE_ASM`, so the portable decoder loops replace libzstd's x86-64 assembly on every target.

## `encoding`

Four parts, each its own file under `src/encoding/`.

**`truncation`** turns full Amsterdam block RLPs into the blocks of the DA payload and serializes them.

```zig
const block = try encoding.truncation.truncateBlockRlp(output, scratch, block_rlp, chain_id);
const payload = try encoding.truncation.encodeBlocks(allocator, &.{block}); // the bytes a conflation's zstd frame decompresses to
```

- A `TruncatedBlock` has the timestamp, the block hash (keccak256 of the header RLP), the parent hash, the prevRandao, the transactions without signature and chain ID, and the senders. The legacy form is `[nonce, gasPrice, gas, to, value, data]`; a typed transaction keeps its type byte and its field list without chain ID, yParity, r and s (an EIP-7702 authorization list keeps the signatures of its tuples).
- Senders are recovered with zesu's `recoverSender` under the Amsterdam rules and the `chain_id` argument, which is positive. A legacy transaction with an EIP-155 signature commits to a chain ID, which has to be the argument.
- The block is canonical RLP, `[header, transactions, ommers, withdrawals]`, with all 23 Amsterdam header fields, and ends with the block. The block's timestamp is a 64-bit integer; an ommer is a header whose timestamp is a 256-bit integer. The transactions are the ones the header's transactions root commits to: the trie over the wire form of the entries of the transaction list, key `RLP(index)`. Each transaction encodes back to exactly the bytes it was decoded from, so every field holds its own value in the width it is decoded to (nonce and gas in 64 bits, fees in 128). Blob and set-code transactions name a destination, and a blob transaction a blob. Every length an RLP item declares is checked against the bytes that remain before the item is decoded, to the depth of a transaction. Anything else is `error.MalformedBlock`; a sender that cannot be recovered, or a chain ID of zero, is `error.SenderRecoveryFailed`.
- The result is allocated from `output`; `scratch` takes the decoded transactions and can be reset between blocks. Both are arenas.

**`blob_payload`** packs a payload, a 0xff terminal and zero padding into the 4096 elements of a blob, 254 bits per element (`pack`), and unpacks a blob back (`unpack`). A blob unpacks only when it is the canonical packing of its payload: every element is below 2^254 and the last nonzero byte of the stream is the terminal.

**`rolling_hash`** is the dataRollingHash fold, `keccak256(parent ‖ chunkHash)`, with the opening of the boundary preimage for a range that starts inside a chunk.

**`blob_kzg`** verifies an EIP-4844 blob proof without recomputing the commitment: the versioned hash of the commitment (`versionedHash`), the Fiat-Shamir challenge `z` (`computeChallenge`), the blob polynomial's value `y` at `z` by the barycentric formula over the bit-reversed 4096th roots of unity (`evaluate`) and the point-evaluation check of (commitment, `z`, `y`, proof) (`verify`). The field arithmetic is the `bls_scalar` module (`src/bls_scalar.zig`, reachable as `lineth_da.bls_scalar`), optimized in every build mode: ReleaseFast, and ReleaseSafe in a Debug build.

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
- **Segments.** One segment per entry of `canonical_payloads`: a nonzero length that stays inside the stream, then one complete frame that decompresses with the decoder's dictionary to exactly that payload.
- **dataRollingHash.** `end_data_rolling_hash` folds the chunk hashes onto `parent_data_rolling_hash`; a positive tail count starts the range inside the first chunk, which the parent hash includes, so `boundary_prev_data_rolling_hash` has to open it.
- **KZG.** Once every chunk is bound, each blob's EIP-4844 proof is verified against the blob and its commitment (`blob_kzg.verify`), in chunk order and before the layout is parsed: a blob is bound to its commitment, and so to the data L1 anchors.
- **Categorized errors.** `decompress.Error` names the rejection. Chunk binding: `NoChunks`, `MalformedChunk`, `NonCanonicalBlob`, `ChunkHashMismatch`, `BlobKzgMismatch`, `BoundaryPreimageMismatch`. Layout: `PartialBlobBeforeBlob`, `InvalidTailTake`, `MissingSegmentLength`, `InvalidSegmentLength`, `ChunkOwnsNoBytes`, `CalldataEndsMidSegment`. Decompression: `InvalidFrame`, `PayloadMismatch`. And `OutOfMemory`.
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

- `zstd_smoke` reads a frame with `read_input`, decompresses it against the embedded dictionary and publishes the decompressed bytes through `write_output`; it exits with 0 on success and with 2 to 8 for the `zstd.Error` category. `compress_frame <frame-out> [<payload-in> [level]]` (`tools/`) produces its input.
- `blob_smoke` reads the 48-byte commitment, the 48-byte proof and the 131072-byte blob, verifies the proof and publishes the versioned hash; it exits with 0 on success, 2 for an input of the wrong size and 3 for a proof that does not verify. `blob_sample <input-out>` (`tools/`) produces its input: a case of the official EIP-4844 reference vectors (c-kzg-4844 v2.1.5) whose proof verifies.
- `decompress_smoke` reads the parent dataRollingHash, one conflation's canonical payload and one chunk (a blob or calldata), verifies the stream and publishes the end dataRollingHash and the unconsumed tail count; it exits with 0 on success, 2 for malformed input and with 3 to 10 for the `decompress.Error` category. `da_sample <input-out> [blob|calldata [<payload-in>]]` (`tools/`) produces its input: the payload compressed against the dictionary, published in one chunk. A blob chunk carries the commitment and the blob proof that c-kzg-4844 computes, so the run verifies the blob's proof.
- `truncation_smoke` reads a chain ID as 8 big-endian bytes and a full block's RLP, truncates the block, round-trips the encoded payload through the blob packing and publishes the block hash, the parent hash and the payload; it exits with 0 on success and with 2 to 6 for its failure categories.

Without a `payload-in` file, `compress_frame` and `da_sample` take the canonical payload of a fixed conflation of three signed blocks.

`truncate_block <chain-id> <block-rlp-file>...` (`tools/`) prints the truncation of full blocks as `ok <timestamp> <block-hash> <parent-hash> <prev-randao> <payload>` lines.

The native tests and tools are built with `-Dhost-tools=true`, which `make test` passes: the tests read the official EIP-4844 reference vectors (`tests/` of c-kzg-4844 v2.1.5, a lazy dependency), compute blob commitments and proofs with c-kzg-4844 over blst (also a lazy dependency), truncate every valid block of the Ethereum Foundation's zkevm fixtures (`tests-zkevm@v21.0.5`, a lazy dependency of 3.4 GB) and compare the result with the hash, parent hash, prevRandao, timestamp and senders the fixtures list, and link the host build of the crypto providers, which builds the Constantine archive. A package that depends on `lineth_da` builds the guest side only.
