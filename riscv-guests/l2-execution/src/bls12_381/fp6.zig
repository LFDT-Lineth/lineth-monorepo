// SPDX-License-Identifier: MIT
//! `Fp6 = Fp2[v] / (v^3 - (u+1))` — the second extension-field tower
//! level, i.e. `v^3 = ξ` where `ξ = u+1 ∈ Fp2` (`fp2.zig`'s
//! `nonresidue`). An element is `c0 + c1*v + c2*v^2`, `c0, c1, c2 ∈
//! Fp2`. `Fp6` has no direct geometric meaning in this module on its
//! own (unlike `Fp2` = `G2`'s coordinate field) — it exists purely as
//! the base of `Fp12` (`fp12.zig`), which IS the pairing's target group
//! `Gt` (`e: G1 x G2 -> Gt ⊂ Fp12*`, Part 2).
//!
//! **Status: implemented** — full `Fp6` arithmetic. The Frobenius
//! coefficients are derived PROGRAMMATICALLY from the verified `p`
//! (`γ_1 = ξ^((p-1)/3)` via `Fp2.pow` with a comptime-derived
//! exponent), never hand-transcribed — see `frobenius`'s doc comment.

const std = @import("std");
const fp = @import("fp.zig");
const fp2mod = @import("fp2.zig");

pub const Fp = fp.Fp;
pub const Fp2 = fp2mod.Fp2;
pub const FpError = fp.FpError;

/// The `Fp6` non-residue `ξ = u + 1 ∈ Fp2` — the value `v^3` is fixed to
/// (see the module doc comment). REAL: a plain comptime `Fp2` literal
/// (`c0 = c1 = 1`), no arithmetic.
pub const nonresidue: Fp2 = .{ .c0 = Fp.one, .c1 = Fp.one };

