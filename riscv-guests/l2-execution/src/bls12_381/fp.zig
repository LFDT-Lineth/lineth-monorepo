// SPDX-License-Identifier: MIT
//! `Fp` — the BLS12-381 base field, `GF(p)` for the 381-bit prime `p`
//! (decimal/hex value below). Every other type in this module (`Fp2`,
//! `Fp6`, `Fp12`, and the `G1`/`G2` curve point coordinates) is built as a
//! tower on top of this single field.
//!
//! **Status: implemented — Montgomery-resident.** Values are stored in
//! the **Montgomery domain** (`a·R mod p`, `R = 2^384`) as six full
//! `2^64` little-endian limbs, and stay in that domain across every
//! `mul`/`square`/`add`/`sub`/`neg`/`inv`. The hot path is a portable
//! constant-time **CIOS Montgomery multiply** (Koç et al.) with a
//! **dedicated SOS squaring** (each off-diagonal `a[i]·a[j]` product
//! computed once and doubled) — structurally the same 6×64-limb layout as
//! the sibling `bn254/src/field.zig` (`1892c81`, 4×64) and `ed448`, just
//! L = 6 for the 381-bit prime. BLS12-381's `p` is NOT a special-form
//! Solinas prime, so the reduction is GENERIC Montgomery (`n0inv =
//! -p[0]^{-1} mod 2^64` + the Montgomery constant `R²`), not a
//! curve-specific fold.
//!
//! This replaces the previous `std.crypto.ff`-backed field, which kept
//! elements canonical (non-Montgomery) at rest and paid a full Montgomery
//! round-trip (2×toMontgomery + montMul + fromMontgomery) on every
//! multiply over 63-bit redundant limbs (measured `Fp.mul` ≈ 1259 ns; see
//! `audit/modules/bls12_381.md` finding F2). The Montgomery-resident field
//! below removes both costs. The public API shape (method names /
//! signatures) is unchanged, so the whole `Fp2`/`Fp6`/`Fp12` tower and the
//! `G1`/`G2`/pairing/hash-to-curve/kzg/bls_sig code above it is untouched
//! — the single exception is `hash_to_curve.zig`'s wide reduction, which
//! moves from a now-removed direct `fp.modulus.reduce(..)` call to the new
//! public `Fp.reduceWide` (same math).
//!
//! Constant-time: `mul`/`square`/`add`/`sub`/`neg` are branch-free
//! (masked conditional subtract/add, with an optimization barrier so LLVM
//! cannot recover the carry/borrow bit and emit a data-dependent branch)
//! — the same guarantee `ff` gave, required by the secret-scalar signing
//! paths (`bls_sig` `[sk]G`, `threshold` shares, and the `bbs`/`coconut`
//! secret witnesses that ride this field). `pow`/`inv`/`sqrt` use a
//! **public-exponent** square-and-multiply (constant-time in the base;
//! every call site here supplies a fixed public exponent — Fermat `p-2`,
//! the sqrt exponent, or a comptime Frobenius exponent — matching how
//! `ff`'s public-exponent variants were used, per audit A6).

const std = @import("std");

/// Number of bits reserved for the fixed-width big-integer container
/// backing `Fp` — 384 (48 bytes), the next byte-aligned width above `p`'s
/// actual 381 bits. Retained (unchanged value) for the sibling
/// `scalar.zig` cross-reference and the `modulus.bits() == 381` test.
pub const modulus_bits = 384;

/// Number of full `2^64` limbs backing an `Fp` element (`381 → 6`).
const L: usize = 6;
/// An `Fp` element's limb array (little-endian, Montgomery domain).
const Limbs = [L]u64;

/// The `std.crypto.ff` modulus for `p` — kept ONLY as a comptime object
/// for the `modulus.bits()` test (`root.zig`) and as the correctness
/// ORACLE the Montgomery arithmetic is differentially tested against (see
/// the "differential vs the old ff path" test at the bottom). NO runtime
/// field arithmetic routes through it any more.
const FfModulus = std.crypto.ff.Modulus(modulus_bits);
const FfFe = FfModulus.Fe;

