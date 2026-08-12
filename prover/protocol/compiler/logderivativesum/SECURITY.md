# Security Analysis of the Log-Derivative Lookup Compiler

This document states and justifies a security theorem for the lookup compiler
implemented in `prover/protocol/compiler/logderivativesum`. The compiler
transforms `query.Inclusion` queries (lookups) into a single
`query.LogDerivativeSum` query (`lookup2logderivsum.go`) and then compiles that
query into committed running-sum columns constrained by local and global
constraints (`logderivativesum.go`, `z_packing.go`). The construction follows
the log-derivative lookup argument of Haböck ("Multivariate lookups based on
logarithmic derivatives", ePrint 2022/1530).

## 1. Setting and notation

- The base field is KoalaBear, `p = 2^31 - 2^24 + 1`. All random verifier
  challenges (coins `ALPHA` and `GAMMA`) are sampled from the degree-4
  extension field `F = F_{p^4}` (`fext`, `coin.FieldExt`), so
  `|F| = p^4 ≈ 2^124`.
- An inclusion query asserts that every row of a *checked* (included) table
  `S` — possibly restricted by a binary filter `f_S` — appears among the rows
  of a *lookup* (including) table `T`, possibly fragmented into columns
  `T = (T^{(1)}, …, T^{(r)})` and possibly filtered by binary filters on the
  including side. Tables may be multi-column of width `w`.
- All column sizes are powers of two (enforced by a panic in
  `CompileLogDerivativeSum`).

## 2. The reduction performed by the compiler

For each lookup table `T` (queries sharing the same table are grouped,
`captureLookupTables`):

1. **Conditional reduction.** A filter on the including side is folded into
   the table itself: the filter column is prepended to `T` and a constant
   `1` column is prepended to `S`. Filters on the included side are kept as
   multiplicands `f_S` of the numerator.
2. **Column collapsing.** For width `w > 1`, a random challenge `α ∈ F` is
   drawn *after* the tables are committed, and each row is collapsed to
   `row(α) = Σ_j α^j · col_j` (`RandLinCombColSymbolic`).
3. **Multiplicity commitment.** The prover commits, per table fragment, to a
   multiplicity column `M` where `M_i` is intended to be the number of times
   row `T_i` is used across all checked tables.
4. **Log-derivative identity.** A second random challenge `γ ∈ F` is drawn
   after `M` is committed, and the compiler emits, as parts of a single
   `LogDerivativeSum` query, the fractions

   ```
   Σ_i  f_S(i) / (γ + S_i(α))     for every checked table S,
   Σ_i  −M_i  / (γ + T_i(α))      for every fragment of T,
   ```

   and the verifier action `CheckLogDerivativeSumMustBeZero` requires the
   total sum to equal `0`.
5. **Running-sum compilation** (`CompileLogDerivativeSum`, `ZCtx.Compile`).
   Parts of equal size are packed in groups of `packingArity = 3` into
   columns `Z`, constrained by:
   - a `Local` constraint fixing `Z[0] · D(0) = N(0)`,
   - a `Global` constraint `(Z[i] − Z[i−1]) · D(i) = N(i)` for all `i`,
   - a `LocalOpening` of `Z[size−1]`,

   where `N`/`D` are the common-denominator numerator and denominator of the
   packed fractions (degree ≤ `packingArity` per row, no field inversions in
   constraints). `FinalEvaluationCheck` verifies that the sum of all final
   openings equals the claimed `LogDerivativeSum` value.

## 3. Security theorem

**Theorem (soundness of the lookup compiler).** Let `Π` be a wizard-IOP
containing a set of inclusion queries over tables of total combined size
`N = Σ (|S_k| + |T|)` (counting all checked tables and all fragments of all
lookup tables, after grouping), maximal width `w`, and let `Π'` be the
protocol output by `CompileLookups`. Assume:

(A1) the columns of `S`, `T`, filters, and `M` are committed (or otherwise
     fixed) before the challenges `α` and `γ` of the corresponding round are
     sampled, and the coins are sampled uniformly from `F = F_{p^4}`
     (guaranteed by the round structure: `M` at round `r`, `α, γ` at `r+1`);

(A2) the `Local`, `Global`, and `LocalOpening` queries emitted by the
     compiler, as well as the binding of committed columns, are enforced
     soundly by the downstream compilers with error `ε_downstream`;

(A3) every filter column (`IncludedFilter` and `IncludingFilter`) is
     constrained to be binary elsewhere in `Π`;

(A4) `char(F) = p > max_i M_i` and `p >` the number of fraction terms of any
     packed group (trivially true: multiplicities are bounded by the number
     of checked rows `< 2^31` only if table sizes stay below `p`; see §4.3).

Then for any (possibly unbounded) prover `P*` that makes the verifier of `Π'`
accept while some inclusion query of `Π` is violated — i.e. some filtered row
of a checked table `S` does not appear among the (filter-selected) rows of
`T` — the acceptance probability is at most

```
ε  ≤  N·(w−1)/|F|      (collision under the α-collapse, per grouped table, union-bounded)
    + N/|F|            (log-derivative identity holding spuriously, or a pole at γ)
    + ε_downstream
```

that is, `ε ≤ N·w/|F| + ε_downstream ≈ N·w / 2^124`. For Linea-scale
parameters (`N ≤ 2^32`, `w ≤ 2^6`) this contribution is at most `≈ 2^-86`.

**Completeness.** If all inclusion queries hold, the honest prover assigns
`M_i = #{(k,j) : f_{S_k}(j)=1, S_{k,j} = T_i}` (choosing, for duplicated
`T`-rows, any split of the count — `MAssignmentTask`), the running sums `Z`
as prescribed, and the verifier accepts with probability 1, provided no
denominator `γ + S_i` or `γ + T_i` vanishes — an event of probability at most
`N/|F|` over the verifier's own coin, which aborts rather than convinces.

## 4. Proof sketch

### 4.1 Reduction to the single-column identity (challenge α)

Fix a grouped table `T` of width `w`. If a filtered row `s` of some checked
table is not a row of (filtered) `T`, then `s ≠ t` as vectors for every row
`t` of `T`. For each pair, `Σ_j α^j (s_j − t_j)` is a nonzero polynomial in
`α` of degree ≤ `w−1`, so by Schwartz–Zippel it vanishes with probability
≤ `(w−1)/|F|`. Union-bounding over at most `N` relevant pairs per row and
over all rows/tables gives the first term. Since `α` is sampled after all
columns are fixed (A1), the adaptive prover gains nothing beyond this bound.
The conditional reduction (step 1 of §2) is lossless given (A3): prepending
the including filter to `T` and a `1`-column to `S` makes a filtered-out
`T`-row (filter `0`) unequal to any active `S`-row (leading coordinate `1`),
so inclusion into the filtered table is exactly inclusion of the extended
rows.

### 4.2 The log-derivative identity (challenge γ)

After collapsing, we have single column vectors `s^{(k)}`, `t` and committed
multiplicities `M`. Consider the rational function in the indeterminate `X`:

```
R(X) = Σ_k Σ_i f_k(i)/(X + s^{(k)}_i)  −  Σ_i M_i/(X + t_i).
```

By Haböck's Lemma (fraction decomposition uniqueness in `F(X)`, valid since
`char(F) = p^... > M_i` by (A4)), `R ≡ 0` if and only if for every value `v`,
the number of filtered occurrences of `v` among the `s^{(k)}` equals
`Σ_{i : t_i = v} M_i`. In particular `R ≡ 0` forces every filtered checked
row to occur in `t` (its total multiplicity on the left is ≥ 1, so it must
receive nonzero mass on the right, and only values of `t` do). If inclusion
is violated, `R` is a nonzero rational function whose numerator (over the
common denominator) has degree < `N`, so `R(γ) = 0` for a uniformly random
`γ` with probability ≤ `N/|F|`. The event that `γ` hits a pole (some
`γ = −s_i` or `−t_i`) is included in the same `N/|F|` counting. Since `γ` is
drawn after `S`, `T`, `M` are committed (A1), the prover cannot steer this.

