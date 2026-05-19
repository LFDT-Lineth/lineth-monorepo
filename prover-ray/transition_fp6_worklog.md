# transition_fp6_worklog.md — KoalaBear E4 → E6 migration

Worklog for the migration of the `prover-ray` package from the degree-4
KoalaBear extension field (`extensions.E4`) to the degree-6 extension
(`extensions.E6`) shipped in `gnark-crypto@v0.20.2-0.20260514182922-df0578435b08`.

The target was every reference to the degree-4 extension under `prover-ray/`
and all of its sub-packages (`maths`, `crypto`, `wiop`). This is not a
source- or binary-compatible change: constructors, public byte encodings, and
some circuit helper names now expose six extension coordinates.

---

## 1. Tower & layout

Before (E4):

- 𝔽_{p^4} = 𝔽_{p^2}[v] / (v² − u)
- 𝔽_{p^2} = 𝔽_p[u]   / (u² − 3)
- Storage layout: `{B0, B1 E2}` where `E2 = {A0, A1 fr.Element}` → 4 base coords.

After (E6):

- 𝔽_{p^6} = 𝔽_{p^2}[v] / (v³ − (u+1))    *(cubic non-residue: `u+1`)*
- 𝔽_{p^2} = 𝔽_p[u]   / (u² − 3)        *(quadratic non-residue: 3)*
- Storage layout: `{B0, B1, B2 E2}` → 6 base coords; gnark-crypto names the
  type `extensions.E6`.

The `field.ExtensionDegree` constant moved from `4` → `6`. `field.RootPowers`
was repurposed to encode the new tower: `[1, 1, 0, 3]` packs the cubic
relation `v³ = u + 1` (first three entries: u + 1) followed by `u² = 3`.

---

## 2. File-by-file summary

### `maths/koalabear/field/ext.go`

Full rewrite for E6.

- `Ext = extensions.E6`
- `ExtensionDegree = 6`
- All coordinate-reading helpers (`IsBase`, `GetBase`, `MulByBase`,
  `DivByBase`, `SetExtFromBase`, `PseudoRandExt`, `ExtToUint64s`,
  `UintsToExt`, `IntsToExt`) updated to scan/write six coordinates.
- `BatchInvertExt` delegates to `extensions.BatchInvertE6`.
- `BatchInvertExtInto`, `ParBatchInvertExt` unchanged in shape — they
  multiplex over the new `Ext` alias.
- `ExtToBytes` / `BytesToExt` now operate on `ExtensionDegree * Bytes`
  bytes (24) per element.
- `ExtToText` formats the six coordinates.

### `maths/koalabear/field/{vec.go, gen.go}`

Comment-level updates: the cost annotations went from "4 muls / ~9 muls" to
"6 muls / ~24 muls" to reflect E6 arithmetic. No behavioral change.

### `maths/koalabear/field/{ext_test.go, gen_test.go, vec_test.go}`

- Test helpers updated to cover six coordinates (`extEq`, `extUpperIsZero`).
- `UintsToExt(10,20,30,40)` → `UintsToExt(10,20,30,40,50,60)`, etc.
- `TestExtFromBytes`: switched to a coordinate-table loop instead of four
  hardcoded `B0.A0` / `B0.A1` / `B1.A0` / `B1.A1` blocks (now six).

### `maths/koalabear/field/ext_bench_test.go` *(new)*

Micro-benchmarks for diffing E4 vs E6. Covers `Mul`, `Square`, `MulByElement`,
`Inverse`, `BatchInvert`, `ParBatchInvert`, vector add/mul/scale.

### `maths/koalabear/circuit/ext.go`

Complete rewrite of the gnark circuit layer.

- `Ext = { B0, B1, B2 E2 }` where each `E2 = {A0, A1 Element}`.
- New helper `e2MulByCubicNonResidue` for multiplying an E2 by `(u + 1)`.
- `MulExt` reimplemented via Algorithm 13 of <https://eprint.iacr.org/2010/354.pdf>:
  6 E2 multiplications (≈24 base muls) instead of 3 (≈9 base muls).
- `SquareExt` reimplemented via Algorithm 16 of the same paper.
- `MulByNonResidueExt` updated: multiplying by `v` shifts coordinates and
  wraps the last slot through `e2MulByCubicNonResidue`.
- `InverseExt` / `DivExt` hint signatures expanded from 4 to 6 inputs/outputs
  per element. The duplicated hint coordinate marshalling was factored into
  `extFromInputs` / `extToOutputs`.
- `NewExtFrom4FrontendVars` renamed to `NewExtFrom6FrontendVars`.
- `BaseValueOfElement`, `IsConstantZeroExt`, `IsZeroExt`, `SelectExt`,
  `AssertIsEqualExt`, `SumExt` extended to all six coordinates.
- `ConjugateExt` removed (was unused; the natural cubic-extension Frobenius
  is non-trivial and would be misleading to expose without callers).

