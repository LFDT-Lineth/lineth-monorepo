// SPDX-License-Identifier: MIT
//! The BLS12-381 optimal ate pairing: `e: G1 x G2 -> Gt`, `Gt` the
//! order-`r` cyclotomic subgroup of `Fp12*` (`fp12.zig`) — Part 2 of this
//! module's multi-part arc (`README.md`). Two stages, the textbook
//! pairing-construction split:
//!
//!   1. **Miller loop** (`multiMillerLoop`/`millerLoop`) — accumulates a
//!      Weil/Tate-style rational function evaluated along a scalar
//!      multiple of `Q` (`G2`), sampled at `P` (`G1`), via a
//!      double-and-add walk keyed to the curve's Frobenius/seed
//!      parameter. This is the "optimal ate" variant (Vercauteren 2010),
//!      the standard choice for the BLS12 curve family — its loop length
//!      is `O(log r) / phi(k)`-ish (`k=12` here), the shortest known for
//!      this embedding degree, which is WHY BLS12-381 is efficient enough
//!      to pair at all.
//!   2. **Final exponentiation** (`finalExponentiation`) — raises the
//!      Miller loop's raw output to the fixed public power `(p^12-1)/r`,
//!      which (a) makes the result independent of which representative
//!      pair of the equivalence classes ended up as inputs, and (b) lands
//!      it in the order-`r` cyclotomic subgroup `Gt` — see that
//!      function's doc comment for the standard easy/hard split.
//!
//! **Status: implemented** (Part-2 crypto-core pass, 2026-07-14).
//! `multiMillerLoop` is the real optimal-ate loop (affine D-type-twist
//! line evaluation, shared accumulator, allocation-free batching);
//! `finalExpHardPart` is the real Hayashida-Hayasaka-Teruya exact-`d`
//! cyclotomic chain (see its doc comment for why the FCKRH `3d` chain
//! was NOT used); the easy part was already real from the scaffold.
//! Verified by the bilinearity property suite below PLUS a byte-exact
//! `e(G1,G2)` KAT sourced from `ethereum/py_ecc` (see the KAT test's
//! comment and `NOTICE`). Remaining `// TODO`s are optimizations only
//! (sparse line multiplication, `pairingCheck`'s skip-the-hard-part
//! shortcut).
//!
//! Zig std GAP: yes — nothing in `std` supplies pairing-friendly curve
//! machinery; this file (and the field/group tower it sits on) is this
//! module's own. Design references: Miller, "Short Programs for
//! Functions on Curves" (1986, unpublished manuscript — the original
//! Miller-loop construction); Vercauteren, "Optimal Pairings" (IEEE
//! Trans. Info. Theory 56(1), 2010 — the "optimal ate" variant this file
//! names); Scott, Benger, Charlemagne, Perez, Kachisa, "On the Final
//! Exponentiation for Calculating Pairings on Ordinary Elliptic Curves"
//! (ePrint 2008/490 — the easy/hard split this file implements);
//! Hayashida, Hayasaka, Teruya, "Efficient Final Exponentiation via
//! Cyclotomic Structure" (ePrint 2020/875 — the exact hard-part
//! identity `finalExpHardPart` implements); Costello-Lange-Naehrig
//! (PKC 2010 — the D-type-twist line evaluation); `zkcrypto/bls12_381`
//! and `supranational/blst` remain UNREAD reference implementations
//! (per `NOTICE`); the output is pinned bit-for-bit against the IETF
//! pairing-friendly-curves draft's official optimal-ate test vector,
//! with `ethereum/py_ecc` as an independent cross-check — see the KAT
//! tests below.

const std = @import("std");
const fp12mod = @import("fp12.zig");
const g1 = @import("g1.zig");
const g2 = @import("g2.zig");

pub const Fp12 = fp12mod.Fp12;
const Fp = fp12mod.Fp;
const Fp2 = fp12mod.Fp2;
const Fp6 = fp12mod.Fp6;

/// The pairing's target group, `Gt` — the order-`r` CYCLOTOMIC SUBGROUP
/// of `Fp12*` (elements `a` with `a * conjugate(a) = 1`, equivalently
/// `a^(p^6-1) = 1`... intersected with the `p^2+1` condition — see
/// `fp12.zig`'s `Fp12.cyclotomicSquare` doc comment for the precise
/// subgroup this module's Miller-loop/final-exponentiation outputs live
/// in). CONVENTION (this module does not mint a distinct wrapper type):
/// a `Gt` element is a plain `Fp12` value that the CALLER knows came out
/// of `finalExponentiation` (and is therefore guaranteed, by
/// construction, to be in the subgroup) — same "trust the constructor,
/// not the type" shape `g1.zig`/`g2.zig` use for `G1`/`G2` membership
/// (an `Affine`/`Jacobian` value is not statically known to be in the
/// order-`r` subgroup either; `subgroupCheck` is a separate, explicit
/// call). `pairing`/`pairingCheck`'s return values are the only
/// `Gt`-typed values this module produces; `Fp12.eql` is the right
/// equality check for comparing two `Gt` elements (used throughout the
/// tests below).
pub const Gt = Fp12;

/// The BLS12-381 seed parameter's absolute value, `|x| =
/// 0xd201000000010000` (`x` itself is NEGATIVE — `x =
/// -0xd201000000010000`, the same parameter `fp.zig`/`g1.zig`'s own
/// provenance comments cite as `z`, in
/// `draft-irtf-cfrg-pairing-friendly-curves`' own notation). 64 bits,
/// LOW Hamming weight (mostly-zero bits) — this is WHY BLS12-381 has a
/// short optimal-ate Miller loop (the loop length is `O(log|x|)`, not
/// `O(log r)`); a seed with more set bits would need more
/// addition-steps. Provided as a plain `u64` for the Miller
/// loop/final-exponentiation hard part to iterate bit-by-bit; the SIGN
/// (`x < 0`) is handled separately at the end of `multiMillerLoop`
/// (conjugating the accumulator) rather than folded into this constant.
pub const bls_x_abs: u64 = 0xd201000000010000;

