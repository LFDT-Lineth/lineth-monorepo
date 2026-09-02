// SPDX-License-Identifier: MIT
//! `G1` — the prime-order-`r` subgroup of `E(Fp): y^2 = x^3 + 4`, the
//! FIRST of BLS12-381's two pairing groups (`e: G1 x G2 -> Gt`). This is
//! the group BLS signatures (and most other consumers — see
//! `README.md`'s arc) put either the smaller, cheaper-to-verify element
//! in (public keys, typically) or the message-hash element in (signatures
//! themselves), depending on the scheme variant — Part 4's job to decide,
//! not this file's.
//!
//! **Status: implemented.** Complete (branchless-degenerate-case)
//! Jacobian addition, doubling, constant-time double-and-add-always
//! scalar multiplication, on-curve/subgroup checks, cofactor clearing
//! and full (de)serialization including decompression. See `SPEC.md`
//! for the BLS subgroup-check pitfall this module's design is built
//! around, and the constant-time notes there for the ctSelect-based
//! branchless discipline used throughout.

const std = @import("std");
const fp = @import("fp.zig");
const scalarmod = @import("scalar.zig");

pub const Fp = fp.Fp;
pub const Fr = scalarmod.Fr;

/// The curve equation constant: `E: y^2 = x^3 + b`, `b = 4`. REAL:
/// `Fp.fromInt` is pure `std.crypto.ff` delegation (see `fp.zig`).
pub const b: Fp = Fp.fromInt(u8, 4) catch @compileError("bls12_381: bad G1 b constant");

/// `G1`'s cofactor `h1 = #E(Fp) / r`, big-endian, 16 bytes:
/// `0x396c8c005555e1568c00aaab0000aaab`. Used by `clearCofactor`.
///
/// Provenance: independently RE-DERIVED (not merely transcribed) from
/// the defining BLS polynomial family with `z = -0xd201000000010000` —
/// `#E(Fp) = p + 1 - (z+1)`, `h1 = #E(Fp) / r` — and confirmed the
/// division is EXACT (zero remainder). See `NOTICE`'s "Verification
/// performed" section for the recomputation.
pub const cofactor_bytes: [16]u8 = blk: {
    @setEvalBranchQuota(2000);
    var out: [16]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, "396c8c005555e1568c00aaab0000aaab") catch unreachable;
    break :blk out;
};

pub const G1Error = error{
    /// Byte length / flag-bit shape is invalid (e.g. the reserved
    /// combination `compression=0, sort=1`, or a nonzero coordinate
    /// paired with the infinity flag).
    InvalidEncoding,
    /// A coordinate failed `Fp.fromBytes` (non-canonical, i.e. `>= p`).
    InvalidFieldElement,
    /// The decoded `(x, y)` (or, for compressed input, the recovered
    /// `y`) does not satisfy `y^2 = x^3 + b`.
    NotOnCurve,
    /// The point is on the curve `E(Fp)` but not in the order-`r`
    /// subgroup `G1` (see `SPEC.md`'s subgroup-check pitfall note).
    NotInSubgroup,
};

/// An affine `G1` point: `(x, y)`, or the point at infinity
/// (`infinity = true`, in which case `x`/`y` are unspecified — per the
/// ZCash/IETF wire convention, encoded as all-zero, see `toBytesCompressed`/
/// `toBytesUncompressed`).
pub const Affine = struct {
    x: Fp,
    y: Fp,
    infinity: bool = false,

    /// `G1`'s standard fixed generator point (the one every BLS12-381
    /// implementation and spec calls "the" `G1` generator — used as the
    /// base point for all published test vectors this module's siblings
    /// will KAT against in later parts).
    ///
    /// Coordinates (hex, big-endian):
    /// ```
    /// x = 0x17f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171
    ///       bac586c55e83ff97a1aeffb3af00adb22c6bb
    /// y = 0x08b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c0
    ///       4b3edd03cc744a2888ae40caa232946c5e7e1
    /// ```
    ///
    /// Provenance: fetched from TWO independent copies of `py_ecc`
    /// (`ethereum/py_ecc`, MIT) — `optimized_bls12_381/optimized_curve.py`
    /// and `bls12_381/bls12_381_curve.py` — which agreed byte-for-byte,
    /// then INDEPENDENTLY VERIFIED on-curve (`y^2 == x^3+4 mod p`) by
    /// direct computation, not merely trusted. See `NOTICE`'s
    /// "Verification performed" section.
    pub const generator: Affine = .{
        .x = Fp.fromBytes(hexBytes(48, "17f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb")) catch
            @compileError("bls12_381: bad G1 generator x"),
        .y = Fp.fromBytes(hexBytes(48, "08b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e1")) catch
            @compileError("bls12_381: bad G1 generator y"),
    };

    /// The point at infinity (`G1`'s group identity).
    pub const identity: Affine = .{ .x = Fp.zero, .y = Fp.zero, .infinity = true };
};

