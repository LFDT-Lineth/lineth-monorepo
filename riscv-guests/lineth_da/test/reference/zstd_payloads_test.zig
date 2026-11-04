//! The encoder and the decoder over the canonical payloads of consecutive blocks of the EF zkevm
//! fixtures.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const zstd_encoder = @import("zstd_encoder");
const zstd_frames = @import("zstd_frames");

const zstd_decoder = lineth_da.zstd_decoder;
const dictionary = lineth_da.dictionary_v2025_04_21;
const gpa = std.testing.allocator;

/// `len` bytes of the real payload stream; payloads of different variants start 6000 bytes apart.
fn real(len: usize, variant: usize) ![]const u8 {
    const offset = variant % 24 * 6_000;
    return (try ef_conflations.stream(std.testing.io, ef_fixtures.amsterdam_blocks, offset + len))[offset..][0..len];
}

test "payloads of real blocks round trip in the DA form" {
    const encoder = try zstd_encoder.Encoder.init(gpa, dictionary);
    defer encoder.deinit();
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();

    for ([_]usize{ 1, 33, 1000, 40_000, 131_073, 300_000 }) |len| {
        const payload = try real(len, len);
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
