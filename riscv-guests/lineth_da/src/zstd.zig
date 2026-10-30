//! Dictionary-based decompression of single zstd frames over upstream libzstd. The decoder
//! allocates only through the Zig allocator it is given (libzstd's `ZSTD_customMem`), and only
//! while it is created.

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
    /// The input is not a zstd frame, the frame declares a window beyond `max_window_size`, or the
    /// frame's content is corrupt.
    MalformedFrame,
    /// The frame decompresses to more bytes than the output buffer holds.
    OutputTooLarge,
};

/// The largest window a frame may declare: the limit of libzstd's streaming decoder, so every frame
/// the guest accepts also decodes in a streaming decoder with default settings.
pub const max_window_size: u64 = (1 << 27) + 1;

const c = struct {
    const DCtx = opaque {};
    const DDict = opaque {};

    const FrameHeader = extern struct {
        frame_content_size: c_ulonglong,
        window_size: c_ulonglong,
        block_size_max: c_uint,
        frame_type: c_int,
        header_size: c_uint,
        dict_id: c_uint,
        checksum_flag: c_uint,
        reserved1: c_uint,
        reserved2: c_uint,
    };

    const CustomMem = extern struct {
        alloc: *const fn (state: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque,
        free: *const fn (state: ?*anyopaque, address: ?*anyopaque) callconv(.c) void,
        state: ?*anyopaque,
    };

    const dlm_by_ref: c_int = 1;
    const dct_full_dict: c_int = 2;

    const error_dictionary_wrong: c_int = 32;
    const error_memory_allocation: c_int = 64;
    const error_dst_size_too_small: c_int = 70;
    const error_src_size_wrong: c_int = 72;

    extern fn ZSTD_versionNumber() c_uint;
    extern fn ZSTD_isError(code: usize) c_uint;
    extern fn ZSTD_getErrorCode(code: usize) c_int;
    extern fn ZSTD_findFrameCompressedSize(src: ?*const anyopaque, src_size: usize) usize;
    extern fn ZSTD_getFrameHeader(header: *FrameHeader, src: ?*const anyopaque, src_size: usize) usize;
    extern fn ZSTD_createDCtx_advanced(mem: CustomMem) ?*DCtx;
    extern fn ZSTD_freeDCtx(dctx: ?*DCtx) usize;
    extern fn ZSTD_createDDict_advanced(
        dict: ?*const anyopaque,
        dict_size: usize,
        load_method: c_int,
        content_type: c_int,
        mem: CustomMem,
    ) ?*DDict;
    extern fn ZSTD_freeDDict(ddict: ?*DDict) usize;
    extern fn ZSTD_decompress_usingDDict(
        dctx: *DCtx,
        dst: *anyopaque,
        dst_capacity: usize,
        src: ?*const anyopaque,
        src_size: usize,
        ddict: *const DDict,
    ) usize;
};

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
        dctx: *c.DCtx,
        ddict: *c.DDict,
        /// Set when the allocator refused a request from libzstd.
        exhausted: bool,
    };

    /// `dictionary` is referenced, not copied: it must outlive the decoder.
    pub fn init(allocator: Allocator, dictionary: []const u8) Error!Decoder {
        const state = try allocator.create(State);
        errdefer allocator.destroy(state);
        state.exhausted = false;
        state.allocator = allocator;

        const mem: c.CustomMem = .{ .alloc = allocate, .free = release, .state = state };
        state.dctx = c.ZSTD_createDCtx_advanced(mem) orelse return error.OutOfMemory;
        errdefer _ = c.ZSTD_freeDCtx(state.dctx);
        state.ddict = c.ZSTD_createDDict_advanced(dictionary.ptr, dictionary.len, c.dlm_by_ref, c.dct_full_dict, mem) orelse
            return if (state.exhausted) error.OutOfMemory else error.InvalidDictionary;
        return .{ .state = state };
    }

    pub fn deinit(self: Decoder) void {
        const allocator = self.state.allocator;
        _ = c.ZSTD_freeDDict(self.state.ddict);
        _ = c.ZSTD_freeDCtx(self.state.dctx);
        allocator.destroy(self.state);
    }

    /// Decompresses `frame`, which must be exactly one complete zstd frame, into the front of `out`
    /// and returns the decompressed bytes. `out.len` bounds the decompressed length.
    pub fn decompress(self: Decoder, frame: []const u8, out: []u8) Error![]u8 {
        const frame_len = c.ZSTD_findFrameCompressedSize(frame.ptr, frame.len);
        if (c.ZSTD_isError(frame_len) != 0) return switch (c.ZSTD_getErrorCode(frame_len)) {
            c.error_src_size_wrong => error.TruncatedFrame,
            else => error.MalformedFrame,
        };
        if (frame_len < frame.len) return error.TrailingBytes;
        var header: c.FrameHeader = undefined;
        if (c.ZSTD_getFrameHeader(&header, frame.ptr, frame.len) != 0 or header.window_size > max_window_size) return error.MalformedFrame;

        // An empty `out` may carry a dangling address; libzstd is handed a valid one for it.
        var placeholder: u8 = undefined;
        const dst: *anyopaque = if (out.len == 0) &placeholder else out.ptr;
        const len = c.ZSTD_decompress_usingDDict(self.state.dctx, dst, out.len, frame.ptr, frame.len, self.state.ddict);
        if (c.ZSTD_isError(len) != 0) return switch (c.ZSTD_getErrorCode(len)) {
            c.error_dictionary_wrong => error.DictionaryMismatch,
            c.error_dst_size_too_small => error.OutputTooLarge,
            c.error_memory_allocation => error.OutOfMemory,
            else => error.MalformedFrame,
        };
        return out[0..len];
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
