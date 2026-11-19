//! The canonical forms of the lists the guests commit to.

const std = @import("std");

/// The distinct elements of `items`, sorted ascending bytewise, in place at the head of `items`.
pub fn sortedDistinct(comptime n: usize, items: [][n]u8) [][n]u8 {
    std.mem.sort([n]u8, items, {}, struct {
        fn lessThan(_: void, a: [n]u8, b: [n]u8) bool {
            return std.mem.order(u8, &a, &b) == .lt;
        }
    }.lessThan);

    var unique_count: usize = 0;
    for (items) |item| {
        if (unique_count == 0 or !std.mem.eql(u8, &item, &items[unique_count - 1])) {
            items[unique_count] = item;
            unique_count += 1;
        }
    }
    return items[0..unique_count];
}
