//! Prints, as raw bytes on stdout, a framed `SszRollupProofPrivateInput` this package's own
//! `rollup_ssz.encodeInput` produced from `support.sampleInput` — the same sample the tests use.
//! Invoked by this guest's own Makefile (`require-input`) to regenerate `make exec`/`debug`'s
//! default input (named `.ssz`, raw bytes) on every run; never checked into git.
//!
//! Raw bytes, not hex text: `elf_to_json_gen`'s `@path` input mode dispatches on file extension —
//! a `.ssz`-suffixed file is read as the raw payload and framed with the 8-byte length prefix
//! `read_input` requires (`sszInputBlobs`); anything else is placed at the input offset verbatim,
//! with no length prefix at all. `read_input` always expects that prefix (matching l2-execution's
//! own convention), so this file must take the `.ssz` path.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
const support = @import("support.zig");

pub fn main(init: std.process.Init) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try support.sampleInput(alloc);
    const encoded = try rollup_ssz.encodeInput(alloc, input);

    try std.Io.File.stdout().writeStreamingAll(init.io, encoded);
}
