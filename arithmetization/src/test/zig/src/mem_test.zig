//! Guest-side test of the word-wise memcpy / memmove / memset that build_common links into every
//! guest ELF (riscv-guests/build_common/mem.s): every size in `sizes` × all 8×8 dst/src
//! alignments, memmove over overlapping and disjoint ranges in both directions, and guard bytes
//! around each destination. Exits non-zero on the first mismatch (2: memcpy, 3: memset,
//! 4: memmove); on success writes the number of checked cases as a little-endian u64 (zero-padded to the
//! 32-byte public output).
//!
//! Run: make TEST=mem_test.zig (in arithmetization/src/test)
//!
//! Buffers are only touched through volatile pointers, so the reference side cannot be lowered to
//! the mem* calls under test.

const lineth_accel = @import("lineth_zkvm_accel");

extern fn memcpy(dst: [*]u8, src: [*]const u8, n: usize) [*]u8;
extern fn memmove(dst: [*]u8, src: [*]const u8, n: usize) [*]u8;
extern fn memset(dst: [*]u8, c: c_int, n: usize) [*]u8;

/// Every size up to 80 (all head/word/block/tail combinations several times over), then a sample
/// up to 300 around the 8- and 32-byte loop boundaries. The full 0..300 grid passes too but takes
/// ~35 min under zkc for no extra path coverage.
const sizes = blk: {
    const larger = [_]usize{ 95, 96, 97, 103, 127, 128, 129, 135, 159, 191, 200, 223, 255, 256, 257, 263, 289, 299, 300 };
    var s: [81 + larger.len]usize = undefined;
    for (0..81) |i| s[i] = i;
    for (larger, 0..) |n, i| s[81 + i] = n;
    break :blk s;
};
const max_size = 300;

/// Guard bytes on each side of a destination window.
const guard = 16;
/// memmove: dst = src + delta for delta in [-max_delta, max_delta].
const max_delta = 24;

const buf_len = guard + 8 + max_size + 8 + guard + 2 * max_delta;
var src_buf: [buf_len]u8 align(8) = undefined;
var dst_buf: [buf_len]u8 align(8) = undefined;

/// Position-dependent byte pattern (odd multiplier: period 256, broken by `k >> 8`).
fn pat(k: usize) u8 {
    return @truncate(k *% 0x9D +% (k >> 8) +% 1);
}
/// The destination's own initial pattern, distinct from `pat` at most positions.
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
                const ret = memcpy(dst_buf[di..].ptr, src_buf[si..].ptr, n);
                if (ret != dst_buf[di..].ptr) fail(2);
                // Check [di - guard, di + n + guard) and restore the copied window.
                for (di - guard..di + n + guard) |k| {
                    const want = if (k >= di and k < di + n) pat(si + (k - di)) else dpat(k);
                    if (d[k] != want) fail(2);
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
    // 0 takes the no-splat path; 0x1A5 checks that only the low byte of `c` is used.
    const values = [_]c_int{ 0, 0xA5, 0x1A5, -1 };
    for (sizes) |n| {
        for (0..8) |da| {
            for (values) |c| {
                const di = guard + da;
                const byte: u8 = @truncate(@as(c_uint, @bitCast(c)));
                const ret = memset(dst_buf[di..].ptr, c, n);
                if (ret != dst_buf[di..].ptr) fail(3);
                for (di - guard..di + n + guard) |k| {
                    const want = if (k >= di and k < di + n) byte else dpat(k);
                    if (d[k] != want) fail(3);
                    d[k] = dpat(k);
                }
                cases += 1;
            }
        }
    }
    return cases;
}

/// memmove within one buffer: dst = src + delta. Small |delta| overlaps (forward when delta < 0,
/// backward when delta > 0); |delta| >= n is disjoint. src alignment 0..7 × delta covers all 64
/// relative alignments.
fn testMemmove() u64 {
    var cases: u64 = 0;
    fill(&dst_buf, pat);
    const b = vptr(&dst_buf);
    for (sizes) |n| {
        for (0..8) |sa| {
            var delta: isize = -max_delta;
            while (delta <= max_delta) : (delta += 1) {
                const si = guard + max_delta + sa;
                const di: usize = @intCast(@as(isize, @intCast(si)) + delta);
                const ret = memmove(dst_buf[di..].ptr, dst_buf[si..].ptr, n);
                if (ret != dst_buf[di..].ptr) fail(4);
                for (di - guard..di + n + guard) |k| {
                    const want = if (k >= di and k < di + n) pat(si + (k - di)) else pat(k);
                    if (b[k] != want) fail(4);
                    b[k] = pat(k);
                }
                cases += 1;
            }
        }
    }
    return cases;
}

export fn main() noreturn {
    var cases: u64 = 0;
    cases += testMemcpy();
    cases += testMemset();
    cases += testMemmove();
    // write_output takes the 32-byte public output: the case count, then zeros.
    var out: [32]u8 = undefined;
    const v: [*]volatile u8 = &out;
    for (0..32) |i| v[i] = if (i < 8) @truncate(cases >> @intCast(8 * i)) else 0;
    lineth_accel.write_output(&out, out.len);
    lineth_accel.zkvm_exit(0);
}
