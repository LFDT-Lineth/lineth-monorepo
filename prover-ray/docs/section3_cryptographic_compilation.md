# 3. Cryptographic Compilation

## 3.1 Overview and Position in the Pipeline

The cryptographic compilation stage begins where ZKC ends. Its input is an
**Algebraic Intermediate Representation (AIR)** — a set of algebraic constraints
together with an **assignment** (the witness data) satisfying them. Its output is
a **concrete proof system**: a fully specified, non-interactive argument together
with the **prover runtime** that produces proofs for it.

This stage follows a methodology that is standard in the proof-systems
literature. Security analysis is most tractable in an *idealized* model, in which
the verifier is granted oracle access to the prover's messages and may pose a
wide class of queries that hold "for free." A real implementation cannot offer
such oracles, so the idealized protocol is **compiled** — through a sequence of
soundness-preserving transformations — into a protocol realizable in the
**standard model**, where the only assumptions are concrete cryptographic
primitives (a collision-resistant hash, the random oracle used for Fiat–Shamir,
and Reed–Solomon proximity).

The compilation proceeds in three conceptual phases:

1. **The Wizard-IOP model** (§3.2) — the abstract framework in which the AIR is
   expressed. The verifier may pose a wide class of queries against
   prover-committed columns.
2. **The Arcane compiler** (§3.3) — an ordered sequence of reduction passes that
   progressively **eliminate query types**, each at a small, quantifiable
   soundness cost, until only **univariate evaluation** queries remain. The
   result is a **Polynomial IOP (Poly-IOP)**.
3. **Polynomial commitment + Fiat–Shamir** (§3.4, §3.5) — the Poly-IOP is closed
   into a concrete interactive protocol by instantiating a polynomial commitment
   scheme (**multi-size FRI**), and made non-interactive via the
   **Fiat–Shamir transform** (which the framework applies continuously at round
   boundaries; see §3.5).

A central design benefit of this approach is that intermediate protocol-design
techniques (lookups, range checks, quotient arguments) become **automatable
compiler passes** rather than hand-built sub-protocols. An optimization or a
security fix applied to a single pass propagates automatically to every protocol
the compiler produces.

## 3.2 The Wizard-IOP Model

Wizard-IOP is the abstract proof model in which the AIR is expressed. It extends
the polynomial-IOP and tensor-IOP perspectives: the prover provides oracle access
to **columns** over a field, and the verifier may issue queries drawn from a
*wide class* rather than being restricted to univariate evaluations. This lets
protocols be specified **top-down** — from a high-level abstract protocol — while
the reduction to low-level univariate queries is left to the compiler.

### 3.2.1 Cells and Columns

The model has two data primitives:

- **Cell** — a single field value. A cell is always verifier-visible (it is part
  of the transcript) and is the natural carrier for scalar results produced
  during compilation (e.g. a claimed evaluation, an opened endpoint).
- **Column** — a vector of field values.

**Columns are dynamically sized by default.** A column's effective size is
determined per proving run from its assignment; columns are grouped into
**modules**, and all columns of a module share one domain size. A module may be
declared dynamic (size inferred at runtime from the longest assignment) or sized
(fixed at definition time). A statically sized column is the *exception*: it
arises when a column carries a **static assignment** known at compile time — the
canonical example being the precomputed table $(0, 1, \dots, B-1)$ introduced when
compiling a range check (§3.3.1). The overwhelming majority of columns in a real
arithmetization are dynamically sized.

> **Design note (sizing).** Because columns are dynamically sized, the
> heterogeneous-size problem that earlier formulations solved with explicit
> *column-splitting* and *column-sticking* passes no longer exists at this layer.
> Size homogenization is deferred to and absorbed by the polynomial commitment
> scheme (§3.4). The Arcane compiler has **no splitting/sticking passes**.

### 3.2.2 How columns reach the verifier

Columns are uniform: there is no per-column visibility level, and no column is
ever transported to the verifier in the clear. Every column declared in an
interactive round is committed to, and the verifier only ever sees

- the **commitment** to each round's columns, absorbed into the Fiat–Shamir
  transcript in place of the columns themselves (§3.4), and
- the **scalar openings** (cells) that the arithmetization passes reduce every
  constraint down to, together with the opening proof binding them to the
  commitment.

Note that "precomputed" is not a property of this classification: a precomputed
column is simply one declared in the offline precomputation round with a static
assignment. It is committed to like any other, once at compile time rather than
per proof.

Since the commitment is what ties the challenges to the witness, a protocol
compiled *without* the polynomial commitment pass has no witness binding: its
Fiat–Shamir challenges are independent of the columns, and the verifier has no
column data to check anything against. The commitment pass is what makes the
protocol sound, not a final size optimisation.

