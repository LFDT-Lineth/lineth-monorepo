# Prover Workflow

This note describes the current `Prove` workflow, with emphasis on the
DEEP quotient and on the bridge between FRI openings and the commitments to the
trace/setup/AIR polynomials.

It is a first draft for readers who already know STARK-style polynomial IOPs and
FRI. The multi-degree FRI protocol is only summarized here; see
`internal/fri/multi_degree_fri.txt` for the dedicated protocol note.

## Main Objects

A statement contains:

- a compiled `board.Program`;
- a `VerificationKey`, i.e. setup Merkle roots for fixed/setup columns;
- public inputs.

A witness contains:

- a witness `trace.Trace`;
- a `ProvingKey`, i.e. setup trace columns in Lagrange form plus the setup
  Merkle trees.

The prover and verifier both build the same canonical commitment layout:

```text
setup trees, decreasing N
trace round 0 trees, decreasing N
trace round 1 trees, decreasing N
...
AIR quotient trees, decreasing N
```

Each tree groups polynomials with the same native domain size `N`. Base-field
and extension-field rails share one tree for a given size. Inside a tree, a
Merkle leaf stores the paired evaluations

```text
{ f(w^i), f(-w^i) }
```

over the Reed-Solomon encoded domain of size `RATE * N`. Therefore a tree for
size `N` has `RATE * N / 2` leaves.

This paired-leaf convention is used both by the trace/setup/AIR commitments and
by FRI level commitments.

## High-Level Prove Sequence

The prover sequence is:

1. Initialize the prover runtime.
2. Execute witness-generation steps and commit trace columns at Fiat-Shamir
   boundaries.
3. Compute and commit AIR quotient chunks.
4. Derive `zeta`.
5. Compute all evaluations at `zeta` needed by the verifier.
6. Derive `alpha_DEEP` from those evaluation claims.
7. Build one DEEP quotient polynomial per distinct module size.
8. Commit the DEEP quotients and prove them with multi-degree FRI.
9. Open all original commitments at the FRI query positions.

The final proof contains:

- trace/AIR commitment roots, excluding setup roots;
- claimed values at `zeta`;
- exposed values;
- DEEP quotient commitment roots;
- the multi-degree FRI proof;
- Merkle openings of setup, trace, and AIR trees at the FRI query positions.

## Transcript Order

The transcript is shared by the main protocol and FRI.

During runtime initialization:

- the hash backend identifier is bound;
- setup roots are bound;
- public inputs are bound;
- the challenge names for trace rounds, `zeta`, and `alpha_DEEP` are registered.

During `ExecuteSteps`, each Fiat-Shamir boundary commits the columns that must
exist before the corresponding challenge:

```text
commit roots for round r dependencies
bind those roots to challenge_r
derive challenge_r
write challenge_r into the trace as an extension-field column
```

After all witness-generation steps, `ComputeAIRQuotients` commits the AIR
quotient chunks and binds their roots to the `zeta` challenge. Then `zeta` is
derived.

After the prover has computed all values at `zeta`, it binds those values to
`alpha_DEEP` and derives `alpha_DEEP`. No new commitment is created between
`zeta` and `alpha_DEEP`; the binding data is the set of claimed evaluations that
the DEEP quotient will batch.

FRI then appends its own challenges to the same transcript.

## Trace Commitments

The program is divided into Fiat-Shamir rounds. Each round has a list of column
dependencies, `program.FScolumnsDependencies[round]`. When a round is reached,
the prover:

1. groups the dependent columns by module size `N`;
2. commits each group with `RSCommit`;
3. records the tree in the canonical layout;
4. stores the Merkle root in `proof.Commitments`;
5. binds the root before deriving the round challenge.

Setup columns are not stored in `proof.Commitments`. Their roots come from the
verification key. On the prover side, setup columns are merged into the trace so
that expression evaluation can treat setup and witness columns uniformly.

## AIR Quotients

Each module has a vanishing relation `V_m(X)` over its native domain of size
`N_m`. The intended AIR statement is:

```text
V_m(X) = 0 on X^N_m - 1
```

Equivalently, there exists a quotient polynomial `Q_m(X)` such that:

```text
V_m(X) = (X^N_m - 1) * Q_m(X)
```

`ComputeAIRQuotients` computes this quotient from the trace. If the quotient is
larger than the native module size, it is split into chunks of size `N_m`:

```text
Q_m(X) = q_0(X) + X^N_m q_1(X) + X^(2N_m) q_2(X) + ...
```

Each chunk is committed like any other polynomial, grouped by size. These AIR
chunk roots are bound before deriving `zeta`.

The prover then evaluates each AIR chunk at `zeta` and stores the values in
`proof.ValuesAtZeta`.

The verifier reconstructs:

```text
Q_m(zeta) = q_0(zeta) + zeta^N_m q_1(zeta) + zeta^(2N_m) q_2(zeta) + ...
```

and checks:

```text
V_m(zeta) = (zeta^N_m - 1) * Q_m(zeta)
```

