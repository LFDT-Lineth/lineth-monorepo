//! The framing of a guest's output: `keccak256(frame) || frame`, where `frame` is the 2-byte
//! big-endian schema id followed by the body.

const std = @import("std");
const builtin = @import("builtin");

const commitment_size: usize = 32;
const schema_id_size: usize = 2;

extern fn zkvm_keccak256(data: [*]const u8, len: usize, output: *[commitment_size]u8) i32;

/// keccak256 of `data`.
fn keccak256(data: []const u8, digest: *[commitment_size]u8) void {
    if (builtin.os.tag == .freestanding) {
        _ = zkvm_keccak256(data.ptr, data.len, digest);
    } else {
        std.crypto.hash.sha3.Keccak256.hash(data, digest, .{});
    }
}

/// keccak256 of the frame of `schema_id` and `body`: the commitment at the head of the output.
/// `alloc` holds the frame while it is hashed.
pub fn commitment(alloc: std.mem.Allocator, schema_id: u16, body: []const u8) std.mem.Allocator.Error![commitment_size]u8 {
    const frame = try alloc.alloc(u8, schema_id_size + body.len);
    defer alloc.free(frame);
    std.mem.writeInt(u16, frame[0..schema_id_size], schema_id, .big);
    @memcpy(frame[schema_id_size..], body);
    var digest: [commitment_size]u8 = undefined;
    keccak256(frame, &digest);
    return digest;
}

/// The output carrying `body` under `schema_id`.
pub fn encode(alloc: std.mem.Allocator, schema_id: u16, body: []const u8) std.mem.Allocator.Error![]u8 {
    const out = try alloc.alloc(u8, commitment_size + schema_id_size + body.len);
    std.mem.writeInt(u16, out[commitment_size..][0..schema_id_size], schema_id, .big);
    @memcpy(out[commitment_size + schema_id_size ..], body);
    keccak256(out[commitment_size..], out[0..commitment_size]);
    return out;
}

/// The body of an output under `schema_id`, which aliases `bytes`. An output shorter than its
/// commitment and schema id, under another schema id or with another commitment is
/// `error.MalformedFrame`.
pub fn decode(bytes: []const u8, schema_id: u16) error{MalformedFrame}![]const u8 {
    if (bytes.len < commitment_size + schema_id_size) return error.MalformedFrame;
    const frame = bytes[commitment_size..];
    if (std.mem.readInt(u16, frame[0..schema_id_size], .big) != schema_id) return error.MalformedFrame;
    var digest: [commitment_size]u8 = undefined;
    keccak256(frame, &digest);
    if (!std.mem.eql(u8, &digest, bytes[0..commitment_size])) return error.MalformedFrame;
    return frame[schema_id_size..];
}
