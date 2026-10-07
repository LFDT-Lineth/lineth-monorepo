# elf_to_json

`elf_to_json` converts a statically linked RISC-V ELF and optional guest input
into the JSON inputs consumed by the R5 ZkC interpreter.

Run the module-pinned tool from the `arithmetization` module:

```sh
go tool elf_to_json <elfFile> <input|@file.hex|@file.ssz|@file.bin> [inputOffset] > input.json
```

The second argument accepts inline raw text, inline `0x`-prefixed hexadecimal
data, or a file prefixed by `@`. `.hex` files contain one hexadecimal value;
`.ssz` files are mapped as an eight-byte little-endian length followed by the
payload; `.bin` files are mapped raw. Other file suffixes are rejected.
When `inputOffset` is omitted, it defaults to `0x08800000`.

The output contains `entry_point_and_blobs_count`, `blobs_offset_and_size`,
`blobs_data`, `instruction_base`, and `decoded`.

Environment variables:

- `ELF2JSON_PREDECODING_PROOF=true` also emits `blobs_executable`.
- `ELF2JSON_WRITE_SECTIONS=true` writes a `.sections` diagnostic file beside
  the ELF.
- `ELF2JSON_MAX_DECODED_RECORDS` overrides the default dense-table safety cap.

ELF mapping is implemented by `gopkg/elfmapping`; instruction decoding and
packing are implemented by `gopkg/predecoding`.

See [package documentation](doc.go) for full command documentation.
