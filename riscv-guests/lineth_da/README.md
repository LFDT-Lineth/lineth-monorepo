# lineth_da

Data-availability building blocks for the RISC-V guests, packaged as the `lineth_da` Zig module. `src/root.zig` is the package module, one namespace per source file; guests consume the package as a path dependency.

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

The C sources are compiled with `-ffunction-sections -fdata-sections` so the linker keeps only what the guest calls.

## Dictionary

`dictionaries/v2025-04-21.bin` is a byte copy of `jvm-libs/linea/blob-compressor/src/main/resources/compressor-dictionaries/v2025-04-21.bin` (64 KiB, zstd dictionary format).

## Development

```bash
make -C lineth_da test      # native unit tests
make -C lineth_da compile   # statically-linked rv64im smoke ELF at zig-out/bin/zstd_smoke
make -C lineth_da exec      # compress README.md with the dictionary, then decompress it under the ZkC interpreter
```

`zstd_smoke` (`test/smoke/zstd_smoke.zig`) reads a frame with `read_input`, decompresses it against the embedded dictionary and publishes the decompressed bytes through `write_output`; it exits with 0 on success and with 2 to 8 for the `zstd.Error` category. Linking it proves the freestanding object needs no symbol the guest does not define. `compress_frame <payload-in> <frame-out> [level]` (`tools/`) produces its input.
