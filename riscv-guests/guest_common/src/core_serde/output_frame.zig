//! The framing of a guest's output: `keccak256(frame) || frame`, where `frame` is the 2-byte
//! big-endian schema id followed by the body.

const std = @import("std");
const zkvm_accelerators = @import("zkvm_accelerators");
const ssz = @import("guest_common").ssz;

const commitment_size: usize = 32;
const schema_id_size: usize = 2;

/// keccak256 of the frame of `value` under `schema_id`: the commitment at the head of the output.
pub fn commitment(comptime T: type, alloc: std.mem.Allocator, schema_id: u16, value: T) ssz.EncodeError![commitment_size]u8 {
    const frame = try ssz.encodeFramed(T, alloc, schema_id, value);
    defer alloc.free(frame);
    var digest: [commitment_size]u8 = undefined;
    zkvm_accelerators.keccak256(frame, &digest);
    return digest;
}

/// The output carrying `value` under `schema_id`.
pub fn encode(comptime T: type, alloc: std.mem.Allocator, schema_id: u16, value: T) ssz.EncodeError![]u8 {
    const out = try ssz.encodeFramedAfter(T, alloc, commitment_size, schema_id, value);
    zkvm_accelerators.keccak256(out[commitment_size..], out[0..commitment_size]);
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
    zkvm_accelerators.keccak256(frame, &digest);
    if (!std.mem.eql(u8, &digest, bytes[0..commitment_size])) return error.MalformedFrame;
    return frame[schema_id_size..];
}