/// A `G1` point in Jacobian projective coordinates `(X, Y, Z)`,
/// representing the affine point `(X/Z^2, Y/Z^3)` — the working
/// representation for `add`/`double`/`scalarMul` (avoids an `Fp.inv` per
/// operation; only `toAffine` needs one).
pub const Jacobian = struct {
    x: Fp,
    y: Fp,
    z: Fp,

    /// The point at infinity, conventionally `(1, 1, 0)` in Jacobian
    /// coordinates (`Z = 0` is infinity regardless of `X`/`Y`; `(1,1,0)`
    /// is the customary normalized representative). REAL: a plain
    /// comptime struct literal, no arithmetic.
    pub const identity: Jacobian = .{ .x = Fp.one, .y = Fp.one, .z = Fp.zero };

    /// `true` iff this represents the point at infinity. REAL: `Z == 0`
    /// is the defining condition, needing only `Fp.isZero` (already
    /// REAL, `fp.zig`) — no multiplication.
    pub fn isIdentity(self: Jacobian) bool {
        return self.z.isZero();
    }

    /// Lifts an affine point into Jacobian coordinates: `(x, y, 1)`, or
    /// `identity` if `p.infinity`. REAL: mechanical embedding, no field
    /// arithmetic (`Fp.one`/`Fp.zero` are comptime constants, not
    /// computed here).
    pub fn fromAffine(p: Affine) Jacobian {
        if (p.infinity) return identity;
        return .{ .x = p.x, .y = p.y, .z = Fp.one };
    }

    /// Converts back to affine: `(X/Z^2, Y/Z^3)`, or `Affine.identity`
    /// if `isIdentity()`. The `Fp.inv` here inverts a Z coordinate —
    /// public in every current caller (serialization of a public
    /// point); revisit with a blinded/batch inversion if an affine
    /// conversion of a secret point ever appears.
    pub fn toAffine(self: Jacobian) Affine {
        if (self.isIdentity()) return Affine.identity;
        const z_inv = self.z.inv() catch unreachable; // z != 0 checked above
        const z_inv2 = z_inv.square();
        return .{
            .x = self.x.mul(z_inv2),
            .y = self.y.mul(z_inv2).mul(z_inv),
        };
    }

    /// Constant-time select: returns `a` if `cond`, else `b` —
    /// coordinate-wise `Fp.ctSelect`. Used by `add` (branchless
    /// degenerate-case resolution) and `scalarMul` (branchless
    /// double-and-add-always accumulator update).
    fn ctSelect(cond: bool, on_true: Jacobian, on_false: Jacobian) Jacobian {
        return .{
            .x = Fp.ctSelect(cond, on_true.x, on_false.x),
            .y = Fp.ctSelect(cond, on_true.y, on_false.y),
            .z = Fp.ctSelect(cond, on_true.z, on_false.z),
        };
    }

    // ── group arithmetic ────────────────────────────────────────────

    /// Jacobian point addition (general case, `a != b`, neither the
    /// identity). Construction: the standard `add-2007-bl`-family
    /// formula (12M+4S — see e.g. the Bernstein–Lange "Explicit-
    /// Formulas Database" `shortw/jacobian` addition formulas, or any
    /// of the reference implementations cited in `NOTICE`) — MUST
    /// handle the degenerate cases correctly and EXPLICITLY: either
    /// operand being the identity, and `a == b`/`a == -b` (which the
    /// general formula divides by zero on) by delegating to `double`/
    /// returning `identity` respectively. Constant-time with respect to
    /// which case fires is a real concern if either operand can be
    /// secret-dependent (e.g. scalar-mul's accumulator) — see
    /// `SPEC.md`'s threat model.
    pub fn add(a: Jacobian, other: Jacobian) Jacobian {
        // General case: the add-2007-bl formula (Bernstein-Lange EFD,
        // shortw/jacobian-0/addition/add-2007-bl). Degenerate cases are
        // resolved BRANCHLESSLY afterwards via ctSelect (constant-time
        // with respect to which case fired — the accumulator's state in
        // scalarMul is secret-dependent): the general formula's output
        // is garbage when h == 0 or an operand is the identity, but it
        // is fully computed either way and then masked out.
        const z1z1 = a.z.square();
        const z2z2 = other.z.square();
        const ua = a.x.mul(z2z2); // U1
        const ub = other.x.mul(z1z1); // U2
        const sa = a.y.mul(other.z).mul(z2z2); // S1
        const sb = other.y.mul(a.z).mul(z1z1); // S2
        const h = ub.sub(ua); // 0 iff same x (P == Q or P == -Q)
        const s_diff = sb.sub(sa); // 0 (given h == 0) iff P == Q

        const i = h.add(h).square();
        const j = h.mul(i);
        const rr = s_diff.add(s_diff);
        const v = ua.mul(i);
        const x3 = rr.square().sub(j).sub(v.add(v));
        const s1j = sa.mul(j);
        const y3 = rr.mul(v.sub(x3)).sub(s1j.add(s1j));
        const z3 = a.z.add(other.z).square().sub(z1z1).sub(z2z2).mul(h);

        var out: Jacobian = .{ .x = x3, .y = y3, .z = z3 };
        // Degenerate-case resolution, lowest to highest precedence.
        const dbl = a.double();
        const h_zero: u1 = @intFromBool(h.isZero());
        const s_zero: u1 = @intFromBool(s_diff.isZero());
        out = ctSelect((h_zero & (1 - s_zero)) == 1, identity, out); // P == -Q
        out = ctSelect((h_zero & s_zero) == 1, dbl, out); // P == Q
        out = ctSelect(other.isIdentity(), a, out);
        out = ctSelect(a.isIdentity(), other, out);
        return out;
    }

    /// Jacobian point doubling. Construction: the standard
    /// `dbl-2009-l`-family formula (specialized for `a = 0` in `y^2 =
    /// x^3 + a*x + b`, which BLS12-381's `G1`/`G2` both have — same
    /// reference as `add`).
    pub fn double(a: Jacobian) Jacobian {
        // dbl-2009-l (EFD, shortw/jacobian-0/doubling), valid for the
        // a=0 curve coefficient. Exception-free: for the identity
        // (Z = 0) — and for a hypothetical Y = 0 point of order 2 —
        // Z3 = 2*Y1*Z1 = 0, i.e. the result is (correctly) the
        // identity, with no branch needed.
        const xx = a.x.square(); // A
        const yy = a.y.square(); // B
        const yyyy = yy.square(); // C
        const d0 = a.x.add(yy).square().sub(xx).sub(yyyy);
        const d = d0.add(d0); // D = 2((X1+B)^2 - A - C)
        const e = xx.add(xx).add(xx); // E = 3A
        const f = e.square(); // F = E^2
        const x3 = f.sub(d.add(d));
        const c8 = blk: { // 8C
            const c2 = yyyy.add(yyyy);
            const c4 = c2.add(c2);
            break :blk c4.add(c4);
        };
        const y3 = e.mul(d.sub(x3)).sub(c8);
        const yz = a.y.mul(a.z);
        return .{ .x = x3, .y = y3, .z = yz.add(yz) };
    }

    /// Negation: `(X, Y, Z) -> (X, -Y, Z)`.
    pub fn negate(a: Jacobian) Jacobian {
        return .{ .x = a.x, .y = a.y.neg(), .z = a.z };
    }

    /// Scalar multiplication `[s]P`. Construction: constant-time
    /// double-and-add (e.g. a fixed-window or Montgomery-ladder variant
    /// — `s` is very often SECRET here, e.g. a BLS secret key times the
    /// generator, or a share value times a point in threshold BLS,
    /// `README.md`'s Part 6) over `s`'s 32-byte (`Fr.encoded_bytes`)
    /// encoding. Public-scalar call sites (if any downstream) may use a
    /// faster variable-time ladder instead — a caller-side choice, not
    /// this function's; consider exposing a separate `scalarMulPublic`
    /// if/when such a call site materializes, mirroring
    /// `fp.zig`/`scalar.zig`'s `pow`/`powPublic` split.
    pub fn scalarMul(p: Jacobian, s: Fr) Jacobian {
        return scalarMulBytes(p, &s.toBytes());
    }

    /// `[s]P` for an arbitrary-width big-endian scalar byte string —
    /// the shared engine behind `scalarMul` (32-byte `Fr` scalars),
    /// `subgroupCheck` (the 32-byte group order `r` itself, NOT a
    /// canonical `Fr` value) and `clearCofactor` (the 16-byte cofactor
    /// `h1`, not `Fr`-reduced). Constant-time double-and-add-ALWAYS:
    /// every bit performs one `double` and one (complete, branchless)
    /// `add`, and the accumulator update is a `ctSelect` — no
    /// secret-dependent branch or memory access; the only quantity
    /// leaked is `s.len`, which is static at every call site.
    pub fn scalarMulBytes(p: Jacobian, s: []const u8) Jacobian {
        var acc = identity;
        for (s) |byte| {
            var bit: u3 = 7;
            while (true) : (bit -= 1) {
                acc = acc.double();
                const sum = acc.add(p);
                acc = ctSelect((byte >> bit) & 1 == 1, sum, acc);
                if (bit == 0) break;
            }
        }
        return acc;
    }

    /// `true` iff this point satisfies the curve equation `y^2 = x^3 +
    /// b` (Jacobian form: `Y^2 = X^3 + b*Z^6`), OR is the identity
    /// (vacuously on-curve). Needed by BOTH decompression (`recoverY`,
    /// below) and as a defensive re-check anywhere an already-parsed
    /// point re-enters this module from an untrusted boundary. Does
    /// NOT imply subgroup membership — see `subgroupCheck` and
    /// `SPEC.md`'s pitfall note.
    pub fn isOnCurve(self: Jacobian) bool {
        if (self.isIdentity()) return true;
        const z2 = self.z.square();
        const z6 = z2.square().mul(z2);
        const rhs = self.x.square().mul(self.x).add(b.mul(z6));
        return self.y.square().eql(rhs);
    }

    /// `true` iff this point is in the order-`r` SUBGROUP `G1` (not
    /// merely on the curve `E(Fp)`, which has a much larger order
    /// `r * h1` — see `cofactor_bytes`). THE classic BLS pitfall this
    /// module's `SPEC.md` centers its threat model on: skipping this
    /// check on an externally-supplied point lets an attacker submit a
    /// small-subgroup point and force a degenerate/predictable pairing
    /// result (a real, exploited class of bug in early BLS
    /// implementations). Construction: the SIMPLE, always-correct
    /// approach is `scalarMul(self, r).isIdentity()` (one full
    /// scalar-mul by the 255-bit group order); a faster
    /// endomorphism-based check exists for `G1` too (BLS12 curves admit
    /// an efficient endomorphism-based subgroup test — see the curve's
    /// defining `z` parameter and e.g. Bowe's "Faster Subgroup Checks
    /// for BLS12-381" or the certified implementations cited in
    /// `NOTICE`) but is a genuine algorithm choice, not mechanical —
    /// implement the simple `scalarMul`-by-`r` version FIRST and treat
    /// the fast path as a follow-up optimization, not a Part-1
    /// scaffolding concern.
    pub fn subgroupCheck(self: Jacobian) bool {
        // The simple, always-correct form: [r]P == O. Note r itself is
        // NOT a canonical Fr value, so this goes through
        // scalarMulBytes, not scalarMul.
        // TODO: the faster endomorphism-based check (Bowe, "Faster
        // Subgroup Checks for BLS12-381" — see NOTICE) is a deferred
        // optimization, per SPEC.md's Backlog.
        return scalarMulBytes(self, &scalarmod.r_bytes).isIdentity();
    }

    /// Multiplies an arbitrary `E(Fp)` point by the cofactor `h1`
    /// (`cofactor_bytes`) to land it in the order-`r` subgroup `G1` —
    /// the usual step after hashing an arbitrary message onto the curve
    /// (Part 3's hash-to-curve, `README.md`) and BEFORE trusting the
    /// result is a valid `G1` element. Construction: `scalarMul` by
    /// `h1`'s 16-byte value (zero-extended to `Fr`'s width is WRONG
    /// here — `h1` is a cofactor, not reduced mod `r`; this needs a
    /// plain big-integer scalar-mul over `h1`'s own 126-bit value, not
    /// `Fr` arithmetic — consider a separate small-scalar `scalarMul`
    /// overload, or zero-pad `h1` into whatever fixed-width type
    /// `scalarMul`'s eventual implementation settles on).
    pub fn clearCofactor(self: Jacobian) Jacobian {
        // Plain big-integer scalar-mul by h1's own 126-bit value (see
        // doc comment for why zero-extending into Fr would be wrong) —
        // scalarMulBytes takes the raw 16-byte cofactor directly.
        // TODO: the faster Bowe cofactor-clearing endomorphism trick is
        // a deferred optimization, per SPEC.md's Backlog.
        return scalarMulBytes(self, &cofactor_bytes);
    }
};