### 3.2.3 Column Views

Queries reference columns through a single lightweight indirection, the **column
view**: a reference to a column together with a **cyclic-shift offset**. We write
$v \ll k$ for the view of column $v$ shifted by $k$ positions (negative $k$
shifts the other way). A view with offset $0$ is the identity view.

A column view is neither a committed column nor a predicate; it is a way to make
queries over committed columns more expressive. The same column may be referenced
by several views across different queries.

> **Design note (references).** The richer abstract-reference machinery of the
> original formulation — in particular the interleaving of repeated columns — has
> been **deliberately removed**: it complicated the system and degraded
> efficiency. The current model retains only **expressions** (polynomial
> combinations of compatible views) and **cyclic-shift views**.

### 3.2.4 Random Coins

The verifier may send public-coin **random field challenges**, drawn once per
round, that subsequent parts of the protocol consume.

The implementation operates over the **KoalaBear** base field (a 31-bit prime,
$p = 2^{31} - 2^{24} + 1$). A single base-field element does not provide adequate
soundness, so **random coins are never sampled in the base field** — they are
always drawn from a **degree-6 (sextic) extension** $\mathbb{F}_{p^6}$. The extension is
built as the tower

$$
\mathbb{F}_{p^6} = \mathbb{F}_{p^2}[v]\,/\,(v^3 - (u + 1)), \qquad
\mathbb{F}_{p^2} = \mathbb{F}_{p}[u]\,/\,(u^2 - 3).
$$
@azam the reason for this extension versus the previous degree 4? 

Throughout this section, when soundness errors are written as $O(\cdot/|F|)$, the
quantity $|F|$ denotes the size of this **extension field** ($|F| = p^6 \approx 2^{186}$),
which is what makes the bounds cryptographically meaningful despite the small
base field. Committed columns produced during compilation that hold sums or
products of challenges (running sums, quotient shares) are therefore
**extension-field columns**.

### 3.2.5 Query Taxonomy

The verifier may issue the following queries. The set has been narrowed
substantially relative to the original formulation (see the note below).

- **Vanishing constraint** — an arithmetic expression that must vanish on every
  non-cancelled row of its module's domain. The same primitive covers two cases,
  decided by whether the expression is vector- or scalar-valued:
  - *Global*: $C(v_1, \dots, v_k) = 0$ on all rows, with views (cyclic shifts) allowed
    in $C$. By default the constraint is **cancelled at wrap-around positions**
    where a shift crosses the column boundary, which is the intended behaviour in
    the overwhelming majority of cases; an explicit cancellation set may be given.
  - *Local*: a row-pinned predicate, an arithmetic equality asserted at a single
    fixed position. Internally this is a scalar vanishing produced by pinning each
    referenced view to a fixed row; it is lifted to a global constraint by the
    local-vanishing pass (§3.3.4).
- **Range check** — for a column $v$ and bound $B$, asserts $0 \le v_i < B$ for all
  $i$. Reduced to an inclusion (§3.3.1).
- **Inclusion (lookup)** — for tables (groups of same-module views) $S$ and $T$,
  asserts every selected row of $S$ appears among the selected rows of $T$,
  ignoring multiplicities. Supports:
  - *Fragmented $T$* — $T$ given as several tables of equal width; the query holds
    against their union.
  - *Conditional inclusion* — per-side row selectors restrict the active rows on
    the $S$ side and/or the $T$ side.

    @azam, the implementation does not seem to follow the spec.

- **Log-derivative sum** — a list of filter-aware fractions
  $\mathrm{Filter}_k \cdot \mathrm{Num}_k / \mathrm{Den}_k$, reduced to a single field-element result asserted
  against a claim cell. This is the intermediate target of the inclusion pass
  (§3.3.2) and the natural home for conditional lookups (rows with a zero filter
  contribute nothing and their denominator is never inverted).
- **Univariate evaluation (UniEval)** — for a column $v$ of size $n$, interpret
  $v$ as the polynomial $v(X)$ taking value $v_i$ over the subgroup of $n$-th roots
  of unity; the query returns $v(x)$ at a verifier-chosen (typically random) point
  $x$. Several columns are usually grouped into one query at the same point,
  signalling to the compiler that they share an evaluation point. This is the
  **terminal** query type (§3.2.6).

