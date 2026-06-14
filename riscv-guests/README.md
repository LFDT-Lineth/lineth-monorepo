# RISC-V Guest Programs

This directory holds the RISC-V guest programs that target the Linea ZKC interpreter. Each guest is a **self-contained Zig package** — its own `build.zig`, `build.zig.zon` (its dependencies), `Makefile` (its compile/test lifecycle) and `src/`. A thin top-level `Makefile` orchestrates them all, and shared build logic lives in `build_common/`. They share one Zig toolchain (`.zigversion`).

## Layout

```text
riscv-guests/
  .zigversion        Required Zig development version (shared by all guests)
  Makefile           Top-level orchestrator — fans compile/test/… out to every guest in GUESTS
  build_common/      Shared build helpers (+ the shared standalone-ELF link: start.s, linker_script.ld)
  l2-execution/      Vanilla EVM execution guest: build.zig + build.zig.zon + Makefile + src/ + test/
```

Within a guest, `src/` holds **only the production code that ships in the rv64im object/ELF**; host-only code (unit tests, the spec-test harness, fixture parsing) lives in `test/`, and committed sample/test data in `test/testdata/`. The split mirrors what `build.zig` builds: the object + `elf` step compile `src/`; `zig build test` / `spec-tests` compile `test/`. (Automated tests pull their EF fixtures from the lazy `execution_spec_tests_zkevm` dependency, not from committed data — `test/testdata/` is just the manual ZkC-run samples.)

**Add a guest:** create `riscv-guests/<name>/` (its own `build.zig`, `build.zig.zon`, `Makefile`, `src/` for production code + `test/` for host tests, depending on `../build_common`) and append `<name>` to `GUESTS` in the top-level `Makefile`. Future guests (Rollup, Aggregation) slot in this way — each with its own dependencies and compile/lint sequence.

## Required Toolchain

- Zig `0.16.0`. Recorded in `.zigversion` and enforced by `build_common` (`requireZigVersion`).
- Go, for converting compiled ELFs into the JSON input consumed by the ZKC interpreter.
- `zkc` on `PATH`, for a guest's `exec` / `debug` / fixture targets.
- Optional: `riscv64-unknown-elf-objdump` for compile-time disassembly output.

Set `ZIG=/path/to/zig` when the required Zig binary is not first on `PATH`.

## Dependencies

Each guest pins its **own** external dependencies in its `build.zig.zon`. For `l2-execution`: **Zesu** (EVM/stateless execution), **Consensys/zesu-zkvm** (its pure-Zig precompile backend `stdlibs_accel`, which the guest's in-guest crypto delegates to), and the **execution-spec-tests `tests-zkevm` fixtures** (a `lazy` dependency, fetched only for the tests). Every guest also takes `../build_common` as a path dependency for the shared build helpers. `make fetch` pre-fetches a guest's tree.

## Native test dependencies

A guest's `make test` runs its logic on the **host**, where Zesu's `default.zig` accelerator backend links native crypto C libraries:

| Library | Provides |
| --- | --- |
| `libsecp256k1` | ecrecover / signature verification |
| OpenSSL (`libssl`, `libcrypto`) | secp256r1 (P-256) |
| `libblst` | BLS12-381 + KZG point evaluation |
| `libmcl` | BN254 |

Expected under a single prefix — `/opt/homebrew` on macOS, `/usr/local` on Linux — overridable with `-Dcrypto-prefix=<prefix>`. Install them all via Zesu's helper (from a Zesu checkout): `make install-deps`. The freestanding guest ELF (`make compile`) needs **none** of these: its precompiles are either pure-Zig (zesu-zkvm's `stdlibs_accel`, compiled in) or a custom RISC-V opcode (keccak) the prover arithmetizes at execution.

## Development

From `riscv-guests/`, the top-level Makefile builds/tests **every** guest in `GUESTS`:

```bash
make compile ZIG=/path/to/zig   # build each guest's statically-linked rv64im ELF
make test    ZIG=/path/to/zig   # run each guest's native host tests
make clean   ZIG=/path/to/zig
make help
```

Work on a single guest by invoking its own Makefile directly:

```bash
make -C l2-execution compile ZIG=/path/to/zig
make -C l2-execution compile ZIG=/path/to/zig IN_ORIGIN=0x08800000   # override the input offset
```

`make compile` builds the guest as a **statically-linked rv64im ELF** under `<guest>/zig-out/bin/` — the [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md) artifact ("Object Format: ELF, statically linked"). `make test` runs the native Zig unit tests (see [Native test dependencies](#native-test-dependencies)).

### Spec tests (l2-execution only — full EF zkevm fixture suite)

The EF stateless-fixture suite is specific to the EVM-execution guest, so `spec-test` is an **l2-execution target**, not an orchestrated one (a rollup/aggregation guest has no equivalent). `make test` is the fast single-fixture smoke test; the full suite:

```bash
make -C l2-execution spec-test ZIG=/path/to/zig
make -C l2-execution spec-test ZIG=/path/to/zig SPEC_ARGS="--fork Amsterdam"
make -C l2-execution spec-test ZIG=/path/to/zig SPEC_ARGS="--match bal_self_transfer"
make -C l2-execution spec-test ZIG=/path/to/zig SPEC_ARGS="--report-only"
```

The runner walks the `blockchain_tests/` tree from the lazy `execution_spec_tests_zkevm` dependency and runs every block through the guest, failing if any output differs from the fixture's expected `statelessOutputBytes`. The corpus walking/reporting is reusable ([`spec_runner.zig`](l2-execution/test/spec_runner.zig)); a future extended-execution guest supplies its own input **adapter** ([`evm_spec_runner.zig`](l2-execution/test/evm_spec_runner.zig) is the vanilla one).

## Continuous Integration

[`riscv-guests-host-tests.yml`](../.github/workflows/riscv-guests-host-tests.yml) runs on every PR touching `riscv-guests/**`, with two parallel host-machine jobs:

- **Guest unit tests** — `zig fmt --check` plus the orchestrated `make test` (every guest in `GUESTS`).
- **l2-execution EF spec tests** — the full fixture suite via `make spec-test` (fail-hard; ~2,900 files / ~23k blocks, minutes on a warm cache).

The shared setup lives in [`.github/actions/setup-riscv-guests`](../.github/actions/setup-riscv-guests/action.yml): it installs the Zig pinned in `.zigversion` (via community mirrors — ziglang.org prunes dev builds), the apt crypto packages, and blst/mcl built from pinned upstream sources into `/usr/local`, with the builds and Zig package fetches cached. Running a guest **inside the ZKC interpreter** in CI is a separate, later stage: the `make compile` → ELF→JSON → `zkc` path works locally (see below); wiring it into CI (which also needs `zkc` + `go`) is still pending.

## ZKC Interpreter Integration

Running a guest in the ZKC interpreter goes ELF → JSON → `zkc`. `make compile` produces the statically-linked ELF (entry stub + rv64im memory layout from `build_common`'s `installGuestElf`, shared by all guests); the ELF→JSON conversion + `zkc` invocation are owned by [`arithmetization/src/test/Makefile`](../arithmetization/src/test/Makefile) (single source of truth). A guest's `exec`/`debug` build the ELF and **delegate** the run there:

```bash
make -C l2-execution exec  ZIG=/path/to/zig IN_BYTES=0x...
make -C l2-execution debug ZIG=/path/to/zig IN_BYTES=0x...
make -C l2-execution fixture-exec ZIG=/path/to/zig
```

These need `zkc` and `go` on `PATH`. The interpreter loads a finished ELF — `elf_to_json_gen` reads its `PT_LOAD` segments + entry point — so there is no relocatable-`.o` step (a `.o` is not statically linked, and the interpreter does not perform a final link).

## Guest Packages

Each guest folder is a complete package: its own dependencies (`build.zig.zon`), compile/test logic (`build.zig`), lifecycle (`Makefile`), production source (`src/`) and host-only test code (`test/`). Shared build helpers are factored into `build_common/`; the toolchain pin (`.zigversion`) is shared at this level.

- `l2-execution/`: vanilla EVM execution guest. See `l2-execution/README.md`.
```