/// One `(P, Q) ∈ G1 x G2` input pair to a (multi-)pairing computation —
/// the shared type `multiMillerLoop`/`pairingCheck` take a slice of, so
/// a caller batching several pairings (BLS signature AGGREGATE
/// verification, KZG batch openings — this module's later parts) has one
/// clean, reusable shape rather than parallel slices or an anonymous
/// struct repeated at every call site.
pub const PairingPair = struct {
    p: g1.Affine,
    q: g2.Affine,
};

// ── Miller loop ─────────────────────────────────────────────────────────

/// The MULTI-PAIRING Miller loop: accumulates the product of the
/// (un-final-exponentiated) Miller loop values for every `(P_i, Q_i)`
/// pair in `pairs`, sharing a SINGLE seed-bit walk and a SINGLE running
/// `Fp12` accumulator `f` across all of them — this is the actual
/// algorithmic payoff of "multi"-pairing (batch/aggregate verification):
/// squaring `f` once per bit for `N` pairs costs the same as squaring it
/// once per bit for `1` pair, versus `N` independent single-pair loops
/// each paying their own `O(log|x|)` squarings. `millerLoop` (below) is
/// the trivial `N=1` specialization built on top of this function — this
/// is the ONE place the actual line-function/loop math lives.
///
/// Construction (optimal ate, BLS12 family — Vercauteren 2010 /
/// Miller 1986; every implementation of a BLS12-381 pairing follows this
/// same shape, e.g. `zkcrypto/bls12_381`'s `multi_miller_loop` and
/// `blst`'s `blst_miller_loop_n` — named as design references per
/// `NOTICE`, not read as source):
///
/// 1. For each pair `i`, initialize a running "accumulator point"
///    `T_i = Jacobian.fromAffine(pairs[i].q)` (in `E'(Fp2)`, `G2`'s
///    curve) — T-doubling drives the loop, mirroring how `x` bits are
///    walked in a scalar multiplication, except each "step" ALSO folds a
///    line-function evaluation into a shared `Fp12` accumulator `f`
///    (initialized to `Fp12.one`) instead of just producing a point.
/// 2. Walk the bits of `bls_x_abs` from the SECOND-HIGHEST bit down to
///    bit 0 (the highest bit is implicitly handled by `f`/`T_i`'s
///    initial values needing no "first squaring" — same convention as a
///    standard square-and-multiply ladder that starts its accumulator at
///    the base rather than at `one`). For each bit:
///    a. **Doubling step**, once per pair `i`: compute the TANGENT LINE
///       to `T_i` at `T_i` itself (in `E'(Fp2)`), evaluate that line at
///       `P_i` (`G1`, via the standard degree-6 twist's untwisting map —
///       BLS12-381's `G2` twist is D-TYPE, so the untwisting folds `P`'s
///       coordinates into the correct `Fp2` slots of a sparse `Fp12`
///       element; see e.g. Costello-Lange-Naehrig "Faster Pairing
///       Computations on Curves with High-Degree Twists", 2010, for the
///       D-type-twist line-evaluation formulas — a further named
///       reference alongside Vercauteren/Miller above), producing a
///       SPARSE `Fp12` value `l_i`; set `T_i <- 2*T_i` (the SAME
///       doubling this line computation already needs the intermediate
///       values for — `doublingStep` fuses the two).
///    b. `f <- f^2 * l_0 * l_1 * ... * l_{n-1}` — ONE shared squaring of
///       the accumulator per bit (the "multi" payoff described above),
///       times every pair's sparse line value from step (a). (Baseline,
///       CORRECTNESS-FIRST path: `f.square()` then `f.mul(l_i)` for each
///       `i`, using `Fp12`'s general `mul`/`square` — both real,
///       `fp12.zig`.)
///    c. **Addition step**, only on SET bits of `bls_x_abs`, once per
///       pair `i`: compute the line through `T_i` AND `Q_i` (NOT `2T_i`
///       and `Q_i` — `Q_i` itself, the FIXED original point for pair
///       `i`), evaluate at `P_i` (same untwisting as step (a)) to get
///       another sparse `l_i`; set `T_i <- T_i + Q_i`; fold `l_i` into
///       `f` the same way as step (b) (a second `f <- f * l_i` per set
///       bit, per pair — no extra squaring, squaring happens once per
///       BIT regardless of how many pairs have a set bit there).
/// 3. Because `x` (the actual BLS seed, not `bls_x_abs`) is NEGATIVE,
///    conjugate the final accumulator: `f <- f.conjugate()` (the
///    standard trick — see e.g. `blst`'s and `zkcrypto`'s BLS12-381
///    Miller loops, both of which special-case the negative seed this
///    exact way; conjugation, not full inversion, suffices because by
///    this point `f` is already in the cyclotomic-ish subgroup the
///    Miller loop's own structure guarantees — `fp12.zig`'s
///    `Fp12.conjugate` doc comment).
/// 4. Return `f` — this is the RAW, NOT-final-exponentiated product of
///    every pair's Miller loop value; `finalExponentiation` (a SEPARATE
///    call, deliberately not folded in here — see `pairingCheck`'s doc
///    comment for why sharing exactly one final exponentiation over the
///    whole product matters) turns it into an actual `Gt` element.
///
/// `// TODO`: step 2(b)'s baseline uses `Fp12.mul`/`.square` for every
/// fold — the standard optimization (every serious pairing
/// implementation applies it) is that a freshly-computed line value
/// `l_i` is SPARSE (3 of its 6 `Fp2` coefficients nonzero — the "014"
/// shape `lineEval` documents), so `f.mul(l_i)` could use a dedicated
/// sparse-multiplication routine (far cheaper than `Fp12.mul`'s general
/// 3x `Fp6.mul` Karatsuba) instead of promoting `l_i` to a dense
/// `Fp12`. Deferred follow-up optimization, not a correctness issue —
/// same "correct-simple first, fast-path later" policy `g1.zig`/
/// `g2.zig`'s `subgroupCheck`/`clearCofactor` apply to their own
/// deferred Bowe fast paths (`SPEC.md` Backlog).
///
/// Special case handled explicitly (not merely falling out of the
/// formulas above): a pair with `p.infinity` or `q.infinity` contributes
/// the trivial factor `Fp12.one` to the product (a pairing with either
/// input at infinity is defined to be `1`) — the line-evaluation
/// formulas above are not defined at infinity and are never run on such
/// a pair (`millerLoopBatch`'s `live` mask).
pub fn multiMillerLoop(pairs: []const PairingPair) Fp12 {
    // Allocation-free multi-pairing: pairs are processed in fixed-size
    // BATCHES of up to `miller_batch_max`, each batch sharing one
    // seed-bit walk and one accumulator (the doc comment's construction
    // verbatim); batch results multiply together. This is exact — the
    // shared-walk value IS the product of the per-pair Miller values
    // (squaring distributes over the product), so splitting into
    // batches only forgoes the shared-squaring saving ACROSS batches,
    // never changes the result. Real call sites (BLS aggregate verify,
    // KZG openings) use 2-3 pairs — always a single batch.
    var f = Fp12.one;
    var i: usize = 0;
    while (i < pairs.len) : (i += miller_batch_max) {
        const end = @min(i + miller_batch_max, pairs.len);
        f = f.mul(millerLoopBatch(pairs[i..end]));
    }
    // The seed x is NEGATIVE (x = -bls_x_abs): conjugate the |x|-walk's
    // accumulator (= the inverse up to factors the final exponentiation
    // kills — the standard negative-seed trick, step 3 of the doc
    // comment's construction).
    return f.conjugate();
}