> **Removed and folded queries.** **Permutation**, **fixed-permutation**,
> **projection**, and **inner-product** queries are **not present** in the current
> system (fixed-permutation may be reintroduced in the future; the others are not
> planned). There is also **no local-opening query type**: where the source
> arithmetization needs to expose a single cell $v[k]$ to the verifier, the
> framework's local-opening constructor directly produces a local constraint
> $v[k] - e = 0$ together with the public cell $e$. No intermediate
> LocalOpening object is ever materialized, even transiently, so the terminal
> query set is univariate-evaluation only by construction rather than by
> reduction.

### 3.2.6 Compilation as Query Elimination, and the Terminal Form

The Wizard-IOP is not proved directly. Instead, the **Arcane compiler** (§3.3)
applies an ordered sequence of passes, each of which **replaces one query type
with others** using a cryptographic technique whose soundness cost is explicitly
quantified. Each query carries a "reduced" flag: a pass that rewrites a query
marks the original reduced, and downstream passes and the verifier skip it.

The passes are ordered so that, after the last one, **only univariate-evaluation
queries remain**. This terminal form — a Wizard-IOP whose only active queries are
univariate evaluations at verifier-chosen random points — is exactly a
**Polynomial IOP**, which the polynomial commitment scheme (§3.4) then closes into
a concrete protocol.



@azam with FRI it is more natural to combine this step inside the PCS.
## 3.3 The Arcane Compiler

Arcane converts a Wizard-IOP into a Poly-IOP through the ordered passes below.
Each pass preserves the protocol's functionality up to a small polynomial
soundness loss. The pipeline order is:

1. **Range → inclusion** (§3.3.1)
2. **Inclusion → log-derivative sum** (§3.3.2)
3. **Log-derivative sum → running-sum constraints** (§3.3.3)
4. **Local → global** (§3.3.4)
5. **Global → univariate** (§3.3.5)

### 3.3.1 Range → Inclusion

For each distinct bound $B$, a single **precomputed** column enumerating
$(0, 1, \dots, B-1)$ is created on a dedicated module sized to the next power of
two at least $B$ (zero-padded), shared across all range checks with that bound.
Every range check $v < B$ is then replaced by the inclusion $v \subset b_B$, asserting that
all entries of $v$ are entries of the range column regardless of position or
multiplicity. This pass introduces no soundness loss of its own; its cost is
inherited from the inclusion reduction (§3.3.2).

### 3.3.2 Inclusion → Log-Derivative Sum

Inclusions are reduced using the univariate **log-derivative** lookup argument.
The core equivalence: for tables $S$ (of $s$ rows) and $T$ (of $t$ rows),
$S \subset T$ holds **iff** there exists a multiplicity vector $M \in \mathbb{F}^t$ (the count of
each $T$-row's occurrences across the $S$ side) such that the rational-function
identity

$$
\sum_{i<s} \frac{1}{\gamma + \langle S_i \rangle} \;=\; \sum_{j<t} \frac{M_j}{\gamma + \langle T_j \rangle}
$$

holds as a function of $\gamma$. For **multi-column** tables, each row $\langle \cdot \rangle$ is folded
into a single field element by a random linear combination in powers of a second
coin $\alpha$, i.e. $\langle S_i \rangle = \sum_c S_{i,c} \cdot \alpha^c$. The verifier checks the identity at
random $(\gamma, \alpha)$; soundness follows from the Schwartz–Zippel lemma for rational
functions.

This pass emits, per inclusion query, a single **log-derivative-sum** query whose
fractions encode the difference of the two sides — concretely, one
$+\mathrm{Filter}_S / (\gamma + \langle S \rangle)$ fraction per $S$ fragment and one $-M / (\gamma + \langle T \rangle)$
fraction per $T$ fragment — together with a verifier check that the resulting sum
is **zero**. A multiplicity column $M$ is committed per $T$ fragment.

Features map onto this structure directly:

- **Multi-column** tables → the $\alpha$-RLC folding above.
- **Conditional inclusion on $S$** → a $\mathrm{Filter}_S$ factor on the corresponding
  numerator (rows with $\mathrm{Filter}_S = 0$ contribute nothing).
- **Conditional inclusion on $T$** → the $T$-side selector is folded into the RLC
  by appending it as an extra table column and appending a constant-$1$ column on
  the $S$ side, so a plain inclusion over the augmented tables is equivalent.
- **Fragmented $T$** → one $M$ and one $-M/(\gamma + \langle T_f \rangle)$ fraction per fragment $f$,
  all collected into the same log-derivative-sum query.

**Soundness.** Modelling the protocol as evaluation of the rational identity at
random $(\gamma, \alpha)$, a malicious prover succeeds only if two distinct rational
functions agree at the sampled point. By Schwartz–Zippel for rational functions,
the statistical soundness error is $O\big((s + t)\cdot \mathrm{ncol} / |F|\big)$, where $\mathrm{ncol}$ is the
table width and $|F|$ is the extension-field size (§3.2.4). Batched and fragmented
cases reduce to this bound by viewing them as splittings of one side of the
identity.

### 3.3.3 Log-Derivative Sum → Running-Sum Constraints

A log-derivative-sum query — a list of fractions $f_k[i] = \mathrm{Filter}_k[i] \cdot \mathrm{Num}_k[i] / \mathrm{Den}_k[i]$
whose total $\sum_k \sum_i f_k[i]$ is asserted to equal a claim — is discharged
by introducing **running-sum columns** and local/global constraints.

For a group of fractions whose vector-valued sides share a module, the prover
commits to an **extension-field** running-sum column $Z$ accumulating their
per-row contribution (up to a fixed packing arity per $Z$, to bound the resulting
constraint degree). For a single fraction the recurrence is $Z[i] = Z[i-1] + f[i]$
with $Z[0] = f[0]$, expressed division-free as constraints:

- **Local constraint (initial condition):**
  $$Z[0] \cdot \mathrm{Den}[0] = \mathrm{Filter}[0] \cdot \mathrm{Num}[0]$$
- **Global constraint (recurrence):**
  $$(Z[i] - Z[i-1]) \cdot \mathrm{Den}[i] = \mathrm{Filter}[i] \cdot \mathrm{Num}[i] \quad (\text{cancelled at row } 0)$$

The endpoint $Z[n-1]$ is exposed to the verifier as a public cell $e$ — per
§3.2.5, the framework's local-opening constructor materializes this directly as
the local constraint $Z[n-1] - e = 0$ together with the cell $e$. A verifier
action then checks the initial condition of every $Z$ and that the **sum of
endpoints** across all $Z$ columns equals the query's claimed result (which, for
a reduced inclusion, is zero).

