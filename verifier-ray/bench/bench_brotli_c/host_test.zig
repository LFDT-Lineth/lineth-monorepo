//! Host-side check that the vendored brotli decoder (with the raw dictionary
//! attached) decodes the fixture to the corpus plaintext. Ground truth is the
//! same corpus prefix bench_zstd's 262,144-byte fixture uses.
const std = @import("std");

const decompressed_len = 262_144;
const expected_crc32: u32 = 0xf8304eb8;
const compressed: []const u8 = @embedFile("brotli_compressed.bin");
const dict: []const u8 = @embedFile("dict.bin");

const BrotliDecoderState = opaque {};
const brotli_shared_dictionary_raw: c_int = 0;

extern fn BrotliDecoderCreateInstance(
    alloc_func: ?*const fn (opaque_ptr: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque,
    free_func: ?*const fn (opaque_ptr: ?*anyopaque, address: ?*anyopaque) callconv(.c) void,
    opaque_ptr: ?*anyopaque,
) ?*BrotliDecoderState;
extern fn BrotliDecoderAttachDictionary(
    state: *BrotliDecoderState,
    dict_type: c_int,
    data_size: usize,
    data: [*]const u8,
) c_int;
extern fn BrotliDecoderDecompressStream(
    state: *BrotliDecoderState,
    avail_in: *usize,
    next_in: *[*]const u8,
    avail_out: *usize,
    next_out: *[*]u8,
    total_out: ?*usize,
) c_int;
extern fn BrotliDecoderDestroyInstance(state: *BrotliDecoderState) void;

test "brotli -q 11 -D dict fixture decodes to the corpus plaintext" {
    const gpa = std.testing.allocator;
    const out = try gpa.alloc(u8, decompressed_len);
    defer gpa.free(out);

    // null/null uses brotli's built-in malloc/free on the host, unlike the
    // guest's bump allocator in main.zig.
    const state = BrotliDecoderCreateInstance(null, null, null).?;
    defer BrotliDecoderDestroyInstance(state);

    try std.testing.expect(BrotliDecoderAttachDictionary(
        state, brotli_shared_dictionary_raw, dict.len, dict.ptr,
    ) != 0);

    var avail_in: usize = compressed.len;
    var next_in: [*]const u8 = compressed.ptr;
    var avail_out: usize = out.len;
    var next_out: [*]u8 = out.ptr;
    var total_out: usize = 0;
    const result = BrotliDecoderDecompressStream(
        state, &avail_in, &next_in, &avail_out, &next_out, &total_out,
    );
    try std.testing.expectEqual(@as(c_int, 1), result); // BROTLI_DECODER_RESULT_SUCCESS
    try std.testing.expectEqual(decompressed_len, total_out);
    try std.testing.expectEqual(expected_crc32, std.hash.Crc32.hash(out));
}