/// Parses a fixed-length compile-time hex string into bytes. Shared
/// comptime helper — used only for the module's own embedded constants
/// below, not exposed as public API.
fn hexBytes(comptime n: usize, comptime hex: *const [2 * n:0]u8) [n]u8 {
    @setEvalBranchQuota(100_000);
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

/// The BLS12-381 base field modulus `p`, big-endian, 48 bytes:
///
/// ```
/// p = 0x1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6
///       241eabfffeb153ffffb9feffffffffaaab
/// ```
///
/// Source: draft-irtf-cfrg-pairing-friendly-curves (the BLS12-381
/// parameter set) and every BLS12-381 reference implementation cited in
/// `NOTICE` agree on this value byte-for-byte; independently re-derived
/// here from the defining BLS polynomial family with parameter
/// `z = -0xd201000000010000` — `p(z) = (z-1)^2 * (z^4 - z^2 + 1) / 3 + z`
/// — and confirmed to match (see `NOTICE`'s "Verification performed"
/// section for the exact recomputation).
pub const p_bytes: [48]u8 = hexBytes(48, "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaaab");

/// `(p-1)/2`, big-endian, 48 bytes — the fixed threshold used by
/// `Fp.isLexicographicallyLargest` (an `Fp` element `y` is the
/// "lexicographically largest" of `{y, -y}` iff `y > (p-1)/2`; this is a
/// FIXED per-modulus constant). Value:
/// `0x0d0088f51cbff34d258dd3db21a5d66bb23ba5c279c2895fb39869507b587b1
/// 20f55ffff58a9ffffdcff7fffffffd555` — mechanically `(p-1)/2` (see
/// `NOTICE`'s recomputation note).
pub const half_p_bytes: [48]u8 = hexBytes(48, "0d0088f51cbff34d258dd3db21a5d66bb23ba5c279c2895fb39869507b587b120f55ffff58a9ffffdcff7fffffffd555");

/// The BLS12-381 base field modulus as a `std.crypto.ff.Modulus(384)` —
/// comptime-only (test oracle + `bits()` check), see `FfModulus` note.
pub const modulus: FfModulus = blk: {
    @setEvalBranchQuota(100_000);
    break :blk FfModulus.fromBytes(&p_bytes, .big) catch
        @compileError("bls12_381: malformed base field modulus bytes");
};

/// Comptime helper: `(p + delta) / div` as a big-endian 48-byte exponent,
/// asserting the division is EXACT — used to derive every fixed exponent
/// this module's tower needs (`p-2` for `Fp.inv`, `(p+1)/4` for `Fp.sqrt`,
/// `(p-3)/4` and `(p-1)/2` for `Fp2.sqrt`, `(p-1)/3` and `(p-1)/6` for the
/// `Fp6`/`Fp12` Frobenius coefficients) directly from the verified
/// `p_bytes`. Pure comptime big-integer arithmetic via `comptime_int` — a
/// wrong `delta`/`div` pair fails the build.
pub fn pExponentBytes(comptime delta: comptime_int, comptime div: comptime_int) [48]u8 {
    @setEvalBranchQuota(100_000);
    comptime var x: comptime_int = 0;
    inline for (p_bytes) |byte| x = x * 256 + @as(comptime_int, byte);
    const v = x + delta;
    if (v % div != 0) @compileError("bls12_381: (p + delta) not divisible by div");
    comptime var q = v / div;
    comptime var out: [48]u8 = undefined;
    comptime var i: usize = 48;
    inline while (i > 0) {
        i -= 1;
        out[i] = q % 256;
        q = q / 256;
    }
    if (q != 0) @compileError("bls12_381: exponent does not fit in 48 bytes");
    return out;
}

/// `p - 2`, big-endian — the Fermat inversion exponent (`Fp.inv`).
const p_minus_2_bytes: [48]u8 = pExponentBytes(-2, 1);

/// `(p + 1) / 4`, big-endian — the square-root exponent for the
/// `p ≡ 3 (mod 4)` simple case (`Fp.sqrt`).
const sqrt_exponent_bytes: [48]u8 = pExponentBytes(1, 4);

/// Errors from `Fp` byte/primitive parsing — kept as the exact
/// `std.crypto.ff` error set the tower's `fromBytes` signatures already
/// propagate (`error.NonCanonical` / `error.Overflow`), so no caller
/// changes even though the backend no longer uses `ff`.
pub const FpError = std.crypto.ff.OverflowError || std.crypto.ff.FieldElementError;

// ── comptime Montgomery constants ─────────────────────────────────────────
//
// Derived once, at comptime, from the verified `p_bytes` via arbitrary-
// precision `comptime_int` — no magic constants transcribed by hand.

/// `p` as a `comptime_int`.
const p_int: comptime_int = blk: {
    var x: comptime_int = 0;
    for (p_bytes) |b| x = x * 256 + @as(comptime_int, b);
    break :blk x;
};

/// Split a nonnegative `comptime_int` into `L` little-endian `u64` limbs.
fn ctToLimbs(comptime x: comptime_int) Limbs {
    comptime var v = x;
    var out: Limbs = undefined;
    inline for (&out) |*w| {
        w.* = @as(u64, @intCast(v & 0xFFFF_FFFF_FFFF_FFFF));
        v = v >> 64;
    }
    if (v != 0) @compileError("bls12_381: value exceeds 6 limbs");
    return out;
}

/// `p` in limb form.
const p_limbs: Limbs = ctToLimbs(p_int);
/// `-p[0]^{-1} mod 2^64` — the CIOS Montgomery reduction constant.
const n0inv: u64 = negInvMod2_64(p_limbs[0]);
/// `R mod p` (`R = 2^384`) — the value `1` in the Montgomery domain.
const one_mont: Limbs = ctToLimbs((1 << 384) % p_int);
/// `R² mod p` — used by `toMont` (`montMul(a, r2) = a·R`).
const r2: Limbs = ctToLimbs(((1 << 384) * (1 << 384)) % p_int);
/// Normal-domain `1`, used by `fromMont` (`montMul(a, 1) = a·R⁻¹`).
const one_normal: Limbs = .{ 1, 0, 0, 0, 0, 0 };
/// `(p-1)/2` in limb form — the `isLexicographicallyLargest` threshold.
const half_p_limbs: Limbs = ctToLimbs((p_int - 1) / 2);

/// `-x⁻¹ mod 2^64` for odd `x`, via Newton–Hensel doubling (6 steps).
fn negInvMod2_64(x: u64) u64 {
    var y: u64 = 1; // x⁻¹ mod 2 (x is odd)
    inline for (0..6) |_| y = y *% (2 -% x *% y);
    return 0 -% y;
}

// ── constant-time barrier ──────────────────────────────────────────────────
//
// Launder a value through an empty inline-asm so LLVM loses all
// range/equality knowledge about it. The masked conditional-subtract/add
// below select on a 0/1 carry/borrow bit via `mask = 0 − bit`; without
// this barrier LLVM can recover `bit ∈ {0,1}` and lower the masked select
// to a data-dependent branch — a secret-dependent branch on the signing
// paths (secret scalars flow through `Fr`, but `Fp` mul/add/sub also carry
// the `bbs`/`coconut` secret witnesses). The `@inComptime()` guard keeps
// it out of the comptime interpreter (the Frobenius γ constants are built
// at comptime through this field, where inline asm cannot run); it is a
// no-op at runtime otherwise.
inline fn blackBox(x: u64) u64 {
    if (@inComptime()) return x;
    return asm volatile (""
        : [ret] "=r" (-> u64),
        : [x] "0" (x),
    );
}

// ── low-level Montgomery arithmetic (portable, constant-time) ──────────────

/// Constant-time conditional subtract of `p` from the `(L+1)`-word value
/// (`v` low `L` words, `top` the carry word 0/1). Subtracts `p` iff the
/// full value ≥ `p`. `v < 2p` on entry, so `top ≤ 1` and the result `< p`.
fn condSubP(v: *Limbs, top: u64) void {
    var diff: Limbs = undefined;
    var borrow: u1 = 0;
    inline for (0..L) |i| {
        const s = @subWithOverflow(v[i], p_limbs[i]);
        const s2 = @subWithOverflow(s[0], borrow);
        diff[i] = s2[0];
        borrow = s[1] | s2[1];
    }
    // full value ≥ p  ⟺  no underflow subtracting `borrow` from `top`.
    const under = @subWithOverflow(top, borrow)[1]; // 1 ⇒ value < p ⇒ keep v
    const keep: u64 = 0 -% blackBox(@as(u64, under));
    inline for (0..L) |i| v[i] = (v[i] & keep) | (diff[i] & ~keep);
}

/// `(a + b) mod p`. `a, b < p ⇒ sum < 2p`, one conditional subtract.
fn addLimbs(a: Limbs, b: Limbs) Limbs {
    var out: Limbs = undefined;
    var carry: u64 = 0;
    inline for (0..L) |i| {
        const s = @as(u128, a[i]) + @as(u128, b[i]) + carry;
        out[i] = @truncate(s);
        carry = @truncate(s >> 64);
    }
    condSubP(&out, carry);
    return out;
}

/// `(a − b) mod p`. On borrow, add `p` back once (masked, constant-time).
fn subLimbs(a: Limbs, b: Limbs) Limbs {
    var out: Limbs = undefined;
    var borrow: u1 = 0;
    inline for (0..L) |i| {
        const s = @subWithOverflow(a[i], b[i]);
        const s2 = @subWithOverflow(s[0], borrow);
        out[i] = s2[0];
        borrow = s[1] | s2[1];
    }
    const mask: u64 = 0 -% blackBox(@as(u64, borrow));
    var carry: u64 = 0;
    inline for (0..L) |i| {
        const s = @as(u128, out[i]) + @as(u128, p_limbs[i] & mask) + carry;
        out[i] = @truncate(s);
        carry = @truncate(s >> 64);
    }
    return out;
}

/// Portable constant-time CIOS Montgomery multiply `z = a·b·R⁻¹ mod p`
/// (Koç et al., Fig. 6; radix 2^64, `u128` widening products). Result is
/// fully reduced (`< p`). No secret-dependent branch/index/early-exit.
fn montMul(a: Limbs, b: Limbs) Limbs {
    var t = [_]u64{0} ** (L + 2);
    inline for (0..L) |i| {
        // t += a * b[i]
        var carry: u64 = 0;
        inline for (0..L) |j| {
            const pr = @as(u128, a[j]) * @as(u128, b[i]) + t[j] + carry;
            t[j] = @truncate(pr);
            carry = @truncate(pr >> 64);
        }
        const s = @as(u128, t[L]) + carry;
        t[L] = @truncate(s);
        t[L + 1] = @truncate(s >> 64);

        // u = t[0] * n0inv mod 2^64 ; t = (t + u*p) / 2^64
        const u = t[0] *% n0inv;
        const p0 = @as(u128, u) * @as(u128, p_limbs[0]) + t[0];
        var carry2: u64 = @truncate(p0 >> 64); // low word is 0 by construction
        inline for (1..L) |j| {
            const pr = @as(u128, u) * @as(u128, p_limbs[j]) + t[j] + carry2;
            t[j - 1] = @truncate(pr);
            carry2 = @truncate(pr >> 64);
        }
        const s2 = @as(u128, t[L]) + carry2;
        t[L - 1] = @truncate(s2);
        t[L] = t[L + 1] +% @as(u64, @truncate(s2 >> 64));
    }
    var z: Limbs = t[0..L].*;
    condSubP(&z, t[L]);
    return z;
}

/// Portable constant-time DEDICATED Montgomery squaring `z = a²·R⁻¹ mod p`
/// (SOS: separated square then Montgomery-reduce). Each off-diagonal
/// `a[i]·a[j]` (`i<j`) product is computed ONCE and doubled — the ~½L²
/// saving over `montMul(a, a)`. Its correctness oracle is `montMul(a, a)`
/// (the differential test below). No secret-dependent branch/index.
fn montSqr(a: Limbs) Limbs {
    // A holds the full 2L-word square across all four phases.
    var A = [_]u64{0} ** (2 * L);

    // 1. cross products: A = Σ_{i<j} a[i]·a[j]·B^(i+j)
    inline for (0..L) |i| {
        var carry: u64 = 0;
        inline for (i + 1..L) |j| {
            const pr = @as(u128, a[i]) * @as(u128, a[j]) + A[i + j] + carry;
            A[i + j] = @truncate(pr);
            carry = @truncate(pr >> 64);
        }
        A[i + L] = carry;
    }
    // 2. double: A ← 2·A  (2·cross < B^(2L), so no word overflows the top)
    var top: u64 = 0;
    inline for (0..2 * L) |k| {
        const nw = (A[k] << 1) | top;
        top = A[k] >> 63;
        A[k] = nw;
    }
    // 3. diagonal: A ← A + Σ_i a[i]²·B^(2i)
    var dcarry: u64 = 0;
    inline for (0..L) |i| {
        const sq = @as(u128, a[i]) * @as(u128, a[i]);
        const p1 = @as(u128, A[2 * i]) + @as(u64, @truncate(sq)) + dcarry;
        A[2 * i] = @truncate(p1);
        const p2 = @as(u128, A[2 * i + 1]) + @as(u64, @truncate(sq >> 64)) + @as(u64, @truncate(p1 >> 64));
        A[2 * i + 1] = @truncate(p2);
        dcarry = @truncate(p2 >> 64);
    }
    // 4. Montgomery reduce: z = A·R⁻¹ mod p
    return montReduce(A);
}

/// Portable constant-time Montgomery reduction `z = A·R⁻¹ mod p` of a
/// `2L`-word value `A < p·R` (single final conditional subtract suffices).
/// Shared by `montSqr`'s phase 4 and `Fp.reduceWide`. No secret-dependent
/// branch/index.
fn montReduce(a_in: [2 * L]u64) Limbs {
    var A = a_in;
    var cc: u64 = 0; // carry into column i+L, threaded between steps
    inline for (0..L) |i| {
        const u = A[i] *% n0inv;
        var carry: u64 = 0;
        inline for (0..L) |j| {
            const pr = @as(u128, u) * @as(u128, p_limbs[j]) + A[i + j] + carry;
            A[i + j] = @truncate(pr);
            carry = @truncate(pr >> 64);
        }
        const s = @as(u128, A[i + L]) + carry + cc;
        A[i + L] = @truncate(s);
        cc = @truncate(s >> 64);
    }
    var z: Limbs = A[L .. 2 * L].*;
    condSubP(&z, cc);
    return z;
}

/// `a·R mod p` (normal → Montgomery domain).
fn toMont(a: Limbs) Limbs {
    return montMul(a, r2);
}

/// `a·R⁻¹ mod p` (Montgomery → normal domain).
fn fromMont(a: Limbs) Limbs {
    return montMul(a, one_normal);
}

/// Read a big-endian 48-byte value into little-endian limbs (no reduction).
fn beToLimbs(bytes: [48]u8) Limbs {
    var out: Limbs = .{ 0, 0, 0, 0, 0, 0 };
    inline for (0..L) |i| {
        const off = 48 - 8 * (i + 1);
        out[i] = std.mem.readInt(u64, bytes[off .. off + 8][0..8], .big);
    }
    return out;
}

/// Write little-endian limbs to a big-endian 48-byte value.
fn limbsToBe(v: Limbs) [48]u8 {
    var out: [48]u8 = undefined;
    inline for (0..L) |i| {
        const off = 48 - 8 * (i + 1);
        std.mem.writeInt(u64, out[off .. off + 8][0..8], v[i], .big);
    }
    return out;
}

/// `v >= p` ? (parse-path canonicality check; on public data.)
fn geP(v: Limbs) bool {
    comptime var i: usize = L;
    inline while (i > 0) {
        i -= 1;
        if (v[i] != p_limbs[i]) return v[i] > p_limbs[i];
    }
    return true; // equal ⇒ ≥ p (reject p itself)
}

/// `a > b` over little-endian limbs (used by the public-data
/// `isLexicographicallyLargest` compression bit).
fn gtLimbs(a: Limbs, b: Limbs) bool {
    comptime var i: usize = L;
    inline while (i > 0) {
        i -= 1;
        if (a[i] != b[i]) return a[i] > b[i];
    }
    return false; // equal ⇒ not strictly greater
}

/// An element of the BLS12-381 base field `GF(p)`, stored in the
/// Montgomery domain as six full `2^64` little-endian limbs (`value·R mod
/// p`), always fully reduced (`< p`) between operations.
pub const Fp = struct {
    limbs: Limbs,

    /// Fixed-size big-endian wire encoding: 48 bytes (matches the
    /// ZCash/IETF BLS12-381 serialization convention's per-coordinate
    /// width — see `g1.zig`/`g2.zig`).
    pub const encoded_bytes = 48;

    /// The additive identity. Montgomery form of `0` is `0`.
    pub const zero: Fp = .{ .limbs = .{ 0, 0, 0, 0, 0, 0 } };

    /// The multiplicative identity — `R mod p` (Montgomery form of `1`).
    pub const one: Fp = .{ .limbs = one_mont };

    /// Parses a big-endian 48-byte value, REJECTING anything `>= p`
    /// (non-canonical), then converts into the Montgomery domain.
    pub fn fromBytes(bytes: [encoded_bytes]u8) FpError!Fp {
        const v = beToLimbs(bytes);
        if (geP(v)) return error.NonCanonical;
        return .{ .limbs = toMont(v) };
    }

    /// Serializes to big-endian 48 bytes (converting out of Montgomery).
    pub fn toBytes(self: Fp) [encoded_bytes]u8 {
        return limbsToBe(fromMont(self.limbs));
    }

    /// Builds a small `Fp` value from a native integer (e.g. the curve
    /// constant `b = 4`). Rejects values `>= p` (`error.Overflow`).
    pub fn fromInt(comptime T: type, v: T) FpError!Fp {
        const uv: u384 = @intCast(v);
        if (uv >= @as(u384, p_int)) return error.Overflow;
        const normal: Limbs = .{
            @truncate(uv),
            @truncate(uv >> 64),
            @truncate(uv >> 128),
            @truncate(uv >> 192),
            @truncate(uv >> 256),
            @truncate(uv >> 320),
        };
        return .{ .limbs = toMont(normal) };
    }

    /// Reduces a wide (`nbytes`-byte, big-endian) integer mod `p` into an
    /// `Fp` element — the `OS2IP(tv) mod p` step of RFC 9380 §5.2's
    /// `hash_to_field`. `nbytes` must be a multiple of 8 and `≤ 2·L·8`
    /// (≤ 96); the input value must be `< p·R` (any hash-to-field wide
    /// input — ≤ 512 bits here — trivially satisfies this). Montgomery
    /// reduce the padded `2L`-word value (`X·R⁻¹`), then two `toMont`
    /// passes yield `(X mod p)·R` — the Montgomery form of `X mod p`.
    pub fn reduceWide(comptime nbytes: usize, bytes: [nbytes]u8) Fp {
        comptime std.debug.assert(nbytes % 8 == 0 and nbytes <= 2 * L * 8);
        var A = [_]u64{0} ** (2 * L);
        inline for (0..nbytes / 8) |i| {
            const off = nbytes - 8 * (i + 1);
            A[i] = std.mem.readInt(u64, bytes[off .. off + 8][0..8], .big);
        }
        const t = montReduce(A); // X·R⁻¹ mod p (< p)
        return .{ .limbs = toMont(toMont(t)) }; // ·R² ⇒ (X mod p)·R
    }

    /// `true` iff the value is `0` (constant-time over the limbs).
    pub fn isZero(self: Fp) bool {
        var acc: u64 = 0;
        inline for (0..L) |i| acc |= self.limbs[i];
        return acc == 0;
    }

    /// Constant-time equality (fully-reduced Montgomery form is unique,
    /// so a limb-wise compare is exact).
    pub fn eql(a: Fp, b: Fp) bool {
        var acc: u64 = 0;
        inline for (0..L) |i| acc |= a.limbs[i] ^ b.limbs[i];
        return acc == 0;
    }

    /// `true` iff `self` is the lexicographically-larger of `{self,
    /// -self}` — i.e. `self > (p-1)/2` as an integer. This is the "sign
    /// of y" bit used by G1/G2 point COMPRESSION (`g1.zig`/`g2.zig`'s
    /// `sort` flag, per the ZCash/IETF serialization format). Operates on
    /// the canonical (normal-domain) value; called only on the PUBLIC `y`
    /// coordinate during (de)compression.
    pub fn isLexicographicallyLargest(self: Fp) bool {
        return gtLimbs(fromMont(self.limbs), half_p_limbs);
    }

    // ── field arithmetic (Montgomery-resident, constant-time) ────────────

    /// `a + b (mod p)`. Constant time.
    pub fn add(a: Fp, b: Fp) Fp {
        return .{ .limbs = addLimbs(a.limbs, b.limbs) };
    }

    /// `a - b (mod p)`. Constant time.
    pub fn sub(a: Fp, b: Fp) Fp {
        return .{ .limbs = subLimbs(a.limbs, b.limbs) };
    }

    /// `-a (mod p)`, i.e. `p - a` (or `0` if `a == 0`). Constant time.
    pub fn neg(a: Fp) Fp {
        return .{ .limbs = subLimbs(Fp.zero.limbs, a.limbs) };
    }

    /// `a * b (mod p)` — CIOS Montgomery multiply. Constant time.
    pub fn mul(a: Fp, b: Fp) Fp {
        return .{ .limbs = montMul(a.limbs, b.limbs) };
    }

    /// `a^2 (mod p)` — dedicated SOS Montgomery squaring. Constant time.
    pub fn square(a: Fp) Fp {
        return .{ .limbs = montSqr(a.limbs) };
    }

    /// Public-exponent square-and-multiply `a^e (mod p)`, `e` big-endian
    /// bytes. Constant time in the BASE `a` (the sq/mul schedule follows
    /// the PUBLIC exponent bits). Shared by `pow`/`inv`/`sqrt`.
    fn powBE(a: Fp, e: []const u8) Fp {
        var result = Fp.one;
        for (e) |byte| {
            var bit: u8 = 0x80;
            while (bit != 0) : (bit >>= 1) {
                result = result.square();
                if (byte & bit != 0) result = result.mul(a);
            }
        }
        return result;
    }

    /// Multiplicative inverse; `error.NotInvertible` if `a == 0`.
    /// Fermat's little theorem, `a^(p-2) mod p` (public exponent).
    pub fn inv(a: Fp) error{NotInvertible}!Fp {
        if (a.isZero()) return error.NotInvertible;
        return a.powBE(&p_minus_2_bytes);
    }

    /// RFC 9380 §4's `inv0`: the zero-preserving multiplicative inverse
    /// (`inv0(0) = 0`, otherwise `a^-1`) — the variant hash-to-curve's
    /// Simplified SWU map (`hash_to_curve.zig`) is specified over.
    pub fn inv0(a: Fp) Fp {
        return a.inv() catch Fp.zero;
    }

    /// RFC 9380 §4.1's `sgn0_m_eq_1`: the "sign" of a base-field element,
    /// defined as the PARITY of its canonical integer representative
    /// (`x mod 2`). NOT the same notion of sign as
    /// `isLexicographicallyLargest` (which compares MAGNITUDE against
    /// `(p-1)/2` for the point-compression sort bit) — do not conflate.
    pub fn sgn0(self: Fp) u1 {
        return @intCast(self.toBytes()[encoded_bytes - 1] & 1);
    }

    /// `a^e (mod p)`, `e` a big-endian byte string. Public-exponent
    /// variant — constant time in the base (every call site here supplies
    /// a fixed public exponent). `e == 0` yields `1` (`a^0 = 1`).
    pub fn pow(a: Fp, e: [encoded_bytes]u8) Fp {
        return a.powBE(&e);
    }

    /// Square root, or `null` if `a` is not a quadratic residue.
    /// `p ≡ 3 (mod 4)` (low byte `0xab`), so the SIMPLE case applies:
    /// candidate `c = a^((p+1)/4)`, verified by `c^2 == a`.
    pub fn sqrt(a: Fp) ?Fp {
        const c = a.powBE(&sqrt_exponent_bytes);
        return if (c.square().eql(a)) c else null;
    }

    /// A uniformly random field element (rejection sampling `< p`; never a
    /// biased `bytes mod p` reduction, which skews the low probability
    /// mass for this "prime just under a byte boundary" field).
    pub fn random(io: std.Io) Fp {
        var buf: [encoded_bytes]u8 = undefined;
        while (true) {
            io.random(&buf);
            return Fp.fromBytes(buf) catch continue; // >= p: reject, redraw
        }
    }

    /// Constant-time select: returns `a` if `cond`, else `b` (masked
    /// limb-wise merge on the Montgomery representation).
    pub fn ctSelect(cond: bool, a: Fp, b: Fp) Fp {
        const mask: u64 = @as(u64, 0) -% @intFromBool(cond);
        var out: Limbs = undefined;
        inline for (0..L) |i| out[i] = (a.limbs[i] & mask) | (b.limbs[i] & ~mask);
        return .{ .limbs = out };
    }
};

// ── tests ────────────────────────────────────────────────────────────────

test "p is odd and 381 bits" {
    try std.testing.expect(p_bytes[47] & 1 == 1);
    try std.testing.expectEqual(@as(usize, 381), modulus.bits());
}

test "Montgomery constants: R·R⁻¹ ≡ 1 and n0inv correct" {
    // p·(-p⁻¹) ≡ -1 (mod 2^64)
    try std.testing.expectEqual(@as(u64, 0), p_limbs[0] *% n0inv +% 1);
    // fromMont(one_mont) == 1 and toMont(1) == one_mont
    try std.testing.expectEqualSlices(u64, &one_normal, &fromMont(one_mont));
    try std.testing.expectEqualSlices(u64, &one_mont, &toMont(one_normal));
}

test "half_p is exactly (p-1)/2" {
    // Cross-check by directly computing (p-1) >> 1 over the raw
    // big-endian byte form — pure byte arithmetic, independent of any Fp
    // method.
    var p_minus_1 = p_bytes;
    var borrow: u1 = 1; // subtract 1 from the whole big-endian value
    var i: usize = p_minus_1.len;
    while (i > 0) {
        i -= 1;
        const r = @subWithOverflow(p_minus_1[i], borrow);
        p_minus_1[i] = r[0];
        borrow = r[1];
        if (borrow == 0) break;
    }

    var shifted = [_]u8{0} ** 48;
    var carry: u8 = 0;
    for (p_minus_1, 0..) |byte, idx| {
        shifted[idx] = (byte >> 1) | (carry << 7);
        carry = byte & 1;
    }

    try std.testing.expectEqualSlices(u8, &half_p_bytes, &shifted);
}

test "Fp.zero / Fp.one round-trip through bytes" {
    const z = Fp.zero.toBytes();
    try std.testing.expect(std.mem.allEqual(u8, &z, 0));
    const o = Fp.one.toBytes();
    var expected = [_]u8{0} ** 48;
    expected[47] = 1;
    try std.testing.expectEqualSlices(u8, &expected, &o);
}

test "Fp.fromBytes rejects p itself (non-canonical) and accepts p-1" {
    try std.testing.expectError(error.NonCanonical, Fp.fromBytes(p_bytes));

    var p_minus_1 = p_bytes;
    p_minus_1[47] -= 1;
    _ = try Fp.fromBytes(p_minus_1); // must not error
}

test "Fp.fromInt(4) matches the b=4 curve-constant byte pattern" {
    const four = try Fp.fromInt(u8, 4);
    var expected = [_]u8{0} ** 48;
    expected[47] = 4;
    try std.testing.expectEqualSlices(u8, &expected, &four.toBytes());
}

test "Fp.isLexicographicallyLargest: half_p+1 is largest, half_p is not, half_p-1 is not" {
    const hp = try Fp.fromBytes(half_p_bytes);
    try std.testing.expect(!hp.isLexicographicallyLargest());
    try std.testing.expect(hp.add(Fp.one).isLexicographicallyLargest());
    try std.testing.expect(!hp.sub(Fp.one).isLexicographicallyLargest());
}

test "Fp arithmetic: additive identities (a + 0 = a, a + (-a) = 0, a - a = 0)" {
    const a = try Fp.fromInt(u64, 0xdead_beef_1234_5678);
    try std.testing.expect(a.add(Fp.zero).eql(a));
    try std.testing.expect(a.add(a.neg()).isZero());
    try std.testing.expect(a.sub(a).isZero());
    try std.testing.expect(Fp.zero.neg().isZero());
}

test "Fp arithmetic: (a + b)^2 == a^2 + 2ab + b^2 and square == mul(a, a)" {
    const a = try Fp.fromInt(u64, 0x0123_4567_89ab_cdef);
    const b = try Fp.fromBytes(half_p_bytes);
    const lhs = a.add(b).square();
    const two_ab = a.mul(b).add(a.mul(b));
    const rhs = a.square().add(two_ab).add(b.square());
    try std.testing.expect(lhs.eql(rhs));
    try std.testing.expect(a.square().eql(a.mul(a)));
}

test "Fp.inv: a * a^-1 == 1; inv(0) errors; inv(1) == 1" {
    const a = try Fp.fromInt(u64, 987_654_321);
    const a_inv = try a.inv();
    try std.testing.expect(a.mul(a_inv).eql(Fp.one));
    try std.testing.expectError(error.NotInvertible, Fp.zero.inv());
    try std.testing.expect((try Fp.one.inv()).eql(Fp.one));
}

test "Fp.inv0 (RFC 9380): inv0(0) == 0, inv0(a) == a^-1 otherwise" {
    try std.testing.expect(Fp.zero.inv0().isZero());
    const a = try Fp.fromInt(u64, 123_456_789);
    try std.testing.expect(a.mul(a.inv0()).eql(Fp.one));
}

test "Fp.sgn0 (RFC 9380 sgn0_m_eq_1): parity of the canonical representative" {
    try std.testing.expectEqual(@as(u1, 0), Fp.zero.sgn0());
    try std.testing.expectEqual(@as(u1, 1), Fp.one.sgn0());
    try std.testing.expectEqual(@as(u1, 0), (try Fp.fromInt(u8, 2)).sgn0());
    // p is odd, so -1 = p-1 is even: sgn0(-1) == 0.
    try std.testing.expectEqual(@as(u1, 0), Fp.one.neg().sgn0());
}

test "Fp.pow: a^0 == 1, a^1 == a, a^2 == square(a)" {
    const a = try Fp.fromInt(u64, 31337);
    var e = [_]u8{0} ** 48;
    try std.testing.expect(a.pow(e).eql(Fp.one));
    e[47] = 1;
    try std.testing.expect(a.pow(e).eql(a));
    e[47] = 2;
    try std.testing.expect(a.pow(e).eql(a.square()));
}

test "Fp.sqrt: sqrt(a^2) in {a, -a}; sqrt(-1) is null (p = 3 mod 4)" {
    const a = try Fp.fromInt(u64, 0xfeed_face_cafe_f00d);
    const c = a.square();
    const s = c.sqrt() orelse return error.TestUnexpectedResult;
    try std.testing.expect(s.eql(a) or s.eql(a.neg()));
    try std.testing.expect(s.square().eql(c));
    // -1 is a quadratic NON-residue exactly when p = 3 (mod 4).
    try std.testing.expect(Fp.one.neg().sqrt() == null);
    // sqrt(0) == 0.
    const z = Fp.zero.sqrt() orelse return error.TestUnexpectedResult;
    try std.testing.expect(z.isZero());
}

test "Fp.ctSelect picks the right operand" {
    const a = try Fp.fromInt(u8, 7);
    const b = try Fp.fromInt(u8, 9);
    try std.testing.expect(Fp.ctSelect(true, a, b).eql(a));
    try std.testing.expect(Fp.ctSelect(false, a, b).eql(b));
}

test "Fp.random produces a canonical, (overwhelmingly) nonzero element" {
    var threaded = std.Io.Threaded.init(std.testing.allocator, .{});
    defer threaded.deinit();
    const io = threaded.io();

    const a = Fp.random(io);
    const b = Fp.random(io);
    _ = try Fp.fromBytes(a.toBytes());
    try std.testing.expect(!a.eql(b));
}

test "pExponentBytes: (p-1)/2 reproduces half_p_bytes and p-2 = p_bytes minus 2" {
    try std.testing.expectEqualSlices(u8, &half_p_bytes, &pExponentBytes(-1, 2));
    var expected = p_bytes;
    expected[47] -= 2; // p's low byte is 0xab, no borrow
    try std.testing.expectEqualSlices(u8, &expected, &p_minus_2_bytes);
}

test "Fp.reduceWide agrees with the ff wide reduction (48- and 64-byte inputs)" {
    var prng = std.Random.DefaultPrng.init(0xB12_381_F1E1D);
    const rand = prng.random();

    var i: usize = 0;
    while (i < 500) : (i += 1) {
        // 64-byte (512-bit) wide input, the hash_to_field width.
        var wide: [64]u8 = undefined;
        rand.bytes(&wide);
        const got = Fp.reduceWide(64, wide);
        const oracle = FfFe.fromBytes(modulus, &limbsToBe(fromMont(got.limbs)), .big) catch unreachable;
        const expect = modulus.reduce(std.crypto.ff.Uint(512).fromBytes(&wide, .big) catch unreachable);
        try std.testing.expect(oracle.eql(expect));
    }
}

// ── differential: Montgomery Fp vs the old std.crypto.ff path ───────────────
//
// The `ff` modulus is retained purely as a comptime correctness ORACLE.
// This test drives thousands of random inputs through BOTH the Montgomery
// arithmetic (the shipped `Fp`) and canonical `ff` arithmetic and asserts
// byte-exact agreement — a direct, local anchor for the field rewrite,
// independent of (and in addition to) the pairing/IETF/eth/c-kzg KATs.

fn ffFromBytes(bytes: [48]u8) FfFe {
    return FfFe.fromBytes(modulus, &bytes, .big) catch unreachable;
}
fn ffToBytes(fe: FfFe) [48]u8 {
    var out: [48]u8 = undefined;
    fe.toBytes(&out, .big) catch unreachable;
    return out;
}

test "differential vs std.crypto.ff: mul/square/add/sub/neg on random inputs" {
    var prng = std.Random.DefaultPrng.init(0xB12_381_54F1);
    const rand = prng.random();

    const draw = struct {
        fn go(r: std.Random) struct { fp: Fp, fe: FfFe } {
            while (true) {
                var b: [48]u8 = undefined;
                r.bytes(&b);
                const f = Fp.fromBytes(b) catch continue;
                return .{ .fp = f, .fe = ffFromBytes(b) };
            }
        }
    }.go;

    var i: usize = 0;
    while (i < 3000) : (i += 1) {
        const a = draw(rand);
        const b = draw(rand);

        try std.testing.expectEqualSlices(u8, &ffToBytes(modulus.mul(a.fe, b.fe)), &a.fp.mul(b.fp).toBytes());
        try std.testing.expectEqualSlices(u8, &ffToBytes(modulus.sq(a.fe)), &a.fp.square().toBytes());
        try std.testing.expectEqualSlices(u8, &ffToBytes(modulus.add(a.fe, b.fe)), &a.fp.add(b.fp).toBytes());
        try std.testing.expectEqualSlices(u8, &ffToBytes(modulus.sub(a.fe, b.fe)), &a.fp.sub(b.fp).toBytes());
        try std.testing.expectEqualSlices(u8, &ffToBytes(modulus.sub(modulus.zero, a.fe)), &a.fp.neg().toBytes());
    }
}
