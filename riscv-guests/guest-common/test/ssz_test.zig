//! Direct unit tests for guest-common's generic SSZ primitives — the byte-level machinery every
//! guest's own container-specific codec builds on. l2-execution's own codec tests exercise these
//! primitives indirectly (through its container decode/encode calls); the tests here isolate the
//! primitives themselves so a regression here fails at the source, not several layers up.

const std = @import("std");

const ssz = @import("guest_common_ssz");

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

test "decodeVariableList: rejects an element count above max_len" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const items = [_][]const u8{ "a", "b", "c" };
    const encoded = try ssz.encodeVariableList(alloc, &items);
    try std.testing.expectError(error.BoundsViolation, ssz.decodeVariableList(alloc, encoded, 2));
}

test "decodeVariableList: rejects a non-monotonic (overlapping) offset table" {
    // Two elements: table[0]=8 (past table[1], i.e. element 0 would start after its own end). The
    // element-slice array is allocated before this bound is checked, so — like every other
    // decodeVariableList caller in this tree — an arena is used here rather than a leak-checking
    // allocator: decodeVariableList does not free that partial allocation on this error path, only
    // its caller's allocator lifetime does.
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const data = [_]u8{ 0x08, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00 };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(alloc, &data, 16));
}

test "decodeVariableList: rejects a huge offset without attempting a giant allocation" {
    // A hostile first-offset value that would otherwise demand allocating room for ~2^30 element
    // slices; must be rejected by the bounds check before any allocation is attempted.
    const data = [_]u8{ 0xFC, 0xFF, 0xFF, 0xFF };
    try std.testing.expectError(error.InvalidSsz, ssz.decodeVariableList(std.testing.allocator, &data, 1 << 16));
}
