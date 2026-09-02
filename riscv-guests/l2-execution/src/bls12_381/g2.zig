// SPDX-License-Identifier: MIT
//! `G2` — the prime-order-`r` subgroup of the SEXTIC TWIST
//! `E': y^2 = x^3 + 4(1+u)` over `Fp2` (`fp2.zig`), the SECOND of
//! BLS12-381's two pairing groups (`e: G1 x G2 -> Gt`). Structurally a
//! mirror of `g1.zig` one field-tower level up — see that file's module
//! doc comment for the shared design reasoning (branchless point
//! arithmetic, the wire-codec flag-bit convention) — this file's own
//! doc comments only call out what's DIFFERENT for `G2`.
//!
//! **Status: implemented.** Same construction set as `g1.zig`, over
//! `Fp2`. NOTE: the scaffold's `cofactor_bytes` value was found to be
//! WRONG (it was `#E(Fp2)/r`, not the sextic twist's `#E'(Fp2)/r`) and
//! was corrected during the crypto-core pass — see that constant's doc
//! comment for the full derivation and triple verification.

const std = @import("std");
const fp = @import("fp.zig");
const fp2mod = @import("fp2.zig");
const scalarmod = @import("scalar.zig");

pub const Fp = fp.Fp;
pub const Fp2 = fp2mod.Fp2;
pub const Fr = scalarmod.Fr;

/// The twist curve equation constant: `E': y^2 = x^3 + b'`, `b' =
/// 4(1+u) = 4 + 4u ∈ Fp2`. REAL: `Fp.fromInt` is pure `std.crypto.ff`
/// delegation (see `fp.zig`); no `Fp2.mul` needed since `4*(1+u) =
/// 4+4u` directly, componentwise.
pub const b: Fp2 = .{
    .c0 = Fp.fromInt(u8, 4) catch @compileError("bls12_381: bad G2 b constant (c0)"),
    .c1 = Fp.fromInt(u8, 4) catch @compileError("bls12_381: bad G2 b constant (c1)"),
};

/// `G2`'s cofactor `h2 = #E'(Fp2) / r`, big-endian, 64 bytes.
///
/// Provenance — CORRECTED during the crypto-core pass (the scaffold's
/// original value was wrong): the scaffold derived `#E'(Fp2)` with the
/// QUADRATIC-twist trace formula `p^2 + 1 - t2` (`t2 = t^2 - 2p`,
/// `t = z+1`), but that expression is the order of `E(Fp2)` — the
/// ORIGINAL curve over `Fp2` — not of the SEXTIC twist `E'` this file
/// implements. `#E(Fp2)` happens to also be divisible by `r` (it
/// contains `E(Fp) ⊇ G1`), which made the scaffold's exact-division
/// cross-check pass despite the wrong curve. The sextic twist's order
/// is `#E'(Fp2) = p^2 + 1 - (t2 - 3f)/2` where `f^2 = (4p^2 - t2^2)/3`
/// (the CM equation; the `(t2 + 3f)/2` sign choice gives the OTHER
/// sextic twist, whose order is NOT divisible by `r` — cross-checked).
/// Verified three independent ways: (1) recomputed from the CM formula
/// above; (2) matches the h2 published for BLS12-381 across
/// draft-irtf-cfrg-pairing-friendly-curves-family implementations
/// (kilic/bls12-381 `bls12_381.go`, paulmillr/noble-bls12-381
/// `math.ts` — see `NOTICE`); (3) empirically, `[r]([h2]P) == O` and
/// `[#E']P == O` hold for several non-subgroup `E'(Fp2)` points in an
/// independent big-integer implementation, and BOTH fail for the
/// scaffold's old value (Lagrange's theorem rules it out as a group
/// order divisor). This module's own tests re-verify behaviorally:
/// `clearCofactor` of a non-subgroup point must pass `subgroupCheck`.
pub const cofactor_bytes: [64]u8 = blk: {
    @setEvalBranchQuota(4000);
    var out: [64]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, "05d543a95414e7f1091d50792876a202cd91de4547085abaa68a205b2e5a7ddfa628f1cb4d9e82ef21537e293a6691ae1616ec6e786f0c70cf1c38e31c7238e5") catch unreachable;
    break :blk out;
};

pub const G2Error = error{
    InvalidEncoding,
    InvalidFieldElement,
    NotOnCurve,
    NotInSubgroup,
};

