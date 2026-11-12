//! SSZ for the guest wire formats: the `ssz.zig` library's generic codec over plain structs, with
//! list types that decode in place, and the framing of a schema id before an SSZ body.
//!
//! The list types alias the decoded buffer: `ByteList` and `FixedList` keep a slice of it and
//! allocate nothing, a `VariableList` allocates once for its element structs. A decoded value is
//! owned by the allocator that decoded it, which is an arena or a bump allocator that never frees.
//!
//! Decoding returns `error.InvalidSsz` for every rejection by the library or by a list type, and
//! `error.MalformedFrame` for a frame without its schema id. Encoding returns
//! `error.BoundsViolation` for a list beyond its bound.

const std = @import("std");
const builtin = @import("builtin");
const lib = @import("ssz");

const Allocator = std.mem.Allocator;
const ArrayList = std.ArrayList;

comptime {
    if (builtin.cpu.arch.endian() != .little) @compileError("aliasing integer lists needs a little-endian target");
}

// ── Views ─────────────────────────────────────────────────────────────────────

/// A list of bytes of at most `N` elements.
pub fn ByteList(comptime N: usize) type {
    return struct {
        const Self = @This();

        bytes: []const u8,

        pub fn sszDecode(serialized: []const u8, out: *Self, _: ?Allocator) !void {
            out.* = .{ .bytes = serialized };
        }

        pub fn sszEncode(self: *const Self, l: *ArrayList(u8), allocator: Allocator) !void {
            try l.appendSlice(allocator, self.bytes);
        }

        pub fn isFixedSizeObject() bool {
            return false;
        }

        pub fn serializedSize(self: *const Self) !usize {
            if (self.bytes.len > N) return error.ListTooLong;
            return self.bytes.len;
        }

        pub fn maxInLength() !usize {
            return N;
        }

        pub fn minInLength() usize {
            return 0;
        }
    };
}

/// A list of at most `N` elements of `T`, a byte array or an integer, laid out back to back.
pub fn FixedList(comptime T: type, comptime N: usize) type {
    comptime {
        const ok = switch (@typeInfo(T)) {
            .array => |array| array.child == u8,
            .int => |int| int.bits % 8 == 0,
            else => false,
        };
        if (!ok) @compileError("FixedList elements are byte arrays or whole-byte integers");
    }
    return struct {
        const Self = @This();

        items: []align(1) const T,

        pub fn sszDecode(serialized: []const u8, out: *Self, _: ?Allocator) !void {
            if (serialized.len % @sizeOf(T) != 0) return error.PartialElement;
            out.* = .{ .items = std.mem.bytesAsSlice(T, serialized) };
        }

        pub fn sszEncode(self: *const Self, l: *ArrayList(u8), allocator: Allocator) !void {
            try l.appendSlice(allocator, std.mem.sliceAsBytes(self.items));
        }

        pub fn isFixedSizeObject() bool {
            return false;
        }

        pub fn serializedSize(self: *const Self) !usize {
            if (self.items.len > N) return error.ListTooLong;
            return self.items.len * @sizeOf(T);
        }

        pub fn maxInLength() !usize {
            return N * @sizeOf(T);
        }

        pub fn minInLength() usize {
            return 0;
        }
    };
}

/// A list of at most `N` variable-size elements of `T`: an offset table over its region, each
/// offset counted from the start of the region. The empty region is the empty list.
pub fn VariableList(comptime T: type, comptime N: usize) type {
    comptime if (lib.isFixedSizeObject(T) catch false) @compileError("VariableList elements are variable-size");
    return struct {
        const Self = @This();

        items: []const T,

        pub fn sszDecode(serialized: []const u8, out: *Self, allocator: ?Allocator) !void {
            const alloc = allocator orelse return error.AllocatorRequired;
            if (serialized.len == 0) {
                out.* = .{ .items = &.{} };
                return;
            }
            if (serialized.len < 4) return error.OffsetExceedsSize;
            const table_len: usize = std.mem.readInt(u32, serialized[0..4], .little);
            if (table_len == 0 or table_len % 4 != 0) return error.OffsetAlignment;
            if (table_len > serialized.len) return error.OffsetExceedsSize;
            const count = table_len / 4;
            if (count > N) return error.ListTooLong;

            const items = try alloc.alloc(T, count);
            errdefer alloc.free(items);
            var start = table_len;
            for (items, 0..) |*item, i| {
                const end: usize = if (i + 1 < count)
                    std.mem.readInt(u32, serialized[(i + 1) * 4 ..][0..4], .little)
                else
                    serialized.len;
                if (end > serialized.len) return error.OffsetExceedsSize;
                if (start > end) return error.OffsetOrdering;
                try lib.deserialize(T, serialized[start..end], item, alloc);
                start = end;
            }
            out.* = .{ .items = items };
        }

        pub fn sszEncode(self: *const Self, l: *ArrayList(u8), allocator: Allocator) !void {
            try lib.serialize([]const T, self.items, l, allocator);
        }

        pub fn isFixedSizeObject() bool {
            return false;
        }

        pub fn serializedSize(self: *const Self) !usize {
            if (self.items.len > N) return error.ListTooLong;
            return lib.serializedSize([]const T, self.items);
        }

        pub fn maxInLength() !usize {
            return N * 4 + N * try lib.maxInLength(T);
        }

        pub fn minInLength() usize {
            return 0;
        }
    };
}

