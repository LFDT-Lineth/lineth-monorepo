# Multi-Degree FRI In prover-ray

This note explains how multi-degree FRI is implemented in `prover-ray`, with
the polynomial-commitment wrapper in `crypto/koalabear/fri/pcs.go` and the WIOP
compiler pass in `wiop/compilers/pcs/pcs.go`.

A frequently asked questions and their answers and an end to end example
is given in a separate document in the current folder. 

The short version:

- WIOP columns can have different native sizes.
- The PCS commits those columns in multi-size Merkle trees.
- For every distinct native size, the PCS constructs a virtual DEEP quotient
  codeword.
- The largest virtual quotient starts FRI at layer 0.
- Smaller virtual quotients are injected into the FRI folding chain exactly
  when the running layer has folded down to their size.
- The verifier reconstructs only the queried virtual quotient values from
  authenticated row openings; there is no separate committed DEEP-quotient tree.

## Code Map

The main pieces are:

| File | Role |
| ---- | ---- |
| `crypto/koalabear/fri/fri.go` | FRI parameters, `Level`, `Proof`, verifier, fold checks. |
| `crypto/koalabear/fri/fri_prover_state.go` | Prover-side folding state machine. |
| `crypto/koalabear/fri/pcs.go` | PCS wrapper that builds virtual DEEP quotient levels. |
| `crypto/koalabear/fri/commitment.go` | Multi-size Reed-Solomon encoding and Merkle commitment. |
| `crypto/koalabear/fri/tree.go` | Multi-size Merkle tree and branch helpers. |
| `wiop/compilers/pcs/pcs.go` | WIOP compiler pass that commits columns and calls the FRI PCS. |

## Parameters And Domains

FRI parameters are:

$$
N = \text{codeword size}, \qquad D = \text{largest plaintext size},
\qquad \rho = \frac{N}{D}.
$$

Both `N` and `D` are powers of two, and `D < N`. The number of folding rounds is

$$
r = \log_2 D.
$$

At FRI round `j`, the running codeword has:

$$
N_j = \frac{N}{2^j}, \qquad D_j = \frac{D}{2^j}.
$$

`fri.NewParams` precomputes domains of sizes `N, N/2, ..., N/D`. The code stores
full FFT domains for proving and lightweight domain descriptors for verification.

The Reed-Solomon encoder returns codewords in bit-reversed order. That choice is
important: evaluations that FRI folds together sit at adjacent positions
`2t` and `2t+1`.

## What "Multi-Degree" Means Here

Standard FRI starts with one degree-`D` object and halves the degree bound until
only a small final polynomial remains. Multi-degree FRI lets the prover include
objects with smaller degree bounds without padding them to the largest size.

A level is represented by:

```go
type Level struct {
    D     int
    Evals []field.Ext
    Trees []*Tree
}
```

For a level with degree bound `D_l`, the introduction round is:

$$
j_l = \log_2\left(\frac{D}{D_l}\right).
$$

At that point the running FRI layer has already shrunk to the same length:

$$
|R_{j_l}| = \frac{N}{2^{j_l}} = N \cdot \frac{D_l}{D}.
$$

So the level can be added pointwise to the fold output, with no padding.

`buildProvePlan` enforces the key invariants:

- `levels[0].D == p.D`;
- each extra `D_l` is a positive power of two;
- each `D_l` divides `D` by a power-of-two ratio;
- each extra level has exactly `N >> j_l` evaluations;
- at most one level enters at a given round.

The PCS satisfies the "one level per round" rule by grouping all columns of the
same native size into one virtual DEEP quotient level.

## The Fold Formula

Let `R_j` be the current running codeword in bit-reversed order. A FRI query pair
at output index `t` uses input positions:

$$
R_j[2t], \qquad R_j[2t+1].
$$

Let `x_t` be the domain point for that pair. In code, `x_t` is obtained from the
round domain using the bit-reversed position. With folding challenge
`\alpha_j`, the ordinary fold is:

$$
\operatorname{fold}_{\alpha_j}(R_j)[t]
=
\frac{R_j[2t] + R_j[2t+1]}{2}
+
\alpha_j \cdot
\frac{R_j[2t] - R_j[2t+1]}{2x_t}.
$$