/// An affine `G2` point: `(x, y) ∈ Fp2 x Fp2`, or the point at infinity.
pub const Affine = struct {
    x: Fp2,
    y: Fp2,
    infinity: bool = false,

    /// `G2`'s standard fixed generator point.
    ///
    /// Coordinates (`Fp2` elements, `c0 + c1*u`, hex big-endian):
    /// ```
    /// x.c0 = 0x024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b6
    ///          47ae3d1770bac0326a805bbefd48056c8c121bdb8
    /// x.c1 = 0x13e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61
    ///          bbdc7f5049334cf11213945d57e5ac7d055d042b7e
    /// y.c0 = 0x0ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a
    ///          695160d12c923ac9cc3baca289e193548608b82801
    /// y.c1 = 0x0606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492
    ///          ab572e99ab3f370d275cec1da1aaa9075ff05f79be
    /// ```
    ///
    /// Provenance: same two independent `py_ecc` sources as `g1.zig`'s
    /// generator (agreed byte-for-byte), then INDEPENDENTLY VERIFIED
    /// on-curve (`y^2 == x^3 + 4(1+u) mod p`, computed in `Fp2` with the
    /// `u^2 = -1` convention `fp2.zig` documents) by direct computation.
    /// See `NOTICE`'s "Verification performed" section.
    pub const generator: Affine = .{
        .x = .{
            .c0 = Fp.fromBytes(hexBytes(48, "024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8")) catch
                @compileError("bls12_381: bad G2 generator x.c0"),
            .c1 = Fp.fromBytes(hexBytes(48, "13e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e")) catch
                @compileError("bls12_381: bad G2 generator x.c1"),
        },
        .y = .{
            .c0 = Fp.fromBytes(hexBytes(48, "0ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801")) catch
                @compileError("bls12_381: bad G2 generator y.c0"),
            .c1 = Fp.fromBytes(hexBytes(48, "0606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be")) catch
                @compileError("bls12_381: bad G2 generator y.c1"),
        },
    };

    /// The point at infinity (`G2`'s group identity).
    pub const identity: Affine = .{ .x = Fp2.zero, .y = Fp2.zero, .infinity = true };
};

