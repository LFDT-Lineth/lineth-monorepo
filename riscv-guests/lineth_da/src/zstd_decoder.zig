//! Decompression of the data-availability zstd frame over upstream libzstd. A frame has one form:
//! its header declares the content size and the ID of the dictionary, and a window of at most
//! `max_window_size` bytes; the frame has no content checksum, is complete and ends the input. The
//! content decodes in one pass into a buffer of exactly the declared size. The decoder allocates
//! through the Zig allocator it is given (libzstd's `ZSTD_customMem`) when it is created, and
//! decompression runs on that memory.

const std = @import("std");

const c = @import("zstd_c");
const zstd_memory = @import("zstd_memory");

const Allocator = std.mem.Allocator;

pub const InitError = error{
    OutOfMemory,
    /// The dictionary is not a well-formed zstd dictionary, or declares no ID, which no frame can
    /// name.
    InvalidDictionary,
};

/// Why a frame is rejected.
pub const FrameError = error{
    /// The input ends before the frame does.
    TruncatedFrame,
    /// The input continues past the end of the frame.
    TrailingBytes,
    /// The input is not a zstd frame, or the frame's content is corrupt.
    MalformedFrame,
    /// The header declares no content size.
    MissingContentSize,
    /// The header declares a content size other than the length of the output buffer.
    ContentSizeMismatch,
    /// The header declares no dictionary ID.
    MissingDictionaryId,
    /// The header declares a dictionary ID other than the dictionary's own.
    DictionaryMismatch,
    /// The frame carries a content checksum.
    ChecksumPresent,
    /// The header declares a window beyond `max_window_size`.
    WindowTooLarge,
};

pub const Error = InitError || FrameError;

/// The largest window a frame may declare: the limit of libzstd's streaming decoder.
pub const max_window_size: u64 = (1 << 27) + 1;

/// `ZSTD_CONTENTSIZE_UNKNOWN`, the content size of a frame that declares none: the largest 64-bit
/// value, which the header spells as the wrap-around `0ULL - 1`.
const content_size_unknown = std.math.maxInt(c_ulonglong);

/// The linked libzstd release as `major * 10000 + minor * 100 + patch`.
pub fn version() u32 {
    return c.ZSTD_versionNumber();
}

/// Decompresses frames against one dictionary. A decoder is reusable across frames.
pub const Decoder = struct {
    state: *State,

    const State = struct {
        memory: zstd_memory.Memory,
        dctx: *c.ZSTD_DCtx,
        ddict: *c.ZSTD_DDict,
        dictionary_id: u32,
    };

    /// The decoder borrows `dictionary` for its whole lifetime.
    pub fn init(allocator: Allocator, dictionary: []const u8) InitError!Decoder {
        const state = try allocator.create(State);
        errdefer allocator.destroy(state);
        state.memory = .{ .allocator = allocator };
        const hooks = state.memory.hooks();

        state.dctx = c.ZSTD_createDCtx_advanced(hooks) orelse return error.OutOfMemory;
        errdefer _ = c.ZSTD_freeDCtx(state.dctx);
        state.ddict = c.ZSTD_createDDict_advanced(dictionary.ptr, dictionary.len, c.ZSTD_dlm_byRef, c.ZSTD_dct_fullDict, hooks) orelse
            return if (state.memory.exhausted) error.OutOfMemory else error.InvalidDictionary;
        errdefer _ = c.ZSTD_freeDDict(state.ddict);
        state.dictionary_id = c.ZSTD_getDictID_fromDDict(state.ddict);
        if (state.dictionary_id == 0) return error.InvalidDictionary;
        return .{ .state = state };
    }

    pub fn deinit(self: Decoder) void {
        const allocator = self.state.memory.allocator;
        _ = c.ZSTD_freeDCtx(self.state.dctx);
        _ = c.ZSTD_freeDDict(self.state.ddict);
        allocator.destroy(self.state);
    }

    /// Decompresses `frame`, which must be exactly one complete frame whose header declares
    /// `out.len` as its content size, into `out`.
    pub fn decompress(self: Decoder, frame: []const u8, out: []u8) FrameError!void {
        var header: c.ZSTD_frameHeader = undefined;
        const needed = c.ZSTD_getFrameHeader(&header, frame.ptr, frame.len);
        if (c.ZSTD_isError(needed) != 0) return error.MalformedFrame;
        if (needed != 0) return error.TruncatedFrame;
        if (header.frameType != c.ZSTD_frame) return error.MalformedFrame;
        if (header.frameContentSize == content_size_unknown) return error.MissingContentSize;
        if (header.frameContentSize != out.len) return error.ContentSizeMismatch;
        if (header.dictID == 0) return error.MissingDictionaryId;
        if (header.dictID != self.state.dictionary_id) return error.DictionaryMismatch;
        if (header.checksumFlag != 0) return error.ChecksumPresent;
        if (header.windowSize > max_window_size) return error.WindowTooLarge;

        const frame_len = c.ZSTD_findFrameCompressedSize(frame.ptr, frame.len);
        if (c.ZSTD_isError(frame_len) != 0) return switch (c.ZSTD_getErrorCode(frame_len)) {
            c.ZSTD_error_srcSize_wrong => error.TruncatedFrame,
            else => error.MalformedFrame,
        };
        if (frame_len < frame.len) return error.TrailingBytes;

        // An empty `out` may carry a dangling address; libzstd is handed a valid one for it.
        var placeholder: u8 = undefined;
        const written = c.ZSTD_decompress_usingDDict(
            self.state.dctx,
            if (out.len == 0) &placeholder else out.ptr,
            out.len,
            frame.ptr,
            frame.len,
            self.state.ddict,
        );
        if (c.ZSTD_isError(written) != 0) return error.MalformedFrame;
        std.debug.assert(written == out.len);
    }
};
