# FRI-Frequently Asked Questions

This FAQ is for developers and cryptographers reading
`prover-ray/crypto/koalabear/fri` and `prover-ray/wiop/compilers/pcs`. 
It answers the recurring questions that come up when connecting the
WIOP PCS compiler, Reed-Solomon encoding, multi-size Merkle trees, DEEP quotient
values, and the FRI folding proof. We also give an end to end example and the 
verifier checks at the bottom.

The descriptions below match the current code shape in this branch, where
`fri.OpeningProof` contains `RowOpenings` plus an inner `FRIProof`.

## Package Map

### What are the main files in `crypto/koalabear/fri`?

- `reedsolomon.go`: RS encoders. They take plaintext evaluations on a small FFT
  domain, interpolate, then evaluate on a larger FFT domain. The returned
  codeword is in bit-reversed order.
- `multi_size_table.go`: data model for grouped polynomial vectors of different
  powers-of-two sizes.
- `commitment.go`: encodes a `MultiSizeTable` and builds a multi-size Merkle
  tree over its encoded rows.
- `tree.go`: Merkle tree and Merkle branch implementation. It supports ordinary
  binary nodes plus optional auxiliary leaves for smaller aligned codewords.
- `fri.go`: FRI parameters, proof structure, verifier, folding arithmetic, and
  low-level query checks.
- `fri_prover_state.go`: state machine for the prover side of folding: build
  layer 0, fold round by round, commit intermediate layers, and open query
  paths.
- `pcs.go`: PCS wrapper around the FRI machinery. It turns committed WIOP
  columns and claimed evaluations into a virtual DEEP quotient and verifies it
  with FRI.

## Domains, Generators, And Reed-Solomon Encoding

### What are the "small" and "large" domains?

The small domain is the plaintext domain of a polynomial. If a committed column
has logical size `D`, its values are treated as evaluations on a size-`D` FFT
domain.

The large domain is the Reed-Solomon codeword domain of size `N`. In the WIOP
PCS compiler:

```text
inverse rate = N / D = 2
```

So for WIOP PCS today:

```text
|large domain| = 2 * |small domain|
```

The FRI package itself is more general: every `RSEncoder` has
`Domain.Cardinality / PlainTextSize` as its inverse rate.

### What is the KoalaBear domain generator formula?

The field package gets a root of unity of maximum order `2^24`:

```text
root = 1791270792
```

For a requested domain size `m`, gnark-crypto first rounds it to:

```text
x = NextPowerOfTwo(m)
logx = log2(x)
```

Then it returns:

```math
g_x = root^{2^{24 - logx}}
```

This gives an element of order `x`. If `logx > 24`, the field does not have the
required power-of-two root of unity.

### How does bit reversal work if codewords are polynomial evaluations on an FFT domain?

Conceptually, the RS codeword is:

```math
R[i] = f(g^i)
```

in natural domain order. But the encoder stores it in bit-reversed order:

```math
encoded[pos] = f(g^{bitReverse(pos)})
```

Equivalently, the natural-order evaluation at index `i` is stored at:

```math
pos = bitReverse(i)
```

This is not changing the domain or the polynomial. It is only changing the array
layout.

The reason is FRI folding. FRI needs the conjugate pair:

```math
f(x), f(-x)
```

to sit next to each other. In a power-of-two multiplicative subgroup,
if the large domain has size `N`, then:

```math
g^{i + N/2} = -g^i
```

Bit-reversed storage makes those conjugates adjacent.

### Small example: why are bit-reversed conjugates adjacent as `(2t, 2t+1)`?

Let `N = 8`, so indices use 3 bits. The stored position `pos` maps to natural
exponent `bitReverse_3(pos)`:

| Stored `pos` | Bits | Natural exponent | Domain point |
|---:|:---:|---:|:---|
| 0 | `000` | 0 | `g^0` |
| 1 | `001` | 4 | `g^4 = -g^0` |
| 2 | `010` | 2 | `g^2` |
| 3 | `011` | 6 | `g^6 = -g^2` |
| 4 | `100` | 1 | `g^1` |
| 5 | `101` | 5 | `g^5 = -g^1` |
| 6 | `110` | 3 | `g^3` |
| 7 | `111` | 7 | `g^7 = -g^3` |

So the adjacent stored pairs are:

