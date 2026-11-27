//! The aggregation statement over the outputs of the real rollup statement: three adjacent rollup
//! requests built from the Linea mainnet window, each run through the rollup guest's statement,
//! proved under a rollup program VK with the devmode prover, and aggregated.

const std = @import("std");
const core_serde = @import("core_serde");
const rollup = @import("rollup");
const rollup_aggregation = @import("rollup_aggregation");
const rollup_scenario = @import("rollup_scenario");
const scenario = @import("scenario");

const Address = rollup_scenario.Address;
const Allocator = std.mem.Allocator;
const Fact = rollup_scenario.Fact;
const Hash = rollup_scenario.Hash;
const ProofSpec = rollup_scenario.ProofSpec;
const l2_l1_roots = rollup_aggregation.l2_l1_roots;
const rollup_proof = core_serde.rollup_proof;
const run = rollup_aggregation.rollup_aggregation.run;

const execution_vk_a: Hash = @splat(0x1e);
const execution_vk_b: Hash = @splat(0x1f);
const rollup_vk_a: Hash = @splat(0x2b);
const rollup_vk_b: Hash = @splat(0x2c);

const address_a: Address = @splat(0xa1);
const address_b: Address = @splat(0xa2);
const address_c: Address = @splat(0xa3);

const message_a: Hash = @splat(0x71);
const message_b: Hash = @splat(0x72);
const message_c: Hash = @splat(0x73);

/// What one rollup request of the window is made of: the program VK its proof is made under and, per
/// conflation, its blocks and its execution proof.
const RequestSpec = struct {
    program_vk: Hash,
    blocks: []const usize,
    proofs: []const ProofSpec,
};

// The five conflations of the window in order: their blocks, the offsets of their blocks that carry
// a message and their L2-to-L1 messages.
const first_blocks = 30;
const second_blocks = 20;
const third_blocks = 40;
const fourth_blocks = 35;
const fifth_blocks = 25;

const first_offsets = [_]u16{ 1, first_blocks };
const second_offsets = [_]u16{2};
const third_offsets = [_]u16{ 5, third_blocks };
const fourth_offsets = [_]u16{3};

const first_messages = [_]Hash{ message_a, message_b };
const second_messages = [_]Hash{message_c};
const fourth_messages = [_]Hash{ message_c, message_a };

/// The requests in window order; their conflations cover the window.
const requests = [_]RequestSpec{
    .{
        .program_vk = rollup_vk_a,
        .blocks = &.{ first_blocks, second_blocks },
        .proofs = &.{
            .{
                .program_vk = execution_vk_a,
                .l2_l1_messages = &first_messages,
                .filtered_addresses = &.{ address_b, address_a },
                .bridge_rolling_hash = @splat(0xb1),
                .bridge_message_number = 4,
                .messaging_offsets = &first_offsets,
            },
            .{
                .program_vk = execution_vk_b,
                .l2_l1_messages = &second_messages,
                .filtered_addresses = &.{ address_a, address_c },
                .messaging_offsets = &second_offsets,
            },
        },
    },
    .{
        .program_vk = rollup_vk_b,
        .blocks = &.{third_blocks},
        .proofs = &.{.{ .program_vk = execution_vk_a, .messaging_offsets = &third_offsets }},
    },
    .{
        .program_vk = rollup_vk_a,
        .blocks = &.{ fourth_blocks, fifth_blocks },
        .proofs = &.{
            .{
                .program_vk = execution_vk_b,
                .l2_l1_messages = &fourth_messages,
                .bridge_rolling_hash = @splat(0xb2),
                .bridge_message_number = 9,
                .messaging_offsets = &fourth_offsets,
            },
            .{ .program_vk = execution_vk_a },
        },
    },
};

/// The L2-to-L1 messages of the requests in window order.
const window_messages = first_messages ++ second_messages ++ fourth_messages;