The prover-side accumulation is **filter-aware**: rows with $\mathrm{Filter}_k[i] = 0$ are
skipped and their denominator is never inverted. The constraint system does not
itself enforce non-zero denominators; the $\gamma$-randomization of §3.3.2 ensures
$\mathrm{Den}[i] \ne 0$ on every active row with overwhelming probability, which is what
uniquely pins down $Z$.

### 3.3.4 Local → Global

A local constraint is an arithmetic predicate $E(\cdot) = 0$ pinned to fixed
positions. It is lifted to a global constraint over the whole domain by
multiplying with a **Lagrange selector** that is $1$ at the pinned row and $0$
elsewhere.

Concretely, let the anchor be the minimum referenced position. Each pinned cell
$P[p]$ is rewritten as the shifted view $P \ll (p - \mathrm{anchor})$, so that at row
$x = \mathrm{anchor}$ the rewritten expression $\hat{E}(X)$ reads exactly the originally pinned
cells. The pass then registers the global constraint

$$
\hat{E}(X) \cdot L_{\mathrm{anchor}}(X) = 0 \quad \text{for all } X.
$$

At the anchor row the selector is $1$ and the constraint reproduces the original
predicate; at every other row the selector is $0$, so the global constraint holds
across the whole domain exactly when the local predicate holds.

The selector is **verifier-defined**: it is never committed by the prover, because
the verifier can evaluate it directly from a closed form. For the anchor at
position $0$,

$$
L_0(X) = \frac{X^n - 1}{n \, (X - 1)},
$$

and the selector at position $k$ is obtained by the substitution $X \to \omega^{-k} X$,
i.e. $L_k(X) = L_0(\omega^{-k} X)$, where $\omega$ is the canonical $n$-th root of unity.
Because the verifier computes $L_k(X)$ itself at the evaluation point of §3.3.5,
this lift adds **no commitment** to the protocol.

### 3.3.5 Global → Univariate

Global constraints are discharged with the standard PLONK quotient argument. Let
$v_1, \dots, v_k$ be the columns of size $n$ referenced by a global constraint with
expression $C$ of degree $d$, and let $v_\bullet(X)$ denote their Lagrange-basis
encodings. The constraint holds **iff** there exists a quotient $Q(X)$ with

$$
C(v_1(X), \dots, v_k(X)) = (X^n - 1) \cdot Q(X).
$$

The pass proceeds per module:

1. **Bucket and merge.** Group the module's global constraints by quotient ratio
   (the degree-determined number of shares $Q$ decomposes into). Within a bucket,
   draw a per-module **merging coin** and replace the bucket with a single
   constraint asserting that a random linear combination of its members vanishes.
2. **Commit the quotient.** The prover commits to the bucket's quotient as a set
   of **extension-field share columns** (the decomposition that accommodates $Q$
   having degree above $n$).