```text
(0, 1), (2, 3), (4, 5), (6, 7)
```

and every pair is `(x, -x)`.

Algebraically:

```math
bitReverse_N(2t + 1) = bitReverse_N(2t) + N/2
```

so positions `2t` and `2t+1` hold evaluations at points differing by a factor
of `g^{N/2} = -1`.

## MultiSizeTable And Merkle Commitment

### What is `MultiSizeTable`?

`MultiSizeTable` is a list of size buckets:

```go
type MultiSizeTable []SizedTable
```

Bucket `table[i]` contains vectors of size `2^i` before encoding, or
`rate * 2^i` after encoding. A `SizedTable` has two lists:

```go
Base [][]field.Element
Ext  [][]field.Ext
```

Each inner slice is one polynomial/codeword. All vectors inside one
`SizedTable` have the same length.

In WIOP language these are committed columns. In the FRI package, comments often
call them rows because `SizedTable.Base[k]` is a row of the table data
structure. Do not confuse that with `RowOpening`, which is a Merkle leaf
preimage at one evaluation row.

### What does `Merkleize()` commit to?

`Merkleize()` hashes the encoded table line by line.

For a fixed size bucket and evaluation index `j`, it hashes:

```text
all base column values at j
all extension column coordinates at j
```

That digest is a Merkle leaf for that size bucket.

Then `NewTree` builds a multi-size tree:

- the largest size bucket is the bottom binary layer;
- smaller size buckets become optional auxiliary leaves at internal levels;
- a node hash is either `H(left, right)` or `H(H(left, right), aux)`.

This lets one Merkle branch authenticate an aligned path through several
different codeword sizes.

### Is a branch always expected to have auxiliary leaves?

No. `Branch.AuxSiblings` has one entry per tree level, but each entry is a
pointer and may be `nil`.

Meaning:

```text
AuxSiblings[i] == nil      -> no smaller committed row at that tree level
AuxSiblings[i] != nil      -> an auxiliary row digest is part of that node hash
```

Running FRI layer trees are plain binary trees, so their aux entries are nil.
Input commitment trees may have non-nil aux entries when the same commitment
contains multiple sizes.

### How does `OpenBranch` work?

`OpenBranch(idx)` starts at a bottom leaf and walks upward to the root. It
returns:

- `Leaf`: the opened bottom leaf;
- `Siblings`: the sibling digest at each tree level;
- `AuxSiblings`: optional auxiliary digest at each tree level.

`RecoverRoot(idx)` reverses the walk:

```text
ancestor = Leaf
for level from bottom to top:
    if current index bit is 0:
        left = ancestor, right = sibling
    else:
        left = sibling, right = ancestor
    ancestor = hashNode(left, right, aux)
    index >>= 1
```

At the end, `ancestor` must equal the advertised Merkle root.

## Shifts, Claims, And The Virtual DEEP Quotient

### What is `S_i`, the shift list for row/column `i`?

For each committed polynomial, the PCS must know which rotations are opened at
the out-of-domain point. A shift list says:

```text
open this column at zeta * omega^shift
```

If column `A` of size 8 is used at the current row and the next row, its shifts
may be:

```text
S_A = [0, 1]
```

If column `B` is used at the previous row, shift `-1` is normalized modulo the
size:

```text
S_B = [7]
```

For each shift, the outer WIOP protocol supplies a claimed value:

```math
y_{i,m} = f_i(\zeta \cdot \omega^{S_i[m]})
```

The PCS checks that the shape of claimed values matches the shape of the shift
lists.

### Does zero shift count as a shift?

Yes. Shift `0` means "open the column at the unrotated point `zeta`". It counts
as one shift.

So:

```text
S_i = [0]
```

is valid and means the row/column has one opening.

An empty shift list:

```text
S_i = []
```

is invalid for an opened row/column.

### What is the virtual DEEP quotient?

The PCS batches all opened committed polynomials into a virtual polynomial:

```math
F(X) =
\sum_i \alpha_{\mathrm{DEEP}}^i
\sum_m \frac{f_i(X) - y_{i,m}}{X - z_{i,m}}
```

where:

```math
z_{i,m} = \zeta \cdot \omega^{S_i[m]}
```

The committed polynomials are the original WIOP columns. The virtual polynomial
`F` is not a new WIOP column and is not committed as a separate input Merkle
tree. Instead:

- the prover can compute the full virtual codeword from the committed columns;
- the verifier reconstructs only the queried values of `F` from opened rows and
  claimed evaluations.

### In the examples, is only `F_8` RS-encoded and committed, or are `A` and `B` also committed?

In the PCS flow, the original columns such as `A` and `B` are RS-encoded and
Merkle-committed.

The virtual DEEP quotient `F_8` is derived from those committed codewords and
the claimed values. It is the first FRI running codeword, but it is virtual from
the verifier's perspective. The verifier authenticates the original column rows
and recomputes the necessary `F_8(x)` values.

So:

```text
A, B, ...      -> RS encoded and Merkle committed
F_8            -> virtual quotient codeword used as FRI layer 0
T_1, T_2, ...  -> folded running FRI layers, Merkle committed by the FRI prover
final poly     -> revealed directly
```

### Why does the verifier compute `F_8(x)`?

FRI checks a folding relation. The first relation needs the layer-0 values:

```text
F_8(x) and F_8(-x)
```

The verifier does not know the whole `F_8` codeword. It reconstructs only the
queried values from:

- opened committed rows for the original columns;
- claimed values `y_{i,m}`;
- claim points `z_{i,m}`;
- the batching challenge `alpha_DEEP`.

That reconstructed value is then compared through the FRI fold relation to the
next committed FRI layer.

### Is `x` outside the FFT domain?

No. In the FRI query check, `x` is a sampled point in the large FFT codeword
domain.

The claim points:

```math
z_{i,m} = \zeta \cdot \omega^{S_i[m]}
```

must be outside the codeword domain. That is what keeps denominators
`x - z_{i,m}` nonzero for every domain query point `x`.

### What is the relationship between `R_j[2t]`, `R_j[2t+1]`, `f_i`, `zeta`, and `y_i`?

`f_i`, `zeta`, shifts, and claimed values define the virtual DEEP quotient.
For layer 0:

```math
R_0(x) = F(x)
```

where:

```math
F(X) =
\sum_i \alpha_{\mathrm{DEEP}}^i
\sum_m \frac{f_i(X) - y_{i,m}}{X - z_{i,m}}
```

The pair:

```text
R_0[2t], R_0[2t+1]
```

is the pair:

```text
F(x), F(-x)
```

in bit-reversed storage.

For later layers, `R_1`, `R_2`, ... are no longer directly described by the
original `f_i` formula. They are the folded running FRI codewords:

```math
R_{j+1}(x^2)
=
\frac{R_j(x) + R_j(-x)}{2}
+
\alpha_j \frac{R_j(x) - R_j(-x)}{2x}
+
\alpha_j^2 \cdot Aux_{j+1}(x^2)
```

The auxiliary term is present only when a lower-degree level is introduced at
that round in multi-degree FRI.

## Folding And Query Paths

### What folding check involves `F_8(x)`?

For the first FRI round, the verifier reconstructs:

```text
self    = F_8(x)
sibling = F_8(-x)
```

Then it computes:

```math
expected =
\frac{self + sibling}{2}
+
\alpha_0 \cdot \frac{self - sibling}{2x}
+
\alpha_0^2 \cdot aux
```

If this is not the last round, `expected` must equal the opened value in the
next FRI layer (in the FRI proof). If this is the last round, it must equal the selected entry of
`FinalPolyExt`.

### If the verifier computes `R_1[t]`, how does it get `R_1[2t]` and `R_1[2t+1]` for the next folding level?

It does not derive both next-level values from one value.

For a sampled query path:

1. The verifier uses the previous layer pair to compute one value in the next
   layer, for example `R_1[t]` (from `R_0[2t]` and `R_0[2t+1]`).
2. The proof also opens the Merkle branch for layer `R_1` at position `t`.
3. That Merkle branch includes the sibling at `t ^ 1`.
4. The verifier uses `R_1[t]` and `R_1[t ^ 1]` for the next fold check.

So every layer provides a fresh authenticated sibling. A single computed value
never determines the whole next codeword.

### Is `R_1` a whole codeword? How can it come from one `F_d(x)`?

`R_1` is a whole codeword on the half-size domain. The prover computes all of
it by folding every adjacent pair of `R_0`.