/// A `G2` point in Jacobian projective coordinates over `Fp2`.
pub const Jacobian = struct {
    x: Fp2,
    y: Fp2,
    z: Fp2,

    pub const identity: Jacobian = .{ .x = Fp2.one, .y = Fp2.one, .z = Fp2.zero };

    /// REAL — see `g1.zig`'s `Jacobian.isIdentity` (identical reasoning,
    /// one tower level up: `Fp2.isZero` is itself REAL, `fp2.zig`).
    pub fn isIdentity(self: Jacobian) bool {
        return self.z.isZero();
    }

    /// REAL — see `g1.zig`'s `Jacobian.fromAffine`.
    pub fn fromAffine(p: Affine) Jacobian {
        if (p.infinity) return identity;
        return .{ .x = p.x, .y = p.y, .z = Fp2.one };
    }

    /// See `g1.zig`'s `Jacobian.toAffine` (identical, over `Fp2`).
    pub fn toAffine(self: Jacobian) Affine {
        if (self.isIdentity()) return Affine.identity;
        const z_inv = self.z.inv() catch unreachable; // z != 0 checked above
        const z_inv2 = z_inv.square();
        return .{
            .x = self.x.mul(z_inv2),
            .y = self.y.mul(z_inv2).mul(z_inv),
        };
    }

    /// Constant-time select over Jacobian points — see `g1.zig`.
    fn ctSelect(cond: bool, on_true: Jacobian, on_false: Jacobian) Jacobian {
        return .{
            .x = Fp2.ctSelect(cond, on_true.x, on_false.x),
            .y = Fp2.ctSelect(cond, on_true.y, on_false.y),
            .z = Fp2.ctSelect(cond, on_true.z, on_false.z),
        };
    }

    // ── group arithmetic ────────────────────────────────────────────
    //
    // Same constructions as g1.zig's Jacobian, over Fp2 instead of Fp —
    // see that file's doc comments for the full formula references and
    // the branchless degenerate-case discipline; not re-quoted here to
    // avoid the two copies drifting out of sync.

    pub fn add(a: Jacobian, other: Jacobian) Jacobian {
        const z1z1 = a.z.square();
        const z2z2 = other.z.square();
        const ua = a.x.mul(z2z2); // U1
        const ub = other.x.mul(z1z1); // U2
        const sa = a.y.mul(other.z).mul(z2z2); // S1
        const sb = other.y.mul(a.z).mul(z1z1); // S2
        const h = ub.sub(ua);
        const s_diff = sb.sub(sa);

        const i = h.add(h).square();
        const j = h.mul(i);
        const rr = s_diff.add(s_diff);
        const v = ua.mul(i);
        const x3 = rr.square().sub(j).sub(v.add(v));
        const s1j = sa.mul(j);
        const y3 = rr.mul(v.sub(x3)).sub(s1j.add(s1j));
        const z3 = a.z.add(other.z).square().sub(z1z1).sub(z2z2).mul(h);

        var out: Jacobian = .{ .x = x3, .y = y3, .z = z3 };
        const dbl = a.double();
        const h_zero: u1 = @intFromBool(h.isZero());
        const s_zero: u1 = @intFromBool(s_diff.isZero());
        out = ctSelect((h_zero & (1 - s_zero)) == 1, identity, out); // P == -Q
        out = ctSelect((h_zero & s_zero) == 1, dbl, out); // P == Q
        out = ctSelect(other.isIdentity(), a, out);
        out = ctSelect(a.isIdentity(), other, out);
        return out;
    }

    pub fn double(a: Jacobian) Jacobian {
        const xx = a.x.square();
        const yy = a.y.square();
        const yyyy = yy.square();
        const d0 = a.x.add(yy).square().sub(xx).sub(yyyy);
        const d = d0.add(d0);
        const e = xx.add(xx).add(xx);
        const f = e.square();
        const x3 = f.sub(d.add(d));
        const c8 = blk: {
            const c2 = yyyy.add(yyyy);
            const c4 = c2.add(c2);
            break :blk c4.add(c4);
        };
        const y3 = e.mul(d.sub(x3)).sub(c8);
        const yz = a.y.mul(a.z);
        return .{ .x = x3, .y = y3, .z = yz.add(yz) };
    }

    pub fn negate(a: Jacobian) Jacobian {
        return .{ .x = a.x, .y = a.y.neg(), .z = a.z };
    }

    /// Same construction as `g1.zig`'s `Jacobian.scalarMul`, over
    /// `Fp2`. `s` is a `G2`-side scalar — in most BLS variants this is
    /// the LESS commonly secret-dependent side (public keys/signatures
    /// tend to put the secret-scalar multiplication on `G1` — Part 4's
    /// job to decide which), but implement this as constant-time
    /// regardless; do not assume a specific scheme variant here.
    pub fn scalarMul(p: Jacobian, s: Fr) Jacobian {
        return scalarMulBytes(p, &s.toBytes());
    }

    /// `[s]P` for an arbitrary-width big-endian scalar byte string —
    /// see `g1.zig`'s `scalarMulBytes` (identical constant-time
    /// double-and-add-always construction, over `Fp2` point
    /// arithmetic).
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

    /// `y^2 = x^3 + b'` (Jacobian: `Y^2 = X^3 + b'*Z^6`) over `Fp2`. See
    /// `g1.zig`'s `Jacobian.isOnCurve`.
    pub fn isOnCurve(self: Jacobian) bool {
        if (self.isIdentity()) return true;
        const z2 = self.z.square();
        const z6 = z2.square().mul(z2);
        const rhs = self.x.square().mul(self.x).add(b.mul(z6));
        return self.y.square().eql(rhs);
    }

    /// Subgroup check for `G2`. Same pitfall/reasoning as `g1.zig`'s
    /// `Jacobian.subgroupCheck` — the simple, always-correct form is
    /// `scalarMul(self, r).isIdentity()`; `G2`'s fast path uses the
    /// UNTWIST-FROBENIUS-TWIST endomorphism (a different, `G2`-specific
    /// technique from `G1`'s — see Bowe's "Faster Subgroup Checks for
    /// BLS12-381", cited in `NOTICE`) — again, a genuine algorithm
    /// choice deferred past Part 1's scaffolding scope; implement the
    /// simple form first.
    pub fn subgroupCheck(self: Jacobian) bool {
        // The simple, always-correct form: [r]P == O (r is not a
        // canonical Fr value — goes through scalarMulBytes).
        // TODO: the untwist-Frobenius-twist fast path (Bowe — see
        // NOTICE) is a deferred optimization, per SPEC.md's Backlog.
        return scalarMulBytes(self, &scalarmod.r_bytes).isIdentity();
    }

    /// Multiplies by the cofactor `h2` (`cofactor_bytes`, 64 bytes) —
    /// see `g1.zig`'s `Jacobian.clearCofactor` for the identical
    /// "not `Fr`-reduced, plain big-integer scalar" caveat, one tower
    /// level up and with a much wider (~636-bit) cofactor here.
    pub fn clearCofactor(self: Jacobian) Jacobian {
        // Plain big-integer scalar-mul by h2's ~636-bit value.
        // TODO: the untwist-Frobenius-twist trick gives a much cheaper
        // cofactor clearing (same Bowe reference as subgroupCheck) —
        // deferred optimization, per SPEC.md's Backlog.
        return scalarMulBytes(self, &cofactor_bytes);
    }
};