/// Batch width for `multiMillerLoop`'s allocation-free shared walk —
/// sized so a batch's per-pair `G2` accumulator points live in a small
/// fixed stack array (`Fp2` x/y pairs: 8 * 192B = 1.5 KiB) while still
/// covering every realistic multi-pairing call in one batch.
const miller_batch_max = 8;

/// A mutable affine point on the sextic twist `E'(Fp2)` — the per-pair
/// accumulator `T_i` the Miller loop's doubling/addition steps advance.
/// Affine (not Jacobian) is DELIBERATE for the correctness-first
/// baseline: the chord/tangent slope `λ` the line function needs is
/// explicit in affine form, at the cost of one `Fp2.inv` per step
/// (see `doublingStep`/`additionStep` for the non-degeneracy argument
/// that makes those inversions total).
const TwistPoint = struct {
    x: Fp2,
    y: Fp2,
};

/// One shared-accumulator Miller walk over `|x|`'s bits for up to
/// `miller_batch_max` pairs — the doc-comment construction of
/// `multiMillerLoop` steps 1-2 (step 3, the negative-seed conjugation,
/// happens ONCE in `multiMillerLoop` after all batches multiply
/// together; conjugation distributes over products, so batch-local vs
/// final placement is equivalent). Pairs with an infinity component
/// are skipped entirely (contribute the factor `1` — the special case
/// the doc comment requires to be explicit).
fn millerLoopBatch(pairs: []const PairingPair) Fp12 {
    std.debug.assert(pairs.len <= miller_batch_max);
    var ts: [miller_batch_max]TwistPoint = undefined;
    var live = [_]bool{false} ** miller_batch_max;
    for (pairs, 0..) |pair, j| {
        live[j] = !(pair.p.infinity or pair.q.infinity);
        if (live[j]) ts[j] = .{ .x = pair.q.x, .y = pair.q.y };
    }

    var f = Fp12.one;
    var bit: u6 = 62; // |x|'s top bit (63) is implicit: f = 1, T = Q.
    while (true) : (bit -= 1) {
        f = f.square();
        for (pairs, 0..) |pair, j| {
            if (!live[j]) continue;
            f = f.mul(doublingStep(&ts[j], pair.p));
        }
        if ((bls_x_abs >> bit) & 1 == 1) {
            for (pairs, 0..) |pair, j| {
                if (!live[j]) continue;
                f = f.mul(additionStep(&ts[j], pair.q, pair.p));
            }
        }
        if (bit == 0) break;
    }
    return f;
}

/// Builds the (dense-`Fp12`-promoted) D-type-twist line value: the line
/// on `E'(Fp2)` with slope `lambda` through `(x_ref, y_ref)`, evaluated
/// at the TWISTED image of `p ∈ G1`.
///
/// Derivation (Costello-Lange-Naehrig 2010, specialized to this tower's
/// `w^2 = v`, `v^3 = ξ` — so `w^6 = ξ` and the D-type untwist is
/// `ψ(x', y') = (x'/w^2, y'/w^3)`): evaluating the line on the twist at
/// `P`'s twisted image `(x_P w^2, y_P w^3)` instead of untwisting `T`
/// differs from the untwisted line at `P` by the fixed factor `w^3` per
/// line — and `(w^3)^((p^12-1)/r) = 1` (`(w^3)^2 = ξ ∈ Fp2` and
/// `(w^3)^(p^6-1) = -1`, killed by the even `(p^2+1)` factor), so every
/// such factor vanishes under the final exponentiation (the RAW Miller
/// value is not a wire format — only the final-exponentiated pairing
/// is). The twisted-image evaluation gives
/// ```
/// l = (λ x_ref − y_ref) + (−λ x_P) w^2 + y_P w^3
/// ```
/// i.e. nonzero `Fp2` coefficients only at `1`, `v` (= `w^2`), and
/// `v·w` (= `w^3`) — tower slots `c0.c0`, `c0.c1`, `c1.c1` (the sparse
/// "014" shape; promoted dense here, see `multiMillerLoop`'s TODO).
/// `x_P, y_P ∈ Fp` embed as `Fp2` elements with zero `u` component, so
/// the two products with `λ` are componentwise `Fp.mul`s.
fn lineEval(lambda: Fp2, x_ref: Fp2, y_ref: Fp2, p: g1.Affine) Fp12 {
    const c00 = lambda.mul(x_ref).sub(y_ref);
    const c01 = Fp2{
        .c0 = lambda.c0.mul(p.x).neg(),
        .c1 = lambda.c1.mul(p.x).neg(),
    };
    const c11 = Fp2{ .c0 = p.y, .c1 = Fp.zero };
    return .{
        .c0 = .{ .c0 = c00, .c1 = c01, .c2 = Fp2.zero },
        .c1 = .{ .c0 = Fp2.zero, .c1 = c11, .c2 = Fp2.zero },
    };
}

