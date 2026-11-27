//! Writes the sample request of the rollup-aggregation guest and the output the statement computes
//! for it: `gen_fixture REQUEST_PATH OUTPUT_PATH`. The request is the framed
//! `RollupAggregationInput` that `rollup_aggregation_ssz.encodeInput` produces for two adjacent
//! rollup proofs with L2-to-L1 messages, filtered addresses and messaging offsets, each carrying a
//! real devmode proof. The output is the framed public inputs `rollup_aggregation.runFramed`
//! returns for it.
//!
//! The request is written as raw bytes to a path ending in `.ssz`, the form the length-prefixed
//! input of the guest is framed from.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");
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

    const request = try scenario.request(alloc, &scenario.sample);
    const encoded = try rollup_aggregation.rollup_aggregation_ssz.encodeInput(alloc, request);
    const output = try rollup_aggregation.rollup_aggregation.runFramed(alloc, encoded);

    const cwd = std.Io.Dir.cwd();
    try cwd.writeFile(init.io, .{ .sub_path = args[1], .data = encoded });
    try cwd.writeFile(init.io, .{ .sub_path = args[2], .data = output });
}
