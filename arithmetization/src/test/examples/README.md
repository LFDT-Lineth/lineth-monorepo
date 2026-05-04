# Makefile

The `Makefile` in this folder has commands to compile and run RISC-V test programs written in assembly, Zig or Rust against the Linea zkVM.
Programs are compiled for the  `riscv64im_zicclsm-unknown-none-elf` architecture. The resulting ELF is converted to JSON, and passed to `zkc` as an input.
The output ELF is also disassembled, producing an explorable `<name>_disassembled.elf` file.

The executable, the json and the disassembled elf file all live in the `<ext>/bin/` folder.

## Requirements

- `riscv64-unknown-elf-as (>= 2.45)` — for assembly programs
- `zig (>= 0.16.0)` — for Zig programs
- `cargo (>= 1.88.0)` — for Rust programs
- `rustc (>= rustc 1.88.0)` with `riscv64imac-unknown-none-elf` target — for Rust programs
- `go (>= 1.26.1)` — to convert ELF to JSON
- `go-corset, zkc (>= 1.2.12)` — to execute/debug the JSON

## Usage

From the `Makefile` directory:

```bash
make TEST=<name>.<ext>
```

and from anywhere using `-f`:

```bash
make -f /path/to/linea-monorepo/arithmetization/src/test/examples/Makefile TEST=<name>.<ext>
```

**Note:** The extension `<ext>` must be `.s`, `.zig`, or `.rs`. Source files are by default expected in the corresponding `asm/src/`, `zig/src/`, or `rust/src/` directory. Alternatively one can provide full paths.


## Alias and usage examples

Useful shell function (add to `~/.zshrc` or `~/.bashrc`):

```bash
zkc-test() {
    case "$1" in
        clean-all)
            make -f "path/to/linea-monorepo/arithmetization/src/test/examples/Makefile" clean-all
            ;;
        exec|debug|compile|clean)
            local target="$1"; shift
            make -f "path/to/linea-monorepo/arithmetization/src/test/examples/Makefile" "$target" TEST="$1" "${@:2}"
            ;;
        *)
            make -f "path/to/linea-monorepo/arithmetization/src/test/examples/Makefile" TEST="$1" "${@:2}"
            ;;
    esac
}

# Usage examples

# Compile and execute
zkc-test <name>.<ext>
# Compile and execute with input bytes
zkc-test <name>.<ext> IN_BYTES="0xAABB"
# Compile and execute with input bytes at a custom offset
zkc-test <name>.<ext> IN_BYTES="0xAABB" IN_BYTES_OFFSET=0x8000000
# Compile and debug
zkc-test debug <name>.<ext>
# Compile and debug with input bytes
zkc-test debug <name>.<ext> IN_BYTES="0xAABB"
# Compile only
zkc-test compile <name>.<ext>
# Clean build artifacts for a specific test
zkc-test clean <name>.<ext>
# Clean all build artifacts
zkc-test clean-all
# Compile and execute a Zig program without stripping
zkc-test <name>.zig ZIG_STRIP=false
```

## Targets

| Target                        | Description                          |
|-------------------------------|--------------------------------------|
| `make TEST=foo.<ext>`         | Compile and execute (default)        |
| `make debug TEST=foo.<ext>`   | Compile and debug                    |
| `make compile TEST=foo.<ext>` | Compile only                         |
| `make clean TEST=foo.<ext>`   | Remove binary and JSON for this test |
| `make clean-all`              | Remove all build artifacts           |

## Options

| Variable         | Default                                                                                 | Description                                                                |
|------------------|-----------------------------------------------------------------------------------------|----------------------------------------------------------------------------|
| `SRC`            | `asm/src/<TEST>`, `zig/src/<TEST>`, or `rust/src/<TEST>` depending on extension         | Path to the source file, can be overridden                                 |
| `BIN`            | `asm/bin/<NAME>`, `zig/zig-out/bin/<NAME>`, or `rust/bin/<NAME>` depending on extension | Path to the output ELF binary, can be overridden                           |
| `JSON`           | same directory as `BIN`, with `.json` extension                                         | Path to the output JSON file, can be overridden                            |
| `STRIP`          | `false`                                                                                 | Strip debug symbols from the ELF after compilation                         |
| `ZIG_STRIP`      | `true`                                                                                  | Strip when compiling Zig (reduces binary size), ignored for `.s` and `.rs` |
| `IN_BYTES`        | `""`                                                                                    | Input bytes written to memory at `IN_BYTES_OFFSET` before execution         |
| `PROGRAM_OFFSET` | `0`                                                                                     | Memory offset where the program is loaded (up to 128 MB)                   |
| `IN_BYTES_OFFSET` | `0x8000000`                                                                             | Memory offset where input bytes are written (up to 1 GB)                   |
| `ENTRY_POINT`    | `0`                                                                                     | Entry point offset                                                         |

## Target ISA

All programs are compiled targeting `RV64IM` accordingly to the [Ethereum zkVM standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md).
Note that `Zicclsm` extension does not affect the generated ELF so it is omitted.
Moreover, ABI being `LP64` (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

## Default memory layout

```
0x00000000  ──  program starts
    ↓  program grows up (up to 128 MiB)
0x07FFFFFF  ──  program ends at most
0x08000000  --  sp ends here
    ↑  stack grows downward
0x087fffff  ──  sp starts here (up to 8 MiB)
0x08800000  ──  input starts
    ↓  input grows up (up to 1 GiB)
0x48800000 ──  input ends at most
```

## Deprecated

**Note.** The following command is done by default.
Run the following command to disassemble the generated ELF:

```
riscv64-unknown-elf-objdump -d --line-numbers -S test
```
