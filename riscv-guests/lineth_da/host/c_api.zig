//! The C ABI of the host library, declared in `include/lineth_da.h`. Each call reports its failure
//! as a status and, into the caller's buffer, a NUL-terminated message.

const std = @import("std");

const compressor = @import("compressor.zig");
const stream = @import("stream.zig");

const allocator = std.heap.c_allocator;

pub const Status = enum(c_int) {
    ok = 0,
    invalid_argument,
    invalid_dictionary,
    invalid_block_rlp,
    seeded_blob_mismatch,
    out_of_memory,
    internal,
};

/// The version of the ABI that `include/lineth_da.h` declares.
pub const abi_version: u32 = 1;

pub const Compressor = opaque {};
pub const Stream = opaque {};

pub const Bytes = extern struct {
    ptr: ?[*]const u8,
    len: usize,
};

pub const Compressed = extern struct {
    frame: ?[*]u8,
    frame_len: usize,
};

pub const SealedBlobs = extern struct {
    blobs: ?[*]stream.SealedBlob,
    count: usize,
};

const Report = struct {
    buffer: ?[*]u8,
    capacity: usize,

    fn ok(self: Report) Status {
        return self.write(.ok, "");
    }

    fn fail(self: Report, status: Status, comptime format: []const u8, args: anytype) Status {
        var text: [256]u8 = undefined;
        return self.write(status, std.fmt.bufPrint(&text, format, args) catch "message too long");
    }

    fn write(self: Report, status: Status, message: []const u8) Status {
        const buffer = self.buffer orelse return status;
        if (self.capacity == 0) return status;
        const len = @min(message.len, self.capacity - 1);
        @memcpy(buffer[0..len], message[0..len]);
        buffer[len] = 0;
        return status;
    }
};

fn bytesOf(ptr: ?[*]const u8, len: usize) ?[]const u8 {
    return if (ptr) |p| p[0..len] else if (len == 0) &.{} else null;
}

fn compressorOf(handle: ?*const Compressor) ?*const compressor.Compressor {
    return @ptrCast(@alignCast(handle));
}

fn streamOf(handle: ?*Stream) ?*stream.Stream {
    return @ptrCast(@alignCast(handle));
}

pub export fn lineth_da_abi_version() u32 {
    return abi_version;
}

pub export fn lineth_da_compressor_new(
    dictionary: ?[*]const u8,
    dictionary_len: usize,
    chain_id: u64,
    out: ?*?*Compressor,
    err: ?[*]u8,
    err_cap: usize,
) Status {
    const report: Report = .{ .buffer = err, .capacity = err_cap };
    const handle = out orelse return report.fail(.invalid_argument, "out is NULL", .{});
    handle.* = null;
    const bytes = bytesOf(dictionary, dictionary_len) orelse return report.fail(.invalid_argument, "dictionary is NULL", .{});

    const state = allocator.create(compressor.Compressor) catch return report.fail(.out_of_memory, "out of memory", .{});
    state.* = compressor.Compressor.init(allocator, bytes, chain_id) catch |e| {
        allocator.destroy(state);
        return switch (e) {
            error.OutOfMemory => report.fail(.out_of_memory, "out of memory", .{}),
            error.InvalidDictionary => report.fail(.invalid_dictionary, "not a zstd dictionary that declares an ID", .{}),
            else => report.fail(.internal, "zstd failed to create the compressor", .{}),
        };
    };
    handle.* = @ptrCast(state);
    return report.ok();
}

pub export fn lineth_da_compressor_free(handle: ?*Compressor) void {
    const state: *compressor.Compressor = @ptrCast(@alignCast(handle orelse return));
    state.deinit();
    allocator.destroy(state);
}

pub export fn lineth_da_compress(
    handle: ?*const Compressor,
    block_rlps: ?[*]const Bytes,
    block_count: usize,
    out: ?*Compressed,
    err: ?[*]u8,
    err_cap: usize,
) Status {
    const report: Report = .{ .buffer = err, .capacity = err_cap };
    const result = out orelse return report.fail(.invalid_argument, "out is NULL", .{});
    result.* = std.mem.zeroes(Compressed);
    const state = compressorOf(handle) orelse return report.fail(.invalid_argument, "compressor is NULL", .{});
    const inputs = if (block_count == 0) &[_]Bytes{} else (block_rlps orelse return report.fail(.invalid_argument, "block_rlps is NULL", .{}))[0..block_count];

    const rlps = allocator.alloc([]const u8, inputs.len) catch return report.fail(.out_of_memory, "out of memory", .{});
    defer allocator.free(rlps);
    for (inputs, rlps, 0..) |input, *rlp, index| {
        rlp.* = bytesOf(input.ptr, input.len) orelse return report.fail(.invalid_argument, "block {d} is NULL", .{index});
    }

    var rejection: compressor.Rejection = undefined;
    const frame = state.compress(rlps, &rejection) catch |e| return switch (e) {
        error.NoBlocks => report.fail(.invalid_argument, "no blocks", .{}),
        error.InvalidBlockRlp => switch (rejection.reason) {
            error.MalformedBlock => report.fail(.invalid_block_rlp, "block {d} is malformed", .{rejection.block_index}),
            error.SenderRecoveryFailed => report.fail(.invalid_block_rlp, "a sender of block {d} cannot be recovered", .{rejection.block_index}),
        },
        error.OutOfMemory => report.fail(.out_of_memory, "out of memory", .{}),
        error.InvalidDictionary, error.CompressionFailed => report.fail(.internal, "zstd failed to compress", .{}),
    };
    result.* = .{ .frame = frame.ptr, .frame_len = frame.len };
    return report.ok();
}

