# Multi-size Vortex commitment 

The Vortex commitment scheme, inspired from Ligero, Brakedown and others from 
the litterature, naturallly allows committing to matrices. For zkVM purposes, it 
would be convenient to have a dynamic multi-size commitment scheme. A popular 
approach for that is what we call the ***Plonky3*** variant of FRI.

## Plonky3

In a few words, the Plonk3 variant works by attaching the hash of the RS-encoded 
committed rows to different levels of the same Merkle-tree. The algo can be 
summarized as follows:

### Grouped / Batched Commitments by Log-Height

Trace columns are grouped by their trace height (i.e., by their log₂(height)). All columns of the same height are committed together using a single Merkle tree (over FRI evaluation domain). Columns of different heights go into separate commitment rounds / separate Merkle trees. So if you have chips with heights 2^16, 2^18, and 2^20, you get (at least) three separate Merkle commitments.

An optimization is batch different-sizes Merkle-trees in the same commitment by attaching the columns hashes to different levels of the same Merkle trees. This works because the spot-check will be done in the same Merkle paths.

### FRI with Multiple Oracles

The FRI (Fast Reed-Solomon Interactive Oracle Proof) protocol is then run in a way that handles multiple polynomial oracles of different degrees:

During the FRI folding process, polynomials of different degrees "enter" the protocol at different folding stages. A polynomial of degree 2^16 doesn't need to be present at the folding steps above degree 2^16. Concretely, the verifier's random challenge is used to batch all polynomials of the same degree together, and as FRI folds downward, smaller-degree polynomials join in at the appropriate level. This is sometimes called "mixed-degree FRI" or "multi-degree commitment" and is a known technique (also described in various STARK references). The key insight is that FRI naturally handles this because folding from degree d to d/2 is iterative, and you can introduce new polynomials at any folding level.

## Why it is not that simple with Vortex

Unlike with FRI, in Vortex there is no recursive argument that the linear combination $U_\alpha$ of the original codewords polynomials are close to RS and there is therefore no clear way other than having different $U_\alpha$ for each size. 

## Multi-size Vortex: adding a multilinear layer

The idea is to replace the current univariate MPTS compilation phase by one based-on sumchecks to convert "many-poly-many-univariate-points" evaluation queries into "many-polys-same-multilinear-points". At this point, we can use SWIRL|Binius's strategy for packing multilinear evaluation.

* The rational is that packing multilinear evaluation is simpler than univariate evaluations. We don't need to change our PCS.

* Since it only happen at the MPTS step, we don't need to convert our entire stack to multilinear. It would require:
    * Adapting Vortex to work with multilinear evaluations
    * Re-designing, re-writing a new MPTS logic

One advantage is that it gives us foot in the multi-linear world. And this can be incrementally upgraded by switching more and more steps to multilinear. It also allows us to keep Vortex as our main commitment scheme.

### Univariate to Multilinear correspondance

Let $P(X)$ a univariate polynomial of degree $2^n - 1$. Let $\sigma$ be the *monomial correspondance* map, mapping univariate polynomials of degree less than $2^n$ to $n$-linear polynomials coefficient-by-coefficient.

\begin{align}
\sigma \colon \mathbb{F}_{\leq 2^n - 1}[X] &\to \mathcal{M}_\mathbb{F}[Z_0, \ldots, Z_{n-1}] \\
\sum_{i < 2^n-1} p_i X^i &\to \sum_{\mathbf{b} \in \mathcal{H}_n} p_{r(\mathbf{b})} \prod_k X_k^{b_k} \\
\end{align}

Where $\mathcal{H}_n = \{0, 1\}^n$ is the boolean hypercube of dimension $n$ is the bit-recomposition function used to map the coefficients of $P$ to the one of $\bar{P}$ 

\begin{align}
r \colon \mathcal{H}_n &\to [[0; 2^n-1]] \\
\mathbf{b} &\to \sum_k b_k 2^k
\end{align}

Using this correspondance, we can transfer evaluation claims in the univariate world to the multilinear world via the following equivalence.

$$P(x) = y \Leftrightarrow \bar{P}(1, x, x^2, x^4, \ldots, x^{n-1}) = y$$

The idea is that the prover claims many instances of the form $P(x) = y$ and then proves $\bar{P}(1, x, x^2, x^4, \ldots, x^{n-1}) = y$ for each claim using a special batching sumcheck.
$\newcommand{\eq}{\mathsf{ eq }}$
### Reducing many multilinear claims into a single one

The multinear land of the ZK worlds has its own way to reduce multiple evaluation claims over different polynomials and points into a batched claim over all polynomials at the same point. For simplicity, we will assume that all the polynomials have the same size. Then, we will discuss how to deal with "many-sizes" and finish outlining how the packing works. We refer the reader to the seminal **LFKN92** paper for an introduction to the sumcheck protocol.

Assume, $m$ polynomials $P_0, P_1, \ldots P_{m-1}$ all $n$-linear and one claim $(\mathbf{z}_i \in \mathbb{F}^n, y_i \in \mathbb{F})_{i < m}$ for each.

Let us define the multilinear polynomial $\eq$, an analog of the Lagrange polynomial for multilinear polynomials. Namely, $\eq$ interpolates the logical equality function over the hypercube: if $\mathbf{z, t} \in H_n$, then $\eq(z, t) = 1$ if $\mathbf{z} == \mathbf{t}$ and $0$ otherwise.

\begin{align}
\eq \colon &&\mathbb{F}^n \times \mathbb{F}^n &\to \mathbb{F} \\
&&\mathbf{z, t} &\to \prod_{k < n} z_k t_k + (1 - z_k)(1 - t_k)
\end{align}

