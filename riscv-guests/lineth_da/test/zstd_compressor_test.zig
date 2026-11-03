const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_compressor = @import("zstd_compressor");
const zstd_frames = @import("zstd_frames");

const zstd = lineth_da.zstd;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// A payload of runs, repeated records and noise: its blocks are raw, run-length and compressed.
fn mixedPayload(len: usize) ![]u8 {
    const payload = try gpa.alloc(u8, len);
    var prng = std.Random.DefaultPrng.init(len);
    const random = prng.random();
    var at: usize = 0;
    while (at < len) {
        const piece = @min(len - at, 1 + random.uintLessThan(usize, 70_000));
        switch (random.uintLessThan(u8, 3)) {
            0 => @memset(payload[at..][0..piece], random.int(u8)),
            1 => random.bytes(payload[at..][0..piece]),
            else => for (payload[at..][0..piece], 0..) |*byte, i| {
                byte.* = @truncate(i % 251);
            },
        }
        at += piece;
    }
    return payload;
}

test "a frame declares its content size, a window within the bound and blocks within the window" {
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for ([_]usize{ 0, 1, 100, 5000, 131_072, 131_073, 600_000, 2_500_000 }) |len| {
        const payload = try mixedPayload(len);
        defer gpa.free(payload);
        const frame = try zstd_compressor.compress(gpa, payload, dictionary);
        defer gpa.free(frame);

        try zstd_frames.expectWithinSpec(gpa, frame, len, zstd.max_window_size);
        const declared = try zstd_frames.header(frame);
        try std.testing.expect(declared.window <= 1 << 21);
        try std.testing.expectEqual(std.mem.readInt(u32, dictionary[4..8], .little), declared.dictionary_id);

        const out = try gpa.alloc(u8, len);
        defer gpa.free(out);
        try std.testing.expectEqualSlices(u8, payload, try decoder.decompress(frame, out));
    }
}

test "a payload beyond one block is split into blocks" {
    const payload = try mixedPayload(1_000_000);
    defer gpa.free(payload);
    const frame = try zstd_compressor.compress(gpa, payload, dictionary);
    defer gpa.free(frame);

    const sizes = try zstd_frames.blockSizes(gpa, frame);
    defer gpa.free(sizes);
    try std.testing.expect(sizes.len > 1);
}

test "the same payload gives the same frame" {
    const payload = try mixedPayload(300_000);
    defer gpa.free(payload);
    const first = try zstd_compressor.compress(gpa, payload, dictionary);
    defer gpa.free(first);
    const second = try zstd_compressor.compress(gpa, payload, dictionary);
    defer gpa.free(second);
    try std.testing.expectEqualSlices(u8, first, second);
}

test "bytes that are not a zstd dictionary are rejected" {
    var raw_content: [4096]u8 = undefined;
    @memset(&raw_content, 'x');
    var corrupt_tables: [4096]u8 = undefined;
    @memset(&corrupt_tables, 0xff);
    @memcpy(corrupt_tables[0..8], dictionary[0..8]);
    const invalid = [_][]const u8{ &.{}, dictionary[0..7], dictionary[0..8], &corrupt_tables, &raw_content };
    for (invalid) |bytes| {
        try std.testing.expectError(error.InvalidDictionary, zstd_compressor.compress(gpa, "payload", bytes));
    }
}

test "a frame compressed against another dictionary ID is rejected by the decoder" {
    const other = try gpa.dupe(u8, dictionary);
    defer gpa.free(other);
    std.mem.writeInt(u32, other[4..8], 0x1234_5678, .little);

    const frame = try zstd_compressor.compress(gpa, "a conflation", other);
    defer gpa.free(frame);
    const decoder = try zstd.Decoder.init(gpa, dictionary);
    defer decoder.deinit();
    var out: [64]u8 = undefined;
    try std.testing.expectError(error.DictionaryMismatch, decoder.decompress(frame, &out));
}
