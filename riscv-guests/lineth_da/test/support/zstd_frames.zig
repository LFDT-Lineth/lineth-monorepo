//! Zstd frames in every form the guest decodes or must reject, built over libzstd's advanced
//! parameters, and the fields of a frame's header and blocks read from its bytes.

const std = @import("std");

const c = @import("zstd_c");

const Allocator = std.mem.Allocator;

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
pub fn compress(allocator: Allocator, payload: []const u8, options: Options) ![]u8 {
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

/// What a frame header declares.
pub const Header = struct {
    /// Null when the frame declares no content size.
    content_size: ?u64,
    /// The window in bytes: the declared one, or the content size of a single-segment frame.
    window: u64,
    /// Zero when the frame declares none.
    dictionary_id: u32,
    /// The bytes the header takes, magic number included.
    len: usize,
};

/// The header of the frame `frame`, read as RFC 8878 lays it out.
pub fn header(frame: []const u8) error{MalformedFrame}!Header {
    if (frame.len < 6 or !std.mem.eql(u8, frame[0..4], &.{ 0x28, 0xb5, 0x2f, 0xfd })) return error.MalformedFrame;
    const descriptor = frame[4];
    const content_size_flag = descriptor >> 6;
    const single_segment = descriptor & 0x20 != 0;
    const dictionary_id_len: usize = switch (descriptor & 3) {
        0 => 0,
        1 => 1,
        2 => 2,
        else => 4,
    };
    const content_size_len: usize = switch (content_size_flag) {
        0 => if (single_segment) 1 else 0,
        1 => 2,
        2 => 4,
        else => 8,
    };
    const window_len: usize = if (single_segment) 0 else 1;
    const len = 5 + window_len + dictionary_id_len + content_size_len;
    if (frame.len < len) return error.MalformedFrame;

    var at: usize = 5;
    var declared_window: u64 = 0;
    if (!single_segment) {
        const exponent: u6 = @intCast(frame[at] >> 3);
        const base = @as(u64, 1) << (10 + exponent);
        declared_window = base + base / 8 * (frame[at] & 7);
        at += 1;
    }
    const dictionary_id = std.mem.readVarInt(u32, frame[at..][0..dictionary_id_len], .little);
    at += dictionary_id_len;
    var content_size: ?u64 = null;
    if (content_size_flag != 0 or single_segment) {
        content_size = std.mem.readVarInt(u64, frame[at..][0..content_size_len], .little);
        if (content_size_flag == 1) content_size.? += 256;
    }
    return .{
        .content_size = content_size,
        .window = if (single_segment) content_size.? else declared_window,
        .dictionary_id = dictionary_id,
        .len = len,
    };
}

/// The `Block_Size` field of every block of the frame, in order: the size of a raw or run-length
/// block's decoded bytes and of a compressed block's encoding.
pub fn blockSizes(allocator: Allocator, frame: []const u8) ![]u32 {
    var sizes: std.ArrayList(u32) = .empty;
    errdefer sizes.deinit(allocator);
    var at = (try header(frame)).len;
    while (true) {
        if (frame.len < at + 3) return error.MalformedFrame;
        const word = std.mem.readInt(u24, frame[at..][0..3], .little);
        const size: u32 = word >> 3;
        const block_type = (word >> 1) & 3;
        try sizes.append(allocator, size);
        at += 3 + if (block_type == 1) @as(usize, 1) else size;
        if (word & 1 == 1) break;
    }
    return sizes.toOwnedSlice(allocator);
}

/// Expects what the frame has to satisfy for the guest: it declares `content_len` as its content
/// size, a window of at most `max_window` bytes, and no block above the smaller of the window and
/// 128 KiB.
pub fn expectWithinSpec(allocator: Allocator, frame: []const u8, content_len: usize, max_window: u64) !void {
    const declared = try header(frame);
    try std.testing.expectEqual(@as(?u64, content_len), declared.content_size);
    try std.testing.expect(declared.window <= max_window);
    const largest_block = @min(declared.window, 128 << 10);
    const sizes = try blockSizes(allocator, frame);
    defer allocator.free(sizes);
    for (sizes) |size| try std.testing.expect(size <= largest_block);
}
