// SPDX-License-Identifier: MIT
//! Hash-to-curve (RFC 9380) for BLS12-381's `G1`/`G2` — Part 3 of this
//! module's multi-part arc (`README.md`). This is the primitive BLS
//! signatures (Part 4) hash a message onto the curve with: given an
//! arbitrary-length message and a domain-separation tag (DST), produce a
//! `G1` or `G2` point statistically close to uniform in the subgroup
//! (`hash_to_curve`, RFC 9380 §3), or a cheaper nonuniform variant
//! (`encode_to_curve`).
//!
//! **Status: IMPLEMENTED (Part-3 crypto-core pass).** Every stage is
//! real and pinned byte-exact against RFC 9380's own official vectors:
//! `expandMessageXmd` (§5.3.1) against Appendix K.1; `hashToFieldFp`/
//! `hashToFieldFp2` (§5.2) against Appendix J.9.1/J.10.1's `u[0]`/
//! `u[1]`; `mapToCurveG1`/`mapToCurveG2` (Simplified SWU §6.6.2 +
//! isogeny map §6.6.3, Appendix E.2/E.3 coefficient tables) against
//! J.9.1/J.10.1's `Q0`/`Q1` intermediates; and the full
//! `hashToCurveG1`/`hashToCurveG2` compositions against J.9.1/J.10.1's
//! final `P` — all 5 published messages each (see the tests below,
//! `SPEC.md`, and `NOTICE`'s Part-3 verification section). The suites:
//!
//!   - `BLS12381G1_XMD:SHA-256_SSWU_RO_` (RFC 9380 §8.8.1): hash-to-field
//!     over `Fp` → Simplified SWU (§6.6.3, for `AB == 0` curves) onto the
//!     11-isogenous curve `E1'` → 11-isogeny map to `G1`'s curve `E` →
//!     `clear_cofactor` by the suite's `h_eff` (see below).
//!   - `BLS12381G2_XMD:SHA-256_SSWU_RO_` (RFC 9380 §8.8.2): the same
//!     shape over `Fp2`, onto the 3-isogenous curve `E2'` → 3-isogeny map
//!     → `clear_cofactor` by that suite's own `h_eff`.
//!
//! **`clear_cofactor` here is `h_eff`-multiplication, NOT Part 1's
//! `clearCofactor`** (which multiplies by the plain cofactor `h`): RFC
//! 9380 §7 defines `clear_cofactor(P) := h_eff * P` with `h_eff` a
//! FIXED suite parameter (§8.8.1: `0xd201000000010001`; §8.8.2: a
//! 636-bit value), and states that when a suite's `h_eff` comes from a
//! fast cofactor-clearing method — both BLS12-381 suites' do (Scott
//! [WB19] for `G1`, Budroni-Pintore [BP17] for `G2`, per the RFC's own
//! notes) — "scalar multiplication by the cofactor h does not
//! generally give the same result as the fast method and MUST NOT be
//! used". Confirmed empirically during this pass (`NOTICE`): `[h]R !=
//! [h_eff]R` for the RFC's own vector points, both groups —
//! multiplying by `h` lands in the right subgroup but on a DIFFERENT
//! point, silently breaking suite conformance (the scaffold's original
//! plan to reuse `g1.Jacobian.clearCofactor` here was therefore wrong
//! and was corrected; `g1`/`g2`'s own `clearCofactor` remains correct
//! for its own purpose — landing arbitrary points in the subgroup — it
//! just is not RFC 9380's `clear_cofactor`).
//!
//! **`expand_message_xmd` reuse decision**: `modules/frost` and
//! `modules/voprf` each already carry an in-module `expand_message_xmd`
//! (RFC 9380 §5.3.1) — frost's specialized to SHA-256/`ell=2`
//! (`len_in_bytes=48`), voprf's to SHA-512/`ell=1` (`len_in_bytes<=64`).
//! Neither is reusable here without either (a) a cross-module dependency
//! (this module would need to import `frost`, adding a `deps` edge to a
//! threshold-signature module for one hash primitive — a layering
//! smell flagged for the orchestrator/owner, NOT done unilaterally) or
//! (b) generalizing frost's SHA-256 variant to arbitrary `ell` (this
//! module needs `ell=4` for `G1`'s `count=2,m=1,L=64` hash_to_field call
//! and `ell=8` for `G2`'s `count=2,m=2,L=64` — both bigger than frost's
//! hardcoded `ell=2`). Given `CONVENTIONS.md`'s zero-cross-module-crypto-
//! dependency posture and this module's own self-contained-clean-room
//! precedent (Parts 1-2 read no third-party source), this file
//! REIMPLEMENTS `expand_message_xmd` from the RFC 9380 §5.3.1 pseudocode
//! directly, generalized to any `ell` up to 255 (frost/voprf's fixed-`ell`
//! versions are each a specialization of this general shape) — this is a
//! deliberate, flagged departure from a literal reuse instruction; see
//! `SPEC.md`'s design record for the full reasoning.
//!
//! Zig std GAP: yes — nothing in `std` supplies RFC 9380 hash-to-curve
//! machinery; `std.crypto.hash.sha2.Sha256` supplies the hash function
//! `expand_message_xmd` is built on, and `std.crypto.ff.Modulus.reduce`
//! (already used by `scalar.zig`'s `Fr.reduceWide`) supplies the
//! wide-integer-to-field reduction `hashToFieldFp`/`hashToFieldFp2` need
//! — both REAL, no new big-integer primitive is implemented here.

const std = @import("std");
const fp = @import("fp.zig");
const fp2mod = @import("fp2.zig");
const g1 = @import("g1.zig");
const g2 = @import("g2.zig");

pub const Fp = fp.Fp;
pub const Fp2 = fp2mod.Fp2;

// ── comptime hex helper (same pattern as fp.zig/g1.zig/g2.zig) ─────────

fn hexBytes(comptime n: usize, comptime hex: *const [2 * n:0]u8) [n]u8 {
    @setEvalBranchQuota(100_000);
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

/// Runtime-friendly variant of `hexBytes` (same shape as `adaptor`'s
/// `kat_test.zig` `hexN` helper): decodes a `[]const u8` hex string
/// (not necessarily a comptime sentinel-terminated literal) — needed
/// for the KAT tests below, which iterate a runtime `for` loop over an
/// array of vector structs whose hex fields are plain `[]const u8`
/// slices, not string literals `hexBytes` could bind at comptime.
fn hexN(comptime n: usize, hex_str: []const u8) [n]u8 {
    var out: [n]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex_str) catch unreachable;
    return out;
}

/// Comptime `Fp` constant from a 96-hex-digit big-endian string — the
/// isogeny coefficient tables below are built from these. A wrong
/// length or a non-canonical (`>= p`) value fails the BUILD, so a
/// corrupted table entry cannot silently ship.
fn fpc(comptime hex: *const [96:0]u8) Fp {
    return Fp.fromBytes(hexBytes(48, hex)) catch
        @compileError("bls12_381: non-canonical isogeny coefficient");
}

/// Comptime `Fp2` constant from two 96-hex-digit strings — `(c0, c1)`
/// in IN-MEMORY component order (this is a struct literal, NOT the
/// `c1 || c0` wire order of `Fp2.toBytes` — see `fp2.zig`'s
/// `encoded_bytes` doc comment for that pitfall).
fn fp2c(comptime c0_hex: *const [96:0]u8, comptime c1_hex: *const [96:0]u8) Fp2 {
    return .{ .c0 = fpc(c0_hex), .c1 = fpc(c1_hex) };
}

// ── expand_message_xmd (RFC 9380 §5.3.1) — REAL ────────────────────────

/// RFC 9380 §5.3.1 `expand_message_xmd`, specialized to SHA-256
/// (`b_in_bytes = 32`, `s_in_bytes = 64`) but GENERAL in `len_in_bytes`
/// (hence `ell = ceil(len_in_bytes / 32)`, up to the RFC's own `ell <=
/// 255` ceiling) — unlike `frost`'s hardcoded `ell = 2` or `voprf`'s
/// hardcoded `ell = 1` (see this file's module doc comment for why
/// those two aren't reused directly). `len_in_bytes` is `comptime`
/// because every call site here has a fixed, suite-determined length
/// (`128` for `G1`'s `hash_to_field(msg, 2)`, `256` for `G2`'s) and a
/// comptime length lets the return type be a plain fixed-size array
/// (no allocator).
///
/// Construction (RFC 9380 §5.3.1, verbatim):
/// ```
/// DST_prime   = DST || I2OSP(len(DST), 1)
/// Z_pad       = I2OSP(0, s_in_bytes)                    # 64 zero bytes
/// l_i_b_str   = I2OSP(len_in_bytes, 2)                  # big-endian u16
/// msg_prime   = Z_pad || msg || l_i_b_str || I2OSP(0,1) || DST_prime
/// b_0         = H(msg_prime)
/// b_1         = H(b_0 || I2OSP(1, 1) || DST_prime)
/// b_i         = H(strxor(b_0, b_{i-1}) || I2OSP(i, 1) || DST_prime)   for i in 2..ell
/// uniform_bytes = b_1 || b_2 || ... || b_ell
/// return substr(uniform_bytes, 0, len_in_bytes)
/// ```
/// `dst.len` MUST be `<= 255` (RFC 9380's own DST-length ceiling before
/// the "long DST" re-hashing procedure of §5.3.3 applies — every DST
/// this module's two target suites use, e.g.
/// `"QUUX-V01-CS02-with-BLS12381G1_XMD:SHA-256_SSWU_RO_"`, is far under
/// this, so the assert is defensive, not a real constraint).
///
/// Verified byte-exact against RFC 9380 Appendix K.1's
/// `expand_message_xmd(SHA-256)` vectors (DST =
/// `"QUUX-V01-CS02-with-expander-SHA256-128"`) for both published
/// `len_in_bytes` values, `0x20` (`ell = 1`) and `0x80` (`ell = 4`) —
/// see the tests below. The `ell = 4` case in particular exercises the
/// general multi-block loop frost/voprf's fixed-`ell` specializations
/// never needed.
pub fn expandMessageXmd(comptime len_in_bytes: usize, msg: []const u8, dst: []const u8) [len_in_bytes]u8 {
    const Sha256 = std.crypto.hash.sha2.Sha256;
    const b_in_bytes = Sha256.digest_length; // 32
    const s_in_bytes = 64; // SHA-256 input block size

    const ell = comptime (len_in_bytes + b_in_bytes - 1) / b_in_bytes;
    comptime std.debug.assert(len_in_bytes > 0);
    comptime std.debug.assert(ell <= 255);
    std.debug.assert(dst.len <= 255);

    var dst_prime_buf: [256]u8 = undefined;
    @memcpy(dst_prime_buf[0..dst.len], dst);
    dst_prime_buf[dst.len] = @intCast(dst.len);
    const dst_prime = dst_prime_buf[0 .. dst.len + 1];

    const z_pad = [_]u8{0} ** s_in_bytes;
    var l_i_b_str: [2]u8 = undefined;
    std.mem.writeInt(u16, &l_i_b_str, len_in_bytes, .big);

    var h0 = Sha256.init(.{});
    h0.update(&z_pad);
    h0.update(msg);
    h0.update(&l_i_b_str);
    h0.update(&[_]u8{0x00});
    h0.update(dst_prime);
    var b0: [b_in_bytes]u8 = undefined;
    h0.final(&b0);

    var out: [len_in_bytes]u8 = undefined;

    var h1 = Sha256.init(.{});
    h1.update(&b0);
    h1.update(&[_]u8{0x01});
    h1.update(dst_prime);
    var b_prev: [b_in_bytes]u8 = undefined;
    h1.final(&b_prev);
    {
        const n = @min(b_in_bytes, len_in_bytes);
        @memcpy(out[0..n], b_prev[0..n]);
    }

    comptime var i: usize = 2;
    inline while (i <= ell) : (i += 1) {
        var xored: [b_in_bytes]u8 = undefined;
        for (&xored, b0, b_prev) |*o, x, y| o.* = x ^ y;

        var hi = Sha256.init(.{});
        hi.update(&xored);
        hi.update(&[_]u8{@intCast(i)});
        hi.update(dst_prime);
        var bi: [b_in_bytes]u8 = undefined;
        hi.final(&bi);

        const offset = (i - 1) * b_in_bytes;
        const n = @min(b_in_bytes, len_in_bytes - offset);
        @memcpy(out[offset..][0..n], bi[0..n]);
        b_prev = bi;
    }

    return out;
}

