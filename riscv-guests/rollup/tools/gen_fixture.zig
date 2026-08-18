//! Prints, as 0x-prefixed hex on stdout, a framed `SszRollupProofPrivateInput` this package's own
//! `rollup_ssz.encodeInput` produced from `support.sampleInput` — the same sample the tests use.
//! Invoked by this guest's own Makefile (`require-input`) to regenerate `make exec`/`debug`'s
//! default input on every run; never checked into git.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
const support = @import("support.zig");

pub fn main(init: std.process.Init) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try support.sampleInput(alloc);
    const encoded = try rollup_ssz.encodeInput(alloc, input);

    var out: std.ArrayListUnmanaged(u8) = .empty;
    try out.appendSlice(alloc, "0x");
    const hex_digits = "0123456789abcdef";
    for (encoded) |byte| {
        try out.append(alloc, hex_digits[byte >> 4]);
        try out.append(alloc, hex_digits[byte & 0x0f]);
    }
    try out.append(alloc, '\n');

    try std.Io.File.stdout().writeStreamingAll(init.io, out.items);
}