/// Miller doubling step: tangent line to `T` on `E'(Fp2)` evaluated at
/// `p` (via `lineEval`), then `T <- 2T` (affine chord-tangent).
///
/// The `catch unreachable` on `(2y)^-1`: an on-twist point never has
/// `y == 0` — `#E'(Fp2) = r * h2` with both factors ODD, so the twist
/// has no 2-torsion, and every intermediate `T` stays on the twist by
/// construction. (A caller feeding a hand-crafted OFF-CURVE `q` voids
/// this — `pairing`'s inputs are on-curve by every decode path's
/// `isOnCurve` check, and subgroup membership at trust boundaries is
/// already the module-wide caller obligation, `SPEC.md`.)
fn doublingStep(t: *TwistPoint, p: g1.Affine) Fp12 {
    const xx = t.x.square();
    const three_xx = xx.add(xx).add(xx);
    const two_y = t.y.add(t.y);
    const lambda = three_xx.mul(two_y.inv() catch unreachable);
    const line = lineEval(lambda, t.x, t.y, p);
    const x3 = lambda.square().sub(t.x).sub(t.x);
    const y3 = lambda.mul(t.x.sub(x3)).sub(t.y);
    t.* = .{ .x = x3, .y = y3 };
    return line;
}

/// Miller addition step: chord line through `T` and the FIXED original
/// `q` evaluated at `p`, then `T <- T + Q`.
///
/// The `catch unreachable` on `(x_T - x_Q)^-1`: it fails only for
/// `T == ±Q`, i.e. `[m ∓ 1]Q == O` for the walk's intermediate scalar
/// `m` (`2 <= m < |x| < 2^64`). For subgroup points (`ord(Q) = r >
/// 2^254`) that is impossible; only an on-curve-but-NON-subgroup `q`
/// of small order could trigger it — the same class of input the
/// module-wide subgroup-check obligation (`SPEC.md`'s threat model)
/// already excludes at trust boundaries, and a panic (not a silently
/// wrong pairing) is the failure mode if violated.
fn additionStep(t: *TwistPoint, q: g2.Affine, p: g1.Affine) Fp12 {
    const lambda = t.y.sub(q.y).mul(t.x.sub(q.x).inv() catch unreachable);
    const line = lineEval(lambda, q.x, q.y, p);
    const x3 = lambda.square().sub(t.x).sub(q.x);
    const y3 = lambda.mul(t.x.sub(x3)).sub(t.y);
    t.* = .{ .x = x3, .y = y3 };
    return line;
}

/// The single-pair Miller loop `e`'s FIRST stage computes for `(p, q)` —
/// a thin, REAL wrapper: the `N=1` specialization of `multiMillerLoop`
/// (see that function's doc comment for the actual construction; this
/// function exists purely for API convenience/naming symmetry with
/// `pairing`, not as a second implementation site).
pub fn millerLoop(p: g1.Affine, q: g2.Affine) Fp12 {
    return multiMillerLoop(&.{.{ .p = p, .q = q }});
}

// ── final exponentiation ────────────────────────────────────────────────

/// The "easy part" of the final exponentiation: raises `f` to
/// `(p^6-1)(p^2+1)`. REAL — this is a fixed composition of already-real,
/// already-tested `Fp12` primitives (`conjugate`/`inv`/`mul`/
/// `frobenius`), not new pairing-specific math: the identity
/// `f^(p^6-1) = conjugate(f) * f^-1` is `fp12.zig`'s OWN documented
/// identity (`Fp12.conjugate`'s doc comment: `a^(p^6) = conjugate(a)` for
/// `a ∈ Fp12*`), and the subsequent `^(p^2+1)` step —
/// `c^(p^2+1) = frobenius(frobenius(c)) * c` — is exactly the
/// pairing-free cyclotomic-subgroup-membership CONSTRUCTION
/// `Fp12.cyclotomicSquare`'s own doc comment already spells out
/// verbatim as its "day-one test oracle" (`d = c.frobenius()
/// .frobenius().mul(c)`). Both identities were therefore already
/// reviewed and committed as correct by the Part-1 crypto-core pass —
/// transcribing them here is wiring, not a new formula.
///
/// `f.inv()` is called with `catch unreachable`: a well-formed Miller
/// loop output (any `f` this module's own `multiMillerLoop` can produce
/// for on-curve, non-degenerate inputs) is always a UNIT of `Fp12*` —
/// never zero — by construction of the pairing (the only way to get `0`
/// would be a malformed line-evaluation denominator, which the
/// `multiMillerLoop` construction's infinity special-case is required to
/// guard against). If the Miller loop implementation could somehow
/// produce a zero value for some input, that would be a bug in THAT
/// implementation, not a case this easy part needs to handle gracefully
/// (mirrors `g1.zig`/`g2.zig`'s own `toAffine`'s `catch unreachable` on
/// `self.z.inv()`, guarded by the caller-side `isIdentity` check instead
/// of a runtime error path).
fn finalExpEasyPart(f: Fp12) Fp12 {
    // f^(p^6-1) = conjugate(f) * f^-1 (Fp12.conjugate's own doc comment).
    const c = f.conjugate().mul(f.inv() catch unreachable);
    // c^(p^2+1) = frobenius^2(c) * c (Fp12.cyclotomicSquare's own doc
    // comment gives this exact construction as its pairing-free oracle).
    return c.frobenius().frobenius().mul(c);
}

