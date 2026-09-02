// SPDX-License-Identifier: MIT
//! threshold — Part 6 of the `bls12_381` arc: **trusted-dealer**
//! threshold BLS, min-pk ciphersuite (public keys/shares live in `G1`,
//! signatures/partial signatures live in `G2` — the SAME convention as
//! Part 4's `bls_sig.zig`, whose DSTs (`bls_sig.dst_sig`) and
//! `SecretKey`/`PublicKey`/`Signature` wire types this file reuses
//! directly rather than reinventing). A `t`-of-`n` Shamir secret
//! sharing of a BLS secret key `sk`, with a Feldman verifiable-secret-
//! sharing (VSS) commitment so every share is publicly checkable
//! against the dealer's claimed sharing polynomial, and a Lagrange-
//! interpolation-in-the-exponent combiner that turns any `t` (or more)
//! partial signatures into a signature **indistinguishable from**
//! (byte-for-byte equal to) `bls_sig.sign(sk, msg)`.
//!
//! This is what makes threshold BLS uniquely simple among threshold
//! signature schemes: unlike threshold Schnorr (the sibling `frost`
//! module, RFC 9591), there is no interactive nonce-commitment round at
//! all. `bls_sig.sign`'s equation `R = SK * Q` is LINEAR in `SK`, so
//! (a) any Shamir-shared piece of `SK` produces a correspondingly-
//! shared piece of `R` with NO coordination between signers (each
//! signer just calls `bls_sig.sign` with their own share, once, ever —
//! `partialSign`), and (b) Lagrange interpolation IN THE EXPONENT
//! recombines those pieces exactly as it would recombine the secret key
//! itself (`combineSignatures`). `frost`'s two-round `commit`/`sign`/
//! `aggregate` protocol (`SigningNonces`, binding factors, a sorted
//! `commitment_list`) has no counterpart here — there is nothing to
//! bind or coordinate. Types and naming mirror `frost` wherever the
//! shape genuinely matches (a trusted-dealer split step returning
//! shares + a VSS commitment, a Lagrange-combine step, the "index/
//! identifier as evaluation point" convention), adjusted for BLS's
//! simpler non-interactive structure — see each function's doc comment
//! for the specific `frost` counterpart it mirrors.
//!
//! **Trusted-dealer model ONLY** (mirrors `frost.trustedDealerKeygen`'s
//! own scope note, RFC 9591 Appendix C): a single party (the "dealer")
//! holds the plaintext `sk`, splits it into `n` Shamir shares via
//! `splitSecretKey`, and hands one to each participant out-of-band — the
//! dealer sees, and could retain, the whole secret. A full
//! Pedersen-style **distributed key generation (DKG)**, where no single
//! party ever learns `sk`, is explicitly **OUT OF SCOPE** for this file
//! (see `SPEC.md`'s "Part 6 design" / threat model) — a DKG needs an
//! interactive complaint/justification sub-protocol (Pedersen's or
//! Gennaro–Jarecki–Krawczyk–Rabin's construction) between ALL
//! participants, not merely a different `splitSecretKey` signature, and
//! would be a distinct follow-up module/part.
//!
//! **Status: COMPLETE (crypto-core pass 2026-07-14).** Every function
//! is real: the Shamir polynomial evaluation (`evalPolynomialAt`),
//! Feldman commitment computation (`feldmanCommitCoefficient`), Feldman
//! evaluation-in-the-exponent (`derivePublicKeyShare`), and Lagrange
//! interpolation-in-the-exponent (`combineSignatures`) cores were
//! filled in per the constructions each doc comment quotes, mirroring
//! the sibling `frost` module's already-KAT-validated Horner/Lagrange
//! loops (RFC 9591 Appendix C) with the field/group types swapped. The
//! keystone self-consistency chain — `splitSecretKey` → `partialSign`
//! (`t` distinct shares) → `combineSignatures` byte-for-byte equal to
//! `bls_sig.sign(sk, msg)` — transitively pins this file to Part 4's
//! ethereum/bls12-381-tests vectors (see the tests at the bottom).
//! `partialSign`, `verifyPartialSignature`, and `groupPublicKey` are
//! thin, judgment-free wrappers over Part 4's `bls_sig.sign`/
//! `bls_sig.verify` (see their own doc comments for why no new crypto
//! judgment is needed).
//!
//! **Const-time discipline**: `SecretKeyShare.scalar` is exactly as
//! sensitive as Part 4's `SecretKey.scalar` — `partialSign` reuses Part
//! 4's already-constant-time `bls_sig.sign` for precisely this reason.
//! The polynomial-evaluation (`evalPolynomialAt`) and
//! Feldman-commitment (`feldmanCommitCoefficient`) paths touch secret
//! coefficients/shares directly and follow the same discipline as
//! `bls_sig.skToPk`/`sign` (`scalar.zig`'s constant-time `Fr` ops;
//! `g1.zig`'s constant-time `scalarMul`) — see `SPEC.md`'s threat
//! model. `combineSignatures`'s Lagrange coefficients
//! are computed from PUBLIC participant indices only (same reasoning as
//! `frost.deriveInterpolatingValue`'s own doc comment: the indices
//! themselves carry no secret, only the values they multiply do), so
//! that arithmetic may use `Fr.inv`'s existing constant-time
//! implementation without further judgment; the per-partial `g2`
//! scalar multiplication by a Lagrange coefficient does NOT need to be
//! constant-time with respect to the (public) coefficient, but the
//! ACCUMULATION touches signature material that is not itself secret
//! post-hoc (a valid signature is meant to be published) — no special
//! discipline beyond "use `g2.zig`'s existing operations" is required
//! there.

const std = @import("std");
const g1 = @import("g1.zig");
const g2 = @import("g2.zig");
const scalarmod = @import("scalar.zig");
const bls_sig = @import("bls_sig.zig");

pub const Fr = scalarmod.Fr;

