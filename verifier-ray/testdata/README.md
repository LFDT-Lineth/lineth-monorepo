# verifier-ray testdata

Fixtures in this directory are exported from local `prover-ray` references and consumed by verifier-ray tests. Keep files small and deterministic.

Generated Zig fixtures live in:

```text
testdata/generated/vectors.zig
testdata/generated/vanishing.zig
```

Native and R5 smoke-test binary inputs live in:

```text
testdata/inputs/passing.bin
testdata/inputs/failing.bin
```

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
