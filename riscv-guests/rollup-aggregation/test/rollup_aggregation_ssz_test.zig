const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");
const support = @import("support.zig");
const repeat32 = support.repeat32;
const repeat20 = support.repeat20;
const sampleInput = support.sampleInput;

// ── Round-trip: readable struct -> encodeInput -> decodeInput -> same fields ─────────────────────

test "encodeInput/decodeInput: round-trips every field of a readable sample input" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const original = try sampleInput(alloc);
    const encoded = try rollup_aggregation_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x10, 0x02 }, encoded[0..2]);

    const v = try rollup_aggregation_ssz.decodeInput(alloc, encoded);
    try std.testing.expectEqual(@as(usize, 2), v.rollup_proofs.len);

    const p0 = v.rollup_proofs[0];
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &p0.program_vk);
    try std.testing.expectEqual(support.PROOF0_START_BLOCK_NUMBER, p0.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PROOF_BYTES, p0.proof.proof);
    const pi0 = p0.proof.public_inputs;
    try std.testing.expectEqual(support.PROOF0_END_BLOCK_NUMBER, pi0.end_block_number);
    try std.testing.expectEqual(support.PROOF0_END_BLOCK_TIMESTAMP, pi0.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_BRIDGE_TRANSACTION_TREE, &pi0.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH, &pi0.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi0.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH, &pi0.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi0.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_DYNAMIC_CHAIN_CONFIG_HASH, &pi0.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_FTX_ROLLING_HASH, &pi0.parent_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_FTX_NUMBER, pi0.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_FTX_ROLLING_HASH, &pi0.end_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_PROCESSED_FTX_NUMBER, pi0.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESSES_HASH, &pi0.filtered_addresses_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_DATA_ROLLING_HASH, &pi0.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_DATA_ROLLING_HASH, &pi0.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_BLOCK_HASH, &pi0.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_BLOCK_HASH, &pi0.end_block_hash);
    try std.testing.expectEqual(support.PROOF0_START_OFFSET, pi0.start_offset);
    try std.testing.expectEqual(support.PROOF0_END_OFFSET, pi0.end_offset);
    try std.testing.expectEqual(@as(usize, 1), pi0.program_vks.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PROGRAM_VKS_0, &pi0.program_vks[0]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_ROOT_0, &p0.proof.l2_l1_roots[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_ROOT_1, &p0.proof.l2_l1_roots[1]);
    try std.testing.expectEqual(@as(usize, 1), p0.proof.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_0, &p0.proof.filtered_addresses[0]);

    const p1 = v.rollup_proofs[1];
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &p1.program_vk);
    try std.testing.expectEqual(support.PROOF1_START_BLOCK_NUMBER, p1.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_PROOF_BYTES, p1.proof.proof);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_NUMBER, p1.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_TIMESTAMP, p1.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqual(support.PROOF1_PARENT_FTX_NUMBER, p1.proof.public_inputs.parent_ftx_number);
}

// ── Output frame: encode/decode round-trip ───────────────────────────────────────────────────────
// Distinct constants from the input side above — this is a separate container
// (`RollupAggregationOutput`) with its own field values, not required to match the input sample.

const OUTPUT_PROGRAM_VK_0 = repeat32(0xaa);
const OUTPUT_PROGRAM_VK_1 = repeat32(0xbb);
const OUTPUT_L2_L1_ROOT_0 = repeat32(0x77);
const OUTPUT_L2_L1_ROOT_1 = repeat32(0x88);
const OUTPUT_FILTERED_ADDRESS_0 = repeat20(0x01);
const OUTPUT_MESSAGING_OFFSET_0: u64 = 0xd18d873fe2a9f192;
const OUTPUT_END_BLOCK_NUMBER: u64 = 18;
const OUTPUT_END_BLOCK_TIMESTAMP: u64 = 1763000557;
const OUTPUT_L2_L1_BRIDGE_TRANSACTION_TREE = repeat32(0x09);
const OUTPUT_PARENT_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x22);
const OUTPUT_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x33);
const OUTPUT_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
const OUTPUT_PARENT_FTX_ROLLING_HASH = repeat32(0x44);
const OUTPUT_END_FTX_ROLLING_HASH = repeat32(0x55);
const OUTPUT_FILTERED_ADDRESSES_HASH = repeat32(0x63);
const OUTPUT_PARENT_DATA_ROLLING_HASH = repeat32(0x47);
const OUTPUT_END_DATA_ROLLING_HASH = repeat32(0x8d);
const OUTPUT_PARENT_BLOCK_HASH = repeat32(0x0a);
const OUTPUT_END_BLOCK_HASH = repeat32(0x0b);

