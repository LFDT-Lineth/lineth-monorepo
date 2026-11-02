//! Generic SSZ decode/encode primitives shared by every riscv-guests SSZ codec: fixed-width
//! little-endian integer reads/writes, the cursor helpers over a container's fixed head, the
//! `List[FixedSizeType, N]` region decoders and the generic `List[VariableSizeType, N]` offset-table
//! codec that a container's own variable-field region also follows. Container-specific field
//! layouts, bounds policies, and schema ids stay in each codec module — only the byte-level
//! machinery every one of those codecs builds on lives here.
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

/// Reads a little-endian u32 from the in-bounds range `data[off..off + 4]`.
pub inline fn readU32(data: []const u8, off: usize) u32 {
    return std.mem.readInt(u32, data[off..][0..4], .little);
}

/// Reads a little-endian u64 from the in-bounds range `data[off..off + 8]`.
pub inline fn readU64(data: []const u8, off: usize) u64 {
    return std.mem.readInt(u64, data[off..][0..8], .little);
}

/// Writes a little-endian u32 to the in-bounds range `out[off..off + 4]`.
pub inline fn writeU32(out: []u8, off: usize, value: u32) void {
    std.mem.writeInt(u32, out[off..][0..4], value, .little);
}

/// Writes a little-endian u64 to the in-bounds range `out[off..off + 8]`.
pub inline fn writeU64(out: []u8, off: usize, value: u64) void {
    std.mem.writeInt(u64, out[off..][0..8], value, .little);
}

// ── Cursor reads/writes over a container's fixed head ────────────────────────

/// Write a 32-byte hash at the cursor and advance it.
pub inline fn putHash(out: []u8, pos: *usize, value: [32]u8) void {
    @memcpy(out[pos.*..][0..32], &value);
    pos.* += 32;
}

/// Write a little-endian u64 at the cursor and advance it.
pub inline fn putU64(out: []u8, pos: *usize, value: u64) void {
    writeU64(out, pos.*, value);
    pos.* += 8;
}

/// Write the offset of a variable-size field's tail at the cursor and advance it.
pub inline fn putOffset(out: []u8, pos: *usize, tail_start: usize) void {
    writeU32(out, pos.*, @intCast(tail_start));
    pos.* += 4;
}

/// Read a 32-byte hash at the cursor and advance it.
pub inline fn getHash(bytes: []const u8, pos: *usize) [32]u8 {
    defer pos.* += 32;
    return bytes[pos.*..][0..32].*;
}

/// Read a little-endian u64 at the cursor and advance it.
pub inline fn getU64(bytes: []const u8, pos: *usize) u64 {
    defer pos.* += 8;
    return readU64(bytes, pos.*);
}

/// Read the offset of a variable-size field's tail at the cursor and advance it.
pub inline fn getOffset(bytes: []const u8, pos: *usize) usize {
    defer pos.* += 4;
    return readU32(bytes, pos.*);
}

// ── "List[FixedSizeType, N]": the plain concatenation of the elements ─────────

/// Alias a region of fixed-size byte-array elements after checking its length against the element size and the bound.
pub fn decodeFixedList(comptime T: type, region: []const u8, max_items: usize) ![]const T {
    if (region.len % @sizeOf(T) != 0) return error.InvalidSsz;
    if (region.len / @sizeOf(T) > max_items) return error.BoundsViolation;
    return std.mem.bytesAsSlice(T, region);
}

/// Decode a region of little-endian u16 elements into a new slice after checking its length against the bound.
pub fn decodeU16List(alloc: std.mem.Allocator, region: []const u8, max_items: usize) ![]const u16 {
    if (region.len % 2 != 0) return error.InvalidSsz;
    if (region.len / 2 > max_items) return error.BoundsViolation;
    const out = try alloc.alloc(u16, region.len / 2);
    for (out, 0..) |*item, i| item.* = std.mem.readInt(u16, region[i * 2 ..][0..2], .little);
    return out;
}

/// Write the elements as little-endian u16 values at `out[0..items.len * 2]`.
pub fn writeU16List(out: []u8, items: []const u16) void {
    for (items, 0..) |item, i| std.mem.writeInt(u16, out[i * 2 ..][0..2], item, .little);
}

// ── Generic "List[VariableSizeType, N]" codec ────────────────────────────────
//
// SSZ encodes a list of variable-size elements exactly like a container's
// variable-field region: an offset table (4 bytes per element, each an
// absolute offset from the start of this region) followed by the
// concatenated element bytes, in order.

pub fn decodeVariableList(alloc: std.mem.Allocator, data: []const u8, max_items: usize) ![]const []const u8 {
    if (data.len == 0) return &.{};
    if (data.len < 4) return error.InvalidSsz;
    if (data.len > std.math.maxInt(u32)) return error.InvalidSsz;

    const first_off = readU32(data, 0);
    if (first_off == 0 or first_off % 4 != 0) return error.InvalidSsz;
    if (first_off > data.len) return error.InvalidSsz;
    const n = first_off / 4;
    if (n > max_items) return error.BoundsViolation;

    const result = try alloc.alloc([]const u8, n);
    errdefer alloc.free(result);
    for (0..n) |i| {
        const off_i = readU32(data, i * 4);
        const end_i: u32 = if (i + 1 < n) readU32(data, (i + 1) * 4) else @intCast(data.len);
        if (off_i > data.len or end_i > data.len or off_i > end_i) return error.InvalidSsz;
        result[i] = data[off_i..end_i];
    }
    return result;
}

pub fn encodeVariableList(alloc: std.mem.Allocator, items: []const []const u8) ![]u8 {
    const n = items.len;
    const table_size = std.math.mul(usize, n, 4) catch return error.InvalidSsz;
    var total = table_size;
    for (items) |item| total = std.math.add(usize, total, item.len) catch return error.InvalidSsz;
    // The offset table is u32; a region that a u32 offset cannot address is unencodable.
    if (total > std.math.maxInt(u32)) return error.InvalidSsz;

    const out = try alloc.alloc(u8, total);
    var offset: u32 = @intCast(table_size);
    for (items, 0..) |item, i| {
        writeU32(out, i * 4, offset);
        offset += @intCast(item.len);
    }
    var pos = table_size;
    for (items) |item| {
        @memcpy(out[pos..][0..item.len], item);
        pos += item.len;
    }
    return out;
}