/// An element of `Fp6`: `c0 + c1*v + c2*v^2`.
pub const Fp6 = struct {
    c0: Fp2,
    c1: Fp2,
    c2: Fp2,

    /// Wire encoding: `c2 (96B, HIGH) || c1 (96B) || c0 (96B, LOW)` —
    /// 288 bytes total. Mirrors `Fp2.encoded_bytes`'s high-to-low
    /// ordering convention (see that file's doc comment for why: the
    /// ZCash/IETF BLS12-381 convention serializes the highest-degree
    /// coefficient first throughout the tower). `Fp6` itself is never
    /// serialized directly on the wire in this module (no `G1`/`G2`
    /// point has `Fp6` coordinates) — this encoding exists for
    /// completeness/testability of the tower and for any future
    /// KZG/pairing-output serialization (`README.md`'s later parts)
    /// that may need it.
    pub const encoded_bytes = 3 * Fp2.encoded_bytes; // 288

    pub const zero: Fp6 = .{ .c0 = Fp2.zero, .c1 = Fp2.zero, .c2 = Fp2.zero };
    pub const one: Fp6 = .{ .c0 = Fp2.one, .c1 = Fp2.zero, .c2 = Fp2.zero };

    /// Parses `c2 || c1 || c0`. REAL.
    pub fn fromBytes(bytes: [encoded_bytes]u8) FpError!Fp6 {
        const c2 = try Fp2.fromBytes(bytes[0..Fp2.encoded_bytes].*);
        const c1 = try Fp2.fromBytes(bytes[Fp2.encoded_bytes .. 2 * Fp2.encoded_bytes].*);
        const c0 = try Fp2.fromBytes(bytes[2 * Fp2.encoded_bytes .. 3 * Fp2.encoded_bytes].*);
        return .{ .c0 = c0, .c1 = c1, .c2 = c2 };
    }

    /// Serializes `c2 || c1 || c0`. REAL.
    pub fn toBytes(self: Fp6) [encoded_bytes]u8 {
        var out: [encoded_bytes]u8 = undefined;
        out[0..Fp2.encoded_bytes].* = self.c2.toBytes();
        out[Fp2.encoded_bytes .. 2 * Fp2.encoded_bytes].* = self.c1.toBytes();
        out[2 * Fp2.encoded_bytes .. 3 * Fp2.encoded_bytes].* = self.c0.toBytes();
        return out;
    }

    /// REAL: component-wise `Fp2.isZero`.
    pub fn isZero(self: Fp6) bool {
        return self.c0.isZero() and self.c1.isZero() and self.c2.isZero();
    }

    /// REAL: component-wise `Fp2.eql`.
    pub fn eql(a: Fp6, b: Fp6) bool {
        return a.c0.eql(b.c0) and a.c1.eql(b.c1) and a.c2.eql(b.c2);
    }

    // ── field arithmetic ────────────────────────────────────────────────

    /// Component-wise `Fp2.add` (`c0`, `c1`, `c2` independently).
    pub fn add(a: Fp6, b: Fp6) Fp6 {
        return .{ .c0 = a.c0.add(b.c0), .c1 = a.c1.add(b.c1), .c2 = a.c2.add(b.c2) };
    }

    /// Component-wise `Fp2.sub`.
    pub fn sub(a: Fp6, b: Fp6) Fp6 {
        return .{ .c0 = a.c0.sub(b.c0), .c1 = a.c1.sub(b.c1), .c2 = a.c2.sub(b.c2) };
    }

    /// Component-wise `Fp2.neg`.
    pub fn neg(a: Fp6) Fp6 {
        return .{ .c0 = a.c0.neg(), .c1 = a.c1.neg(), .c2 = a.c2.neg() };
    }

    /// Degree-3 extension multiplication over `v^3 = ξ`
    /// (`fp2.zig`/this file's `nonresidue`). The standard construction
    /// (used by every cited BLS12-381 pairing implementation — see
    /// `NOTICE` — e.g. following the schoolbook-with-Karatsuba
    /// reduction from "Multiplication and Squaring on Pairing-Friendly
    /// Fields", Devegili et al.):
    ///
    /// For `a = a0+a1 v+a2 v^2`, `b = b0+b1 v+b2 v^2`:
    /// ```
    /// v0 = a0*b0,  v1 = a1*b1,  v2 = a2*b2   (three Fp2.mul)
    /// c0 = v0 + ξ*((a1+a2)*(b1+b2) - v1 - v2)
    /// c1 = (a0+a1)*(b0+b1) - v0 - v1 + ξ*v2
    /// c2 = (a0+a2)*(b0+b2) - v0 + v1 - v2
    /// ```
    /// (`ξ*x` is `Fp2.mulByNonresidue`, not a general `Fp2.mul`.) This
    /// costs 6 `Fp2.mul` total (vs. 9 for pure schoolbook).
    pub fn mul(a: Fp6, b: Fp6) Fp6 {
        // The 6-Fp2.mul Karatsuba-style formula from the doc comment
        // (Devegili et al.).
        const v0 = a.c0.mul(b.c0);
        const v1 = a.c1.mul(b.c1);
        const v2 = a.c2.mul(b.c2);
        return .{
            .c0 = a.c1.add(a.c2).mul(b.c1.add(b.c2)).sub(v1).sub(v2).mulByNonresidue().add(v0),
            .c1 = a.c0.add(a.c1).mul(b.c0.add(b.c1)).sub(v0).sub(v1).add(v2.mulByNonresidue()),
            .c2 = a.c0.add(a.c2).mul(b.c0.add(b.c2)).sub(v0).add(v1).sub(v2),
        };
    }

    /// `Fp6` squaring — a dedicated formula (fewer `Fp2.mul`/`Fp2.square`
    /// calls than `mul(a,a)`; e.g. the CH-SQR2 method from the same
    /// Devegili et al. reference cited in `mul`'s doc comment) rather
    /// than delegating to `mul`.
    pub fn square(a: Fp6) Fp6 {
        // CH-SQR2 (Devegili et al., cited in mul's doc comment):
        //   s0 = a0^2, s1 = 2 a0 a1, s2 = (a0 - a1 + a2)^2,
        //   s3 = 2 a1 a2, s4 = a2^2
        //   c0 = s0 + xi*s3, c1 = s1 + xi*s4, c2 = s1 + s2 + s3 - s0 - s4
        const s0 = a.c0.square();
        const ab = a.c0.mul(a.c1);
        const s1 = ab.add(ab);
        const s2 = a.c0.sub(a.c1).add(a.c2).square();
        const bc = a.c1.mul(a.c2);
        const s3 = bc.add(bc);
        const s4 = a.c2.square();
        return .{
            .c0 = s3.mulByNonresidue().add(s0),
            .c1 = s4.mulByNonresidue().add(s1),
            .c2 = s1.add(s2).add(s3).sub(s0).sub(s4),
        };
    }

    /// Multiplicative inverse. Construction: the same "compute the norm
    /// down to a smaller field, invert there, scale back up" pattern as
    /// `Fp2.inv`, one level up — see e.g. Devegili et al. (cited in
    /// `mul`'s doc comment) for the closed-form `Fp6` inversion formula
    /// (it needs one `Fp2.inv` plus several `Fp2.mul`s, not three
    /// independent `Fp2.inv` calls).
    pub fn inv(a: Fp6) error{NotInvertible}!Fp6 {
        // Norm-down-to-Fp2 trick (Devegili et al., cited in mul's doc
        // comment; same closed form as zkcrypto/bls12_381's Fp6::invert
        // — see NOTICE):
        //   c0 = a0^2 - xi*(a1*a2), c1 = xi*a2^2 - a0*a1, c2 = a1^2 - a0*a2
        //   t  = xi*(a1*c2 + a2*c1) + a0*c0     (the Fp2-valued norm)
        //   a^-1 = (c0/t, c1/t, c2/t)
        const c0 = a.c0.square().sub(a.c1.mul(a.c2).mulByNonresidue());
        const c1 = a.c2.square().mulByNonresidue().sub(a.c0.mul(a.c1));
        const c2 = a.c1.square().sub(a.c0.mul(a.c2));
        const t = a.c1.mul(c2).add(a.c2.mul(c1)).mulByNonresidue().add(a.c0.mul(c0));
        const t_inv = try t.inv(); // t == 0 iff a == 0 (norm is multiplicative)
        return .{ .c0 = c0.mul(t_inv), .c1 = c1.mul(t_inv), .c2 = c2.mul(t_inv) };
    }

    /// Multiplication by the `Fp12` non-residue `v` (`fp12.zig`'s
    /// `nonresidue`) — i.e. `(c0+c1 v+c2 v^2) * v = c2*ξ + c0*v + c1*v^2`
    /// (using `v^3 = ξ`, so `c2*v^3 = c2*ξ`). A cyclic coefficient
    /// rotation plus one `Fp2.mulByNonresidue` — cheap, and called
    /// heavily by `Fp12.mul`/`Fp12.square`, same role as
    /// `Fp2.mulByNonresidue` one tower level down.
    pub fn mulByNonresidue(a: Fp6) Fp6 {
        // (c0 + c1 v + c2 v^2) * v = xi*c2 + c0 v + c1 v^2 (v^3 = xi).
        return .{ .c0 = a.c2.mulByNonresidue(), .c1 = a.c0, .c2 = a.c1 };
    }

    /// Scales every `Fp2` coefficient by a single `Fp2` value —
    /// `(c0 + c1 v + c2 v^2) * s` for `s ∈ Fp2 ⊂ Fp6`. Cheaper than a
    /// general `Fp6.mul` by an embedded `Fp2` element (3 `Fp2.mul` vs
    /// 6); needed by `fp12.zig`'s Frobenius (its `w^p` coefficient is
    /// an `Fp2` value scaling an `Fp6` component).
    pub fn mulByFp2(a: Fp6, s: Fp2) Fp6 {
        return .{ .c0 = a.c0.mul(s), .c1 = a.c1.mul(s), .c2 = a.c2.mul(s) };
    }

    /// The Frobenius endomorphism `x -> x^p` restricted to `Fp6`: for
    /// `a = c0+c1 v+c2 v^2`, `a^p = c0^p + c1^p v^p + c2^p v^(2p)` —
    /// `c_i^p` is `Fp2.frobenius(c_i)` (conjugation, `fp2.zig`), and
    /// `v^p`/`v^(2p)` reduce to `γ_1 = ξ^((p-1)/3)` and `γ_2 = γ_1^2`
    /// (standard notation, e.g. Devegili et al. / Aranha–Karabina-style
    /// pairing papers): `a^p = Fp2.frobenius(c0) + γ_1*Fp2.frobenius(c1)*v
    /// + γ_2*Fp2.frobenius(c2)*v^2`.
    ///
    /// The Frobenius coefficients are computed PROGRAMMATICALLY (the
    /// scaffold's recommended option (a)): `γ_1 = ξ^((p-1)/3)` via
    /// `Fp2.pow` with a comptime-derived exponent (`fp.pExponentBytes` —
    /// exactness of the division by 3 is a comptime assertion), and
    /// `γ_2 = γ_1^2`. No hand-transcribed constant table exists to get
    /// wrong; a derivation bug breaks the "frobenius == pow(p)"
    /// definitional test below immediately. The coefficients are
    /// PRECOMPUTED ONCE at comptime (see `frobenius_gamma_1`) — the
    /// pairing's final exponentiation applies Frobenius a dozen-plus
    /// times, so re-deriving a 381-bit `pow` per call was a large chunk
    /// of the whole pairing cost; now `frobenius` is just two `Fp2.mul`s.
    pub fn frobenius(a: Fp6) Fp6 {
        return .{
            .c0 = a.c0.frobenius(),
            .c1 = a.c1.frobenius().mul(frobenius_gamma_1),
            .c2 = a.c2.frobenius().mul(frobenius_gamma_2),
        };
    }
};