The verifier samples only a few query paths. For one query, it reconstructs only
the two layer-0 values needed for that path:

```text
R_0[x], R_0[-x]
```

and checks that their fold equals the opened value of `R_1` on that path.

So:

```text
prover:   computes full R_1 array
verifier: checks sampled local consistency relations
```

This is the usual FRI soundness tradeoff.

### Why is `FinalPolyExt` a slice if the final folded value in a query is one value?

After `numRounds = log2(D)` folds, the codeword length is:

```math
N / D
```

In the WIOP PCS configuration, `N / D = 2`, so `FinalPolyExt` has two extension
field elements.

For a particular query position `s`, the verifier compares against:

```go
FinalPolyExt[s >> numRounds]
```

So each query checks one selected final value, but the final polynomial contains
all values on the final tiny domain.

## Multi-Degree FRI

### What does "multi-degree FRI" mean here?

The prover may have several virtual levels of different sizes:

```text
level 0: degree/size D
level 1: degree/size D/2
level 2: degree/size D/2^2
...
```

Instead of running separate FRI proofs for every level, the smaller levels are
mixed into the main folding path when the running codeword has folded down to
their size.

If a level has size `D_l`, it is introduced at:

```math
j_l = \log_2(D / D_l)
```

That schedule is computed by `buildProvePlan`.

### What happens in `buildProvePlan`?

`buildProvePlan` validates the levels and builds:

```go
levelAtRound map[int]int
```

This says which extra level is introduced at which folding round.

It checks:

- there is at least one level;
- `levels[0].D == p.D`;
- `levels[0].Evals` has length `p.N`;
- every level has at least one backing Merkle tree;
- extra levels have power-of-two sizes;
- an extra level's size divides the top size by a power of two;
- no two levels enter at the same round.

During folding, if a level enters at round `j+1`, its value is batched into the
fold output with:

```math
\alpha_j^2 \cdot aux
```

### Is there one root per FRI round?

There is one Merkle root for each intermediate running FRI layer:

```text
T_1, T_2, ..., T_{r-1}
```

The final folded layer is revealed directly as `FinalPolyExt`, so it has no
Merkle root.

Layer 0 roots are not stored in `FRIRoots`. They are supplied externally as the
roots of the input commitments. In PCS, these are the Merkle roots of committed
WIOP batches.

### Is there a single FRI proof for all dynamic-size columns in WIOP?

Yes, at the WIOP PCS level there is one opening proof that batches all committed
rounds and sizes sharing the same opening point `zeta`.

The flow is:

```text
all committed WIOP columns
    -> grouped into batch commitments
    -> claims collected from LagrangeEval queries
    -> one virtual DEEP quotient schedule
    -> one FRI folding proof
```

There can still be multiple input Merkle roots because each WIOP round is a
separate commitment batch, and the precomputed round may be another batch.

## Proof Structure

### What is inside `fri.Proof`?

In the current code:

```go
type Proof struct {
    LevelQueries [][]QueryLayer
    FRIRoots     []field.Octuplet
    FinalPolyExt []field.Ext
    FRIQueries   []Query
}
```

Meaning:

- `LevelQueries`: openings for extra multi-degree levels.
- `FRIRoots`: roots for running FRI layers `T_1..T_{r-1}`.
- `FinalPolyExt`: final tiny codeword, revealed directly.
- `FRIQueries`: query paths through the running FRI layers.

At the PCS layer:

```go
type OpeningProof struct {
    RowOpenings []QueryRowOpenings
    FRIProof    Proof
}
```

`RowOpenings` carry the original committed row values needed to reconstruct
the virtual DEEP quotient at sampled points.

### What are `inputValues` and `exactInputBranches`?

`inputValues` is an internal verifier helper. It stores the values that the
fold checker should use as FRI inputs:

- `Top`: the top layer pair `(self, sibling)` for every query;
- `Levels`: auxiliary lower-degree level values for every query;
- `Leaves`: expected row digests reconstructed by PCS;
- `TopSiblings`: expected conjugate row digests for the top level.

There are two modes:

1. Standalone FRI verification can decode values directly from Merkle proof
   leaves. In that case `inputValues` may be constructed from the proof.
2. PCS verification reconstructs layer values from opened committed rows and
   claimed values. In that case it supplies `inputValues` to bind the FRI proof
   to PCS row openings.

