//! The canonical forms of the lists the guests commit to.

const std = @import("std");

const lists = @import("guest_common").lists;

fn addressOf(byte: u8) [20]u8 {
    return @splat(byte);
}

test "sortedDistinct orders the elements ascending bytewise and drops the repeated ones" {
    var items = [_][20]u8{ addressOf(0x05), addressOf(0x03), addressOf(0x05), addressOf(0x04), addressOf(0x03) };
    const distinct = lists.sortedDistinct(20, &items);
    try std.testing.expectEqualSlices([20]u8, &.{ addressOf(0x03), addressOf(0x04), addressOf(0x05) }, distinct);
    // The result is the head of the input.
    try std.testing.expectEqual(@intFromPtr(&items[0]), @intFromPtr(distinct.ptr));
}

test "sortedDistinct compares elements from their first byte" {
    var low_first = addressOf(0x00);
    low_first[0] = 0x01;
    var high_last = addressOf(0x00);
    high_last[19] = 0xff;
    var items = [_][20]u8{ high_last, low_first };
    try std.testing.expectEqualSlices([20]u8, &.{ high_last, low_first }, lists.sortedDistinct(20, &items));
}

test "sortedDistinct of no element and of one element" {
    var none: [0][32]u8 = .{};
    try std.testing.expectEqual(@as(usize, 0), lists.sortedDistinct(32, &none).len);
    var one = [_][32]u8{@splat(0x07)};
    try std.testing.expectEqualSlices([32]u8, &.{@as([32]u8, @splat(0x07))}, lists.sortedDistinct(32, &one));
}

test "sortedDistinct of equal elements is one element" {
    var items = [_][32]u8{ @splat(0x09), @splat(0x09), @splat(0x09) };
    try std.testing.expectEqualSlices([32]u8, &.{@as([32]u8, @splat(0x09))}, lists.sortedDistinct(32, &items));
}

test "sortedDistinct agrees with a reference over random lists" {
    var prng = std.Random.DefaultPrng.init(2026);
    const random = prng.random();
    for (0..50) |_| {
        var items: [40][4]u8 = undefined;
        // A narrow value range makes repeated elements common.
        for (&items) |*item| for (item) |*byte| {
            byte.* = random.uintLessThan(u8, 3);
        };

        var expected: [40][4]u8 = items;
        var expected_len: usize = 0;
        for (items) |candidate| {
            var seen = false;
            for (expected[0..expected_len]) |kept| seen = seen or std.mem.eql(u8, &kept, &candidate);
            if (!seen) {
                expected[expected_len] = candidate;
                expected_len += 1;
            }
        }
        // Insertion sort of the distinct elements.
        for (1..expected_len) |i| {
            var j = i;
            while (j > 0 and std.mem.order(u8, &expected[j - 1], &expected[j]) == .gt) : (j -= 1) {
                std.mem.swap([4]u8, &expected[j - 1], &expected[j]);
            }
        }

        try std.testing.expectEqualSlices([4]u8, expected[0..expected_len], lists.sortedDistinct(4, &items));
    }
}
