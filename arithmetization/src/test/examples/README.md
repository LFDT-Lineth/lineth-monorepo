# Makefile

The `Makefile` in this folder has commands to compile and run RISC-V test programs written in assembly, Zig or Rust against the Linea zkVM.
Programs are compiled for the  `riscv64im_zicclsm-unknown-none-elf` architecture. The resulting ELF is converted to JSON, and passed to `zkc` as an input.
The output ELF is also disassembled, producing an explorable `<name>.objdump` file.

The executable, the JSON and the disassembled file live in `asm/bin/` for assembly, `zig/zig-out/bin/` for Zig, and `rust/target/riscv64im-unknown-none-elf/release/` for Rust.

## Requirements

- `riscv64-unknown-elf-as (>= 2.45)` — for assembly programs and Zig
- `zig (>= 0.16.0)` — for Zig programs
- `cargo (>= 1.88.0)` — for Rust programs
- `rustc (>= rustc 1.88.0)` with `riscv64imac-unknown-none-elf` target — for Rust programs
- `go (>= 1.26.1)` — to convert ELF to JSON
- `go-corset, zkc (>= 1.2.12)` — to execute/debug the JSON

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

To install Sail for ACT4 host builds, from `linea-monorepo/`:

```bash
make -C arithmetization install-sail
```

Then build the ACT4 ELFs on the host:

```bash
cd arithmetization/src/test/examples
make build-act4
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
make -f /path/to/linea-monorepo/arithmetization/src/test/examples/Makefile TEST=<src_optional_subfolder>/<name>.<ext>
```

**Note:** The extension `<ext>` must be `.s`, `.zig`, or `.rs`. Source files are by default expected in the corresponding `asm/src/`, `zig/src/`, or `rust/src/` directory or in subfolders.

## Alias and usage examples

Useful shell function (add to `~/.zshrc` or `~/.bashrc`):

```bash
riscv-test() {
    local makefile="path/to/linea-monorepo/arithmetization/src/test/examples/Makefile"
    case "$1" in
        clean-all|linker-script|blake-all|build-act4|run-act4|install-zkc)
            # targets that do NOT require TEST argument
            make -f "$makefile" "$1" "${@:2}"
            ;;
        exec|debug|compile|zkc-exec|zkc-debug|clean|verify-elf)
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
# Compile and execute with input bytes
riscv-test <name>.<ext> IN_BYTES="0xAABB"
# Compile and debug
riscv-test debug <name>.<ext>
# Compile and debug with input bytes
riscv-test debug <name>.<ext> IN_BYTES="0xAABB"
# Compile and execute with input bytes at a custom offset
riscv-test <name>.<ext> IN_BYTES="0xAABB" IN_BYTES_OFFSET=0x08800008
# Compile only
riscv-test compile <name>.<ext>
# Clean build artifacts for a specific test
riscv-test clean <name>.<ext>
# Clean all build artifacts
riscv-test clean-all
# Run all converted Blake test vectors
riscv-test blake-all
# Build ACT4 ELFs
riscv-test build-act4
# Run ACT4 ELFs through zkc
riscv-test run-act4
# Run blake_with_in_embedded.rs (input bytes are embedded in main())
riscv-test blake/blake_with_in_embedded.rs
# Run blake_with_in_bytes.rs with IN_BYTES="0x<213_bytes_input_hex><64_bytes_expected_output_hex>"
riscv-test blake/blake_with_in_bytes.rs IN_BYTES="0x0000000c48c9bdf267e6096a3ba7ca8485ae67bb2bf894fe72f36e3cf1361d5f3af54fa5d182e6ad7f520e511f6c3e2b8c68059b6bbd41fbabd9831f79217e1319cde05b61626300000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000001ba80a53f981c4d0d6a2797b69f12f6e94c212f14685ac4b74b12bb6fdbffa2d17d87c5392aab792dc252d5de4533cc9518d38aa8dbf1925ab92386edd4009923"
# Generate the linker script with custom input bytes offset
riscv-test linker-script IN_BYTES_OFFSET=0x00000042
# Verify ELF offsets, entry point and sp match the default ones
riscv-test verify-elf <name>.<ext>
# Verify ELF offsets, entry point and sp match the custom ones
riscv-test verify-elf <name>.<ext> PROGRAM_OFFSET=0x10000000 IN_BYTES_OFFSET=0x18800000 SP=0x187fffff 
# Compile and verify generated ELF offsets, entry point and sp match the default ones
riscv-test compile <name>.<ext> VERIFY_ELF=true
```

## Targets

