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

/// `0x01 ‖ sha256(commitment)[1..]`: the hash L1 anchors a blob by.
pub fn versionedHash(commitment: *const [48]u8) [32]u8 {
    var hash: [32]u8 = undefined;
    Sha256.hash(commitment, &hash, .{});
    hash[0] = 0x01;
    return hash;
}
