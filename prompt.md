# Context for "transpile to zkc" prompts

This document exists because "transpile it to zkc" is not enough context for an
agent to act on. It answers seven background questions so that context can be
pasted (or linked) into a prompt instead of re-derived from scratch. Every claim
below is cited to a file:line in this repo, or in the vendored `zkc` Go module,
as of 2026-09-16. **Verify signatures against whatever `zkc` version is actually
pinned in the relevant `go.mod` before relying on them** — see the version note
in §5, and treat quoted line numbers as pointers to re-check, not eternal truths.

---

## 1. What is arithmetization, and where does it come from?

Arithmetization is the process of turning VM execution semantics into algebraic
constraints a proving system can consume — the bridge between an execution
trace and a cryptographic proof. `prover-ray/docs/section2_arithmetization.md:1-16`
(§2.1 Overview):

> Arithmetization is the process of converting VM execution semantics into
> algebraic constraints that a proving system can consume. It is the bridge
> between an execution trace and a cryptographic proof: it defines *what it
> means* for a trace to represent a valid RISC-V execution, expressed as a
> system of algebraic constraints over a finite field.
>
> In this system the arithmetization is not a hand-written circuit. It is a
> **RISC-V interpreter written in ZkC** (§2.5), compiled by the `zkc` toolchain
> into an algebraic constraint system. The interpreter defines the machine; the
> compiler turns its execution into constraints. The arithmetization therefore
> lives as ZkC source code — which is then regenerated into constraints on each
> build — rather than as a hand-maintained constraint listing.

`arithmetization/README.md:1-5` gives the concrete location and language:

> This directory holds the arithmetization of RISC-V, with target =
> `riscv64im_zicclsm-unknown-none-elf`.
>
> Arithmetization is written in ZkC, a simple imperative language designed
> primarily for writing programs whose executions can be proved.

So: it is **hand-written ZkC source** (an interpreter, currently named
`interpreter()`, under `arithmetization/src/main/riscv/{interpreter.zkc,main.zkc,memory.zkc,ram/,utils/}`
— per `section2_arithmetization.md` §2.9 "External References"), not corset-lang
and not a manually maintained constraint list. It is compiled fresh on every
build by the `zkc`/go-corset toolchain into an **AIR schema** (see §2 below).

**Design note worth carrying into any transpilation prompt** — this repo's
arithmetization deliberately avoids a single monolithic CPU trace.
`section2_arithmetization.md:18-26` (§2.2, "no-CPU architecture"):

> The arithmetization follows a **no-CPU** style... What it names is the choice
> to avoid materializing the program's trace as **one monolithic chip / module
> whose trace size is $O(n)$ in the number of executed R5 instructions**.
> Instead the work is distributed across a collection of specialized
> **tables**, each with its own much smaller trace. This design originates
> with **OpenVM** (initiated by Axiom, Scroll, and collaborators).

**Caveat:** there is a second, older, unrelated arithmetization system in this
repo — the legacy Java/Besu EVM tracer (`docs/features/tracer.md:1-18`,
`tracer/arithmetization/`, Rust `corset/`). That system is not what "transpile
to zkc" refers to; it's mentioned here only so an agent doesn't confuse the two
when grepping for "arithmetization" and land on stale/legacy docs.

---

## 2. What does the prover do with the arithmetization to get its representation?

The prover ("R5 zkVM", built on gnark) does not read ZkC source directly. It
reads a **compiled binary constraints file** and translates it into its own
internal IR. `prover-ray/AGENTS.md:9-13`:

> The proving pipeline works as follows. The `go-corset` library supplies the
> circuit description and witness assignments for the zkEVM arithmetization.
> The prover extends this circuit with constraints for precompiles and public
> inputs, then compiles it using the custom proving framework in
> `./protocol/wiop` to produce an inner proof system.

Concretely, three steps:

**a) Load the compiled constraints file (a `.bin`).** This is a gob-encoded
file containing metadata (e.g. build git commit) plus a serialized AIR schema.
`prover-ray/zkcdriver/files.go:22-54` (`ReadConstraintsFile`/`UnmarshalConstraintsFile`)
and `prover-ray/zkcdriver/zkcdriver.go:19-26`:

