//! Generic SSZ decode/encode primitives shared by every riscv-guests SSZ codec: fixed-width
//! little-endian integer reads/writes and the generic `List[VariableSizeType, N]` offset-table
//! codec that a container's own variable-field region also follows. Container-specific field
//! layouts, bounds policies, and schema ids stay in each guest's own codec module — only the
//! byte-level machinery every one of those codecs builds on lives here.
//!
//! `decodeVariableList`/`encodeVariableList` are the byte-level inverse of each other: an offset
//! table (4 bytes per element, each an absolute offset from the start of the region) followed by
//! the concatenated element bytes, in order — the same convention SSZ uses for a container's own
//! variable-field region, applied here to a list of variable-size elements.
//!
//! `error.InvalidSsz` is the one name every primitive below returns for structurally malformed
//! input (a missing/short region, a zero or misaligned offset, an offset out of bounds, or a
//! non-monotonic/overlapping element ordering) — a caller catches one error across an entire
//! decode call, regardless of which layer (this module's list walking or a container's own
//! fixed-head checks) detected the problem.

const std = @import("std");

// ── Primitive reads/writes (little-endian, matching SSZ) ────────────────────

pub inline fn readU32(data: []const u8, off: usize) u32 {
    return std.mem.readInt(u32, data[off..][0..4], .little);
}

pub inline fn readU64(data: []const u8, off: usize) u64 {
    return std.mem.readInt(u64, data[off..][0..8], .little);
}

pub inline fn writeU32(out: []u8, off: usize, value: u32) void {
    std.mem.writeInt(u32, out[off..][0..4], value, .little);
}

pub inline fn writeU64(out: []u8, off: usize, value: u64) void {
    std.mem.writeInt(u64, out[off..][0..8], value, .little);
}

// ── Generic "List[VariableSizeType, N]" codec ────────────────────────────────
//
// SSZ encodes a list of variable-size elements exactly like a container's
// variable-field region: an offset table (4 bytes per element, each an
// absolute offset from the start of this region) followed by the
// concatenated element bytes, in order.

pub fn decodeVariableList(alloc: std.mem.Allocator, data: []const u8, max_len: usize) ![]const []const u8 {
    if (data.len == 0) return &.{};
    if (data.len < 4) return error.InvalidSsz;

    const first_off = readU32(data, 0);
    if (first_off == 0 or first_off % 4 != 0) return error.InvalidSsz;
    if (first_off > data.len) return error.InvalidSsz;
    const n = first_off / 4;
    if (n > max_len) return error.BoundsViolation;

    const result = try alloc.alloc([]const u8, n);
    for (0..n) |i| {
        const off_i = readU32(data, i * 4);
        const end_i: u32 = if (i + 1 < n) readU32(data, (i + 1) * 4) else blk: {
            if (data.len > std.math.maxInt(u32)) return error.InvalidSsz;
            break :blk @intCast(data.len);
        };
        if (off_i > data.len or end_i > data.len or off_i > end_i) return error.InvalidSsz;
        result[i] = data[off_i..end_i];
    }
    return result;
}

pub fn encodeVariableList(alloc: std.mem.Allocator, items: []const []const u8) ![]u8 {
    const n = items.len;
    var total: usize = n * 4;
    for (items) |item| total += item.len;
    // The offset table is u32; a region that a u32 offset cannot address is unencodable.
    if (total > std.math.maxInt(u32)) return error.InvalidSsz;

    const out = try alloc.alloc(u8, total);
    var offset: u32 = @intCast(n * 4);
    for (items, 0..) |item, i| {
        writeU32(out, i * 4, offset);
        offset += @intCast(item.len);
    }
    var pos: usize = n * 4;
    for (items) |item| {
        @memcpy(out[pos..][0..item.len], item);
        pos += item.len;
    }
    return out;
}