// ── hash_to_field (RFC 9380 §5.2) — REAL ────────────────────────────────

/// `L` for BLS12-381 (RFC 9380 §8.8.1/§8.8.2, both `G1` and `G2`):
/// `L = ceil((ceil(log2(p)) + k) / 8) = ceil((381 + 128) / 8) = 64`
/// bytes per base-field (`Fp`) element — the same `L` for both suites
/// since `G2`'s base field is `GF(p^2)` (`m = 2`) built on the SAME `p`.
const l_bytes = 64;

/// Reduces a wide (`l_bytes`-byte, i.e. 512-bit) big-endian integer mod
/// `p` into an `Fp` element — the `OS2IP(tv) mod p` step of RFC 9380
/// §5.2's `hash_to_field` (step 7), specialized to this module's `L`.
/// Construction: `Fp.reduceWide` — the Montgomery-resident field's own
/// wide-reduction entry point (Montgomery-reduce the padded `2L`-word
/// value, then two `toMont` passes yield `(X mod p)·R`). `Fr` has the
/// analogous `reduceWide` (`scalar.zig`), still on `std.crypto.ff`; this
/// call moved off the removed direct `fp.modulus.reduce(..)` when `Fp`
/// gained its Montgomery backend, with no change to the math.
fn reduceWideToFp(bytes: [l_bytes]u8) Fp {
    return Fp.reduceWide(l_bytes, bytes);
}

/// `hash_to_field(msg, count)` (RFC 9380 §5.2) for the base field `Fp`
/// (`m = 1`) — used by the `G1` suite. `len_in_bytes = count * L =
/// count * 64`; `count` is `comptime` for the same reason
/// `expandMessageXmd`'s `len_in_bytes` is.
///
/// Verified byte-exact for `count = 2` against RFC 9380 Appendix
/// J.9.1's `u[0]`/`u[1]` intermediates (all 5 published messages) — see
/// the tests below. This is a REAL, PASSING test today (unlike the
/// `hashToCurveG1` tests further down, which additionally need the
/// still-stubbed `mapToCurveG1`).
pub fn hashToFieldFp(comptime count: usize, msg: []const u8, dst: []const u8) [count]Fp {
    const uniform = expandMessageXmd(count * l_bytes, msg, dst);
    var out: [count]Fp = undefined;
    inline for (0..count) |i| {
        out[i] = reduceWideToFp(uniform[i * l_bytes ..][0..l_bytes].*);
    }
    return out;
}

/// `hash_to_field(msg, count)` (RFC 9380 §5.2) for `Fp2 = GF(p^2)`
/// (`m = 2`, basis `(1, u)` — the SAME `u^2 = -1` convention `fp2.zig`
/// uses throughout, matching RFC 9380 §8.8.2's `(1, I)` with
/// `I^2 + 1 == 0`) — used by the `G2` suite. `len_in_bytes = count * m *
/// L = count * 128`. Per §5.2 step 8, `u_i = (e_0, e_1)` with `e_0` the
/// coefficient of the basis's first element (`1`, i.e. `Fp2.c0`) and
/// `e_1` the coefficient of the second (`I`/`u`, i.e. `Fp2.c1`) — NOT
/// `fp2.zig`'s WIRE order (`c1 || c0`, high-first — see that file's
/// `encoded_bytes` doc comment); this is an in-memory `Fp2{c0,c1}`
/// construction, not a serialization, so that wire-order pitfall does
/// not apply here.
///
/// Verified byte-exact for `count = 2` against RFC 9380 Appendix
/// J.10.1's `u[0]`/`u[1]` intermediates (all 5 published messages,
/// `c0`/`c1` both checked) — see the tests below. REAL and PASSING
/// today, same status as `hashToFieldFp`.
pub fn hashToFieldFp2(comptime count: usize, msg: []const u8, dst: []const u8) [count]Fp2 {
    const uniform = expandMessageXmd(count * 2 * l_bytes, msg, dst);
    var out: [count]Fp2 = undefined;
    inline for (0..count) |i| {
        const e0 = reduceWideToFp(uniform[(2 * i) * l_bytes ..][0..l_bytes].*);
        const e1 = reduceWideToFp(uniform[(2 * i + 1) * l_bytes ..][0..l_bytes].*);
        out[i] = .{ .c0 = e0, .c1 = e1 };
    }
    return out;
}

// ── G1 map_to_curve: Simplified SWU (§6.6.3) + 11-isogeny (§E.2) ───────
//
// RFC 9380 §8.8.1 (BLS12381G1_XMD:SHA-256_SSWU_RO_):
//   E:  y^2 = x^3 + 4                          (G1's actual curve)
//   E': y'^2 = x'^3 + A'*x' + B'                (the 11-isogenous curve)
//   Z = 11, A', B' below (cited directly from the RFC text — small,
//       single field constants, not an error-prone table)
//   f = Simplified SWU for AB == 0 (§6.6.3): map_to_curve_simple_swu
//       (§6.6.2) onto E', then iso_map (the 11-isogeny, Appendix E.2)
//       onto E.

/// `Z` for the `G1` SSWU map (RFC 9380 §8.8.1): `11`, as an `Fp`
/// element. A single small constant — safe to embed directly (per this
/// module's "don't hand-transcribe error-prone tables" rule, which
/// targets the LARGE isogeny coefficient lists below, not one-element
/// citations like this).
pub const g1_iso_z: Fp = Fp.fromInt(u8, 11) catch @compileError("bls12_381: bad G1 iso Z");

/// `A'` for `G1`'s 11-isogenous curve `E1'` (RFC 9380 §8.8.1), hex
/// big-endian:
/// `0x144698a3b8e9433d693a02c96d4982b0ea985383ee66a8d8e8981aefd881ac9
/// 8936f8da0e0f97f5cf428082d584c1d`. Cited directly from the RFC's own
/// text (a single field-element constant, not a coefficient table).
pub const g1_iso_a: Fp = Fp.fromBytes(hexBytes(48, "00144698a3b8e9433d693a02c96d4982b0ea985383ee66a8d8e8981aefd881ac98936f8da0e0f97f5cf428082d584c1d")) catch
    @compileError("bls12_381: bad G1 iso A'");

/// `B'` for `G1`'s 11-isogenous curve `E1'` (RFC 9380 §8.8.1), hex
/// big-endian:
/// `0x12e2908d11688030018b12e8753eee3b2016c1f0f24f4070a0b9c14fcef35e
/// f55a23215a316ceaa5d1cc48e98e172be0`.
pub const g1_iso_b: Fp = Fp.fromBytes(hexBytes(48, "12e2908d11688030018b12e8753eee3b2016c1f0f24f4070a0b9c14fcef35ef55a23215a316ceaa5d1cc48e98e172be0")) catch
    @compileError("bls12_381: bad G1 iso B'");

/// A point on the 11-isogenous curve `E1': y'^2 = x'^3 + A'x' + B'` —
/// the intermediate `sswuG1` produces and `isogenyMap11` consumes.
pub const E1PrimeAffine = struct { x: Fp, y: Fp };

/// Simplified SWU (RFC 9380 §6.6.2 `map_to_curve_simple_swu`) mapping
/// `u ∈ Fp` onto `E1': y'^2 = x'^3 + A'x' + B'` (`A' = g1_iso_a`,
/// `B' = g1_iso_b`, `Z = g1_iso_z`).
///
/// Construction (RFC 9380 §6.6.2, operations 1-10, `A = A'`, `B = B'`):
/// ```
/// tv1 = inv0(Z^2 * u^4 + Z * u^2)
/// x1  = (-B / A) * (1 + tv1);  if tv1 == 0, x1 = B / (Z * A)
/// gx1 = x1^3 + A*x1 + B
/// x2  = Z * u^2 * x1
/// gx2 = x2^3 + A*x2 + B
/// if is_square(gx1): x = x1, y = sqrt(gx1)
/// else:               x = x2, y = sqrt(gx2)
/// if sgn0(u) != sgn0(y): y = -y
/// return (x, y)
/// ```
/// where `inv0(0) = 0` (NOT an error — RFC 9380 §4's `inv0`, the
/// zero-preserving inverse) and `sgn0` is RFC 9380 §4.1's `sgn0_m_eq_1`
/// (for `Fp`: the parity of the canonical integer representative,
/// `x mod 2`). Public-input path (hash-to-curve's `u` is always
/// message-derived and public — see `SPEC.md`'s threat-model note) —
/// branching on `is_square`/`sgn0` is fine, no constant-time obligation.
pub fn sswuG1(u: Fp) E1PrimeAffine {
    const zu2 = g1_iso_z.mul(u.square()); // Z * u^2
    const tv1 = zu2.square().add(zu2).inv0(); // inv0(Z^2 u^4 + Z u^2)
    const x1 = if (tv1.isZero())
        // Exceptional case (u == 0, or Z u^2 == -1): x1 = B / (Z*A).
        // g(x1) is a QR at this specific x1 for E1' (verified
        // numerically — see NOTICE), so the gx2 branch below is
        // unreachable from here.
        g1_iso_b.mul(g1_iso_z.mul(g1_iso_a).inv0())
    else
        g1_iso_b.neg().mul(g1_iso_a.inv0()).mul(Fp.one.add(tv1)); // (-B/A)(1 + tv1)
    const gx1 = x1.square().mul(x1).add(g1_iso_a.mul(x1)).add(g1_iso_b);

    const xy: E1PrimeAffine = if (gx1.sqrt()) |s|
        .{ .x = x1, .y = s }
    else blk: {
        // gx2 = g(Z u^2 x1) = Z^3 u^6 gx1 (the SSWU identity): Z is a
        // non-square and u^6 a square, so gx2 is a QR exactly when gx1
        // is NOT — this sqrt cannot fail (and the tv1 == 0 exceptional
        // case never reaches here, per the note above).
        const x2 = zu2.mul(x1);
        const gx2 = x2.square().mul(x2).add(g1_iso_a.mul(x2)).add(g1_iso_b);
        break :blk .{ .x = x2, .y = gx2.sqrt() orelse unreachable };
    };
    // Operation 10: fix the sign of y to match u's (RFC 9380 §4.1 sgn0).
    const y = if (u.sgn0() != xy.y.sgn0()) xy.y.neg() else xy.y;
    return .{ .x = xy.x, .y = y };
}