3. **Evaluate.** Draw a random **evaluation coin** $\alpha$ and issue univariate-
   evaluation queries for the witness columns and the quotient shares at $\alpha$.
4. **Check.** The verifier checks $C(v_1(\alpha), \dots) = (\alpha^n - 1) \cdot Q(\alpha)$, reconstructing
   $Q(\alpha)$ from the share evaluations and evaluating the verifier-defined selector
   factors (§3.3.4) at $\alpha$ directly.

After this pass the only remaining queries are univariate evaluations: the
protocol is a Poly-IOP.

**Soundness.** For a single constraint, the existence of $Q$ is equivalent to the
predicate, so a cheating prover succeeds only if the polynomial identity fails yet
holds at $\alpha$; by Schwartz–Zippel the error is $d / |F|$. The bucket merge adds an
error of at most $m / |F|$ for $m$ merged constraints (a merged constraint may
hold while a member does not). The combined error per bucket is $(d + m) / |F|$.

## 3.4 Multi-Size FRI

The Poly-IOP produced by §3.3 is closed into a concrete interactive protocol by
a **polynomial commitment scheme** based on **multi-size FRI** — a hash-based
univariate PCS that commits to several polynomials of *different sizes* under
one low-degree test, with size mismatch absorbed by FRI's natural halving
structure. The same generic construction underlies the PCS layers of OpenVM,
Plonky3, and others; the internal documentation refers to it as *multi-degree
FRI*.

The Poly-IOP delivers a collection of univariate-evaluation claims (§3.3.5):
tuples $(v_i, z_i, y_i)$ in which each $v_i$ is a committed column,
$z_i \in \mathbb{F}_{p^6}$ is its evaluation point (drawn by the global pass —
one per module), and $y_i$ is the claimed value. The PCS discharges all such
claims jointly.

Two structural points are worth surfacing up front:

- **Column-size homogenization is not required.** Earlier formulations were
  built on a PCS variant (the earlier "Vortex" scheme) under which all
  committed polynomials had to share a single domain size; reconciling
  heterogeneous-size columns therefore required explicit *splitting* and
  *sticking* passes inside the compiler. Multi-size FRI removes this
  requirement entirely: each column is committed at its native size, and the
  different sizes are joined into a single low-degree test natively by the
  PCS (§3.4.2). This is why Arcane (§3.3) has no splitting/sticking passes.
- **Evaluation claims at distinct points are discharged jointly, not via a
  separate reduction.** Earlier formulations carried a standalone
  "multi-point-to-single-point" compilation layer that reduced many evaluation
  claims at distinct points to a single claim at one point. That layer no
  longer exists: the per-claim quotients are folded into one polynomial per
  size **as part of the protocol below** (§3.4.2), with the randomness for the
  fold drawn at the FRI round where the corresponding size enters the test.

### 3.4.1 Commitment Layout

All committed columns are grouped by their **native size** $N$ (a power of
two). Within a size, base-field and extension-field columns share one
commitment. Each column $f$ of native size $N$ is Reed–Solomon encoded to a
domain of size $\rho N$, where $\rho$ is a **shared blowup factor** common to
all sizes — the precondition that makes single-test multi-size folding
possible.

The commitment is a **paired-leaf Merkle tree**: each leaf stores the pair
$\{ f(x), f(-x) \}$ for $x$ ranging over half the encoding domain ($\rho N / 2$
leaves per size-$N$ tree). The pairing matches FRI's two-to-one folding
structure, so the same Merkle openings serve both the polynomial-commitment
role and the FRI query role with shared Merkle paths.

Each size's tree is committed once and its root absorbed into the transcript
before any random coin depending on it is drawn (§3.5).

### 3.4.2 The Protocol

Multi-size FRI tests proximity, for several polynomials of different native
sizes simultaneously, by introducing each polynomial into a single FRI
low-degree test at the fold round where the running polynomial's degree has
been folded down to match. The polynomial introduced at the round
corresponding to size $N$ is the **folded evaluation quotient** $\Phi_N$,
defined below — a random linear combination of the per-claim evaluation
quotients at that size.

#### Folded evaluation quotient

Fix a native size $N$ and let $I_N$ index the Poly-IOP's evaluation claims at
that size (rotations $z\omega^s$ count as separate claim points). When the FRI
test reaches the round corresponding to size $N$, the verifier samples a
per-size coin $\beta_N \in \mathbb{F}_{p^6}$, and the folded evaluation
quotient is

$$
\Phi_N(X) \;=\; \sum_{i \in I_N} \beta_N^{\,i} \cdot \frac{v_i(X) - y_i}{X - z_i}.
$$

