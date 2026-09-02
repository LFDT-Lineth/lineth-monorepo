// SPDX-License-Identifier: MIT
//! `Fp12 = Fp6[w] / (w^2 - v)` — the top of the extension-field tower,
//! i.e. `w^2 = v` (`v` = `Fp6`'s indeterminate, `fp6.zig`). An element is
//! `c0 + c1*w`, `c0, c1 ∈ Fp6`. This is the pairing's TARGET field: the
//! output of `e: G1 x G2 -> Gt`, `Gt` being the order-`r` subgroup of
//! `Fp12*` (the Miller loop + final exponentiation themselves live in
//! `pairing.zig`; this file supplies the field ops they run on).
//!
//! **Status: implemented** — general field arithmetic (`add`/`sub`/
//! `neg`/`mul`/`square`/`inv`/`conjugate`/`frobenius`) plus the two
//! pairing-specific helpers the Part-2 crypto-core pass filled in:
//! `cyclotomicSquare` (Granger–Scott, consumed by the final
//! exponentiation's hard part) and `frobeniusMap` (repeated Frobenius).
//! The sparse Miller-loop line multiplication remains a `// TODO`
//! optimization marker at the bottom of the struct (dense `mul` is the
//! correctness-first baseline `pairing.zig` uses).

const std = @import("std");
const fp = @import("fp.zig");
const fp2mod = @import("fp2.zig");
const fp6mod = @import("fp6.zig");

pub const Fp = fp.Fp;
pub const Fp2 = fp2mod.Fp2;
pub const Fp6 = fp6mod.Fp6;
pub const FpError = fp.FpError;

/// The `Fp12` non-residue is `v` itself, i.e. the `Fp6` element `(c0=0,
/// c1=1, c2=0)` — `w^2 = v` is fixed to this (see module doc comment).
/// REAL: a plain comptime `Fp6` literal, no arithmetic.
pub const nonresidue: Fp6 = .{ .c0 = Fp2.zero, .c1 = Fp2.one, .c2 = Fp2.zero };

