# L2 Execution Guest

This package contains the RISC-V guest program for vanilla EVM execution. The guest is a thin wrapper over Zesu's stateless executor: it decodes an SSZ-encoded `StatelessInput`, executes the block, and serializes the SSZ validation result — the same pipeline as Zesu's `runner.runStateless` / `zkevm-blockchain-test-runner`. Rollup-specific validation is intentionally out of scope for this iteration.

## Scope

- Decodes an SSZ `SszStatelessInput` (execution payload + execution witness + chain config) with Zesu's `ssz_decode`, executes it with Zesu's stateless executor, and serializes the 105-byte `SszStatelessValidationResult` with `ssz_output`.
- The native Zig test replays a real execution-spec-tests `tests-zkevm` fixture — pulled in as a lazy `build.zig.zon` dependency, not checked in — and asserts the serialized result matches the fixture's expected output.
- Does not include blob compression, recursive proof aggregation, or Rollup-specific public-input validation.
- Keeps cryptographic precompile/signature acceleration behind Zesu's `accel_impl` boundary. The freestanding guest leaves the `zkvm_*` accelerator symbols **unresolved** for the proving system to supply/intercept — there is no in-guest software provider. The native host test instead links Zesu's `default.zig` backend against system crypto libraries (see [Native test dependencies](../README.md#native-test-dependencies)).

## Development

The Zig version, dependency checkout, build manifest, and ZKC helper commands are shared by all guests at `riscv-guests/`.

Run from the parent directory (the top `riscv-guests/Makefile` has no `gp-*`
targets — they live in this guest's Makefile, so invoke it with `-C`):

```bash
make -C l2-execution gp-exec
```

`make -C l2-execution gp-compile` writes the guest as a **statically-linked rv64im ELF** to `riscv-guests/l2-execution/zig-out/bin/evm_execution_guest` — the [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md) artifact ("Object Format: ELF, statically linked"), linked via `build_common`'s shared `installGuestElf`. The ZKC interpreter loads it (via ELF→JSON); `make -C l2-execution gp-exec` builds it and runs it there — see the [parent README](../README.md#zkc-interpreter-integration). `make test` runs the native Zig test, which requires the native crypto libraries documented in the [parent README](../README.md#native-test-dependencies).

## Compilation

`make -C l2-execution gp-compile` (and `gp-exec`/`gp-debug`) build the guest with
the **standard** zig keccak by default. Pass `KECCAK_ACCEL=true` to build with the
arithmetization keccak wrapper (the prover-accelerated custom op) instead:

```bash
make -C l2-execution gp-compile                     # standard zig keccak
make -C l2-execution gp-compile KECCAK_ACCEL=true   # arithmetization keccak wrapper
```

Equivalently, running `zig build` directly from this directory (requires the generated linker script; run `make gp-linker-script` once after a clean checkout):

    make gp-linker-script
    zig build                       # standard zig keccak
    zig build -Dkeccak-accel=true   # arithmetization keccak wrapper

## Shell alias

`agp` (accelerated guest program): build this guest with the keccak wrapper and run
it in the ZKC interpreter on an SSZ input, from anywhere. Add to `~/.zshrc`:

```bash
agp() {
    local input
    input="$(realpath "$1")" || { echo "agp: no such file: $1" >&2; return 1; }
    /usr/bin/time -p make -C /path/to/lineth-monorepo/riscv-guests/l2-execution \
        gp-exec KECCAK_ACCEL=true GP_INPUT="$input" "${@:2}"
}
```

`realpath` resolves the input against your current directory *before* `make -C`
switches into the guest directory (and the `|| return` aborts on a bad path
instead of launching a build with no input); `/usr/bin/time -p` prints wall-clock
real/user/sys.
