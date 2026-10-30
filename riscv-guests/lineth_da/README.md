# lineth_da

Data-availability building blocks for the RISC-V guests, packaged as the `lineth_da` Zig module. `src/root.zig` is the package module, one namespace per source file; guests consume the package as a path dependency.

The `zstd` namespace decompresses frames, the `encoding` namespace holds the byte formats of the DA stream. A guest that uses `encoding` links the `zkvm_*` crypto providers of `lineth_accelerators` (`-Dzkvm-provide=true`), which supply sha256, keccak256, secp256k1 recovery and the KZG point evaluation.

## `zstd`

Decompression of single zstd frames against a dictionary, over upstream libzstd v1.5.6 (the unmodified release tarball pinned in `build.zig.zon`).

```zig
const decoder = try lineth_da.zstd.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21);
defer decoder.deinit();
const bytes = try decoder.decompress(frame, out_buffer); // out_buffer.len bounds the output
```

- **Dictionary required.** `Decoder.init` takes the dictionary as runtime bytes and references them for the decoder's lifetime. Only a well-formed zstd dictionary is accepted. `lineth_da.dictionary_v2025_04_21` embeds `dictionaries/v2025-04-21.bin`; a host can pass bytes loaded from anywhere else.
- **One complete frame.** `decompress` accepts exactly one frame: an input that ends early, an input with bytes after the frame, and output beyond `out.len` are rejected. A frame that declares a dictionary ID other than the dictionary's own is rejected; a frame that declares none decodes against the supplied dictionary. A frame declaring a window beyond `zstd.max_window_size` (libzstd's streaming-decoder default of 2^27 bytes) is rejected, so every accepted frame decodes in a default streaming decoder.
- **Categorized errors.** `zstd.Error` names the rejection: `OutOfMemory`, `InvalidDictionary`, `DictionaryMismatch`, `TruncatedFrame`, `TrailingBytes`, `MalformedFrame`, `OutputTooLarge`.
- **Allocator.** libzstd allocates through `ZSTD_customMem` backed by the Zig allocator passed to `Decoder.init`, and only there: `decompress` allocates nothing and a decoder is reusable across frames, including after a rejected one.

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
- Senders are recovered with zesu's `recoverSender` under the Amsterdam rules and the `chain_id` argument. A legacy transaction with an EIP-155 signature commits to a chain ID, which has to be the argument.
- The block is canonical RLP, `[header, transactions, ommers, withdrawals]`, with all 23 Amsterdam header fields, and ends with the block. Blob and set-code transactions name a destination. Anything else is `error.MalformedBlock`; a sender that cannot be recovered is `error.SenderRecoveryFailed`.
- The result is allocated from `output`; `scratch` takes the decoded transactions and can be reset between blocks. Both are arenas.

**`blob_payload`** packs a payload, a 0xff terminal and zero padding into the 4096 elements of a blob, 254 bits per element (`pack`), and unpacks a blob back (`unpack`). A blob unpacks only when it is the canonical packing of its payload: every element is below 2^254 and the last nonzero byte of the stream is the terminal.

**`rolling_hash`** is the dataRollingHash fold, `keccak256(parent ‖ chunkHash)`, with the opening of the boundary preimage for a range that starts inside a chunk.

**`blob_kzg`** verifies an EIP-4844 blob proof without recomputing the commitment: the versioned hash of the commitment (`versionedHash`), the Fiat-Shamir challenge `z` (`computeChallenge`), the blob polynomial's value `y` at `z` by the barycentric formula over the bit-reversed 4096th roots of unity (`evaluate`) and the point-evaluation check of (commitment, `z`, `y`, proof) (`verify`). The field arithmetic is the `bls_scalar` module (`src/bls_scalar.zig`), built for speed in every build mode but a debug one.

## Dictionary

`dictionaries/v2025-04-21.bin` is a byte copy of `jvm-libs/linea/blob-compressor/src/main/resources/compressor-dictionaries/v2025-04-21.bin` (64 KiB, zstd dictionary format).

## Development

```bash
make -C lineth_da test      # native unit tests
make -C lineth_da compile   # statically-linked rv64im smoke ELFs at zig-out/bin/{zstd,blob,truncation}_smoke
make -C lineth_da exec      # compress README.md with the dictionary, then decompress it under the ZkC interpreter
make -C lineth_da exec SMOKE=blob INPUT=blob.ssz # verify a blob proof under the ZkC interpreter
```

The smoke guests (`test/smoke/`) run one part of the package on the guest target; linking one proves its freestanding object needs no symbol the guest does not define. `zig build -Dsmoke=<name>` links one of them.

- `zstd_smoke` reads a frame with `read_input`, decompresses it against the embedded dictionary and publishes the decompressed bytes through `write_output`; it exits with 0 on success and with 2 to 8 for the `zstd.Error` category. `compress_frame <payload-in> <frame-out> [level]` (`tools/`) produces its input.
- `blob_smoke` reads the 48-byte commitment, the 48-byte proof and the 131072-byte blob, verifies the proof and publishes the versioned hash; it exits with 0 on success, 2 for an input of the wrong size and 3 for a proof that does not verify.
- `truncation_smoke` reads a chain ID as 8 big-endian bytes and a full block's RLP, truncates the block, round-trips the encoded payload through the blob packing and publishes the block hash, the parent hash and the payload; it exits with 0 on success and with 2 to 6 for its failure categories.

`truncate_block <chain-id> <block-rlp-file>...` (`tools/`) prints the truncation of full blocks as `ok <timestamp> <block-hash> <parent-hash> <prev-randao> <payload>` lines.

The native tests and tools are built with `-Dhost-tools=true`, which `make test` passes: the tests read the official EIP-4844 reference vectors (`tests/` of c-kzg-4844 v2.1.5, a lazy dependency) and link the host build of the crypto providers, which builds the Constantine archive. A package that depends on `lineth_da` builds the guest side only.
