# L2 Execution Guest

This package contains the RISC-V guest program for vanilla EVM execution. The guest is a thin wrapper over Zesu's stateless executor: it decodes an SSZ-encoded `StatelessInput`, executes the block, and serializes the SSZ validation result — the same pipeline as Zesu's `runner.runStateless` / `zkevm-blockchain-test-runner`. Rollup-specific validation is intentionally out of scope for this iteration.

## Scope

- Decodes an SSZ `SszStatelessInput` (execution payload + execution witness + chain config) with Zesu's `ssz_decode`, executes it with Zesu's stateless executor, and serializes the 105-byte `SszStatelessValidationResult` with `ssz_output`.
- The native Zig test replays a real execution-spec-tests `tests-zkevm` fixture — pulled in as a lazy `build.zig.zon` dependency, not checked in — and asserts the serialized result matches the fixture's expected output.
- Does not include blob compression, recursive proof aggregation, or Rollup-specific public-input validation.
- Keeps cryptographic precompile/signature acceleration behind Zesu's `accel_impl` boundary. The freestanding guest leaves the `zkvm_*` accelerator symbols **unresolved** for the proving system to supply/intercept — there is no in-guest software provider. The native host test instead links Zesu's `default.zig` backend against system crypto libraries (see [Native test dependencies](../README.md#native-test-dependencies)).

## Development

The Zig version, dependency checkout, build manifest, and ZKC helper commands are shared by all guests at `riscv-guests/`.

Run from the parent directory:

```bash
cd riscv-guests
make compile GUEST=l2-execution ZIG=/path/to/zig
make test ZIG=/path/to/zig
```

`make compile` writes the guest as a **statically-linked rv64im ELF** to `riscv-guests/l2-execution/zig-out/bin/evm_execution_guest` — the [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md) artifact ("Object Format: ELF, statically linked"), linked via `build_common`'s shared `installGuestElf`. The ZKC interpreter loads it (via ELF→JSON); `make exec`/`fixture-exec` build it and run it there — see the [parent README](../README.md#zkc-interpreter-integration). `make test` runs the native Zig test, which requires the native crypto libraries documented in the [parent README](../README.md#native-test-dependencies).