/// The "hard part" of the final exponentiation: raises the easy part's
/// output (now guaranteed to be in the order-`(p^4-p^2+1)` cyclotomic
/// subgroup) to the further power `d = (p^4-p^2+1)/r`, landing the
/// FINAL result in the order-`r` subgroup `Gt`. REAL (Part-2
/// crypto-core pass).
///
/// Construction — the EXACT-`d` cyclotomic-structure identity of
/// Hayashida, Hayasaka, Teruya, "Efficient Final Exponentiation via
/// Cyclotomic Structure for Pairings over Families of Elliptic Curves"
/// (ePrint 2020/875), BLS12 case (within the Scott et al. ePrint
/// 2008/490 easy/hard framework; chosen over the Fuentes-Castañeda-
/// Knapp-Rodríguez-Henríquez ePrint 2011/465 chain the scaffold also
/// named because FCKRH computes `f^(3d)` — a fixed CUBE of the
/// canonical pairing, bilinearity-equivalent but NOT byte-equal to the
/// `e(G1,G2)` reference value the KAT test below pins):
/// ```
/// d = k * (x + p) * (x^2 + p^2 - 1) + 1,   k = (x - 1)^2 / 3
/// ```
/// with `x` the (negative) BLS seed. The identity was re-verified
/// NUMERICALLY for this exact seed with independent big-integer
/// arithmetic (exact integer equality against `(p^4-p^2+1)/r` — see
/// `NOTICE`), and `k` is derived at COMPTIME from `bls_x_abs`
/// (`hard_part_k_bytes` — no transcribed constant). Evaluated
/// inside-out on cyclotomic elements:
/// ```
/// m1 = f^k                                  (cyclotomicPow, 126-bit k)
/// m2 = m1^x * m1^p                          (seed exp + frobeniusMap(1))
/// m3 = m2^(x^2) * m2^(p^2) * m2^-1          (seed exp twice + frobeniusMap(2) + conjugate)
/// return m3 * f                             (the trailing +1)
/// ```
/// Conjugation-as-inversion is valid throughout: every intermediate is
/// a power of the cyclotomic input, and `p^6 ≡ -1 (mod p^4-p^2+1)`
/// makes `conjugate` (`= ^p^6`) the exact inverse on that subgroup.
/// All squarings inside the exponentiations use
/// `Fp12.cyclotomicSquare` (see `cyclotomicPow`).
fn finalExpHardPart(f: Fp12) Fp12 {
    const m1 = cyclotomicPow(f, &hard_part_k_bytes);
    const m2 = cyclotomicExpSeed(m1).mul(m1.frobeniusMap(1));
    const m3 = cyclotomicExpSeed(cyclotomicExpSeed(m2))
        .mul(m2.frobeniusMap(2))
        .mul(m2.conjugate());
    return m3.mul(f);
}

/// `k = (x-1)^2 / 3` (`x` the NEGATIVE seed, so `(x-1)^2 =
/// (bls_x_abs+1)^2`), big-endian — the hard part's leading factor,
/// derived at comptime from `bls_x_abs` (division-by-3 exactness
/// enforced at comptime; same no-transcribed-constants policy as
/// `fp.pExponentBytes`). 126 bits -> 16 bytes.
const hard_part_k_bytes: [16]u8 = blk: {
    const zm1: comptime_int = @as(comptime_int, bls_x_abs) + 1; // |x - 1|
    const sq = zm1 * zm1;
    if (sq % 3 != 0) @compileError("bls12_381: (x-1)^2 not divisible by 3");
    var q = sq / 3;
    var out: [16]u8 = undefined;
    var i: usize = 16;
    while (i > 0) {
        i -= 1;
        out[i] = q % 256;
        q = q / 256;
    }
    if (q != 0) @compileError("bls12_381: hard-part k does not fit in 16 bytes");
    break :blk out;
};

/// `f^e` for `f` in the CYCLOTOMIC subgroup, `e` a big-endian PUBLIC
/// byte string — square-and-multiply with `Fp12.cyclotomicSquare` for
/// every doubling (the whole point of the hard part being fast; the
/// subgroup precondition is exactly `cyclotomicSquare`'s own).
/// Variable-time in the exponent bits: every caller's exponent is a
/// fixed public curve constant (`k`, `|x|`), same policy as `Fp2.pow`.
fn cyclotomicPow(f: Fp12, e: []const u8) Fp12 {
    var acc = Fp12.one;
    for (e) |byte| {
        var bit: u3 = 7;
        while (true) : (bit -= 1) {
            acc = acc.cyclotomicSquare();
            if ((byte >> bit) & 1 == 1) acc = acc.mul(f);
            if (bit == 0) break;
        }
    }
    return acc;
}

/// `f^x` for `f` in the cyclotomic subgroup, `x` the NEGATIVE BLS seed:
/// `f^|x|` (`cyclotomicPow` over `bls_x_abs`) then conjugate (= invert,
/// on this subgroup — see `finalExpHardPart`'s doc comment).
fn cyclotomicExpSeed(f: Fp12) Fp12 {
    var e: [8]u8 = undefined;
    std.mem.writeInt(u64, &e, bls_x_abs, .big);
    return cyclotomicPow(f, &e).conjugate();
}

/// The full final exponentiation `f -> f^((p^12-1)/r)`: the easy part
/// composed with the hard part (both above, both real).
pub fn finalExponentiation(f: Fp12) Fp12 {
    return finalExpHardPart(finalExpEasyPart(f));
}

