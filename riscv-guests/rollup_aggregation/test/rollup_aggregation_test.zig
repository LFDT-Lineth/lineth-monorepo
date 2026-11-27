//! The rollup-aggregation statement over literal rollup proofs, each carrying a real devmode proof:
//! the public inputs of one proof and of a chain of three.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");
const scenario = @import("scenario");

const Allocator = std.mem.Allocator;
const FinalizationPublicInput = rollup_ssz.FinalizationPublicInput;
const l2_l1_roots = rollup_aggregation.l2_l1_roots;
const rollup_ssz = rollup_aggregation.rollup_aggregation_ssz;
const run = rollup_aggregation.rollup_aggregation.run;

// Each group is in ascending bytewise order, the order of the lists the statement outputs.
const address_a: [20]u8 = @splat(0xa1);
const address_b: [20]u8 = @splat(0xa2);
const address_c: [20]u8 = @splat(0xa3);
const execution_vk: [32]u8 = @splat(0x1f);
const rollup_vk_a = scenario.default_program_vk;
const rollup_vk_b: [32]u8 = @splat(0x2b);
const rollup_vk_c: [32]u8 = @splat(0x2c);

/// `count` distinct messages, the first of which is filled with `from`.
fn messages(a: Allocator, from: u8, count: usize) ![][32]u8 {
    const out = try a.alloc([32]u8, count);
    for (out, 0..) |*m, i| m.* = @splat(from +% @as(u8, @intCast(i)));
    return out;
}

/// `pi` starts at the parent boundary of the first proof of `specs` and ends at the end boundary
/// of the last.
fn expectSpansChain(specs: []const scenario.Spec, pi: FinalizationPublicInput) !void {
    const end = scenario.endBlock(specs);
    try std.testing.expectEqual(end, pi.end_block_number);
    try std.testing.expectEqual(scenario.timestamp(end), pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &scenario.chain_config_hash, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, 0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(0), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, specs.len), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(specs.len), pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, 0), &pi.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, specs.len), &pi.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, 0), &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, specs.len), &pi.end_block_hash);
    try std.testing.expectEqual(scenario.tailBytes(0), pi.parent_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(scenario.tailBytes(specs.len), pi.final_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(l2_l1_roots.tree_depth, pi.l2_l1_tree_depth);
}

test "one proof: its range, boundaries, bridge pair and lists are those of the finalization" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proof: scenario.Spec = .{
        .messages = try messages(a, 1, 3),
        .offsets = &.{ 2, 5 },
        .filtered_addresses = &.{ address_b, address_a },
        .program_vks = &.{execution_vk},
        .program_vk = rollup_vk_a,
        .bridge_rolling_hash = @splat(0xb1),
        .bridge_message_number = 4,
    };
    const pi = try run(a, try scenario.request(a, &.{proof}));

    try expectSpansChain(&.{proof}, pi);
    try std.testing.expectEqualSlices(u8, &proof.bridge_rolling_hash, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(proof.bridge_message_number, pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualDeep(try l2_l1_roots.build(a, proof.messages), pi.l2_l1_roots.items);
    try std.testing.expectEqualDeep(proof.offsets, pi.l2_messaging_blocks_offsets.items);
    try std.testing.expectEqualDeep(&[_][20]u8{ address_a, address_b }, pi.filtered_addresses.items);
    try std.testing.expectEqualDeep(&[_][32]u8{ execution_vk, rollup_vk_a }, pi.program_ids.items);
}

test "three adjacent proofs: the finalization spans the chain, rebases offsets and merges the lists" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const first: scenario.Spec = .{
        .block_count = 10,
        .messages = try messages(a, 1, 20),
        .offsets = &.{ 1, 10 },
        .filtered_addresses = &.{ address_b, address_a },
        .program_vks = &.{execution_vk},
        .program_vk = rollup_vk_a,
        .bridge_rolling_hash = @splat(0xb1),
        .bridge_message_number = 3,
    };
    const second: scenario.Spec = .{
        .block_count = 5,
        .messages = try messages(a, 101, 20),
        .offsets = &.{2},
        .filtered_addresses = &.{ address_a, address_c },
        .program_vk = rollup_vk_b,
    };
    const third: scenario.Spec = .{
        .block_count = 7,
        .program_vks = &.{ execution_vk, rollup_vk_a },
        .program_vk = rollup_vk_c,
    };
    const specs = [_]scenario.Spec{ first, second, third };
    const pi = try run(a, try scenario.request(a, &specs));

    try expectSpansChain(&specs, pi);

    // The first proof is the last one whose bridge pair has a message.
    try std.testing.expectEqualSlices(u8, &first.bridge_rolling_hash, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(first.bridge_message_number, pi.end_l1_l2_bridge_rolling_hash_message_number);

    const all_messages = try std.mem.concat(a, [32]u8, &.{ first.messages, second.messages, third.messages });
    try std.testing.expectEqualDeep(try l2_l1_roots.build(a, all_messages), pi.l2_l1_roots.items);

    try std.testing.expectEqualDeep(&[_]u16{
        first.offsets[0],
        first.offsets[1],
        @intCast(first.block_count + second.offsets[0]),
    }, pi.l2_messaging_blocks_offsets.items);
    try std.testing.expectEqualDeep(&[_][20]u8{ address_a, address_b, address_c }, pi.filtered_addresses.items);
    try std.testing.expectEqualDeep(&[_][32]u8{ execution_vk, rollup_vk_a, rollup_vk_b, rollup_vk_c }, pi.program_ids.items);
}

test "the framed statement outputs the encoded public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try scenario.request(a, &.{ .{}, .{} });
    const framed = try rollup_aggregation.rollup_aggregation.runFramed(a, try rollup_ssz.encodeInput(a, request));
    try std.testing.expectEqualDeep(try run(a, request), try rollup_ssz.decodeOutput(a, framed));
}
