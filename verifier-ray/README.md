# verifier-ray

`verifier-ray` is the Zig verifier package for Ray proofs. Its job is to
reimplement the verifier-side pieces of `prover-ray` with a small, fixed
runtime that can eventually be used inside a zkVM precompile.

The package is intentionally independent from `prover-ray` at runtime. Tests and
fixture generation may import `prover-ray`, but the Zig verifier library under
`src/` does not.

## What Lives Here

- `src/field/` implements Koalabear base and extension field arithmetic.
- `src/crypto/` implements Poseidon2 and the Fiat-Shamir transcript.
- `src/pcs/` implements polynomial helpers used by verifier logic.
- `src/runtime.zig` holds the verifier-side runtime state, currently the
  Fiat-Shamir transcript and round counter.
- `src/generated/` contains generated verifier stubs.
- `codegen/` contains the Go code generation tool skeleton.
- `test/` contains Zig tests.
- `testdata/generate/` is a Go fixture generator that imports `prover-ray`.
- `testdata/generated/vectors.zig` is generated Zig test data. Do not edit it by
  hand.

## Relationship With prover-ray

`prover-ray` is the source of truth for the protocol implementation. The Zig
verifier should match its public verifier behavior, but it should not copy the
entire prover runtime. Instead, we port only the verifier-visible subset:

- field and extension arithmetic
- Poseidon2 hashing
- Fiat-Shamir transcript updates and squeezes
- round advancement and random coin derivation
- later, FRI verifier query logic

Golden tests enforce this. The Go generator in `testdata/generate/` computes
expected values with `prover-ray`, writes them into `vectors.zig`, and Zig tests
compare `verifier-ray` against those values.

## Runtime And Rounds

The verifier runtime is in `src/runtime.zig`.

`Runtime` contains:

- `transcript`: the Fiat-Shamir transcript.
- `current_round`: the round the verifier expects to process next.
- `total_rounds`: the total number of protocol rounds in the scripted protocol.

`Visibility` contains only the verifier-relevant tags:

- `oracle = 1`
- `public = 2`

These numeric values intentionally match `prover-ray`'s WIOP visibility
encoding. The verifier runtime does not model `internal = 0`; callers should
filter internal columns before constructing a `RoundMessage`.

The main round API is:

```zig
pub fn advanceRoundWithMessage(
    self: *Runtime,
    expected_round: usize,
    message: RoundMessage,
    out_coins: []Coin,
) Error![]const Coin
```

It mirrors the verifier-relevant behavior of `prover-ray/wiop/wiop_runtime.go`
`AdvanceRound()`:

1. Require the caller to advance the runtime's current round.
2. Reject protocols with no rounds, invalid round indexes, or attempts to
   advance the final round.
3. Reject requests for more output coins than the caller-provided backing slice
   can hold.
4. Absorb every column assignment included in `message.columns`, in order.
5. Absorb every public cell included in `message.cells`, in order.
6. Advance `current_round`.
7. Squeeze the requested number of Koalabear E6 extension coins into
   `out_coins` and return the initialized prefix.

`RoundMessage` is the already-filtered verifier-visible message for one round.
Columns in the message are concrete assignments for `oracle` or `public`
columns. The runtime does not receive internal columns and does not validate
missing assignments for columns or cells that are not included in the message;
that filtering and completeness check belongs to the caller or generated
verifier code.

## Test Data Workflow

Generated vectors are intentionally checked into git. When code in
`testdata/generate/` or `prover-ray` behavior changes, regenerate them:

```bash
cd verifier-ray/testdata/generate
go run .
```

Then run:

```bash
cd verifier-ray
make verify-testdata
zig build test
```

`make verify-testdata` regenerates vectors and then runs:

```bash
git diff --exit-code -- testdata/generated/vectors.zig
```

This means it passes only when generated fixtures are already committed or no
fixture changes are expected.

## Common Commands

From `verifier-ray/`:

```bash
make fmt
make build
zig build test
cd codegen && go test ./...
cd testdata/generate && go test ./...
```

The broader check is:

```bash
make test
```

`make test` first verifies generated test data, then runs Zig tests and Go
codegen tests.

## Current Scope

Implemented and golden-tested:

- Koalabear base field
- Koalabear E6 extension field
- polynomial evaluation helpers
- Poseidon2 compression and Merkle-Damgard hashing
- Fiat-Shamir transcript updates and random field/extension squeezes
- per-round verifier random coin derivation for extension-field coins

Still incomplete or placeholder:

- FRI PCS query checks
- full generated verifier logic
- zkVM `zkc` execution smoke test