/// An element of `Fp12`: `c0 + c1*w`.
pub const Fp12 = struct {
    c0: Fp6,
    c1: Fp6,

    /// Wire encoding: `c1 (288B, HIGH) || c0 (288B, LOW)` — 576 bytes
    /// total. Same high-to-low convention as `Fp2`/`Fp6` (see
    /// `fp2.zig`'s doc comment). This is the shape a pairing OUTPUT
    /// (`Gt` element) would serialize as, if this module ever exposes
    /// raw `Fp12` serialization — the more common wire format for a
    /// `Gt` element in downstream protocols is a COMPRESSED form (not
    /// scaffolded here; out of Part 1's scope, see `README.md`).
    pub const encoded_bytes = 2 * Fp6.encoded_bytes; // 576

    pub const zero: Fp12 = .{ .c0 = Fp6.zero, .c1 = Fp6.zero };
    pub const one: Fp12 = .{ .c0 = Fp6.one, .c1 = Fp6.zero };

    /// Parses `c1 || c0`. REAL.
    pub fn fromBytes(bytes: [encoded_bytes]u8) FpError!Fp12 {
        const c1 = try Fp6.fromBytes(bytes[0..Fp6.encoded_bytes].*);
        const c0 = try Fp6.fromBytes(bytes[Fp6.encoded_bytes..encoded_bytes].*);
        return .{ .c0 = c0, .c1 = c1 };
    }

    /// Serializes `c1 || c0`. REAL.
    pub fn toBytes(self: Fp12) [encoded_bytes]u8 {
        var out: [encoded_bytes]u8 = undefined;
        out[0..Fp6.encoded_bytes].* = self.c1.toBytes();
        out[Fp6.encoded_bytes..encoded_bytes].* = self.c0.toBytes();
        return out;
    }

    /// REAL: component-wise `Fp6.isZero`.
    pub fn isZero(self: Fp12) bool {
        return self.c0.isZero() and self.c1.isZero();
    }

    /// REAL: component-wise `Fp6.eql`.
    pub fn eql(a: Fp12, b: Fp12) bool {
        return a.c0.eql(b.c0) and a.c1.eql(b.c1);
    }

    // ── field arithmetic ────────────────────────────────────────────────

    /// Component-wise `Fp6.add`.
    pub fn add(a: Fp12, b: Fp12) Fp12 {
        return .{ .c0 = a.c0.add(b.c0), .c1 = a.c1.add(b.c1) };
    }

    /// Component-wise `Fp6.sub`.
    pub fn sub(a: Fp12, b: Fp12) Fp12 {
        return .{ .c0 = a.c0.sub(b.c0), .c1 = a.c1.sub(b.c1) };
    }

    /// Component-wise `Fp6.neg`.
    pub fn neg(a: Fp12) Fp12 {
        return .{ .c0 = a.c0.neg(), .c1 = a.c1.neg() };
    }

    /// Degree-2 extension multiplication over `w^2 = v`
    /// (`nonresidue`, this file). Karatsuba: for `a = a0+a1 w`,
    /// `b = b0+b1 w`:
    /// ```
    /// v0 = a0*b0,  v1 = a1*b1   (two Fp6.mul)
    /// c0 = v0 + v*v1            (v*v1 is Fp6.mulByNonresidue)
    /// c1 = (a0+a1)*(b0+b1) - v0 - v1
    /// ```
    /// 3 `Fp6.mul` total (vs. 4 for schoolbook) — the standard
    /// construction (same Devegili et al. reference cited throughout
    /// `fp6.zig`).
    pub fn mul(a: Fp12, b: Fp12) Fp12 {
        // Karatsuba (doc comment): v0 = a0*b0, v1 = a1*b1,
        // c0 = v0 + v*v1, c1 = (a0+a1)(b0+b1) - v0 - v1.
        const v0 = a.c0.mul(b.c0);
        const v1 = a.c1.mul(b.c1);
        return .{
            .c0 = v0.add(v1.mulByNonresidue()),
            .c1 = a.c0.add(a.c1).mul(b.c0.add(b.c1)).sub(v0).sub(v1),
        };
    }

    /// `Fp12` squaring via the same Karatsuba-style dedicated formula as
    /// `mul(a,a)`, or (once the Part-2 pairing needs performance) the
    /// CYCLOTOMIC squaring formula for elements already known to be in
    /// the cyclotomic subgroup (`cyclotomicSquare`, below) — those are
    /// TWO DIFFERENT operations with different preconditions; do not
    /// conflate them. This `square` is the general one, valid for any
    /// `Fp12` element.
    pub fn square(a: Fp12) Fp12 {
        // Standard 2-mul complex-style squaring for w^2 = v (same shape
        // as zkcrypto/bls12_381's Fp12::square — see NOTICE):
        //   ab = a0*a1
        //   c0 = (a0 + a1)(a0 + v*a1) - ab - v*ab
        //   c1 = 2 ab
        const ab = a.c0.mul(a.c1);
        const c0 = a.c0.add(a.c1).mul(a.c0.add(a.c1.mulByNonresidue())).sub(ab).sub(ab.mulByNonresidue());
        return .{ .c0 = c0, .c1 = ab.add(ab) };
    }

    /// Multiplicative inverse. Construction: `a^-1 = conj(a) *
    /// norm(a)^-1` where `norm(a) = a0^2 - v*a1^2 ∈ Fp6` (mirrors
    /// `Fp2.inv`'s norm trick, one tower level up) — one `Fp6.inv` plus
    /// a few `Fp6.mul`/`Fp6.square`.
    pub fn inv(a: Fp12) error{NotInvertible}!Fp12 {
        // Norm trick (doc comment): a^-1 = conj(a) * (a0^2 - v*a1^2)^-1.
        const norm = a.c0.square().sub(a.c1.square().mulByNonresidue());
        const norm_inv = try norm.inv(); // norm == 0 iff a == 0
        return .{
            .c0 = a.c0.mul(norm_inv),
            .c1 = a.c1.neg().mul(norm_inv),
        };
    }

    /// The `Fp12/Fp6` conjugation automorphism: `w -> -w`, i.e.
    /// `conjugate(c0+c1 w) = c0 - c1 w`. NOTE this is a DIFFERENT
    /// operation from `Fp2.frobenius` (which happens to also BE
    /// conjugation, one tower level down, as a special case of `p ≡ 3
    /// mod 4` — see `fp2.zig`'s `frobenius` doc comment) — `Fp12`
    /// conjugation is the degree-2-extension automorphism `w -> -w`
    /// unconditionally, independent of `p mod 4`. Central to the
    /// pairing's FINAL EXPONENTIATION "easy part": `a^(p^6-1) =
    /// conjugate(a) * a^-1` (since `a^(p^6) = conjugate(a)` for `a ∈
    /// Fp12*` — a standard identity for THIS specific tower
    /// construction, `[Fp12:Fp6]=2` with the Fp6-Frobenius-to-the-6th
    /// collapsing to conjugation; Part 2's job to use, not this file's).
    pub fn conjugate(a: Fp12) Fp12 {
        return .{ .c0 = a.c0, .c1 = a.c1.neg() };
    }

    /// The Frobenius endomorphism `x -> x^p` restricted to `Fp12`: for
    /// `a = c0+c1 w`, `a^p = c0^p + c1^p w^p` — `c_i^p` is
    /// `Fp6.frobenius(c_i)` (`fp6.zig`), and `w^p` reduces to the `Fp2`
    /// coefficient `γ = ξ^((p-1)/6)` scaling `c1^p`. Derivation of the
    /// exponent (the scaffold's doc comment said `(p-1)/2`; the exact
    /// reduction is): `w^p = w^(p-1) · w = (w^2)^((p-1)/2) · w =
    /// v^((p-1)/2) · w`, and since `p ≡ 1 (mod 6)` the exponent
    /// `(p-1)/2` is a multiple of 3, so `v^((p-1)/2) = (v^3)^((p-1)/6)
    /// = ξ^((p-1)/6)` — both readings agree, the `(p-1)/6` form just
    /// lands the coefficient in `Fp2` explicitly. Computed
    /// PROGRAMMATICALLY (same policy as `Fp6.frobenius` — no
    /// transcribed table) and PRECOMPUTED ONCE at comptime (see
    /// `frobenius_gamma`), so no runtime `pow` runs per call; validated
    /// by the definitional "frobenius == pow(p)" test below.
    pub fn frobenius(a: Fp12) Fp12 {
        return .{
            .c0 = a.c0.frobenius(),
            .c1 = a.c1.frobenius().mulByFp2(frobenius_gamma),
        };
    }

    /// Squaring specialized to the CYCLOTOMIC SUBGROUP (elements `a`
    /// with `a^(p^4-p^2+1) = 1`, i.e. `a * conjugate(a) = 1` — the
    /// subgroup every final-exponentiation intermediate lives in AFTER
    /// the easy part). REAL (Part-2 crypto-core pass): the
    /// Granger–Scott cyclotomic-squaring formula ("Faster Squaring in
    /// the Cyclotomic Subgroup of Sixth Degree Extensions", PKC 2010 —
    /// see `NOTICE`), via the `Fp4`-subfield decomposition: with
    /// `s = w^3` (`s^2 = ξ`), an `Fp12` element splits into three `Fp4`
    /// components `A = (a_0, a_3)`, `B = (a_1, a_4)`, `C = (a_2, a_5)`
    /// (`a_i` = the `Fp2` coefficient of `w^i`), and for cyclotomic
    /// inputs the square is
    /// ```
    /// A' = 3*A^2 - 2*conj(A)
    /// B' = 3*s*C^2 + 2*conj(B)
    /// C' = 3*B^2 - 2*conj(C)
    /// ```
    /// (Fp4 conjugation = negate the `s` component; `s*(x + y*s) =
    /// ξ*y + x*s`.) 3 `fp4Square` calls — significantly cheaper than
    /// the general `square`. It is INCORRECT to call this on an
    /// arbitrary `Fp12` element NOT in the cyclotomic subgroup
    /// (silently wrong answer, no error — the formula's correctness
    /// relies on the subgroup's defining relation). Pinned by the
    /// "cyclotomicSquare == square on subgroup elements" oracle test
    /// below (the exact day-one oracle the Part-1 stub's comment
    /// promised).
    pub fn cyclotomicSquare(a: Fp12) Fp12 {
        // Fp4 pairs (a_i = Fp2 coefficient of w^i in the flat basis;
        // tower slots: c0 = (a0, a2, a4), c1 = (a1, a3, a5)):
        //   A = (a0, a3) = (c0.c0, c1.c1)
        //   B = (a1, a4) = (c1.c0, c0.c2)
        //   C = (a2, a5) = (c0.c1, c1.c2)
        const asq = fp4Square(a.c0.c0, a.c1.c1);
        const bsq = fp4Square(a.c1.c0, a.c0.c2);
        const csq = fp4Square(a.c0.c1, a.c1.c2);

        // A' = 3*A^2 - 2*conj(A): x' = 2(x2 - x) + x2, y' = 2(y2 + y) + y2.
        var r00 = asq.c0.sub(a.c0.c0);
        r00 = r00.add(r00).add(asq.c0);
        var r11 = asq.c1.add(a.c1.c1);
        r11 = r11.add(r11).add(asq.c1);

        // C' = 3*B^2 - 2*conj(C) (B^2 feeds C' — the Granger–Scott
        // cross-wiring): x' = 2(x2 - x) + x2, y' = 2(y2 + y) + y2.
        var r01 = bsq.c0.sub(a.c0.c1);
        r01 = r01.add(r01).add(bsq.c0);
        var r12 = bsq.c1.add(a.c1.c2);
        r12 = r12.add(r12).add(bsq.c1);

        // B' = 3*s*C^2 + 2*conj(B), s*C^2 = (ξ*C2y, C2x):
        //   x' = 2(ξ*C2y + x) + ξ*C2y, y' = 2(C2x - y) + C2x.
        const t = csq.c1.mulByNonresidue();
        var r10 = t.add(a.c1.c0);
        r10 = r10.add(r10).add(t);
        var r02 = csq.c0.sub(a.c0.c2);
        r02 = r02.add(r02).add(csq.c0);

        return .{
            .c0 = .{ .c0 = r00, .c1 = r01, .c2 = r02 },
            .c1 = .{ .c0 = r10, .c1 = r11, .c2 = r12 },
        };
    }

    /// `a^(p^power)`: the Frobenius endomorphism applied `power` times —
    /// the repeated-Frobenius primitive `pairing.zig`'s final-exponentiation
    /// HARD part needs (its factors get twisted by `p`, `p^2` before
    /// being combined — see that file's `finalExpHardPart` doc comment).
    /// REAL (Part-2 crypto-core pass): the NAIVE baseline — `power % 12`
    /// repeated calls to the already-real `frobenius()` above
    /// (`frobenius` has order 12 on `Fp12`, pinned by the order-12 test
    /// below, so reducing mod 12 is exact, and `frobeniusMap(a, 12) ==
    /// a` by construction). The precomputed power-indexed
    /// Frobenius-coefficient table remains a deferred optimization
    /// (`SPEC.md` Backlog — same entry as caching the single-application
    /// coefficients): the hard part calls this a handful of times per
    /// pairing, so the naive form costs a few extra fixed-exponent
    /// `Fp2.pow`s per call, dwarfed by the Miller loop itself.
    pub fn frobeniusMap(a: Fp12, power: usize) Fp12 {
        var out = a;
        for (0..power % 12) |_| out = out.frobenius();
        return out;
    }

    // TODO: a sparse "line-function times Fp12" multiplication (the
    // Miller loop's inner accumulation step multiplies by a
    // structurally-sparse Fp12 element — only 3 of its 6 Fp2
    // coefficients nonzero — much cheaper than the general `mul`
    // pairing.zig's dense baseline uses). Deferred optimization, not a
    // correctness prerequisite — see SPEC.md Backlog and
    // `multiMillerLoop`'s own TODO note.
};