Multi-degree FRI adds one optional auxiliary term. If a smaller level `F_{j+1}`
enters when producing `R_{j+1}`, prover-ray computes:

$$
R_{j+1}[t]
=
\operatorname{fold}_{\alpha_j}(R_j)[t]
+
\alpha_j^2 F_{j+1}[t].
$$

This timing is the implementation detail to remember. The lower-size level does
not get batched before the fold, and there is no separate batching challenge
`\gamma`. The same fold challenge contributes the `\alpha_j^2` coefficient.

In `ProverState.Fold`, round `j` checks whether a level is scheduled at
`j+1`, passes it as `aux`, and `foldLayerInternally` adds the
`\alpha_j^2 * aux[t]` term.

After the final round, the running codeword has length:

$$
N_r = \frac{N}{D} = \rho.
$$

That final codeword is revealed as `FinalPolyExt`; it is not Merkle-committed.

## Multi-Size Commitments

The PCS commits columns by batch. In WIOP, a batch is one committed round, plus
the precomputed round if it owns columns. A batch is stored as a `MultiSizeTable`:

```go
type MultiSizeTable []SizedTable

type SizedTable struct {
    Base [][]field.Element
    Ext  [][]field.Ext
}
```

Index `i` corresponds to plaintext size `2^i`. Inside a size bucket, base rows
come before extension rows, and rows keep column declaration order.

`Commit` does two things:

1. Reed-Solomon encodes every row with the encoder for its native size.
2. Merkleizes all encoded size buckets into one multi-size tree.

This is not a paired-leaf tree. Each encoded row position is hashed into a row
digest. Smaller size buckets become auxiliary leaves at the matching internal
tree levels. A branch can therefore authenticate:

- a largest-size row as the branch leaf; or
- a smaller-size row as an auxiliary leaf along the same branch.

The FRI conjugate pair is recovered from a branch leaf and its deepest sibling,
not from a single pre-paired leaf.

## Canonical Layout

Before building virtual DEEP quotients, the PCS canonicalizes every opened row.
For each native size in descending order:

```text
for batch in batch declaration order:
  for the SizedTable at this size:
    emit base rows in declaration order
    emit extension rows in declaration order
```

For each size, the `alpha_DEEP` power counter resets to zero. All shifts of one
row share the same `alpha_DEEP` power.

For one emitted row `i`, with shifts `s` and claimed evaluations `y_{i,s}`, the
claim point is:

$$
z_{i,s} = \zeta \cdot \omega_d^s,
$$

where `d` is the row's plaintext size and `\omega_d` is the generator of that
plaintext-size domain. Shifts must be non-empty, unique within a row, and in
`[0, d)`.

## Virtual DEEP Quotient Levels

For each distinct size, `pcs.reconstructLevels` builds a virtual codeword:

$$
F_d(x)
=
\sum_i \alpha_{\mathrm{DEEP}}^i
  \sum_{s \in S_i}
  \frac{f_i(x) - y_{i,s}}{x - z_{i,s}}.
$$

Here:

- `d` is the plaintext size for this level;
- `x` ranges over the Reed-Solomon codeword domain for size `d`;
- `f_i(x)` is the encoded row value at `x`;
- `y_{i,s}` is the claimed evaluation at `z_{i,s}`;
- `S_i` is the shift list for row `i`;
- base-field rows are lifted into the extension field.

The code computes this at every codeword position for the prover. It batches all
distinct denominators with one batch inversion per level:

$$
\frac{1}{x - z_{i,s}}.
$$

If a claim point lands on the codeword domain, reconstruction fails. The verifier
also checks this by testing `zeta` against every distinct size domain; multiplying
by a domain rotation does not change domain membership.

The virtual levels are sorted from largest size to smallest size and then passed
to `fri.NewProverState`.

## Direct FRI Versus PCS FRI

The low-level FRI package can be used directly: in that mode, each `Level.Trees`
normally commits to `Level.Evals`, and the verifier decodes opened FRI values
directly from Merkle branches.

The PCS integration uses the same FRI verifier with explicit input values:

- `Level.Evals` are the prover-side virtual DEEP quotient codewords.
- `Level.Trees` are the original batch commitment trees, not separate quotient
  trees.
- `OpeningProof.RowOpenings` carries the original row values opened at each FRI
  query position.
- `pcs.Verify` recomputes the queried virtual quotient values from those row
  openings and the claimed values at `zeta`.
