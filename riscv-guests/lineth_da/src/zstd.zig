//! Dictionary-based decompression of single zstd frames over upstream libzstd's streaming decoder.
//! The decoder allocates only through the Zig allocator it is given (libzstd's `ZSTD_customMem`).

const std = @import("std");

const Allocator = std.mem.Allocator;

pub const Error = error{
    OutOfMemory,
    /// The dictionary is not a well-formed zstd dictionary.
    InvalidDictionary,
    /// The frame declares a dictionary ID other than the dictionary's own.
    DictionaryMismatch,
    /// The input ends before the frame does.
    TruncatedFrame,
    /// The input continues past the end of the frame.
    TrailingBytes,
    /// The input is not a zstd frame, the frame declares a window beyond `max_window_size`, the
    /// frame declares no content size and a window smaller than its content, or the frame's content
    /// is corrupt.
    MalformedFrame,
    /// The frame decompresses to more bytes than the output buffer holds.
    OutputTooLarge,
};

/// The largest window a frame may declare: the limit of libzstd's streaming decoder.
pub const max_window_size: u64 = (1 << 27) + 1;

const c = @import("zstd_c");

/// `ZSTD_CONTENTSIZE_UNKNOWN`, the content size of a frame that declares none: a macro whose
/// unsigned wrap-around the C translation does not evaluate.
const content_size_unknown = std.math.maxInt(c_ulonglong);

/// The linked libzstd release as `major * 10000 + minor * 100 + patch`.
pub fn version() u32 {
    return c.ZSTD_versionNumber();
}

/// Decompresses single zstd frames against one dictionary. A decoder is reusable across frames and
/// is unaffected by a rejected frame.
pub const Decoder = struct {
    state: *State,

    const State = struct {
        allocator: Allocator,
        dctx: *c.ZSTD_DCtx,
        ddict: *c.ZSTD_DDict,
        /// Set when the allocator refused a request from libzstd.
        exhausted: bool,
    };

    /// The decoder borrows `dictionary` for its whole lifetime.
    pub fn init(allocator: Allocator, dictionary: []const u8) Error!Decoder {
        const state = try allocator.create(State);
        errdefer allocator.destroy(state);
        state.exhausted = false;
        state.allocator = allocator;

        const mem: c.ZSTD_customMem = .{ .customAlloc = allocate, .customFree = release, .@"opaque" = state };
        state.dctx = c.ZSTD_createDCtx_advanced(mem) orelse return error.OutOfMemory;
        errdefer _ = c.ZSTD_freeDCtx(state.dctx);
        state.ddict = c.ZSTD_createDDict_advanced(dictionary.ptr, dictionary.len, c.ZSTD_dlm_byRef, c.ZSTD_dct_fullDict, mem) orelse
            return if (state.exhausted) error.OutOfMemory else error.InvalidDictionary;
        errdefer _ = c.ZSTD_freeDDict(state.ddict);
        std.debug.assert(c.ZSTD_isError(c.ZSTD_DCtx_refDDict(state.dctx, state.ddict)) == 0);
        std.debug.assert(c.ZSTD_isError(c.ZSTD_DCtx_setParameter(state.dctx, c.ZSTD_d_stableOutBuffer, 1)) == 0);
        return .{ .state = state };
    }

    pub fn deinit(self: Decoder) void {
        const allocator = self.state.allocator;
        _ = c.ZSTD_freeDCtx(self.state.dctx);
        _ = c.ZSTD_freeDDict(self.state.ddict);
        allocator.destroy(self.state);
    }

    /// Decompresses `frame`, which must be exactly one complete zstd frame, into the front of `out`
    /// and returns the decompressed bytes. `out.len` bounds the decompressed length. A frame that
    /// declares its content size decodes in one pass. A frame without content size decodes block by
    /// block through libzstd's streaming decoder with `out` as its history: each block holds at most
    /// `min(window, 128 KiB)` bytes and the window covers the content.
    pub fn decompress(self: Decoder, frame: []const u8, out: []u8) Error![]u8 {
        const frame_len = c.ZSTD_findFrameCompressedSize(frame.ptr, frame.len);
        if (c.ZSTD_isError(frame_len) != 0) return switch (c.ZSTD_getErrorCode(frame_len)) {
            c.ZSTD_error_srcSize_wrong => error.TruncatedFrame,
            else => error.MalformedFrame,
        };
        if (frame_len < frame.len) return error.TrailingBytes;
        var header: c.ZSTD_frameHeader = undefined;
        if (c.ZSTD_getFrameHeader(&header, frame.ptr, frame.len) != 0 or header.windowSize > max_window_size) return error.MalformedFrame;

        // An empty `out` may carry a dangling address; libzstd is handed a valid one for it.
        var placeholder: u8 = undefined;
        var output: c.ZSTD_outBuffer = .{ .dst = if (out.len == 0) &placeholder else out.ptr, .size = out.len, .pos = 0 };
        var input: c.ZSTD_inBuffer = .{ .src = frame.ptr, .size = frame.len, .pos = 0 };
        const remaining = c.ZSTD_decompressStream(self.state.dctx, &output, &input);
        // A rejected frame leaves the stream mid-frame; every call starts a new one.
        _ = c.ZSTD_DCtx_reset(self.state.dctx, c.ZSTD_reset_session_only);
        if (c.ZSTD_isError(remaining) != 0) return switch (c.ZSTD_getErrorCode(remaining)) {
            c.ZSTD_error_dictionary_wrong => error.DictionaryMismatch,
            c.ZSTD_error_dstSize_tooSmall => error.OutputTooLarge,
            c.ZSTD_error_memory_allocation => error.OutOfMemory,
            else => error.MalformedFrame,
        };
        if (remaining != 0 or input.pos != frame.len) return error.MalformedFrame;
        if (header.frameContentSize == content_size_unknown and header.windowSize < output.pos) return error.MalformedFrame;
        return out[0..output.pos];
    }
};

// libzstd allocations carry a header holding the block size, so `release` can return the exact
// block to the allocator. The header keeps the payload 16-byte aligned.
const block_alignment: std.mem.Alignment = .@"16";
const header_len = block_alignment.toByteUnits();

fn allocate(opaque_state: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque {
    const state: *Decoder.State = @ptrCast(@alignCast(opaque_state));
    const total = std.math.add(usize, size, header_len) catch return refuse(state);
    const block = state.allocator.alignedAlloc(u8, block_alignment, total) catch return refuse(state);
    const header: *usize = @ptrCast(block.ptr);
    header.* = total;
    return block.ptr + header_len;
}

fn release(opaque_state: ?*anyopaque, address: ?*anyopaque) callconv(.c) void {
    const state: *Decoder.State = @ptrCast(@alignCast(opaque_state));
    const payload: [*]u8 = @ptrCast(address orelse return);
    const base: [*]align(block_alignment.toByteUnits()) u8 = @alignCast(payload - header_len);
    const header: *const usize = @ptrCast(base);
    state.allocator.free(base[0..header.*]);
}

fn refuse(state: *Decoder.State) ?*anyopaque {
    state.exhausted = true;
    return null;
}