/// Squaring in the `Fp4` subfield `Fp2[s]/(s^2 - ξ)` (`s = w^3`), on a
/// pair `(c0, c1)` representing `c0 + c1*s`:
/// `(c0 + c1 s)^2 = (c0^2 + ξ c1^2) + 2 c0 c1 s`, with `2 c0 c1 =
/// (c0+c1)^2 - c0^2 - c1^2` (no extra mul). `Fp12` itself never
/// materializes an `Fp4` type — this helper exists solely as
/// `cyclotomicSquare`'s building block (Granger–Scott decomposes the
/// cyclotomic square into three `Fp4` squarings — see that function).
fn fp4Square(c0: Fp2, c1: Fp2) struct { c0: Fp2, c1: Fp2 } {
    const t0 = c0.square();
    const t1 = c1.square();
    return .{
        .c0 = t1.mulByNonresidue().add(t0),
        .c1 = c0.add(c1).square().sub(t0).sub(t1),
    };
}

/// `(p-1)/6`, big-endian — the `Fp12` Frobenius-coefficient exponent
/// (comptime-derived from `fp.zig`'s verified `p_bytes`; `p ≡ 1 (mod
/// 6)`, so the division is exact — enforced at comptime).
const p_minus_1_over_6_bytes: [48]u8 = fp.pExponentBytes(-1, 6);

