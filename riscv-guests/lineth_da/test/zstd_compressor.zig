//! Host-side zstd compression over upstream libzstd, producing the frames the guest decompresses.

const std = @import("std");

const c = struct {
    const CCtx = opaque {};

    const InBuffer = extern struct { src: ?*const anyopaque, size: usize, pos: usize };
    const OutBuffer = extern struct { dst: ?*anyopaque, size: usize, pos: usize };

    const param_compression_level: c_int = 100;
    const param_window_log: c_int = 101;
    const param_content_size_flag: c_int = 200;
    const param_checksum_flag: c_int = 201;
    const param_dict_id_flag: c_int = 202;

    const end_flush: c_int = 1;
    const end_frame: c_int = 2;

    extern fn ZSTD_isError(code: usize) c_uint;
    extern fn ZSTD_compressBound(src_size: usize) usize;
    extern fn ZSTD_createCCtx() ?*CCtx;
    extern fn ZSTD_freeCCtx(cctx: ?*CCtx) usize;
    extern fn ZSTD_CCtx_setParameter(cctx: *CCtx, param: c_int, value: c_int) usize;
    extern fn ZSTD_CCtx_loadDictionary(cctx: *CCtx, dict: ?*const anyopaque, dict_size: usize) usize;
    extern fn ZSTD_compress2(
        cctx: *CCtx,
        dst: ?*anyopaque,
        dst_capacity: usize,
        src: ?*const anyopaque,
        src_size: usize,
    ) usize;
    extern fn ZSTD_compressStream2(cctx: *CCtx, output: *OutBuffer, input: *InBuffer, end_op: c_int) usize;
};

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
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_compression_level, options.level));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_content_size_flag, @intFromBool(options.content_size)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_checksum_flag, @intFromBool(options.checksum)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_dict_id_flag, @intFromBool(options.dictionary_id)));
    if (options.window_log != 0) try check(c.ZSTD_CCtx_setParameter(cctx, c.param_window_log, options.window_log));
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

fn compressInBlocks(cctx: *c.CCtx, out: []u8, payload: []const u8, block_len: usize) !usize {
    var output: c.OutBuffer = .{ .dst = out.ptr, .size = out.len, .pos = 0 };
    var start: usize = 0;
    while (start < payload.len) : (start += block_len) {
        const chunk = payload[start..@min(start + block_len, payload.len)];
        var input: c.InBuffer = .{ .src = chunk.ptr, .size = chunk.len, .pos = 0 };
        while (true) {
            const pending = c.ZSTD_compressStream2(cctx, &output, &input, c.end_flush);
            try check(pending);
            if (pending == 0) break;
        }
    }
    var rest: c.InBuffer = .{ .src = null, .size = 0, .pos = 0 };
    while (true) {
        const pending = c.ZSTD_compressStream2(cctx, &output, &rest, c.end_frame);
        try check(pending);
        if (pending == 0) break;
    }
    return output.pos;
}

fn check(code: usize) error{ZstdFailure}!void {
    if (c.ZSTD_isError(code) != 0) return error.ZstdFailure;
}
