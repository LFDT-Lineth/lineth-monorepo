//! Direct unit tests for guest_common's generic SSZ primitives — the byte-level machinery every
//! guest's own container-specific codec builds on. l2-execution's own codec tests exercise these
//! primitives indirectly (through its container decode/encode calls); the tests here isolate the
//! primitives themselves so a regression here fails at the source, not several layers up.

const std = @import("std");

const ssz = @import("guest_common").ssz;

test "readU32/writeU32 round-trip, little-endian" {
    var buf: [4]u8 = undefined;
    ssz.writeU32(&buf, 0, 0x11223344);
    try std.testing.expectEqual(@as(u32, 0x11223344), ssz.readU32(&buf, 0));
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x44, 0x33, 0x22, 0x11 }, &buf);
}

test "readU64/writeU64 round-trip, little-endian" {
    var buf: [8]u8 = undefined;
    ssz.writeU64(&buf, 0, 0x1122334455667788);
    try std.testing.expectEqual(@as(u64, 0x1122334455667788), ssz.readU64(&buf, 0));
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11 }, &buf);
}

test "readU32/readU64 honor a non-zero base offset" {
    const buf = [_]u8{ 0xEE, 0xEE, 0x01, 0x00, 0x00, 0x00, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE, 0xEE };
    try std.testing.expectEqual(@as(u32, 1), ssz.readU32(&buf, 2));
}

test "decodeVariableList: empty input decodes to an empty list" {
    const result = try ssz.decodeVariableList(std.testing.allocator, &.{}, 16);
    try std.testing.expectEqual(@as(usize, 0), result.len);
}

test "decodeVariableList/encodeVariableList round-trip: single element" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const items = [_][]const u8{"hello"};
    const encoded = try ssz.encodeVariableList(alloc, &items);
    const decoded = try ssz.decodeVariableList(alloc, encoded, 16);

    try std.testing.expectEqual(@as(usize, 1), decoded.len);
    try std.testing.expectEqualSlices(u8, "hello", decoded[0]);
}

test "decodeVariableList/encodeVariableList round-trip: multiple elements, including an empty one" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const items = [_][]const u8{ "abc", "", "de" };
    const encoded = try ssz.encodeVariableList(alloc, &items);
    const decoded = try ssz.decodeVariableList(alloc, encoded, 16);

    try std.testing.expectEqual(items.len, decoded.len);
    for (items, decoded) |want, got| try std.testing.expectEqualSlices(u8, want, got);
}

test "decodeVariableList: rejects a region shorter than one offset-table entry" {
    const data = [_]u8{ 0x01, 0x02, 0x03 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 16));
}

test "decodeVariableList: rejects a zero first offset" {
    const data = [_]u8{ 0x00, 0x00, 0x00, 0x00 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 16));
}

test "decodeVariableList: rejects a first offset not a multiple of 4" {
    const data = [_]u8{ 0x02, 0x00, 0x00, 0x00 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 16));
}

test "decodeVariableList: rejects a first offset past the end of the buffer" {
    const data = [_]u8{ 0xFF, 0xFF, 0x00, 0x00 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 16));
}

test "decodeVariableList: rejects an element count above max_items" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const items = [_][]const u8{ "a", "b", "c" };
    const encoded = try ssz.encodeVariableList(alloc, &items);
    try std.testing.expectError(error.BoundsViolation, ssz.decodeVariableList(alloc, encoded, 2));
}

test "decodeVariableList: rejects a non-monotonic (overlapping) offset table" {
    const data = [_]u8{ 0x08, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 16));
}

test "decodeVariableList: rejects a huge offset without attempting a giant allocation" {
    // A hostile first-offset value that would otherwise demand allocating room for ~2^30 element
    // slices; must be rejected by the bounds check before any allocation is attempted.
    const data = [_]u8{ 0xFC, 0xFF, 0xFF, 0xFF };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 1 << 16));
}

test "put and get cursor helpers lay fixed-head fields out back to back and read them back" {
    var head: [32 + 8 + 4]u8 = undefined;
    const hash: [32]u8 = @splat(0xab);

    var write_pos: usize = 0;
    ssz.putHash(&head, &write_pos, hash);
    ssz.putU64(&head, &write_pos, 0x1122334455667788);
    ssz.putOffset(&head, &write_pos, 0x0a0b0c0d);
    try std.testing.expectEqual(head.len, write_pos);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x0d, 0x0c, 0x0b, 0x0a }, head[32..]);

    var read_pos: usize = 0;
    try std.testing.expectEqualSlices(u8, &hash, &ssz.getHash(&head, &read_pos));
    try std.testing.expectEqual(@as(u64, 0x1122334455667788), ssz.getU64(&head, &read_pos));
    try std.testing.expectEqual(@as(usize, 0x0a0b0c0d), ssz.getOffset(&head, &read_pos));
    try std.testing.expectEqual(head.len, read_pos);
}

test "decodeFixedList: aliases whole elements, accepts the bound and rejects one element more" {
    var region: [3 * 20]u8 = undefined;
    for (&region, 0..) |*byte, i| byte.* = @intCast(i);

    const decoded = try ssz.decodeFixedList([20]u8, &region, 3);
    try std.testing.expectEqual(@as(usize, 3), decoded.len);
    try std.testing.expectEqualSlices(u8, region[20..40], &decoded[1]);
    try std.testing.expectEqual(@intFromPtr(&region), @intFromPtr(decoded.ptr));

    try std.testing.expectError(error.BoundsViolation, ssz.decodeFixedList([20]u8, &region, 2));
    try std.testing.expectEqual(@as(usize, 0), (try ssz.decodeFixedList([20]u8, &.{}, 0)).len);
}

test "decodeFixedList: rejects a region that is not whole elements" {
    const region = [_]u8{0} ** 31;
    try std.testing.expectError(error.InvalidSsz, ssz.decodeFixedList([32]u8, &region, 16));
}

test "writeU16List/decodeU16List round-trip, little-endian" {
    const items = [_]u16{ 1, 0x0203, 0xffff };
    var region: [items.len * 2]u8 = undefined;
    ssz.writeU16List(&region, &items);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x01, 0x00, 0x03, 0x02, 0xff, 0xff }, &region);

    const decoded = try ssz.decodeU16List(std.testing.allocator, &region, items.len);
    defer std.testing.allocator.free(decoded);
    try std.testing.expectEqualSlices(u16, &items, decoded);
}

test "decodeU16List: rejects an odd-length region and an element count above max_items" {
    const odd = [_]u8{ 1, 0, 2 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeU16List(std.testing.allocator, &odd, 16));

    const three = [_]u8{ 1, 0, 2, 0, 3, 0 };
    try std.testing.expectError(error.BoundsViolation, ssz.decodeU16List(std.testing.allocator, &three, 2));
}