pub export fn lineth_da_compressed_free(compressed: ?*Compressed) void {
    const result = compressed orelse return;
    if (result.frame) |frame| allocator.free(frame[0..result.frame_len]);
    result.* = std.mem.zeroes(Compressed);
}

pub export fn lineth_da_stream_open(
    position: ?*const stream.Position,
    out: ?*?*Stream,
    err: ?[*]u8,
    err_cap: usize,
) Status {
    const report: Report = .{ .buffer = err, .capacity = err_cap };
    const handle = out orelse return report.fail(.invalid_argument, "out is NULL", .{});
    handle.* = null;
    const start = position orelse return report.fail(.invalid_argument, "position is NULL", .{});

    const state = allocator.create(stream.Stream) catch return report.fail(.out_of_memory, "out of memory", .{});
    state.* = stream.Stream.open(allocator, start) catch |e| {
        allocator.destroy(state);
        return streamFailure(report, e);
    };
    handle.* = @ptrCast(state);
    return report.ok();
}

pub export fn lineth_da_stream_close(handle: ?*Stream) void {
    const state = streamOf(handle) orelse return;
    state.deinit();
    allocator.destroy(state);
}

pub export fn lineth_da_stream_append(
    handle: ?*Stream,
    frame: ?[*]const u8,
    frame_len: usize,
    conflation_end_block: u64,
    out: ?*SealedBlobs,
    err: ?[*]u8,
    err_cap: usize,
) Status {
    const report: Report = .{ .buffer = err, .capacity = err_cap };
    const result = out orelse return report.fail(.invalid_argument, "out is NULL", .{});
    result.* = .{ .blobs = null, .count = 0 };
    const state = streamOf(handle) orelse return report.fail(.invalid_argument, "stream is NULL", .{});
    const bytes = bytesOf(frame, frame_len) orelse return report.fail(.invalid_argument, "frame is NULL", .{});

    const sealed = state.append(bytes, conflation_end_block) catch |e| return streamFailure(report, e);
    return deliver(report, result, sealed);
}

pub export fn lineth_da_stream_flush(
    handle: ?*Stream,
    out: ?*SealedBlobs,
    err: ?[*]u8,
    err_cap: usize,
) Status {
    const report: Report = .{ .buffer = err, .capacity = err_cap };
    const result = out orelse return report.fail(.invalid_argument, "out is NULL", .{});
    result.* = .{ .blobs = null, .count = 0 };
    const state = streamOf(handle) orelse return report.fail(.invalid_argument, "stream is NULL", .{});

    const sealed = state.flush() catch |e| return streamFailure(report, e);
    return deliver(report, result, sealed);
}

pub export fn lineth_da_sealed_blobs_free(sealed: ?*SealedBlobs) void {
    const result = sealed orelse return;
    if (result.blobs) |blobs| stream.freeSealed(allocator, blobs[0..result.count]);
    result.* = .{ .blobs = null, .count = 0 };
}

fn deliver(report: Report, result: *SealedBlobs, sealed: []stream.SealedBlob) Status {
    if (sealed.len > 0) result.* = .{ .blobs = sealed.ptr, .count = sealed.len };
    return report.ok();
}

fn streamFailure(report: Report, err: stream.Error) Status {
    return switch (err) {
        error.OutOfMemory => report.fail(.out_of_memory, "out of memory", .{}),
        error.EmptyFrame => report.fail(.invalid_argument, "the frame is empty", .{}),
        error.FrameTooLong => report.fail(.invalid_argument, "the frame is too long for its length prefix", .{}),
        error.MissingSharedBlob => report.fail(.invalid_argument, "shared_blob is NULL while the carry-over tail is positive", .{}),
        error.NonCanonicalSharedBlob => report.fail(.invalid_argument, "the shared blob is not the canonical packing of a payload", .{}),
        error.TailTooLong => report.fail(.invalid_argument, "the carry-over tail is not shorter than the payload of the shared blob", .{}),
        error.SharedBlobHashMismatch => report.fail(.invalid_argument, "the shared blob does not fold from parent_blob_data_rolling_hash to data_rolling_hash", .{}),
        error.SeededBlobMismatch => report.fail(.seeded_blob_mismatch, "the appended bytes differ from the shared blob", .{}),
        error.FlushWhileReproducing => report.fail(.invalid_argument, "the stream has bytes of the shared blob left to reproduce", .{}),
        error.ContextUnavailable => report.fail(.internal, "the KZG trusted setup is unavailable", .{}),
        error.KzgFailed => report.fail(.internal, "the KZG prover failed", .{}),
    };
}