/// The blocks of the window that carry a message, counted from one: every messaging offset of the
/// requests, moved by the blocks of the conflations before its own.
const window_messaging_blocks = [_]u16{
    first_offsets[0],
    first_offsets[1],
    first_blocks + second_offsets[0],
    first_blocks + second_blocks + third_offsets[0],
    first_blocks + second_blocks + third_offsets[1],
    first_blocks + second_blocks + third_blocks + fourth_offsets[0],
};

/// The rollup proofs of the window's requests: each runs the rollup statement over a request that
/// continues the one before it, and carries a devmode proof of its output under its program VK.
fn rollupProofs(a: Allocator, window: []const Fact) ![]rollup_proof.VerifiableRollupProof {
    var covered: usize = 0;
    for (requests) |spec| for (spec.blocks) |count| {
        covered += count;
    };
    try std.testing.expectEqual(window.len, covered);

    const proofs = try a.alloc(rollup_proof.VerifiableRollupProof, requests.len);
    var next_block: usize = 0;
    var executions: usize = 0;
    var previous: ?rollup_scenario.Request = null;
    for (requests, proofs) |spec, *verifiable| {
        const conflations = try a.alloc(rollup_scenario.Conflation, spec.blocks.len);
        for (spec.blocks, conflations) |count, *conflation| {
            conflation.* = try rollup_scenario.conflationFrom(a, rollup_scenario.linea_chain_id, window[next_block..][0..count]);
            next_block += count;
        }
        var options: rollup_scenario.Options = .{ .first_ftx_index = executions, .proofs = spec.proofs };
        if (previous) |request| options.parent_data_rolling_hash = request.end_data_rolling_hash;
        const request = try rollup_scenario.build(a, conflations, options);
        verifiable.* = .{
            .program_vk = spec.program_vk,
            .proof = .{
                .start_block_number = conflations[0].first_number,
                .proof = .{ .bytes = &.{} },
                .public_inputs = try rollup.rollup.run(a, request.input()),
            },
        };
        executions += spec.proofs.len;
        previous = request;
    }
    try scenario.prove(a, proofs);
    return proofs;
}

test "three adjacent rollup proofs of the Linea window finalize its range" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const window = try rollup_scenario.lineaWindow(a, std.testing.io);
    const proofs = try rollupProofs(a, window);
    const pi = try run(a, scenario.input(proofs));

    const first = proofs[0].proof.public_inputs;
    const last = proofs[proofs.len - 1].proof.public_inputs;
    const window_end = window[window.len - 1];

    try std.testing.expectEqual(window_end.number, pi.end_block_number);
    try std.testing.expectEqual(window_end.timestamp, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &window[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &window_end.hash, &pi.end_block_hash);

    try std.testing.expectEqualSlices(u8, &first.dynamic_chain_config_hash, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &first.parent_ftx_rolling_hash, &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(first.parent_ftx_number, pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &last.end_ftx_rolling_hash, &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(last.end_processed_ftx_number, pi.end_processed_ftx_number);

    try std.testing.expectEqualSlices(u8, &first.parent_data_rolling_hash, &pi.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &last.end_data_rolling_hash, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(first.parent_shared_blob_carry_over_tail_bytes, pi.parent_shared_blob_carry_over_tail_bytes);
    try std.testing.expectEqual(last.final_shared_blob_carry_over_tail_bytes, pi.final_shared_blob_carry_over_tail_bytes);

    const last_bridge = requests[requests.len - 1].proofs[0];
    try std.testing.expectEqualSlices(u8, &last_bridge.bridge_rolling_hash, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(last_bridge.bridge_message_number, pi.end_l1_l2_bridge_rolling_hash_message_number);

    try std.testing.expectEqualDeep(try l2_l1_roots.build(a, &window_messages), pi.l2_l1_roots.items);
    try std.testing.expectEqualDeep(&window_messaging_blocks, pi.l2_messaging_blocks_offsets.items);

    try std.testing.expectEqualDeep(&[_]Address{ address_a, address_b, address_c }, pi.filtered_addresses.items);
    try std.testing.expectEqualDeep(&[_]Hash{ execution_vk_a, execution_vk_b, rollup_vk_a, rollup_vk_b }, pi.program_ids.items);
}