/// `γ = ξ^((p-1)/6)` — the `w^p` reduction coefficient scaling `c1^p`
/// (see `Fp12.frobenius`). PRECOMPUTED ONCE at COMPTIME (the fixed-public
/// `Fp2.pow` is evaluated by the compiler), so `frobenius` costs one
/// `Fp6.mulByFp2` at runtime instead of re-deriving a 381-bit `pow` on
/// every call. The byte-exact `γ` KAT and the definitional
/// "frobenius == pow(p)" test below remain the anchors.
const frobenius_gamma: Fp2 = blk: {
    @setEvalBranchQuota(50_000_000);
    break :blk fp6mod.nonresidue.pow(&p_minus_1_over_6_bytes);
};

// ── tests ────────────────────────────────────────────────────────────────

test "Fp12.zero / Fp12.one round-trip through bytes" {
    const z = Fp12.zero.toBytes();
    try std.testing.expect(std.mem.allEqual(u8, &z, 0));

    const o = Fp12.one.toBytes();
    var expected = [_]u8{0} ** Fp12.encoded_bytes;
    expected[Fp12.encoded_bytes - 1] = 1; // c0's innermost Fp low byte
    try std.testing.expectEqualSlices(u8, &expected, &o);
}

test "Fp12.fromBytes / toBytes round-trip" {
    var bytes = [_]u8{0} ** Fp12.encoded_bytes;
    // NOTE: byte 0 is the top byte of the innermost Fp element (c1's
    // Fp6.c2's Fp2.c1), which must stay < p's own top byte (0x1a) to be
    // guaranteed canonical regardless of the remaining 47 bytes — 0x11
    // (same margin as fp6.zig's identical test) is safely below that.
    bytes[0] = 0x11; // top byte of c1
    bytes[Fp12.encoded_bytes - 1] = 0x44; // low byte of c0 (safe: not a top byte)
    const el = try Fp12.fromBytes(bytes);
    try std.testing.expectEqualSlices(u8, &bytes, &el.toBytes());
}