```go
// BinaryFile represents a given set of constraints generated from a ZkC
// program. ... it provides access to the AIR constraints representing the
// given ZkC program; secondly, it provides a means to generate a trace of
// that program from a given set of inputs.
type BinaryFile = constraints.BinaryFile[koalabear.Element]
```

**b) Extract the AIR schema and compile it into the prover's own IR
(`wiop.System`).** `prover-ray/zkcdriver/zkcdriver.go:52-72` (`NewZkCDriver`):

```go
binf, metadata, errS := ReadConstraintsFile(bin)
schema := binf.AirConstraints()      // air.Schema[koalabear.Element]
Define(sys, &schema)                 // translate air.Schema -> wiop.System
```

`Define` (`prover-ray/zkcdriver/definition.go:65-88`) walks the schema's modules,
columns, and constraints and registers equivalents in `wiop.System`:

```go
func Define(sys *wiop.System, schema *air.Schema[koalabear.Element]) {
	modules := schema.Modules().Collect()
	scanner := &schemaScanner{Sys: sys, Schema: schema, Modules: modules, ...}
	scanner.scanColumns()
	scanner.scanConstraints()
	...
}
```

`section2_arithmetization.md:193-199` names this the concrete AIR→prover bridge:

> The **`zkcdriver`** integration layer is the concrete bridge: it scans the
> `air.Schema`'s modules, columns, and constraints into a `wiop.System`
> (preserving a corset-name → column map), and populates the `Runtime` with the
> trace. The proving system is therefore independent of arithmetization
> details: it sees only columns and constraints.

**c) Get the trace/assignment into the same representation.** Independently of
the constraint schema, an execution trace is produced (`BinaryFile.Trace`, §3
below) and copied cell-by-cell into a `wiop.Runtime` via
`ZkCDriver.AssignTraceShard` → `AssignFromTraceShard`
(`prover-ray/zkcdriver/assignment.go:22-97`), matching corset-qualified column
names between the two representations.