/// `(p-1)/3`, big-endian — the `Fp6` Frobenius-coefficient exponent
/// (comptime-derived; `p ≡ 1 (mod 3)`, so the division is exact).
const p_minus_1_over_3_bytes: [48]u8 = fp.pExponentBytes(-1, 3);

/// `γ_1 = ξ^((p-1)/3)` and `γ_2 = γ_1^2` — the `v^p`/`v^(2p)` reduction
/// coefficients (see `Fp6.frobenius`). PRECOMPUTED ONCE at COMPTIME from
/// the tower's own `ξ` and the comptime-derived exponent (the
/// fixed-public `Fp2.pow` is evaluated by the compiler, not per call),
/// so `Fp6.frobenius` costs only two `Fp2.mul`s at runtime instead of
/// re-deriving a 381-bit `pow` on every call. Still derived, never
/// hand-transcribed: the "frobenius == pow(p)" definitional test and the
/// byte-exact `γ_1` KAT below remain the anchors, and the field ops ride
/// `std.crypto.ff`, which is comptime-evaluable.
const frobenius_gamma_1: Fp2 = blk: {
    @setEvalBranchQuota(50_000_000);
    break :blk nonresidue.pow(&p_minus_1_over_3_bytes);
};
const frobenius_gamma_2: Fp2 = blk: {
    @setEvalBranchQuota(50_000_000);
    break :blk frobenius_gamma_1.square();
};

