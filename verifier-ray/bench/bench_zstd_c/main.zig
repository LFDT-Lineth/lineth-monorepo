// Micro-benchmark: RISC-V cycle cost of decoding a zstd frame with the C
// reference decoder (facebook/zstd v1.5.7, decode-only sources vendored under
// vendor/zstd), compiled for the guest by `zig cc`.
//
// This exists to separate two things bench_zstd could not. There, Zig's
// std.compress.zstd measured 364 cycles/byte with 80% of the run inside
// memcpy, moving ~109 bytes per output byte -- its Reader/Writer plumbing
// shuttling data in ~8-byte pieces. That is an implementation property, not a
// property of the zstd format. The C reference decodes literals and matches
// straight into the output buffer, so this run measures the format much more
// closely.
//
// One-shot ZSTD_decompressDCtx into a flat destination, with a static DCtx, so
// there is no allocator and no streaming ring buffer -- the shape the DA
// circuit would use.
//
// Same fixture as bench_zstd: the first 262,144 bytes of
// ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin compressed with
// `zstd -19 -q` (69,767 bytes, ratio 3.7574), no dictionary.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start decode, 11 = end decode (value = decoded length)
//   12 = FNV-1a of the output, checked by run.go against the corpus plaintext

const std = @import("std");
const verifier_ray = @import("verifier_ray");
const accel = @import("lineth_accelerators");
const profiling = verifier_ray.profiling;

const decompressed_len = 262_144;

const compressed: []const u8 = @embedFile("zstd_compressed.bin");

// ZSTD_initStaticDCtx is part of the static-linking-only API. Declared here
// rather than via @cImport because zstd.h gates these behind
// ZSTD_STATIC_LINKING_ONLY and pulls in a large amount of unrelated surface.
extern fn ZSTD_estimateDCtxSize() usize;
extern fn ZSTD_initStaticDCtx(workspace: *anyopaque, workspaceSize: usize) ?*anyopaque;
extern fn ZSTD_decompressDCtx(
    dctx: *anyopaque,
    dst: *anyopaque,
    dstCapacity: usize,
    src: *const anyopaque,
    srcSize: usize,
) usize;
extern fn ZSTD_isError(code: usize) c_uint;
extern fn ZSTD_DCtx_setParameter(dctx: *anyopaque, param: c_int, value: c_int) usize;

/// ZSTD_d_forceIgnoreChecksum, which zstd.h defines as ZSTD_d_experimentalParam3.
/// Spelled numerically because the name is a macro, not an enumerator.
const zstd_d_force_ignore_checksum: c_int = 1002;

/// Generous fixed workspace; the guest asserts the estimate fits rather than
/// sizing it from a runtime call it cannot allocate against.
var dctx_workspace: [1 << 20]u8 align(16) = @splat(0);
var out: [decompressed_len]u8 = @splat(0);

/// Word-at-a-time implementations of the three libc functions zstd needs
/// (declared by vendor/shim/string.h). compiler-rt supplies byte loops for
/// this target, which dominated the bench_zstd profile.
export fn memcpy(noalias dest: [*]u8, noalias src: [*]const u8, len: usize) callconv(.c) [*]u8 {
    var i: usize = 0;
    while (i + 8 <= len) : (i += 8) {
        std.mem.writeInt(u64, dest[i..][0..8], std.mem.readInt(u64, src[i..][0..8], .little), .little);
    }
    while (i < len) : (i += 1) dest[i] = src[i];
    return dest;
}

export fn memset(dest: [*]u8, c: c_int, len: usize) callconv(.c) [*]u8 {
    const byte: u8 = @truncate(@as(c_uint, @bitCast(c)));
    const word: u64 = @as(u64, byte) * 0x0101010101010101;
    var i: usize = 0;
    while (i + 8 <= len) : (i += 8) std.mem.writeInt(u64, dest[i..][0..8], word, .little);
    while (i < len) : (i += 1) dest[i] = byte;
    return dest;
}

/// Unlike memcpy, the regions may overlap, so the copy direction depends on
/// their order; zstd calls this for match copies that can overlap.
export fn memmove(dest: [*]u8, src: [*]const u8, len: usize) callconv(.c) [*]u8 {
    if (@intFromPtr(dest) < @intFromPtr(src)) {
        var i: usize = 0;
        while (i + 8 <= len) : (i += 8) {
            std.mem.writeInt(u64, dest[i..][0..8], std.mem.readInt(u64, src[i..][0..8], .little), .little);
        }
        while (i < len) : (i += 1) dest[i] = src[i];
    } else {
        var i: usize = len;
        while (i >= 8) : (i -= 8) {
            std.mem.writeInt(u64, dest[i - 8 ..][0..8], std.mem.readInt(u64, src[i - 8 ..][0..8], .little), .little);
        }
        while (i > 0) : (i -= 1) dest[i - 1] = src[i - 1];
    }
    return dest;
}

/// Declared by vendor/shim/stdlib.h. The guest has no heap; a static DCtx must
/// never reach these, and failing loudly-but-safely (null) makes zstd return a
/// memory error if it ever does.
export fn malloc(size: usize) callconv(.c) ?*anyopaque {
    _ = size;
    return null;
}

export fn calloc(n: usize, size: usize) callconv(.c) ?*anyopaque {
    _ = n;
    _ = size;
    return null;
}

export fn free(ptr: ?*anyopaque) callconv(.c) void {
    _ = ptr;
}

fn fnv1a(data: []const u8) u64 {
    var h: u64 = 0xcbf29ce484222325;
    for (data) |b| {
        h ^= b;
        h = h *% 0x100000001b3;
    }
    return h;
}

pub export fn main() noreturn {
    profiling.markR5Value(0, 0);
    var i: u64 = 0;
    while (i < 1) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    profiling.markR5Value(1, 0);

    profiling.markR5Value(10, 0);
    // A failed init or decode reports length 0, which run.go rejects rather
    // than reporting as a fast decode.
    var decoded: u64 = 0;
    if (ZSTD_initStaticDCtx(&dctx_workspace, dctx_workspace.len)) |dctx| {
        // Skip xxh64 frame-checksum validation, matching bench_zstd's
        // verify_checksum = false so the two are comparable. It is also what
        // the DA circuit would do: the payload is already committed to by the
        // Poseidon2 hash counted as kappa_fix, so the frame checksum adds
        // integrity checking the system does not rely on. Measured at 28.3% of
        // decode when left on.
        _ = ZSTD_DCtx_setParameter(dctx, zstd_d_force_ignore_checksum, 1);
        const n = ZSTD_decompressDCtx(dctx, &out, out.len, compressed.ptr, compressed.len);
        if (ZSTD_isError(n) == 0) decoded = n;
    }
    profiling.markR5Value(11, decoded);
    profiling.markR5Value(12, fnv1a(&out));

    accel.zkvm_exit(0);
}
