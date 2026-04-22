# AGENTS.md — prover

> Inherits all rules from [root AGENTS.md](../AGENTS.md). Only overrides and additions below.

## Package Overview

This is the ZK prover for Linea, written in Go and built on top of gnark.
It implements the Vortex polynomial commitment scheme and the Consensys zkEVM.
The proving pipeline works as follows. The `go-corset` library supplies the
circuit description and witness assignments for the zkEVM arithmetization.
The prover extends this circuit with constraints for precompiles and public
inputs, then compiles it using the custom proving framework in
`./protocol/wiop` to produce an inner proof system.

## How to Run

```bash
# Run tests (excludes corset, includes fuzz-light)
cd prover && go test ./... -timeout 30m

# Static checks
cd prover && gofmt -l .
cd prover && golangci-lint run
```

## Go-Specific Conventions

- **Go version:** 1.25.7 (see `go.mod`)
- **Formatting:** `gofmt` (standard Go formatting, tabs)
- **Linting:** `golangci-lint`
- **Indentation:** tabs (per `.editorconfig` Go section)
- **Cross-compilation:** Targets Linux AMD64 (musl) and Darwin ARM64

### Key Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/consensys/gnark` | ZK circuit compiler |
| `github.com/consensys/gnark-crypto` | Cryptographic primitives |
| `github.com/consensys/go-corset` | Constraint system |

### Directory Structure

```
backend/        Backend logic
crypto/         Low-level cryptographic utilities
maths           Low-level maths functions, finite field arithmetic.
wiop/       Main framework for the description of the ZK-EVM and the implementation of the proof system.
config/         Configuration
```

## Prover-Specific Safety Rules

- Never commit proving keys or large binary assets (checked via `.gitignore`)
- Cryptographic code changes require careful review — affects proof validity
- Test timeouts are 30 minutes due to proof generation complexity

## Testing

To test the package:
- CI workflow: `.github/workflows/prover-testing.yml`
- Static checks run first (gofmt, golangci-lint)
- Compressor tests run separately from main test suite
- 30-minute timeout for test suite

### Conventions

- **Naming:** `TestFoo` for the happy path, `TestFoo_Scenario` for sub-cases
  (e.g. `TestVerify_InvalidSignature`).
- **Table-driven:** Use `t.Run` with a case slice when there are 3 or more
  input variants.
- **One concern per test:** Each test should verify a single behavior. If a
  test needs multiple independent scenarios, split it into sub-tests.
- **Failure messages:** Include a description in assertions:
  `require.Equal(t, want, got, "after padding, slice length should be n")`.
  A failing test must say what it expected, not just that it failed.
- **Test independence:** Tests must not share mutable state. Each test must
  be runnable in isolation with `go test -run TestFoo`.
- **Package naming:** Use `package foo_test` (black-box) by default. Switch
  to `package foo` (white-box) only when the behavior to test is unreachable
  through the public API.
- **Negative tests:** Every non-trivial function must have at least one
  failure case that asserts the expected error.
- **Circuit soundness tests are mandatory.** For every circuit, write at
  least one test that provides an invalid witness and asserts proof
  generation fails. A circuit tested only with valid witnesses is untested
  against its core security property.
- **Regression tests:** When a bug is fixed, add a test named
  `TestFoo_Regression_ShortDescription` that would have caught it.
- **Benchmarks:** Add `BenchmarkFoo` for any function on a hot path or where
  algorithmic choice matters. Run with `go test -bench=. -benchmem`.
- **Assertions:** Use `require` (stops the test immediately) for setup and
  preconditions. Use `assert` for independent checks within the same test.

### Randomized and Fuzz Testing

- For mathematical functions, validate properties over random inputs rather
  than hand-crafted cases only. Use a deterministic seed (e.g. `chacha8rand`
  with a fixed seed) so failures are reproducible.
- Add `FuzzFoo` tests for serialization and deserialization entry points —
  these are common sources of bugs at input boundaries. Use the `fuzzlight`
  build tag for corpus-driven fuzz tests that run in CI.
- Every known edge case found in production should have a corresponding fuzz
  corpus entry.

### Slow Tests

- Tests that run the full prover or generate proofs are slow. Guard them so
  under build-tags like fuzzlight so that they do not block fast feedback:

## Agent Rules (Overrides)

- Always run `gofmt` and `golangci-lint` before proposing Go changes
- Do not touch circuit code without explicit user directive (see Circuit Rules 
    below)
- Binary assets in `prover-assets/` are version-controlled selectively — check 
    `.gitignore` exceptions

## Code Style

### Package Structure

- Fewer packages is better. Before creating a new package, propose the structure 
    to the user — do not define new packages without consent.
- `./utils` is for domain-agnostic functionality — an extension of the standard 
    library or key dependencies. 
- It is not a dumping ground for orphan functions.

  **May go in:** assertion helpers (`utils.Require`), parallel execution 
    primitives, generic slice operations (`RightPadWith`, `SortedKeysOf`), safe 
    numeric conversions (`DivCeil`, `NextMultipleOf`), iterator combinators 
    (`ChainIterators`) — anything explainable without domain knowledge.

  **Cannot go in:**
  - Functions with fewer than 3 call sites in the codebase.
  - Functions already available in the standard library or key dependencies, or 
    that are trivial one-liners.
  - Functions that require domain knowledge to explain — if understanding it 
    requires knowing what a ZK-EVM, a Vortex polynomial, or a Compiled-IOP is, 
    it does not belong here.

