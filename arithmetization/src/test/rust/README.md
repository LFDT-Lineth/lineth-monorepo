## `riscv64im-unknown-none-elf.json` — Rust Custom Target Specification

Passed to `rustc` via `--target riscv64im-unknown-none-elf.json`. Used with nightly and `-Z build-std=core` to recompile `core` without the `c` (compressed) extension, ensuring the generated ELF contains only 4-byte instructions as required by the zkVM constraint system.

### Notes

- **`cpu`**: `generic-rv64` is equivalent to Zig's `baseline_rv64` — no microarchitecture assumptions beyond the specified features.
- **`data-layout`**: describes type-level memory layout (alignment, pointer width). Unrelated to runtime memory layout (program/stack/input placement), which is controlled by `sp` initialization and linker scripts.

### Relation to linker scripts

`riscv64im-unknown-none-elf.json` controls compiler-level code generation. Linker scripts control section placement and load addresses. They are independent — a linker script can be provided via `-C link-arg=-Tscript.ld` without conflicting with this file.