This check is purely algebraic in the claimed `ValuesAtZeta`. The DEEP quotient
and FRI bridge are what tie those claimed values back to the committed
polynomials.

## Values At Zeta

After `zeta` is sampled, the prover computes every value needed to evaluate the
AIR relations at `zeta`.

For an unrotated column `c`, it stores:

```text
c(zeta)
```

For a rotated column `c[k]`, it stores:

```text
c(zeta * omega^k)
```

where `omega` is the generator of the module domain.

The verifier also populates some `ValuesAtZeta` entries by itself:

- challenge columns are recomputed from the transcript;
- Lagrange columns are evaluated directly;
- public-input columns are reconstructed from the public input entries;
- exposed columns are reconstructed from values carried in the proof.

## DEEP Quotient Layout

The prover and verifier both call `BuildDeepQuotientLayout(program)`. This
layout fixes a deterministic order for all polynomials batched by the DEEP
quotient.

For each distinct size `N`, in decreasing order, the layout contains:

1. all trace/setup columns appearing in vanishing relations, grouped by rotation
   shift;
2. all AIR quotient chunks of size `N`.

Columns are deduplicated by their expression leaf key, so the same column/shift
claim is bound once per size.

The same layout is used for three things:

- binding claimed evaluations before deriving `alpha_DEEP`;
- building the prover's DEEP quotient polynomials;
- recomputing the bridge checks in the verifier.

## Alpha Deep

`alpha_DEEP` batches all DEEP evaluation claims. Before sampling it, the prover
binds every claimed evaluation that will enter the DEEP quotient:

```text
for each size N
  for each rotation shift
    bind c(zeta * omega^shift) for every selected column c
  bind q_i(zeta) for every AIR quotient chunk q_i of size N
derive alpha_DEEP
```

The verifier repeats the same binding order. This prevents the prover from
choosing `alpha_DEEP` before fixing the claimed evaluations.

## DEEP Quotient Construction

Fix a size `N`. We should build one extension-field DEEP quotient polynomial
`DQ_N` for all claims of that size.

### Rotated Trace And Setup Columns

For a fixed rotation shift `s`, define:

```text
z_s = zeta * omega^s
```

Let the selected columns at this size and shift be:

```text
c_0, c_1, ..., c_t
```

and let their claimed evaluations be:

```text
v_i = c_i(z_s)
```

Using powers of `alpha_DEEP`, the prover forms the linear combination:

```text
C_s(X) = c_0(X) + alpha c_1(X) + alpha^2 c_2(X) + ...
v_s    = v_0    + alpha v_1    + alpha^2 v_2    + ...
```

Then it computes the DEEP quotient:

```text
DQ_s(X) = (v_s - C_s(X)) / (z_s - X)
```

This is done in Lagrange form over the native domain of size `N`.

### AIR Quotient Chunks

AIR quotient chunks are not rotated. If the chunks of size `N` are:

```text
q_0, q_1, ..., q_u
```

and their claimed evaluations are:

```text
w_i = q_i(zeta)
```

the prover continues the same powers of `alpha_DEEP` and forms:

```text
C_air(X) = alpha^a q_0(X) + alpha^(a+1) q_1(X) + ...
v_air    = alpha^a w_0    + alpha^(a+1) w_1    + ...
```

Then:

```text
DQ_air(X) = (v_air - C_air(X)) / (zeta - X)
```

### One Polynomial Per Size

The per-size DEEP quotient is:

```text
DQ_N(X) = sum over shifts s of DQ_s(X) + DQ_air(X)
```

The prover Reed-Solomon encodes `DQ_N` on the domain of size `RATE * N` and
builds a paired-leaf Merkle tree. The root is stored in
`proof.DeepQuotientCommitment`.

There is one such polynomial for every distinct module size. These polynomials
are the levels passed to multi-degree FRI.

## Same-Degree Folding

We fold same-degree objects before invoking FRI.

For a fixed size `N`, all trace/setup columns and AIR quotient chunks that need
to be linked to `zeta` are folded into one DEEP quotient polynomial `DQ_N` using
the random powers of `alpha_DEEP`.

This includes AIR quotient chunks. They are not treated as a separate PCS
object after this point: their committed openings enter the same DEEP quotient
bridge as trace/setup column openings.

The result is:

```text
one DEEP quotient polynomial per distinct size N
```

The remaining different sizes are handled by multi-degree FRI. The largest
`DQ_N` starts as FRI level 0. Smaller `DQ_N` polynomials are introduced at the
FRI round where the running degree has been folded down to their degree bound.
At that round, FRI samples a batching challenge and mixes the level polynomial
pointwise into the running polynomial.

This note does not try to explain the full multi-degree FRI protocol. The
important point here is that FRI checks low-degree proximity for the folded
collection of all `DQ_N` polynomials without forcing every size to be padded to
the largest domain.

## Opening The Original Commitments

FRI proves low degree of the DEEP quotient codewords. It does not, by itself,
prove that those codewords were computed from the committed trace/setup/AIR
polynomials. We should therefore add a bridge.