Each summand is a polynomial iff $v_i(z_i) = y_i$; if every claim at size $N$
holds, $\Phi_N$ has degree $< N$. The low-degree test below is exactly the
test that $\Phi_N$ is a polynomial of that degree, for every $N$ jointly.

Importantly, $\Phi_N$ is **not separately committed**: it is implicitly
determined by the already-committed columns $v_i$, the Poly-IOP's claims
$(z_i, y_i)$, and the coin $\beta_N$. Whenever the protocol needs $\Phi_N(x)$
at a query point, the verifier reconstructs it from the $v_i$ values opened
against the size-$N$ Merkle root via the formula above. The sequencing —
$\beta_N$ is drawn only after every $v_i$ at size $N$ has been committed — is
what the soundness argument relies on.

> **Naming note (DEEP).** The construction above shares its algebraic
> shape — $(P - y)/(X - z)$ batched by verifier randomness — with the *DEEP*
> quotient of DEEP-FRI literature (Ben-Sasson, Goldberg, Kopparty, Saraf,
> *DEEP-FRI*, ITCS 2020). It is **not** the DEEP construction, and the two
> should not be conflated. DEEP draws its evaluation point at *commitment
> time*, with no statement attached: its sole purpose is to eliminate
> pretenders from the proximity decoding list — to pin down *which* low-degree
> polynomial the committed codeword is close to, when more than one is at
> proximity distance. The construction here is different in kind: the
> evaluation points and claimed values come from the Poly-IOP statement
> produced by §3.3.5, and the quotient is what discharges that statement.
> Earlier internal documentation uses "DEEP quotient" for what this
> specification names the folded evaluation quotient; the names are kept
> distinct here deliberately.

#### The FRI loop

Let $N_{\max}$ be the largest native size and let $r = \log_2 N_{\max}$ be the
total number of folding rounds. The native sizes are ordered by decreasing
$N$: $N_0 = N_{\max} > N_1 > \cdots$. The **introduction round** of size $N_l$
is

$$
j_l \;=\; \log_2(N_{\max} / N_l).
$$

At round $j_l$, the running polynomial has degree $< N_l$, so it shares a
domain with $\Phi_{N_l}$ and the two may be added pointwise.

Before round $0$, the verifier draws $\beta_{N_{\max}}$ and the running
polynomial is initialized to $\Phi_{N_{\max}}$. For $j = 0, 1, \dots, r-1$:

1. **Level introduction.** If some native size $N_l$ has $j_l = j$ and
   $j > 0$, the verifier draws $\beta_{N_l} \in \mathbb{F}_{p^6}$ and the
   prover updates the running polynomial pointwise:
   $$
   \mathrm{running} \;\leftarrow\; \mathrm{running} \;+\; \Phi_{N_l}.
   $$
   No separate level-batching coin is needed: the randomness inside
   $\Phi_{N_l}$ — namely $\beta_{N_l}$, drawn at this round — already serves
   both the within-size batching role (combining the $|I_{N_l}|$ per-claim
   quotients into one polynomial) and the inter-level mixing role (binding
   the level addition to the running polynomial).
2. **Fold.** The prover commits the current running polynomial as a paired-leaf
   Merkle tree, the verifier draws a fold coin
   $\alpha_j \in \mathbb{F}_{p^6}$, and the prover folds in the standard FRI
   way, halving the domain.

After $r$ rounds the running polynomial has constant degree $< \rho$; the
prover transmits its evaluation vector explicitly.

#### Queries

The verifier samples $Q$ query positions on the encoding domain. For each
query position the prover opens, for every native size $N_l$, every column
$v_i$ at size $N_l$ at the corresponding paired-leaf position against the
size-$N_l$ Merkle root (one Merkle path per size, shared across columns of
that size). It also opens the running polynomial at every round $j \ge 0$
against the respective fold-round Merkle root.

Verification, for each query:

1. Authenticates all paired-leaf openings against their Merkle roots.
2. For each native size $N_l$, reconstructs $\Phi_{N_l}(x)$ and the sibling
   $\Phi_{N_l}(-x)$ at the query position from the opened $v_i$ values, using
   the $(z_i, y_i)$ from the Poly-IOP and the drawn $\beta_{N_l}$.