### Dependencies

| Source | Policy |
|--------|--------|
| Go standard library | Always allowed |
| `golang.org/x` packages | Allowed; notify the user |
| Packages already in `go.mod` | Allowed; notify the user |
| New external dependencies | Requires user consent |

New external dependencies are discouraged but not prohibited. When proposing 
one, explain why it is preferable to a local implementation.

### Abstractions

- Prefer extending existing types and interfaces over creating new ones.
- Only introduce a new struct or interface if it reduces maintenance cost or 
    complexity.

### Naming

- Prefer names that make comments unnecessary. If a name requires a
  comment to be understood, the name is wrong.

### Size Limits

| Unit | Target | Hard limit |
|------|--------|------------|
| Line length | ≤ 80 chars | 120 chars |
| File length | — | 1000 lines |
| Function length | ≤ 50 lines | — |

Regarding function length, prefer smaller functions. If you can factor out a 
common functionality from several overlapping functions, do it but the behavior 
of the function should be easy to explain.

### Error Handling

- Return errors when failures are expected or recoverable at the call site.
- Panic on invariant violations — use `panic` or `utils.Require` when a
  condition should never be false given correct usage.
- Use hard assertions liberally in internal APIs. Particularly,
  * For function or structure invariants.
  * For function pre-conditions
  * Pay attention to nil-ness checks
  * Length consistency checks

## Documentation

The documentation philosophy of the project follows a 4-tier system:

1) docs.go (godoc): Package introduction - what you'll find here. 
    * Entry point for users browsing docs. 
    * May contain examples, especially if they are user-facing.

2) godoc comments: 
    * What the function does, 
    * Inputs/expectations, preconditions, invariants.
    * Avoid obvious statement (like "Sum adds a and b") if possible

3) Inline comments:
    * Giving context ("this is a non-trivial bug fix")
    * Instructions to maintainers ("don't reorder these")
    * Design decisions ("implemented this way not that way because...")
    Avoid using them for
    * Stating TODOS, issues are the right tools for TODOs - not comments.
    * Paraphrasing the code

4) Readme: 
    * Design rationale
    * Algorithms
    * Workflows
    * Security analysis. 
    * NOT about code/API - about context
    * Are located in the same package as the code implementing them
    * Should be kept reasonably concise
    * If more content is needed, split the Readmes in smaller file, each focusing on their own topic.

Generally speaking, all of these should embrace conciseness and try to maximize 
signal-to-noise ratio. Favor good naming and simpler design when possible.

## Circuit Rules

Circuit definitions encode security-critical constraints. They are
off-limits without an explicit directive from the user.

- Do not modify, or generate `Define()` implementations or constraint 
    expressions without explicit user instruction. This applies to any function 
    that takes in a gnark's `frontend.API` in their parameter.
- Do not add, remove, or reorder fields in circuit structs without explicit user 
    instruction — this changes the verification key and invalidates existing 
    proofs.
- Explaining circuit code is always fine.
- Be proactive in raising concerns about soundness of a circuits.
- Do not insert, modify or remove items in a Compiled-IOP without explicit 
    user instruction.
- Do not modify any method with the signature `Check(ifaces.Runtime) error`.
    * But be proactive in raising concern regarding potentially missing checks.
- Do not remove any statement registering a constraints or a query in the wizard 
    protocol.

### Documentation (when writing or modifying circuits)

Every circuit struct must carry a comment documenting:

- **What it proves.** One or two sentences in plain language, written for an 
    auditor who is unfamiliar with the codebase.
- **Public vs private.** Each field must be annotated `// public` or 
    `// private`. Public inputs are visible to the verifier; private fields are 
    known only to the prover.
- **Constraint budget.** Approximate constraint count and the dominant cost 
    (e.g. "~500k constraints, dominated by Keccak permutations").

## Performance

### Tooling

- Profile before optimizing: `go test -cpuprofile cpu.prof` then
  `go tool pprof` for CPU, `-memprofile mem.prof` for heap.
- New algorithmic choices on hot paths require a benchmark comparison
  (`go test -bench=. -benchmem`) before committing.
- Use `go tool trace` to diagnose goroutine scheduling and GC pauses.

### Allocations

- Pre-allocate slices when the size is known: `make([]T, 0, n)`.
- Pass buffers as parameters rather than allocating and returning them to lets 
  callers reuse allocations across calls.
- Avoid `interface{}` / `any` boxing in hot loops — boxing forces a heap 
  allocation for the value.
- Prefer arrays over slices for fixed-size data — no pointer, no GC pressure, 
  better cache behaviour.
- Avoid `big.Int` in hot paths — every arithmetic operation allocates. Use the 
  field element types from gnark-crypto instead.
- Measure with `-benchmem` to track allocation pressure.

### CPU

- Order struct fields largest-to-smallest to minimize padding.
- Avoid `fmt.Sprintf` in hot paths; use `strings.Builder` or direct byte 
  manipulation.
- For data-parallel loops over large arrays, prefer struct-of-arrays (SoA) 
  layout over array-of-structs (AoS) for cache locality.

### Parallelism

- Use `parallel.Execute` for parallel computation. Assume 100+ CPUs.
- Avoid channels and locks for single-task dispatch — the scheduling overhead 
  dominates. Prepare uniform-shaped work chunks for `parallel.Execute` instead; 
  use `runtime.GOMAXPROCS(0)` for sizing.