// ── the pairing, and its multi/batch form ───────────────────────────────

/// The BLS12-381 optimal ate pairing `e(p, q)`: `finalExponentiation`
/// composed with `millerLoop`. REAL wiring (both stages are separately
/// documented above; this function is the textbook composition, nothing
/// more) — the module's primary single-pair entry point.
pub fn pairing(p: g1.Affine, q: g2.Affine) Fp12 {
    return finalExponentiation(millerLoop(p, q));
}

/// The batch/aggregate pairing-product check every BLS-signature
/// aggregate-verification and KZG batch-opening scheme (this module's
/// later parts, `README.md`) actually wants: `true` iff `∏ e(p_i, q_i) ==
/// 1` over every pair in `pairs` — WITHOUT ever materializing the
/// individual `e(p_i, q_i)` values or paying for more than ONE final
/// exponentiation. REAL wiring: `multiMillerLoop` already accumulates the
/// product of every pair's RAW (pre-final-exponentiation) Miller loop
/// value into a single `Fp12` (see that function's doc comment); because
/// `finalExponentiation` is itself just raising to a fixed public power —
/// a group homomorphism on `Fp12*` — `finalExponentiation(∏ miller_i) ==
/// ∏ finalExponentiation(miller_i) == ∏ e(p_i, q_i)`, so ONE
/// `finalExponentiation` call over the accumulated product is
/// mathematically equivalent to (but far cheaper than) `N` separate ones.
/// This is precisely why real-world BLS signature aggregation is
/// affordable: verifying `N` aggregated signatures costs one
/// multi-Miller-loop + one final exponentiation, not `N` of each.
///
/// `// TODO`: some production implementations (e.g. `blst`) skip
/// running the FULL final exponentiation for a pairing-product CHECK
/// specifically (only the easy part, plus a cheaper subgroup-membership
/// test on the un-hard-part-exponentiated value, can sometimes suffice
/// to detect "not equal to 1" without paying for the hard part) — a
/// possible follow-up optimization, not required for correctness; this
/// `pairingCheck` deliberately takes the simple, obviously-correct
/// "run the real `finalExponentiation`, then compare to `one`" path
/// (`SPEC.md` Backlog).
pub fn pairingCheck(pairs: []const PairingPair) bool {
    return finalExponentiation(multiMillerLoop(pairs)).eql(Fp12.one);
}

// ── tests ────────────────────────────────────────────────────────────────
//
// Two-layer verification net (the "property tests as the strong oracle,
// byte-exact KAT layered on top" pattern from `adaptor`/`spake2plus`):
//
// 1. The BILINEARITY property suite — needs no external constant at
//    all; an implementation satisfying non-degeneracy, bilinearity in
//    both arguments, the `pairingCheck` identities, AND the `f^r == 1`
//    subgroup-membership check simultaneously is overwhelmingly likely
//    to be THE pairing, not merely "a" self-consistent one.
// 2. A byte-exact `e(G1, G2)` KAT from TWO independent external
//    sources, per this module's established
//    fetch-and-independently-recompute policy: the IETF
//    draft-irtf-cfrg-pairing-friendly-curves-11 Appendix B optimal-ate
//    test vector (primary — matches this module's output byte-for-byte)
//    and `ethereum/py_ecc` 8.0.0 (cross-check — matches up to exactly
//    one conjugation, the two implementations' documented
//    negative-seed-convention difference). See the KAT constants'
//    comment below for the full sourcing/conversion story and
//    `NOTICE`'s "Verification performed (Part-2 crypto-core pass)".

fn g1Gen() g1.Jacobian {
    return g1.Jacobian.fromAffine(g1.Affine.generator);
}

fn g2Gen() g2.Jacobian {
    return g2.Jacobian.fromAffine(g2.Affine.generator);
}

/// Small-exponent `Fp12` power, `base^n` for a small COMPTIME `n` — a
/// plain repeated-multiplication test helper (no need for a general
/// square-and-multiply ladder at these sizes), mirroring `fp12.zig`'s own
/// test-local `pow12` helper's role but scoped to this file's tests.
fn powSmall(base: Fp12, comptime n: usize) Fp12 {
    var acc = Fp12.one;
    for (0..n) |_| acc = acc.mul(base);
    return acc;
}

/// General big-exponent `Fp12` power (square-and-multiply, VARIABLE-TIME
/// in the exponent — fine here, `e` is always a PUBLIC test constant like
/// `Fr`'s modulus `r`) — used only by the `f^r == 1` subgroup-membership
/// sanity test below. Same shape as `fp12.zig`'s test-local `pow12`.
fn powBig(a: Fp12, e: []const u8) Fp12 {
    var acc = Fp12.one;
    for (e) |byte| {
        var bit: u3 = 7;
        while (true) : (bit -= 1) {
            acc = acc.square();
            if ((byte >> bit) & 1 == 1) acc = acc.mul(a);
            if (bit == 0) break;
        }
    }
    return acc;
}

test "Gt is Fp12 (documented convention, not a distinct type)" {
    try std.testing.expectEqual(Fp12, Gt);
}

test "PairingPair is the (G1.Affine, G2.Affine) pair pairingCheck/multiMillerLoop share" {
    const pair: PairingPair = .{ .p = g1.Affine.generator, .q = g2.Affine.generator };
    try std.testing.expect(pair.p.x.eql(g1.Affine.generator.x));
    try std.testing.expect(pair.q.x.eql(g2.Affine.generator.x));
}

test "bls_x_abs matches the documented BLS12-381 seed's absolute value" {
    try std.testing.expectEqual(@as(u64, 0xd201000000010000), bls_x_abs);
}