3. Replays the fold recurrence along the FRI path, checking at each
   introduction round $j_l$ that the running value after introduction equals
   the running value before introduction (carried over from the previous
   round's fold) plus the reconstructed $\Phi_{N_l}$ at the corresponding
   point.

### 3.4.3 Soundness

Two distinct soundness contributions enter the bound for this stage.

**Quotient-batching error.** For a fixed size $N$, the folded evaluation
quotient $\Phi_N = \sum_i \beta_N^{\,i} \, Q_i$ may, with probability
$O(|I_N| / |F|)$ over $\beta_N$, yield a low-degree $\Phi_N$ even though some
$Q_i$ is not a polynomial (i.e. some $v_i(z_i) \ne y_i$). The error is bounded
by Schwartz–Zippel. Aggregated across all sizes, the contribution is
$O\bigl((\sum_N |I_N|) / |F|\bigr)$.

**Low-degree-test error.** Multi-size FRI inherits its soundness from the
**proximity-gap framework** for Reed–Solomon codes. The operative theorem at
the join between the per-size codewords and the joint FRI test is **Correlated
Agreement**: if a random linear combination of codewords is close to a
low-degree polynomial, then with overwhelming probability *each* contributing
codeword is itself close to a low-degree polynomial.

This specification commits to operating in the **list-decoding (Johnson /
Guruswami–Sudan) regime** — the largest proximity radius the proximity-gap
framework currently supports. Two bounds in that regime are available:

- **Ben-Sasson, Carmon, Ishai, Kopparty, Saraf** — *Proximity Gaps for
  Reed–Solomon Codes* (FOCS 2020; *J. ACM* 70(5), 2023; ePrint
  [2020/654](https://eprint.iacr.org/2020/654)). Proves the proximity gap up
  to the Johnson radius, with $O(n^2)$ exceptional linear combinations (where
  $n$ is the codeword length).
- **Ben-Sasson, Carmon, Haböck, Kopparty, Saraf** — *On Proximity Gaps for
  Reed–Solomon Codes* (ePrint [2025/2055](https://eprint.iacr.org/2025/2055),
  ECCC TR25-169, 2025). The same team (minus Ishai, plus Haböck) tightens the
  bound at the Johnson radius from $O(n^2)$ to $O(n)$ — roughly a factor-$n$
  improvement in the proximity-side soundness error. The 2025 paper also
  proves a matching lower bound: any bound proving proximity *beyond* the
  Johnson radius must admit $\Omega(n^{1.99})$ exceptional combinations, which
  independently motivates anchoring the regime at the Johnson radius.

Concrete parameter selection — the Reed–Solomon rate $\rho$, the query count
$Q$, and which of the two bounds is plugged in — is the proof-system-design
choice that meets the security parameter of §1.2. The choice between the
2020/2023 and 2025 bounds is an open item under team discussion.

### 3.4.4 Hash Instantiation

The protocol uses a hash function in two places: the paired-leaf Merkle trees
at every level (§3.4.1), and the Fiat–Shamir transcript that derives every
verifier coin (§3.5). The security analysis above — and the proximity-gap
framework it rests on — assumes the hash behaves as a **random oracle**. This
is the standard assumption under which FRI is proved, and it is the
assumption this specification carries.

The concrete instantiation is **Poseidon2** — currently selected for its
arithmetization-friendliness inside the recursive verifier (§1.6), where every
hash invocation is itself proved at the next layer.

> **Provisional choice.** Recent cryptanalytic progress on Poseidon2 —
> notably results disclosed during the Ethereum Foundation hash-function
> competition — hints at weaknesses without yet constituting a break. The
> choice is therefore provisional: a successor will be selected and this
> specification updated. Until then, the spec uses "Poseidon2" as the named
> primitive throughout (§3.4 and §3.5) so that the eventual replacement is a
> mechanical substitution.

Performance characterization (proof size, verifier cost) is out of scope for
this version of the specification.

## 3.5 Fiat–Shamir

The interactive protocol above is made non-interactive with the **Fiat–Shamir
transform**. In this framework Fiat–Shamir is **not** a single post-hoc step:
it is applied **continuously at round boundaries** by the runtime. When a
round is closed:

1. Every oracle column committed in the round is absorbed into the transcript
   as its commitment, and every public cell is absorbed as raw field
   elements.
2. The runtime advances to the next round.
3. For each random coin declared in the new round, one extension-field
   ($\mathbb{F}_{p^6}$) challenge is derived from the transcript.

This binds every challenge to the full history of prover messages preceding
it. Fiat–Shamir is treated as the single most security-sensitive part of the
construction — historically the dominant source of real-world exploits in
deployed STARK / PLONK systems (cf. the *Frozen Heart* class of
vulnerabilities) — and is therefore implemented as one self-contained
transcript object with a single, auditable derivation path, rather than
scattered across the individual passes.

The hash is **Poseidon2**, the same primitive used for the PCS Merkle
commitments and subject to the same provisional-choice caveat (§3.4.4).

### 3.5.1 Transcript and Challenge Sampling

The transcript carries a Poseidon2 sponge whose state is an **octuplet** —
eight KoalaBear elements. The state is updated by writing absorbed field
elements directly through the Poseidon2 compression function; absorption
methods exist for base-field elements, $\mathbb{F}_{p^6}$ elements (six base
limbs per element), and vectors of either.

**Sampling an $\mathbb{F}_{p^6}$ challenge.** The runtime reads the current
octuplet and uses the first six components as the coefficients of a single
$\mathbb{F}_{p^6}$ element in the tower representation of §3.2.4; the
remaining two components are discarded.

**Domain separation between successive draws.** Immediately after each
sampling step, the runtime writes a single zero base-field element back into
the state. This *safeguard update* ensures that two successive challenge
draws from the same transcript produce different values, without requiring an
external counter or per-coin label. It is the entire domain-separation
discipline between draws within a round.

**Named challenges.** Where a coin needs a stable, position-independent name
(used, for example, by multi-size FRI to bind level / fold / query coins to
fixed roles), the runtime derives the challenge from a seed octuplet plus a
string label: the label is hashed with BLAKE2b into eight base-field
elements, the sponge state is temporarily replaced by the seed, the label
elements are absorbed, and the $\mathbb{F}_{p^6}$ challenge is sampled. The
original transcript state is restored afterward, so named-challenge
derivations do not perturb the running transcript.

**Random integer batches.** Query positions (and other power-of-two-bounded
integer samples) are produced by reading successive eight-element digests
from the transcript and reducing each component modulo the upper bound.

### 3.5.2 Domain Separation Under Dynamic-Size Columns

Earlier formulations of the system absorbed only fixed-size objects, so the
boundary between successive ingestions was implicit in the transcript
ordering — every absorbed object's length was known from the protocol
definition, and the verifier could replay the ingestion without any
explicit length information in the transcript. With **dynamic-size columns**
(§3.2.1), public-data columns can have a length that depends on the proof
itself, and per-element absorption without explicit length information is no
longer self-delimiting: a prover could otherwise split one length-$n$ column
into two of total length $n$ and the transcript would not see the
difference.

The system addresses this by encoding each dynamic-size module's length as a
base-field element — one element suffices, since every length fits in a
KoalaBear scalar — and prepending the length to its absorbed data. The
length is included in the proof so the verifier replays the same ingestion.
Fixed-size content remains self-delimiting by its protocol-defined length
and absorbs as before.

### 3.5.3 Grinding

The Fiat–Shamir component supports a *grinding* step — a per-round
proof-of-work soundness amplification standard in production FRI-based
systems. After a transcript ingestion that precedes a challenge derivation,
the prover finds and emits a nonce such that absorbing the nonce produces a
state whose first $k$ base-field components have a specified number of
leading zero bits in their binary representation. The verifier checks the
leading-zero condition on the post-absorption state cheaply.

Each grinded bit raises the cost of a Fiat–Shamir forgery by a factor of two:
an adversary attempting to bias a challenge by rewinding the transcript must
additionally solve the grinding puzzle for each candidate. With a grinding
budget of $b$ bits, the effective soundness of the round-boundary derivation
is raised by $b$ bits over what the bare proximity-gap / Schwartz–Zippel
accounting of §3.4.3 / §3.3 delivers, at the cost of $2^b$ expected hashes on
the prover side and one cheap check on the verifier side.

Locating grinding inside the transcript object (rather than as a separate
proof component) is a deliberate architectural choice: grinding operates on
the sponge state and so co-locates naturally with the rest of the transcript
discipline, keeping the soundness-critical surface inside one auditable
module.

---

### Open items carried into this draft

- **§3.4.3 proximity-gap bound** — list-decoding (Johnson) regime committed;
  choice between the 2020/2023 and 2025 bounds within that regime is under team
  discussion.
- **§3.4 concrete parameters** — Reed–Solomon rate $\rho$ and query count $Q$
  remain to be fixed against the security parameter of §1.2.
- **§3.4.4 / §3.5 hash** — Poseidon2 is provisional pending the
  Ethereum-Foundation-competition cryptanalytic picture; a successor will
  trigger a global rename across §3.4 and §3.5.
- **§3.5.3 grinding budget** — concrete number of grinded bits $b$ to be
  fixed against the security target of §1.2; implementation in active
  development.
- **§3.5 round-by-round soundness statement** — formal soundness statement
  for the round-by-round Fiat–Shamir transform over $\mathbb{F}_{p^6}$
  remains to be filled in.