// ── wire codec (ZCash/IETF BLS12-381 serialization format) ─────────────
//
// Flag bits live in the TOP 3 bits of the very first serialized byte:
// bit 7 = compression flag, bit 6 = infinity flag, bit 5 = sort flag
// (sign of y, compressed encoding only). See `NOTICE` for the spec
// citation and `fp2.zig`'s doc comment for the analogous — and easier to
// get backwards — Fp2 component-order pitfall in `G2`.

const Flags = struct {
    compression: bool,
    infinity: bool,
    sort: bool,
};

/// Extracts the flag bits from a serialized point's first byte and
/// returns them alongside that byte with the flag bits cleared (i.e.
/// the true high byte of the first coordinate). REAL: pure bit
/// manipulation, no field arithmetic.
fn splitFlags(b0: u8) struct { flags: Flags, byte: u8 } {
    return .{
        .flags = .{
            .compression = (b0 & 0x80) != 0,
            .infinity = (b0 & 0x40) != 0,
            .sort = (b0 & 0x20) != 0,
        },
        .byte = b0 & 0x1f,
    };
}

/// Packs flag bits into a coordinate's already-serialized first byte.
/// REAL: pure bit manipulation.
fn packFlags(byte: u8, flags: Flags) u8 {
    var b0 = byte & 0x1f;
    if (flags.compression) b0 |= 0x80;
    if (flags.infinity) b0 |= 0x40;
    if (flags.sort) b0 |= 0x20;
    return b0;
}

