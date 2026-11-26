//! The rollup-aggregation statement over literal rollup proofs, each carrying a real devmode proof:
//! the public inputs of one proof and of a chain of three, and the rejection of a chain that does
//! not tile or continue, of a proof that does not verify and of a request without proofs.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");
const scenario = @import("scenario");

const Allocator = std.mem.Allocator;
const l2_l1_roots = rollup_aggregation.l2_l1_roots;
const rollup_ssz = rollup_aggregation.rollup_aggregation_ssz;
const run = rollup_aggregation.rollup_aggregation.run;

fn addressOf(byte: u8) [20]u8 {
    return @splat(byte);
}

fn hashOf(byte: u8) [32]u8 {
    return @splat(byte);
}

/// `count` distinct messages, the first of which is `from`.
fn messages(a: Allocator, from: u8, count: usize) ![][32]u8 {
    const out = try a.alloc([32]u8, count);
    for (out, 0..) |*m, i| m.* = hashOf(from +% @as(u8, @intCast(i)));
    return out;
}

test "one proof: its range, boundaries, bridge pair and lists are those of the finalization" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const own_messages = try messages(a, 1, 3);
    const pi = try run(a, try scenario.request(a, &.{.{
        .messages = own_messages,
        .offsets = &.{ 2, 5 },
        .filtered_addresses = &.{ addressOf(0xa2), addressOf(0xa1) },
        .program_vks = &.{hashOf(0x1f)},
        .bridge_rolling_hash = hashOf(0xb1),
        .bridge_message_number = 4,
    }}));

    try std.testing.expectEqual(scenario.first_block + 9, pi.end_block_number);
    try std.testing.expectEqual(1_763_000_000 + scenario.first_block + 9, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &hashOf(0xb1), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &scenario.chain_config_hash, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, 0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(0), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, 1), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(1), pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, 0), &pi.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, 1), &pi.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, 0), &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, 1), &pi.end_block_hash);
    try std.testing.expectEqual(scenario.tailBytes(0), pi.parent_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(scenario.tailBytes(1), pi.final_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(@as(u64, 5), pi.l2_l1_tree_depth);

    const roots = try l2_l1_roots.build(a, own_messages);
    try std.testing.expectEqual(@as(usize, 1), pi.l2_l1_roots.items.len);
    try std.testing.expectEqualSlices(u8, &roots[0], &pi.l2_l1_roots.items[0]);
    try std.testing.expectEqualDeep(&[_]u16{ 2, 5 }, pi.l2_messaging_blocks_offsets.items);
    try std.testing.expectEqualDeep(&[_][20]u8{ addressOf(0xa1), addressOf(0xa2) }, pi.filtered_addresses.items);
    try std.testing.expectEqualDeep(&[_][32]u8{ hashOf(0x1f), hashOf(0x2a) }, pi.program_ids.items);
}

test "three adjacent proofs: the finalization spans the chain, rebases offsets and merges the lists" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const first_messages = try messages(a, 1, 20);
    const second_messages = try messages(a, 101, 20);
    const pi = try run(a, try scenario.request(a, &.{
        .{
            .block_count = 10,
            .messages = first_messages,
            .offsets = &.{ 1, 10 },
            .filtered_addresses = &.{ addressOf(0xa2), addressOf(0xa1) },
            .program_vks = &.{hashOf(0x1f)},
            .bridge_rolling_hash = hashOf(0xb1),
            .bridge_message_number = 3,
        },
        .{
            .block_count = 5,
            .messages = second_messages,
            .offsets = &.{2},
            .filtered_addresses = &.{ addressOf(0xa1), addressOf(0xa3) },
            .program_vk = hashOf(0x2b),
        },
        .{
            .block_count = 7,
            .program_vks = &.{ hashOf(0x1f), hashOf(0x2a) },
            .program_vk = hashOf(0x2c),
        },
    }));

    try std.testing.expectEqual(scenario.first_block + 21, pi.end_block_number);
    try std.testing.expectEqual(1_763_000_000 + scenario.first_block + 21, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &hashOf(0xb1), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 3), pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &scenario.chain_config_hash, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, 0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(0), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.ftx_rolling_hash, 3), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(scenario.ftxNumber(3), pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, 0), &pi.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.data_rolling_hash, 3), &pi.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, 0), &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &scenario.boundary(.block_hash, 3), &pi.end_block_hash);
    try std.testing.expectEqual(scenario.tailBytes(0), pi.parent_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(scenario.tailBytes(3), pi.final_shared_blob_carry_over_tail_bytes);

    const all_messages = try std.mem.concat(a, [32]u8, &.{ first_messages, second_messages });
    const first_root = try l2_l1_roots.build(a, all_messages[0..32]);
    const second_root = try l2_l1_roots.build(a, all_messages[32..]);
    try std.testing.expectEqual(@as(usize, 2), pi.l2_l1_roots.items.len);
    try std.testing.expectEqualSlices(u8, &first_root[0], &pi.l2_l1_roots.items[0]);
    try std.testing.expectEqualSlices(u8, &second_root[0], &pi.l2_l1_roots.items[1]);

    try std.testing.expectEqualDeep(&[_]u16{ 1, 10, 12 }, pi.l2_messaging_blocks_offsets.items);
    try std.testing.expectEqualDeep(&[_][20]u8{ addressOf(0xa1), addressOf(0xa2), addressOf(0xa3) }, pi.filtered_addresses.items);
    try std.testing.expectEqualDeep(&[_][32]u8{ hashOf(0x1f), hashOf(0x2a), hashOf(0x2b), hashOf(0x2c) }, pi.program_ids.items);
}

test "the framed statement outputs the encoded public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try scenario.request(a, &.{ .{}, .{ .block_count = 4 } });
    const framed = try rollup_aggregation.rollup_aggregation.runFramed(a, try rollup_ssz.encodeInput(a, request));
    try std.testing.expectEqualDeep(try run(a, request), try rollup_ssz.decodeOutput(a, framed));
}

test "a request without proofs is rejected" {
    try std.testing.expectError(error.InvalidRequest, run(std.testing.allocator, scenario.input(&.{})));
}

test "proofs that leave a gap between their block ranges are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.start_block_number += 1;
    try scenario.prove(a, proofs);
    try std.testing.expectError(error.TilingBroken, run(a, scenario.input(proofs)));
}

test "proofs that do not continue each other's block hash are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_block_hash = hashOf(0xee);
    try scenario.prove(a, proofs);
    try std.testing.expectError(error.ContinuityBroken, run(a, scenario.input(proofs)));
}

test "a proof of another public input or another program VK is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    try scenario.prove(a, proofs);

    proofs[0].proof.public_inputs.end_block_timestamp += 1;
    try std.testing.expectError(error.RecursiveProofRejected, run(a, scenario.input(proofs)));
    proofs[0].proof.public_inputs.end_block_timestamp -= 1;
    try std.testing.expectEqual(scenario.first_block + 9, (try run(a, scenario.input(proofs))).end_block_number);

    proofs[0].program_vk = hashOf(0x2b);
    try std.testing.expectError(error.RecursiveProofRejected, run(a, scenario.input(proofs)));
}
