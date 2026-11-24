//! Guest-side test of the word-wise memcpy / memmove / memset that build_common links into every
//! guest ELF (riscv-guests/build_common/mem.s): boundary sizes up to 300 × all 8×8 dst/src
//! alignments, memmove over overlapping (up to and past the 32-byte block stride) and disjoint
//! ranges in both directions, memset masking (including zero-low-byte non-zero ints), n == 0 with
//! unmapped pointers, return-value checks, and guard bytes around each destination. Exits non-zero
//! on the first mismatch (2: memcpy, 3: memset, 4: memmove); on success writes the number of
//! checked cases as a little-endian u64 (zero-padded to the 32-byte public output).
//!
//! Run: make TEST=mem_test.zig IN_BYTES=0x00 (in arithmetization/src/test)
//!
//! Buffers are only touched through volatile pointers so the reference side cannot be lowered to
//! the mem* calls under test, and mem* are invoked through volatile function pointers so LLVM
//! cannot fold away the `ret != dst` checks or `n == 0` calls.

const lineth_accel = @import("lineth_zkvm_accel");

extern fn memcpy(dst: [*]u8, src: [*]const u8, n: usize) [*]u8;
extern fn memmove(dst: [*]u8, src: [*]const u8, n: usize) [*]u8;
extern fn memset(dst: [*]u8, c: c_int, n: usize) [*]u8;

const MemcpyFn = *const fn ([*]u8, [*]const u8, usize) callconv(.c) [*]u8;
const MemmoveFn = *const fn ([*]u8, [*]const u8, usize) callconv(.c) [*]u8;
const MemsetFn = *const fn ([*]u8, c_int, usize) callconv(.c) [*]u8;

var memcpy_fn: MemcpyFn = &memcpy;
var memmove_fn: MemmoveFn = &memmove;
var memset_fn: MemsetFn = &memset;

fn callMemcpy(dst: [*]u8, src: [*]const u8, n: usize) [*]u8 {
    const f: *const volatile MemcpyFn = &memcpy_fn;
    return f.*(dst, src, n);
}

fn callMemmove(dst: [*]u8, src: [*]const u8, n: usize) [*]u8 {
    const f: *const volatile MemmoveFn = &memmove_fn;
    return f.*(dst, src, n);
}

fn callMemset(dst: [*]u8, c: c_int, n: usize) [*]u8 {
    const f: *const volatile MemsetFn = &memset_fn;
    return f.*(dst, c, n);
}

/// Every size 0..9 plus 8*k + {-1, 0, +1} across 0..2 blocks (hitting every head/block/word/tail
/// combination across dst alignments 0..7) and a sample up to 300 past the 256-byte pattern period.
const sizes = [_]usize{
    0,   1,   2,   3,   4,  5,  6,  7,   8,   9,
    15,  16,  17,  23,  24, 25, 31, 32,  33,  39,
    40,  41,  47,  48,  49, 55, 56, 57,  63,  64,
    65,  71,  72,  73,  95, 96, 97, 127, 128, 129,
    255, 256, 257, 300,
};
const max_size = 300;

/// Guard bytes on each side of a destination window.
const guard = 16;
/// memmove fixed deltas: self-copy (0), all 8 relative alignments + word boundary (±1..±9),
/// half-block/3-word offsets (±15..±17, ±24), and the 32-byte unrolled block boundary (±31..±33).
const fixed_deltas = [_]isize{
    -33, -32, -31, -24, -17, -16, -15, -9, -8, -7, -6, -5, -4, -3, -2, -1,
    0,   1,   2,   3,   4,   5,   6,   7,  8,  9,  15, 16, 17, 24, 31, 32,
    33,
};
/// Maximum |delta| for dynamic overlap/disjoint boundary tests (±(n-1), ±n, ±(n+1) up to n = 129).
const max_delta = 136;

const buf_len = guard + max_delta + 8 + max_size + max_delta + guard;
var src_buf: [buf_len]u8 align(8) = undefined;
var dst_buf: [buf_len]u8 align(8) = undefined;

/// Position-dependent byte pattern (odd multiplier: period 256, broken by `k >> 8`).
fn pat(k: usize) u8 {
    return @truncate(k *% 0x9D +% (k >> 8) +% 1);
}
/// The destination's own initial pattern, distinct from `pat` at every position.
fn dpat(k: usize) u8 {
    return pat(k) ^ 0x5A;
}

fn vptr(buf: *[buf_len]u8) [*]volatile u8 {
    return @ptrCast(buf);
}

fn fill(buf: *[buf_len]u8, comptime p: fn (usize) u8) void {
    const v = vptr(buf);
    for (0..buf_len) |k| v[k] = p(k);
}

fn fail(code: u32) noreturn {
    lineth_accel.zkvm_exit(code);
}

