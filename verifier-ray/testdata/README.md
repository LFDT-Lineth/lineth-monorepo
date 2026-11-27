# verifier-ray testdata

Fixtures in this directory are exported from local `prover-ray` references and consumed by verifier-ray tests. Keep files small and deterministic.

The committed end-to-end fixtures live in:

```text
testdata/generated/verify.zig
```

The proof image used by the native and R5 smoke tests is generated (not
committed) by prover-ray's `prover verifier-assets --system devmode`, alongside
the system it proves:

```text
testdata/generated/devmode_system.zig
testdata/devmode_proof_image.bin
```

`cmd/generate-testdata` also writes the proof image of one `verify.zig` case
(`image_case_index`), which `test/proof_image_test.zig` validates, verifies and
tampers with:

```text
testdata/fixture_proof_image.bin
```

Refresh generated Zig fixtures from `verifier-ray/` with:

```bash
make generate-testdata
```
