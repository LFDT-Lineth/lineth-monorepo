//! Compression of a conflation's canonical payload into the zstd frame the guest decompresses, over
//! the libzstd the decoder is built from. Native targets build it; the guest build holds the decoder
//! alone.

const std = @import("std");

const c = @import("zstd_c");

const Allocator = std.mem.Allocator;

pub const Error = error{
    OutOfMemory,
    /// The dictionary is not a well-formed zstd dictionary.
    InvalidDictionary,
    /// libzstd reported a failure other than an exhausted allocator.
    CompressionFailed,
};

/// The compression level of every frame.
pub const level = 3;

/// Compresses `payload` into one zstd frame against `dictionary`. The settings are fixed: level 3
/// in one pass, so the frame declares the content size; the dictionary ID in the header; no content
/// checksum. The window libzstd takes for level 3 is at most 2^21 bytes, under the 2^27 + 1 the
/// decoder accepts, and no block exceeds the smaller of the window and 128 KiB. The caller owns
/// the result.
pub fn compress(allocator: Allocator, payload: []const u8, dictionary: []const u8) Error![]u8 {
    var memory: Memory = .{};
    const custom: c.ZSTD_customMem = .{ .customAlloc = allocate, .customFree = release, .@"opaque" = &memory };

    const cdict = c.ZSTD_createCDict_advanced(dictionary.ptr, dictionary.len, c.ZSTD_dlm_byCopy, c.ZSTD_dct_fullDict, c.ZSTD_getCParams(level, 0, dictionary.len), custom) orelse
        return if (memory.refused) error.OutOfMemory else error.InvalidDictionary;
    defer _ = c.ZSTD_freeCDict(cdict);
    const cctx = c.ZSTD_createCCtx_advanced(custom) orelse return error.OutOfMemory;
    defer _ = c.ZSTD_freeCCtx(cctx);
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_compressionLevel, level));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_contentSizeFlag, 1));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_checksumFlag, 0));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_dictIDFlag, 1));
    try check(c.ZSTD_CCtx_refCDict(cctx, cdict));

    const out = try allocator.alloc(u8, c.ZSTD_compressBound(payload.len));
    errdefer allocator.free(out);
    const len = c.ZSTD_compress2(cctx, out.ptr, out.len, payload.ptr, payload.len);
    try check(len);
    return allocator.realloc(out, len);
}

fn check(code: usize) Error!void {
    if (c.ZSTD_isError(code) == 0) return;
    return switch (c.ZSTD_getErrorCode(code)) {
        c.ZSTD_error_memory_allocation => error.OutOfMemory,
        else => error.CompressionFailed,
    };
}

/// Records whether the allocator refused a request, which libzstd reports as a failed creation
/// like any other.
const Memory = struct { refused: bool = false };

fn allocate(opaque_memory: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque {
    const memory: *Memory = @ptrCast(@alignCast(opaque_memory));
    return std.c.malloc(size) orelse {
        memory.refused = true;
        return null;
    };
}

fn release(_: ?*anyopaque, address: ?*anyopaque) callconv(.c) void {
    std.c.free(address);
}