// ── wire codec (ZCash/IETF BLS12-381 serialization format) ─────────────
//
// Same flag-bit convention as g1.zig (top 3 bits of the very first
// byte), applied to the HIGH Fp2 component (x.c1) — see that file's
// module doc comment, and fp2.zig's doc comment for the c1||c0 ordering
// this inherits.

const Flags = struct {
    compression: bool,
    infinity: bool,
    sort: bool,
};

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

fn packFlags(byte: u8, flags: Flags) u8 {
    var b0 = byte & 0x1f;
    if (flags.compression) b0 |= 0x80;
    if (flags.infinity) b0 |= 0x40;
    if (flags.sort) b0 |= 0x20;
    return b0;
}

/// Uncompressed encoding: 192 bytes, `flags|x.c1 || x.c0 || y.c1 || y.c0`
/// (48 bytes each).
pub const uncompressed_bytes = 4 * Fp.encoded_bytes;

/// Compressed encoding: 96 bytes, `flags|x.c1 || x.c0` (`y`'s sign is
/// the `sort` flag; magnitude recovered via `recoverY`).
pub const compressed_bytes = 2 * Fp.encoded_bytes;

/// REAL — see `g1.zig`'s `toBytesUncompressed` (identical mechanical
/// shape, `x`/`y` now each 96-byte `Fp2.toBytes()` calls).
pub fn toBytesUncompressed(p: Affine) [uncompressed_bytes]u8 {
    var out: [uncompressed_bytes]u8 = undefined;
    if (p.infinity) {
        @memset(&out, 0);
        out[0] = packFlags(0, .{ .compression = false, .infinity = true, .sort = false });
        return out;
    }
    var x_bytes = p.x.toBytes(); // c1 || c0, 96 bytes (fp2.zig)
    x_bytes[0] = packFlags(x_bytes[0], .{ .compression = false, .infinity = false, .sort = false });
    out[0..Fp2.encoded_bytes].* = x_bytes;
    out[Fp2.encoded_bytes..uncompressed_bytes].* = p.y.toBytes();
    return out;
}

/// REAL — see `g1.zig`'s `toBytesCompressed`. The sort bit uses
/// `Fp2.isLexicographicallyLargest` (REAL, `fp2.zig`).
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

/// Same shape as `g1.zig`'s `recoverY`, over `Fp2` (uses `Fp2.sqrt`,
/// the complex method — see `fp2.zig`). Public-input path.
fn recoverY(x: Fp2, sort: bool) G2Error!Fp2 {
    var y = x.square().mul(x).add(b).sqrt() orelse return error.NotOnCurve;
    if (y.isLexicographicallyLargest() != sort) y = y.neg();
    return y;
}

/// Parses the 192-byte uncompressed form, including the on-curve
/// check. Does NOT check subgroup membership — see `g1.zig`'s
/// `fromBytesUncompressed` doc comment for the identical caveat.
pub fn fromBytesUncompressed(bytes: [uncompressed_bytes]u8) G2Error!Affine {
    const split = splitFlags(bytes[0]);
    if (split.flags.compression) return error.InvalidEncoding;
    if (split.flags.infinity) {
        if (split.byte != 0) return error.InvalidEncoding;
        for (bytes[1..]) |byte| if (byte != 0) return error.InvalidEncoding;
        return Affine.identity;
    }
    var x_bytes = bytes[0..Fp2.encoded_bytes].*;
    x_bytes[0] = split.byte;
    const x = Fp2.fromBytes(x_bytes) catch return error.InvalidFieldElement;
    const y = Fp2.fromBytes(bytes[Fp2.encoded_bytes..uncompressed_bytes].*) catch return error.InvalidFieldElement;
    const p = Affine{ .x = x, .y = y };
    if (!Jacobian.fromAffine(p).isOnCurve()) return error.NotOnCurve;
    return p;
}