`exactInputBranches` controls branch shape strictness. Standalone FRI expects
the branch length to match the exact level size. PCS can open a level embedded
inside a larger multi-size input tree, so the branch may carry extra path data.

## WIOP PCS Compiler

### How does `wiop/compilers/pcs` use KoalaBear FRI PCS?

The compiler is an adapter:

```text
WIOP columns, rounds, cells, claims, transcript timing
    -> KoalaBear FRI PCS batches, roots, DEEP quotient, FRI proof
```

At compile time, `Compile`:

1. finds every round with committed columns;
2. commits the static precomputed round once;
3. hides interactive committed columns by marking them internal;
4. marks rounds as carrying a commitment;
5. registers one commit action per committed round;
6. registers one final opening action and one verifier action.

During proving:

1. each commit action calls `commitToRound`;
2. `commitToRound` pads columns to powers of two, groups them by size, and calls
   `fri.Commit`;
3. the Merkle root is stored in `Runtime.Commitments`;
4. `Runtime.AdvanceRound` absorbs the root into Fiat-Shamir instead of raw
   hidden columns;
5. the final opening action collects all `LagrangeEval` claims;
6. it registers every batch with `fri.PCS.AddOpening`;
7. it derives `alpha_DEEP`, creates a FRI prover state, folds, absorbs roots,
   absorbs the final polynomial, derives query positions, and opens the proof.

During verification, the verifier action replays the same transcript and calls
`fri.PCS.Verify` with roots, shapes, shifts, claimed values, challenges, and the
proof.

### Why does the compiler have `staticFRIOnce`?

The WIOP PCS parameters are fixed process-wide:

```text
inverse rate = 2
max column size = 2^22
query count = 229
```

Building all domains and encoders for the maximum schedule is relatively
expensive and deterministic. `sync.Once` ensures it happens only once per
process, safely even if multiple proofs are built concurrently.

Each proof then creates a fresh lightweight `fri.PCS` wrapper around those
shared parameters and encoders.

### Why is the query count 229?

`friNumQueries = 229` is the WIOP PCS security parameter for inverse rate 2.
The comment points to Ethereum's `soundcalc`; the selected value targets about
128 bits of security under the FRI soundness model used there.

In rough terms, more queries reduce the chance that a bad codeword avoids all
sampled checks. The exact number depends on the rate, degree bound, field, and
soundness model.

## Tests And Sanity Checks

### What is the Reed-Solomon encoding test checking?

The test checks that the encoded codeword agrees with polynomial evaluation on
the large domain, while respecting bit-reversed storage.

For each stored position `pos`, it computes:

```text
naturalIndex = bitReverse(pos)
x = domainPoint(domain, pos) = g^naturalIndex
```

If `naturalIndex` is a multiple of the inverse rate, then `x` is also a point
of the original small domain, so the encoded value should equal the original
input value.

Otherwise, the test evaluates the polynomial by Lagrange interpolation at `x`
and checks that the encoded value matches.

### Why do mutation tests matter here?

FRI/PCS proofs have many moving pieces:

- Merkle roots;
- row openings;
- sibling paths;
- final polynomial entries;
- claimed values;
- fold challenges;
- query positions.

Mutation tests intentionally alter proof fields and assert verification rejects
them. They are especially useful here because a malformed proof should return an
error, not panic or silently ignore extra data.

## Common Terminology Traps

### Are "rows" in code the same as columns in the PCS docs?

Not always.

In WIOP/PCS terminology, the committed objects are columns: each column is a
polynomial/vector.

In the Merkle tree, a leaf opens one evaluation row across many committed
columns. That is why `RowOpening` contains:

```go
Base []field.Element
Ext  []field.Ext
```

It is one Merkle leaf preimage at a domain position, not one WIOP column.

So:

```text
PCS column        -> one committed polynomial
Merkle row/leaf   -> values of many columns at one domain point
```

### What is the difference between input commitment roots and FRI roots?

Input commitment roots authenticate original committed WIOP columns.

FRI roots authenticate intermediate folded running layers.

They are different:

```text
input roots  -> roots of committed columns/batches
FRIRoots     -> roots of T_1, T_2, ..., T_{r-1}
FinalPolyExt -> final folded layer, revealed directly
```

## One End-To-End Example

Suppose the WIOP has five committed columns with three native sizes:

