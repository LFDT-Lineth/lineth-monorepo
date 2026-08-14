//! Host-side check that std.compress.zstd decodes the fixture at all, and to
//! the right bytes. Ground truth is the corpus plaintext checksum, the same
//! constant bench_lzss/host_test.zig uses.
const std = @import("std");

const decompressed_len = 262_144;
const expected_crc32: u32 = 0xf8304eb8;
const compressed: []const u8 = @embedFile("zstd_compressed.bin");
const window_len = 1 << 20;

test "zstd -19 fixture decodes to the corpus plaintext" {
    const gpa = std.testing.allocator;
    const out = try gpa.alloc(u8, decompressed_len);
    defer gpa.free(out);

    // Direct mode, matching main.zig: the flat output buffer is the window.
    var input: std.Io.Reader = .fixed(compressed);
    var d: std.compress.zstd.Decompress = .init(&input, &.{}, .{
        .window_len = window_len,
        .verify_checksum = false,
    });
    var w: std.Io.Writer = .fixed(out);
    const n = try d.reader.streamRemaining(&w);
    try std.testing.expectEqual(decompressed_len, n);
    try std.testing.expectEqual(expected_crc32, std.hash.Crc32.hash(out));
}
