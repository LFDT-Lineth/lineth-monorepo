# Makefile

The `Makefile` in this folder has commands to compile and run RISC-V test programs written in assembly, Zig or Rust against the Linea zkVM.
Programs are compiled for the  `riscv64im_zicclsm-unknown-none-elf` architecture. The resulting ELF is converted to JSON, and passed to `zkc` as an input.
The output ELF is also optinally disassembled, producing an explorable `<name>.objdump` file.

The executable, the JSON and the disassembled file live in `asm/bin/` for assembly, `zig/zig-out/bin/` for Zig, and `rust/target/riscv64im-unknown-none-elf/release/` for Rust.

## Requirements

- `riscv64-unknown-elf-gcc (>= 15)` — for assembly programs and Zig
- `zig (>= 0.16.0)` — for Zig programs
- `cargo (>= 1.88.0)` — for Rust programs
- nightly `rustc (>= rustc 1.88.0)` — for Rust programs
- `go (>= 1.26.1)` — to convert ELF to JSON
- `zkc` — to execute/debug the JSON (`make install-zkc` from the arithmetization Makefile)

ACT4 tests can be built either with Docker or directly on the host.

For Docker builds:

- `docker` — to build and run the ACT4 container

For host builds on Linux or macOS, install these prerequisites with your package manager of choice:

- `git`, `curl`, `tar`, `make`
- `riscv64-unknown-elf-gcc (>= 15)` and `riscv64-unknown-elf-objdump` — to compile and inspect ACT4 ELFs
- one ACT4 tool-management option:
  - `mise` — recommended by `riscv-arch-test` to provide `uv`, Python, Ruby and Bundler
  - `uv`, plus Ruby and Bundler
  - a Python 3.10+ virtualenv where `python3 -m pip install -e ./framework -e ./generators/testgen -e ./generators/coverage` was run from the `riscv-arch-test` checkout, plus Ruby and Bundler

For macOS host builds, also install:

- Xcode Command Line Tools or equivalent compiler tools
- native `z3`/`libz3` — used by UDB while validating ACT4 configs

To install Sail for ACT4 host builds, from `lineth-monorepo/`:

```bash
make -C arithmetization install-sail
```

Then build the ACT4 ELFs on the host:

```bash
cd arithmetization/src/test
make act4-build
```

#### Note for MacOS

If your `mise` setup does not auto-trust project configuration, run `mise trust .mise.toml` in the `riscv-arch-test` checkout once.

On macOS, UDB `0.1.9` may cache a Linux `libz3.so`. If ACT4 config validation fails with `slice is not valid mach-o file`, point UDB's cache to your native `libz3.dylib`.
Set `Z3_LIB` to the library path from your Z3 installation:

```bash
udb_cpu="$(uname -m)"
case "$udb_cpu" in arm64|aarch64) udb_cpu=arm64 ;; x86_64|x64) udb_cpu=x64 ;; esac
udb_z3_dir="${XDG_CACHE_HOME:-$HOME/.cache}/udb/z3/z3-4.16.0/$udb_cpu"
mkdir -p "$udb_z3_dir"
for name in libz3.so.4.8 libz3.so libz3 z3; do
  ln -sf "$Z3_LIB" "$udb_z3_dir/$name"
done
```

## Usage

From the `Makefile` directory:

```bash
make TEST=<src_optional_subfolder>/<name>.<ext>
```

and from anywhere using `-f`:

```bash
make -f /path/to/lineth-monorepo/arithmetization/src/test/Makefile TEST=<src_optional_subfolder>/<name>.<ext>
```

**Note:** The extension `<ext>` must be `.s`, `.zig`, or `.rs`. Source files are by default expected in the corresponding `asm/src/`, `zig/src/`, or `rust/src/` directory or in subfolders.

## Alias and usage examples

Useful shell function (add to `~/.zshrc` or `~/.bashrc`):

