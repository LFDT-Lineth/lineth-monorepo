//! Word-at-a-time memcpy, memmove and memset for the R5 guest.
//!
//! The R5 build is ReleaseSmall, for which Zig's compiler_rt exports byte loops
//! (`memcpySmall` and friends): one load, one store and the loop overhead per
//! byte. Under zkc every instruction is a cycle, and the verifier copies a lot
//! of fixed-size structs (24-byte extension elements, layout tables), so those
//! byte loops cost about a quarter of R5 verification. These versions move
//! eight bytes per iteration when both pointers allow it, four when only
//! 4-byte alignment holds, and fall back to bytes otherwise. compiler_rt's
//! symbols are weak, so main.zig exporting these overrides them.
//!
//! The accesses go through volatile pointers so that LLVM cannot recognise the
//! loops as a copy or fill idiom and lower them back into a call to the very
//! symbol being defined.

pub fn memcpy(noalias dest: ?[*]u8, noalias src: ?[*]const u8, len: usize) callconv(.c) ?[*]u8 {
    @setRuntimeSafety(false);
    copyForward(dest.?, src.?, len);
    return dest;
}

pub fn memmove(dest: ?[*]u8, src: ?[*]const u8, len: usize) callconv(.c) ?[*]u8 {
    @setRuntimeSafety(false);
    const d = @intFromPtr(dest.?);
    const s = @intFromPtr(src.?);
    // Forward is safe unless dest starts inside [src, src + len).
    if (d <= s or d >= s + len) {
        copyForward(dest.?, src.?, len);
    } else {
        copyBackward(dest.?, src.?, len);
    }
    return dest;
}

pub fn memset(dest: ?[*]u8, c: u8, len: usize) callconv(.c) ?[*]u8 {
    @setRuntimeSafety(false);
    const d: [*]volatile u8 = dest.?;
    var i: usize = 0;
    if (@intFromPtr(d) & 7 == 0) {
        const pattern = @as(u64, c) * 0x0101_0101_0101_0101;
        const words: [*]volatile u64 = @ptrCast(@alignCast(d));
        const n = len / 8;
        var k: usize = 0;
        while (k < n) : (k += 1) words[k] = pattern;
        i = n * 8;
    }
    while (i < len) : (i += 1) d[i] = c;
    return dest;
}

inline fn copyForward(dest: [*]u8, src: [*]const u8, len: usize) void {
    const d: [*]volatile u8 = dest;
    const s: [*]const volatile u8 = src;
    const both = @intFromPtr(d) | @intFromPtr(s);
    var i: usize = 0;
    if (both & 7 == 0) {
        const dw: [*]volatile u64 = @ptrCast(@alignCast(d));
        const sw: [*]const volatile u64 = @ptrCast(@alignCast(s));
        const n = len / 8;
        var k: usize = 0;
        while (k < n) : (k += 1) dw[k] = sw[k];
        i = n * 8;
    } else if (both & 3 == 0) {
        const dw: [*]volatile u32 = @ptrCast(@alignCast(d));
        const sw: [*]const volatile u32 = @ptrCast(@alignCast(s));
        const n = len / 4;
        var k: usize = 0;
        while (k < n) : (k += 1) dw[k] = sw[k];
        i = n * 4;
    }
    while (i < len) : (i += 1) d[i] = s[i];
}

inline fn copyBackward(dest: [*]u8, src: [*]const u8, len: usize) void {
    const d: [*]volatile u8 = dest;
    const s: [*]const volatile u8 = src;
    var i = len;
    // Same alignment of both ends: copy the unaligned tail bytes first, then
    // whole words down to the start.
    if ((@intFromPtr(d) | @intFromPtr(s)) & 7 == 0) {
        while (i % 8 != 0) {
            i -= 1;
            d[i] = s[i];
        }
        const dw: [*]volatile u64 = @ptrCast(@alignCast(d));
        const sw: [*]const volatile u64 = @ptrCast(@alignCast(s));
        var k = i / 8;
        while (k > 0) {
            k -= 1;
            dw[k] = sw[k];
        }
        return;
    }
    while (i > 0) {
        i -= 1;
        d[i] = s[i];
    }
}
