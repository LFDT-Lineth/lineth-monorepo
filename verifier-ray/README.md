# verifier-ray

`verifier-ray` is the Zig verifier package for Ray proofs. It reimplements the verifier-visible pieces of `prover-ray` with a small runtime.

The Zig library under `src/` is independent from `prover-ray` at runtime. Go tests and fixture generation may import `prover-ray` to produce compatibility data.

## Documentation

- `docs/global-constraint.md` explains how the vanishing polynomial/global constraint check is asserted.

## Testdata Generation

The end-to-end fixtures are checked into git:

```text
testdata/generated/verify.zig
```

The devmode system, its proof image and the proof image of one `verify.zig`
case are generated, not committed (see `.gitignore`):

```text
testdata/generated/devmode_system.zig
testdata/devmode_proof_image.bin
testdata/fixture_proof_image.bin
```

`verify.zig` and the fixture image are written by `cmd/generate-testdata/`. The
devmode system and its image are written by prover-ray's
`prover verifier-assets --system devmode` command, from the code
generator in `prover-ray/verifierray/codegen`, so a circuit change in prover-ray
needs no change here.

Regenerate all generated Zig fixtures from the prover-ray checkout next to this
directory with:

```bash
make generate-testdata
```

## Zig Tests

Run the Zig test suite with:

```bash
make test-zig
```

Run the Go tests (the proof ABI agreement check against the prover-ray encoder)
with:

```bash
make test-go
```

The codegen tests run with prover-ray's tests.

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

Build the Lineth R5 zkVM executable:

```bash
make build-r5
```

The native and R5 executable targets currently run the smoke-test entry point in `src/main.zig`, which verifies a proof of the devmode system. Binary smoke-test inputs live in:

```text
testdata/devmode_proof_image.bin
```

verifier-ray only verifies the roots of the proving pipeline (the devmode system
now, the recursion root next), never an R5 shard proof directly, so no RISC-V
system is generated.

## Running Example Programs

Run the native executable against the generated honest proof image (`make generate-testdata` produces it; `make run` generates it if missing):

```bash
make run
```

Run the negative embedded-fixture check:

```bash
make run-failing
make run-failing-expected
```

`run-failing` is expected to exit non-zero. `run-failing-expected` wraps it and succeeds only when the failure happens.

Run through `zkc` against the generated honest proof image:

```bash
make zkc-verify
```

Run the negative embedded-fixture check:

```bash
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
make fmt-go
```
