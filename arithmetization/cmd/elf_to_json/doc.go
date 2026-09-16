// Command elf_to_json converts a statically linked RISC-V ELF and optional
// guest input into the JSON public inputs consumed by the R5 ZkC interpreter.
//
// # Invocation
//
// The command is registered as a module-pinned Go tool. Run it from the
// arithmetization module:
//
//	go tool elf_to_json <elf-file> <input> [input-offset] > input.json
//
// The positional arguments are the ELF executable, guest input, and an optional
// decimal or 0x-prefixed guest memory address. When omitted, input-offset is
// [elfmapping.DefaultInputOrigin]. Use -- before positional arguments when an
// argument itself begins with a hyphen.
//
// # Input forms
//
// An argument beginning with @ is always a file. Files ending in .hex must
// contain one whitespace-delimited 0x-prefixed value; files ending in .ssz or
// .bin are read as binary data. Other file suffixes are rejected.
//
// SSZ files use the guest input convention: an eight-byte little-endian length
// followed by the payload. Binary .bin files are mapped raw, without a length
// prefix. The R5 interpreter itself does not impose either layout.
//
// An argument not beginning with @ is inline input. Plain text is mapped
// verbatim. A 0x-prefixed value is interpreted as
// big-endian display text and byte-reversed before reaching guest memory,
// preserving the historical CLI convention.
//
// Examples:
//
//	go tool elf_to_json guest.elf "hello" 0x08800000
//	go tool elf_to_json guest.elf 0x01020304 0x08800000
//	go tool elf_to_json guest.elf @input.hex 0x08800000
//	go tool elf_to_json guest.elf @stateless_input.ssz 0x08800000
//	go tool elf_to_json guest.elf @private_input.bin
//
// # JSON output
//
// The command emits these inputs in stable order:
//
//   - entry_point_and_blobs_count;
//   - blobs_offset_and_size;
//   - optional blobs_executable;
//   - blobs_data;
//   - instruction_base;
//   - decoded.
//
// Values are 0x-prefixed hexadecimal strings. Underscores separate logical
// fields and records while remaining compatible with ZkC's parser. ELF
// sections are sparse blobs; decoded instructions form a dense table spanning
// the aligned executable region.
//
// # Environment
//
// ELF2JSON_PREDECODING_PROOF accepts true or false. When true, output includes
// blobs_executable, one packed bit identifying each executable blob. Executable
// bytes are present in blobs_data regardless of this setting.
//
// ELF2JSON_WRITE_SECTIONS accepts true or false. When true, the command writes
// <elf-file-without-.elf>.sections containing blob indexes, addresses, sizes,
// executable flags, and section names.
//
// ELF2JSON_MAX_DECODED_RECORDS overrides the dense-table safety limit. It
// accepts decimal or 0x-prefixed unsigned integers and protects against very
// large gaps between executable sections.
//
// ELF parsing and sparse encoding are implemented by gopkg/elfmapping.
// Instruction classification and bit packing are implemented by
// gopkg/predecoding.
package main
