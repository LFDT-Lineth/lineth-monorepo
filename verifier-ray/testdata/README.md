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

`testdata/scratch/` holds real honest proofs for every guest ELF in
`codegen`'s `HonestRiscvGuests` (not just the one committed above), written by
`codegen/generate-riscv-guest-proofs` and read by
`test/riscv_guest_proofs_test.zig`. Not committed — ~52MB per guest would be
~525MB of binary fixtures for no benefit over the one guest already committed
above — so the whole directory is gitignored; `make generate-testdata`
regenerates it every time.

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
