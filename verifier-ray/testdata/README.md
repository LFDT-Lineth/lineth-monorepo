# verifier-ray testdata

Fixtures in this directory are exported from local `prover-ray` references and consumed by verifier-ray tests. Keep files small and deterministic.

Generated Zig fixtures live in:

```text
testdata/generated/vectors.zig
testdata/generated/vanishing.zig
testdata/generated/verify.zig
```

Legacy smoke-test binary inputs live in:

```text
testdata/inputs/passing.bin
testdata/inputs/failing.bin
```

`make generate-testdata` also writes serialized valid verifier proofs as ignored
runtime inputs:

```text
testdata/inputs/verify_case_<N>.bin
```

Those files are generated from `testdata/generated/verify.zig` metadata and are
used by passing native/R5 runs when `EMBEDDED_INPUT=none`.

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