**d) Downstream of that**, the `wiop.System` + `Runtime` pair is handed to the
Arcane compiler (`prover-ray/docs/section1_goals_and_objective.md:192-196`:
"reducing the Wizard-IOP to a Poly-IOP... and closes it with the polynomial
commitment scheme"), i.e. Wizard IOP → Arcane → Vortex PCS → Proof. That stage
is outside the scope of "arithmetization" itself; the AIR/trace boundary is the
relevant interface for transpilation work.

`section2_arithmetization.md:210-217` frames why this boundary (`.bin` file)
matters practically: "The pre-compiled binary (`.bin`) that `zkcdriver` reads
is a serialized AIR schema, so the AIR boundary is also the boundary at which
the arithmetization can be cached and reloaded without recompiling from
source."

---

## 3. How to obtain traces (see `r5_test.go`)

`prover-ray/zkcdriver/r5_test.go` is a single benchmark
(`BenchmarkRisc5Arithmetization`), not a table of unit tests — the file
comment explains why (`r5_test.go:13-14`: "a benchmark ... and not a test so
that we don't crash the CI on every PR"). The bulk of R5 trace/prove/verify
benchmarks actually live in the sibling file
`prover-ray/zkcdriver/r5_benchmark_test.go`.

"R5" is the codename of this repo's RISC-V zkVM itself (not a pipeline-stage
count) — `prover-ray/docs/README.md:1,3-4`: "R5 zkVM Specification ... a
hash-based, post-quantum-friendly proof system for RISC-V execution targeting
blockchain and rollup workloads."

**The canonical trace-obtaining call chain**, as used by `r5_test.go:31` via
the shared helper `traceZkc` (`prover-ray/zkcdriver/example_test.go:106-137`):

```go
outputs, tr, errs := binFile.Trace(input, tracingCfg)
if withCheck {
	errsSchema := binFile.Check(tracingCfg, tr)
	// non-empty errsSchema => constraints violated (see the exit-code gotcha in §5)
}
```

`binFile` is a `*constraints.BinaryFile[koalabear.Element]` (from the vendored
`zkc` module). `Trace` is implemented at
`.../zkc@v1.2.32/pkg/zkc/constraints/binary_file.go:311-333`:

```go
func (p *BinaryFile[F]) Trace(input map[string][]byte, cfg vm.TraceConfig,
) (output map[string][]byte, trace trace.Trace[F], errors []error) {
	builder := vm.NewTraceBuilder[vm.Uint32, F, Tracer[F]](cfg, p.TracingProgram(), p.TracingProgram())
	trace, output, errors = builder.BootAndTrace(input)   // actually runs the interpreter
	if len(trace) > 0 {
		trace, errs = p.expandTrace(cfg, trace)            // expands raw trace against the AIR schema
	}
	return output, trace, errors
}
```

**Trace type**: `trace.Trace[F] = []Shard[F]` (sharded), each `Shard[F]` holds
named `Module[F]`s, each `Module[F]` is `{height, descriptor (name + column
descriptors), columns []array.Array[F]}` — i.e. a sharded, column-major table
of named modules (`.../zkc/pkg/trace/{trace,module}.go`). There is no separate
"register state"/"memory event" struct; registers, RAM, and control state are
just named columns inside named modules — consistent with the no-CPU design in
§1.

**Where the *inputs* to `Trace` come from** (this is the part `r5_test.go`
demonstrates and that matters most for transpilation prompts): `r5_test.go:26`
calls `predecoding.PrepareInputs(verifElf, payload)`
(`arithmetization/gopkg/predecoding/decode.go:711-746`):

```go
func PrepareInputs(elfBytes []byte, inputData []byte, options ...Option) (map[string][]byte, error) {
	program, _ := elfmapping.Load(bytes.NewReader(elfBytes))
	decoded, _ := predecode(program, cfg.maxDecodedRecords)
	inputBlobs, _ := elfmapping.NewData(elfmapping.DefaultInputOrigin, inputData, elfmapping.WithLengthPrefix())
	inputs, _ := elfmapping.EncodeInputs(program, inputBlobs, cfg.mappingOptions...)
	maps.Copy(inputs, decoded.EncodeInputs())
	return inputs, nil
}
```

This takes a RISC-V ELF (guest program bytes) plus a guest input payload and
returns the `map[string][]byte` that `BinaryFile.Trace`/`Execute` accept
directly — **entirely in Go, in one process, no JSON file and no CLI**. This is
the modern, direct route; see §4 for how it differs from the older
JSON/Makefile route, and §5 for why this matters for "no os.Exec" transpilation
work.

**Production equivalent** (same `Trace` call, wrapped for the live driver):
`ZkCDriver.TraceZkcInputs` (`prover-ray/zkcdriver/zkcdriver.go:101-136`).

**Other files that demonstrate the same pattern** (all in
`prover-ray/zkcdriver/`): `example_test.go` (shared harness:
`compileBinaryConstraints`, `parseTestCase`, `traceZkc`, `runProveVerify`),
`zkcdriver_test.go` (`TestRunZKCExamples`), `sync_test.go`
(`TestZkcIntegrationTestSynced`), `native_modules_test.go` (`runZkcCase`),
`r5_benchmark_test.go` (`loadR5BenchmarkFixture` — walks
`trace.Shard`/`trace.Module` directly to compute per-shard cell counts, the
clearest example of *inspecting* a trace object).

---

## 4. How to serialize traces — we currently target Zig; prefer something else for zkc

There is **no dedicated "trace serialization format for Zig"** in this repo —
that phrase conflates two different things that are worth separating clearly
in any prompt:

**a) Zig's actual role is as the *guest program source language*, not a trace
target.** `riscv-guests/README.md:1-3`: each guest under `riscv-guests/` "is a
self-contained Zig package" compiled by the ordinary Zig compiler to a RISC-V
ELF. The **current, CLI/Makefile-oriented** path from that ELF into `zkc` is:

> `riscv-guests/README.md:96`: Running a guest in the ZKC interpreter goes
> **ELF → JSON → `zkc`**. `make -C l2-execution compile` produces the
> statically-linked ELF...; the ELF→JSON conversion + `zkc` invocation are
> owned by `arithmetization/src/test/Makefile`.