/// A boolean that is one byte, 0 or 1. The wrapper struct gives the library the one-byte fixed
/// size it derives from a struct's fields.
pub const StrictBool = struct {
    const Self = @This();

    value: bool,

    pub fn sszDecode(serialized: []const u8, out: *Self, _: ?Allocator) !void {
        out.* = .{ .value = switch (serialized[0]) {
            0 => false,
            1 => true,
            else => return error.InvalidBool,
        } };
    }

    pub fn sszEncode(self: *const Self, l: *ArrayList(u8), allocator: Allocator) !void {
        try l.append(allocator, @intFromBool(self.value));
    }

    pub fn isFixedSizeObject() bool {
        return true;
    }

    pub fn maxInLength() usize {
        return 1;
    }

    pub fn minInLength() usize {
        return 1;
    }
};

// ── Codec ─────────────────────────────────────────────────────────────────────

pub const DecodeError = error{InvalidSsz} || Allocator.Error;
pub const FramedDecodeError = error{MalformedFrame} || DecodeError;
pub const EncodeError = error{BoundsViolation} || Allocator.Error;

const schema_id_size: usize = 2;

/// The `T` that `bytes` encodes. Anything but the canonical encoding is `error.InvalidSsz`.
pub fn decode(comptime T: type, alloc: Allocator, bytes: []const u8) DecodeError!T {
    var out: T = undefined;
    lib.deserialize(T, bytes, &out, alloc) catch |err| {
        const any: anyerror = err;
        return if (any == error.OutOfMemory) error.OutOfMemory else error.InvalidSsz;
    };
    return out;
}

/// The SSZ encoding of `value`, in a buffer of exactly its size.
pub fn encode(comptime T: type, alloc: Allocator, value: T) EncodeError![]u8 {
    return encodeAfter(T, alloc, 0, &.{}, value);
}

/// The `T` encoded behind the 2-byte big-endian `schema_id`. A frame shorter than the schema id or
/// under another one is `error.MalformedFrame`.
pub fn decodeFramed(comptime T: type, alloc: Allocator, data: []const u8, schema_id: u16) FramedDecodeError!T {
    if (data.len < schema_id_size) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..schema_id_size], .big) != schema_id) return error.MalformedFrame;
    return decode(T, alloc, data[schema_id_size..]);
}

/// `schema_id` as 2 big-endian bytes followed by the SSZ encoding of `value`.
pub fn encodeFramed(comptime T: type, alloc: Allocator, schema_id: u16, value: T) EncodeError![]u8 {
    return encodeFramedAfter(T, alloc, 0, schema_id, value);
}

/// `encodeFramed` in a buffer that starts with `head_len` bytes left for the caller to fill, so a
/// caller that puts a header before the frame needs no second buffer.
pub fn encodeFramedAfter(comptime T: type, alloc: Allocator, head_len: usize, schema_id: u16, value: T) EncodeError![]u8 {
    return encodeAfter(T, alloc, head_len, &std.mem.toBytes(std.mem.nativeToBig(u16, schema_id)), value);
}

/// `head_len` unwritten bytes, `prefix`, then the SSZ encoding of `value`. The buffer is sized once
/// from the library's `serializedSize`, which also rejects a list beyond its bound: the guest
/// allocator never frees, so a grown buffer would waste its old copies.
fn encodeAfter(comptime T: type, alloc: Allocator, head_len: usize, prefix: []const u8, value: T) EncodeError![]u8 {
    const size = lib.serializedSize(T, value) catch |err| {
        const any: anyerror = err;
        return if (any == error.ListTooLong) error.BoundsViolation else unreachable;
    };
    var out = try ArrayList(u8).initCapacity(alloc, head_len + prefix.len + size);
    errdefer out.deinit(alloc);
    out.items.len = head_len;
    out.appendSliceAssumeCapacity(prefix);
    lib.serialize(T, value, &out, alloc) catch |err| {
        const any: anyerror = err;
        return if (any == error.OutOfMemory) error.OutOfMemory else unreachable;
    };
    std.debug.assert(out.items.len == out.capacity);
    return out.items;
}