/// Uncompressed encoding: 96 bytes, `flags|x (48B) || y (48B)`.
pub const uncompressed_bytes = 2 * Fp.encoded_bytes;

/// Compressed encoding: 48 bytes, `flags|x (48B)` (`y`'s sign is the
/// `sort` flag; its magnitude is recovered on decode via `recoverY`).
pub const compressed_bytes = Fp.encoded_bytes;

/// Serializes `p` in the 96-byte uncompressed form. REAL: mechanical
/// flag-packing plus `Fp.toBytes` (itself REAL) — no curve arithmetic.
pub fn toBytesUncompressed(p: Affine) [uncompressed_bytes]u8 {
    var out: [uncompressed_bytes]u8 = undefined;
    if (p.infinity) {
        @memset(&out, 0);
        out[0] = packFlags(0, .{ .compression = false, .infinity = true, .sort = false });
        return out;
    }
    var x_bytes = p.x.toBytes();
    x_bytes[0] = packFlags(x_bytes[0], .{ .compression = false, .infinity = false, .sort = false });
    out[0..Fp.encoded_bytes].* = x_bytes;
    out[Fp.encoded_bytes..uncompressed_bytes].* = p.y.toBytes();
    return out;
}

/// Serializes `p` in the 48-byte compressed form. REAL: mechanical
/// flag-packing (including the `sort` bit, via `Fp.isLexicographically
/// Largest` — itself REAL, `fp.zig`) plus `Fp.toBytes` — no curve
/// arithmetic, and NO `Fp.sqrt` needed (unlike decoding).
pub fn toBytesCompressed(p: Affine) [compressed_bytes]u8 {
    if (p.infinity) {
        var out: [compressed_bytes]u8 = [_]u8{0} ** compressed_bytes;
        out[0] = packFlags(0, .{ .compression = true, .infinity = true, .sort = false });
        return out;
    }
    var x_bytes = p.x.toBytes();
    x_bytes[0] = packFlags(x_bytes[0], .{
        .compression = true,
        .infinity = false,
        .sort = p.y.isLexicographicallyLargest(),
    });
    return x_bytes;
}