`arithmetization/cmd/elf_to_json/README.md:1-19` — this JSON is an **input**
file (`entry_point_and_blobs_count`, `blobs_offset_and_size`, `blobs_data`,
`instruction_base`, `decoded`), not an execution trace; it's what `zkc
exec`/`zkc trace` reads as their program input, and today it's produced by
shelling out to a separate `elf_to_json` binary from a Makefile. Tracing itself
is also flagged as incomplete on this path:
`riscv-guests/README.md:90,103`: "`--fast` ... skips tracing, which is not
implemented yet" / "what CI uses while the interpreter's trace path is
unimplemented" (for the *guest* CI workflow specifically — `zkc trace` itself
works and is used elsewhere, see `arithmetization/src/test/Makefile:137-159`).

**b) The actual execution trace, once produced, is a Go-native
`trace.Trace[F]` object (§3) — it is never serialized to bytes in the
`zkcdriver` prove/verify path at all.** It's consumed in-process:
`AssignTraceShard` copies it straight into a `wiop.Runtime`
(`prover-ray/zkcdriver/example_test.go:160-206`,
`prover-ray/zkcdriver/assignment.go:23-97`). The only thing that *is*
byte-serialized in that path is the **constraint schema** (`BinaryFile.MarshalBinary`,
gob-encoded `air.Schema`+program — `example_test.go:170-178`), for a
round-trip sanity check, not the per-execution trace data.

**c) The one place a genuinely Zig-ABI-shaped binary format exists is *proof*
serialization, not trace serialization** — worth knowing so it isn't confused
with what's being asked for. `prover-ray/wiop/proofserialization/README.md:1-16`:

> The layout is not a free choice: it mirrors the Zig ABI of verifier-ray's
> `verifier.Proof`, measured from the compiler and pinned by
> `verifier-ray/src/proof_abi.zig`... Produce, from a `wiop.Proof`, a single
> contiguous byte image that the Zig verifier consumes with **zero decode
> work**... The image doubles as the guest's RAM witness: it is written into
> the ZkC input region and the guest reads its proof straight out of that
> memory.

**What's actually preferable when working through `zkc`'s Go API directly:**
skip file-based JSON serialization entirely. `predecoding.PrepareInputs` (§3)
already returns the native `map[string][]byte` shape `BinaryFile.Trace`/`Execute`
want — so the ELF→JSON→CLI round-trip (`elf_to_json` binary + Makefile +
`zkc` subprocess) can be replaced by one in-process Go call
(`predecoding.PrepareInputs(elfBytes, payload)` → `binFile.Trace(inputs, cfg)`),
with no intermediate serialization format at all. This is exactly what
`r5_test.go` itself does (§3) — it is the existing precedent to point an agent
at, not something to invent.

---

## 5. How to run the zkc tracer using Go APIs (no CLI, no Makefiles, no `os.Exec`)

**Version caveat, check this first:** the `zkc` module version pinned in this
repo's `go.mod` files (`arithmetization/go.mod`, `prover-ray/go.mod`) is
`v1.2.32`; a different, slightly older commit
(`v1.2.25-0.20260724062105-9d6ddad19ab7`) is also present in the local module
cache and has minor signature differences (`Check`'s argument order,
`TraceConfig`'s package, `NewBinaryFile`'s arity, `Program()` vs
`RawProgram()`/`TracingProgram()`/`ExecutionProgram()`). **Always check the
consuming module's own `go.mod` for the pinned version rather than assuming
one** — code below is quoted against `v1.2.32` (matching this repo) unless
noted.

**Confirmed: every `zkc` CLI subcommand is a thin wrapper around plain Go
functions** — `cmd/zkc/main.go:15-19` is just `zkc.Execute()`, and every
subcommand's implementation calls the same functions available to any Go test:

| CLI command | Underlying Go call |
|---|---|
| `zkc compile file.zkc -o out.bin` | `compiler.Compile` → `ast.Compile` → `constraints.NewBinaryFile` → `MarshalBinary` |
| `zkc compile --air`/`--stats` | `constraints.GenerateAirConstraints[W,F]`, `debug.PrintAnySchema`, `PrintCompileStats` |
| `zkc trace input.json file.zkc [-c]` | `binf.Trace(input, cfg)`, then `binf.Check(cfg, tr)` if `-c` |
| `zkc execute input.json file.zkc` | `binf.Execute(input, n)` (→ `vm.BootAndExecute` internally) |
| `zkc execute -c` | `binf.Trace(...)` then `binf.Check(...)` — same as `trace -c` |

**Minimal pure-Go recipe** (no files, no subprocess) — compile ZkC source held
as a Go `[]byte`/string, trace it, check constraints:

```go
srcFile := source.NewSourceFile("myprogram.zkc", zkcSourceBytes) // no disk I/O required
macroProgram, _, errs := compiler.Compile(field.KOALABEAR_16, maxStaticHeight, *srcFile)
ir, errs := ast.Compile(macroProgram, codegenCfg)
binf := constraints.NewBinaryFile[koalabear.Element](nil, nil, ir)

