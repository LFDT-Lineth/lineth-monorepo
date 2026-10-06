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
testdata/riscv_proof_image.bin
```

This is a distinct file from `testdata/proof_image.bin`, which belongs to
prover-ray's `TestVerifierRayImageIsUpToDate` (a small synthetic `VerifyInput`
at a different base address, for a cross-language ABI-agreement check) — the
two fixtures serve different purposes and must not share a path.

Both are generated from proving `zkc_r5.AllInOneGuestELF` (the one guest in
`codegen`'s `HonestRiscvGuests`, which exercises the full RV64I +
M-extension + custom-precompile surface in a single witness) against the
real `arithmetization/src/main/riscv/main.zkc` interpreter circuit.

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
