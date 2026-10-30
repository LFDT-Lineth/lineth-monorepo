//! ECDSA signing over secp256k1 for Ethereum: signatures over 32-byte digests with a recovery id
//! and the low-s form EIP-2 requires.

const std = @import("std");

const Curve = std.crypto.ecc.Secp256k1;

pub const order: u256 = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141;

pub const Signature = struct {
    r: [32]u8,
    s: [32]u8,
    recovery_id: u1,
};

fn mulMod(a: u256, b: u256) u256 {
    return @intCast(@as(u512, a) * b % order);
}

fn powMod(base: u256, exponent: u256) u256 {
    var result: u256 = 1;
    var bit: usize = 256;
    while (bit > 0) {
        bit -= 1;
        result = mulMod(result, result);
        if ((exponent >> @intCast(bit)) & 1 == 1) result = mulMod(result, base);
    }
    return result;
}

fn bytesOf(value: u256) [32]u8 {
    var bytes: [32]u8 = undefined;
    std.mem.writeInt(u256, &bytes, value, .big);
    return bytes;
}

/// The Ethereum address of the key with the 32-byte `secret`.
pub fn addressOf(secret: [32]u8) [20]u8 {
    const point = (Curve.basePoint.mul(secret, .big) catch unreachable).affineCoordinates();
    var coordinates: [64]u8 = undefined;
    coordinates[0..32].* = point.x.toBytes(.big);
    coordinates[32..64].* = point.y.toBytes(.big);
    var hash: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(&coordinates, &hash, .{});
    return hash[12..32].*;
}

/// Signs `digest` with a deterministic nonce derived from the key and the digest.
pub fn sign(secret: [32]u8, digest: [32]u8) Signature {
    const d = std.mem.readInt(u256, &secret, .big);
    const z = std.mem.readInt(u256, &digest, .big) % order;
    var counter: u8 = 0;
    while (true) : (counter += 1) {
        var seed: [32]u8 = undefined;
        var hasher = std.crypto.hash.sha2.Sha256.init(.{});
        hasher.update(&secret);
        hasher.update(&digest);
        hasher.update(&.{counter});
        hasher.final(&seed);
        const k = std.mem.readInt(u256, &seed, .big) % order;
        if (k == 0) continue;

        const point = (Curve.basePoint.mul(bytesOf(k), .big) catch continue).affineCoordinates();
        const r = std.mem.readInt(u256, &point.x.toBytes(.big), .big) % order;
        if (r == 0) continue;
        const sum: u512 = @as(u512, z) + mulMod(r, d);
        var s = mulMod(powMod(k, order - 2), @intCast(sum % order));
        if (s == 0) continue;
        var recovery_id: u1 = @intFromBool(point.y.isOdd());
        if (s > order / 2) {
            s = order - s;
            recovery_id ^= 1;
        }
        return .{ .r = bytesOf(r), .s = bytesOf(s), .recovery_id = recovery_id };
    }
}