fn checkGuards(b: [*]volatile u8, di: usize, n: usize, comptime p: fn (usize) u8, code: u32) void {
    for (di - guard..di) |k| {
        if (b[k] != p(k)) fail(code);
    }
    for (di + n..di + n + guard) |k| {
        if (b[k] != p(k)) fail(code);
    }
}

fn testMemcpy() u64 {
    var cases: u64 = 0;
    fill(&src_buf, pat);
    fill(&dst_buf, dpat);
    const d = vptr(&dst_buf);
    for (sizes) |n| {
        for (0..8) |da| {
            for (0..8) |sa| {
                const di = guard + da;
                const si = guard + sa;
                const ret = callMemcpy(dst_buf[di..].ptr, src_buf[si..].ptr, n);
                if (ret != dst_buf[di..].ptr) fail(2);
                checkGuards(d, di, n, dpat, 2);
                for (0..n) |i| {
                    const k = di + i;
                    if (d[k] != pat(si + i)) fail(2);
                    d[k] = dpat(k);
                }
                cases += 1;
            }
        }
    }
    return cases;
}

fn testMemset() u64 {
    var cases: u64 = 0;
    fill(&dst_buf, dpat);
    const d = vptr(&dst_buf);
    // 0 takes the no-splat path; 0x1A5 and -1 check masking for non-zero low bytes;
    // 0x100 checks that masking happens before the zero-fill branch.
    const values = [_]c_int{ 0, 0xA5, 0x1A5, 0x100, -1 };
    for (sizes) |n| {
        for (0..8) |da| {
            for (values) |c| {
                const di = guard + da;
                const byte: u8 = @truncate(@as(c_uint, @bitCast(c)));
                const ret = callMemset(dst_buf[di..].ptr, c, n);
                if (ret != dst_buf[di..].ptr) fail(3);
                checkGuards(d, di, n, dpat, 3);
                for (di..di + n) |k| {
                    if (d[k] != byte) fail(3);
                    d[k] = dpat(k);
                }
                cases += 1;
            }
        }
    }
    return cases;
}

fn checkMemmoveCase(b: [*]volatile u8, si: usize, delta: isize, n: usize) void {
    const di: usize = @intCast(@as(isize, @intCast(si)) + delta);
    const ret = callMemmove(dst_buf[di..].ptr, dst_buf[si..].ptr, n);
    if (ret != dst_buf[di..].ptr) fail(4);
    checkGuards(b, di, n, pat, 4);
    for (0..n) |i| {
        const k = di + i;
        if (b[k] != pat(si + i)) fail(4);
        b[k] = pat(k);
    }
}

/// memmove within one buffer: dst = src + delta. Covers fixed deltas around the 8-byte word and
/// 32-byte block boundaries plus dynamic ±(n-1), ±n, ±(n+1) to test the exact `dst - src >= n`
/// forward/backward decision boundary up through multi-block sizes.
fn testMemmove() u64 {
    var cases: u64 = 0;
    fill(&dst_buf, pat);
    const b = vptr(&dst_buf);
    for (sizes) |n| {
        for (0..8) |sa| {
            const si = guard + max_delta + sa;
            for (fixed_deltas) |delta| {
                checkMemmoveCase(b, si, delta, n);
                cases += 1;
            }
            if (n > 33 and n + 1 <= max_delta) {
                const ni: isize = @intCast(n);
                const edge_deltas = [_]isize{ -(ni + 1), -ni, -(ni - 1), ni - 1, ni, ni + 1 };
                for (edge_deltas) |delta| {
                    checkMemmoveCase(b, si, delta, n);
                    cases += 1;
                }
            }
        }
    }
    return cases;
}

/// Guest RAM is strictly below 2^32; any load or store at >= 2^32 traps in zkc.
/// Proves n == 0 never dereferences dst or src (both aligned and unaligned).
fn testZeroLenUnmapped() u64 {
    const addrs = [_]usize{ 0x1_0000_0000, 0x1_0000_0001 };
    var cases: u64 = 0;
    for (addrs) |da| {
        const dst: [*]u8 = @ptrFromInt(da);
        if (callMemset(dst, 0xA5, 0) != dst) fail(3);
        if (callMemset(dst, 0, 0) != dst) fail(3);
        cases += 2;
        for (addrs) |sa| {
            const src: [*]const u8 = @ptrFromInt(sa);
            if (callMemcpy(dst, src, 0) != dst) fail(2);
            if (callMemmove(dst, src, 0) != dst) fail(4);
            cases += 2;
        }
    }
    return cases;
}

export fn main() noreturn {
    var cases: u64 = 0;
    cases += testMemcpy();
    cases += testMemset();
    cases += testMemmove();
    cases += testZeroLenUnmapped();
    // write_output takes the 32-byte public output: the case count, then zeros.
    var out: [32]u8 = undefined;
    const v: [*]volatile u8 = &out;
    for (0..32) |i| v[i] = if (i < 8) @truncate(cases >> @intCast(8 * i)) else 0;
    lineth_accel.write_output(&out, out.len);
    lineth_accel.zkvm_exit(0);
}