test "millerLoop(p, q) is the N=1 specialization of multiMillerLoop (same value)" {
    const p = g1.Affine.generator;
    const q = g2.Affine.generator;
    try std.testing.expect(millerLoop(p, q).eql(multiMillerLoop(&.{.{ .p = p, .q = q }})));
}

test "pairing non-degeneracy: e(G1, G2) != 1" {
    const e = pairing(g1.Affine.generator, g2.Affine.generator);
    try std.testing.expect(!e.eql(Fp12.one));
}

test "pairing bilinearity: e([a]P, [b]Q) == e(P,Q)^(a*b) for small a=3, b=5" {
    const p3 = g1Gen().double().add(g1Gen()).toAffine(); // [3]G1
    const q5 = g2Gen().double().double().add(g2Gen()).toAffine(); // [5]G2
    const lhs = pairing(p3, q5);
    const base = pairing(g1.Affine.generator, g2.Affine.generator);
    const rhs = powSmall(base, 15); // a*b = 15
    try std.testing.expect(lhs.eql(rhs));
}

test "pairing additivity in G1: e(P1+P2, Q) == e(P1,Q) * e(P2,Q)" {
    const p1 = g1.Affine.generator;
    const p2 = g1Gen().double().toAffine(); // [2]G1
    const sum = g1Gen().add(g1Gen().double()).toAffine(); // P1 + P2
    const q = g2.Affine.generator;

    const lhs = pairing(sum, q);
    const rhs = pairing(p1, q).mul(pairing(p2, q));
    try std.testing.expect(lhs.eql(rhs));
}

test "pairing additivity in G2: e(P, Q1+Q2) == e(P,Q1) * e(P,Q2)" {
    const p = g1.Affine.generator;
    const q1 = g2.Affine.generator;
    const q2 = g2Gen().double().toAffine(); // [2]G2
    const sum = g2Gen().add(g2Gen().double()).toAffine(); // Q1 + Q2

    const lhs = pairing(p, sum);
    const rhs = pairing(p, q1).mul(pairing(p, q2));
    try std.testing.expect(lhs.eql(rhs));
}

test "pairingCheck identity: e(P,Q) * e(-P,Q) == 1" {
    const p = g1.Affine.generator;
    const neg_p = g1Gen().negate().toAffine();
    const q = g2.Affine.generator;

    try std.testing.expect(pairingCheck(&.{
        .{ .p = p, .q = q },
        .{ .p = neg_p, .q = q },
    }));
}

test "pairingCheck rejects a non-trivial product: e(P,Q) alone is not 1" {
    const p = g1.Affine.generator;
    const q = g2.Affine.generator;
    try std.testing.expect(!pairingCheck(&.{.{ .p = p, .q = q }}));
}

test "final exponentiation lands in the order-r subgroup: e(G1,G2)^r == 1" {
    const e = pairing(g1.Affine.generator, g2.Affine.generator);
    const scalar_mod = @import("scalar.zig");
    try std.testing.expect(powBig(e, &scalar_mod.r_bytes).eql(Fp12.one));
}

test "pairing symmetry: e([a]P, Q) == e(P, [a]Q) for a = 7" {
    // [7]G via double-and-add on each side's own group arithmetic.
    const p7 = g1Gen().double().double().double().add(g1Gen().negate()).toAffine(); // [8-1]G1
    const q7 = g2Gen().double().double().double().add(g2Gen().negate()).toAffine(); // [8-1]G2
    const lhs = pairing(p7, g2.Affine.generator);
    const rhs = pairing(g1.Affine.generator, q7);
    try std.testing.expect(lhs.eql(rhs));
    // ... and both equal e(G1,G2)^7.
    const base = pairing(g1.Affine.generator, g2.Affine.generator);
    try std.testing.expect(lhs.eql(powSmall(base, 7)));
}

test "pairingCheck accepts a 3-pair product that multiplies to 1" {
    // e(P,Q) * e(P,Q) * e([-2]P,Q) == e([2]P,Q) * e([2]P,Q)^-1 == 1.
    const p = g1.Affine.generator;
    const q = g2.Affine.generator;
    const neg_2p = g1Gen().double().negate().toAffine();
    try std.testing.expect(pairingCheck(&.{
        .{ .p = p, .q = q },
        .{ .p = p, .q = q },
        .{ .p = neg_2p, .q = q },
    }));
}

test "pairing with either input at infinity is 1 (and skips the line math)" {
    const e1 = pairing(g1.Affine.identity, g2.Affine.generator);
    const e2 = pairing(g1.Affine.generator, g2.Affine.identity);
    try std.testing.expect(e1.eql(Fp12.one));
    try std.testing.expect(e2.eql(Fp12.one));
    // An infinity pair contributes a trivial factor in a multi-pairing.
    try std.testing.expect(pairingCheck(&.{
        .{ .p = g1.Affine.generator, .q = g2.Affine.identity },
        .{ .p = g1.Affine.identity, .q = g2.Affine.generator },
    }));
}

