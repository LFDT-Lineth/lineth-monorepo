//! Compression of a conflation's canonical payload into the one zstd frame form the decoder
//! accepts, over the libzstd the decoder is built from: level 3 in one pass, so the header declares
//! the content size, with the dictionary ID in the header and no content checksum. libzstd's window
//! for level 3 is at most 2^21 bytes, under the decoder's `max_window_size`. Native targets build
//! the encoder; the guest build holds the decoder alone. Like the decoder, it allocates through the
//! Zig allocator it is given (libzstd's `ZSTD_customMem`).

const std = @import("std");

const c = @import("zstd_c");
const zstd_memory = @import("zstd_memory");

const Allocator = std.mem.Allocator;

pub const Error = error{
    OutOfMemory,
    /// The dictionary is not a well-formed zstd dictionary, or declares no ID, which a frame could
    /// not carry.
    InvalidDictionary,
    /// libzstd reported a failure other than an exhausted allocator.
    CompressionFailed,
};

/// The compression level of every frame.
pub const level = 3;

/// Compresses payloads against one dictionary. The dictionary is digested once, when the encoder is
/// created.
pub const Encoder = struct {
    state: *State,

    const State = struct {
        memory: zstd_memory.Memory,
        cctx: *c.ZSTD_CCtx,
        cdict: *c.ZSTD_CDict,
    };

    /// The encoder borrows `dictionary` for its whole lifetime.
    pub fn init(allocator: Allocator, dictionary: []const u8) Error!Encoder {
        const state = try allocator.create(State);
        errdefer allocator.destroy(state);
        state.memory = .{ .allocator = allocator };
        const hooks = state.memory.hooks();

        state.cdict = c.ZSTD_createCDict_advanced(dictionary.ptr, dictionary.len, c.ZSTD_dlm_byRef, c.ZSTD_dct_fullDict, c.ZSTD_getCParams(level, 0, dictionary.len), hooks) orelse
            return if (state.memory.exhausted) error.OutOfMemory else error.InvalidDictionary;
        errdefer _ = c.ZSTD_freeCDict(state.cdict);
        if (c.ZSTD_getDictID_fromCDict(state.cdict) == 0) return error.InvalidDictionary;
        state.cctx = c.ZSTD_createCCtx_advanced(hooks) orelse return error.OutOfMemory;
        errdefer _ = c.ZSTD_freeCCtx(state.cctx);
        try check(c.ZSTD_CCtx_setParameter(state.cctx, c.ZSTD_c_compressionLevel, level));
        try check(c.ZSTD_CCtx_setParameter(state.cctx, c.ZSTD_c_contentSizeFlag, 1));
        try check(c.ZSTD_CCtx_setParameter(state.cctx, c.ZSTD_c_checksumFlag, 0));
        try check(c.ZSTD_CCtx_setParameter(state.cctx, c.ZSTD_c_dictIDFlag, 1));
        try check(c.ZSTD_CCtx_refCDict(state.cctx, state.cdict));
        return .{ .state = state };
    }

    pub fn deinit(self: Encoder) void {
        const allocator = self.state.memory.allocator;
        _ = c.ZSTD_freeCCtx(self.state.cctx);
        _ = c.ZSTD_freeCDict(self.state.cdict);
        allocator.destroy(self.state);
    }

    /// Compresses `payload` into one frame, allocated from `allocator`, which the caller owns.
    pub fn compress(self: Encoder, allocator: Allocator, payload: []const u8) Error![]u8 {
        const out = try allocator.alloc(u8, c.ZSTD_compressBound(payload.len));
        errdefer allocator.free(out);
        const len = c.ZSTD_compress2(self.state.cctx, out.ptr, out.len, payload.ptr, payload.len);
        try check(len);
        return allocator.realloc(out, len);
    }
};

fn check(code: usize) Error!void {
    if (c.ZSTD_isError(code) == 0) return;
    return switch (c.ZSTD_getErrorCode(code)) {
        c.ZSTD_error_memory_allocation => error.OutOfMemory,
        else => error.CompressionFailed,
    };
}