pub const ThresholdError = error{
    /// `splitSecretKey`'s own "invalid parameters" cases: `t == 0`,
    /// `n == 0`, `t > n`, or `coefficients.len != t - 1` — mirrors
    /// `frost.TrustedDealerKeygenError.InvalidParameters` (RFC 9591
    /// Appendix C's identical precondition on `secret_share_shard`).
    InvalidParameters,
    /// A participant index was `0` (wire-decoded by any `fromBytes`, or
    /// carried by an in-memory-constructed partial handed to
    /// `combineSignatures`). Shamir/Feldman indices are evaluation
    /// points `x_i`; `x = 0` is reserved for the SECRET itself
    /// (`f(0) = sk`) and is never a valid participant index — mirrors
    /// RFC 9591's own `NonZeroScalar` `Identifier` convention
    /// (`frost.Identifier.fromU16`/`fromBytes`, both of which reject 0).
    ZeroIndex,
    /// A serialized value's length or count field did not match the
    /// expected wire shape (`VerificationVector.fromBytesAlloc`).
    InvalidEncoding,
    /// `combineSignatures` was given fewer than `t` partial signatures
    /// (or `t == 0`) — Lagrange interpolation cannot determine a unique
    /// degree-`(t-1)` polynomial from fewer than `t` points.
    InsufficientShares,
    /// `combineSignatures` was given two or more partials carrying the
    /// SAME index — Lagrange interpolation is undefined on a repeated
    /// node, and this is almost always a caller bug (the same partial
    /// submitted twice, or two distinct signers accidentally dealt the
    /// same share index).
    DuplicateIndex,
} || std.mem.Allocator.Error || g1.G1Error || g2.G2Error || scalarmod.FrError;

// ── wire types ───────────────────────────────────────────────────────

/// One participant's Shamir share of a BLS secret key: `index` is the
/// polynomial evaluation point `x_i` (`1 <= index`, conventionally
/// `1..=n` for an `n`-participant dealing — see `splitSecretKey`),
/// `scalar` is `f(index)` for the dealer's sharing polynomial `f`
/// (`f(0) = sk`). SECRET: `scalar` must be handled with the same care
/// as a `bls_sig.SecretKey.scalar` (never logged, zeroed after use by a
/// careful caller, etc — this module does not itself add any
/// additional handling beyond what `Fr` already provides).
pub const SecretKeyShare = struct {
    index: u32,
    scalar: Fr,

    pub const encoded_bytes = 4 + Fr.encoded_bytes; // 36

    /// REAL: `index` big-endian (4 bytes) `||` `Fr.toBytes()` (32
    /// bytes) — mechanical concatenation over already-real codecs.
    pub fn toBytes(self: SecretKeyShare) [encoded_bytes]u8 {
        var out: [encoded_bytes]u8 = undefined;
        std.mem.writeInt(u32, out[0..4], self.index, .big);
        out[4..encoded_bytes].* = self.scalar.toBytes();
        return out;
    }

    /// REAL. Rejects `index == 0` (see `ThresholdError.ZeroIndex`) and
    /// a non-canonical scalar (`Fr.fromBytes`'s own `>= r` rejection).
    pub fn fromBytes(bytes: [encoded_bytes]u8) ThresholdError!SecretKeyShare {
        const index = std.mem.readInt(u32, bytes[0..4], .big);
        if (index == 0) return error.ZeroIndex;
        const scalar = try Fr.fromBytes(bytes[4..encoded_bytes].*);
        return .{ .index = index, .scalar = scalar };
    }

    /// Zeroize the secret `scalar` field in place; `index` is a public
    /// share identifier (not secret) and is left untouched. Idempotent.
    /// Hygiene only — no effect on any public share/signature already
    /// derived from it.
    pub fn deinit(self: *SecretKeyShare) void {
        std.crypto.secureZero(u8, std.mem.asBytes(&self.scalar));
    }
};

/// Participant `index`'s PUBLIC verifying share `[share_i]G1` (a `G1`
/// point, same convention as `bls_sig.PublicKey` — min-pk). Obtained
/// either by evaluating a dealer's `VerificationVector` at `index`
/// (`derivePublicKeyShare`, without ever seeing `share_i` itself — the
/// VSS-consistency property) or, degenerately, by a participant
/// computing `bls_sig.skToPk` on their own `SecretKeyShare` directly.
pub const PublicKeyShare = struct {
    index: u32,
    point: g1.Affine,

    pub const encoded_bytes = 4 + g1.compressed_bytes; // 52

    /// REAL: `index` big-endian `||` `g1.toBytesCompressed`.
    pub fn toBytes(self: PublicKeyShare) [encoded_bytes]u8 {
        var out: [encoded_bytes]u8 = undefined;
        std.mem.writeInt(u32, out[0..4], self.index, .big);
        out[4..encoded_bytes].* = g1.toBytesCompressed(self.point);
        return out;
    }

    /// REAL. Rejects `index == 0`; does NOT subgroup-check `point` (the
    /// same convention as `bls_sig.PublicKey.fromBytes` — callers
    /// crossing a trust boundary must additionally call
    /// `bls_sig.keyValidate` on the unwrapped point, or rely on
    /// `verifyPartialSignature`'s own fail-closed checks, which run
    /// `bls_sig.verify`'s mandatory `keyValidate`/subgroup checks).
    pub fn fromBytes(bytes: [encoded_bytes]u8) ThresholdError!PublicKeyShare {
        const index = std.mem.readInt(u32, bytes[0..4], .big);
        if (index == 0) return error.ZeroIndex;
        const point = try g1.fromBytesCompressed(bytes[4..encoded_bytes].*);
        return .{ .index = index, .point = point };
    }
};

/// Participant `index`'s partial signature: `[share_i]Q` for
/// `Q = hashToCurveG2(msg, bls_sig.dst_sig)` (a `G2` point, `bls_sig.
/// Signature`'s own shape — min-pk). Produced by `partialSign`,
/// consumed by `verifyPartialSignature` and `combineSignatures`.
pub const PartialSignature = struct {
    index: u32,
    point: g2.Affine,

    pub const encoded_bytes = 4 + g2.compressed_bytes; // 100

    /// REAL: `index` big-endian `||` `g2.toBytesCompressed`.
    pub fn toBytes(self: PartialSignature) [encoded_bytes]u8 {
        var out: [encoded_bytes]u8 = undefined;
        std.mem.writeInt(u32, out[0..4], self.index, .big);
        out[4..encoded_bytes].* = g2.toBytesCompressed(self.point);
        return out;
    }

    /// REAL. Rejects `index == 0`; does NOT subgroup-check `point` —
    /// same convention as `bls_sig.Signature.fromBytes`
    /// (`verifyPartialSignature`/`combineSignatures`'s eventual
    /// implementation are the trust-boundary checkpoints, exactly as
    /// `bls_sig.verify` is for an ordinary `Signature`).
    pub fn fromBytes(bytes: [encoded_bytes]u8) ThresholdError!PartialSignature {
        const index = std.mem.readInt(u32, bytes[0..4], .big);
        if (index == 0) return error.ZeroIndex;
        const point = try g2.fromBytesCompressed(bytes[4..encoded_bytes].*);
        return .{ .index = index, .point = point };
    }
};