### `maths/koalabear/polynomials/{canonical_test.go, lagrange_test.go}`

- `randExt` delegates to `field.RandomElementExt()` (which already covers
  six coordinates).
- `extEq` and the local FFT coordinate-extract loop in
  `lagrange_test.go` updated for six coordinates.

### `maths/koalabear/polynomials/lagrange_bench_test.go` *(new)*

Benchmarks for `EvalLagrange` and `ComputeLagrangeAtZ` on extension inputs.

### `crypto/koalabear/fiatshamir/poseidon2.go`

- `UpdateExt` constant `4` replaced by `field.ExtensionDegree`.
- `RandomFext` now consumes 6 of the 8 hashed Poseidon outputs (positions
  4–5 added) instead of 4. The trailing two octuplet slots are discarded.

### `crypto/koalabear/reedsolomon/reedsolomon.go`

- `FFTExt` → `FFTExt6` and `FFTInverseExt` → `FFTInverseExt6` everywhere.
- The vector cast in the fast path is now `extensions.VectorE6` instead of
  `extensions.Vector` (which still aliases `[]E4` in upstream).

### `crypto/koalabear/ringsis/ringsis.go`

- `FFTExt` → `FFTExt6`, `FFTInverseExt` → `FFTInverseExt6`.

### `crypto/koalabear/vortex/utils.go`

- The hint function now expects chunks of `field.ExtensionDegree (= 6)` big
  ints instead of 4. Renamed `ErrSizeNotAMultipleOfFour` →
  `ErrSizeNotAMultipleOfExtDegree` and updated the error message.
- `FFTInverseExt` → `FFTInverseExt6`.

### `crypto/koalabear/vortex/verifier_common.go`

- The verifier now uses the local `polynomials.EvalCanonical` helper, whose
  `field.Vec` / `field.Gen` dispatch supports both base-field and E6
  coefficients.
- The opening-check comparison switched from `!=` (compile-error on E6) to
  `!y.Equal(&other)`.

### `wiop/compilers/global/global.go`

- Replaced the coordinate-by-coordinate `applyBaseFFT4` helper (4 disjoint
  base FFTs) by a direct `largeDomain.FFTInverseExt6(..., DIF, OnCoset())`
  / `domain.FFTExt6(..., DIT)` call. This:
  - eliminates 4 coordinate-slice allocations per proof (the
    `scratchC0..C3` fields and their `AllocField` calls are gone);
  - traverses the E6 array once per FFT pass instead of six times for the
    six coordinates, which is more cache-friendly on HPC nodes.
- `proverBucket` lost the `scratchC0..C3` fields; `Plan` no longer
  allocates them.

### `wiop` query tests and helpers

Files such as `query_lagrange_eval.go`, `query_lagrange_eval_test.go`, and
`query_vanishing_test.go` did not need structural changes.

No structural change needed: the test helpers and production code only
access `B0.A0` (the lifted base slot). Those references work unchanged on
E6 because the layout retains the same first-coordinate.

### `go.mod`

Dependency was already updated to
`github.com/consensys/gnark-crypto v0.20.2-0.20260514182922-df0578435b08`
which exposes `extensions.E6`, `BatchInvertE6`, `VectorE6`, `FFTExt6`,
`FFTInverseExt6`.

---

## 3. Performance impact

Microbenchmarks (Apple M5 Max, `-count=3`, `benchstat`).
Baseline = E4 main, after = E6 migration. Times in ns / µs / ms as appropriate.

```
                          │ baseline (E4) │ after (E6)  │ delta
ExtMul-18                 │     5.34 ns   │  22.0 ns    │ +311 %
ExtSquare-18              │     4.77 ns   │  15.7 ns    │ +229 %
ExtMulByBase-18           │     2.04 ns   │   2.89 ns   │  +42 %
ExtInverse-18             │    57.7  ns   │  83.6 ns    │  +45 %
BatchInvertExt n=65536    │     1.81 ms   │   5.70 ms   │ +215 %
ParBatchInvertExt n=65536 │   436 µs      │ 891 µs      │ +104 %
VecAddExtExt   n=65536    │   109 µs      │ 115 µs      │   +5 %
VecMulExtExt   n=65536    │   393 µs      │ 1820 µs     │ +363 %
VecScaleBaseExt n=65536   │   142 µs      │ 212 µs      │  +49 %
EvalLagrangeExtExt n=16k  │   669 µs      │ 2047 µs     │ +206 %
ComputeLagrangeAtZExt 16k │  4023 µs      │ 5100 µs     │  +27 %
geomean (sec/op)          │     9.18 µs   │  19.9 µs    │ +117 %
```

### Reading these numbers

