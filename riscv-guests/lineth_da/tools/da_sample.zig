//! `da_sample` — builds the input of the `decompress` smoke guest: a file as the canonical payload
//! of one conflation, published as one segment in one chunk.
//!
//! usage: da_sample <payload-in> <input-out> [blob|calldata]
//!
//! The segment is the file compressed against the package dictionary behind a 4-byte length prefix.
//! A blob chunk packs it into a blob, anchored by the versioned hash of a commitment derived from
//! the segment: the decompress smoke verifies the chunk binding and the stream, and the blob's KZG
//! proof is the `blob` smoke's input.

const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");

const blob_payload = lineth_da.encoding.blob_payload;
const Sha256 = std.crypto.hash.sha2.Sha256;

const usage = "usage: da_sample <payload-in> <input-out> [blob|calldata]\n";

pub fn main(init: std.process.Init) !void {
    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 3 and args.len != 4) {
        std.debug.print("error: expected <payload-in> <input-out> [blob|calldata]\n{s}", .{usage});
        std.process.exit(2);
    }
    const as_blob = if (args.len == 4) blk: {
        if (std.mem.eql(u8, args[3], "blob")) break :blk true;
        if (std.mem.eql(u8, args[3], "calldata")) break :blk false;
        std.debug.print("error: chunk kind '{s}' is neither blob nor calldata\n{s}", .{ args[3], usage });
        std.process.exit(2);
    } else true;

    const payload = std.Io.Dir.cwd().readFileAlloc(init.io, args[1], init.gpa, .limited(1 << 30)) catch |err| {
        std.debug.print("error: cannot read '{s}': {s}\n", .{ args[1], @errorName(err) });
        std.process.exit(1);
    };
    defer init.gpa.free(payload);

    var input: std.ArrayList(u8) = .empty;
    defer input.deinit(init.gpa);
    try build(init.gpa, &input, payload, as_blob);

    std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[2], .data = input.items }) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ args[2], @errorName(err) });
        std.process.exit(1);
    };
}

fn build(allocator: std.mem.Allocator, input: *std.ArrayList(u8), payload: []const u8, as_blob: bool) !void {
    const frame = try zstd_compressor.compress(allocator, payload, .{ .dictionary = lineth_da.dictionary_v2025_04_21 });
    defer allocator.free(frame);
    var segment: std.ArrayList(u8) = .empty;
    defer segment.deinit(allocator);
    try segment.appendSlice(allocator, &std.mem.toBytes(std.mem.nativeToBig(u32, @intCast(frame.len))));
    try segment.appendSlice(allocator, frame);

    try input.appendNTimes(allocator, 0x11, 32); // parent dataRollingHash
    try input.appendSlice(allocator, &std.mem.toBytes(std.mem.nativeToBig(u32, @intCast(payload.len))));
    try input.appendSlice(allocator, payload);

    if (!as_blob) {
        var hash: [32]u8 = undefined;
        std.crypto.hash.sha3.Keccak256.hash(segment.items, &hash, .{});
        try input.append(allocator, 1);
        try input.appendSlice(allocator, &hash);
        try input.appendSlice(allocator, segment.items);
        return;
    }

    var commitment: [48]u8 = undefined;
    var proof: [48]u8 = undefined;
    standIn(&commitment, 'c', segment.items);
    standIn(&proof, 'p', segment.items);
    var hash: [32]u8 = undefined;
    Sha256.hash(&commitment, &hash, .{});
    hash[0] = 0x01; // the versioned hash of the commitment

    const blob = try allocator.create([blob_payload.blob_len]u8);
    defer allocator.destroy(blob);
    blob_payload.pack(segment.items, blob) catch {
        std.debug.print("error: the segment of {d} bytes does not fit one blob\n", .{segment.items.len});
        std.process.exit(1);
    };
    try input.append(allocator, 0);
    try input.appendSlice(allocator, &hash);
    try input.appendSlice(allocator, &commitment);
    try input.appendSlice(allocator, &proof);
    try input.appendSlice(allocator, blob);
}

/// 48 bytes derived from `seed`, in place of KZG material.
fn standIn(out: *[48]u8, label: u8, seed: []const u8) void {
    var bytes: [64]u8 = undefined;
    for ([_]u8{ 0, 1 }, 0..) |half, i| {
        var hasher = Sha256.init(.{});
        hasher.update(&.{ label, half });
        hasher.update(seed);
        hasher.final(bytes[i * 32 ..][0..32]);
    }
    out.* = bytes[0..48].*;
}