```bash
riscv-test() {
    local makefile="path/to/lineth-monorepo/arithmetization/src/test/Makefile"
    case "$1" in
        elf-exec|elf-debug|elf-to-json|install-zkc|clean-all|linker-script|vector-exec|keccak-rust-build|keccak-rust-json|keccak-rust-exec|blake-rust-build|blake-rust-json|blake-rust-exec|act4-build|act4-exec)
            # targets that do NOT require TEST argument
            make -f "$makefile" "$1" "${@:2}"
            ;;
        exec|debug|compile|zkc-exec|zkc-debug|clean|verify-elf|vector-build|vector-json)
            # targets that require TEST argument
            make -f "$makefile" "$1" TEST="$2" "${@:3}"
            ;;
        *)
            # default target (riscv-test foo.<ext> is the same as riscv-test exec TEST=foo.<ext>)
            make -f "$makefile" TEST="$1" "${@:2}"
            ;;
    esac
}

# Usage examples

# Compile and execute (note that <name>.<ext> can be replaced by <src_optional_subfolder>/<name>.<ext>)
riscv-test <name>.<ext>
# Compile and execute with input bytes as hex string
riscv-test <name>.<ext> IN_BYTES="0xAABB"
# Compile and execute with input bytes from a file
riscv-test <name>.<ext> IN_BYTES="@path/to/in_bytes"
# Compile, execute and generate an objdump
riscv-test <name>.<ext> OBJDUMP=true
# Compile and debug
riscv-test debug <name>.<ext>
# Compile and debug with input bytes
riscv-test debug <name>.<ext> IN_BYTES="0xAABB"
# Compile and execute with input bytes at a custom input origin
riscv-test <name>.<ext> IN_BYTES="0xAABB" IN_ORIGIN=0x08800008
# Compile only
riscv-test compile <name>.<ext>
# Build, convert and execute vectors with the generic vector targets
riscv-test vector-build <name>.<ext>
riscv-test vector-json <name>.<ext> VECTOR_FILE=path/to/vectors.all VECTOR_N_VECTORS=10
riscv-test vector-exec VECTOR_JSON_DIR=path/to/json_dir
# Use VECTOR_N_VECTORS=-1 to select all vectors
riscv-test vector-json <name>.<ext> VECTOR_FILE=path/to/vectors.all VECTOR_N_VECTORS=-1
# Build one batched JSON from selected .all vectors
riscv-test vector-build <name>.<ext>
riscv-test vector-json <name>.<ext> VECTOR_FILE=path/to/vectors.all VECTOR_N_VECTORS=10 VECTOR_JSON_MODE=batched VECTOR_JSON_FILE=path/to/vectors.json
riscv-test vector-exec VECTOR_JSON_MODE=batched VECTOR_JSON_FILE=path/to/vectors.json
# Convert an already compiled ELF to JSON
riscv-test elf-to-json BIN_EXT=asm/bin/test
# Execute an already compiled ELF
riscv-test elf-exec BIN_EXT=path/to/bin
# Debug an already compiled ELF
riscv-test elf-debug BIN_EXT=path/to/bin
# Execute an already compiled ELF in quiet mode
riscv-test elf-exec BIN_EXT=path/to/bin ZKC_EXEC_FLAGS=-q
# Clean build artifacts for a specific test
riscv-test clean <name>.<ext>
# Clean all build artifacts
riscv-test clean-all
# Run all Blake vectors from blake10.all
riscv-test blake-rust-exec
# Build one batched Keccak JSON
riscv-test keccak-rust-json KECCAK_N_VECTORS=10 KECCAK_JSON_FILE=/path/to/keccak.json
# Build ACT4 ELFs
riscv-test act4-build
# Run ACT4 ELFs through zkc
riscv-test act4-exec
# Run blake_with_in_embedded.rs (input bytes are embedded in main())
riscv-test blake/blake_with_in_embedded.rs
# Run blake_with_in_bytes.rs
# written in RAM as <213-byte input><64-byte expected output>
riscv-test blake/blake_with_in_bytes.rs IN_BYTES="0x239900d4ed8623b95a92f1dba88ad31895cc3345ded552c22d79ab2a39c5877dd1a2ffdb6fbb124bb7c45a68142f214ce9f6129fb697276a0d4d1c983fa580ba010000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006362615be0cd19137e21791f83d9abfb41bd6b9b05688c2b3e6c1f510e527fade682d1a54ff53a5f1d36f13c6ef372fe94f82bbb67ae8584caa73b6a09e667f2bdc9480c000000"
# Generate the linker script with a custom input origin
riscv-test linker-script IN_ORIGIN=0x08800008
# Verify ELF entry point, origins, lengths, stack and heap symbols match default ones
riscv-test verify-elf <name>.<ext>
# Compile and verify ELF entry point, origins, lengths, stack and heap symbols match a custom stack
riscv-test compile <name>.<ext> SP=0x01000000 VERIFY_ELF=true
# Compile and verify generated ELF entry point, origins, lengths, stack and heap symbols match default ones
riscv-test compile <name>.<ext> VERIFY_ELF=true
```