test "Fp12.isZero / eql are component-wise" {
    try std.testing.expect(Fp12.zero.isZero());
    try std.testing.expect(!Fp12.one.isZero());
    try std.testing.expect(Fp12.one.eql(Fp12.one));
    try std.testing.expect(!Fp12.one.eql(Fp12.zero));
}

test "nonresidue is the Fp6 element v (c1=1)" {
    try std.testing.expect(nonresidue.c0.isZero());
    try std.testing.expect(nonresidue.c1.eql(Fp2.one));
    try std.testing.expect(nonresidue.c2.isZero());
}

// A fixed, arbitrary test element with all twelve Fp coefficients
// distinct and nonzero.
fn testElement() !Fp12 {
    var out: Fp12 = undefined;
    var k: u64 = 3;
    inline for (.{ &out.c0, &out.c1 }) |c6| {
        inline for (.{ &c6.c0, &c6.c1, &c6.c2 }) |c2| {
            c2.* = .{
                .c0 = try Fp.fromInt(u64, k * 0x1111_1111),
                .c1 = try Fp.fromInt(u64, k * k + 7),
            };
            k += 1;
        }
    }
    return out;
}

// Test-local generic exponentiation over Fp12 — used only for the
// definitional frobenius check below.
fn pow12(a: Fp12, e: []const u8) Fp12 {
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

test "Fp12 ring identities: a + (-a) = 0, square == mul(a,a), w^2 == v" {
    const a = try testElement();
    try std.testing.expect(a.add(a.neg()).isZero());
    try std.testing.expect(a.square().eql(a.mul(a)));
    // w^2 must equal v embedded in the c0 (Fp6) component.
    const w = Fp12{ .c0 = Fp6.zero, .c1 = Fp6.one };
    const w2 = w.mul(w);
    try std.testing.expect(w2.c0.eql(nonresidue));
    try std.testing.expect(w2.c1.isZero());
}

test "Fp12.inv: a * a^-1 == 1; inv(0) errors" {
    const a = try testElement();
    try std.testing.expect(a.mul(try a.inv()).eql(Fp12.one));
    try std.testing.expectError(error.NotInvertible, Fp12.zero.inv());
}

test "Fp12.conjugate: conj(a)*a has no w component (lands in Fp6)" {
    const a = try testElement();
    const n = a.conjugate().mul(a);
    try std.testing.expect(n.c1.isZero());
    // conjugation is an involution
    try std.testing.expect(a.conjugate().conjugate().eql(a));
}

test "Fp12.frobenius equals pow(p) (definitional) and has order 12" {
    const a = try testElement();
    try std.testing.expect(a.frobenius().eql(pow12(a, &fp.p_bytes)));
    var x = a;
    for (0..12) |_| x = x.frobenius();
    try std.testing.expect(x.eql(a));
    // frobenius^6 == conjugate on Fp12 (the final-exponentiation "easy
    // part" identity from conjugate's doc comment).
    x = a;
    for (0..6) |_| x = x.frobenius();
    try std.testing.expect(x.eql(a.conjugate()));
}

// Builds a REAL cyclotomic-subgroup element without any pairing: for any
// unit a, c = conjugate(a) * a^-1 is a^(p^6-1), and d = frobenius^2(c) *
// c is a^((p^6-1)(p^2+1)) — the exact easy-part construction, whose
// output has order dividing p^4 - p^2 + 1 (the cyclotomic subgroup).
// This is the day-one oracle Part 1's cyclotomicSquare stub promised.
fn cyclotomicTestElement() !Fp12 {
    const a = try testElement();
    const c = a.conjugate().mul(try a.inv());
    return c.frobenius().frobenius().mul(c);
}

test "Fp12.cyclotomicSquare == square on cyclotomic-subgroup elements" {
    const d = try cyclotomicTestElement();
    // Sanity: d really is cyclotomic (d * conjugate(d) == 1).
    try std.testing.expect(d.mul(d.conjugate()).eql(Fp12.one));
    // The oracle, iterated a few steps to catch shape-preserving bugs
    // (a wrong formula that happens to agree once would still have to
    // agree on its own outputs, which stay in the subgroup).
    var x = d;
    for (0..3) |_| {
        try std.testing.expect(x.cyclotomicSquare().eql(x.square()));
        x = x.cyclotomicSquare();
    }
    // Trivial subgroup element.
    try std.testing.expect(Fp12.one.cyclotomicSquare().eql(Fp12.one));
}

test "Fp12.frobeniusMap: power 0/1/2/6/12 consistency with frobenius/conjugate" {
    const a = try testElement();
    try std.testing.expect(a.frobeniusMap(0).eql(a));
    try std.testing.expect(a.frobeniusMap(1).eql(a.frobenius()));
    try std.testing.expect(a.frobeniusMap(2).eql(a.frobenius().frobenius()));
    try std.testing.expect(a.frobeniusMap(3).eql(a.frobenius().frobenius().frobenius()));
    // frobenius^6 == conjugate (the same identity the frobenius test
    // above pins), and frobenius^12 == identity — so frobeniusMap
    // reduces its power mod 12 exactly.
    try std.testing.expect(a.frobeniusMap(6).eql(a.conjugate()));
    try std.testing.expect(a.frobeniusMap(12).eql(a));
    try std.testing.expect(a.frobeniusMap(13).eql(a.frobenius()));
}

test "Fp12 Frobenius gamma matches the independently-derived constant" {
    // gamma = xi^((p-1)/6) with
    //   c0 = 0x1904d3bf02bb0667c231beb4202c0d1f0fd603fd3cbd5f4f7b2443d7
    //          84bab9c4f67ea53d63e7813d8d0775ed92235fb8
    //   c1 = 0x00fc3e2b36c4e03288e9e902231f9fb854a14787b6c7b36fec0c8ec9
    //          71f63c5f282d5ac14d6c7ec22cf78a126ddc4af3
    // Independently recomputed with big-integer exponentiation outside
    // this module (see NOTICE); also equals zkcrypto/bls12_381
    // fp12.rs FROBENIUS_COEFF_FP12_C1[1].
    const gamma = fp6mod.nonresidue.pow(&p_minus_1_over_6_bytes);
    var expected_c0: [48]u8 = undefined;
    var expected_c1: [48]u8 = undefined;
    _ = try std.fmt.hexToBytes(&expected_c0, "1904d3bf02bb0667c231beb4202c0d1f0fd603fd3cbd5f4f7b2443d784bab9c4f67ea53d63e7813d8d0775ed92235fb8");
    _ = try std.fmt.hexToBytes(&expected_c1, "00fc3e2b36c4e03288e9e902231f9fb854a14787b6c7b36fec0c8ec971f63c5f282d5ac14d6c7ec22cf78a126ddc4af3");
    try std.testing.expectEqualSlices(u8, &expected_c0, &gamma.c0.toBytes());
    try std.testing.expectEqualSlices(u8, &expected_c1, &gamma.c1.toBytes());
}