// ── 11-isogeny coefficient tables (RFC 9380 Appendix E.2) ──────────────
//
// SOURCED programmatically from RFC 9380's raw text (rfc-editor.org's
// rfc9380.txt, Appendix E.2) — NOT hand-transcribed: a scratch parser
// extracted all 53 `k_(i,j)` values (multi-line hex joined
// mechanically), and an INDEPENDENT Python implementation of the whole
// SSWU-plus-isogeny chain (big-integer arithmetic, affine group
// formulas — a different algorithm family from this module's) built on
// exactly these parsed values reproduces RFC 9380 Appendix J.9.1's
// `Q0`/`Q1`/`P` byte-exactly for all 5 published messages, and maps
// arbitrary non-vector `E1'` points onto `E` — see `NOTICE`'s Part-3
// crypto-core section. Layout: index `j` holds `k_(i,j)` (the `x'^j`
// coefficient, ascending); the monic leading terms (`x_den`'s `x'^10`,
// `y_den`'s `x'^15`) are implicit, handled by `evalPolyFp`'s `monic`
// flag — exactly as the RFC writes the polynomials.

const g1_iso_x_num = [12]Fp{
    fpc("11a05f2b1e833340b809101dd99815856b303e88a2d7005ff2627b56cdb4e2c85610c2d5f2e62d6eaeac1662734649b7"),
    fpc("17294ed3e943ab2f0588bab22147a81c7c17e75b2f6a8417f565e33c70d1e86b4838f2a6f318c356e834eef1b3cb83bb"),
    fpc("0d54005db97678ec1d1048c5d10a9a1bce032473295983e56878e501ec68e25c958c3e3d2a09729fe0179f9dac9edcb0"),
    fpc("1778e7166fcc6db74e0609d307e55412d7f5e4656a8dbf25f1b33289f1b330835336e25ce3107193c5b388641d9b6861"),
    fpc("0e99726a3199f4436642b4b3e4118e5499db995a1257fb3f086eeb65982fac18985a286f301e77c451154ce9ac8895d9"),
    fpc("1630c3250d7313ff01d1201bf7a74ab5db3cb17dd952799b9ed3ab9097e68f90a0870d2dcae73d19cd13c1c66f652983"),
    fpc("0d6ed6553fe44d296a3726c38ae652bfb11586264f0f8ce19008e218f9c86b2a8da25128c1052ecaddd7f225a139ed84"),
    fpc("17b81e7701abdbe2e8743884d1117e53356de5ab275b4db1a682c62ef0f2753339b7c8f8c8f475af9ccb5618e3f0c88e"),
    fpc("080d3cf1f9a78fc47b90b33563be990dc43b756ce79f5574a2c596c928c5d1de4fa295f296b74e956d71986a8497e317"),
    fpc("169b1f8e1bcfa7c42e0c37515d138f22dd2ecb803a0c5c99676314baf4bb1b7fa3190b2edc0327797f241067be390c9e"),
    fpc("10321da079ce07e272d8ec09d2565b0dfa7dccdde6787f96d50af36003b14866f69b771f8c285decca67df3f1605fb7b"),
    fpc("06e08c248e260e70bd1e962381edee3d31d79d7e22c837bc23c0bf1bc24c6b68c24b1b80b64d391fa9c8ba2e8ba2d229"),
};

const g1_iso_x_den = [10]Fp{
    fpc("08ca8d548cff19ae18b2e62f4bd3fa6f01d5ef4ba35b48ba9c9588617fc8ac62b558d681be343df8993cf9fa40d21b1c"),
    fpc("12561a5deb559c4348b4711298e536367041e8ca0cf0800c0126c2588c48bf5713daa8846cb026e9e5c8276ec82b3bff"),
    fpc("0b2962fe57a3225e8137e629bff2991f6f89416f5a718cd1fca64e00b11aceacd6a3d0967c94fedcfcc239ba5cb83e19"),
    fpc("03425581a58ae2fec83aafef7c40eb545b08243f16b1655154cca8abc28d6fd04976d5243eecf5c4130de8938dc62cd8"),
    fpc("13a8e162022914a80a6f1d5f43e7a07dffdfc759a12062bb8d6b44e833b306da9bd29ba81f35781d539d395b3532a21e"),
    fpc("0e7355f8e4e667b955390f7f0506c6e9395735e9ce9cad4d0a43bcef24b8982f7400d24bc4228f11c02df9a29f6304a5"),
    fpc("0772caacf16936190f3e0c63e0596721570f5799af53a1894e2e073062aede9cea73b3538f0de06cec2574496ee84a3a"),
    fpc("14a7ac2a9d64a8b230b3f5b074cf01996e7f63c21bca68a81996e1cdf9822c580fa5b9489d11e2d311f7d99bbdcc5a5e"),
    fpc("0a10ecf6ada54f825e920b3dafc7a3cce07f8d1d7161366b74100da67f39883503826692abba43704776ec3a79a1d641"),
    fpc("095fc13ab9e92ad4476d6e3eb3a56680f682b4ee96f7d03776df533978f31c1593174e4b4b7865002d6384d168ecdd0a"),
};

const g1_iso_y_num = [16]Fp{
    fpc("090d97c81ba24ee0259d1f094980dcfa11ad138e48a869522b52af6c956543d3cd0c7aee9b3ba3c2be9845719707bb33"),
    fpc("134996a104ee5811d51036d776fb46831223e96c254f383d0f906343eb67ad34d6c56711962fa8bfe097e75a2e41c696"),
    fpc("00cc786baa966e66f4a384c86a3b49942552e2d658a31ce2c344be4b91400da7d26d521628b00523b8dfe240c72de1f6"),
    fpc("01f86376e8981c217898751ad8746757d42aa7b90eeb791c09e4a3ec03251cf9de405aba9ec61deca6355c77b0e5f4cb"),
    fpc("08cc03fdefe0ff135caf4fe2a21529c4195536fbe3ce50b879833fd221351adc2ee7f8dc099040a841b6daecf2e8fedb"),
    fpc("16603fca40634b6a2211e11db8f0a6a074a7d0d4afadb7bd76505c3d3ad5544e203f6326c95a807299b23ab13633a5f0"),
    fpc("04ab0b9bcfac1bbcb2c977d027796b3ce75bb8ca2be184cb5231413c4d634f3747a87ac2460f415ec961f8855fe9d6f2"),
    fpc("0987c8d5333ab86fde9926bd2ca6c674170a05bfe3bdd81ffd038da6c26c842642f64550fedfe935a15e4ca31870fb29"),
    fpc("09fc4018bd96684be88c9e221e4da1bb8f3abd16679dc26c1e8b6e6a1f20cabe69d65201c78607a360370e577bdba587"),
    fpc("0e1bba7a1186bdb5223abde7ada14a23c42a0ca7915af6fe06985e7ed1e4d43b9b3f7055dd4eba6f2bafaaebca731c30"),
    fpc("19713e47937cd1be0dfd0b8f1d43fb93cd2fcbcb6caf493fd1183e416389e61031bf3a5cce3fbafce813711ad011c132"),
    fpc("18b46a908f36f6deb918c143fed2edcc523559b8aaf0c2462e6bfe7f911f643249d9cdf41b44d606ce07c8a4d0074d8e"),
    fpc("0b182cac101b9399d155096004f53f447aa7b12a3426b08ec02710e807b4633f06c851c1919211f20d4c04f00b971ef8"),
    fpc("0245a394ad1eca9b72fc00ae7be315dc757b3b080d4c158013e6632d3c40659cc6cf90ad1c232a6442d9d3f5db980133"),
    fpc("05c129645e44cf1102a159f748c4a3fc5e673d81d7e86568d9ab0f5d396a7ce46ba1049b6579afb7866b1e715475224b"),
    fpc("15e6be4e990f03ce4ea50b3b42df2eb5cb181d8f84965a3957add4fa95af01b2b665027efec01c7704b456be69c8b604"),
};

const g1_iso_y_den = [15]Fp{
    fpc("16112c4c3a9c98b252181140fad0eae9601a6de578980be6eec3232b5be72e7a07f3688ef60c206d01479253b03663c1"),
    fpc("1962d75c2381201e1a0cbd6c43c348b885c84ff731c4d59ca4a10356f453e01f78a4260763529e3532f6102c2e49a03d"),
    fpc("058df3306640da276faaae7d6e8eb15778c4855551ae7f310c35a5dd279cd2eca6757cd636f96f891e2538b53dbf67f2"),
    fpc("16b7d288798e5395f20d23bf89edb4d1d115c5dbddbcd30e123da489e726af41727364f2c28297ada8d26d98445f5416"),
    fpc("0be0e079545f43e4b00cc912f8228ddcc6d19c9f0f69bbb0542eda0fc9dec916a20b15dc0fd2ededda39142311a5001d"),
    fpc("08d9e5297186db2d9fb266eaac783182b70152c65550d881c5ecd87b6f0f5a6449f38db9dfa9cce202c6477faaf9b7ac"),
    fpc("166007c08a99db2fc3ba8734ace9824b5eecfdfa8d0cf8ef5dd365bc400a0051d5fa9c01a58b1fb93d1a1399126a775c"),
    fpc("16a3ef08be3ea7ea03bcddfabba6ff6ee5a4375efa1f4fd7feb34fd206357132b920f5b00801dee460ee415a15812ed9"),
    fpc("1866c8ed336c61231a1be54fd1d74cc4f9fb0ce4c6af5920abc5750c4bf39b4852cfe2f7bb9248836b233d9d55535d4a"),
    fpc("167a55cda70a6e1cea820597d94a84903216f763e13d87bb5308592e7ea7d4fbc7385ea3d529b35e346ef48bb8913f55"),
    fpc("04d2f259eea405bd48f010a01ad2911d9c6dd039bb61a6290e591b36e636a5c871a5c29f4f83060400f8b49cba8f6aa8"),
    fpc("0accbb67481d033ff5852c1e48c50c477f94ff8aefce42d28c0f9a88cea7913516f968986f7ebbea9684b529e2561092"),
    fpc("0ad6b9514c767fe3c3613144b45f1496543346d98adf02267d5ceef9a00d9b8693000763e3b90ac11e99b138573345cc"),
    fpc("02660400eb2e4f3b628bdd0d53cd76f2bf565b94e72927c1cb748df27942480e420517bd8714cc80d1fadc1326ed06f7"),
    fpc("0e0fa1d816ddc03e6b24255e0d7819c171c40f65e273b853324efcd6356caa205ca2f570f13497804415473a1d634b8f"),
};

/// Evaluates `sum(coeffs[j] * x^j)` — plus the implicit MONIC leading
/// term `x^coeffs.len` if `monic` — by Horner's method over `Fp`.
fn evalPolyFp(coeffs: []const Fp, monic: bool, x: Fp) Fp {
    var acc = if (monic) Fp.one else Fp.zero;
    var j = coeffs.len;
    while (j > 0) {
        j -= 1;
        acc = acc.mul(x).add(coeffs[j]);
    }
    return acc;
}