### 4.3 From the identity to the verified statement

`CheckLogDerivativeSumMustBeZero` accepts iff the claimed sum is `0`, and
`FinalEvaluationCheck` accepts iff that claim equals the sum of the final
openings `Z[size−1]` of all packed columns. The `Local` and `Global`
constraints of `ZCtx.Compile` enforce, for each packed group with numerator
terms `n_1..n_a` and denominators `d_1..d_a` (`a ≤ 3`):

```
Z[0]·Πd(0) = Σ_j n_j(0)·Π_{k≠j} d_k(0),     (Z[i]−Z[i−1])·Πd(i) = Σ_j n_j(i)·Π_{k≠j} d_k(i).
```

Whenever all `d_k(i) = γ + (·) ≠ 0` (which holds except with the pole
probability already counted), these equations have the unique solution
`Z[i] = Σ_{i'≤i} Σ_j n_j(i')/d_j(i')`, i.e. `Z[size−1]` *is* the partial
log-derivative sum of the group; summing the openings yields exactly `R(γ)`.
Note the constraints are multiplicative (no inversion), so a vanishing
denominator would make the row equation unsatisfiable rather than
under-constrained — soundness degrades only through the counted pole event.
These constraint checks themselves are only as sound as the downstream
compilation of `Local`/`Global`/`LocalOpening` queries and the commitment
binding, hence the additive `ε_downstream` (A2).