```text
A_1, A_2  have size 16
B_1, B_2  have size 8
C_1       has size 4
```

For readability, assume these columns are shown as one logical inventory. In the
real compiler they are still indexed by their owning WIOP commitment batch
(round root), size bucket, base/ext kind, and column position.

Now suppose two WIOP constraints request the following column rotations.

Round 1 has a constraint involving only `A` and `B` columns:

```text
g_1(A_1[0], A_2[+1], B_1[0], B_2[-1]) = 0
```

Round 2 has a constraint involving `A`, `B`, and `C` columns:

```text
g_2(A_1[+3], A_2[0], B_1[+2], B_2[+1], C_1[0]) = 0
```

The PCS compiler does not prove these algebraic constraints directly here.
Earlier compiler passes reduce them to `LagrangeEval` claims. The PCS compiler's
job is to bind those claimed evaluations to the committed columns.

The combined shift lists are:

```text
S_{A_1} = [0, 3]
S_{A_2} = [1, 0]
S_{B_1} = [0, 2]
S_{B_2} = [7, 1]    // -1 mod 8 = 7
S_{C_1} = [0]
```

At the shared opening point `zeta`, WIOP supplies matching claimed values. Let
`omega_16`, `omega_8`, and `omega_4` be the small-domain generators for the
size-16, size-8, and size-4 columns. Then, for example:

```math
y_{A_1,0} = A_1(\zeta)
```

```math
y_{A_1,3} = A_1(\zeta \omega_{16}^3)
```

```math
y_{B_2,7} = B_2(\zeta \omega_8^7)
```

and similarly for the other shifts.

With inverse rate 2, the RS-encoded codeword sizes are:

```text
A_1, A_2: 16 -> 32
B_1, B_2:  8 -> 16
C_1:       4 -> 8
```

For this example, assume all five columns are committed in one WIOP batch. Then
there is one input Merkle root:

```text
rho = root of Merkleize(
    Encode(A_1), Encode(A_2),   // encoded size 32
    Encode(B_1), Encode(B_2),   // encoded size 16
    Encode(C_1),                // encoded size 8
)
```

This is the proper multi-size commitment picture. The bottom layer of the tree
has 32 leaves, one per encoded `A` position. For a bottom position `j`, the
bottom leaf hashes:

```text
A_1_encoded[j], A_2_encoded[j]
```

The encoded `B` rows are auxiliary leaves one level higher. The `B` row aligned
with bottom positions `2j` and `2j+1` hashes:

```text
B_1_encoded[j], B_2_encoded[j]
```

The encoded `C` rows are auxiliary leaves one level higher again. The `C` row
aligned with bottom positions `4j..4j+3` hashes:

```text
C_1_encoded[j]
```

All of these row digests are folded into the same Merkle root `rho` through the
multi-size tree. A branch from a bottom `A` position therefore also carries the
aligned `B` and `C` auxiliary leaves when those levels are present.

So the PCS constructs three virtual DEEP quotient levels:

```text
F_16 over the size-32 codeword domain
F_8  over the size-16 codeword domain
F_4  over the size-8 codeword domain
```

Using the shorthand:

```math
Q_f(X; S_f) =
\sum_{h \in S_f}
\frac{f(X) - y_{f,h}}{X - \zeta \omega_{|f|}^h}
```

the levels are, schematically:

```math
F_{16}(X) =
Q_{A_1}(X; [0,3])
+
\alpha_{\mathrm{DEEP}} \cdot Q_{A_2}(X; [1,0])
```

```math
F_8(X) =
Q_{B_1}(X; [0,2])
+
\alpha_{\mathrm{DEEP}} \cdot Q_{B_2}(X; [7,1])
```

```math
F_4(X) =
Q_{C_1}(X; [0])
```

The exact alpha powers come from the canonical layout. The important point is
that alpha powers reset per native size in this PCS convention, so `F_16`, `F_8`,
and `F_4` each get their own size-local batching schedule.

The roots associated with these virtual levels are the same input root, not new
roots of the virtual quotients:

```text
levelRoots[0] = [rho]   // authenticates rows used to reconstruct F_16
levelRoots[1] = [rho]   // authenticates rows used to reconstruct F_8
levelRoots[2] = [rho]   // authenticates rows used to reconstruct F_4
```

