# verifier-ray

`verifier-ray` is the initial Zig verifier package for Ray proofs.

The package is intentionally independent from `prover-ray` at the directory
level. The fixed verifier runtime, field arithmetic, cryptographic primitives,
and zkVM precompile interface live here. Code generation can also live here and
import `prover-ray` structures when needed.

## Layout

- `src/` contains the Zig verifier library and executable entry point.
- `src/generated/` contains generated verifier stubs.
- `codegen/` contains the Go code generation tool skeleton.
- `test/` contains Zig unit tests.
- `testdata/` holds fixtures exported from `prover-ray`.

## Local Checks

```bash
make fmt
make verify-testdata
make build
make build-release
make test
make zkc-verify
```

The current implementation covers Milestone 1 static field, extension,
polynomial, Poseidon2, and Fiat-Shamir primitives with prover-ray golden tests.
Vortex verification and zkVM `zkc` execution are still placeholders.