/// A Feldman VSS commitment to the dealer's degree-`(t-1)` sharing
/// polynomial `f(x) = sk + a_1*x + a_2*x^2 + ... + a_{t-1}*x^{t-1}`:
/// `commitments[j] = [a_j]G1` for `j` in `0..t` (`a_0 := sk`), so
/// `commitments[0] == [sk]G1` (`groupPublicKey`) and
/// `commitments.len == t` (`threshold()`). PUBLIC — this is the
/// information the dealer broadcasts alongside the (secret,
/// out-of-band) per-participant shares, letting every participant
/// verify their own share (and derive anyone else's PUBLIC key share)
/// without trusting the dealer any further. Owned slice: free
/// `commitments` with the same allocator that produced it
/// (`splitSecretKey`/`fromBytesAlloc`).
pub const VerificationVector = struct {
    commitments: []const g1.Affine,

    /// `t`, the reconstruction threshold this vector commits to. REAL:
    /// plain length read.
    pub fn threshold(self: VerificationVector) usize {
        return self.commitments.len;
    }

    /// Length-prefixed encoding: `u32-BE count || commitments[0] || ...
    /// || commitments[count-1]` (each a 48-byte `g1` compressed point).
    /// REAL — mechanical concatenation over already-real
    /// `g1.toBytesCompressed`, the same "allocate the exact owned
    /// slice, encode in order" shape as `frost.
    /// encodeGroupCommitmentList`. Caller frees the returned slice with
    /// `allocator`.
    pub fn toBytesAlloc(self: VerificationVector, allocator: std.mem.Allocator) std.mem.Allocator.Error![]u8 {
        const out = try allocator.alloc(u8, 4 + self.commitments.len * g1.compressed_bytes);
        std.mem.writeInt(u32, out[0..4], @intCast(self.commitments.len), .big);
        for (self.commitments, 0..) |c, i| {
            const off = 4 + i * g1.compressed_bytes;
            out[off..][0..g1.compressed_bytes].* = g1.toBytesCompressed(c);
        }
        return out;
    }

    /// Inverse of `toBytesAlloc`. REAL: validates the length-prefix
    /// against the actual byte length, then decodes each compressed
    /// point (on-curve checked by `g1.fromBytesCompressed`; NOT
    /// subgroup-checked — same convention as every other `fromBytes` in
    /// this module). Caller frees the returned `commitments` slice with
    /// `allocator`.
    pub fn fromBytesAlloc(allocator: std.mem.Allocator, bytes: []const u8) ThresholdError!VerificationVector {
        if (bytes.len < 4) return error.InvalidEncoding;
        const count = std.mem.readInt(u32, bytes[0..4], .big);
        const expected_len = 4 + @as(usize, count) * g1.compressed_bytes;
        if (bytes.len != expected_len) return error.InvalidEncoding;

        const commitments = try allocator.alloc(g1.Affine, count);
        errdefer allocator.free(commitments);
        for (commitments, 0..) |*slot, i| {
            const off = 4 + i * g1.compressed_bytes;
            slot.* = try g1.fromBytesCompressed(bytes[off..][0..g1.compressed_bytes].*);
        }
        return .{ .commitments = commitments };
    }
};

// ── trusted-dealer split (Shamir + Feldman) ─────────────────────────

/// `splitSecretKey`'s return value: `n` shares plus the `t`-length
/// Feldman commitment to the sharing polynomial. `shares` and
/// `vvec.commitments` are TWO SEPARATE owned allocations (both made
/// with the same `allocator` passed to `splitSecretKey`) — free each
/// individually.
pub const SplitResult = struct {
    shares: []SecretKeyShare,
    vvec: VerificationVector,
};

/// Trusted-dealer Shamir splitting of `sk` into `n` shares
/// reconstructible by any `t` of them, PLUS a Feldman VSS commitment so
/// every share is publicly checkable (`derivePublicKeyShare`) without
/// revealing any share value. Mirrors `frost.trustedDealerKeygen`
/// (`secret_share_shard` + `vss_commit`, RFC 9591 Appendix C.1/C.2)
/// over THIS module's `Fr`/`g1` instead of secp256k1 — the identical
/// two-part construction:
///
/// ```text
/// // 1. secret_share_shard (Shamir): the degree-(t-1) polynomial
/// //      f(x) = sk + coefficients[0]*x + ... + coefficients[t-2]*x^(t-1)
/// //    (coefficients is the caller-supplied RANDOM a_1..a_{t-1}; f(0) = sk)
/// for i in 1..=n:
///     shares[i] = (i, f(i))                 // Horner's method — evalPolynomialAt
///
/// // 2. vss_commit (Feldman): one G1 commitment per coefficient
/// commitments[0] = [sk]G1
/// commitments[j] = [coefficients[j-1]]G1     for j in 1..t            // feldmanCommitCoefficient
/// ```
///
/// `coefficients` is CALLER-SUPPLIED randomness (length exactly
/// `t - 1`), not sampled internally — the same "deterministic entry
/// point" shape `frost.trustedDealerKeygen` uses for its own
/// `coefficients` parameter (reproducible tests; a real deployment's
/// caller samples fresh coefficients via `Fr.random`/`std.Io`, once per
/// dealing, and discards them immediately after — retaining them
/// defeats Shamir's security exactly as retaining `sk` itself would).
///
/// The two inner per-iteration computations (`evalPolynomialAt`,
/// `feldmanCommitCoefficient`) are the crypto core — see their own doc
/// comments for the exact construction and const-time notes.
pub fn splitSecretKey(
    allocator: std.mem.Allocator,
    sk: bls_sig.SecretKey,
    t: u32,
    n: u32,
    coefficients: []const Fr,
) ThresholdError!SplitResult {
    if (t == 0 or n == 0 or t > n) return error.InvalidParameters;
    if (coefficients.len != @as(usize, t) - 1) return error.InvalidParameters;

    const shares = try allocator.alloc(SecretKeyShare, n);
    errdefer allocator.free(shares);
    var i: u32 = 1;
    while (i <= n) : (i += 1) {
        shares[i - 1] = .{ .index = i, .scalar = evalPolynomialAt(sk.scalar, coefficients, i) };
    }

    const commitments = try allocator.alloc(g1.Affine, t);
    errdefer allocator.free(commitments);
    for (commitments, 0..) |*slot, j| {
        const coeff = if (j == 0) sk.scalar else coefficients[j - 1];
        slot.* = feldmanCommitCoefficient(coeff);
    }

    return .{ .shares = shares, .vvec = .{ .commitments = commitments } };
}

