# verifier-ray

`verifier-ray` is the Zig verifier package for Ray proofs. It reimplements the verifier-visible pieces of `prover-ray` with a small runtime.

The Zig library under `src/` is independent from `prover-ray` at runtime. Go tests and fixture generation may import `prover-ray` to produce compatibility data.

## Documentation

- `docs/system-codegen.md` explains how compiled prover-ray systems are extracted and rendered as comptime Zig verifier data.
- `docs/global-constraint.md` explains how the vanishing polynomial/global constraint check is asserted.
- `docs/vanishing-pcs-integration-notes.md` tracks assumptions to revisit when PCS/FRI verification is wired in.

## Testdata Generation

Generated fixtures are checked into git and live in:

```text
testdata/generated/vectors.zig
testdata/generated/vanishing.zig
```

The generator is in:

```text
testdata/generate/
```

Regenerate all generated Zig fixtures from local prover-ray references with:

```bash
make generate-testdata
```

## Zig Tests

Run the Zig test suite with:

```bash
make test-zig
```

Run Go codegen tests with:

```bash
make test-codegen
```

Run both with:

```bash
make test
```

## Building Programs

Build the native debug executable:

```bash
make build
```

Build the native optimized executable:

```bash
make build-release
```

Build the Linea R5 zkVM executable:

```bash
make build-r5
```

The native and R5 executable targets currently run the smoke-test entry point in `src/main.zig`. Binary smoke-test inputs live in:

```text
testdata/inputs/passing.bin
testdata/inputs/failing.bin
```

## Running Example Programs

Run the native executable with `INPUT_FILE`, defaulting to the passing fixture:

```bash
make run
```

Run explicit native fixtures:

```bash
make run-passing
make run-failing
make run-failing-expected
```

`run-failing` is expected to exit non-zero. `run-failing-expected` wraps it and succeeds only when the failure happens.

Run through `zkc` with `INPUT_FILE`, defaulting to the passing fixture:

```bash
make zkc-verify
```

Run explicit `zkc` fixtures:

```bash
make zkc-verify-passing
make zkc-verify-failing
make zkc-verify-failing-expected
```

`zkc-verify-failing` is expected to exit non-zero. `zkc-verify-failing-expected` wraps it and succeeds only when the failure happens.

## Formatting

Run all formatting checks with:

```bash
make fmt
```

Or run them by area:

```bash
make fmt-zig
make fmt-codegen
make fmt-testdata-generate
```