/// The 11-isogeny map from `E1'` to `G1`'s actual curve `E: y^2 = x^3 +
/// 4` (RFC 9380 §6.6.3 `iso_map`, coefficient table in **Appendix
/// E.2**): `x = x_num(x') / x_den(x')`, `y = y' * y_num(x') /
/// y_den(x')`, where `x_num` is degree 11 (12 coefficients `k_(1,0)` ..
/// `k_(1,11)`), `x_den` is MONIC degree 10 (10 coefficients `k_(2,0)`
/// .. `k_(2,9)`, leading term `x'^10` implicit), `y_num` is degree 15
/// (16 coefficients `k_(3,0)` .. `k_(3,15)`), `y_den` is MONIC degree
/// 15 (15 coefficients `k_(4,0)` .. `k_(4,14)`) — 53 `Fp` constants
/// total (the tables above; see their sourcing/verification note).
/// Exceptional case (RFC 9380 §6.6.3): if either denominator evaluates
/// to zero, return `G1`'s identity point.
pub fn isogenyMap11(p: E1PrimeAffine) g1.Affine {
    const x_num = evalPolyFp(&g1_iso_x_num, false, p.x);
    const x_den = evalPolyFp(&g1_iso_x_den, true, p.x);
    const y_num = evalPolyFp(&g1_iso_y_num, false, p.x);
    const y_den = evalPolyFp(&g1_iso_y_den, true, p.x);
    if (x_den.isZero() or y_den.isZero()) return g1.Affine.identity;
    return .{
        .x = x_num.mul(x_den.inv0()),
        .y = p.y.mul(y_num).mul(y_den.inv0()),
    };
}

/// `map_to_curve` for `G1` (RFC 9380 §8.8.1's `f`): `sswuG1` then
/// `isogenyMap11` — the composition RFC 9380 §6.6.3 operations 1-2
/// define. Pinned byte-exact against RFC 9380 Appendix J.9.1's
/// `Q0`/`Q1` intermediates (all 5 messages) by the tests below.
pub fn mapToCurveG1(u: Fp) g1.Affine {
    return isogenyMap11(sswuG1(u));
}

// ── G2 map_to_curve: Simplified SWU (§6.6.3) + 3-isogeny (§E.3) ────────
//
// RFC 9380 §8.8.2 (BLS12381G2_XMD:SHA-256_SSWU_RO_):
//   E:  y^2 = x^3 + 4*(1+I)                    (G2's actual twist)
//   E': y'^2 = x'^3 + A'*x' + B'                (the 3-isogenous curve)
//   Z = -(2 + I), A' = 240*I, B' = 1012*(1+I)   (all SHORT Fp2 constants)
//   f = Simplified SWU for AB == 0 (§6.6.3), same shape as G1's, over
//       Fp2, with iso_map the 3-isogeny of Appendix E.3.

/// `Z` for the `G2` SSWU map (RFC 9380 §8.8.2): `-(2 + I)`, i.e.
/// `Fp2{c0=-2, c1=-1}`. Short enough to embed directly (two small `Fp`
/// values, not a table).
pub const g2_iso_z: Fp2 = .{
    .c0 = (Fp.fromInt(u8, 2) catch unreachable).neg(),
    .c1 = Fp.one.neg(),
};

/// `A'` for `G2`'s 3-isogenous curve `E2'` (RFC 9380 §8.8.2): `240 * I`,
/// i.e. `Fp2{c0=0, c1=240}`.
pub const g2_iso_a: Fp2 = .{
    .c0 = Fp.zero,
    .c1 = Fp.fromInt(u16, 240) catch unreachable,
};

/// `B'` for `G2`'s 3-isogenous curve `E2'` (RFC 9380 §8.8.2):
/// `1012 * (1 + I)`, i.e. `Fp2{c0=1012, c1=1012}`.
pub const g2_iso_b: Fp2 = .{
    .c0 = Fp.fromInt(u16, 1012) catch unreachable,
    .c1 = Fp.fromInt(u16, 1012) catch unreachable,
};

/// A point on the 3-isogenous curve `E2': y'^2 = x'^3 + A'x' + B'`
/// (`Fp2`-valued) — the intermediate `sswuG2` produces and
/// `isogenyMap3` consumes.
pub const E2PrimeAffine = struct { x: Fp2, y: Fp2 };

/// Simplified SWU (RFC 9380 §6.6.2) mapping `u ∈ Fp2` onto
/// `E2': y'^2 = x'^3 + A'x' + B'` (`A' = g2_iso_a`, `B' = g2_iso_b`,
/// `Z = g2_iso_z`). Identical operation sequence to `sswuG1`'s doc
/// comment, over `Fp2` throughout — `Fp2` already has every primitive
/// needed (`mul`/`square`/`add`/`sub`/`neg`/`sqrt`/`inv`); `sgn0` for
/// `m = 2` is RFC 9380 §4.1's `sgn0_m_eq_2` (lexicographic over
/// `(c0, c1)` by PARITY, not `fp2.zig`'s existing
/// `isLexicographicallyLargest` — that helper compares MAGNITUDE
/// against `(p-1)/2` for point-compression's sign bit, a DIFFERENT
/// notion of "sign" than RFC 9380's `sgn0`; do not conflate the two).
pub fn sswuG2(u: Fp2) E2PrimeAffine {
    const zu2 = g2_iso_z.mul(u.square()); // Z * u^2
    const tv1 = zu2.square().add(zu2).inv0(); // inv0(Z^2 u^4 + Z u^2)
    const x1 = if (tv1.isZero())
        // Exceptional case (u == 0; -1/Z is a non-residue in Fp2, so no
        // nonzero u reaches this): x1 = B / (Z*A). g(x1) is a QR at
        // this x1 for E2' (verified numerically — see NOTICE).
        g2_iso_b.mul(g2_iso_z.mul(g2_iso_a).inv0())
    else
        g2_iso_b.neg().mul(g2_iso_a.inv0()).mul(Fp2.one.add(tv1)); // (-B/A)(1 + tv1)
    const gx1 = x1.square().mul(x1).add(g2_iso_a.mul(x1)).add(g2_iso_b);

    const xy: E2PrimeAffine = if (gx1.sqrt()) |s|
        .{ .x = x1, .y = s }
    else blk: {
        // Same SSWU identity as sswuG1's: gx2 = Z^3 u^6 gx1 is a QR
        // exactly when gx1 is not — this sqrt cannot fail.
        const x2 = zu2.mul(x1);
        const gx2 = x2.square().mul(x2).add(g2_iso_a.mul(x2)).add(g2_iso_b);
        break :blk .{ .x = x2, .y = gx2.sqrt() orelse unreachable };
    };
    // Operation 10: fix y's sign to u's (RFC 9380 §4.1 sgn0_m_eq_2).
    const y = if (u.sgn0() != xy.y.sgn0()) xy.y.neg() else xy.y;
    return .{ .x = xy.x, .y = y };
}

/// The 3-isogeny map from `E2'` to `G2`'s actual twist `E: y^2 = x^3 +
/// 4(1+I)` (RFC 9380 §6.6.3 `iso_map`, coefficient table in **Appendix
/// E.3**): same rational-function shape as `isogenyMap11`, much
/// smaller — `x_num` degree 3 (4 `Fp2` coefficients), `x_den` MONIC
/// degree 2 (2 coefficients), `y_num` degree 3 (4 coefficients),
/// `y_den` MONIC degree 3 (3 coefficients) — 13 `Fp2` constants total
/// (each itself a `c0 + c1*I` pair, so 26 `Fp` values). Exceptional
/// case: either denominator zero → return `G2`'s identity point.
///
/// Coefficient tables: below (Appendix E.3, 13 `Fp2` constants —
/// sourced and verified exactly the same way as the E.2 tables, see
/// the comment above `g1_iso_x_num`; the RFC writes each `Fp2` value
/// as `c0 + c1 * I`, mapped here to the in-memory `(c0, c1)` order via
/// `fp2c` — NOT `fp2.zig`'s `c1 || c0` WIRE order, which does not
/// apply to struct literals).
pub fn isogenyMap3(p: E2PrimeAffine) g2.Affine {
    const x_num = evalPolyFp2(&g2_iso_x_num, false, p.x);
    const x_den = evalPolyFp2(&g2_iso_x_den, true, p.x);
    const y_num = evalPolyFp2(&g2_iso_y_num, false, p.x);
    const y_den = evalPolyFp2(&g2_iso_y_den, true, p.x);
    if (x_den.isZero() or y_den.isZero()) return g2.Affine.identity;
    return .{
        .x = x_num.mul(x_den.inv0()),
        .y = p.y.mul(y_num).mul(y_den.inv0()),
    };
}

// ── 3-isogeny coefficient tables (RFC 9380 Appendix E.3) ────────────────
//
// Same programmatic sourcing + independent Python end-to-end
// verification as the E.2 tables above (all 13 `Fp2` values parsed
// from the RFC's raw text; the independent implementation reproduces
// Appendix J.10.1's `Q0`/`Q1`/`P` byte-exactly on these values).

const g2_iso_x_num = [4]Fp2{
    fp2c(
        "05c759507e8e333ebb5b7a9a47d7ed8532c52d39fd3a042a88b58423c50ae15d5c2638e343d9c71c6238aaaaaaaa97d6",
        "05c759507e8e333ebb5b7a9a47d7ed8532c52d39fd3a042a88b58423c50ae15d5c2638e343d9c71c6238aaaaaaaa97d6",
    ),
    fp2c(
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "11560bf17baa99bc32126fced787c88f984f87adf7ae0c7f9a208c6b4f20a4181472aaa9cb8d555526a9ffffffffc71a",
    ),
    fp2c(
        "11560bf17baa99bc32126fced787c88f984f87adf7ae0c7f9a208c6b4f20a4181472aaa9cb8d555526a9ffffffffc71e",
        "08ab05f8bdd54cde190937e76bc3e447cc27c3d6fbd7063fcd104635a790520c0a395554e5c6aaaa9354ffffffffe38d",
    ),
    fp2c(
        "171d6541fa38ccfaed6dea691f5fb614cb14b4e7f4e810aa22d6108f142b85757098e38d0f671c7188e2aaaaaaaa5ed1",
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
    ),
};

const g2_iso_x_den = [2]Fp2{
    fp2c(
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaa63",
    ),
    fp2c(
        "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000c",
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaa9f",
    ),
};

const g2_iso_y_num = [4]Fp2{
    fp2c(
        "1530477c7ab4113b59a4c18b076d11930f7da5d4a07f649bf54439d87d27e500fc8c25ebf8c92f6812cfc71c71c6d706",
        "1530477c7ab4113b59a4c18b076d11930f7da5d4a07f649bf54439d87d27e500fc8c25ebf8c92f6812cfc71c71c6d706",
    ),
    fp2c(
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "05c759507e8e333ebb5b7a9a47d7ed8532c52d39fd3a042a88b58423c50ae15d5c2638e343d9c71c6238aaaaaaaa97be",
    ),
    fp2c(
        "11560bf17baa99bc32126fced787c88f984f87adf7ae0c7f9a208c6b4f20a4181472aaa9cb8d555526a9ffffffffc71c",
        "08ab05f8bdd54cde190937e76bc3e447cc27c3d6fbd7063fcd104635a790520c0a395554e5c6aaaa9354ffffffffe38f",
    ),
    fp2c(
        "124c9ad43b6cf79bfbf7043de3811ad0761b0f37a1e26286b0e977c69aa274524e79097a56dc4bd9e1b371c71c718b10",
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
    ),
};