## Targets

| Target                                                   | Description                                                                            |
|----------------------------------------------------------|----------------------------------------------------------------------------------------|
| `make TEST=foo.<ext>`                                    | Compile and execute (default)                                                          |
| `make debug TEST=foo.<ext>`                              | Compile and debug                                                                      |
| `make compile TEST=foo.<ext>`                            | Compile only                                                                           |
| `make elf-exec BIN_EXT=foo`                              | Convert and execute an already compiled ELF (`JSON_EXT=foo.json` by default)           |
| `make elf-debug BIN_EXT=foo`                             | Convert and debug an already compiled ELF (`JSON_EXT=foo.json` by default)             |
| `make elf-to-json BIN_EXT=foo`                           | Convert an already compiled ELF to JSON (`JSON_EXT=foo.json` by default)               |
| `make install-zkc`                                       | Invoke `../../../Makefile install-zkc` to install zkc if not already installed         |
| `make zkc-exec TEST=foo.<ext>`                           | Execute without recompiling                                                            |
| `make zkc-debug TEST=foo.<ext>`                          | Debug without recompiling                                                              |
| `make clean TEST=foo.<ext>`                              | Remove binary and JSON for this test                                                   |
| `make clean-all`                                         | Remove all build artifacts                                                             |
| `make linker-script`                                     | Generate the linker script with the memory layout                                      |
| `make verify-elf TEST=foo.<ext>`                         | Verify ELF entry point, origins, lengths, stack and heap symbols match default ones    |
| `make vector-build TEST=foo.<ext>`                       | Compile the test and build vector helpers                                              |
| `make vector-json TEST=foo.<ext> VECTOR_FILE=foo.all`    | Generate vector JSON inputs; run `vector-build` first                                  |
| `make vector-exec`                                       | Execute vector JSON inputs generated by `vector-json`                                  |
| `make keccak-rust-build`                                 | Build the Keccak Rust vector test and helper binaries                                  |
| `make keccak-rust-json`                                  | Generate one batched Keccak vector JSON input                                          |
| `make keccak-rust-exec`                                  | Run the batched Keccak vector JSON input                                               |
| `make blake-rust-build`                                  | Build the Blake Rust vector test and helper binaries                                   |
| `make blake-rust-json`                                   | Generate Blake vector JSON inputs                                                      |
| `make blake-rust-exec`                                   | Run all Blake vectors from `rust/src/blake/blake10.all`                                |
| `make act4-build`                                        | Build ACT4 ELFs with the Linea ACT4 config                                             |
| `make act4-exec`                                         | Build and run ACT4 ELFs through zkc and write results/logs under `act4/bin/`           |

`require-*` targets are internal support targets used to validate mandatory command-line variables before running the targets above.

## Options

