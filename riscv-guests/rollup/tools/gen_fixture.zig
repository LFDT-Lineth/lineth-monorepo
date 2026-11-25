//! Writes the sample request of the rollup guest and the output the statement computes for it:
//! `gen_fixture REQUEST_PATH OUTPUT_PATH`. The request is the framed `RollupProofPrivateInput`
//! that `rollup_ssz.encodeInput` produces for one conflation of the first block of the Linea
//! mainnet window, an Osaka block, whose stream is carried by one blob chunk with the commitment
//! and proof of Constantine. The output is the framed public inputs `rollup.runFramed` returns for
//! it.
//!
//! The request is written as raw bytes to a path ending in `.ssz`, the form the length-prefixed
//! input of the guest is framed from.

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

    const window = try scenario.lineaWindow(alloc, init.io);
    const conflation = try scenario.conflationFrom(alloc, scenario.linea_chain_id, window[0..1]);
    const request = try scenario.build(alloc, &.{conflation}, .{});
    const encoded = try rollup.rollup_ssz.encodeInput(alloc, request.input());
    const output = try rollup.rollup.runFramed(alloc, encoded);

    const cwd = std.Io.Dir.cwd();
    try cwd.writeFile(init.io, .{ .sub_path = args[1], .data = encoded });
    try cwd.writeFile(init.io, .{ .sub_path = args[2], .data = output });
}