/// Shamir polynomial evaluation `f(index)` via Horner's method, over
/// `Fr`:
///
/// ```text
/// acc = 0
/// // coefficients holds a_1..a_{t-1}; visit HIGHEST degree first
/// for a_j in reverse(coefficients):
///     acc = acc * Fr(index) + a_j
/// return acc * Fr(index) + secret          // constant term (a_0 = secret) added LAST
/// ```
///
/// (`frost.trustedDealerKeygen`'s inline `while (j > 0)` Horner loop is
/// the byte-for-byte-identical shape over `Secp256k1.scalar.Scalar`
/// instead of this module's `Fr` — that loop, ported directly with the
/// field type and the `x` encoding swapped: `index` here is a plain
/// `u32`, not a `frost.Identifier`, converted to an `Fr` via
/// `frFromIndex`.) SECRET-touching: `secret` and every entry of
/// `coefficients` are secret, and the polynomial's OUTPUT (a Shamir
/// share) is secret too — every operation here goes through `Fr`'s
/// already-constant-time `add`/`mul` (`scalar.zig`, `std.crypto.ff`
/// Montgomery arithmetic), with no secret-dependent branching (the loop
/// bound `coefficients.len` and `index` are both PUBLIC, so looping/
/// indexing on them is fine).
fn evalPolynomialAt(secret: Fr, coefficients: []const Fr, index: u32) Fr {
    const x = frFromIndex(index);
    var acc = Fr.zero;
    var j: usize = coefficients.len;
    while (j > 0) : (j -= 1) acc = acc.mul(x).add(coefficients[j - 1]);
    return acc.mul(x).add(secret); // constant term (a_0 = secret) last
}

/// Converts a PUBLIC participant index (a Shamir evaluation point) to
/// its `Fr` element via a zero-padded 32-byte big-endian encoding.
/// Every `u32` is far below the group order `r` (~2^255), so
/// `Fr.fromBytes`'s canonicality rejection can never fire.
fn frFromIndex(index: u32) Fr {
    var buf = [_]u8{0} ** Fr.encoded_bytes;
    std.mem.writeInt(u32, buf[Fr.encoded_bytes - 4 ..][0..4], index, .big);
    return Fr.fromBytes(buf) catch unreachable; // u32 << r: always canonical
}

/// One Feldman VSS commitment: `[coeff]G1`
/// (`g1.Jacobian.fromAffine(g1.Affine.generator).scalarMul(coeff).
/// toAffine()`) — PUBLIC output, but `coeff` itself is a secret
/// polynomial coefficient (or `sk` itself, for `j == 0`), so the
/// multiplication MUST use `g1.zig`'s constant-time `scalarMul` — this
/// is EXACTLY `bls_sig.skToPk`'s own construction, one call site
/// removed (`sk`/`coeff` play the identical role `SecretKey.scalar`
/// plays there — `g1.Jacobian.scalarMul` is constant-time
/// double-and-add-always, see `g1.zig`).
fn feldmanCommitCoefficient(coeff: Fr) g1.Affine {
    return g1.Jacobian.fromAffine(g1.Affine.generator).scalarMul(coeff).toAffine();
}

// ── public-key-share derivation ─────────────────────────────────────

/// The group's BLS public key: `vvec.commitments[0]`, i.e. `[sk]G1` —
/// the same "`vss_commitment[0]` IS the group public key" identity RFC
/// 9591 Appendix C documents (`frost.TrustedDealerKeygenResult.
/// group_public_key`). REAL: plain indexing, no field/group arithmetic
/// of its own — `commitments[0]` was already computed (by
/// `splitSecretKey`, or by a caller
/// constructing a `VerificationVector` some other way).
pub fn groupPublicKey(vvec: VerificationVector) bls_sig.PublicKey {
    return .{ .point = vvec.commitments[0] };
}

/// Evaluates the Feldman commitment polynomial
/// `C(x) = commitments[0] + commitments[1]*x + ... + commitments[t-1]*x^(t-1)`
/// AT `x = index`, IN THE EXPONENT — no discrete log needed, since
/// `C(x) == [f(x)]G1` for the SAME sharing polynomial `f`
/// `splitSecretKey` dealt shares from (linearity of `[.]G1` in the
/// coefficients). This reconstructs participant `index`'s PUBLIC key
/// share WITHOUT ever seeing that participant's secret share `f(index)`
/// — the defining VSS-consistency property: `derivePublicKeyShare(vvec,
/// i)` MUST equal `[share_i]G1` for every share `splitSecretKey` dealt,
/// which is exactly what this file's Feldman-consistency test checks.
///
/// Construction — Horner's method IN THE EXPONENT (mirrors
/// `evalPolynomialAt`'s scalar-field Horner loop one tower level up;
/// `index` is PUBLIC here, unlike `evalPolynomialAt`'s secret
/// coefficients, so a variable-time ladder would be an acceptable
/// follow-up optimization once correctness is established — not a
/// requirement for the first crypto-core pass):
///
/// ```text
/// x = Fr(index)                                   // same u32 -> Fr conversion as evalPolynomialAt
/// acc = commitments[t-1]
/// for j in (t-2)..=0:
///     acc = [x]acc + commitments[j]                // g1 scalarMul (by x) + g1 add
/// return PublicKeyShare{ .index = index, .point = acc }   // == Σ_j x^j * commitments[j]
/// ```
pub fn derivePublicKeyShare(vvec: VerificationVector, index: u32) PublicKeyShare {
    const x = frFromIndex(index);
    const commitments = vvec.commitments;
    var acc = g1.Jacobian.fromAffine(commitments[commitments.len - 1]);
    var j: usize = commitments.len - 1;
    while (j > 0) : (j -= 1) {
        acc = acc.scalarMul(x).add(g1.Jacobian.fromAffine(commitments[j - 1]));
    }
    return .{ .index = index, .point = acc.toAffine() };
}

