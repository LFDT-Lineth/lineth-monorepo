//! Freestanding rv64im smoke guest: verifies a DA stream of one conflation carried by one chunk and
//! publishes the end dataRollingHash and the unconsumed tail count.
//!
//! Input: the parent dataRollingHash (32 bytes), the canonical payload's length (4 bytes, big
//! endian) and bytes, the chunk kind (1 byte: 0 blob, 1 calldata) and the chunk hash (32 bytes),
//! then a blob's 48-byte commitment, 48-byte proof and 131072-byte body, or a calldata chunk's bytes.
//! Output: the end dataRollingHash and the tail count as 8 big-endian bytes.
//!
//! Exit codes: 0 verified; 2 input malformed; 3 out of memory; 4 chunk malformed; 5 blob not
//! canonical; 6 chunk hash mismatch; 7 stream layout; 8 frame invalid; 9 payload mismatch.

const std = @import("std");
const builtin = @import("builtin");

const lineth_da = @import("lineth_da");
const accelerators = @import("lineth_accelerators");
const zkvm_provide = @import("zkvm_provide");

const blob_kzg = lineth_da.encoding.blob_kzg;
const decompress = lineth_da.decompress;
const zstd = lineth_da.zstd;

extern var _heap_start: u8;
extern var _heap_end: u8;

fn smokeMain() callconv(.c) noreturn {
    const heap = @as([*]u8, @ptrCast(&_heap_start))[0 .. @intFromPtr(&_heap_end) - @intFromPtr(&_heap_start)];
    var fba = std.heap.FixedBufferAllocator.init(heap);
    const allocator = fba.allocator();
    zkvm_provide.setAllocator(allocator);

    var input_ptr: [*]const u8 = undefined;
    var input_len: usize = undefined;
    accelerators.read_input(&input_ptr, &input_len);
    var rest: []const u8 = input_ptr[0..input_len];

    const parent = (take(&rest, 32) orelse malformed()).*;
    const canonical_len = std.mem.readInt(u32, take(&rest, 4) orelse malformed(), .big);
    const canonical = takeSlice(&rest, canonical_len) orelse malformed();
    const kind = (take(&rest, 1) orelse malformed())[0];
    const chunk_hash = (take(&rest, 32) orelse malformed()).*;
    const chunk: decompress.Chunk = switch (kind) {
        0 => .{
            .chunk_hash = chunk_hash,
            .is_calldata = false,
            .kzg_commitment = take(&rest, blob_kzg.commitment_len) orelse malformed(),
            .kzg_proof = take(&rest, blob_kzg.proof_len) orelse malformed(),
            .blob_bytes = take(&rest, blob_kzg.blob_len) orelse malformed(),
        },
        1 => .{ .chunk_hash = chunk_hash, .is_calldata = true, .calldata_bytes = rest },
        else => malformed(),
    };

    const decoder = zstd.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21) catch accelerators.zkvm_exit(3);
    const verified = decompress.verifyDaChunks(allocator, decoder, &.{chunk}, &.{canonical}, parent, 0, null) catch |err| {
        accelerators.zkvm_exit(exitCode(err));
    };

    var output: [40]u8 = undefined;
    @memcpy(output[0..32], &verified.end_data_rolling_hash);
    std.mem.writeInt(u64, output[32..40], verified.final_data_tail_discard_bytes, .big);
    accelerators.write_output(&output, output.len);
    accelerators.zkvm_exit(0);
}

fn take(rest: *[]const u8, comptime len: usize) ?*const [len]u8 {
    const bytes = takeSlice(rest, len) orelse return null;
    return bytes[0..len];
}

fn takeSlice(rest: *[]const u8, len: usize) ?[]const u8 {
    if (rest.len < len) return null;
    defer rest.* = rest.*[len..];
    return rest.*[0..len];
}

fn malformed() noreturn {
    accelerators.zkvm_exit(2);
}

fn exitCode(err: decompress.Error) u32 {
    return switch (err) {
        error.OutOfMemory => 3,
        error.NoChunks, error.MalformedChunk => 4,
        error.NonCanonicalBlob => 5,
        error.ChunkHashMismatch, error.BoundaryPreimageMismatch => 6,
        error.PartialBlobBeforeBlob,
        error.InvalidTailTake,
        error.MissingSegmentLength,
        error.InvalidSegmentLength,
        error.ChunkOwnsNoBytes,
        error.CalldataEndsMidSegment,
        => 7,
        error.InvalidFrame => 8,
        error.PayloadMismatch => 9,
    };
}

comptime {
    if (builtin.cpu.arch == .riscv64) {
        @export(&smokeMain, .{ .name = "main" });
        _ = zkvm_provide;
    }
}