const g2_iso_y_den = [3]Fp2{
    fp2c(
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffa8fb",
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffa8fb",
    ),
    fp2c(
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffa9d3",
    ),
    fp2c(
        "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000012",
        "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaa99",
    ),
};

/// `evalPolyFp`'s `Fp2` twin — Horner's method with the same implicit
/// monic-leading-term handling.
fn evalPolyFp2(coeffs: []const Fp2, monic: bool, x: Fp2) Fp2 {
    var acc = if (monic) Fp2.one else Fp2.zero;
    var j = coeffs.len;
    while (j > 0) {
        j -= 1;
        acc = acc.mul(x).add(coeffs[j]);
    }
    return acc;
}

/// `map_to_curve` for `G2` (RFC 9380 §8.8.2's `f`): `sswuG2` then
/// `isogenyMap3`. Pinned byte-exact against RFC 9380 Appendix
/// J.10.1's `Q0`/`Q1` intermediates (all 5 messages) by the tests
/// below.
pub fn mapToCurveG2(u: Fp2) g2.Affine {
    return isogenyMap3(sswuG2(u));
}

// ── hash_to_curve / encode_to_curve (RFC 9380 §3) ───────────────────────

/// `h_eff` for the `G1` suites (RFC 9380 §8.8.1): `0xd201000000010001`
/// (= `1 - z` for BLS12-381's defining parameter `z` — the Scott
/// [WB19] fast-clearing scalar), cited from the RFC's own text;
/// big-endian, fed straight to `scalarMulBytes`. See the module doc
/// comment for why RFC 9380's `clear_cofactor` MUST multiply by this
/// and not by the plain cofactor `h1`.
const g1_h_eff_bytes: [8]u8 = hexBytes(8, "d201000000010001");

/// `h_eff` for the `G2` suites (RFC 9380 §8.8.2) — the Budroni-Pintore
/// [BP17] fast-clearing scalar, a 636-bit value cited from the RFC's
/// own text (left-padded one nibble to 80 whole bytes); big-endian.
const g2_h_eff_bytes: [80]u8 = hexBytes(80, "0bc69f08f2ee75b3584c6a0ea91b352888e2a8e9145ad7689986ff031508ffe1329c2f178731db956d82bf015d1212b02ec0ec69d7477c1ae954cbc06689f6a359894c0adebbf6b4e8020005aaa95551");

/// `hash_to_curve` (RFC 9380 §3) for `G1`, suite
/// `BLS12381G1_XMD:SHA-256_SSWU_RO_`: `hash_to_field(msg, 2)` → map each
/// of the two field elements onto `E` via `mapToCurveG1` → add the two
/// points (RFC 9380 §6.6.3's note: `iso_map` is a group homomorphism, so
/// adding on `E1'` before applying it would save one `iso_map`
/// evaluation — a documented optimization, NOT taken here; this
/// baseline maps-then-adds on `E` for clarity, matching §3's literal
/// algorithm) → `clear_cofactor`, i.e. multiplication by the suite's
/// `h_eff` (RFC 9380 §7 — NOT `g1.Jacobian.clearCofactor`'s plain-`h1`
/// multiplication, which §7 forbids for this suite and which produces
/// a different point; see the module doc comment).
///
/// Pinned byte-exact against RFC 9380 Appendix J.9.1's final `P` (all
/// 5 published messages) by the tests below.
pub fn hashToCurveG1(msg: []const u8, dst: []const u8) g1.Affine {
    const u = hashToFieldFp(2, msg, dst);
    const q0 = g1.Jacobian.fromAffine(mapToCurveG1(u[0]));
    const q1 = g1.Jacobian.fromAffine(mapToCurveG1(u[1]));
    return q0.add(q1).scalarMulBytes(&g1_h_eff_bytes).toAffine();
}

/// `encode_to_curve` (RFC 9380 §3) for `G1`: the NONUNIFORM one-element
/// variant — `hash_to_field(msg, 1)` → `mapToCurveG1` →
/// `clear_cofactor` (same `h_eff` as `hashToCurveG1` — the `_NU_`
/// suite shares it). Cheaper than `hashToCurveG1` (one hash-to-field
/// draw and one map instead of two) but its output distribution is NOT
/// uniform in `G1` (RFC 9380 §10.4) — use `hashToCurveG1` unless the
/// caller specifically wants the nonuniform encoding (suite
/// `BLS12381G1_XMD:SHA-256_SSWU_NU_`).
pub fn encodeToCurveG1(msg: []const u8, dst: []const u8) g1.Affine {
    const u = hashToFieldFp(1, msg, dst);
    return g1.Jacobian.fromAffine(mapToCurveG1(u[0])).scalarMulBytes(&g1_h_eff_bytes).toAffine();
}

/// `hash_to_curve` (RFC 9380 §3) for `G2`, suite
/// `BLS12381G2_XMD:SHA-256_SSWU_RO_` — same shape as `hashToCurveG1`,
/// one field-tower level up (`Fp2`, `mapToCurveG2`, and §8.8.2's own
/// `h_eff` for `clear_cofactor`). Pinned byte-exact against RFC 9380
/// Appendix J.10.1's final `P` (all 5 messages) by the tests below.
pub fn hashToCurveG2(msg: []const u8, dst: []const u8) g2.Affine {
    const u = hashToFieldFp2(2, msg, dst);
    const q0 = g2.Jacobian.fromAffine(mapToCurveG2(u[0]));
    const q1 = g2.Jacobian.fromAffine(mapToCurveG2(u[1]));
    return q0.add(q1).scalarMulBytes(&g2_h_eff_bytes).toAffine();
}

/// `encode_to_curve` (RFC 9380 §3) for `G2` — see `encodeToCurveG1`'s
/// doc comment for the uniform-vs-nonuniform caveat (suite
/// `BLS12381G2_XMD:SHA-256_SSWU_NU_`).
pub fn encodeToCurveG2(msg: []const u8, dst: []const u8) g2.Affine {
    const u = hashToFieldFp2(1, msg, dst);
    return g2.Jacobian.fromAffine(mapToCurveG2(u[0])).scalarMulBytes(&g2_h_eff_bytes).toAffine();
}

// ── tests ────────────────────────────────────────────────────────────────

// -- expand_message_xmd: RFC 9380 Appendix K.1 (all 5 messages, BOTH
//   published len_in_bytes values — 0x20 exercises ell=1, 0x80 exercises
//   ell=4, the general multi-block loop frost/voprf's fixed-ell
//   specializations never needed). DST = "QUUX-V01-CS02-with-expander-
//   SHA256-128". REAL, PASSING today.

const k1_dst = "QUUX-V01-CS02-with-expander-SHA256-128";

test "expandMessageXmd: RFC 9380 Appendix K.1, len_in_bytes=0x20 (ell=1), all 5 messages" {
    const Vec = struct { msg: []const u8, uniform_hex: []const u8 };
    const vectors = [_]Vec{
        .{ .msg = "", .uniform_hex = "68a985b87eb6b46952128911f2a4412bbc302a9d759667f87f7a21d803f07235" },
        .{ .msg = "abc", .uniform_hex = "d8ccab23b5985ccea865c6c97b6e5b8350e794e603b4b97902f53a8a0d605615" },
        .{ .msg = "abcdef0123456789", .uniform_hex = "eff31487c770a893cfb36f912fbfcbff40d5661771ca4b2cb4eafe524333f5c1" },
        .{ .msg = "q128_" ++ "q" ** 128, .uniform_hex = "b23a1d2b4d97b2ef7785562a7e8bac7eed54ed6e97e29aa51bfe3f12ddad1ff9" },
        .{ .msg = "a512_" ++ "a" ** 512, .uniform_hex = "4623227bcc01293b8c130bf771da8c298dede7383243dc0993d2d94823958c4c" },
    };
    for (vectors) |vec| {
        const expected = hexN(32, vec.uniform_hex);
        const got = expandMessageXmd(32, vec.msg, k1_dst);
        try std.testing.expectEqualSlices(u8, &expected, &got);
    }
}

test "expandMessageXmd: RFC 9380 Appendix K.1, len_in_bytes=0x80 (ell=4), all 5 messages" {
    const Vec = struct { msg: []const u8, uniform_hex: []const u8 };
    const vectors = [_]Vec{
        .{
            .msg = "",
            .uniform_hex = "af84c27ccfd45d41914fdff5df25293e221afc53d8ad2ac06d5e3e29485dadbee0d121587713a3e0dd4d5e69e93eb7cd4f5df4cd103e188cf60cb02edc3edf18eda8576c412b18ffb658e3dd6ec849469b979d444cf7b26911a08e63cf31f9dcc541708d3491184472c2c29bb749d4286b004ceb5ee6b9a7fa5b646c993f0ced",
        },
        .{
            .msg = "abc",
            .uniform_hex = "abba86a6129e366fc877aab32fc4ffc70120d8996c88aee2fe4b32d6c7b6437a647e6c3163d40b76a73cf6a5674ef1d890f95b664ee0afa5359a5c4e07985635bbecbac65d747d3d2da7ec2b8221b17b0ca9dc8a1ac1c07ea6a1e60583e2cb00058e77b7b72a298425cd1b941ad4ec65e8afc50303a22c0f99b0509b4c895f40",
        },
        .{
            .msg = "abcdef0123456789",
            .uniform_hex = "ef904a29bffc4cf9ee82832451c946ac3c8f8058ae97d8d629831a74c6572bd9ebd0df635cd1f208e2038e760c4994984ce73f0d55ea9f22af83ba4734569d4bc95e18350f740c07eef653cbb9f87910d833751825f0ebefa1abe5420bb52be14cf489b37fe1a72f7de2d10be453b2c9d9eb20c7e3f6edc5a60629178d9478df",
        },
        .{
            .msg = "q128_" ++ "q" ** 128,
            .uniform_hex = "80be107d0884f0d881bb460322f0443d38bd222db8bd0b0a5312a6fedb49c1bbd88fd75d8b9a09486c60123dfa1d73c1cc3169761b17476d3c6b7cbbd727acd0e2c942f4dd96ae3da5de368d26b32286e32de7e5a8cb2949f866a0b80c58116b29fa7fabb3ea7d520ee603e0c25bcaf0b9a5e92ec6a1fe4e0391d1cdbce8c68a",
        },
        .{
            .msg = "a512_" ++ "a" ** 512,
            .uniform_hex = "546aff5444b5b79aa6148bd81728704c32decb73a3ba76e9e75885cad9def1d06d6792f8a7d12794e90efed817d96920d728896a4510864370c207f99bd4a608ea121700ef01ed879745ee3e4ceef777eda6d9e5e38b90c86ea6fb0b36504ba4a45d22e86f6db5dd43d98a294bebb9125d5b794e9d2a81181066eb954966a487",
        },
    };
    for (vectors) |vec| {
        const expected = hexN(128, vec.uniform_hex);
        const got = expandMessageXmd(128, vec.msg, k1_dst);
        try std.testing.expectEqualSlices(u8, &expected, &got);
    }
}

