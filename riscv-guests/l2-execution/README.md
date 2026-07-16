# L2 Execution Guest

This package contains the RISC-V guest program for the Rollup's extended l2-execution proof: the Linea-layer logic (`l2_execution.zig`) on top of per-block stateless execution — conflation of a contiguous block range, forced transactions, the L1<->L2 message bridge, and the 16-field public-input tuple. Per-block execution itself is delegated to a log-preserving seam (`execution.zig`) over Zesu's stateless executor, which decodes an SSZ-encoded `StatelessInput` and executes the block.

## Scope

- Decodes the extended `L2ExecutionProofPrivateInput` SSZ envelope (a contiguous run of payloads, each carrying an opaque vanilla `SszStatelessInput` plus forced-transaction witnesses), runs `l2_execution.runL2Execution`, and emits the SSZ output — `keccak256` of the public-input tuple plus the revealed hash preimages the rollup guest needs.
- The native Zig tests replay a real execution-spec-tests `tests-zkevm` fixture and hand-built fixtures against Python-oracle-computed expected values (see `Readme.md` §6.3/§6.5/§2.1); `zig build extended-vanilla` reference-tests the whole EF zkevm corpus by wrapping each block into a dummy-filled extended input and checking the extended guest's validity verdict against the fixture's own expected result — the reference-test corpus is the source of truth, not a second re-run implementation.
- Does not include blob compression or recursive proof aggregation — those are the rollup/rollup-aggregation guests' concern.
- Keeps cryptographic precompile/signature acceleration behind Zesu's `accel_impl` boundary. The freestanding guest leaves the `zkvm_*` accelerator symbols **unresolved** for the proving system to supply/intercept — there is no in-guest software provider. The native host test instead links Zesu's `default.zig` backend against system crypto libraries (see [Native test dependencies](../README.md#native-test-dependencies)).

## Development

The Zig version, dependency checkout, build manifest, and ZKC helper commands are shared by all guests at `riscv-guests/`.

Run from the parent directory:

```bash
make -C l2-execution exec
```

`make -C l2-execution compile` writes the guest as a **statically-linked rv64im ELF** to `riscv-guests/l2-execution/zig-out/bin/evm_execution_guest` — the [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md) artifact ("Object Format: ELF, statically linked"), linked via `build_common`'s shared `installGuestElf`. The ZKC interpreter loads it (via ELF→JSON); `make -C l2-execution exec` builds it and runs it there — see the [parent README](../README.md#zkc-interpreter-integration). `make test` runs the native Zig test, which requires the native crypto libraries documented in the [parent README](../README.md#native-test-dependencies).

## Compilation

`make -C l2-execution compile` (and `exec`/`debug`) build the guest with
the **standard** zig keccak by default. Pass `KECCAK_ACCEL=true` to build with the
arithmetization keccak wrapper (the prover-accelerated custom op) instead:

```bash
make -C l2-execution compile                     # standard zig keccak
make -C l2-execution compile KECCAK_ACCEL=true   # arithmetization keccak wrapper
```

Equivalently, running `zig build` directly from this directory (requires the generated linker script; run `make linker-script` once after a clean checkout):

    make linker-script
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
        exec KECCAK_ACCEL=true INPUT="$input" "${@:2}"
}
```

`realpath` resolves the input against your current directory *before* `make -C`
switches into the guest directory (and the `|| return` aborts on a bad path
instead of launching a build with no input); `/usr/bin/time -p` prints wall-clock
real/user/sys.