After `fri.Prove`, the prover knows the FRI query positions. For each query
position `s`, it opens every original commitment tree in the canonical layout:

```text
setup trees
trace-round trees
AIR quotient trees
```

For a tree with native size `N`, the prover opens:

```text
s mod (RATE * N / 2)
```

and sends the paired leaf:

```text
{ f(X), f(-X) }
```

for every polynomial in that tree, plus the Merkle path.

These openings are stored in:

```text
proof.PointSamplings[query][tree]
```

The verifier first checks all these Merkle paths against the setup roots and the
proof commitment roots. This authenticates the local values of the committed
trace/setup/AIR codewords at the same points used by FRI.

## FRI Bridge Check

The verifier then recomputes the DEEP quotient locally at each FRI query point.

For a query and size `N`, let:

```text
X  = the sampled point on the RATE * N domain
-X = the paired sibling point
```

Using `proof.PointSamplings`, the verifier reads the committed values:

```text
c_i(X), c_i(-X)
q_j(X), q_j(-X)
```

Using `proof.ValuesAtZeta`, it reads the claimed out-of-domain values:

```text
c_i(zeta * omega^s)
q_j(zeta)
```

Using the same `alpha_DEEP` powers and the same layout, it recomputes:

```text
DQ_N(X)
DQ_N(-X)
```

from the quotient formulas above.

It then compares these values against the DEEP quotient values opened by the
FRI proof:

- for the largest size, against the level-0 FRI query;
- for smaller sizes, against the corresponding multi-degree FRI level query.

This is the explicit bridge:

```text
original Merkle commitments
        |
        | PointSamplings + checkFRIBridge
        v
DEEP quotient codewords
        |
        | multi-degree FRI
        v
low-degree claim
```

## Why The Bridge Is Sound

The verifier checks three different kinds of evidence:

1. Merkle openings bind sampled values to committed codewords.
2. The DEEP quotient bridge checks that those sampled values and the claimed
   `ValuesAtZeta` satisfy the DEEP quotient identities at the FRI sample
   points.
3. Multi-degree FRI checks that the DEEP quotient codewords are close to
   low-degree polynomials.

The soundness intuition is:

- If a committed column is close to a low-degree polynomial `c`, and a claimed
  value `v` is really `c(z)`, then `(v - c(X)) / (z - X)` is also low degree.
- If `v` is not the value of the low-degree polynomial committed by the prover,
  the resulting quotient is not a low-degree polynomial correlated with the
  committed codeword except with small probability.
- The random `alpha_DEEP` combines many such claims into one relation, so the
  prover cannot choose cancellations between different bad claims after seeing
  the challenge.
- The verifier samples the same points in the original commitments and in the
  DEEP quotient FRI proof. Therefore the prover must make the committed
  codewords and the FRI-proven quotient agree under the DEEP relation at random
  locations.

This is the role of the Correlated Agreement Theorem. Informally, it says that
if the DEEP quotient codeword is low-degree and agrees at the sampled points
with the quotient relation induced by the committed codewords, then the claimed
out-of-domain evaluations are jointly consistent with the same low-degree
objects, except with the usual proximity/query soundness error.

This theorem is what justifies using the bridge checks to connect:

```text
AIR relation checks at zeta
```

to:

```text
Merkle commitments to trace/setup/AIR codewords
```

without opening every committed polynomial at `zeta` through a separate PCS
proof.

## Verifier Workflow Summary

The verifier mirrors the prover:

1. Rebuild the canonical layout.
2. Reconstruct the flat root list:

   ```text
   setup roots from VerificationKey ++ proof.Commitments
   ```

3. Replay transcript bindings to derive trace challenges and `zeta`.
4. Reconstruct verifier-owned `ValuesAtZeta`: challenges, public inputs,
   exposed values, and Lagrange columns.
5. Check logup buses.
6. Check AIR equations at `zeta`.
7. Bind all DEEP evaluation claims and derive `alpha_DEEP`.
8. Verify the multi-degree FRI proof for the DEEP quotient commitments.
9. Verify Merkle openings in `proof.PointSamplings`.
10. Run `checkFRIBridge` to tie the FRI-proven DEEP quotients to the original
    trace/setup/AIR commitments.

The order matters: `alpha_DEEP` is derived before FRI appends its own
challenges, and after all evaluation claims entering the DEEP quotient have
been transcript-bound.

## Current Limitations And Notes

- This document treats multi-degree FRI as a black box. It only states where
  smaller-degree DEEP quotient polynomials enter the FRI folding process.
- The DEEP quotient is currently built on the extension rail.
- Setup columns are committed once in `Setup`; the prover carries the setup
  trace in the proving key so it can evaluate setup columns when building the
  DEEP quotient and query openings.
- Public input columns are materialized in the prover trace, while the verifier
  reconstructs their value at `zeta` from the public input entries.
- Exposed values are proof-carried values reconstructed by the verifier at
  `zeta`; they are distinct from statement public inputs even though they share
  similar data representation.