/// Test/back-compat accessor for the comptime-precomputed `γ_1`.
fn frobeniusGamma1() Fp2 {
    return frobenius_gamma_1;
}

// ── tests ────────────────────────────────────────────────────────────────

test "Fp6.zero / Fp6.one round-trip through bytes" {
    const z = Fp6.zero.toBytes();
    try std.testing.expect(std.mem.allEqual(u8, &z, 0));

    const o = Fp6.one.toBytes();
    var expected = [_]u8{0} ** Fp6.encoded_bytes;
    expected[Fp6.encoded_bytes - 1] = 1; // c0's Fp2 low byte
    try std.testing.expectEqualSlices(u8, &expected, &o);
}

test "Fp6.fromBytes / toBytes round-trip" {
    var bytes = [_]u8{0} ** Fp6.encoded_bytes;
    bytes[0] = 0x11; // top byte of c2
    bytes[Fp6.encoded_bytes - 1] = 0x22; // low byte of c0
    const el = try Fp6.fromBytes(bytes);
    try std.testing.expectEqualSlices(u8, &bytes, &el.toBytes());
}

test "Fp6.isZero / eql are component-wise" {
    try std.testing.expect(Fp6.zero.isZero());
    try std.testing.expect(!Fp6.one.isZero());
    try std.testing.expect(Fp6.one.eql(Fp6.one));
    try std.testing.expect(!Fp6.one.eql(Fp6.zero));
}

test "nonresidue is u+1" {
    try std.testing.expect(nonresidue.c0.eql(Fp.one));
    try std.testing.expect(nonresidue.c1.eql(Fp.one));
}

// A fixed, arbitrary test element with all six Fp coefficients distinct
// and nonzero.
fn testElement() !Fp6 {
    return .{
        .c0 = .{ .c0 = try Fp.fromInt(u64, 0x1111_2222_3333_4444), .c1 = try Fp.fromInt(u64, 5) },
        .c1 = .{ .c0 = try Fp.fromInt(u64, 0x5555_6666_7777_8888), .c1 = try Fp.fromInt(u64, 7) },
        .c2 = .{ .c0 = try Fp.fromInt(u64, 0x9999_aaaa_bbbb_cccc), .c1 = try Fp.fromInt(u64, 11) },
    };
}

