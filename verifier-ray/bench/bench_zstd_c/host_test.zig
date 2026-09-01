//! Host-side checks for the vendored zstd decoder, run against real libc with
//! `zig test host_test.zig <the vendor/zstd .c files>` (see the command in
//! run.go's header, or just use the sweep script which builds the guest).
//!
//! These exist because a decode failure in the guest is expensive to diagnose:
//! a ten-minute zkc run reports "decoded 0 bytes" with no further detail. Both
//! tests decode through the exact call main.zig makes, so an API-level mistake
//! surfaces in a second rather than after a full sweep.
//!
//! The prefix test in particular guards a failure that already bit once:
//! ZSTD_DCtx_refPrefix builds a DDict through dctx->customMem, which for a
//! STATIC DCtx is an allocator that always fails, so refPrefix returns
//! memory_allocation and the prefix is silently never attached. Decoding then
//! fails, but only in the guest, and only after ten minutes.
//!
//! Fixtures are generated (and gitignored): run
//!   python3 docs/blob-compression/dictionary_vs_window.py --cycles
//! to produce zstd_compressed.bin, prefix.bin and fixture_params.zig.

const std = @import("std");

const params = @import("fixture_params.zig");
const compressed: []const u8 = @embedFile("zstd_compressed.bin");
const prefix: []const u8 = @embedFile("prefix.bin");

extern fn ZSTD_initStaticDCtx(workspace: *anyopaque, workspaceSize: usize) ?*anyopaque;
extern fn ZSTD_isError(code: usize) c_uint;
extern fn ZSTD_DCtx_setParameter(dctx: *anyopaque, param: c_int, value: c_int) usize;
extern fn ZSTD_decompress_usingDict(
    dctx: *anyopaque,
    dst: *anyopaque,
    dstCapacity: usize,
    src: *const anyopaque,
    srcSize: usize,
    dict: ?*const anyopaque,
    dictSize: usize,
) usize;

const zstd_d_force_ignore_checksum: c_int = 1002;

fn fnv1a(data: []const u8) u64 {
    var h: u64 = 0xcbf29ce484222325;
    for (data) |b| {
        h ^= b;
        h = h *% 0x100000001b3;
    }
    return h;
}

/// Decodes exactly as main.zig does: static DCtx, checksum validation off, and
/// the context (prefix or dictionary, possibly empty) passed as a dict.
fn decodeLikeGuest(gpa: std.mem.Allocator, ctx: []const u8) !void {
    const ws = try gpa.alignedAlloc(u8, .@"16", 1 << 20);
    defer gpa.free(ws);
    const out = try gpa.alloc(u8, params.decompressed_len);
    defer gpa.free(out);

    const dctx = ZSTD_initStaticDCtx(ws.ptr, ws.len) orelse return error.StaticDCtxInitFailed;
    _ = ZSTD_DCtx_setParameter(dctx, zstd_d_force_ignore_checksum, 1);
    const n = ZSTD_decompress_usingDict(
        dctx,
        out.ptr,
        out.len,
        compressed.ptr,
        compressed.len,
        if (ctx.len != 0) ctx.ptr else null,
        ctx.len,
    );
    try std.testing.expectEqual(@as(c_uint, 0), ZSTD_isError(n));
    try std.testing.expectEqual(params.decompressed_len, n);
    try std.testing.expectEqual(params.expected_fnv1a, fnv1a(out));
}

test "fixture decodes to the corpus plaintext through the guest's call path" {
    try decodeLikeGuest(std.testing.allocator, prefix);
}

test "a static DCtx can attach a context" {
    // Skipped only when the fixture genuinely has no context (the `none` arm).
    // Covers both context kinds: a raw multi-MiB prefix and a 64 KiB trained
    // dictionary, which take different paths inside dct_auto.
    if (prefix.len == 0) return error.SkipZigTest;
    try decodeLikeGuest(std.testing.allocator, prefix);
}