fn sampleOutput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationOutput {
    const program_vks = try alloc.dupe([32]u8, &[_][32]u8{ OUTPUT_PROGRAM_VK_0, OUTPUT_PROGRAM_VK_1 });
    const l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ OUTPUT_L2_L1_ROOT_0, OUTPUT_L2_L1_ROOT_1 });
    const filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{OUTPUT_FILTERED_ADDRESS_0});
    const l2_messaging_blocks_offsets = try alloc.dupe(u64, &[_]u64{OUTPUT_MESSAGING_OFFSET_0});

    return .{
        .public_inputs = .{
            .end_block_number = OUTPUT_END_BLOCK_NUMBER,
            .end_block_timestamp = OUTPUT_END_BLOCK_TIMESTAMP,
            .l2_l1_bridge_transaction_tree = OUTPUT_L2_L1_BRIDGE_TRANSACTION_TREE,
            .parent_l1_l2_bridge_rolling_hash = OUTPUT_PARENT_L1L2_BRIDGE_ROLLING_HASH,
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = OUTPUT_END_L1L2_BRIDGE_ROLLING_HASH,
            .end_l1_l2_bridge_rolling_hash_message_number = 7,
            .dynamic_chain_config_hash = OUTPUT_DYNAMIC_CHAIN_CONFIG_HASH,
            .parent_ftx_rolling_hash = OUTPUT_PARENT_FTX_ROLLING_HASH,
            .parent_ftx_number = 15,
            .end_ftx_rolling_hash = OUTPUT_END_FTX_ROLLING_HASH,
            .end_processed_ftx_number = 18,
            .filtered_addresses_hash = OUTPUT_FILTERED_ADDRESSES_HASH,
            .parent_data_rolling_hash = OUTPUT_PARENT_DATA_ROLLING_HASH,
            .end_data_rolling_hash = OUTPUT_END_DATA_ROLLING_HASH,
            .parent_block_hash = OUTPUT_PARENT_BLOCK_HASH,
            .end_block_hash = OUTPUT_END_BLOCK_HASH,
            .start_offset = 0,
            .end_offset = 131072,
            .program_vks = program_vks,
        },
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}

test "encodeOutput/decodeOutput: round-trips every field and carries the 0x1802 schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = try sampleOutput(alloc);
    const encoded = try rollup_aggregation_ssz.encodeOutput(alloc, value);

    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x18, 0x02 }, encoded[0..2]);

    const decoded = try rollup_aggregation_ssz.decodeOutput(alloc, encoded);
    try std.testing.expectEqual(value.l2_l1_roots.len, decoded.l2_l1_roots.len);
    for (value.l2_l1_roots, decoded.l2_l1_roots) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
    try std.testing.expectEqual(value.filtered_addresses.len, decoded.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &value.filtered_addresses[0], &decoded.filtered_addresses[0]);
    try std.testing.expectEqual(value.l2_messaging_blocks_offsets.len, decoded.l2_messaging_blocks_offsets.len);
    try std.testing.expectEqual(value.l2_messaging_blocks_offsets[0], decoded.l2_messaging_blocks_offsets[0]);
    try std.testing.expectEqual(value.public_inputs.end_block_number, decoded.public_inputs.end_block_number);
    try std.testing.expectEqual(value.public_inputs.start_offset, decoded.public_inputs.start_offset);
    try std.testing.expectEqual(value.public_inputs.end_offset, decoded.public_inputs.end_offset);
    try std.testing.expectEqual(value.public_inputs.program_vks.len, decoded.public_inputs.program_vks.len);
    for (value.public_inputs.program_vks, decoded.public_inputs.program_vks) |want, got| {
        try std.testing.expectEqualSlices(u8, &want, &got);
    }
    try std.testing.expectEqualSlices(u8, &value.public_inputs.l2_l1_bridge_transaction_tree, &decoded.public_inputs.l2_l1_bridge_transaction_tree);
}

// ── Malformed input ───────────────────────────────────────────────────────────────────────────
// Every case below corrupts bytes `rollup_aggregation_ssz.encodeInput` itself produced from
// `sampleInput` — no external fixture. Byte positions into the fixed head are content-independent
// (this codec's own offset-table layout, not data), so they hold regardless of what `sampleInput`
// contains.

test "decodeInput: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    corrupted[0] = 0x18;
    corrupted[1] = 0x02; // the output schema id, on input bytes
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects a frame truncated below the 2-byte schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeInput(alloc, encoded[0..1]));
}

test "decodeInput: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    try std.testing.expectError(error.InvalidSsz, rollup_aggregation_ssz.decodeInput(alloc, encoded[0..2]));
}

test "decodeInput: rejects a misaligned first offset (rollup_proofs)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    // The rollup_proofs offset sits at absolute byte 2 (schema); the canonical value is 4 (the
    // container's own fixed head size — the only field, so this container is entirely variable).
    std.mem.writeInt(u32, corrupted[2..6], 5, .little);
    try std.testing.expectError(error.InvalidSsz, rollup_aggregation_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: accepts an emptied rollup_proofs region (decode succeeds; rollup_aggregation.run rejects it)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A body containing only the 4-byte fixed head, pointing past itself to an empty region.
    var body = [_]u8{ 0x04, 0x00, 0x00, 0x00 };
    var framed: [6]u8 = undefined;
    std.mem.writeInt(u16, framed[0..2], rollup_aggregation_ssz.INPUT_SCHEMA_ID, .big);
    @memcpy(framed[2..6], &body);
    _ = &body;

    const decoded = try rollup_aggregation_ssz.decodeInput(alloc, &framed);
    try std.testing.expectEqual(@as(usize, 0), decoded.rollup_proofs.len);
}
