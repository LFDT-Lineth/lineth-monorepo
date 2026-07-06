//! Integration smoke test for the delegated precompiles (zesu-zkvm `stdlibs_accel`, imported as
//! `zesu_zkvm_accel`). The guest's in-guest precompiles delegate to this module via zkvm_provide.zig;
//! correctness of the implementation is upstream's responsibility, but this guards the pinned
//! dependency version + our import wiring by round-tripping its secp256k1 ecrecover on the host.
//!
//! std + the dependency only — no fixtures, no native crypto libs.

const std = @import("std");
const accel = @import("zesu_zkvm_accel");

const Secp256k1 = std.crypto.ecc.Secp256k1;
const Scalar = Secp256k1.scalar.Scalar;

fn scalarFromU64(v: u64) Scalar {
    var b = [_]u8{0} ** 32;
    std.mem.writeInt(u64, b[24..32], v, .big);
    return Scalar.fromBytes(b, .big) catch unreachable;
}

/// Textbook ECDSA sign (independent oracle): R = k·G, r = R.x mod n, s = k⁻¹(z + r·d),
/// recid = parity of R.y. Returns null if this nonce is unusable (R.x ≥ n, or r/s == 0).
fn signOnce(d: Scalar, k: Scalar, z: Scalar) ?struct { sig: [64]u8, recid: u8 } {
    const R = Secp256k1.basePoint.mul(k.toBytes(.little), .little) catch return null;
    const aff = R.affineCoordinates();
    const r = Scalar.fromBytes(aff.x.toBytes(.big), .big) catch return null; // skip R.x ≥ n
    if (r.isZero()) return null;
    const s = k.invert().mul(z.add(r.mul(d)));
    if (s.isZero()) return null;
    var sig: [64]u8 = undefined;
    sig[0..32].* = r.toBytes(.big);
    sig[32..].* = s.toBytes(.big);
    return .{ .sig = sig, .recid = if (aff.y.isOdd()) 1 else 0 };
}

test "delegated ecrecover round-trips signatures back to the signing key" {
    const keys = [_]u64{ 1, 2, 0xDEAD_BEEF, 0x0123_4567_89AB_CDEF, 0xFFFF_FFFF_FFFF_FFFF };
    const msgs = [_]u64{ 0xABC, 0x9999, 1, 0x0BAD_C0DE, 0xFEED_FACE_CAFE_BEEF };

    for (keys, msgs) |dv, zv| {
        const d = scalarFromU64(dv);
        const z = scalarFromU64(zv);

        const P = try Secp256k1.basePoint.mul(d.toBytes(.little), .little);
        const Paff = P.affineCoordinates();
        var expected: [64]u8 = undefined;
        expected[0..32].* = Paff.x.toBytes(.big);
        expected[32..].* = Paff.y.toBytes(.big);

        var k = scalarFromU64(2);
        var attempts: usize = 0;
        const found = while (attempts < 256) : (attempts += 1) {
            if (signOnce(d, k, z)) |sg| break sg;
            k = k.add(Scalar.one);
        } else null;
        try std.testing.expect(found != null);

        const zb = z.toBytes(.big);
        var out: [64]u8 = undefined;
        try std.testing.expect(accel.ecrecover(&zb, &found.?.sig, found.?.recid, &out));
        try std.testing.expectEqualSlices(u8, &expected, &out);
    }
}

test "delegated ecrecover rejects malformed signatures" {
    const z = scalarFromU64(0x1234).toBytes(.big);
    var out: [64]u8 = undefined;

    // r = 0 and s = 0 are invalid.
    try std.testing.expect(!accel.ecrecover(&z, &([_]u8{0} ** 64), 0, &out));

    // r ≥ n (all 0xFF) is non-canonical.
    var sig_bad_r: [64]u8 = undefined;
    sig_bad_r[0..32].* = [_]u8{0xFF} ** 32;
    sig_bad_r[32..].* = scalarFromU64(9).toBytes(.big);
    try std.testing.expect(!accel.ecrecover(&z, &sig_bad_r, 0, &out));
}
