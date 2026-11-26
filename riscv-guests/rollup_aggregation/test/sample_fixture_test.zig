//! The committed sample request: the statement over it yields the committed output, the public
//! inputs of two adjacent rollup proofs.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");

const sample_request = @embedFile("sample.request.ssz");
const sample_output = @embedFile("sample.output.bin");

test "the statement over the sample request outputs the committed output" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const out = try rollup_aggregation.rollup_aggregation.runFramed(arena.allocator(), sample_request);
    try std.testing.expectEqualSlices(u8, sample_output, out);
}

test "the committed output is the finalization of the two proofs of the sample request" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const pi = try rollup_aggregation.rollup_aggregation_ssz.decodeOutput(a, sample_output);
    try std.testing.expectEqual(@as(u64, 1_000_515), pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 4), pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqual(@as(usize, 1), pi.l2_l1_roots.items.len);
    try std.testing.expectEqualDeep(&[_]u16{ 2, 7, 13 }, pi.l2_messaging_blocks_offsets.items);
    try std.testing.expectEqual(@as(usize, 2), pi.filtered_addresses.items.len);
    try std.testing.expectEqual(@as(usize, 2), pi.program_ids.items.len);
}
