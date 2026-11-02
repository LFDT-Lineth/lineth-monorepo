//! Host-side zstd compression over upstream libzstd, producing the frames the guest decompresses.

const std = @import("std");

const c = @import("zstd_c");

pub const Options = struct {
    level: i32 = 3,
    /// Raw dictionary bytes the frame is compressed against.
    dictionary: ?[]const u8 = null,
    /// Writes the decompressed length into the frame header.
    content_size: bool = true,
    /// Appends the content checksum to the frame.
    checksum: bool = false,
    /// Writes the dictionary ID into the frame header.
    dictionary_id: bool = true,
    /// The base-2 logarithm of the window; zero leaves it to the level.
    window_log: u5 = 0,
    /// Ends a block after every this many payload bytes, in a frame that cannot declare its content
    /// size; zero leaves the blocks to the compressor.
    block_len: usize = 0,
};

/// Compresses `payload` into one zstd frame; the caller owns the result.
pub fn compress(allocator: std.mem.Allocator, payload: []const u8, options: Options) ![]u8 {
    const cctx = c.ZSTD_createCCtx() orelse return error.OutOfMemory;
    defer _ = c.ZSTD_freeCCtx(cctx);
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_compressionLevel, options.level));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_contentSizeFlag, @intFromBool(options.content_size)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_checksumFlag, @intFromBool(options.checksum)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_dictIDFlag, @intFromBool(options.dictionary_id)));
    if (options.window_log != 0) try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_windowLog, options.window_log));
    if (options.dictionary) |dictionary| try check(c.ZSTD_CCtx_loadDictionary(cctx, dictionary.ptr, dictionary.len));

    const block_overhead = 32;
    const blocks = if (options.block_len == 0) 0 else payload.len / options.block_len + 1;
    const out = try allocator.alloc(u8, c.ZSTD_compressBound(payload.len) + blocks * block_overhead);
    errdefer allocator.free(out);
    const len = if (options.block_len == 0)
        c.ZSTD_compress2(cctx, out.ptr, out.len, payload.ptr, payload.len)
    else
        try compressInBlocks(cctx, out, payload, options.block_len);
    try check(len);
    return allocator.realloc(out, len);
}

fn compressInBlocks(cctx: *c.ZSTD_CCtx, out: []u8, payload: []const u8, block_len: usize) !usize {
    var output: c.ZSTD_outBuffer = .{ .dst = out.ptr, .size = out.len, .pos = 0 };
    var start: usize = 0;
    while (start < payload.len) : (start += block_len) {
        const chunk = payload[start..@min(start + block_len, payload.len)];
        var input: c.ZSTD_inBuffer = .{ .src = chunk.ptr, .size = chunk.len, .pos = 0 };
        while (true) {
            const pending = c.ZSTD_compressStream2(cctx, &output, &input, c.ZSTD_e_flush);
            try check(pending);
            if (pending == 0) break;
        }
    }
    var rest: c.ZSTD_inBuffer = .{ .src = null, .size = 0, .pos = 0 };
    while (true) {
        const pending = c.ZSTD_compressStream2(cctx, &output, &rest, c.ZSTD_e_end);
        try check(pending);
        if (pending == 0) break;
    }
    return output.pos;
}

fn check(code: usize) error{ZstdFailure}!void {
    if (c.ZSTD_isError(code) != 0) return error.ZstdFailure;
}