| Variable                     | Default                                                                                | Description                                                                                                                                   |
|------------------------------|----------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| `TEST`                       | `""`                                                                                   | Source file path with extension, relative to the corresponding `src/` folder                                                                  |
| `ZKC_EXEC_FLAGS`             | `""`                                                                                   | Flags to use when invoking `zkc exec` within `zkc-exec` and `elf-exec` targets                                                                |
| `BIN_EXT`                    | `""`                                                                                   | Already compiled ELF used by `elf-to-json`, `elf-exec` and `elf-debug`                                                                        |
| `JSON_EXT`                   | `$(BIN_EXT).json`                                                                      | JSON output path used by `elf-to-json`, `elf-exec` and `elf-debug`                                                                            |
| `VECTOR_FILE`                | `""`                                                                                   | `.all` vector file consumed by `vector-json`; one `IN_BYTES` per line                                                                         |
| `VECTOR_N_VECTORS`           | `""`                                                                                   | Number of vectors selected by `vector-json`; `-1` means all vectors                                                                           |
| `VECTOR_JSON_MODE`           | `per-vector`                                                                           | `per-vector` for one JSON per vector, `batched` for one JSON with selected vectors concatenated                                               |
| `VECTOR_JSON_FILE`           | `$(JSON)`                                                                              | Batched JSON path used when `VECTOR_JSON_MODE=batched`                                                                                        |
| `VECTOR_JSON_DIR`            | `$(dir $(JSON))vector_json`                                                            | JSON directory used when `VECTOR_JSON_MODE=per-vector`                                                                                        |
| `VECTOR_SUBSET_FILE`         | `$(BIN).all`                                                                           | Intermediate `.all` file selected from `VECTOR_FILE`; one line per vector, or one blob including all vectors                                  |
| `IN_BYTES`                   | `""`                                                                                   | Hex big-endian input written in RAM at `IN_ORIGIN` as little-endian bytes before execution (either string or `@path/to/in_bytes`)             |
| `STACK_ORIGIN`               | `0x00000000`                                                                           | Low stack boundary; `_stack_end` is generated from this value                                                                                 |
| `SP`                         | `STACK_ORIGIN + 0x00800000`                                                            | Initial stack pointer; `_stack_start` is generated from this value                                                                            |
| `PROGRAM_ORIGIN`             | `SP`                                                                                   | Program start address                                                                                                                         |
| `IN_ORIGIN`                  | `PROGRAM_ORIGIN + 0x08000000`                                                          | Input region start address; `_in_start` is generated from this value                                                                          |
| `HEAP_ORIGIN`                | `IN_ORIGIN + 0x40000000`                                                               | Heap start address; `_heap_start` is generated from this value                                                                                |
| `OBJDUMP`                    | `false`                                                                                | Set to `true` to generate an objdump file for each compiled ELF                                                                               |
| `VERIFY_ELF`                 | `false`                                                                                | Set to `true` to verify ELF entry point, origins, lengths, stack and heap symbols match default ones                                          |
| `ACT4_BUILD_MODE`            | `host`                                                                                 | Build ACT4 ELFs with `host` or `docker`                                                                                                       |
| `ACT4_REF`                   | `9798a554ce4139f472c9ccd3a18c9061d0f7024d`                                             | `riscv-arch-test` tag or commit used to build ACT4 ELFs                                                                                       |
| `ACT4_REPO`                  | `../riscv-arch-test`                                                                   | Local `riscv-arch-test` checkout used for ACT4 builds                                                                                         |
| `ACT4_RISCV_DIR`             | `~/riscv`                                                                              | Directory where `install-sail` installs `sail_riscv_sim`                                                                                      |
| `ACT4_FAST`                  | `false`                                                                                | Set to `true` to skip ACT4 objdump generation for faster builds                                                                               |
| `ACT4_DEBUG`                 | `true`                                                                                 | Set to `false` to skip ACT4 debug artifacts                                                                                                   |
| `BLAKE_ALL_FILE`             | `rust/src/blake/blake10.all`                                                           | Blake `.all` vector file used by `blake-rust-json`                                                                                            |
| `BLAKE_N_VECTORS`            | `-1`                                                                                   | Number of Blake vectors to generate; `-1` means all vectors                                                                                   |
| `BLAKE_JSON_DIR`             | `rust/target/riscv64im-unknown-none-elf/release/blake_json`                            | Directory where `blake-rust-json` writes per-vector JSON files                                                                                |
| `KECCAK_ALL_FILE`            | `rust/src/keccak/keccak.all`                                                           | Keccak `.all` vector file used by `keccak-rust-json`                                                                                          |
| `KECCAK_N_VECTORS`           | `10`                                                                                   | Number of Keccak vectors compiled into and packed for the Keccak guest; `-1` means all vectors                                                |
| `KECCAK_JSON_FILE`           | `rust/target/riscv64im-unknown-none-elf/release/keccak.json`                           | JSON path written by `keccak-rust-json`                                                                                                       |

`IN_BYTES` values are expected in big-endian hex format.
All `.all` vector files contain one big-endian `IN_BYTES` value per line.
In `batched` mode, the Makefile writes the selected vectors as one big-endian `IN_BYTES` blob, and `main.go` reverses that blob into the RAM-order input consumed by the guest.

## ELF-to-JSON helper

`main.go` converts an ELF and optional input bytes into the JSON consumed by `zkc`:

```bash
go run main.go <elfFile> <inBytes|@hexFile> <inBytesOffset>
```

Use inline `0x...` for small inputs and `@path/to/in_bytes` for a file containing one `0x...` blob.
Both forms are interpreted as big-endian `IN_BYTES` and reversed before being written to RAM.

## JSON input format

The ELF-to-JSON helper writes sparse memory blobs in this shape:

