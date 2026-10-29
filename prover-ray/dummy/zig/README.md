# Dummy verifier (Zig)

A standalone verifier for the dummy circuit of `prover-ray/dummy` (`../dummy.go`), with the
same signatures as `verifier-ray`, for building and testing guests that verify
ray proofs.

**Test builds only.** A dummy proof is not sound: anyone who knows the public
inputs can produce one.

Everything is in one file, `src/dummy.zig`: the verifier and the circuit, which
is fixed and embedded (`system_0_*`). Its public inputs are those of the
conglomeration root: the guest-output hash (32 cells), then the program
verification key (8 cells, one per limb of the program-round commitment).

## What it checks

A dummy proof has the shape of a ray proof: one round message whose cells are
the witness `Y`, no commitment, no module sizes, an empty PCS opening. It is
accepted iff, limb by limb over KoalaBear, `Y_i = X_i^5` for the public inputs
`X`. `x ↦ x^5` is one-to-one, so a proof is accepted for exactly the public
inputs it was made for. A real ray proof is rejected (it commits to its rounds).

## Same signatures as verifier-ray

The build exports `src/dummy.zig` under the two module names a guest imports
from verifier-ray, `verifier_ray` and `riscv_system`, so a guest's code does not
change (see `test/api_surface.zig`):

- `verifier`: `verify`, `verifyWithWorkspace`, `verifyPair`,
  `verifyPairWithWorkspace`, `Workspace`, `VerifyInput`, `AggregatorInput`,
  `Proof`, `PublicInput`, `Systems`, `checkPublicInputConsistency`,
  `pairStatementEncodedLen`, `encodePairStatement`;
- `image_relocation`: `rebase`, `validateImage`, `rebasePair`, `validatePair`;
- `system_0_spec`, `system_0_systems`, `system_0_public_input`.

Dummy proofs are serialized with prover-ray's `proofserialization`, like any
ray proof. The pair statement uses its own domain tag,
`lineth-verifier-dummy-pair-v1`. The package depends on Zig std only.

## Fixtures

`testdata/` holds proof images written by `prover-ray/dummy` and checked by its
`TestZigFixturesAreUpToDate`. To regenerate:

```bash
cd prover-ray && UPDATE_DUMMY_FIXTURES=1 go test ./dummy/
```

## Build and test

Zig 0.16.0.

```bash
zig build test        # unit tests and Go/Zig image agreement
zig build check-r5    # compile for the R5 guest target
```