/// Parses the 96-byte compressed form. See `g1.zig`'s
/// `fromBytesCompressed` for the identical caveat re: `recoverY`.
pub fn fromBytesCompressed(bytes: [compressed_bytes]u8) G2Error!Affine {
    const split = splitFlags(bytes[0]);
    if (!split.flags.compression) return error.InvalidEncoding;
    if (split.flags.infinity) {
        if (split.flags.sort) return error.InvalidEncoding;
        if (split.byte != 0) return error.InvalidEncoding;
        for (bytes[1..]) |byte| if (byte != 0) return error.InvalidEncoding;
        return Affine.identity;
    }
    var x_bytes = bytes;
    x_bytes[0] = split.byte;
    const x = Fp2.fromBytes(x_bytes) catch return error.InvalidFieldElement;
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

test "b' == 4 + 4u" {
    try std.testing.expectEqual(@as(u8, 4), b.c0.toBytes()[47]);
    try std.testing.expectEqual(@as(u8, 4), b.c1.toBytes()[47]);
}

test "generator components are nonzero" {
    try std.testing.expect(!Affine.generator.x.isZero());
    try std.testing.expect(!Affine.generator.y.isZero());
}

test "Jacobian.identity.isIdentity" {
    try std.testing.expect(Jacobian.identity.isIdentity());
    try std.testing.expect(!Jacobian.fromAffine(Affine.generator).isIdentity());
}

test "cofactor_bytes decodes to the expected (corrected) 64-byte value" {
    // The CORRECTED h2 — matches the published BLS12-381 G2 cofactor
    // (see cofactor_bytes' doc comment for the scaffold-bug story and
    // the CM-formula re-derivation).
    var expected: [64]u8 = undefined;
    _ = try std.fmt.hexToBytes(&expected, "05d543a95414e7f1091d50792876a202cd91de4547085abaa68a205b2e5a7ddfa628f1cb4d9e82ef21537e293a6691ae1616ec6e786f0c70cf1c38e31c7238e5");
    try std.testing.expectEqualSlices(u8, &expected, &cofactor_bytes);
}

// KAT: the generator's compressed/uncompressed serialization, computed
// independently for this scaffold by direct application of the
// documented encoding rule to the (independently verified — see
// `Affine.generator`'s doc comment) generator coordinates. Both flag
// bytes fall in x.c1's own top-bit range the same way g1.zig's do (see
// that file's KAT comment) — not copied from any third-party test
// suite (see `NOTICE`).
const generator_uncompressed_hex =
    "13e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e" ++
    "024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8" ++
    "0606c4a02ea734cc32acd2b02bc28b99cb3e287e85a763af267492ab572e99ab3f370d275cec1da1aaa9075ff05f79be" ++
    "0ce5d527727d6e118cc9cdc6da2e351aadfd9baa8cbdd3a76d429a695160d12c923ac9cc3baca289e193548608b82801";
const generator_compressed_hex =
    "93e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e" ++
    "024aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb8";

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

fn expectSamePoint(lhs: Jacobian, rhs: Jacobian) !void {
    const aa = lhs.toAffine();
    const bb = rhs.toAffine();
    try std.testing.expectEqual(aa.infinity, bb.infinity);
    if (!aa.infinity) {
        try std.testing.expect(aa.x.eql(bb.x));
        try std.testing.expect(aa.y.eql(bb.y));
    }
}

test "G2 generator is on the twist (real isOnCurve)" {
    try std.testing.expect(jacGen().isOnCurve());
    try std.testing.expect(Jacobian.identity.isOnCurve());
    var bad = jacGen();
    bad.x = bad.x.add(Fp2.one);
    try std.testing.expect(!bad.isOnCurve());
}

test "G2 group law: identity element, inverses, add/double consistency, associativity" {
    const g = jacGen();
    try expectSamePoint(g.add(Jacobian.identity), g);
    try expectSamePoint(Jacobian.identity.add(g), g);
    try std.testing.expect(g.add(g.negate()).isIdentity());
    try expectSamePoint(g.add(g), g.double());
    try std.testing.expect(Jacobian.identity.double().isIdentity());
    try std.testing.expect(g.double().isOnCurve());
    const g2p = g.double();
    const g4p = g2p.double();
    try expectSamePoint(g.add(g2p).add(g4p), g.add(g2p.add(g4p)));
    try expectSamePoint(g.add(g2p), g2p.add(g));
}

// Cross-check vectors: compressed serializations of [2]G2 and [k]G2 for
// the same k as g1.zig's KAT, computed with the same INDEPENDENT
// from-scratch affine implementation (see NOTICE).
const two_g_compressed_hex =
    "aa4edef9c1ed7f729f520e47730a124fd70662a904ba1074728114d1031e1572c6c886f6b57ec72a6178288c47c33577" ++
    "1638533957d540a9d2370f17cc7ed5863bc0b995b8825e0ee1ea1e1e4d00dbae81f14b0bf3611b78c952aacab827a053";
const k_scalar_hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";
const k_g_compressed_hex =
    "afc7ac61f71e90fc3f8663602fed1d3602fab2b3248ef8c5cbde7cc6d6ae491f4e88482ad451051224d97b96c60c48a4" ++
    "0ae3f4bcb510f27a4e8a0815b98be6db7a609998618c80d3e20cc30330273313298e134f5bcd27441790472b8b1a62b4";

test "G2 KAT: [2]G2 compressed matches the independent cross-check vector" {
    const expected = hexBytes(compressed_bytes, two_g_compressed_hex);
    const two_g = jacGen().double().toAffine();
    try std.testing.expectEqualSlices(u8, &expected, &toBytesCompressed(two_g));
}

test "G2 KAT: [k]G2 compressed matches the independent cross-check vector" {
    const k = try Fr.fromBytes(hexBytes(32, k_scalar_hex));
    const expected = hexBytes(compressed_bytes, k_g_compressed_hex);
    const kg = jacGen().scalarMul(k).toAffine();
    try std.testing.expectEqualSlices(u8, &expected, &toBytesCompressed(kg));
}

test "G2 scalarMul edge cases and distributivity" {
    const g = jacGen();
    try std.testing.expect(g.scalarMul(Fr.zero).isIdentity());
    try expectSamePoint(g.scalarMul(Fr.one), g);
    const two = Fr.one.add(Fr.one);
    try expectSamePoint(g.scalarMul(two), g.double());
    var a_bytes = [_]u8{0} ** 32;
    a_bytes[31] = 0x2f;
    a_bytes[20] = 0xe1;
    const sa = try Fr.fromBytes(a_bytes);
    try expectSamePoint(g.scalarMul(sa.add(two)), g.scalarMul(sa).add(g.double()));
}

test "G2 subgroupCheck: generator passes, non-subgroup twist point fails; clearCofactor fixes it" {
    try std.testing.expect(jacGen().subgroupCheck());
    // x = u (c0 = 0, c1 = 1) gives an on-twist point NOT in the
    // order-r subgroup (verified independently: [r]P != O — see
    // NOTICE). Constructed via decompression (c1 || c0 wire order).
    var comp = [_]u8{0} ** compressed_bytes;
    comp[0] = 0x80; // compression flag, sort = 0; x.c1's top byte
    comp[Fp.encoded_bytes - 1] = 1; // x.c1 = 1
    const p = try fromBytesCompressed(comp);
    const jac = Jacobian.fromAffine(p);
    try std.testing.expect(jac.isOnCurve());
    try std.testing.expect(!jac.subgroupCheck());

    const cleared = jac.clearCofactor();
    try std.testing.expect(!cleared.isIdentity());
    try std.testing.expect(cleared.isOnCurve());
    try std.testing.expect(cleared.subgroupCheck());
}

test "G2 decompression: generator and [2]G2 round-trip (sort bit both values)" {
    const g_bytes = hexBytes(compressed_bytes, generator_compressed_hex);
    const g = try fromBytesCompressed(g_bytes);
    try std.testing.expect(g.x.eql(Affine.generator.x));
    try std.testing.expect(g.y.eql(Affine.generator.y));

    // [2]G2's compressed form has the sort bit SET (first byte 0xaa),
    // exercising recoverY's negation path over Fp2.
    const two_g_bytes = hexBytes(compressed_bytes, two_g_compressed_hex);
    try std.testing.expectEqual(@as(u8, 0xaa), two_g_bytes[0]);
    const two_g = try fromBytesCompressed(two_g_bytes);
    const expected = jacGen().double().toAffine();
    try std.testing.expect(two_g.x.eql(expected.x));
    try std.testing.expect(two_g.y.eql(expected.y));
    try std.testing.expectEqualSlices(u8, &two_g_bytes, &toBytesCompressed(two_g));
}

test "G2 uncompressed round-trip through Jacobian arithmetic" {
    const g5 = jacGen().double().double().add(jacGen()).toAffine(); // [5]G2
    const bytes = toBytesUncompressed(g5);
    const back = try fromBytesUncompressed(bytes);
    try std.testing.expect(back.x.eql(g5.x));
    try std.testing.expect(back.y.eql(g5.y));
}
