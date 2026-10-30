//! A recursive RLP encoder over an item tree.

const std = @import("std");

const Allocator = std.mem.Allocator;

pub const Item = union(enum) {
    string: []const u8,
    list: []const Item,
    /// Bytes that are already encoded and are written as they are.
    raw: []const u8,
};

pub fn encode(allocator: Allocator, item: Item) Allocator.Error![]u8 {
    var out: std.ArrayList(u8) = .empty;
    errdefer out.deinit(allocator);
    try encodeInto(allocator, &out, item);
    return out.toOwnedSlice(allocator);
}

fn encodeInto(allocator: Allocator, out: *std.ArrayList(u8), item: Item) Allocator.Error!void {
    switch (item) {
        .raw => |bytes| try out.appendSlice(allocator, bytes),
        .string => |bytes| {
            if (bytes.len == 1 and bytes[0] < 0x80) return out.append(allocator, bytes[0]);
            try appendLength(allocator, out, 0x80, bytes.len);
            try out.appendSlice(allocator, bytes);
        },
        .list => |items| {
            var payload: std.ArrayList(u8) = .empty;
            defer payload.deinit(allocator);
            for (items) |child| try encodeInto(allocator, &payload, child);
            try appendLength(allocator, out, 0xc0, payload.items.len);
            try out.appendSlice(allocator, payload.items);
        },
    }
}

fn appendLength(allocator: Allocator, out: *std.ArrayList(u8), base: u8, len: usize) Allocator.Error!void {
    if (len < 56) return out.append(allocator, base + @as(u8, @intCast(len)));
    var digits: [8]u8 = undefined;
    var count: usize = 0;
    var rest = len;
    while (rest > 0) : (rest >>= 8) {
        digits[count] = @truncate(rest);
        count += 1;
    }
    try out.append(allocator, base + 55 + @as(u8, @intCast(count)));
    while (count > 0) {
        count -= 1;
        try out.append(allocator, digits[count]);
    }
}

/// The string item of an integer: minimal big-endian bytes, empty for zero.
pub fn int(allocator: Allocator, value: u256) Allocator.Error!Item {
    var digits: std.ArrayList(u8) = .empty;
    var rest = value;
    while (rest > 0) : (rest >>= 8) try digits.append(allocator, @truncate(rest));
    std.mem.reverse(u8, digits.items);
    return .{ .string = try digits.toOwnedSlice(allocator) };
}

pub fn string(bytes: []const u8) Item {
    return .{ .string = bytes };
}

pub fn list(items: []const Item) Item {
    return .{ .list = items };
}

pub fn raw(bytes: []const u8) Item {
    return .{ .raw = bytes };
}
