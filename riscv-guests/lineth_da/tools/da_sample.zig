//! `da_sample` — builds the input of the `decompress` smoke guest: a payload as the canonical
//! payload of one conflation, published as one segment in one chunk.
//!
//! usage: da_sample <input-out> [blob|calldata [<payload-in>]]
//!
//! The payload is the file `payload-in`, or the canonical payload of a fixed sample conflation. The
//! segment is the payload compressed against the package dictionary behind a 4-byte length prefix.
//! A blob chunk packs it into a blob, anchored by the versioned hash of the blob's commitment and
//! carrying the commitment and the blob proof computed by c-kzg-4844.

const std = @import("std");

const conflation_sample = @import("conflation_sample");
const lineth_da = @import("lineth_da");
const support = @import("support");
const zstd_compressor = @import("zstd_compressor");

const blob_payload = lineth_da.encoding.blob_payload;
const da_stream = support.da_stream;

const usage = "usage: da_sample <input-out> [blob|calldata [<payload-in>]]\n";

pub fn main(init: std.process.Init) !void {
    const arena = init.arena.allocator();
    const args = try init.minimal.args.toSlice(arena);
    if (args.len < 2 or args.len > 4) {
        std.debug.print("error: expected <input-out> [blob|calldata [<payload-in>]]\n{s}", .{usage});
        std.process.exit(2);
    }
    const as_blob = if (args.len >= 3) blk: {
        if (std.mem.eql(u8, args[2], "blob")) break :blk true;
        if (std.mem.eql(u8, args[2], "calldata")) break :blk false;
        std.debug.print("error: chunk kind '{s}' is neither blob nor calldata\n{s}", .{ args[2], usage });
        std.process.exit(2);
    } else true;

    const payload = if (args.len == 4)
        std.Io.Dir.cwd().readFileAlloc(init.io, args[3], arena, .limited(1 << 30)) catch |err| {
            std.debug.print("error: cannot read '{s}': {s}\n", .{ args[3], @errorName(err) });
            std.process.exit(1);
        }
    else
        try conflation_sample.fixedConflationPayload(arena);

    var input: std.ArrayList(u8) = .empty;
    defer input.deinit(init.gpa);
    try build(init.gpa, &input, payload, as_blob);

    std.Io.Dir.cwd().writeFile(init.io, .{ .sub_path = args[1], .data = input.items }) catch |err| {
        std.debug.print("error: cannot write '{s}': {s}\n", .{ args[1], @errorName(err) });
        std.process.exit(1);
    };
}

fn build(allocator: std.mem.Allocator, input: *std.ArrayList(u8), payload: []const u8, as_blob: bool) !void {
    const frame = try zstd_compressor.compress(allocator, payload, lineth_da.dictionary_v2025_04_21);
    defer allocator.free(frame);
    const segment = try da_stream.lengthPrefixed(allocator, frame);
    defer allocator.free(segment);

    try input.appendNTimes(allocator, 0x11, 32); // parent dataRollingHash
    try input.appendSlice(allocator, &std.mem.toBytes(std.mem.nativeToBig(u32, @intCast(payload.len))));
    try input.appendSlice(allocator, payload);

    if (!as_blob) {
        var hash: [32]u8 = undefined;
        std.crypto.hash.sha3.Keccak256.hash(segment, &hash, .{});
        try input.append(allocator, 1);
        try input.appendSlice(allocator, &hash);
        try input.appendSlice(allocator, segment);
        return;
    }

    const blob = try allocator.create([blob_payload.blob_len]u8);
    defer allocator.destroy(blob);
    blob_payload.pack(segment, blob) catch {
        std.debug.print("error: the segment of {d} bytes does not fit one blob\n", .{segment.len});
        std.process.exit(1);
    };
    const opening = support.kzg.open(blob);
    try input.append(allocator, 0);
    try input.appendSlice(allocator, &da_stream.versionedHash(&opening.commitment));
    try input.appendSlice(allocator, &opening.commitment);
    try input.appendSlice(allocator, &opening.proof);
    try input.appendSlice(allocator, blob);
}