The prover then computes the full virtual codeword `F_16` and starts FRI
folding. Since `D = 16` and `N = 32`, there are four folds and the final
polynomial has length `N / D = 2`.

The running FRI roots are computed as follows:

```text
T_1 = fold_alpha0(F_16) + alpha0^2 * F_8
rho_1 = MerkleRoot(T_1)

T_2 = fold_alpha1(T_1) + alpha1^2 * F_4
rho_2 = MerkleRoot(T_2)

T_3 = fold_alpha2(T_2)
rho_3 = MerkleRoot(T_3)

FinalPolyExt = fold_alpha3(T_3)    // length 2, revealed directly
```

So the inner FRI proof carries:

```text
FRIRoots     = [rho_1, rho_2, rho_3]
FinalPolyExt = two extension-field values
```

There is no `rho_4`, because the last fold is revealed rather than Merkle
committed.

For a query at top stored position `s` in the size-32 domain:

```text
A rows are opened at s
A conjugate rows are opened at s ^ 1
B rows are opened at s >> 1
C rows are opened at s >> 2
```

All four rows are authenticated against the same root `rho`: the `A` values are
bottom leaves, and the `B` and `C` values are aligned auxiliary leaves in the
multi-size branch. In the current proof shape these may appear as level-specific
Merkle openings under the same root; conceptually they are all openings of the
same committed multi-size table.

The verifier reconstructs:

```text
F_16(x), F_16(-x), F_8(x^2), F_4(x^4)
```

from those authenticated rows and the claimed values. The first fold checks:

```math
T_1(x^2)
=
\frac{F_{16}(x) + F_{16}(-x)}{2}
+
\alpha_0 \frac{F_{16}(x) - F_{16}(-x)}{2x}
+
\alpha_0^2 F_8(x^2)
```

So the size-8 virtual level is introduced exactly when the size-16 top level
folds down to size 8.

The next fold checks:

```math
T_2(x^4)
=
\frac{T_1(x^2) + T_1(-x^2)}{2}
+
\alpha_1 \frac{T_1(x^2) - T_1(-x^2)}{2x^2}
+
\alpha_1^2 F_4(x^4)
```

So the size-4 virtual level enters one round later. After that, the verifier
continues through the authenticated running FRI layers until it reaches the
revealed `FinalPolyExt`.

For one query path, the verifier performs these checks:

1. Rebuild the canonical layout from roots, shapes, and shifts, and check the
   claimed values match those shifts.
2. Check proof shape: `FRIRoots` must be `[rho_1, rho_2, rho_3]`,
   `FinalPolyExt` must have length 2, query positions must be in `[0, 32)`,
   and the proof must carry the expected number of row and FRI query openings.
3. Check every claim point `zeta * omega^shift` is outside the relevant encoded
   domain.
4. Replay Fiat-Shamir: derive `alpha_DEEP`; derive `alpha_0`, absorb `rho_1`;
   derive `alpha_1`, absorb `rho_2`; derive `alpha_2`, absorb `rho_3`; derive
   `alpha_3`; absorb `FinalPolyExt`; then derive query positions.
5. Authenticate input Merkle branches under the same root `rho`:
   - the bottom `A` opening at `s`, including the top conjugate sibling at
     `s ^ 1`;
   - the aligned `B` opening at `s >> 1`;
   - the aligned `C` opening at `s >> 2`.
6. Recompute row digests from `RowOpenings` and check they match the Merkle
   leaves and auxiliary leaves in those branches.
7. Reconstruct `F_16(x)`, `F_16(-x)`, `F_8(x^2)`, and `F_4(x^4)` from the
   authenticated rows, `zeta`, the shifts, the claimed values, and `alpha_DEEP`.
8. Authenticate running FRI branches:
   - `T_1` under `rho_1` at `s >> 1`;
   - `T_2` under `rho_2` at `s >> 2`;
   - `T_3` under `rho_3` at `s >> 3`.
9. Check the four fold equations:
   - round 0: `F_16` folds to the opened `T_1` value, with `F_8` mixed in;
   - round 1: `T_1` folds to the opened `T_2` value, with `F_4` mixed in;
   - round 2: `T_2` folds to the opened `T_3` value;
   - round 3: `T_3` folds to `FinalPolyExt[s >> 4]`.