/// Recovers `y` from `x` for compressed decoding: `y = sqrt(x^3 + b)`,
/// then negate if the recovered candidate's sign does not match `sort`.
/// Public-input path (deserialization) — branching on the sqrt result
/// and the sort bit is fine here.
fn recoverY(x: Fp, sort: bool) G1Error!Fp {
    var y = x.square().mul(x).add(b).sqrt() orelse return error.NotOnCurve;
    if (y.isLexicographicallyLargest() != sort) y = y.neg();
    return y;
}

/// Parses the 96-byte uncompressed form, including the on-curve check.
/// Does NOT check subgroup membership — callers that need a validated
/// `G1` element (e.g. anything crossing a network/deserialization
/// boundary) MUST additionally call `Jacobian.subgroupCheck` (see
/// `SPEC.md`).
pub fn fromBytesUncompressed(bytes: [uncompressed_bytes]u8) G1Error!Affine {
    const split = splitFlags(bytes[0]);
    if (split.flags.compression) return error.InvalidEncoding;
    if (split.flags.infinity) {
        // Spec requires the remaining bits/bytes to be all-zero.
        if (split.byte != 0) return error.InvalidEncoding;
        for (bytes[1..]) |byte| if (byte != 0) return error.InvalidEncoding;
        return Affine.identity;
    }
    var x_bytes = bytes[0..Fp.encoded_bytes].*;
    x_bytes[0] = split.byte;
    const x = Fp.fromBytes(x_bytes) catch return error.InvalidFieldElement;
    const y = Fp.fromBytes(bytes[Fp.encoded_bytes..uncompressed_bytes].*) catch return error.InvalidFieldElement;
    const p = Affine{ .x = x, .y = y };
    if (!Jacobian.fromAffine(p).isOnCurve()) return error.NotOnCurve;
    return p;
}