- The FRI proof authenticates the row digests against the original roots, then
  checks the FRI folding equations using the recomputed virtual quotient values.

This is why the PCS does not need to commit a separate DEEP quotient tree. The
quotient is virtual but query-checkable from authenticated original rows.

## Prover Flow In WIOP

`wiop/compilers/pcs.Compile` runs after the arithmetization passes have reduced
constraints to `LagrangeEval` claims. It wires three pieces:

1. A commit action for each committed interactive round.
2. A compile-time commitment for the precomputed round, if present.
3. A final opening action and verifier action.

Each commit action:

- pads columns to the next power of two;
- sorts them into a `fri.MultiSizeTable`;
- commits with inverse rate `2`;
- records the Merkle root in `Runtime.Commitments`;
- hides the raw columns from the proof by setting them internal.

The final opening action:

1. Recovers every `(batch, column, shift)` opening required by the WIOP
   `LagrangeEval` queries.
2. Registers each committed batch with `pcs.AddOpening`.
3. Squeezes `alpha_DEEP`.
4. Calls `pcs.NewProverState(alpha_DEEP)`, which reconstructs virtual levels.
5. Squeezes one FRI fold challenge per round.
6. Absorbs each intermediate FRI root.
7. Absorbs `FinalPolyExt`.
8. Derives query positions.
9. Adds row openings and the FRI proof to `PCSOpeningProof`.

The final fold returns a zero root and is not absorbed; the final polynomial is
absorbed instead.

The verifier repeats the same transcript schedule, reconstructs the same shifts,
claims, shapes, roots, `alpha_DEEP`, fold challenges, and query positions, and
then calls `pcs.Verify`.

## Verification Checks

For each query position `s`, the verifier checks:

1. Row openings match the declared shapes.
2. Merkle branches authenticate against the original batch roots.
3. The authenticated row digests equal the row preimages in `RowOpenings`.
4. The queried virtual quotient values are recomputed from:

   $$
   \frac{f_i(x) - y_{i,s}}{x - z_{i,s}}.
   $$

5. The FRI folding chain is valid.

For the folding chain, the queried position at round `j` is:

$$
\text{base}_j = s >> j.
$$

The branch gives the opened value at `base_j` and the deepest sibling at
`base_j ^ 1`. The verifier computes:

$$
\frac{\text{self} + \text{sibling}}{2}
+
\alpha_j \cdot
\frac{\text{self} - \text{sibling}}{2x}
$$

where `x` is the domain point for the opened leaf. If `base_j` is odd, `x`
already includes the sign change, so the same formula works.

If a level enters at `j+1`, the verifier adds:

$$
\alpha_j^2 \cdot F_{j+1}[\text{base}_{j+1}].
$$

The result must equal the opened leaf in round `j+1`, or the matching entry in
`FinalPolyExt` on the last round.

## Small Example

Take inverse rate `2`, largest plaintext size `D = 8`, and top codeword size
`N = 16`. Then `r = log2(8) = 3`, and the final polynomial has length:

$$
\frac{N}{D} = 2.
$$

Suppose there are two native sizes:

- size `8`: one row `A`, opened at shift `0`;
- size `4`: one row `B`, opened at shift `1`.

Let:

$$
a = A(\zeta), \qquad
b = B(\zeta \omega_4).
$$

The PCS constructs two virtual quotient levels:

$$
F_8(x) = \frac{A(x) - a}{x - \zeta},
$$

and

$$
F_4(u) = \frac{B(u) - b}{u - \zeta\omega_4}.
$$

`F_8` has `16` codeword evaluations and starts FRI as:

$$
R_0 = F_8.
$$

`F_4` has `8` codeword evaluations. Its introduction round is:

$$
j_4 = \log_2(8/4) = 1.
$$

Because `ProverState.Fold` checks for a level scheduled at `j+1`, `F_4` is
mixed in while producing `R_1` from `R_0`.

For output index `t`, the first fold is:

$$
R_1[t]
=
\frac{R_0[2t] + R_0[2t+1]}{2}
+
\alpha_0
\frac{R_0[2t] - R_0[2t+1]}{2x_t}
+
\alpha_0^2 F_4[t].
$$

The next folds have no extra level in this example:

