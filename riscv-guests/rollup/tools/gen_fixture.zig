//! Writes the sample request of the rollup guest and the output the statement computes for it:
//! `gen_fixture REQUEST_PATH OUTPUT_PATH`. The request is a framed `RollupProofPrivateInput` this
//! package's own `rollup_ssz.encodeInput` produced from a valid request of one conflation whose
//! stream is carried by one blob chunk, with the commitment and proof of c-kzg-4844. The output is
//! the framed public inputs `rollup.runFramed` returns for it, which the guest has to emit too.
//! Invoked by this guest's own Makefile (`require-input`) to regenerate the default input of
//! `make exec`/`debug` and the output `make zkc-smoke-exec` compares with on every run; never
//! checked into git.
//!
//! The request is raw bytes, not hex text: `elf_to_json_gen`'s `@path` input mode dispatches on
//! file extension — a `.ssz`-suffixed file is read as the raw payload and framed with the 8-byte
//! length prefix `read_input` requires (`sszInputBlobs`); anything else is placed at the input
//! offset verbatim, with no length prefix at all. `read_input` always expects that prefix
//! (matching l2-execution's own convention), so the request path must take the `.ssz` form.

const std = @import("std");
const rollup = @import("rollup");
const scenario = @import("scenario");

pub fn main(init: std.process.Init) !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const args = try init.minimal.args.toSlice(init.arena.allocator());
    if (args.len != 3) {
        std.debug.print("usage: gen_fixture REQUEST_PATH OUTPUT_PATH\n", .{});
        std.process.exit(2);
    }

    const conflations = try scenario.conflations(alloc, .{}, &.{.{ .blocks = 3 }});
    const request = try scenario.build(alloc, conflations, .{});
    const encoded = try rollup.rollup_ssz.encodeInput(alloc, request.input());
    const output = try rollup.rollup.runFramed(alloc, encoded);

    const cwd = std.Io.Dir.cwd();
    try cwd.writeFile(init.io, .{ .sub_path = args[1], .data = encoded });
    try cwd.writeFile(init.io, .{ .sub_path = args[2], .data = output });
}
