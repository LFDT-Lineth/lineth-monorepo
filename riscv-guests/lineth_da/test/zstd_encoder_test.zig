const std = @import("std");

const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");
const zstd_frames = @import("zstd_frames");

const zstd_decoder = lineth_da.zstd_decoder;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// A payload of runs, repeated records and noise.
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

test "a frame is in the DA form and decodes to its payload" {
    const encoder = try zstd_encoder.Encoder.init(gpa, dictionary);
    defer encoder.deinit();
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for ([_]usize{ 0, 1, 100, 5000, 131_072, 131_073, 600_000, 2_500_000 }) |len| {
        const payload = try mixedPayload(len);
        defer gpa.free(payload);
        const frame = try encoder.compress(gpa, payload);
        defer gpa.free(frame);

        const header = try zstd_frames.header(frame);
        try std.testing.expectEqual(@as(?u64, len), header.content_size);
        try std.testing.expectEqual(std.mem.readInt(u32, dictionary[4..8], .little), header.dictionary_id);
        try std.testing.expect(!header.checksum);
        try std.testing.expect(header.window <= zstd_decoder.max_window_size);

        const out = try gpa.alloc(u8, len);
        defer gpa.free(out);
        try decoder.decompress(frame, out);
        try std.testing.expectEqualSlices(u8, payload, out);
    }
}

test "bytes that are not a usable zstd dictionary are rejected" {
    var raw_content: [4096]u8 = undefined;
    @memset(&raw_content, 'x');
    const without_id = try zstd_frames.dictionaryWithId(gpa, dictionary, 0);
    defer gpa.free(without_id);
    const invalid = [_][]const u8{ &raw_content, without_id };
    for (invalid) |bytes| {
        try std.testing.expectError(error.InvalidDictionary, zstd_encoder.Encoder.init(gpa, bytes));
    }
}
