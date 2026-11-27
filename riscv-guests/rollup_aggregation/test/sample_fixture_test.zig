//! The committed sample request and output: the request is the sample chain, and the statement
//! over it yields the committed output.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");
const scenario = @import("scenario");

const sample_request = @embedFile("sample.request.ssz");
const sample_output = @embedFile("sample.output.bin");

test "the committed request is the sample chain" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();
    const request = try scenario.request(a, &scenario.sample);
    try std.testing.expectEqualSlices(u8, try rollup_aggregation.rollup_aggregation_ssz.encodeInput(a, request), sample_request);
}

test "the statement over the sample request outputs the committed output" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const out = try rollup_aggregation.rollup_aggregation.runFramed(arena.allocator(), sample_request);
    try std.testing.expectEqualSlices(u8, sample_output, out);
}
