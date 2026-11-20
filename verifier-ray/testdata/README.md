# verifier-ray testdata

Fixtures in this directory are exported from local `prover-ray` references and consumed by verifier-ray tests. Keep files small and deterministic.

The committed end-to-end fixtures live in:

```text
testdata/generated/verify.zig
```

The real proof image used by the native and R5 smoke tests is generated (not
committed) by `codegen/generate-riscv-system`, alongside `riscv_system.zig`:

```text
testdata/generated/riscv_system.zig
testdata/riscv_proof_image.bin
```

Both are generated from proving `zkc_r5.AllInOneGuestELF` (the one guest in
`codegen`'s `HonestRiscvGuests`, which exercises the full RV64I +
M-extension + custom-precompile surface in a single witness) against the
real `arithmetization/src/main/riscv/main.zkc` interpreter circuit.

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