| Target                           | Description                                                                            |
|----------------------------------|----------------------------------------------------------------------------------------|
| `make TEST=foo.<ext>`            | Compile and execute (default)                                                          |
| `make debug TEST=foo.<ext>`      | Compile and debug                                                                      |
| `make compile TEST=foo.<ext>`    | Compile only                                                                           |
| `make zkc-exec TEST=foo.<ext>`   | Execute without recompiling                                                            |
| `make zkc-debug TEST=foo.<ext>`  | Debug without recompiling                                                              |
| `make clean TEST=foo.<ext>`      | Remove binary and JSON for this test                                                   |
| `make clean-all`                 | Remove all build artifacts                                                             |
| `make linker-script`             | Generate the linker script with the memory layout                                      |
| `make verify-elf TEST=foo.<ext>` | Verify ELF offsets, entry point and sp match the ones in the Makefile                  |
| `make blake-all`                 | Run all blake test vectors in `rust/src/blake/blake.all`                               |
| `make build-act4`                | Build ACT4 ELFs with the Linea ACT4 config                                             |
| `make run-act4`                  | Build and run ACT4 ELFs through zkc and write results/logs under `act4/bin/`           |

## Options

| Variable         | Default                                                                                 | Description                                                                   |
|------------------|-----------------------------------------------------------------------------------------|-------------------------------------------------------------------------------|
| `IN_BYTES`       | `""`                                                                                    | Input bytes written to memory at `IN_BYTES_OFFSET` before execution           |
| `PROGRAM_OFFSET` | `0x00000000`                                                                            | Memory address where the program is loaded (up to 128 MiB)                    |
| `IN_BYTES_OFFSET`| `0x08800000`                                                                            | Memory address where input bytes are written (up to 1 GiB)                    |
| `SP`             | `0x08800000`                                                                            | Top of the stack region, stack grows downward from this address (8 MiB)       |
| `VERIFY_ELF`     | `false`                                                                                 | Set to `true` to verify offsets, entry point and sp match the ELF ones        |
| `ACT4_BUILD_MODE`| `host`                                                                                  | Build ACT4 ELFs with `host` or `docker`                                       |
| `ACT4_REF`       | `9798a554ce4139f472c9ccd3a18c9061d0f7024d`                                              | `riscv-arch-test` tag or commit used to build ACT4 ELFs                       |
| `ACT4_REPO`      | `../riscv-arch-test`                                                                    | Local `riscv-arch-test` checkout used for ACT4 builds                         |
| `ACT4_RISCV_DIR` | `~/riscv`                                                                               | Directory where `install-sail` installs `sail_riscv_sim`                      |
| `ACT4_DEBUG`     | `true`                                                                                  | Set to `false` to skip ACT4 debug artifacts                                   |
| `ACT4_FAST`      | `false`                                                                                 | Set to `true` to skip ACT4 objdump generation for faster builds               |

## Target ISA

All programs are compiled targeting `RV64IM` accordingly to the [Ethereum zkVM standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md).
Note that `Zicclsm` extension does not affect the generated ELF so it is omitted.
Moreover, ABI being `LP64` (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

## ACT4

The `Makefile` in this folder allows running tests from the [RISC-V Architectural Certification Tests (ACTs)](https://github.com/riscv/riscv-arch-test) framework (currently ACT4), which is set of assembly language tests designed to certify that a design faithfully implements the RISC-V specification.

Tests can be inspected by looking at:

```
https://github.com/riscv/riscv-arch-test/tree/act4/tests/rv64i/I
https://github.com/riscv/riscv-arch-test/tree/act4/tests/rv64i/M
```

ACT4 uses the configuration in `act4/config/linea-rv64im-zicclsm/`.
`make build-act4` clones `riscv-arch-test` next to `linea-monorepo` if needed, checks out `ACT4_REF`, and builds ELFs either with Docker or on the host.
The folder structure is the following:

```text
parent/
├── linea-monorepo/
│   └── arithmetization/src/test/examples/
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

From `linea-monorepo/arithmetization/src/test/examples`:

```bash
make run-act4                         # build on the host and run
make run-act4 ACT4_BUILD_MODE=docker  # build with Docker and run
```

By default, ACT4 is built with debug artifacts enabled and fast mode disabled.
To build faster without objdumps, traces and trap reports:

```bash
make build-act4 ACT4_DEBUG=false ACT4_FAST=true
```

The `build/` directory contains ACT4 intermediate and debug artifacts: signature-generating ELFs, signatures, objdumps, traces and trap reports.
The `elfs/` directory contains the final self-checking ELFs run by `make run-act4`.
The `logs/` directory contains one JSON input per test, non-empty JSON conversion stderr in `.json.err`, and the filtered ecall output (for `exit` or `write`) in `.out`.
The full zkc output is kept as `.full` only for failing tests. A summary of ACT4 results is written in `results.txt`.

To rerun one generated ACT4 test through zkc:

```bash
zkc exec act4/bin/logs/<test-name>.json ../../main/riscv/main.zkc
```

## Default memory layout

```
0x00000000  ──  program starts
    ↓  program grows up (up to 128 MiB)
0x07FFFFFF  ──  program ends at most
0x08000000  --  sp ends here
    ↑  stack grows downward
0x08800000  ──  sp starts here (up to 8 MiB)
0x08800000  ──  input starts
    ↓  input grows up (up to 1 GiB)
0x48800000  ──  input ends at most
```
