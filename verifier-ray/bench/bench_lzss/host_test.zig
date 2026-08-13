//! Host-side regression test for lzss.zig, runnable with `zig test host_test.zig`.
//!
//! The guest benchmark (main.zig) can only report cycle counts; it cannot tell
//! us the decoder is still CORRECT after an optimization. This test pins that
//! down on the host, against a checksum derived from the original corpus rather
//! than from this decoder's own output, so it is ground truth and not a
//! self-consistency check:
//!
//!   crc32(first 780,000 bytes of
//!         ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin)
//!     = 0xf9c30160
//!
//! That payload is the exact input the two embedded fixtures were compressed
//! from by the Go reference implementations (see main.zig for provenance), so a
//! decoder that reproduces this CRC has inverted the real encoder on real data.
//! The corpus itself is far too large to track in-repo; the checksum is the
//! part that has to travel with the code.
//!
//! Reproduce the constant with:
//!   python3 -c "import zlib; print(hex(zlib.crc32(open('PAYLOAD','rb').read(780000))))"

const std = @import("std");
const lzss = @import("lzss.zig");

const decompressed_len = 780_000;
const expected_crc32: u32 = 0xf9c30160;

const dict: []const u8 = @embedFile("dict.bin");
const a0_compressed: []const u8 = @embedFile("a0_compressed.bin");
const huffman_compressed: []const u8 = @embedFile("huffman_compressed.bin");
const huffman_table_bytes: []const u8 = @embedFile("huffman-table.bin");

const huffman_lengths: [512]u8 = blk: {
    var lengths: [512]u8 = undefined;
    @memcpy(&lengths, huffman_table_bytes);
    break :blk lengths;
};
const huffman_table = lzss.HuffmanTable.build(huffman_lengths);

fn checkDecode(
    comptime huffman: bool,
    comptime table: if (huffman) lzss.HuffmanTable else void,
    src: []const u8,
) !void {
    const out = try std.testing.allocator.alloc(u8, decompressed_len);
    defer std.testing.allocator.free(out);

    const n = lzss.decompress(huffman, table, src, out, dict, decompressed_len);
    try std.testing.expectEqual(decompressed_len, n);
    try std.testing.expectEqual(expected_crc32, std.hash.Crc32.hash(out));
}

test "A0 (deployed lzss v0.3.0) decodes the corpus fixture" {
    try checkDecode(false, {}, a0_compressed);
}

test "huffman-on-lengths decodes the corpus fixture" {
    try checkDecode(true, huffman_table, huffman_compressed);
}
