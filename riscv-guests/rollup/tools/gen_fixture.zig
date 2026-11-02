//! Prints, as raw bytes on stdout, a framed `RollupProofPrivateInput` this package's own
//! `rollup_ssz.encodeInput` produced from a valid request of one conflation whose stream is carried
//! by one blob chunk, with the commitment and proof of c-kzg-4844, and checked by running the
//! statement over it. Invoked by this guest's own Makefile (`require-input`) to regenerate
//! `make exec`/`debug`'s default input (named `.ssz`, raw bytes) on every run; never checked into git.
//!
//! Raw bytes, not hex text: `elf_to_json_gen`'s `@path` input mode dispatches on file extension —
//! a `.ssz`-suffixed file is read as the raw payload and framed with the 8-byte length prefix
//! `read_input` requires (`sszInputBlobs`); anything else is placed at the input offset verbatim,
//! with no length prefix at all. `read_input` always expects that prefix (matching l2-execution's
//! own convention), so this file must take the `.ssz` path.

const std = @import("std");
const rollup = @import("rollup");
const scenario = @import("scenario");

pub fn main(init: std.process.Init) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const conflations = try scenario.conflations(alloc, .{}, &.{.{ .blocks = 3 }});
    const request = try scenario.build(alloc, conflations, .{});
    _ = try rollup.rollup.run(alloc, request.input());

    const encoded = try rollup.rollup_ssz.encodeInput(alloc, request.input());
    try std.Io.File.stdout().writeStreamingAll(init.io, encoded);
}
