const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
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
    const encoded = try rollup_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x10, 0x01 }, encoded[0..2]);

    const v = try rollup_ssz.decodeInput(alloc, encoded);

    try std.testing.expectEqualSlices(u8, &support.PARENT_DATA_ROLLING_HASH, &v.parent_data_rolling_hash);
    try std.testing.expectEqual(support.START_OFFSET, v.start_offset);
    try std.testing.expectEqual(support.CHAIN_ID, v.chain_id);

    try std.testing.expectEqual(@as(usize, 2), v.conflations.len);
    try std.testing.expectEqual(@as(usize, 2), v.conflations[0].block_rlps.len);
    try std.testing.expectEqualSlices(u8, &support.CONFLATION_0_BLOCK_RLP_0, v.conflations[0].block_rlps[0]);
    try std.testing.expectEqualSlices(u8, &support.CONFLATION_0_BLOCK_RLP_1, v.conflations[0].block_rlps[1]);
    try std.testing.expectEqualSlices(u8, &support.CONFLATION_1_BLOCK_RLP_0, v.conflations[1].block_rlps[0]);
    try std.testing.expectEqualSlices(u8, &support.CONFLATION_1_BLOCK_RLP_1, v.conflations[1].block_rlps[1]);

    try std.testing.expectEqual(@as(usize, 1), v.chunks.len);
    try std.testing.expectEqualSlices(u8, &support.CHUNK_0, &v.chunks[0]);

    try std.testing.expectEqualSlices(u8, &support.OPAQUE_PREFIX_BYTES, v.opaque_prefix_bytes);
    try std.testing.expectEqual(@as(usize, 0), v.opaque_suffix_bytes.len);
    try std.testing.expect(v.boundary_prev_data_rolling_hash != null);
    try std.testing.expectEqualSlices(u8, &support.BOUNDARY_PREV_DATA_ROLLING_HASH, &v.boundary_prev_data_rolling_hash.?);

    try std.testing.expectEqual(@as(usize, 2), v.l2_execution_proofs.len);

    const p0 = v.l2_execution_proofs[0];
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &p0.program_vk);
    try std.testing.expectEqual(support.PROOF0_START_BLOCK_NUMBER, p0.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PROOF_BYTES, p0.proof.proof);
    try std.testing.expectEqual(support.PROOF0_END_BLOCK_NUMBER, p0.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(support.PROOF0_END_BLOCK_TIMESTAMP, p0.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_BLOCK_HASH, &p0.proof.public_inputs.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_BLOCK_HASH, &p0.proof.public_inputs.end_block_hash);
    try std.testing.expectEqual(@as(usize, 1), p0.proof.l2_l1_messages.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_MESSAGE_0, &p0.proof.l2_l1_messages[0]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.tx_froms.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_TX_FROM_0, &p0.proof.tx_froms[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_TX_FROM_1, &p0.proof.tx_froms[1]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_0, &p0.proof.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_1, &p0.proof.filtered_addresses[1]);

    const p1 = v.l2_execution_proofs[1];
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &p1.program_vk);
    try std.testing.expectEqual(support.PROOF1_START_BLOCK_NUMBER, p1.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_PROOF_BYTES, p1.proof.proof);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_NUMBER, p1.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_TIMESTAMP, p1.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqual(support.PROOF1_PARENT_FTX_NUMBER, p1.proof.public_inputs.parent_ftx_number);
    try std.testing.expectEqual(support.PROOF1_END_PROCESSED_FTX_NUMBER, p1.proof.public_inputs.end_processed_ftx_number);
}

// ── Output frame: encode/decode round-trip ───────────────────────────────────────────────────────
// Distinct constants from the input side above — this is a separate container (`RollupOutput`) with
// its own field values, not required to match the input sample.

const OUTPUT_PROGRAM_VK_0 = repeat32(0xaa);
const OUTPUT_PROGRAM_VK_1 = repeat32(0xbb);
const OUTPUT_L2_L1_ROOT_0 = repeat32(0x45);
const OUTPUT_FILTERED_ADDRESS_0 = repeat20(0x03);
const OUTPUT_FILTERED_ADDRESS_1 = repeat20(0x04);
const OUTPUT_END_BLOCK_NUMBER: u64 = 14;
const OUTPUT_END_BLOCK_TIMESTAMP: u64 = 1763000210;
const OUTPUT_L2_L1_BRIDGE_TRANSACTION_TREE = repeat32(0xbc);
const OUTPUT_PARENT_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x02);
const OUTPUT_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x03);
const OUTPUT_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
const OUTPUT_PARENT_FTX_ROLLING_HASH = repeat32(0x04);
const OUTPUT_END_FTX_ROLLING_HASH = repeat32(0x05);
const OUTPUT_FILTERED_ADDRESSES_HASH = repeat32(0x8f);
const OUTPUT_PARENT_DATA_ROLLING_HASH = repeat32(0x47);
const OUTPUT_END_DATA_ROLLING_HASH = repeat32(0x1f);
const OUTPUT_PARENT_BLOCK_HASH = repeat32(0x0a);
const OUTPUT_END_BLOCK_HASH = repeat32(0x0b);
const OUTPUT_START_OFFSET: u64 = 4;
const OUTPUT_END_OFFSET: u64 = 0x1ab5956f53caf2ea;
const OUTPUT_START_BLOCK_NUMBER: u64 = 10;