// ── partial signing / verification (REAL — thin over Part 4) ───────

/// A participant's non-interactive partial signature:
/// `[share.scalar]Q` where `Q = hashToCurveG2(msg, bls_sig.dst_sig)` —
/// EXACTLY Part 4's `bls_sig.sign`, just called with the SHARE's scalar
/// instead of the whole secret key. This is the entire reason
/// threshold BLS needs no interactive signing round: `sign`'s equation
/// `R = SK * Q` is LINEAR in `SK`, so any Shamir-shared piece of `SK`
/// produces a correspondingly-shared piece of `R`, and Lagrange
/// interpolation in the exponent (`combineSignatures`) recombines the
/// pieces exactly as it would recombine the secret key itself. REAL: a
/// thin, judgment-free wrapper over Part 4's already-real, already-
/// constant-time `bls_sig.sign` — no new crypto judgment is introduced
/// here.
pub fn partialSign(share: SecretKeyShare, msg: []const u8) PartialSignature {
    const partial_sk: bls_sig.SecretKey = .{ .scalar = share.scalar };
    const sig = bls_sig.sign(partial_sk, msg);
    return .{ .index = share.index, .point = sig.point };
}

/// Checks partial signature `partial` against participant `partial.
/// index`'s public key share `pk_share` (obtained via
/// `derivePublicKeyShare`, or a participant's own `bls_sig.skToPk` on
/// their `SecretKeyShare`) — lets a Coordinator identify a misbehaving
/// signer BEFORE combining, the same "Identifiable Abort" motivation as
/// `frost.verifySignatureShare`. REAL: once unwrapped, `pk_share.point`
/// and `partial.point` are literally a `bls_sig.PublicKey`/`bls_sig.
/// Signature` pair, and `partialSign`'s construction (above) IS
/// `bls_sig.sign` under a share-derived key — `bls_sig.verify`'s
/// existing pairing equation checks it back exactly the same way it
/// checks any ordinary BLS signature (`e(pk_share, Q) == e(G1_gen,
/// partial)`), including ALL of that function's mandatory fail-closed
/// subgroup/`keyValidate` checks. This is a thin wrapper, not new
/// crypto. Returns `false` (never panics) on an index mismatch between
/// `pk_share`/`partial` — a caller bug this function can and should
/// catch cheaply before doing any pairing work.
pub fn verifyPartialSignature(pk_share: PublicKeyShare, msg: []const u8, partial: PartialSignature) bool {
    if (pk_share.index != partial.index) return false;
    const pk: bls_sig.PublicKey = .{ .point = pk_share.point };
    const sig: bls_sig.Signature = .{ .point = partial.point };
    return bls_sig.verify(pk, msg, sig);
}

// ── combining (SECURITY-CRITICAL) ───────────────────────────────────

/// Combines `t` (or more) valid, distinctly-indexed partial signatures
/// into ONE ordinary BLS `Signature` — byte-for-byte the SAME signature
/// `bls_sig.sign(sk, msg)` would have produced directly, because
/// Lagrange interpolation in the exponent reconstructs `[sk]Q` from the
/// partials' `[share_i]Q` pieces exactly as `frost.secretShareCombine`
/// (or this file's own `evalPolynomialAt`'s scalar-field inverse
/// operation) reconstructs `sk` itself from `share_i` values — only
/// every scalar multiplication happens in `G2` (on `Q`) instead of on
/// the scalar field directly:
///
/// ```text
/// x_coords = [i for (i, _) in partials]
/// combined = Identity_G2
/// for (i, partial_i) in partials:
///     lambda_i = Π_{j in x_coords, j != i} (0 - x_j) / (x_i - x_j)   // Fr; division via Fr.inv
///     combined = combined + [lambda_i] * partial_i                    // g2 scalarMul + g2 add
/// return Signature{ .point = combined.toAffine() }
/// ```
///
/// **The Lagrange coefficient formula** (identical in shape to
/// `frost.deriveInterpolatingValue`'s, restated here for this module's
/// own `Fr` and its `u32` indices):
///
/// ```text
/// lambda_i = Π_{j != i} (0 - x_j) / (x_i - x_j)
///          = Π_{j != i} (-x_j) * (x_i - x_j)^-1        // x_j, x_i as Fr(index)
/// ```
///
/// — evaluating the unique degree-`(t-1)` polynomial the dealt shares
/// lie on AT `x = 0` (`splitSecretKey`'s own `f(0) = sk`). Requires
/// `>= t` DISTINCT indices among `partials` (validated below, REAL);
/// using MORE than `t` distinct, mutually-consistent partials is
/// mathematically fine — an over-determined but internally-consistent
/// Lagrange interpolation reconstructs the SAME unique low-degree
/// polynomial (the identical reasoning `frost.secretShareCombine`'s own
/// doc comment gives for accepting an arbitrary-size `shares` slice
/// without a separate `t` check on ITS OWN — this function is handed an
/// explicit `t` instead so it can reject a too-small set up front,
/// before doing any curve arithmetic).
///
/// The distinct-index / minimum-count / nonzero-index precondition
/// checks below run first (mechanical, no field/group arithmetic — same
/// `O(n^2)` public-index comparison `frost.deriveInterpolatingValue`/
/// `secretShareCombine` use; index `0` is additionally rejected here
/// because an in-memory-constructed partial bypasses
/// `PartialSignature.fromBytes`'s own `ZeroIndex` check, and `x = 0` as
/// a Lagrange node would silently return that partial VERBATIM instead
/// of interpolating — actively wrong, not merely undefined). All index
/// arithmetic is over PUBLIC values (see the module doc comment's
/// const-time note); `Fr.inv` is constant-time regardless.
pub fn combineSignatures(partials: []const PartialSignature, t: u32) ThresholdError!bls_sig.Signature {
    if (t == 0 or partials.len < t) return error.InsufficientShares;
    for (partials, 0..) |a, i| {
        if (a.index == 0) return error.ZeroIndex;
        for (partials[i + 1 ..]) |b| {
            if (a.index == b.index) return error.DuplicateIndex;
        }
    }

    var combined = g2.Jacobian.identity;
    for (partials) |partial_i| {
        // lambda_i = Π_{j != i} x_j * (x_j - x_i)^-1 — frost.
        // deriveInterpolatingValue's exact numerator/denominator-product
        // form (equivalently Π (0 - x_j)/(x_i - x_j): both signs flip,
        // so the quotient is identical).
        const xi = frFromIndex(partial_i.index);
        var numerator = Fr.one;
        var denominator = Fr.one;
        for (partials) |partial_j| {
            if (partial_j.index == partial_i.index) continue;
            const xj = frFromIndex(partial_j.index);
            numerator = numerator.mul(xj);
            denominator = denominator.mul(xj.sub(xi));
        }
        // Every factor x_j - x_i is nonzero (indices are distinct u32s
        // < r, so distinct as Fr elements), and a product of nonzero
        // elements of the prime field Fr is nonzero — invertible.
        const lambda_i = numerator.mul(denominator.inv() catch unreachable);
        combined = combined.add(g2.Jacobian.fromAffine(partial_i.point).scalarMul(lambda_i));
    }
    return .{ .point = combined.toAffine() };
}

