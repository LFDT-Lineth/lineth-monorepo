# System Code Generation

`verifier-ray` is built around dedicated verifier programs. The Zig verifier does not load and interpret a prover system at runtime. Instead, prover-ray compiles a WIOP `System`, verifier-ray extracts the compiled verifier metadata it needs, and the build/test flow emits Zig constants that are passed to verifier functions at comptime.

This keeps the production shape aligned with the eventual verifier application: one compiled prover system produces one dedicated verifier binary.

## Source Of Truth

`prover-ray` remains the source of truth for circuit compilation. For vanishing quotient checks, the important compiler pass is:

```text
prover-ray/wiop/compilers/global/global.go
```

That pass lowers module vanishing constraints into global quotient verifier actions. Each compiled `global.Verifier` contains the metadata needed by the Zig checker:

- the module being checked
- module size mode, from `Verifier.Module`
- witness column views and their evaluation claim cells
- quotient buckets, ratios, quotient claim cells, and vanishing expressions
- merge and evaluation coins

`verifier-ray/codegen` walks `System.Rounds[*].VerifierActions`, selects actions of type `*global.Verifier`, and converts them into a compact `vanishing.System` description for Zig.

## Extraction And Rendering

The codegen package is split by responsibility:

- `verifier-ray/codegen/vanishing.go` extracts prover-ray objects into Go structs used by verifier-ray.
- `verifier-ray/codegen/vanishing_zig.go` renders those structs as Zig source using `text/template`.
- `verifier-ray/codegen/coin_routing.go` extracts the protocol-level Fiat-Shamir coin layout into `CoinRouting`. Shared across all sub-verifiers; enforces the `round_coin_counts[0] == 0` invariant at generation time.
- `verifier-ray/codegen/spec_zig.go` renders `CoinRouting` as a standalone `protocol.Spec` Zig constant via `WriteSpecZig`.

The generated Zig describes data, not executable polynomial code. The evaluator in `src/query/vanishing.zig` consumes this data at comptime:

```zig
pub fn verify(comptime system: System, input: CheckInput) Error!void
```

For tests, generated prover scenarios live in:

```text
verifier-ray/testdata/generated/vanishing.zig
```

The generated file contains the extracted `vanishing.System` values and matching proof views side by side. This file is test fixture data for prover-ray scenarios; it is not part of the verifier library API.

The repository also commits one generated production-style system:

```text
verifier-ray/testdata/generated/riscv_system.zig
```

That file is produced by `verifier-ray/codegen/generate-riscv-system`, which compiles the honest RISC-V arithmetization entrypoint `arithmetization/src/main/riscv/main.zkc`, proves the one guest in `codegen`'s `HonestRiscvGuests` (`AllInOneGuestELF`, which exercises the full RV64I + M-extension + custom-precompile surface — memory round-trip, branches, load/store widths, the Poseidon2/Keccak/write-output precompiles, immediate ALU ops, and RV64 word-width ops — in a single witness), and renders the resulting verifier metadata as a `verifier.Systems` value. Unlike the broad scenario fixtures above, this path exists specifically to prove that verifier-ray can bootstrap a dedicated RISC-V verifier system from the real prover-side interpreter circuit rather than from a synthetic pub-input-only test program.

The matching real proof image is committed as `verifier-ray/testdata/riscv_proof_image.bin` and verified against this same system by `test/riscv_proof_image_test.zig`, the cross-language (Go writes, Zig mmaps and verifies) end-to-end check.

## Static And Dynamic Module Sizes

Module size is part of the generated system whenever prover-ray knows it at compile time:

```zig
pub const ModuleSize = union(enum) {
    static: usize,
    dynamic: usize,
};
```

Static modules use `.static = n`. Dynamic modules use `.dynamic = i`, where `i` indexes `CheckInput.module_sizes`:

```zig
pub const CheckInput = struct {
    ...
    module_sizes: []const usize = &.{},
};
```

The static path lets Zig specialize operations such as `r^n`, cancellation roots, and quotient recombination for a concrete domain size. Dynamic modules still validate the supplied size at runtime and use runtime exponentiation/root lookup.

## Why Comptime Matters

The vanishing checker intentionally uses `inline for` only for loops over generated metadata: modules, buckets, vanishings, and cancellation positions. Those loops keep expression indices and static module metadata comptime-known. Data loops, such as quotient-share recombination, remain ordinary `for` loops.

This distinction is important. If the generated `System` is traversed as ordinary runtime data, Zig can leave a VM-like expression evaluator in the compiled program. A small `ReleaseFast` experiment with one static module and expression `claim[0] + 7` showed the difference.

With comptime traversal and inline metadata loops, the exported function compiled to direct arithmetic:

```asm
<inline_entry>:
  mov rax, QWORD PTR [rdi]
  add rax, 0xb
  ret
```

The equivalent plain-loop version still materialized metadata and called a runtime evaluator:

```asm
<plain_entry>:
  ...
  call <evalExprPlain>
```

`evalExprPlain` contained runtime expression-node dispatch over the expression array. That is exactly what the generated verifier is trying to avoid, especially for zkVM execution where verifier step count matters.

The rule for this package is therefore:

- use comptime parameters for the generated verifier system
- use inline metadata loops when their loop variables feed comptime-only expression or static-size helpers
- keep ordinary loops for proof data and other runtime input

## Test Scenario Generation

The testdata generator imports prover-ray, builds each `wioptest.VanishingScenario`, runs the global quotient compiler, and extracts the resulting `global.Verifier` actions through `verifier-ray/codegen`.

The generator also runs prover-ray runtimes for honest and invalid assignments, then writes:

- verifier-visible initial round messages
- quotient round messages
- witness evaluation claims
- quotient evaluation claims
- dynamic module sizes, when required

These fixtures are regenerated with:

```bash
cd verifier-ray
make generate-testdata
```

The Zig tests consume `testdata/generated/vanishing.zig` and call `vanishing.verify` with the generated system at comptime.

## Production Direction

The test suite currently generates many systems because it covers many prover-ray scenarios. A real verifier application should normally have one serialized prover-ray compiled system. The expected build flow is:

1. prover-ray compiles the final `wiop.System`
2. verifier-ray codegen extracts once and renders into one generated file:
   - `BuildCoinRouting()` → `CoinRouting` → `WriteSpecZig()` → `pub const spec`
   - `CoinRouting` + `BuildVanishingSystem()` → `VanishingSystem` → `WriteVanishingSystemZig()` → vanishing constants + `pub const systems`
   - future sub-verifiers (`BuildLogDerivSystem()`, …) receive the same `CoinRouting`
3. the Zig build includes that generated file
4. the output binary is dedicated to that concrete system

Polynomial commitment verification is still out of scope for the current vanishing compatibility tests. PCS/FRI integration notes live in:

```text
verifier-ray/docs/vanishing-pcs-integration-notes.md
```