// Test-local generic exponentiation over Fp6 (square-and-multiply) —
// used only for the definitional frobenius check below.
fn pow6(a: Fp6, e: []const u8) Fp6 {
    var acc = Fp6.one;
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

test "Fp6 ring identities: a + (-a) = 0, square == mul(a,a), (a+b)^2 law" {
    const a = try testElement();
    const b = Fp6{ .c0 = Fp2.one, .c1 = nonresidue, .c2 = Fp2.one };
    try std.testing.expect(a.add(a.neg()).isZero());
    try std.testing.expect(a.square().eql(a.mul(a)));
    const two_ab = a.mul(b).add(a.mul(b));
    try std.testing.expect(a.add(b).square().eql(a.square().add(two_ab).add(b.square())));
}

test "Fp6.inv: a * a^-1 == 1; inv(0) errors" {
    const a = try testElement();
    try std.testing.expect(a.mul(try a.inv()).eql(Fp6.one));
    try std.testing.expectError(error.NotInvertible, Fp6.zero.inv());
}

test "Fp6.mulByNonresidue == mul by v; v^3 == xi (the defining relation)" {
    const a = try testElement();
    const v = Fp6{ .c0 = Fp2.zero, .c1 = Fp2.one, .c2 = Fp2.zero };
    try std.testing.expect(a.mulByNonresidue().eql(a.mul(v)));
    // v^3 must equal xi = u+1 embedded in Fp6's constant coefficient.
    const v3 = v.mul(v).mul(v);
    try std.testing.expect(v3.c0.eql(nonresidue));
    try std.testing.expect(v3.c1.isZero());
    try std.testing.expect(v3.c2.isZero());
}

test "Fp6.mulByFp2 matches mul by the embedded Fp2 element" {
    const a = try testElement();
    const s = Fp2{ .c0 = try Fp.fromInt(u64, 13), .c1 = try Fp.fromInt(u64, 17) };
    const s6 = Fp6{ .c0 = s, .c1 = Fp2.zero, .c2 = Fp2.zero };
    try std.testing.expect(a.mulByFp2(s).eql(a.mul(s6)));
}

test "Fp6.frobenius equals pow(p) (definitional) and has order 6" {
    const a = try testElement();
    // x -> x^p IS the Frobenius endomorphism by definition — this
    // validates the programmatically-derived gamma_1/gamma_2 end-to-end
    // against a literal a^p computed with nothing but mul/square.
    try std.testing.expect(a.frobenius().eql(pow6(a, &fp.p_bytes)));
    // frobenius^6 = identity on Fp6 ([Fp6:Fp] = 6).
    var x = a;
    for (0..6) |_| x = x.frobenius();
    try std.testing.expect(x.eql(a));
    // ... and no smaller power fixes a generic element (frobenius^k for
    // k in 1..5 moves our test element).
    x = a;
    for (0..5) |_| {
        x = x.frobenius();
        try std.testing.expect(!x.eql(a));
    }
}

test "Fp6 Frobenius gamma_1 matches the independently-derived constant" {
    // gamma_1 = xi^((p-1)/3) = 0 + c1*u with
    // c1 = 0x1a0111ea397fe699ec02408663d4de85aa0d857d89759ad4897d2965
    //        0fb85f9b409427eb4f49fffd8bfd00000000aaac
    // Independently recomputed with big-integer exponentiation outside
    // this module (see NOTICE's "Verification performed"); also equals
    // zkcrypto/bls12_381 fp6.rs FROBENIUS_COEFF_FP6_C1[1].
    const g = frobeniusGamma1();
    try std.testing.expect(g.c0.isZero());
    var expected_c1: [48]u8 = undefined;
    _ = try std.fmt.hexToBytes(&expected_c1, "1a0111ea397fe699ec02408663d4de85aa0d857d89759ad4897d29650fb85f9b409427eb4f49fffd8bfd00000000aaac");
    try std.testing.expectEqualSlices(u8, &expected_c1, &g.c1.toBytes());
}