// ── tests: wire/plumbing ─────────────────────────────────────────────

test "SecretKeyShare/PublicKeyShare/PartialSignature byte codecs round-trip" {
    const sk_share: SecretKeyShare = .{ .index = 3, .scalar = try Fr.fromBytes([_]u8{0} ** 31 ++ [_]u8{7}) };
    const sk_bytes = sk_share.toBytes();
    try std.testing.expectEqual(@as(usize, 36), sk_bytes.len);
    const sk_back = try SecretKeyShare.fromBytes(sk_bytes);
    try std.testing.expectEqual(sk_share.index, sk_back.index);
    try std.testing.expect(sk_share.scalar.eql(sk_back.scalar));

    const pk_share: PublicKeyShare = .{ .index = 5, .point = g1.Affine.generator };
    const pk_bytes = pk_share.toBytes();
    try std.testing.expectEqual(@as(usize, 52), pk_bytes.len);
    const pk_back = try PublicKeyShare.fromBytes(pk_bytes);
    try std.testing.expectEqual(pk_share.index, pk_back.index);
    try std.testing.expect(pk_share.point.x.eql(pk_back.point.x));

    const partial: PartialSignature = .{ .index = 2, .point = g2.Affine.generator };
    const partial_bytes = partial.toBytes();
    try std.testing.expectEqual(@as(usize, 100), partial_bytes.len);
    const partial_back = try PartialSignature.fromBytes(partial_bytes);
    try std.testing.expectEqual(partial.index, partial_back.index);
    try std.testing.expect(partial.point.x.c0.eql(partial_back.point.x.c0));
}

test "SecretKeyShare.deinit zeroizes scalar but leaves index untouched (regression: fails if secureZero is removed)" {
    var sk_share: SecretKeyShare = .{ .index = 3, .scalar = try Fr.fromBytes([_]u8{0} ** 31 ++ [_]u8{7}) };
    // `Fr`'s in-memory (Montgomery) size may exceed its 32-byte canonical
    // encoding, so assert over the raw struct bytes, not a fixed-width buffer.
    var any_nonzero = false;
    for (std.mem.asBytes(&sk_share.scalar)) |b| {
        if (b != 0) any_nonzero = true;
    }
    try std.testing.expect(any_nonzero);
    sk_share.deinit();
    for (std.mem.asBytes(&sk_share.scalar)) |b| try std.testing.expectEqual(@as(u8, 0), b);
    try std.testing.expectEqual(@as(u32, 3), sk_share.index);
}

test "SecretKeyShare/PublicKeyShare/PartialSignature fromBytes rejects a zero index" {
    var sk_bytes = [_]u8{0} ** SecretKeyShare.encoded_bytes;
    sk_bytes[SecretKeyShare.encoded_bytes - 1] = 1; // scalar = 1, index = 0
    try std.testing.expectError(error.ZeroIndex, SecretKeyShare.fromBytes(sk_bytes));

    var pk_bytes = [_]u8{0} ** PublicKeyShare.encoded_bytes;
    pk_bytes[4..PublicKeyShare.encoded_bytes].* = g1.toBytesCompressed(g1.Affine.generator);
    try std.testing.expectError(error.ZeroIndex, PublicKeyShare.fromBytes(pk_bytes));

    var sig_bytes = [_]u8{0} ** PartialSignature.encoded_bytes;
    sig_bytes[4..PartialSignature.encoded_bytes].* = g2.toBytesCompressed(g2.Affine.generator);
    try std.testing.expectError(error.ZeroIndex, PartialSignature.fromBytes(sig_bytes));
}

test "VerificationVector toBytesAlloc/fromBytesAlloc round-trip" {
    const allocator = std.testing.allocator;
    const commitments = [_]g1.Affine{
        g1.Affine.generator,
        g1.Jacobian.fromAffine(g1.Affine.generator).double().toAffine(),
    };
    const vvec: VerificationVector = .{ .commitments = &commitments };
    try std.testing.expectEqual(@as(usize, 2), vvec.threshold());

    const bytes = try vvec.toBytesAlloc(allocator);
    defer allocator.free(bytes);
    try std.testing.expectEqual(@as(usize, 4 + 2 * g1.compressed_bytes), bytes.len);

    const back = try VerificationVector.fromBytesAlloc(allocator, bytes);
    defer allocator.free(back.commitments);
    try std.testing.expectEqual(@as(usize, 2), back.threshold());
    try std.testing.expect(back.commitments[0].x.eql(commitments[0].x));
    try std.testing.expect(back.commitments[1].x.eql(commitments[1].x));
}

test "VerificationVector.fromBytesAlloc rejects a length/count mismatch" {
    const allocator = std.testing.allocator;
    var bytes = [_]u8{0} ** 4;
    std.mem.writeInt(u32, bytes[0..4], 3, .big); // claims 3 commitments, has 0
    try std.testing.expectError(error.InvalidEncoding, VerificationVector.fromBytesAlloc(allocator, &bytes));
}

test "groupPublicKey extracts commitments[0]" {
    const commitments = [_]g1.Affine{
        g1.Affine.generator,
        g1.Jacobian.fromAffine(g1.Affine.generator).double().toAffine(),
    };
    const vvec: VerificationVector = .{ .commitments = &commitments };
    const pk = groupPublicKey(vvec);
    try std.testing.expect(pk.point.x.eql(g1.Affine.generator.x));
}