```json
{
  "entry_point_and_blobs_count": "0x<entry_point:u64>_<blobs_count:u64>",
  "blobs_offset_and_size": "0x<blob0_offset:u64>_<blob0_size:u64>____<blob1_offset:u64>_<blob1_size:u64>____...",
  "blobs_data": "0x<blob0_bytes>____<blob1_bytes>____..."
}
```

Use `_` to read fields inside one packed value and `____` to read separate blobs or array items.
`zkc` ignores `_` in JSON input strings, so these separators are only for inspection.
For `IN_BYTES`, pass hex in big-endian order; the ELF-to-JSON helper writes the reversed bytes into `blobs_data`.

## Target ISA

All programs are compiled targeting `RV64IM` accordingly to the [Ethereum zkVM standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md).
Note that `Zicclsm` extension does not affect the generated ELF so it is omitted.
Moreover, ABI being `LP64` (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

## ACT4

The `Makefile` in this folder allows running tests from the [RISC-V Architectural Certification Tests (ACTs)](https://github.com/riscv/riscv-arch-test) framework (currently ACT4), which is a set of assembly language tests designed to certify that a design faithfully implements the RISC-V specification.

Tests can be inspected by looking at:

```
https://github.com/riscv/riscv-arch-test/tree/act4/tests/rv64i/I
https://github.com/riscv/riscv-arch-test/tree/act4/tests/rv64i/M
```

ACT4 uses the configuration in `act4/config/linea-rv64im-zicclsm/`.
`make act4-build` clones `riscv-arch-test` next to `lineth-monorepo` if needed, checks out `ACT4_REF`, rebuilds ELFs either with Docker or on the host, and generates one JSON input per ELF.
The folder structure is the following:

```text
parent/
├── lineth-monorepo/
│   └── arithmetization/src/test/
│       └── act4/
│           ├── config/linea-rv64im-zicclsm/    # Linea ACT4 config
│           └── bin/
│               ├── work/linea-rv64im-zicclsm/  # ACT4 generated files
│               │   ├── build/                  # intermediate/debug ACT4 artifacts
│               │   └── elfs/                   # final self-checking ACT4 ELFs
│               ├── logs/                       # per-test JSON and zkc logs
│               └── results.txt                 # PASS/FAIL summary
└── riscv-arch-test/                            # ACT4 framework checkout
```

From `lineth-monorepo/arithmetization/src/test`:

```bash
make act4-exec                         # build on the host and run
make act4-exec ACT4_BUILD_MODE=docker  # build with Docker and run
```

By default, ACT4 is built with debug artifacts enabled and fast mode disabled.
To build faster without objdumps, traces and trap reports:

```bash
make act4-build ACT4_DEBUG=false ACT4_FAST=true
```

The `build/` directory contains ACT4 intermediate and debug artifacts: signature-generating ELFs, signatures, objdumps, traces and trap reports.
The `elfs/` directory contains the final self-checking ELFs run by `make act4-exec`.
The `logs/` directory contains one JSON input per test generated by `act4-build`, non-empty JSON conversion stderr in `.json.err`, and non-empty zkc stderr in `.err` for failing tests.
Add `export ELF2JSON_WRITE_SECTIONS=true` to your shell startup file (e.g. `~/.bashrc` or `~/.zshrc`) to also write `.sections` files next to the ELF files, listing the sparse blobs included in the generated JSON input with their indexes, offsets, sizes and names.
`act4-exec` runs `zkc exec -q`, so stdout is suppressed and a test passes when zkc exits with code 0 and stderr is empty. A summary of ACT4 results is written in `results.txt`.

To rerun one generated ACT4 test through zkc:

```bash
zkc exec -q act4/bin/logs/<test-name>.json ../../main/riscv/main.zkc
```

## Default memory layout

```
___________________________________________ 0x00000000 ≡ STACK_ORIGIN * = _stack_end
 STACK:   ↑ 8 MiB STACK_LENGTH
___________________________________________ 0x00800000 ≡ SP * = _stack_start = PROGRAM_ORIGIN * = _program_start
 PROGRAM: ↓ 128 MiB PROGRAM_LENGTH
___________________________________________ 0x08800000 ≡ _program_end = IN_ORIGIN * = _in_start
 IN:      ↓ 1 GiB: IN_LENGTH
___________________________________________ 0x48800000 ≡ _in_end = HEAP_ORIGIN * = _heap_start
 HEAP:    ↓ until address-space overflows

* = Parameters to optionally set, others are derived (all have default values).
≡ = Default values.
The order of memory regions is fixed.
```
