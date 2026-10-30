//! Host-side zstd compression over upstream libzstd, producing the frames the guest decompresses.

const std = @import("std");

const c = struct {
    const CCtx = opaque {};

    const param_compression_level: c_int = 100;
    const param_content_size_flag: c_int = 200;
    const param_checksum_flag: c_int = 201;
    const param_dict_id_flag: c_int = 202;

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
};

/// Compresses `payload` into one zstd frame; the caller owns the result.
pub fn compress(allocator: std.mem.Allocator, payload: []const u8, options: Options) ![]u8 {
    const cctx = c.ZSTD_createCCtx() orelse return error.OutOfMemory;
    defer _ = c.ZSTD_freeCCtx(cctx);
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_compression_level, options.level));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_content_size_flag, @intFromBool(options.content_size)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_checksum_flag, @intFromBool(options.checksum)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.param_dict_id_flag, @intFromBool(options.dictionary_id)));
    if (options.dictionary) |dictionary| try check(c.ZSTD_CCtx_loadDictionary(cctx, dictionary.ptr, dictionary.len));

    const out = try allocator.alloc(u8, c.ZSTD_compressBound(payload.len));
    errdefer allocator.free(out);
    const len = c.ZSTD_compress2(cctx, out.ptr, out.len, payload.ptr, payload.len);
    try check(len);
    return allocator.realloc(out, len);
}

fn check(code: usize) error{ZstdFailure}!void {
    if (c.ZSTD_isError(code) != 0) return error.ZstdFailure;
}
