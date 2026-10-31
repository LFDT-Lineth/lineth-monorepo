//! Byte-level pieces of a DA stream, written without the package under test.

const std = @import("std");

const Allocator = std.mem.Allocator;
const Sha256 = std.crypto.hash.sha2.Sha256;

/// One segment of the stream: `frame` behind its length as four big-endian bytes.
pub fn lengthPrefixed(allocator: Allocator, frame: []const u8) Allocator.Error![]u8 {
    const out = try allocator.alloc(u8, 4 + frame.len);
    std.mem.writeInt(u32, out[0..4], @intCast(frame.len), .big);
    @memcpy(out[4..], frame);
    return out;
}

/// 48 bytes derived from `seed` under `label`, standing in for a commitment or a proof: a chunk
/// binding reads a commitment through its versioned hash, and only a blob verification reads a proof.
pub fn standInKzg(label: u8, seed: []const u8) [48]u8 {
    var bytes: [64]u8 = undefined;
    for ([_]u8{ 0, 1 }, 0..) |half, i| {
        var hasher = Sha256.init(.{});
        hasher.update(&.{ label, half });
        hasher.update(seed);
        hasher.final(bytes[i * 32 ..][0..32]);
    }
    return bytes[0..48].*;
}