/// Parses the 48-byte compressed form (`y` recovered via `recoverY`,
/// i.e. `Fp.sqrt`). Like `fromBytesUncompressed`, does NOT check
/// subgroup membership.
pub fn fromBytesCompressed(bytes: [compressed_bytes]u8) G1Error!Affine {
    const split = splitFlags(bytes[0]);
    if (!split.flags.compression) return error.InvalidEncoding;
    if (split.flags.infinity) {
        if (split.flags.sort) return error.InvalidEncoding; // reserved combination
        if (split.byte != 0) return error.InvalidEncoding;
        for (bytes[1..]) |byte| if (byte != 0) return error.InvalidEncoding;
        return Affine.identity;
    }
    var x_bytes = bytes;
    x_bytes[0] = split.byte;
    const x = Fp.fromBytes(x_bytes) catch return error.InvalidFieldElement;
    const y = try recoverY(x, split.flags.sort);
    return .{ .x = x, .y = y };
}

fn hexBytes(comptime n: usize, comptime hex: *const [2 * n:0]u8) [n]u8 {
    @setEvalBranchQuota(100_000);
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

// ── tests ────────────────────────────────────────────────────────────────

test "b == 4" {
    try std.testing.expectEqual(@as(u8, 4), b.toBytes()[47]);
}

test "generator components are nonzero (constant sanity; on-curve tested below)" {
    try std.testing.expect(!Affine.generator.x.isZero());
    try std.testing.expect(!Affine.generator.y.isZero());
}

test "Jacobian.identity.isIdentity" {
    try std.testing.expect(Jacobian.identity.isIdentity());
    try std.testing.expect(!Jacobian.fromAffine(Affine.generator).isIdentity());
}

test "cofactor_bytes decodes to the expected 16-byte value" {
    var expected: [16]u8 = undefined;
    _ = try std.fmt.hexToBytes(&expected, "396c8c005555e1568c00aaab0000aaab");
    try std.testing.expectEqualSlices(u8, &expected, &cofactor_bytes);
}

// KAT: the generator's compressed/uncompressed serialization, per the
// ZCash/IETF flag-bit format. Computed independently for this scaffold
// by direct application of the documented encoding rule to the
// (independently verified — see above) generator coordinates; NOT
// copied from any third-party test suite (see NOTICE). Both flag bytes
// start with 0x17/0x97 (uncompressed/compressed) rather than 0x1f/0x9f
// because `x`'s own top bits happen to be `0b00010111` — the flag bits
// are OR'd into, not prepended to, that same byte (see `packFlags`).
const generator_uncompressed_hex =
    "17f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb" ++
    "08b3f481e3aaa0f1a09e30ed741d8ae4fcf5e095d5d00af600db18cb2c04b3edd03cc744a2888ae40caa232946c5e7e1";
const generator_compressed_hex =
    "97f1d3a73197d7942695638c4fa9ac0fc3688c4f9774b905a14e3a3f171bac586c55e83ff97a1aeffb3af00adb22c6bb";

test "generator toBytesUncompressed matches the computed KAT" {
    const expected = hexBytes(uncompressed_bytes, generator_uncompressed_hex);
    try std.testing.expectEqualSlices(u8, &expected, &toBytesUncompressed(Affine.generator));
}

test "generator toBytesCompressed matches the computed KAT (sort bit = 0)" {
    const expected = hexBytes(compressed_bytes, generator_compressed_hex);
    try std.testing.expectEqualSlices(u8, &expected, &toBytesCompressed(Affine.generator));
}

test "identity toBytesUncompressed / toBytesCompressed are all-zero plus the infinity flag" {
    const u = toBytesUncompressed(Affine.identity);
    try std.testing.expectEqual(@as(u8, 0x40), u[0]);
    try std.testing.expect(std.mem.allEqual(u8, u[1..], 0));

    const c = toBytesCompressed(Affine.identity);
    try std.testing.expectEqual(@as(u8, 0xc0), c[0]);
    try std.testing.expect(std.mem.allEqual(u8, c[1..], 0));
}

test "fromBytesUncompressed rejects the compression flag set" {
    var bytes = hexBytes(uncompressed_bytes, generator_uncompressed_hex);
    bytes[0] |= 0x80;
    try std.testing.expectError(error.InvalidEncoding, fromBytesUncompressed(bytes));
}

test "fromBytesCompressed rejects the compression flag unset" {
    var bytes = hexBytes(compressed_bytes, generator_compressed_hex);
    bytes[0] &= 0x7f;
    try std.testing.expectError(error.InvalidEncoding, fromBytesCompressed(bytes));
}

test "fromBytesUncompressed round-trips the generator's flag/coordinate parsing" {
    const bytes = hexBytes(uncompressed_bytes, generator_uncompressed_hex);
    const p = try fromBytesUncompressed(bytes);
    try std.testing.expect(p.x.eql(Affine.generator.x));
    try std.testing.expect(p.y.eql(Affine.generator.y));
}

// ── group arithmetic tests ──────────────────────────────────────────────

fn jacGen() Jacobian {
    return Jacobian.fromAffine(Affine.generator);
}

// Affine equality through toAffine (Jacobian representations of the
// same point differ coordinate-wise).
fn expectSamePoint(lhs: Jacobian, rhs: Jacobian) !void {
    const aa = lhs.toAffine();
    const bb = rhs.toAffine();
    try std.testing.expectEqual(aa.infinity, bb.infinity);
    if (!aa.infinity) {
        try std.testing.expect(aa.x.eql(bb.x));
        try std.testing.expect(aa.y.eql(bb.y));
    }
}

test "G1 generator is on the curve (real isOnCurve)" {
    try std.testing.expect(jacGen().isOnCurve());
    try std.testing.expect(Jacobian.identity.isOnCurve());
    // ... and a corrupted point is not.
    var bad = jacGen();
    bad.x = bad.x.add(Fp.one);
    try std.testing.expect(!bad.isOnCurve());
}

test "G1 group law: identity element, inverses, add/double consistency" {
    const g = jacGen();
    // G + O == G, O + G == G
    try expectSamePoint(g.add(Jacobian.identity), g);
    try expectSamePoint(Jacobian.identity.add(g), g);
    // G + (-G) == O
    try std.testing.expect(g.add(g.negate()).isIdentity());
    // G + G == double(G) (the a == b degenerate case of add)
    try expectSamePoint(g.add(g), g.double());
    // double(O) == O
    try std.testing.expect(Jacobian.identity.double().isIdentity());
    // results stay on the curve
    try std.testing.expect(g.double().isOnCurve());
    try std.testing.expect(g.double().add(g).isOnCurve());
}

test "G1 group law: associativity and commutativity ((G+2G)+4G == G+(2G+4G))" {
    const g = jacGen();
    const g2 = g.double();
    const g4 = g2.double();
    try expectSamePoint(g.add(g2).add(g4), g.add(g2.add(g4)));
    try expectSamePoint(g.add(g2), g2.add(g));
}

// Cross-check vectors: compressed serializations of [2]G and [k]G for
// k = 0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef,
// computed with an INDEPENDENT from-scratch implementation (textbook
// affine-coordinate formulas over Python big integers — a different
// algorithm family from this module's Jacobian/Montgomery arithmetic;
// see NOTICE's "Verification performed").
const two_g_compressed_hex =
    "a572cbea904d67468808c8eb50a9450c9721db309128012543902d0ac358a62ae28f75bb8f1c7c42c39a8c5529bf0f4e";
const k_scalar_hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
const k_g_compressed_hex =
    "86b50179774296419b7e8375118823ddb06940d9a28ea045ab418c7ecbe6da84d416cb55406eec6393db97ac26e38bd4";

test "G1 KAT: [2]G compressed matches the independent cross-check vector" {
    const expected = hexBytes(compressed_bytes, two_g_compressed_hex);
    const two_g = jacGen().double().toAffine();
    try std.testing.expectEqualSlices(u8, &expected, &toBytesCompressed(two_g));
}

test "G1 KAT: [k]G compressed matches the independent cross-check vector" {
    const k = try Fr.fromBytes(hexBytes(32, k_scalar_hex));
    const expected = hexBytes(compressed_bytes, k_g_compressed_hex);
    const kg = jacGen().scalarMul(k).toAffine();
    try std.testing.expectEqualSlices(u8, &expected, &toBytesCompressed(kg));
}

test "G1 scalarMul edge cases: [0]P = O, [1]P = P, [2]P = double(P)" {
    const g = jacGen();
    try std.testing.expect(g.scalarMul(Fr.zero).isIdentity());
    try expectSamePoint(g.scalarMul(Fr.one), g);
    const two = Fr.one.add(Fr.one);
    try expectSamePoint(g.scalarMul(two), g.double());
    // [s]O == O
    try std.testing.expect(Jacobian.identity.scalarMul(two).isIdentity());
}

test "G1 scalarMul distributes: [a+b]G == [a]G + [b]G and [a*b]G == [a]([b]G)" {
    var a_bytes = [_]u8{0} ** 32;
    a_bytes[31] = 0x35;
    a_bytes[16] = 0x9c;
    const sa = try Fr.fromBytes(a_bytes);
    var b_bytes = [_]u8{0} ** 32;
    b_bytes[31] = 0x0b;
    b_bytes[8] = 0x77;
    const sb = try Fr.fromBytes(b_bytes);
    const g = jacGen();
    try expectSamePoint(g.scalarMul(sa.add(sb)), g.scalarMul(sa).add(g.scalarMul(sb)));
    try expectSamePoint(g.scalarMul(sa.mul(sb)), g.scalarMul(sb).scalarMul(sa));
}

test "G1 subgroupCheck: generator passes ([r]G == O), non-subgroup curve point fails" {
    try std.testing.expect(jacGen().subgroupCheck());
    // x = 4 gives an on-curve point that is NOT in the order-r subgroup
    // (verified independently: [r]P != O — see NOTICE). Constructed via
    // decompression to also exercise recoverY on a non-generator x.
    var comp = [_]u8{0} ** compressed_bytes;
    comp[0] = 0x80; // compression flag, sort = 0
    comp[compressed_bytes - 1] = 4;
    const p = try fromBytesCompressed(comp);
    const jac = Jacobian.fromAffine(p);
    try std.testing.expect(jac.isOnCurve());
    try std.testing.expect(!jac.subgroupCheck());

    // clearCofactor([h1]P) lands it in the subgroup (and not at O).
    const cleared = jac.clearCofactor();
    try std.testing.expect(!cleared.isIdentity());
    try std.testing.expect(cleared.isOnCurve());
    try std.testing.expect(cleared.subgroupCheck());
}

test "G1 decompression: generator and [2]G round-trip (sort bit both values)" {
    // Generator: sort = 0 (its y is the lexicographically smaller root).
    const g_bytes = hexBytes(compressed_bytes, generator_compressed_hex);
    const g = try fromBytesCompressed(g_bytes);
    try std.testing.expect(g.x.eql(Affine.generator.x));
    try std.testing.expect(g.y.eql(Affine.generator.y));

    // [2]G: its compressed form has the sort bit SET (first byte 0xa5),
    // exercising the y-negation path in recoverY.
    const two_g_bytes = hexBytes(compressed_bytes, two_g_compressed_hex);
    try std.testing.expectEqual(@as(u8, 0xa5), two_g_bytes[0]);
    const two_g = try fromBytesCompressed(two_g_bytes);
    const expected = jacGen().double().toAffine();
    try std.testing.expect(two_g.x.eql(expected.x));
    try std.testing.expect(two_g.y.eql(expected.y));

    // Round-trip: compress what we decompressed, byte-identical.
    try std.testing.expectEqualSlices(u8, &two_g_bytes, &toBytesCompressed(two_g));
}

test "G1 decompression rejects an x with no square root (not on curve)" {
    // x = 1: 1^3 + 4 = 5 is a quadratic non-residue mod p (verified
    // independently — see NOTICE), so decompression must fail with
    // NotOnCurve.
    var comp = [_]u8{0} ** compressed_bytes;
    comp[0] = 0x80;
    comp[compressed_bytes - 1] = 1;
    try std.testing.expectError(G1Error.NotOnCurve, fromBytesCompressed(comp));
}

test "G1 uncompressed round-trip through Jacobian arithmetic" {
    const g5 = jacGen().double().double().add(jacGen()).toAffine(); // [5]G
    const bytes = toBytesUncompressed(g5);
    const back = try fromBytesUncompressed(bytes);
    try std.testing.expect(back.x.eql(g5.x));
    try std.testing.expect(back.y.eql(g5.y));
}