fn sampleOutput(alloc: std.mem.Allocator) !rollup_ssz.RollupOutput {
    const program_vks = try alloc.dupe([32]u8, &[_][32]u8{ OUTPUT_PROGRAM_VK_0, OUTPUT_PROGRAM_VK_1 });
    const l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{OUTPUT_L2_L1_ROOT_0});
    const filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ OUTPUT_FILTERED_ADDRESS_0, OUTPUT_FILTERED_ADDRESS_1 });

    return .{
        .public_inputs = .{
            .end_block_number = OUTPUT_END_BLOCK_NUMBER,
            .end_block_timestamp = OUTPUT_END_BLOCK_TIMESTAMP,
            .l2_l1_bridge_transaction_tree = OUTPUT_L2_L1_BRIDGE_TRANSACTION_TREE,
            .parent_l1_l2_bridge_rolling_hash = OUTPUT_PARENT_L1L2_BRIDGE_ROLLING_HASH,
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = OUTPUT_END_L1L2_BRIDGE_ROLLING_HASH,
            .end_l1_l2_bridge_rolling_hash_message_number = 4,
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
            .start_offset = OUTPUT_START_OFFSET,
            .end_offset = OUTPUT_END_OFFSET,
            .program_vks = program_vks,
        },
        .start_block_number = OUTPUT_START_BLOCK_NUMBER,
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
    };
}

test "encodeOutput/decodeOutput: round-trips every field and carries the 0x1801 schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = try sampleOutput(alloc);
    const encoded = try rollup_ssz.encodeOutput(alloc, value);

    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x18, 0x01 }, encoded[0..2]);

    const decoded = try rollup_ssz.decodeOutput(alloc, encoded);
    try std.testing.expectEqual(value.start_block_number, decoded.start_block_number);
    try std.testing.expectEqualSlices(u8, value.filtered_addresses[0][0..], decoded.filtered_addresses[0][0..]);
    try std.testing.expectEqual(value.l2_l1_roots.len, decoded.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &value.l2_l1_roots[0], &decoded.l2_l1_roots[0]);
    try std.testing.expectEqual(value.public_inputs.end_block_number, decoded.public_inputs.end_block_number);
    try std.testing.expectEqual(value.public_inputs.start_offset, decoded.public_inputs.start_offset);
    try std.testing.expectEqual(value.public_inputs.end_offset, decoded.public_inputs.end_offset);
    try std.testing.expectEqual(value.public_inputs.program_vks.len, decoded.public_inputs.program_vks.len);
    for (value.public_inputs.program_vks, decoded.public_inputs.program_vks) |want, got| {
        try std.testing.expectEqualSlices(u8, &want, &got);
    }
    try std.testing.expectEqualSlices(u8, &value.public_inputs.l2_l1_bridge_transaction_tree, &decoded.public_inputs.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &value.public_inputs.parent_block_hash, &decoded.public_inputs.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &value.public_inputs.end_block_hash, &decoded.public_inputs.end_block_hash);
}

// ── Malformed input ───────────────────────────────────────────────────────────────────────────
// Every case below corrupts bytes `rollup_ssz.encodeInput` itself produced from `sampleInput` — no
// external fixture. Byte positions into the fixed head are content-independent (they are this
// codec's own offset-table layout, not data), so they hold regardless of what `sampleInput`
// contains.

test "decodeInput: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    corrupted[0] = 0x18;
    corrupted[1] = 0x01; // the output schema id, on input bytes
    try std.testing.expectError(error.MalformedFrame, rollup_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects a frame truncated below the 2-byte schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    try std.testing.expectError(error.MalformedFrame, rollup_ssz.decodeInput(alloc, encoded[0..1]));
}

test "decodeInput: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    try std.testing.expectError(error.InvalidSsz, rollup_ssz.decodeInput(alloc, encoded[0 .. 2 + 10]));
}

test "decodeInput: rejects a misaligned first offset (conflations)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    // The conflations offset sits at absolute byte 2 (schema) + 48 (parent_data_rolling_hash(32) +
    // start_offset(8) + chain_id(8)) = 50; the canonical value is the 72-byte fixed head size.
    std.mem.writeInt(u32, corrupted[50..54], 73, .little);
    try std.testing.expectError(error.InvalidSsz, rollup_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects an out-of-order offset pair (l2_execution_proofs region ending before it starts)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    // Force off_prefix (absolute byte 2 + 60) below off_proofs (absolute byte 2 + 56), an
    // out-of-order pair the fixed-head monotonicity guard must reject.
    const off_proofs = std.mem.readInt(u32, corrupted[2 + 56 ..][0..4], .little);
    std.mem.writeInt(u32, corrupted[2 + 60 ..][0..4], off_proofs - 4, .little);
    try std.testing.expectError(error.InvalidSsz, rollup_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: accepts an emptied l2_execution_proofs region (decode succeeds; rollup.run rejects it)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    // Collapse the l2_execution_proofs region to empty by setting the next field's offset
    // (opaque_prefix_bytes, absolute byte 2 + 60) equal to its own start offset (absolute byte
    // 2 + 56) — the region [off_proofs, off_prefix) becomes zero-length.
    const off_proofs = std.mem.readInt(u32, corrupted[2 + 56 ..][0..4], .little);
    std.mem.writeInt(u32, corrupted[2 + 60 ..][0..4], off_proofs, .little);

    const decoded = try rollup_ssz.decodeInput(alloc, corrupted);
    try std.testing.expectEqual(@as(usize, 0), decoded.l2_execution_proofs.len);
}