$$
R_2[t]
=
\frac{R_1[2t] + R_1[2t+1]}{2}
+
\alpha_1
\frac{R_1[2t] - R_1[2t+1]}{2x_t},
$$

and

$$
R_3[t]
=
\frac{R_2[2t] + R_2[2t+1]}{2}
+
\alpha_2
\frac{R_2[2t] - R_2[2t+1]}{2x_t}.
$$

`R_3` has length `2` and is revealed as `FinalPolyExt`.

Now choose query position `s = 10`. The verifier follows the same path:

| Round | Opened index | Sibling index | Next index |
| ----- | ------------ | ------------- | ---------- |
| `0` | `10` | `11` | `5` |
| `1` | `5` | `4` | `2` |
| `2` | `2` | `3` | `1` |

At round `0`, the verifier recomputes:

$$
\operatorname{fold}_{\alpha_0}(R_0[10], R_0[11])
+
\alpha_0^2 F_4[5],
$$

and checks that it equals the opened `R_1[5]`.

At round `1`, it checks the fold from `R_1[5]` and `R_1[4]` to `R_2[2]`.
At round `2`, it checks the fold to `FinalPolyExt[1]`.

The important part is that `F_4[5]` is not an unauthenticated value. In PCS mode,
the verifier reconstructs it from the authenticated row opening for `B` at query
index `5` in the size-4 codeword:

$$
F_4[5]
=
\frac{B(u_5) - b}{u_5 - \zeta\omega_4}.
$$

The row opening for `B` is tied back to the original batch Merkle root, so the
virtual quotient value is tied back to the committed column.

## Why This Saves Work

Without multi-degree FRI, `F_4` would have to be padded or evaluated on the
size-16 top domain just to participate in the same FRI proof as `F_8`.

With multi-degree FRI, `F_4` is evaluated only on its natural size-8 codeword
domain. It enters after one fold, exactly when the running codeword also has
length `8`.

For many module sizes, this avoids evaluating, hashing, opening, and folding
smaller objects on domains they do not naturally need.

## Soundness Intuition

The PCS wants to bind claimed evaluations at `zeta` to committed low-degree
rows. For one row, the DEEP quotient term is:

$$
\frac{f(x) - y}{x - z}.
$$

If `y` is not the value of the committed low-degree polynomial at `z`, this
quotient is unlikely to behave like a low-degree codeword. `alpha_DEEP` batches
many rows and shifts so the prover cannot arrange cancellations after seeing the
challenge.

FRI then checks that the batched virtual quotient levels are jointly close to
low-degree polynomials. The multi-degree fold equation lets those levels have
different natural sizes while still ending in one final polynomial and one query
schedule.

## Practical Invariants And Gotchas

- The WIOP PCS uses static FRI parameters up to the maximum supported column
  size, but each proof restricts the schedule to the largest size actually
  opened. A size-`2^k` proof folds `k` times, not all the way from the static
  maximum.
- Query positions are sampled in `[0, effectiveN)`, where `effectiveN` is the
  codeword size of the largest committed column in that proof.
- The current WIOP PCS inverse rate is `2`, and the query count is `229`.
- All columns opened in one PCS proof share the same `zeta`.
- Every opened row must have at least one shift (zero shifts count).
- Duplicate shifts for one row are rejected.
- Duplicate WIOP claims for the same `(batch, size, row, shift)` are deduplicated
  only if the claimed value is identical.
- Base rows and extension rows can share a size bucket; base rows are lifted
  into the extension field for quotient reconstruction.
- `alpha_DEEP` powers reset for each native size. This is intentional and part
  of the canonical layout.
- `ProverState.NewProverState` sorts levels in place by decreasing `D`.
- In direct low-level FRI, the level trees normally commit to `Level.Evals`.
  In PCS mode, the trees are original batch trees and queried virtual values are
  supplied through explicit verifier inputs.

## Mental Model

Think of the PCS as building this object:

```text
original committed rows
        |
        | zeta claims + alpha_DEEP
        v
virtual DEEP quotient per native size
        |
        | multi-degree FRI
        v
one final polynomial
```

The virtual quotient codewords are not separately committed. Instead, every FRI
query asks the prover to open the original rows needed to recompute the quotient
value at that query. Those row openings are authenticated against the original
commitment roots, and the recomputed values are plugged into the FRI fold checks.
