//! Zstd frames that break the rules of the DA frame form, built through libzstd's parameters, and
//! a frame's header as libzstd reads it.

const std = @import("std");

const c = @import("zstd_c");

const Allocator = std.mem.Allocator;

pub const Options = struct {
    /// Raw dictionary bytes the frame is compressed against.
    dictionary: []const u8,
    /// Writes the decompressed length into the frame header.
    content_size: bool = true,
    /// Appends the content checksum to the frame.
    checksum: bool = false,
    /// Writes the dictionary ID into the frame header.
    dictionary_id: bool = true,
};

/// Compresses `payload` into one zstd frame; the caller owns the result.
pub fn compress(allocator: Allocator, payload: []const u8, options: Options) ![]u8 {
    const cctx = c.ZSTD_createCCtx() orelse return error.OutOfMemory;
    defer _ = c.ZSTD_freeCCtx(cctx);
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_contentSizeFlag, @intFromBool(options.content_size)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_checksumFlag, @intFromBool(options.checksum)));
    try check(c.ZSTD_CCtx_setParameter(cctx, c.ZSTD_c_dictIDFlag, @intFromBool(options.dictionary_id)));
    try check(c.ZSTD_CCtx_loadDictionary(cctx, options.dictionary.ptr, options.dictionary.len));

    const out = try allocator.alloc(u8, c.ZSTD_compressBound(payload.len));
    errdefer allocator.free(out);
    const len = c.ZSTD_compress2(cctx, out.ptr, out.len, payload.ptr, payload.len);
    try check(len);
    return allocator.realloc(out, len);
}

fn check(code: usize) error{ZstdFailure}!void {
    if (c.ZSTD_isError(code) != 0) return error.ZstdFailure;
}

/// A copy of `dictionary` that announces itself under the ID `id`.
pub fn dictionaryWithId(allocator: Allocator, dictionary: []const u8, id: u32) ![]u8 {
    const copy = try allocator.dupe(u8, dictionary);
    std.mem.writeInt(u32, copy[4..8], id, .little);
    return copy;
}

/// What a frame header declares.
pub const Header = struct {
    /// Null when the frame declares no content size.
    content_size: ?u64,
    /// The window in bytes: the declared one, or the content size of a single-segment frame.
    window: u64,
    /// Zero when the frame declares none.
    dictionary_id: u32,
    checksum: bool,
};

/// The header of `frame`, read by libzstd.
pub fn header(frame: []const u8) error{MalformedFrame}!Header {
    var parsed: c.ZSTD_frameHeader = undefined;
    if (c.ZSTD_getFrameHeader(&parsed, frame.ptr, frame.len) != 0) return error.MalformedFrame;
    return .{
        .content_size = if (parsed.frameContentSize == std.math.maxInt(c_ulonglong)) null else parsed.frameContentSize,
        .window = parsed.windowSize,
        .dictionary_id = parsed.dictID,
        .checksum = parsed.checksumFlag != 0,
    };
}

/// Rewrites the window descriptor of a frame that declares one, the byte behind the frame header
/// descriptor: every frame whose content is larger than its window.
pub fn patchWindowDescriptor(frame: []u8, descriptor: u8) void {
    frame[5] = descriptor;
}