- **Single `Mul` slowed ≈4×.** The E4 `Mul` in gnark-crypto is a custom
  `uint64`-accumulating kernel using 4 Montgomery reductions; the E6 `Mul`
  is the textbook Karatsuba-over-E2 (6 E2 muls = ≈24 base muls) without an
  inlined accumulator. The reduction-per-coordinate cost is real and
  unavoidable in the chosen tower.
- **`VecMulExtExt` slowed ≈4.6×.** The E4 vector kernel has an AVX-512 path
  (`vectorMul_avx512`); E6 has no AVX path in gnark-crypto yet. On Apple
  silicon that asm is not used anyway (ARM), but on Linux/x86 HPC the gap
  will be even larger until an E6 SIMD kernel ships. Future work for
  upstream gnark-crypto.
- **`VecAddExtExt` is almost a wash** (+5%). Addition is bandwidth-bound;
  growing the working set from 16 bytes/elem to 24 bytes/elem barely
  registers because the loop body is already cheap.
- **`ExtMulByBase`** only sees +42% because it's 6 base muls instead of 4.
- **Inversion path** is dominated by `Inverse(E2)` (one) + several E2 muls
  (six for the cofactor expansion); +45% closely matches the proportional
  arithmetic cost change.
- **Allocation footprint grew ≈50%** as expected (24 B/element vs 16 B
  before).

### Implications for HPC throughput

The hottest paths in the prover are:

1. **Vortex `TransversalHash`** — pure base-field SIS; unaffected.
2. **Quotient computation** in `wiop/compilers/global` — replaces 4 base FFTs
   with 1 E6 FFT (`FFTExt6`). Wall-time should be neutral-to-better thanks
   to cache locality, even though the arithmetic is +50% per coord (six
   coordinates × E6 butterflies, vs four base butterflies).
3. **Reed-Solomon encoding on extension columns** — same `FFTExt6` switch.
4. **Lagrange evaluation** — the new degree dominates: ≈3× slower per call.
   These calls are not in the innermost prover loop, so end-to-end impact
   should remain modest, but worth re-benchmarking on the full prover when
   ready.

---

## 4. Behavioural compatibility

- `field.Ext` is a struct value, so direct pointer comparison `!=` (which
  existed in `verifier_common.go`) had to be changed to `!Equal(&...)`.
  All other call sites only read/write `B0.A0` and similar named accessors,
  which carry over cleanly.
- `ConjugateExt` was removed from the circuit API — the natural Frobenius
  conjugation in a cubic tower is not the same shape as the previous
  "negate B1" definition. No callers were using it.
- Public byte-encoding sizes changed:
  - `len(ExtToBytes(z))` = 24 bytes (was 16).
  - `BytesToExt` consumes 24 bytes.
  This means any persisted artifact serialized with the previous E4 layout
  is **not** loadable as E6 — re-encode any saved transcripts or proofs.

---

## 5. Cleanup / simplify pass

Ran code-reuse, code-quality and efficiency audits on the diff. Findings and
resolutions:

- **Test-only helper exported**: `ExtToUint64sTuple` in `ext_test.go` was
  capitalized despite being test-internal. Renamed to `extToUint64sTuple`.
- **Allocation thrashing in `SumExt`**: the closure-based getter was
  allocating six `[]Element` slices per call. Hoisted the scratch slice to
  a single allocation reused across the six coordinate reductions.
- **Reuse suggestion: `EvalCanonical` for Horner kernels in vortex**:
  after the merge from `main`, `EvalCanonical` is `field.Gen`-aware, so the
  verifier uses it directly instead of local Horner kernels.
- **`NewHintExt` native vs emulated DRY-up**: kept the two branches
  separate. The element constructors and input/output element types
  differ; unifying via reflection or type parameters would obscure rather
  than help.

After the simplify pass: `go vet ./...` clean, `go build ./...` clean,
`go test ./...` passes.

---

## 6. Verification matrix

```
go build ./...                       # OK
go vet   ./...                       # OK
go test  ./...  -count=1 -timeout=600s
  maths/koalabear/{circuit,field,polynomials}                OK
  crypto/koalabear/{poseidon2,reedsolomon,ringsis,smt,vortex} OK
  wiop, wiop/compilers/{global,logderivativesum,lookuptologderivsum,rangecheck},
  wiop/codegen, zkcdriver                                    OK
go test -bench=Benchmark...  -count=3 -run=^$                OK
```

---

## 7. Open follow-ups (not in scope)

- Upstream gnark-crypto SIMD kernels for E6 vector operations (`Mul`,
  `ScalarMul`, `MulByElement`, `Butterfly`). Today, E4 has AVX-512 fast
  paths and E6 does not; the gap is most visible on x86 Linux HPC. When
  these land, regen baselines and revisit.
- Profile end-to-end prover throughput on a representative trace to confirm
  the FFT cache-locality win (moving from 4 base FFTs to 1 E6 FFT) holds
  outside the microbench harness.
- Decide whether `field.RootPowers` is still useful; nothing in the repo
  consumes it as of this migration.