// -- hash_to_field: RFC 9380 Appendix J.9.1 (G1, Fp) / J.10.1 (G2, Fp2)
//   `u[0]`/`u[1]` intermediates, all 5 published messages each. REAL,
//   PASSING today (unlike the hashToCurve* tests further down, which
//   additionally need the still-stubbed map_to_curve).

const g1_dst = "QUUX-V01-CS02-with-BLS12381G1_XMD:SHA-256_SSWU_RO_";
const g2_dst = "QUUX-V01-CS02-with-BLS12381G2_XMD:SHA-256_SSWU_RO_";

const G1HashToFieldVec = struct { msg: []const u8, u0: []const u8, u1: []const u8 };
const g1_j91_vectors = [_]G1HashToFieldVec{
    .{
        .msg = "",
        .u0 = "0ba14bd907ad64a016293ee7c2d276b8eae71f25a4b941eece7b0d89f17f75cb3ae5438a614fb61d6835ad59f29c564f",
        .u1 = "019b9bd7979f12657976de2884c7cce192b82c177c80e0ec604436a7f538d231552f0d96d9f7babe5fa3b19b3ff25ac9",
    },
    .{
        .msg = "abc",
        .u0 = "0d921c33f2bad966478a03ca35d05719bdf92d347557ea166e5bba579eea9b83e9afa5c088573c2281410369fbd32951",
        .u1 = "003574a00b109ada2f26a37a91f9d1e740dffd8d69ec0c35e1e9f4652c7dba61123e9dd2e76c655d956e2b3462611139",
    },
    .{
        .msg = "abcdef0123456789",
        .u0 = "062d1865eb80ebfa73dcfc45db1ad4266b9f3a93219976a3790ab8d52d3e5f1e62f3b01795e36834b17b70e7b76246d4",
        .u1 = "0cdc3e2f271f29c4ff75020857ce6c5d36008c9b48385ea2f2bf6f96f428a3deb798aa033cd482d1cdc8b30178b08e3a",
    },
    .{
        .msg = "q128_" ++ "q" ** 128,
        .u0 = "010476f6a060453c0b1ad0b628f3e57c23039ee16eea5e71bb87c3b5419b1255dc0e5883322e563b84a29543823c0e86",
        .u1 = "0b1a912064fb0554b180e07af7e787f1f883a0470759c03c1b6509eb8ce980d1670305ae7b928226bb58fdc0a419f46e",
    },
    .{
        .msg = "a512_" ++ "a" ** 512,
        .u0 = "0a8ffa7447f6be1c5a2ea4b959c9454b431e29ccc0802bc052413a9c5b4f9aac67a93431bd480d15be1e057c8a08e8c6",
        .u1 = "05d487032f602c90fa7625dbafe0f4a49ef4a6b0b33d7bb349ff4cf5410d297fd6241876e3e77b651cfc8191e40a68b7",
    },
};

test "hashToFieldFp: RFC 9380 Appendix J.9.1 u[0]/u[1], all 5 messages" {
    for (g1_j91_vectors) |vec| {
        const expected_u0 = try Fp.fromBytes(hexN(48, vec.u0));
        const expected_u1 = try Fp.fromBytes(hexN(48, vec.u1));
        const u = hashToFieldFp(2, vec.msg, g1_dst);
        try std.testing.expect(u[0].eql(expected_u0));
        try std.testing.expect(u[1].eql(expected_u1));
    }
}

const G2HashToFieldVec = struct {
    msg: []const u8,
    u0_c0: []const u8,
    u0_c1: []const u8,
    u1_c0: []const u8,
    u1_c1: []const u8,
};
const g2_j101_vectors = [_]G2HashToFieldVec{
    .{
        .msg = "",
        .u0_c0 = "03dbc2cce174e91ba93cbb08f26b917f98194a2ea08d1cce75b2b9cc9f21689d80bd79b594a613d0a68eb807dfdc1cf8",
        .u0_c1 = "05a2acec64114845711a54199ea339abd125ba38253b70a92c876df10598bd1986b739cad67961eb94f7076511b3b39a",
        .u1_c0 = "02f99798e8a5acdeed60d7e18e9120521ba1f47ec090984662846bc825de191b5b7641148c0dbc237726a334473eee94",
        .u1_c1 = "145a81e418d4010cc027a68f14391b30074e89e60ee7a22f87217b2f6eb0c4b94c9115b436e6fa4607e95a98de30a435",
    },
    .{
        .msg = "abc",
        .u0_c0 = "15f7c0aa8f6b296ab5ff9c2c7581ade64f4ee6f1bf18f55179ff44a2cf355fa53dd2a2158c5ecb17d7c52f63e7195771",
        .u0_c1 = "01c8067bf4c0ba709aa8b9abc3d1cef589a4758e09ef53732d670fd8739a7274e111ba2fcaa71b3d33df2a3a0c8529dd",
        .u1_c0 = "187111d5e088b6b9acfdfad078c4dacf72dcd17ca17c82be35e79f8c372a693f60a033b461d81b025864a0ad051a06e4",
        .u1_c1 = "08b852331c96ed983e497ebc6dee9b75e373d923b729194af8e72a051ea586f3538a6ebb1e80881a082fa2b24df9f566",
    },
    .{
        .msg = "abcdef0123456789",
        .u0_c0 = "0313d9325081b415bfd4e5364efaef392ecf69b087496973b229303e1816d2080971470f7da112c4eb43053130b785e1",
        .u0_c1 = "062f84cb21ed89406890c051a0e8b9cf6c575cf6e8e18ecf63ba86826b0ae02548d83b483b79e48512b82a6c0686df8f",
        .u1_c0 = "1739123845406baa7be5c5dc74492051b6d42504de008c635f3535bb831d478a341420e67dcc7b46b2e8cba5379cca97",
        .u1_c1 = "01897665d9cb5db16a27657760bbea7951f67ad68f8d55f7113f24ba6ddd82caef240a9bfa627972279974894701d975",
    },
    .{
        .msg = "q128_" ++ "q" ** 128,
        .u0_c0 = "025820cefc7d06fd38de7d8e370e0da8a52498be9b53cba9927b2ef5c6de1e12e12f188bbc7bc923864883c57e49e253",
        .u0_c1 = "034147b77ce337a52e5948f66db0bab47a8d038e712123bb381899b6ab5ad20f02805601e6104c29df18c254b8618c7b",
        .u1_c0 = "0930315cae1f9a6017c3f0c8f2314baa130e1cf13f6532bff0a8a1790cd70af918088c3db94bda214e896e1543629795",
        .u1_c1 = "10c4df2cacf67ea3cb3108b00d4cbd0b3968031ebc8eac4b1ebcefe84d6b715fde66bef0219951ece29d1facc8a520ef",
    },
    .{
        .msg = "a512_" ++ "a" ** 512,
        .u0_c0 = "190b513da3e66fc9a3587b78c76d1d132b1152174d0b83e3c1114066392579a45824c5fa17649ab89299ddd4bda54935",
        .u0_c1 = "12ab625b0fe0ebd1367fe9fac57bb1168891846039b4216b9d94007b674de2d79126870e88aeef54b2ec717a887dcf39",
        .u1_c0 = "0e6a42010cf435fb5bacc156a585e1ea3294cc81d0ceb81924d95040298380b164f702275892cedd81b62de3aba3f6b5",
        .u1_c1 = "117d9a0defc57a33ed208428cb84e54c85a6840e7648480ae428838989d25d97a0af8e3255be62b25c2a85630d2dddd8",
    },
};

test "hashToFieldFp2: RFC 9380 Appendix J.10.1 u[0]/u[1] (c0, c1), all 5 messages" {
    for (g2_j101_vectors) |vec| {
        const expected_u0: Fp2 = .{
            .c0 = try Fp.fromBytes(hexN(48, vec.u0_c0)),
            .c1 = try Fp.fromBytes(hexN(48, vec.u0_c1)),
        };
        const expected_u1: Fp2 = .{
            .c0 = try Fp.fromBytes(hexN(48, vec.u1_c0)),
            .c1 = try Fp.fromBytes(hexN(48, vec.u1_c1)),
        };
        const u = hashToFieldFp2(2, vec.msg, g2_dst);
        try std.testing.expect(u[0].eql(expected_u0));
        try std.testing.expect(u[1].eql(expected_u1));
    }
}

// -- mapToCurveG1/G2: RFC 9380 Appendix J.9.1 / J.10.1 `Q0`/`Q1`
//   intermediates (the mapped points BEFORE the final add +
//   clear_cofactor), all 5 messages each — these localize any bug to
//   the SSWU+isogeny map specifically (hash_to_field is pinned by the
//   `u[]` tests above; the final-P tests below additionally pin the
//   add + h_eff clear_cofactor tail). Vector values extracted
//   programmatically from the RFC's raw text (same sourcing as the
//   coefficient tables — see NOTICE).

const G1MapVec = struct { q0x: []const u8, q0y: []const u8, q1x: []const u8, q1y: []const u8 };
const g1_j91_map_vectors = [_]G1MapVec{
    .{
        .q0x = "11a3cce7e1d90975990066b2f2643b9540fa40d6137780df4e753a8054d07580db3b7f1f03396333d4a359d1fe3766fe",
        .q0y = "0eeaf6d794e479e270da10fdaf768db4c96b650a74518fc67b04b03927754bac66f3ac720404f339ecdcc028afa091b7",
        .q1x = "160003aaf1632b13396dbad518effa00fff532f604de1a7fc2082ff4cb0afa2d63b2c32da1bef2bf6c5ca62dc6b72f9c",
        .q1y = "0d8bb2d14e20cf9f6036152ed386d79189415b6d015a20133acb4e019139b94e9c146aaad5817f866c95d609a361735e",
    },
    .{
        .q0x = "125435adce8e1cbd1c803e7123f45392dc6e326d292499c2c45c5865985fd74fe8f042ecdeeec5ecac80680d04317d80",
        .q0y = "0e8828948c989126595ee30e4f7c931cbd6f4570735624fd25aef2fa41d3f79cfb4b4ee7b7e55a8ce013af2a5ba20bf2",
        .q1x = "11def93719829ecda3b46aa8c31fc3ac9c34b428982b898369608e4f042babee6c77ab9218aad5c87ba785481eff8ae4",
        .q1y = "0007c9cef122ccf2efd233d6eb9bfc680aa276652b0661f4f820a653cec1db7ff69899f8e52b8e92b025a12c822a6ce6",
    },
    .{
        .q0x = "08834484878c217682f6d09a4b51444802fdba3d7f2df9903a0ddadb92130ebbfa807fffa0eabf257d7b48272410afff",
        .q0y = "0b318f7ecf77f45a0f038e62d7098221d2dbbca2a394164e2e3fe953dc714ac2cde412d8f2d7f0c03b259e6795a2508e",
        .q1x = "158418ed6b27e2549f05531a8281b5822b31c3bf3144277fbb977f8d6e2694fedceb7011b3c2b192f23e2a44b2bd106e",
        .q1y = "1879074f344471fac5f839e2b4920789643c075792bec5af4282c73f7941cda5aa77b00085eb10e206171b9787c4169f",
    },
    .{
        .q0x = "0cbd7f84ad2c99643fea7a7ac8f52d63d66cefa06d9a56148e58b984b3dd25e1f41ff47154543343949c64f88d48a710",
        .q0y = "052c00e4ed52d000d94881a5638ae9274d3efc8bc77bc0e5c650de04a000b2c334a9e80b85282a00f3148dfdface0865",
        .q1x = "06493fb68f0d513af08be0372f849436a787e7b701ae31cb964d968021d6ba6bd7d26a38aaa5a68e8c21a6b17dc8b579",
        .q1y = "02e98f2ccf5802b05ffaac7c20018bc0c0b2fd580216c4aa2275d2909dc0c92d0d0bdc979226adeb57a29933536b6bb4",
    },
    .{
        .q0x = "0cf97e6dbd0947857f3e578231d07b309c622ade08f2c08b32ff372bd90db19467b2563cc997d4407968d4ac80e154f8",
        .q0y = "127f0cddf2613058101a5701f4cb9d0861fd6c2a1b8e0afe194fccf586a3201a53874a2761a9ab6d7220c68661a35ab3",
        .q1x = "092f1acfa62b05f95884c6791fba989bbe58044ee6355d100973bf9553ade52b47929264e6ae770fb264582d8dce512a",
        .q1y = "028e6d0169a72cfedb737be45db6c401d3adfb12c58c619c82b93a5dfcccef12290de530b0480575ddc8397cda0bbebf",
    },
};