### 4.4 The multiplicity column needs no independent constraint

`M` is committed by the prover with no direct constraint. This is safe:
soundness in §4.2 quantifies over *arbitrary* `M` fixed before `γ`. A cheating
`M` can only change the right-hand mass distribution over values of `t`; it
can never place mass on a value outside `t`, which is what inclusion
requires. (A4) matters here: multiplicities are field elements, so the
argument counts occurrences mod `p`; as long as the true occurrence counts
are `< p ≈ 2^31` — guaranteed when total checked-row count per table is below
`p` — no wrap-around equivocation exists. Deployments must keep
`Σ_k |S_k| < p` per lookup table.

## 5. Assumptions, caveats, and non-goals

1. **Filters must be proven binary elsewhere (A3).** The compiler uses filter
   columns as numerators and as prepended table columns but does not itself
   constrain them to `{0,1}`. A filter value `c ∉ {0,1}` on the included side
   contributes mass `c` to a row, which (for `c ≠ 0`) still requires the row
   to be in `T` — mildly safe — but a non-binary *including* filter breaks
   the reduction of §4.1. This is an external proof obligation.
2. **Round ordering is load-bearing (A1).** `α` and `γ` live at
   `round+1` where `round` is the max declaration round of the grouped
   queries and of `M`. Any refactor moving coin sampling to the same round as
   a commitment it must dominate voids the theorem.
3. **Segmenter mode is out of scope.** When a `ColumnSegmenter` is supplied
   (distributed prover), `CheckLogDerivativeSumMustBeZero` is deliberately
   *not* registered and the query result is nonzero by design; soundness then
   rests on the distributed-wizard recombination layer checking that the
   segment sums cancel globally. This theorem covers only the monolithic
   (`seg == nil`) path.
4. **Zero-knowledge is not claimed.** `M` and `Z` leak multiplicity
   information; hiding is the responsibility of downstream compilers if
   required.
5. **Grouping is a pure optimization.** Merging queries that share a table
   into one `M`/one fraction family, and packing fractions 3-per-`Z`, changes
   neither the statement nor the bound — the union bound in §3 is already
   over the grouped totals.

## 6. Summary

Under assumptions (A1)–(A4), `CompileLookups` is a sound reduction: any
violated inclusion query survives compilation with probability at most
`≈ N·w/2^124 + ε_downstream`, and honest provers always succeed. The two
random challenges do independent work — `α` reduces tuples to scalars,
`γ` reduces multiset inclusion to a single field equation — and the packed
running-sum columns merely re-express that field equation in a
low-degree-constraint-friendly form without weakening it.