inputs := vm.FilterInputs(binf.TracingProgram(), myInputsMap) // map[string][]byte
_, tr, errs := binf.Trace(inputs, vm.DEFAULT_TRACE_CONFIG)
failures := binf.Check(vm.DEFAULT_TRACE_CONFIG, tr)
// len(failures) == 0 && len(errs) == 0  =>  constraints hold
```

This exact shape is already proven out with zero `os/exec` in this repo:
`arithmetization/gopkg/embedded/embedded.go:163-201` +
`embedded_test.go:7-15` (compile + validate), and
`prover-ray/zkcdriver/zkcdriver_test.go:42-137` (`compileBinaryConstraints`,
`traceZkc` — compile from source bytes, trace, check). It's also how the `zkc`
module tests itself: `pkg/test/util/check_valid.go:439-469`
(`testConstraintsWithField`) is the module's own reference implementation of
"compile → trace → check", exercised by `pkg/test/zkc_unit_test.go` and
friends via plain `go test`.

**Practical implication for transpilation prompts:** a Go test that wants to
compile-and-check a ZkC program never needs a Makefile target or `os.Exec` —
it needs `compiler.Compile`, `ast.Compile`, `constraints.NewBinaryFile`,
`.Trace(...)`, `.Check(...)`, all importable directly from
`github.com/LFDT-Lineth/zkc/pkg/zkc/...`.

**Known gotcha to carry into any such test** (from prior work in this repo,
see memory `zkc-execute-exit-code-not-an-oracle`): `zkc execute -c`/`Check`
can report a constraint violation while the process/call still returns without
a hard error — a violated-AIR result surfaces as a non-empty `[]schema.Failure`
(or, at the CLI, `stderr` text with `level=error`) rather than a non-zero exit
code alone. Any Go test built on this must check `len(failures) == 0`
explicitly, not just the absence of a Go `error`.

---

## 6. How to define a ZKC program and run it

There is **no Go builder/fluent API for constructing ZkC program logic** — the
only supported flow is: **write `.zkc` source text (Go string/byte slice is
fine — no file required), then compile+run it via the Go functions in §5.**

```go
src := source.NewSourceFile("filename-for-diagnostics.zkc", []byte(`
	pub input in(v: u32) -> (out: u32)
	pub fn main() { ... }
`))
prog, _, errs := compiler.Compile(field.KOALABEAR_16, maxStaticHeight, *src)  // parse+link+typecheck
ir, errs := ast.Compile(prog, codegenCfg)                                     // AST -> vm.Program (bytecode IR)
binf := constraints.NewBinaryFile[koalabear.Element](metadata, attrs, ir)     // wrap for trace/execute/check
```

`filename` in `source.NewSourceFile` (`pkg/util/source/source_file.go:81`) is
purely for error messages — it does not need to correspond to a real path.
This in-memory pattern is exactly what
`arithmetization/gopkg/embedded/embedded.go:52-53` does when compiling the
whole `arithmetization/src/main/riscv/` tree from embedded bytes, and what
`prover-ray/zkcdriver/zkcdriver_test.go:55-71` does per-test-case.

Once compiled into a `*constraints.BinaryFile[F]`, "running" it means one of:

- `binf.Execute(input, n)` — plain execution (fast, no trace/constraints)
- `binf.Trace(input, cfg)` — execution + trace (§3)
- `binf.Trace(...)` + `binf.Check(...)` — execution + trace + constraint
  verification (§5's table)

If a prebuilt `.bin` already exists (produced once via the pipeline above),
skip recompiling ZkC source entirely: `constraints.BinaryFile[F].UnmarshalBinary(data)`
(`pkg/zkc/constraints/binary_file.go:254` in the `v1.2.25` cache; equivalent in
`v1.2.32`) loads it directly — this is what production code
(`ZkCDriver.NewZkCDriver`, §2) does when given a `.bin` produced by CI rather
than compiling from source on every run.

---

## 7. How to get a repeatable arithmetization description usable in ZKC (a la codegen for a Spec)

**There is no `zkc spec` subcommand and no built-in JSON/protobuf export of the
arithmetization.** What exists instead is a deterministic, programmatically
walkable **Go object graph**:

```go
schema := constraints.GenerateAirConstraints[W, F](program, fieldCfg, maxStaticDepth) // air.Schema[F]
// or, cached on an already-compiled BinaryFile:
schema := binf.AirConstraints() // air.Schema[F]
```

`air.Schema[F]` is a type alias for `schema.UniformSchema[F, Module[F]]`
(`pkg/ir/air/schema.go:35`) and is walkable via the generic `schema.Schema`
interface (`pkg/schema/schema.go:39`):
`.Modules() iter.Iterator[Module[F]]`, and per module
(`pkg/schema/module.go:55`): `.Name()`, `.Registers() []register.Register`
(the column layout), `.Constraints() iter.Iterator[Constraint[F]]`, `.Width()`.
Given the same compiled program, this call is fully deterministic and
repeatable — the "codegen for obtaining Spec" the question is asking for
already exists at this level, it's just a Go struct graph rather than a
serialized artifact.

There is no off-the-shelf serialization of that graph. The one real precedent
in this repo for turning it into something durable is exactly the
`zkcdriver.Define`/`schemaScanner` code from §2
(`prover-ray/zkcdriver/definition.go:65-88`) — bespoke, hand-written Go that
walks `air.Schema[F]` once and re-registers everything into `wiop.System`. Any
new "Spec" consumer (including a zkc transpilation target) should expect to
write the analogous walk, not find a ready-made schema file.

A useful **pattern precedent** for making that walk repeatable/build-time
rather than ad hoc, from elsewhere in this repo (Zig verifier codegen, not the
ZkC arithmetization layer, but the same shape of problem — "derive a
repeatable downstream artifact from a compiled schema"):
`verifier-ray/docs/system-codegen.md:3`:

> The Zig verifier does not load and interpret a prover system at runtime.
> Instead, prover-ray compiles a WIOP `System`, verifier-ray extracts the
> compiled verifier metadata it needs, and the build/test flow emits Zig
> constants that are passed to verifier functions at comptime.

That is: compile once, walk the compiled representation once, and codegen a
static artifact — the same recipe applies to extracting a ZkC-consumable
description from `air.Schema[F]`, rather than re-parsing/re-walking it at
runtime on every consumer.

Two adjacent, already-existing (but human-facing, not machine-readable) views
of the same schema, useful for debugging a transpilation rather than for
programmatic consumption: `constraints.Validate[F](schema) []error`
(`pkg/zkc/constraints/validate.go:31` — checks every register is touched by
some constraint; used by `zkc compile`'s validation step and by
`arithmetization/gopkg/embedded/embedded.go`'s `WithAirValidation()`), and
`PrintCompileStats`/`debug.PrintAnySchema` (`pkg/cmd/zkc/stats.go:151`,
`pkg/cmd/corset/debug/schema.go:43` — human-readable column/constraint-degree
tables behind `zkc compile --stats`/`--air`, terminal output only).

---

## Related gotchas already known in this repo (worth attaching alongside this doc)

These are from prior zkc precompile work in this repo and are likely to bite
any zkc transpilation task; each is tracked in more detail in project memory:

- `zkc execute -c` can report a violated constraint on stderr while still
  exiting 0 — never trust exit code alone (`zkc-execute-exit-code-not-an-oracle`).
- `zkc compile` hard-errors on any module unreachable from `main` — no
  `#[allow_unused]` escape hatch exists; incremental library code must stay
  100% reachable from the compile entry point (`zkc-rejects-unreachable-modules`).
- A caller that branches on two separate return values from one multi-return
  call inside a loop can panic the tracing transform ("conflicting read on
  register") — collapse to a single return value
  (`zkc-multi-return-conditional-conflict`).
