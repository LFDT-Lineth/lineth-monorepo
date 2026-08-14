// Micro-benchmark: RISC-V cycle cost of decoding a brotli frame with the C
// reference decoder (google/brotli v1.2.0, decode-only sources vendored under
// vendor/brotli), compiled for the guest by `zig cc`.
//
// brotli-11 dictionaried was the best-compressing arm measured across all four
// corpus windows (aggregate ratio 5.012, vs zstd's 4.838), so its decode cost
// is the remaining unknown standing between it and being the leading
// candidate. The zstd measurement showed a naive port (Zig std) can cost 39x
// more than a decoder that avoids incidental copying, so this uses the same
// approach that worked there: the actual C reference, decoding straight into
// a flat destination.
//
// Unlike zstd there is no static-context API: BrotliDecoderCreateInstance
// takes alloc/free callbacks and an opaque pointer. bumpAlloc below is a
// linear allocator over a fixed arena; brotli never frees mid-decode, so a
// real free() is unnecessary.
//
// Fixture: the first 262,144 bytes of
// ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin, compressed with
// `brotli -q 11 -D dict.bin` (66,006 bytes, ratio 3.9715), where dict.bin is
// the same 64 KiB production dictionary the LZSS arms use. Attached on decode
// via BrotliDecoderAttachDictionary(..., BROTLI_SHARED_DICTIONARY_RAW, ...),
// brotli's raw-LZ77-prefix mode -- the decoder-side counterpart of `-D`, so
// this is the same dictionary mechanism a real deployment would use, not an
// approximation of it.
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

const compressed: []const u8 = @embedFile("brotli_compressed.bin");
const dict: []const u8 = @embedFile("dict.bin");

const BrotliDecoderState = opaque {};
const BrotliDecoderResult = c_int;
const brotli_shared_dictionary_raw: c_int = 0; // BROTLI_SHARED_DICTIONARY_RAW

extern fn BrotliDecoderCreateInstance(
    alloc_func: *const fn (opaque_ptr: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque,
    free_func: *const fn (opaque_ptr: ?*anyopaque, address: ?*anyopaque) callconv(.c) void,
    opaque_ptr: ?*anyopaque,
) ?*BrotliDecoderState;
extern fn BrotliDecoderAttachDictionary(
    state: *BrotliDecoderState,
    dict_type: c_int,
    data_size: usize,
    data: [*]const u8,
) c_int;
extern fn BrotliDecoderDecompressStream(
    state: *BrotliDecoderState,
    avail_in: *usize,
    next_in: *[*]const u8,
    avail_out: *usize,
    next_out: *[*]u8,
    total_out: ?*usize,
) BrotliDecoderResult;
extern fn BrotliDecoderIsUsed(state: *const BrotliDecoderState) c_int;

/// Fixed arena the bump allocator hands out from; brotli's decode-time
/// allocations (Huffman tables, ring buffer, context map) are bounded by the
/// window size and compressed input, well under this for a 262 KiB fixture.
var arena: [1 << 20]u8 align(16) = @splat(0);
var arena_used: usize = 0;

fn bumpAlloc(opaque_ptr: ?*anyopaque, size: usize) callconv(.c) ?*anyopaque {
    _ = opaque_ptr;
    // 16-byte alignment matches the arena's; brotli does not request a
    // specific alignment, but its structs contain pointers and u64 fields.
    const aligned = (arena_used + 15) & ~@as(usize, 15);
    if (aligned + size > arena.len) return null;
    arena_used = aligned + size;
    return &arena[aligned];
}

fn bumpFree(opaque_ptr: ?*anyopaque, address: ?*anyopaque) callconv(.c) void {
    _ = opaque_ptr;
    _ = address;
    // No-op: the arena is reclaimed as a whole when the guest exits, and
    // brotli's decode path does not depend on individual frees succeeding.
}

/// platform.c's BrotliDefaultAllocFunc/FreeFunc call these directly as the
/// fallback used only when a caller passes null alloc/free callbacks; this
/// guest always supplies bumpAlloc/bumpFree, so these are unreachable dead
/// code kept only to satisfy the linker. malloc fails loudly rather than
/// silently succeeding, in case that assumption is ever wrong.
export fn malloc(size: usize) callconv(.c) ?*anyopaque {
    _ = size;
    return null;
}

export fn free(ptr: ?*anyopaque) callconv(.c) void {
    _ = ptr;
}

var out: [decompressed_len]u8 = @splat(0);

/// Word-at-a-time implementations of the libc functions brotli's decode path
/// calls (declared by vendor/shim/string.h). compiler-rt supplies byte loops
/// for this target, which the zstd measurement showed dominates decode cost
/// when left in place.
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

export fn memcmp(a: [*]const u8, b: [*]const u8, len: usize) callconv(.c) c_int {
    var i: usize = 0;
    while (i < len) : (i += 1) {
        if (a[i] != b[i]) return @as(c_int, a[i]) - @as(c_int, b[i]);
    }
    return 0;
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
    var decoded: u64 = 0;
    if (BrotliDecoderCreateInstance(bumpAlloc, bumpFree, null)) |state| {
        // BROTLI_TRUE is 1; a failed attach or decode leaves `decoded` at 0,
        // which run.go rejects rather than reporting as a fast decode.
        if (BrotliDecoderAttachDictionary(state, brotli_shared_dictionary_raw, dict.len, dict.ptr) != 0) {
            var avail_in: usize = compressed.len;
            var next_in: [*]const u8 = compressed.ptr;
            var avail_out: usize = out.len;
            var next_out: [*]u8 = &out;
            var total_out: usize = 0;
            const result = BrotliDecoderDecompressStream(
                state, &avail_in, &next_in, &avail_out, &next_out, &total_out,
            );
            // BROTLI_DECODER_RESULT_SUCCESS == 1.
            if (result == 1) decoded = total_out;
        }
    }
    profiling.markR5Value(11, decoded);
    profiling.markR5Value(12, fnv1a(&out));

    accel.zkvm_exit(0);
}