test "mapToCurveG1: RFC 9380 Appendix J.9.1 Q0/Q1 intermediates, all 5 messages" {
    for (g1_j91_vectors, g1_j91_map_vectors) |uv, mv| {
        const u = hashToFieldFp(2, uv.msg, g1_dst);
        const q0 = mapToCurveG1(u[0]);
        const q1 = mapToCurveG1(u[1]);
        try std.testing.expect(g1.Jacobian.fromAffine(q0).isOnCurve());
        try std.testing.expect(g1.Jacobian.fromAffine(q1).isOnCurve());
        try std.testing.expect(q0.x.eql(try Fp.fromBytes(hexN(48, mv.q0x))));
        try std.testing.expect(q0.y.eql(try Fp.fromBytes(hexN(48, mv.q0y))));
        try std.testing.expect(q1.x.eql(try Fp.fromBytes(hexN(48, mv.q1x))));
        try std.testing.expect(q1.y.eql(try Fp.fromBytes(hexN(48, mv.q1y))));
    }
}

const G2MapVec = struct {
    q0x_c0: []const u8,
    q0x_c1: []const u8,
    q0y_c0: []const u8,
    q0y_c1: []const u8,
    q1x_c0: []const u8,
    q1x_c1: []const u8,
    q1y_c0: []const u8,
    q1y_c1: []const u8,
};
const g2_j101_map_vectors = [_]G2MapVec{
    .{
        .q0x_c0 = "019ad3fc9c72425a998d7ab1ea0e646a1f6093444fc6965f1cad5a3195a7b1e099c050d57f45e3fa191cc6d75ed7458c",
        .q0x_c1 = "171c88b0b0efb5eb2b88913a9e74fe111a4f68867b59db252ce5868af4d1254bfab77ebde5d61cd1a86fb2fe4a5a1c1d",
        .q0y_c0 = "0ba10604e62bdd9eeeb4156652066167b72c8d743b050fb4c1016c31b505129374f76e03fa127d6a156213576910fef3",
        .q0y_c1 = "0eb22c7a543d3d376e9716a49b72e79a89c9bfe9feee8533ed931cbb5373dde1fbcd7411d8052e02693654f71e15410a",
        .q1x_c0 = "113d2b9cd4bd98aee53470b27abc658d91b47a78a51584f3d4b950677cfb8a3e99c24222c406128c91296ef6b45608be",
        .q1x_c1 = "13855912321c5cb793e9d1e88f6f8d342d49c0b0dbac613ee9e17e3c0b3c97dfbb5a49cc3fb45102fdbaf65e0efe2632",
        .q1y_c0 = "0fd3def0b7574a1d801be44fde617162aa2e89da47f464317d9bb5abc3a7071763ce74180883ad7ad9a723a9afafcdca",
        .q1y_c1 = "056f617902b3c0d0f78a9a8cbda43a26b65f602f8786540b9469b060db7b38417915b413ca65f875c130bebfaa59790c",
    },
    .{
        .q0x_c0 = "12b2e525281b5f4d2276954e84ac4f42cf4e13b6ac4228624e17760faf94ce5706d53f0ca1952f1c5ef75239aeed55ad",
        .q0x_c1 = "05d8a724db78e570e34100c0bc4a5fa84ad5839359b40398151f37cff5a51de945c563463c9efbdda569850ee5a53e77",
        .q0y_c0 = "02eacdc556d0bdb5d18d22f23dcb086dd106cad713777c7e6407943edbe0b3d1efe391eedf11e977fac55f9b94f2489c",
        .q0y_c1 = "04bbe48bfd5814648d0b9e30f0717b34015d45a861425fabc1ee06fdfce36384ae2c808185e693ae97dcde118f34de41",
        .q1x_c0 = "19f18cc5ec0c2f055e47c802acc3b0e40c337256a208001dde14b25afced146f37ea3d3ce16834c78175b3ed61f3c537",
        .q1x_c1 = "15b0dadc256a258b4c68ea43605dffa6d312eef215c19e6474b3e101d33b661dfee43b51abbf96fee68fc6043ac56a58",
        .q1y_c0 = "05e47c1781286e61c7ade887512bd9c2cb9f640d3be9cf87ea0bad24bd0ebfe946497b48a581ab6c7d4ca74b5147287f",
        .q1y_c1 = "19f98db2f4a1fcdf56a9ced7b320ea9deecf57c8e59236b0dc21f6ee7229aa9705ce9ac7fe7a31c72edca0d92370c096",
    },
    .{
        .q0x_c0 = "0f48f1ea1318ddb713697708f7327781fb39718971d72a9245b9731faaca4dbaa7cca433d6c434a820c28b18e20ea208",
        .q0x_c1 = "06051467c8f85da5ba2540974758f7a1e0239a5981de441fdd87680a995649c211054869c50edbac1f3a86c561ba3162",
        .q0y_c0 = "168b3d6df80069dbbedb714d41b32961ad064c227355e1ce5fac8e105de5e49d77f0c64867f3834848f152497eb76333",
        .q0y_c1 = "134e0e8331cee8cb12f9c2d0742714ed9eee78a84d634c9a95f6a7391b37125ed48bfc6e90bf3546e99930ff67cc97bc",
        .q1x_c0 = "004fd03968cd1c99a0dd84551f44c206c84dcbdb78076c5bfee24e89a92c8508b52b88b68a92258403cbe1ea2da3495f",
        .q1x_c1 = "1674338ea298281b636b2eb0fe593008d03171195fd6dcd4531e8a1ed1f02a72da238a17a635de307d7d24aa2d969a47",
        .q1y_c0 = "0dc7fa13fff6b12558419e0a1e94bfc3cfaf67238009991c5f24ee94b632c3d09e27eca329989aee348a67b50d5e236c",
        .q1y_c1 = "169585e164c131103d85324f2d7747b23b91d66ae5d947c449c8194a347969fc6bbd967729768da485ba71868df8aed2",
    },
    .{
        .q0x_c0 = "09eccbc53df677f0e5814e3f86e41e146422834854a224bf5a83a50e4cc0a77bfc56718e8166ad180f53526ea9194b57",
        .q0x_c1 = "0c3633943f91daee715277bd644fba585168a72f96ded64fc5a384cce4ec884a4c3c30f08e09cd2129335dc8f67840ec",
        .q0y_c0 = "0eb6186a0457d5b12d132902d4468bfeb7315d83320b6c32f1c875f344efcba979952b4aa418589cb01af712f98cc555",
        .q0y_c1 = "119e3cf167e69eb16c1c7830e8df88856d48be12e3ff0a40791a5cd2f7221311d4bf13b1847f371f467357b3f3c0b4c7",
        .q1x_c0 = "0eb3aabc1ddfce17ff18455fcc7167d15ce6b60ddc9eb9b59f8d40ab49420d35558686293d046fc1e42f864b7f60e381",
        .q1x_c1 = "198bdfb19d7441ebcca61e8ff774b29d17da16547d2c10c273227a635cacea3f16826322ae85717630f0867539b5ed8b",
        .q1y_c0 = "0aaf1dee3adf3ed4c80e481c09b57ea4c705e1b8d25b897f0ceeec3990748716575f92abff22a1c8f4582aff7b872d52",
        .q1y_c1 = "0d058d9061ed27d4259848a06c96c5ca68921a5d269b078650c882cb3c2bd424a8702b7a6ee4e0ead9982baf6843e924",
    },
    .{
        .q0x_c0 = "17cadf8d04a1a170f8347d42856526a24cc466cb2ddfd506cff01191666b7f944e31244d662c904de5440516a2b09004",
        .q0x_c1 = "0d13ba91f2a8b0051cf3279ea0ee63a9f19bc9cb8bfcc7d78b3cbd8cc4fc43ba726774b28038213acf2b0095391c523e",
        .q0y_c0 = "17ef19497d6d9246fa94d35575c0f8d06ee02f21a284dbeaa78768cb1e25abd564e3381de87bda26acd04f41181610c5",
        .q0y_c1 = "12c3c913ba4ed03c24f0721a81a6be7430f2971ffca8fd1729aafe496bb725807531b44b34b59b3ae5495e5a2dcbd5c8",
        .q1x_c0 = "16ec57b7fe04c71dfe34fb5ad84dbce5a2dbbd6ee085f1d8cd17f45e8868976fc3c51ad9eeda682c7869024d24579bfd",
        .q1x_c1 = "13103f7aace1ae1420d208a537f7d3a9679c287208026e4e3439ab8cd534c12856284d95e27f5e1f33eec2ce656533b0",
        .q1y_c0 = "0958b2c4c2c10fcef5a6c59b9e92c4a67b0fae3e2e0f1b6b5edad9c940b8f3524ba9ebbc3f2ceb3cfe377655b3163bd7",
        .q1y_c1 = "0ccb594ed8bd14ca64ed9cb4e0aba221be540f25dd0d6ba15a4a4be5d67bcf35df7853b2d8dad3ba245f1ea3697f66aa",
    },
};

fn fp2FromHexPair(c0_hex: []const u8, c1_hex: []const u8) !Fp2 {
    return .{
        .c0 = try Fp.fromBytes(hexN(48, c0_hex)),
        .c1 = try Fp.fromBytes(hexN(48, c1_hex)),
    };
}

test "mapToCurveG2: RFC 9380 Appendix J.10.1 Q0/Q1 intermediates, all 5 messages" {
    for (g2_j101_vectors, g2_j101_map_vectors) |uv, mv| {
        const u = hashToFieldFp2(2, uv.msg, g2_dst);
        const q0 = mapToCurveG2(u[0]);
        const q1 = mapToCurveG2(u[1]);
        try std.testing.expect(g2.Jacobian.fromAffine(q0).isOnCurve());
        try std.testing.expect(g2.Jacobian.fromAffine(q1).isOnCurve());
        try std.testing.expect(q0.x.eql(try fp2FromHexPair(mv.q0x_c0, mv.q0x_c1)));
        try std.testing.expect(q0.y.eql(try fp2FromHexPair(mv.q0y_c0, mv.q0y_c1)));
        try std.testing.expect(q1.x.eql(try fp2FromHexPair(mv.q1x_c0, mv.q1x_c1)));
        try std.testing.expect(q1.y.eql(try fp2FromHexPair(mv.q1y_c0, mv.q1y_c1)));
    }
}