test "splitSecretKey rejects invalid (t, n, coefficients) combinations before any crypto work" {
    const allocator = std.testing.allocator;
    const sk = try bls_sig.keyGen(&([_]u8{0x9a} ** 32), "");
    try std.testing.expectError(error.InvalidParameters, splitSecretKey(allocator, sk, 0, 5, &.{}));
    try std.testing.expectError(error.InvalidParameters, splitSecretKey(allocator, sk, 3, 0, &.{}));
    try std.testing.expectError(error.InvalidParameters, splitSecretKey(allocator, sk, 4, 3, &.{ Fr.one, Fr.one, Fr.one }));
    // t=3 needs exactly 2 coefficients; supplying 1 must be rejected
    // before any polynomial evaluation is attempted.
    try std.testing.expectError(error.InvalidParameters, splitSecretKey(allocator, sk, 3, 5, &.{Fr.one}));
}

test "combineSignatures rejects too few, duplicate-indexed, or zero-indexed partials before any crypto work" {
    const partials = [_]PartialSignature{
        .{ .index = 1, .point = g2.Affine.generator },
        .{ .index = 2, .point = g2.Affine.generator },
    };
    try std.testing.expectError(error.InsufficientShares, combineSignatures(&partials, 3));
    try std.testing.expectError(error.InsufficientShares, combineSignatures(&partials, 0));

    const dup_partials = [_]PartialSignature{
        .{ .index = 1, .point = g2.Affine.generator },
        .{ .index = 1, .point = g2.Affine.generator },
    };
    try std.testing.expectError(error.DuplicateIndex, combineSignatures(&dup_partials, 2));

    // An in-memory-constructed partial can carry index 0 (fromBytes
    // would have rejected it); combineSignatures must too — x = 0 is
    // the SECRET's own evaluation point, never a participant's.
    const zero_partials = [_]PartialSignature{
        .{ .index = 0, .point = g2.Affine.generator },
        .{ .index = 2, .point = g2.Affine.generator },
    };
    try std.testing.expectError(error.ZeroIndex, combineSignatures(&zero_partials, 2));
}

test "partialSign/verifyPartialSignature round-trip in isolation (a share IS just a bls_sig.SecretKey)" {
    // A hand-rolled "share" (not dealt via splitSecretKey — this test
    // exercises partialSign/verifyPartialSignature in isolation, both
    // of which are REAL today) — any nonzero Fr scalar behaves exactly
    // like an ordinary bls_sig.SecretKey for these two functions' own
    // contracts, since partialSign IS bls_sig.sign under the hood.
    const sk = try bls_sig.keyGen(&([_]u8{0x5c} ** 32), "");
    const share: SecretKeyShare = .{ .index = 7, .scalar = sk.scalar };
    const pk_share: PublicKeyShare = .{ .index = 7, .point = bls_sig.skToPk(sk).point };

    const msg = "threshold BLS partial signing, no dealer involved";
    const partial = partialSign(share, msg);
    try std.testing.expectEqual(@as(u32, 7), partial.index);
    try std.testing.expect(verifyPartialSignature(pk_share, msg, partial));

    // The plain bls_sig equation agrees: a partial signed with a
    // share's raw scalar IS an ordinary bls_sig.sign output under the
    // corresponding bls_sig public key.
    const ordinary_sig = bls_sig.sign(sk, msg);
    try std.testing.expectEqualSlices(u8, &ordinary_sig.toBytes(), &(bls_sig.Signature{ .point = partial.point }).toBytes());

    // Wrong message, wrong key, and an index mismatch must all reject.
    try std.testing.expect(!verifyPartialSignature(pk_share, "wrong message", partial));
    const other_sk = try bls_sig.keyGen(&([_]u8{0x5d} ** 32), "");
    const other_pk_share: PublicKeyShare = .{ .index = 7, .point = bls_sig.skToPk(other_sk).point };
    try std.testing.expect(!verifyPartialSignature(other_pk_share, msg, partial));
    const mismatched_partial: PartialSignature = .{ .index = 8, .point = partial.point };
    try std.testing.expect(!verifyPartialSignature(pk_share, msg, mismatched_partial));
}

// ── tests: the keystone self-consistency chain ──────────────────────

/// Builds a fresh `(sk, t, n)` dealing with fixed, reproducible
/// "randomness" (test-only — a real deployment MUST use genuinely
/// random coefficients, see `splitSecretKey`'s doc comment) and returns
/// the split result. Shared by every keystone test below so they all
/// dial the same knobs consistently.
fn testSplit(allocator: std.mem.Allocator, ikm_byte: u8, t: u32, n: u32) !struct { sk: bls_sig.SecretKey, split: SplitResult } {
    const sk = try bls_sig.keyGen(&([_]u8{ikm_byte} ** 32), "");
    var coeffs: [8]Fr = undefined;
    var j: usize = 0;
    while (j < t - 1) : (j += 1) {
        var seed = [_]u8{0} ** 32;
        seed[31] = @intCast(j + 1);
        seed[30] = ikm_byte;
        coeffs[j] = Fr.reduceWide(&seed);
    }
    const split = try splitSecretKey(allocator, sk, t, n, coeffs[0 .. t - 1]);
    return .{ .sk = sk, .split = split };
}

test "keystone round trip: splitSecretKey -> partialSign (t of n) -> combineSignatures == bls_sig.sign, and verifies under groupPublicKey" {
    const allocator = std.testing.allocator;
    const t: u32 = 3;
    const n: u32 = 5;
    const built = try testSplit(allocator, 0x01, t, n);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "threshold BLS keystone round trip";
    const partials = [_]PartialSignature{
        partialSign(built.split.shares[0], msg),
        partialSign(built.split.shares[2], msg),
        partialSign(built.split.shares[4], msg),
    };
    const combined = try combineSignatures(&partials, t);

    // This is the chain that ties the threshold path to Part 4's
    // eth-vector-pinned signatures: the combined signature must be
    // byte-for-byte the SAME as an ordinary bls_sig.sign, and must
    // verify under the group public key via the ordinary bls_sig.verify
    // equation.
    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expectEqualSlices(u8, &expected.toBytes(), &combined.toBytes());
    try std.testing.expect(bls_sig.verify(groupPublicKey(built.split.vvec), msg, combined));
}

