const std = @import("std");
const example = @import("example");

pub fn main() !void {
    // Prints to stderr, ignoring potential errors.
    std.debug.print("All your {s} are belong to us.\n", .{"codebase"});
    try example.bufferedPrint();
}
