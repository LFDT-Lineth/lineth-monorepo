//! secp256k1 ecrecover and verify over `std.crypto.ecc.Secp256k1`.
//!
//! Same signatures as the Constantine bindings in `guest_crypto.zig`, so `zkvm_provide.zig` can
//! select either at comptime.

const std = @import("std");

const Secp256k1 = std.crypto.ecc.Secp256k1;
const Scalar = Secp256k1.scalar.Scalar;

/// Recovers the raw 64-byte x‖y public key from a pre-hash and compact big-endian r‖s signature.
/// `recid` selects the y parity as 0/1 or Ethereum's 27/28.
pub fn ecrecover(msg: *const [32]u8, sig: *const [64]u8, recid: u8, output: *[64]u8) bool {
    const parity: bool = switch (recid) {
        0, 27 => false,
        1, 28 => true,
        else => return false,
    };

    const r_scalar = Scalar.fromBytes(sig[0..32].*, .big) catch return false;
    const s_scalar = Scalar.fromBytes(sig[32..64].*, .big) catch return false;
    if (r_scalar.isZero() or s_scalar.isZero()) return false;

    // R is the curve point with x = r; r < n < p, so there is no wrap-around candidate.
    const r_fe = Secp256k1.Fe.fromBytes(sig[0..32].*, .big) catch return false;
    const r_y = Secp256k1.recoverY(r_fe, parity) catch return false;
    const R = Secp256k1.fromAffineCoordinates(.{ .x = r_fe, .y = r_y }) catch return false;

    // Q = r⁻¹·(s·R − e·G)
    const r_inv = r_scalar.invert();
    const u_r = r_inv.mul(s_scalar).toBytes(.big);
    const u_g = r_inv.mul(reduceToScalar(msg).neg()).toBytes(.big);
    const Q = Secp256k1.mulDoubleBasePublic(R, u_r, Secp256k1.basePoint, u_g, .big) catch return false;
    Q.rejectIdentity() catch return false;

    const uncompressed = Q.toUncompressedSec1(); // 0x04 ‖ x ‖ y
    @memcpy(output, uncompressed[1..65]);
    return true;
}

/// Verifies a compact big-endian r‖s signature against a pre-hash and raw 64-byte x‖y key.
pub fn secp256k1Verify(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8, verified: *bool) void {
    verified.* = verifyInner(msg, sig, pubkey) catch false;
}

fn verifyInner(msg: *const [32]u8, sig: *const [64]u8, pubkey: *const [64]u8) !bool {
    var sec1: [65]u8 = undefined;
    sec1[0] = 0x04;
    @memcpy(sec1[1..65], pubkey);
    const Q = try Secp256k1.fromSec1(&sec1);

    const r_scalar = try Scalar.fromBytes(sig[0..32].*, .big);
    const s_scalar = try Scalar.fromBytes(sig[32..64].*, .big);
    if (r_scalar.isZero() or s_scalar.isZero()) return false;

    const s_inv = s_scalar.invert();
    const u_g = reduceToScalar(msg).mul(s_inv).toBytes(.big);
    const u_q = r_scalar.mul(s_inv).toBytes(.big);

    const R = try Secp256k1.mulDoubleBasePublic(Secp256k1.basePoint, u_g, Q, u_q, .big);
    try R.rejectIdentity();

    // R.x lives in F_p with p > n, so reduce it mod n rather than rejecting x ≥ n.
    return reduceToScalar(&R.affineCoordinates().x.toBytes(.big)).equivalent(r_scalar);
}

fn reduceToScalar(bytes: *const [32]u8) Scalar {
    var wide: [64]u8 = .{0} ** 64;
    @memcpy(wide[32..64], bytes);
    return Scalar.fromBytes64(wide, .big);
}