// -- hashToCurveG1/G2: RFC 9380 Appendix J.9.1 / J.10.1 final points
//   `P`, all 5 messages each — the byte-exact oracle for the whole
//   Part-3 composition (hash_to_field → SSWU → isogeny → add → h_eff
//   clear_cofactor).

const G1CurveVec = struct { msg: []const u8, px: []const u8, py: []const u8 };
const g1_j91_curve_vectors = [_]G1CurveVec{
    .{
        .msg = "",
        .px = "052926add2207b76ca4fa57a8734416c8dc95e24501772c814278700eed6d1e4e8cf62d9c09db0fac349612b759e79a1",
        .py = "08ba738453bfed09cb546dbb0783dbb3a5f1f566ed67bb6be0e8c67e2e81a4cc68ee29813bb7994998f3eae0c9c6a265",
    },
    .{
        .msg = "abc",
        .px = "03567bc5ef9c690c2ab2ecdf6a96ef1c139cc0b2f284dca0a9a7943388a49a3aee664ba5379a7655d3c68900be2f6903",
        .py = "0b9c15f3fe6e5cf4211f346271d7b01c8f3b28be689c8429c85b67af215533311f0b8dfaaa154fa6b88176c229f2885d",
    },
    .{
        .msg = "abcdef0123456789",
        .px = "11e0b079dea29a68f0383ee94fed1b940995272407e3bb916bbf268c263ddd57a6a27200a784cbc248e84f357ce82d98",
        .py = "03a87ae2caf14e8ee52e51fa2ed8eefe80f02457004ba4d486d6aa1f517c0889501dc7413753f9599b099ebcbbd2d709",
    },
    .{
        .msg = "q128_" ++ "q" ** 128,
        .px = "15f68eaa693b95ccb85215dc65fa81038d69629f70aeee0d0f677cf22285e7bf58d7cb86eefe8f2e9bc3f8cb84fac488",
        .py = "1807a1d50c29f430b8cafc4f8638dfeeadf51211e1602a5f184443076715f91bb90a48ba1e370edce6ae1062f5e6dd38",
    },
    .{
        .msg = "a512_" ++ "a" ** 512,
        .px = "082aabae8b7dedb0e78aeb619ad3bfd9277a2f77ba7fad20ef6aabdc6c31d19ba5a6d12283553294c1825c4b3ca2dcfe",
        .py = "05b84ae5a942248eea39e1d91030458c40153f3b654ab7872d779ad1e942856a20c438e8d99bc8abfbf74729ce1f7ac8",
    },
};

test "hashToCurveG1: RFC 9380 Appendix J.9.1 final P, all 5 messages" {
    for (g1_j91_curve_vectors) |vec| {
        const expected_x = try Fp.fromBytes(hexN(48, vec.px));
        const expected_y = try Fp.fromBytes(hexN(48, vec.py));
        const p = hashToCurveG1(vec.msg, g1_dst);
        try std.testing.expect(!p.infinity);
        try std.testing.expect(p.x.eql(expected_x));
        try std.testing.expect(p.y.eql(expected_y));
    }
}

const G2CurveVec = struct { msg: []const u8, px_c0: []const u8, px_c1: []const u8, py_c0: []const u8, py_c1: []const u8 };
const g2_j101_curve_vectors = [_]G2CurveVec{
    .{
        .msg = "",
        .px_c0 = "0141ebfbdca40eb85b87142e130ab689c673cf60f1a3e98d69335266f30d9b8d4ac44c1038e9dcdd5393faf5c41fb78a",
        .px_c1 = "05cb8437535e20ecffaef7752baddf98034139c38452458baeefab379ba13dff5bf5dd71b72418717047f5b0f37da03d",
        .py_c0 = "0503921d7f6a12805e72940b963c0cf3471c7b2a524950ca195d11062ee75ec076daf2d4bc358c4b190c0c98064fdd92",
        .py_c1 = "12424ac32561493f3fe3c260708a12b7c620e7be00099a974e259ddc7d1f6395c3c811cdd19f1e8dbf3e9ecfdcbab8d6",
    },
    .{
        .msg = "abc",
        .px_c0 = "02c2d18e033b960562aae3cab37a27ce00d80ccd5ba4b7fe0e7a210245129dbec7780ccc7954725f4168aff2787776e6",
        .px_c1 = "139cddbccdc5e91b9623efd38c49f81a6f83f175e80b06fc374de9eb4b41dfe4ca3a230ed250fbe3a2acf73a41177fd8",
        .py_c0 = "1787327b68159716a37440985269cf584bcb1e621d3a7202be6ea05c4cfe244aeb197642555a0645fb87bf7466b2ba48",
        .py_c1 = "00aa65dae3c8d732d10ecd2c50f8a1baf3001578f71c694e03866e9f3d49ac1e1ce70dd94a733534f106d4cec0eddd16",
    },
    .{
        .msg = "abcdef0123456789",
        .px_c0 = "121982811d2491fde9ba7ed31ef9ca474f0e1501297f68c298e9f4c0028add35aea8bb83d53c08cfc007c1e005723cd0",
        .px_c1 = "190d119345b94fbd15497bcba94ecf7db2cbfd1e1fe7da034d26cbba169fb3968288b3fafb265f9ebd380512a71c3f2c",
        .py_c0 = "05571a0f8d3c08d094576981f4a3b8eda0a8e771fcdcc8ecceaf1356a6acf17574518acb506e435b639353c2e14827c8",
        .py_c1 = "0bb5e7572275c567462d91807de765611490205a941a5a6af3b1691bfe596c31225d3aabdf15faff860cb4ef17c7c3be",
    },
    .{
        .msg = "q128_" ++ "q" ** 128,
        .px_c0 = "19a84dd7248a1066f737cc34502ee5555bd3c19f2ecdb3c7d9e24dc65d4e25e50d83f0f77105e955d78f4762d33c17da",
        .px_c1 = "0934aba516a52d8ae479939a91998299c76d39cc0c035cd18813bec433f587e2d7a4fef038260eef0cef4d02aae3eb91",
        .py_c0 = "14f81cd421617428bc3b9fe25afbb751d934a00493524bc4e065635b0555084dd54679df1536101b2c979c0152d09192",
        .py_c1 = "09bcccfa036b4847c9950780733633f13619994394c23ff0b32fa6b795844f4a0673e20282d07bc69641cee04f5e5662",
    },
    .{
        .msg = "a512_" ++ "a" ** 512,
        .px_c0 = "01a6ba2f9a11fa5598b2d8ace0fbe0a0eacb65deceb476fbbcb64fd24557c2f4b18ecfc5663e54ae16a84f5ab7f62534",
        .px_c1 = "11fca2ff525572795a801eed17eb12785887c7b63fb77a42be46ce4a34131d71f7a73e95fee3f812aea3de78b4d01569",
        .py_c0 = "0b6798718c8aed24bc19cb27f866f1c9effcdbf92397ad6448b5c9db90d2b9da6cbabf48adc1adf59a1a28344e79d57e",
        .py_c1 = "03a47f8e6d1763ba0cad63d6114c0accbef65707825a511b251a660a9b3994249ae4e63fac38b23da0c398689ee2ab52",
    },
};

test "hashToCurveG2: RFC 9380 Appendix J.10.1 final P, all 5 messages" {
    for (g2_j101_curve_vectors) |vec| {
        const expected_x: Fp2 = .{
            .c0 = try Fp.fromBytes(hexN(48, vec.px_c0)),
            .c1 = try Fp.fromBytes(hexN(48, vec.px_c1)),
        };
        const expected_y: Fp2 = .{
            .c0 = try Fp.fromBytes(hexN(48, vec.py_c0)),
            .c1 = try Fp.fromBytes(hexN(48, vec.py_c1)),
        };
        const p = hashToCurveG2(vec.msg, g2_dst);
        try std.testing.expect(!p.infinity);
        try std.testing.expect(p.x.eql(expected_x));
        try std.testing.expect(p.y.eql(expected_y));
    }
}

// -- Property tests: every hash/encode output must be on-curve AND in
//   the order-r subgroup (RFC 9380 §3's output contract), for messages
//   the RFC publishes no vectors for.

test "hashToCurveG1/encodeToCurveG1: on-curve + r-subgroup for arbitrary messages" {
    const msgs = [_][]const u8{ "zig-libs bls12_381 property test", "\x00\xff binary\x00message" };
    for (msgs) |m| {
        const p = g1.Jacobian.fromAffine(hashToCurveG1(m, g1_dst));
        try std.testing.expect(!p.isIdentity());
        try std.testing.expect(p.isOnCurve());
        try std.testing.expect(p.subgroupCheck());
    }
    const e = g1.Jacobian.fromAffine(encodeToCurveG1("encode-to-curve property test", g1_dst));
    try std.testing.expect(e.isOnCurve());
    try std.testing.expect(e.subgroupCheck());
}

test "hashToCurveG2/encodeToCurveG2: on-curve + r-subgroup for arbitrary messages" {
    const msgs = [_][]const u8{ "zig-libs bls12_381 property test", "\x00\xff binary\x00message" };
    for (msgs) |m| {
        const p = g2.Jacobian.fromAffine(hashToCurveG2(m, g2_dst));
        try std.testing.expect(!p.isIdentity());
        try std.testing.expect(p.isOnCurve());
        try std.testing.expect(p.subgroupCheck());
    }
    const e = g2.Jacobian.fromAffine(encodeToCurveG2("encode-to-curve property test", g2_dst));
    try std.testing.expect(e.isOnCurve());
    try std.testing.expect(e.subgroupCheck());
}

test "sswuG1/sswuG2: u = 0 exercises the tv1 == 0 exceptional branch, lands on E1'/E2'" {
    // On-E1' check done manually (E1PrimeAffine is not a g1.Jacobian):
    // y^2 == x^3 + A'x + B'.
    const p1 = sswuG1(Fp.zero);
    const rhs1 = p1.x.square().mul(p1.x).add(g1_iso_a.mul(p1.x)).add(g1_iso_b);
    try std.testing.expect(p1.y.square().eql(rhs1));
    const p2 = sswuG2(Fp2.zero);
    const rhs2 = p2.x.square().mul(p2.x).add(g2_iso_a.mul(p2.x)).add(g2_iso_b);
    try std.testing.expect(p2.y.square().eql(rhs2));
    // sgn0(u) == sgn0(0) == 0, so the returned y must have sgn0 == 0.
    try std.testing.expectEqual(@as(u1, 0), p1.y.sgn0());
    try std.testing.expectEqual(@as(u1, 0), p2.y.sgn0());
}

test "iso curve constants: g1_iso_z is nonzero, g2_iso_a/g2_iso_b decode as documented" {
    try std.testing.expect(!g1_iso_z.isZero());
    try std.testing.expect(g2_iso_a.c0.isZero());
    try std.testing.expectEqual(@as(u8, 240), g2_iso_a.c1.toBytes()[47]);
    try std.testing.expect(g2_iso_b.c0.eql(g2_iso_b.c1));
}
