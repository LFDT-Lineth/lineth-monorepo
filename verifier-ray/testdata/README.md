# verifier-ray testdata

Fixtures in this directory are exported from local `prover-ray` references and consumed by verifier-ray tests. Keep files small and deterministic.

Generated Zig fixtures live in:

```text
testdata/generated/vectors.zig
testdata/generated/vanishing.zig
testdata/generated/verify.zig
testdata/generated/riscv_system.zig
```

The real proof image used by the native and R5 smoke tests lives in:

```text
testdata/proof_image.bin
```

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