test "any t-of-n subset combines to the SAME signature (two different 3-of-5 subsets)" {
    const allocator = std.testing.allocator;
    const t: u32 = 3;
    const n: u32 = 5;
    const built = try testSplit(allocator, 0x02, t, n);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "any t-of-n subset agrees";
    const shares = built.split.shares;

    const subset_a = [_]PartialSignature{
        partialSign(shares[0], msg),
        partialSign(shares[1], msg),
        partialSign(shares[2], msg),
    };
    const subset_b = [_]PartialSignature{
        partialSign(shares[1], msg),
        partialSign(shares[3], msg),
        partialSign(shares[4], msg),
    };

    const combined_a = try combineSignatures(&subset_a, t);
    const combined_b = try combineSignatures(&subset_b, t);
    try std.testing.expectEqualSlices(u8, &combined_a.toBytes(), &combined_b.toBytes());

    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expectEqualSlices(u8, &expected.toBytes(), &combined_a.toBytes());
}

test "Feldman VSS consistency: derivePublicKeyShare(vvec, i) equals [share_i]G1 for every dealt share" {
    const allocator = std.testing.allocator;
    const t: u32 = 2;
    const n: u32 = 3;
    const built = try testSplit(allocator, 0x03, t, n);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    for (built.split.shares) |share| {
        const derived = derivePublicKeyShare(built.split.vvec, share.index);
        const expected = bls_sig.skToPk(.{ .scalar = share.scalar });
        try std.testing.expectEqual(share.index, derived.index);
        try std.testing.expect(derived.point.x.eql(expected.point.x));
    }
}

test "(t=2, n=3) end-to-end keystone chain" {
    const allocator = std.testing.allocator;
    const t: u32 = 2;
    const n: u32 = 3;
    const built = try testSplit(allocator, 0x04, t, n);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "t=2,n=3 case";
    const partials = [_]PartialSignature{
        partialSign(built.split.shares[0], msg),
        partialSign(built.split.shares[1], msg),
    };
    const combined = try combineSignatures(&partials, t);
    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expectEqualSlices(u8, &expected.toBytes(), &combined.toBytes());
    try std.testing.expect(bls_sig.verify(groupPublicKey(built.split.vvec), msg, combined));
}

test "fewer than t partials do NOT reconstruct the group signature (t is really the threshold)" {
    // A t=3 dealing, but only 2 partials combined (with a LYING t'=2 so
    // the count check passes): Lagrange through 2 points of a genuinely
    // degree-2 polynomial interpolates the WRONG constant term — the
    // result must neither equal bls_sig.sign nor verify under the group
    // key. (This is the caller-side precondition frost.
    // secretShareCombine's doc comment describes: too few shares give a
    // mathematically wrong answer, not an error — combineSignatures's
    // explicit `t` parameter exists precisely so an honest caller CAN
    // get the error instead, as the plumbing test above checks.)
    const allocator = std.testing.allocator;
    const built = try testSplit(allocator, 0x06, 3, 5);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "below-threshold subsets must fail";
    const partials = [_]PartialSignature{
        partialSign(built.split.shares[0], msg),
        partialSign(built.split.shares[1], msg),
    };
    const under = try combineSignatures(&partials, 2);
    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expect(!std.mem.eql(u8, &expected.toBytes(), &under.toBytes()));
    try std.testing.expect(!bls_sig.verify(groupPublicKey(built.split.vvec), msg, under));
}

test "more than t distinct partials combine to the same signature (over-determined interpolation)" {
    const allocator = std.testing.allocator;
    const t: u32 = 3;
    const built = try testSplit(allocator, 0x07, t, 5);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "over-determined but consistent";
    const partials = [_]PartialSignature{
        partialSign(built.split.shares[0], msg),
        partialSign(built.split.shares[1], msg),
        partialSign(built.split.shares[2], msg),
        partialSign(built.split.shares[3], msg),
    };
    const combined = try combineSignatures(&partials, t);
    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expectEqualSlices(u8, &expected.toBytes(), &combined.toBytes());
}

test "verifyPartialSignature over a real dealing: accepts each valid partial, rejects wrong-share and wrong-index partials" {
    const allocator = std.testing.allocator;
    const built = try testSplit(allocator, 0x08, 3, 5);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "identifiable abort: check partials before combining";
    const shares = built.split.shares;

    // Every honestly-produced partial verifies against the VSS-derived
    // public key share for its own index.
    for (shares) |share| {
        const pk_share = derivePublicKeyShare(built.split.vvec, share.index);
        try std.testing.expect(verifyPartialSignature(pk_share, msg, partialSign(share, msg)));
    }

    // A partial made with share 2's scalar but claiming index 1 must be
    // rejected against index 1's derived public key share (wrong share
    // under the claimed identity — the misbehaving-signer case).
    const pk_share_1 = derivePublicKeyShare(built.split.vvec, 1);
    const forged: PartialSignature = .{ .index = 1, .point = partialSign(shares[1], msg).point };
    try std.testing.expect(shares[1].index == 2); // shares[1] IS index 2
    try std.testing.expect(!verifyPartialSignature(pk_share_1, msg, forged));

    // An index mismatch between the pk share and the partial is caught
    // before any pairing work.
    const honest_1 = partialSign(shares[0], msg);
    const relabeled: PartialSignature = .{ .index = 3, .point = honest_1.point };
    try std.testing.expect(!verifyPartialSignature(pk_share_1, msg, relabeled));
}

test "(t=n=4) end-to-end keystone chain (every share required)" {
    const allocator = std.testing.allocator;
    const t: u32 = 4;
    const n: u32 = 4;
    const built = try testSplit(allocator, 0x05, t, n);
    defer allocator.free(built.split.shares);
    defer allocator.free(built.split.vvec.commitments);

    const msg = "t=n=4 case: no slack, every share needed";
    const partials = [_]PartialSignature{
        partialSign(built.split.shares[0], msg),
        partialSign(built.split.shares[1], msg),
        partialSign(built.split.shares[2], msg),
        partialSign(built.split.shares[3], msg),
    };
    const combined = try combineSignatures(&partials, t);
    const expected = bls_sig.sign(built.sk, msg);
    try std.testing.expectEqualSlices(u8, &expected.toBytes(), &combined.toBytes());
    try std.testing.expect(bls_sig.verify(groupPublicKey(built.split.vvec), msg, combined));
}