The truthfulness of one of the claims is equivalent to the following relation rewriting $P(x) using $\eq$. This is analog to Lagrange basis decomposition of a polynomial

\begin{align}
y = \sum_{h \in \mathcal{H}_n} P(h) \eq(h, x) 
\end{align}

To fold the multiple claims, the prover and the verifier engage in the following subprotocol. The verifier sample and sends the random coin $\rho \in \mathbb{F}$ that will be used to fold the evaluation claims.

Note that, the claims being satisfied is equivalent to the following sum relation.

\begin{align}
\sum_{j < m} y_j \rho^j &= \sum_{h \in \mathcal{H}_n} \sum_{j < m} \rho^j P_j(h) \eq(h, x_j)
\end{align}

The verifier may compute the left-hand side of the relation on his own as it only depends on public data and engage in a sumcheck protocol on the right-hand side. At the end of the sumcheck protocol, the prover hold to a claim that for $h'$ sampled during the sumcheck protocol:

$$y' = \sum_{j < m} \rho^j P_j(h') \eq(h', x_j)$$

The prover justifies this by sending evaluations claims $(h', u_j)_{j<m}$ for $P_j(h')$, these are our "same-point-reduced-evaluation-claims" and the verifier checks that $y' = \sum_{j < m} \rho^j u_j \eq(h', x_j)$ as this can be evaluated in log-time.

The above-described technique can be extended to support $P$ with varying number of variables. The sumcheck will work in the same way. Interestingly, it is also possible to pack all the $P_i$ in the same polynomial $Q$ with $n' = n + \log m$ variables by setting $P_j(x) = Q(a_j, x)$ where $a_j$ is a locator tuple of variables indicating the positions of $P_j$ inside of $Q$. This, instead of reducing claims formed as $y_j = P_j(x_j)$, we reduce claims as $y_j = Q(a_j, x_j)$. The sumcheck trick will resolve all of them as a unique claim $Q(h') = u$.

### Impacts on Vortex as a commitment scheme

The remaining part is that Vortex is currently described as a univariate PCS. But we could easily lift it to becoming more general linear-commitment scheme. This can be done as follows:

The commitment protocol is almost the same as before. The difference is that we commit to only one polynomial. We need to split in rows. This is done by considering that some variables of $Q$ index rows and some of them index columns $Q(X) = Q(x_{r}, x_c)$. For the evaluation phase, the prover proves partial evaluations of each rows independantly $\forall h \in n_r \colon Q(h, x_c) = y_c$ and the verifier uses that to reconstruct the evaluation of the entire poly. In the following, we use $y_h$ and $y_i$ indiscriminately to index the opening claims for each rows depending on what is more convenient at hand.

The commitment phase is unchanged:
* RS-encode the rows corresponding to $Q(X) = Q(h, x_c)$. The exact mapping used for RS is $\sigma^-1$. It does really not matter which mapping is used here. It just needs to be bijective.
* Hash the columns. Merkle-ize the hashes. Return the root of the Merkle tree


The opening phase is also quite similar:
* Verifier samples $\alpha$
* Prover sends a linear combination of the rows $U_\alpha$ which is the encoding of $u_\alpha(Z) = \sum_{i} Q(h_i, Z) \alpha^i$.
* Verifier checks that,
    1)  $U_\alpha$ is a codeword for $u_\alpha$, the corresponding multilinear polynomial.
    2)  $u_\alpha(x_c) = \sum_i y_i \alpha^i$
    3)  $y = \sum_h \eq(h, x_r) y_h$
* Verifier samples $t$ positions to open, toward a spot-check phase with the prover. That part is completely identical to the rest.

In summary, the only differences are in 2) because the $u_\alpha$ evaluation is a multilinear evaluation. and 3) because the check does not exist currently. The poses a few challenges on the self-recursion compilation because we'd need a wizard tool to prove multilinear evaluations using only global constraints and we would need something efficient. Some research is needed.

## Symbolic expansion of the smaller columns

Another idea for more efficiently committing to variable size columns would be to artificially expand them. Here is how it would go:

* The expansion $e$ function maps a polynomial $P(X)$ of degree $n$ to one of degree $kn$. It goes as,

$$e: P(X) \rightarrow \bar{P}(X) = P(X^k)$$

The transformation has a trivial materialization in the Lagrange domain of the roots of unity as it just consists in just repeating $k$ times the Lagrange basis coordinates of $P(X)$. Beside, the transformation is an automorphism of the ring $\mathbb{F}[X]$.

We can use this observation, to argue that traditional polynomial identity is completely preserved by the action of $e$. 

$$\mathcal{C}(X, P_1, \ldots) = (X^n - 1) Q(X) \Leftrightarrow \mathcal{C}(X^k, \bar{P_1}, \ldots) = (X^{kn} - 1) \bar{Q}(X)$$

This gives us the following protocol:

* Commit to $\bar{P_i}$ instead of $P_i$
* Compute $Q$ as we would normally do in the global constraint compiler
* Commit to $\bar{Q}$, (we explain later how to make this just as efficient as committing to $Q$).
* The evaluation claim is made over the $\bar{P}$ version of the polynomial identity.

For the commitment part, the idea is to sort the module by smallest to largest and use an incremental hash function (like any hash function).

For each module separately:
* Let $k$ be the number of repetition for that particular module
* RS-encode the rows of the module
* Hash the columns using the hash function
* Repeat $k$ times the column hashes to obtain the module's hash vector

At the end,
    * rehash the hash vectors of all modules conjointly to obtain the Merkle leaves.

The construction could be even optimized, by allowing accumulating the hash-vectors of the modules from smallest to largest.