fn hexBytes(comptime n: usize, comptime hex: *const [2 * n:0]u8) [n]u8 {
    @setEvalBranchQuota(1_000_000);
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

// KAT: e(G1.generator, G2.generator), byte-exact in this module's own
// Fp12.toBytes wire order (c1||c0, high-to-low through the tower).
// PRIMARY SOURCE (fetched 2026-07-14):
// draft-irtf-cfrg-pairing-friendly-curves-11, Appendix B, the
// BLS12_381 optimal-ate test vector — its twelve flat-basis GF(p)
// coefficients `e_0..e_11` (coefficient of `w^j`'s Fp2 = `(e_2j,
// e_2j+1)`) reordered mechanically into this module's tower/wire
// layout: wire = e_11 e_10 e_9 e_8 e_7 e_6 (the c1 half, high slots
// first) || e_5 e_4 e_3 e_2 e_1 e_0 (the c0 half). The draft's input
// points were confirmed to be this module's own G1/G2 generators.
// CROSS-CHECK: `ethereum/py_ecc` 8.0.0's
// `py_ecc.bls12_381.pairing(G2, G1)` (independently computed, its flat
// `Fp[w]/(w^12-2w^6+2)` output converted into this tower via
// `u = w^6 - 1`, the conversion first validated as a ring isomorphism
// on random products) agrees with the draft value up to EXACTLY one
// conjugation — py_ecc's Miller loop walks `|x|` WITHOUT the
// negative-seed conjugation (verified by reading its
// `bls12_381_pairing.py`), so its output is the canonical value's
// conjugate (= Gt inverse); the c0 halves are byte-identical and the
// c1 halves exactly negate, which the second test below pins. See
// `NOTICE`'s "Verification performed (Part-2 crypto-core pass)".
const e_g1_g2_hex =
    "1454814f3085f0e6602247671bc408bbce2007201536818c901dbd4d2095dd86c1ec8b888e59611f60a301af7776be3d" ++ // e_11
    "10900338a92ed0b47af211636f7cfdec717b7ee43900eee9b5fc24f0000c5874d4801372db478987691c566a8c474978" ++ // e_10
    "0fe63f185f56dd29150fc498bbeea78969e7e783043620db33f75a05a0a2ce5c442beaff9da195ff15164c00ab66bdde" ++ // e_9
    "0e61c752414ca5dfd258e9606bac08daec29b3e2c57062669556954fb227d3f1260eedf25446a086b0844bcd43646c10" ++ // e_8
    "08890726743a1f94a8193a166800b7787744a8ad8e2f9365db76863e894b7a11d83f90d873567e9d645ccf725b32d26f" ++ // e_7
    "01ecfcf31c86257ab00b4709c33f1c9c4e007659dd5ffc4a735192167ce197058cfb4c94225e7f1b6c26ad9ba68f63bc" ++ // e_6
    "111061f398efc2a97ff825b04d21089e24fd8b93a47e41e60eae7e9b2a38d54fa4dedced0811c34ce528781ab9e929c7" ++ // e_5
    "09c92cf02f3cd3d2f9d34bc44eee0dd50314ed44ca5d30ce6a9ec0539be7a86b121edc61839ccc908c4bdde256cd6048" ++ // e_4
    "16deedaa683124fe7260085184d88f7d036b86f53bb5b7f1fc5e248814782065413e7d958d17960109ea006b2afdeb5f" ++ // e_3
    "095668fb4a02fe930ed44767834c915b283b1c6ca98c047bd4c272e9ac3f3ba6ff0b05a93e59c71fba77bce995f04692" ++ // e_2
    "153ce14a76a53e205ba8f275ef1137c56a566f638b52d34ba3bf3bf22f277d70f76316218c0dfd583a394b8448d2be7f" ++ // e_1
    "11619b45f61edfe3b47a15fac19442526ff489dcda25e59121d9931438907dfd448299a87dde3a649bdba96e84d54558"; //   e_0

// The py_ecc 8.0.0 cross-check value (see the KAT comment above): the
// canonical vector's CONJUGATE — c0 half identical, c1 half negated.
const e_g1_g2_pyecc_hex =
    "05ac909b08f9f5b3eaf9604f2787a41b96574464de4e9132d7131553d61b189d5cbf747622fa9ee0595bfe508888ec6e" ++
    "09710eb1905115e5d0299652d3ceaeeaf2fbcca0ba8423d5b134adb0f6a49daf4a2bec8bd60c767850e2a99573b86133" ++
    "0a1ad2d1da290971360be31d875d054dfa8f6401ef4ef1e43339789b560e27c7da8014ff13b26a00a4e8b3ff5498eccd" ++
    "0b9f4a97f83340ba78c2be55d79fa3fc784d97a22e14b058d1da3d5144892232f89d120c5d0d5f79097ab432bc9b3e9b" ++
    "11780ac3c545c705a3026d9fdb4af55eed32a2d765557f598bba4c626d657c12466c6f263dfd816255a2308da4ccd83c" ++
    "181414f71cf9c11f9b1060ac800c903b1676d52b16251674f3df408a79cf5f1e91b0b36a8ef580e44dd85264597046ef" ++
    "111061f398efc2a97ff825b04d21089e24fd8b93a47e41e60eae7e9b2a38d54fa4dedced0811c34ce528781ab9e929c7" ++
    "09c92cf02f3cd3d2f9d34bc44eee0dd50314ed44ca5d30ce6a9ec0539be7a86b121edc61839ccc908c4bdde256cd6048" ++
    "16deedaa683124fe7260085184d88f7d036b86f53bb5b7f1fc5e248814782065413e7d958d17960109ea006b2afdeb5f" ++
    "095668fb4a02fe930ed44767834c915b283b1c6ca98c047bd4c272e9ac3f3ba6ff0b05a93e59c71fba77bce995f04692" ++
    "153ce14a76a53e205ba8f275ef1137c56a566f638b52d34ba3bf3bf22f277d70f76316218c0dfd583a394b8448d2be7f" ++
    "11619b45f61edfe3b47a15fac19442526ff489dcda25e59121d9931438907dfd448299a87dde3a649bdba96e84d54558";

test "KAT: pairing(G1, G2) matches the IETF draft's optimal-ate test vector" {
    const expected = hexBytes(Fp12.encoded_bytes, e_g1_g2_hex);
    const e = pairing(g1.Affine.generator, g2.Affine.generator);
    try std.testing.expectEqualSlices(u8, &expected, &e.toBytes());
}

test "KAT cross-check: py_ecc's value is exactly the conjugate (negative-seed convention)" {
    const expected = hexBytes(Fp12.encoded_bytes, e_g1_g2_pyecc_hex);
    const e = pairing(g1.Affine.generator, g2.Affine.generator);
    try std.testing.expectEqualSlices(u8, &expected, &e.conjugate().toBytes());
}
