const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

/// The checked-in golden rollup-input vector, as 0x-prefixed hex text (reviewable in a diff,
/// unlike a raw binary blob) — the same format `make exec`'s ELF->JSON tooling accepts natively
/// for a non-`.ssz`-suffixed `@path` input. Expected values in the tests below are transcribed
/// once from the fixture and are not re-derived at test time; rollup_spec's own golden-byte test
/// is what proves these bytes are still what the canonical encoder produces.
const golden_input_hex = @embedFile("testdata/10-14-getZkRollupProofV1.request.ssz.hex");

fn hexToOwnedBytes(allocator: std.mem.Allocator, hex: []const u8) ![]u8 {
    const stripped = if (hex.len >= 2 and hex[0] == '0' and (hex[1] == 'x' or hex[1] == 'X')) hex[2..] else hex;
    const trimmed = std.mem.trimEnd(u8, stripped, "\r\n");
    if (trimmed.len % 2 != 0) return error.OddHexLength;
    const out = try allocator.alloc(u8, trimmed.len / 2);
    _ = try std.fmt.hexToBytes(out, trimmed);
    return out;
}

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// ── Golden decode ─────────────────────────────────────────────────────────────────────────────

test "decodeInput: golden vector decodes to its transcribed field values" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const v = try rollup_ssz.decodeInput(alloc, golden_input);

    try std.testing.expectEqualSlices(u8, &repeat32(0x47), &v.parent_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), v.start_offset);
    try std.testing.expectEqual(@as(u64, 59144), v.chain_id);

    try std.testing.expectEqual(@as(usize, 2), v.conflations.len);
    try std.testing.expectEqual(@as(usize, 2), v.conflations[0].block_rlps.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xf9, 0x02, 0x15, 0xa0 }, v.conflations[0].block_rlps[0]);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xf9, 0x02, 0x16, 0xb1 }, v.conflations[0].block_rlps[1]);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xf9, 0x02, 0x15, 0xaa }, v.conflations[1].block_rlps[0]);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xf9, 0x02, 0x16, 0xbb }, v.conflations[1].block_rlps[1]);

    try std.testing.expectEqual(@as(usize, 1), v.chunks.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0x1a), &v.chunks[0]);

    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xab, 0xab, 0xab, 0xab }, v.opaque_prefix_bytes);
    try std.testing.expectEqual(@as(usize, 0), v.opaque_suffix_bytes.len);
    try std.testing.expect(v.boundary_prev_data_rolling_hash != null);
    try std.testing.expectEqualSlices(u8, &repeat32(0x39), &v.boundary_prev_data_rolling_hash.?);

    try std.testing.expectEqual(@as(usize, 2), v.l2_execution_proofs.len);

    const p0 = v.l2_execution_proofs[0];
    try std.testing.expectEqualSlices(u8, &repeat32(0xaa), &p0.program_vk);
    try std.testing.expectEqual(@as(u64, 10), p0.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xab, 0xcd, 0xef }, p0.proof.proof);
    try std.testing.expectEqual(@as(u64, 11), p0.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000200), p0.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0a), &p0.proof.public_inputs.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0b), &p0.proof.public_inputs.end_block_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x02), &p0.proof.public_inputs.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), p0.proof.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x03), &p0.proof.public_inputs.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), p0.proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0xc0), &p0.proof.public_inputs.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x04), &p0.proof.public_inputs.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 15), p0.proof.public_inputs.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x05), &p0.proof.public_inputs.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 18), p0.proof.public_inputs.end_processed_ftx_number);
    try std.testing.expectEqual(@as(usize, 1), p0.proof.l2_l1_messages.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0x08), &p0.proof.l2_l1_messages[0]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.tx_froms.len);
    try std.testing.expectEqualSlices(u8, &repeat20(0x01), &p0.proof.tx_froms[0]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x02), &p0.proof.tx_froms[1]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &repeat20(0x03), &p0.proof.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x04), &p0.proof.filtered_addresses[1]);

    const p1 = v.l2_execution_proofs[1];
    try std.testing.expectEqualSlices(u8, &repeat32(0xaa), &p1.program_vk);
    try std.testing.expectEqual(@as(u64, 12), p1.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xab, 0xcd, 0xff }, p1.proof.proof);
    try std.testing.expectEqual(@as(u64, 14), p1.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000210), p1.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, 18), p1.proof.public_inputs.parent_ftx_number);
    try std.testing.expectEqual(@as(u64, 18), p1.proof.public_inputs.end_processed_ftx_number);
}

// ── Output frame: encode/decode round-trip ───────────────────────────────────────────────────────

fn sampleOutput(alloc: std.mem.Allocator) !rollup_ssz.RollupOutput {
    const program_vks = try alloc.dupe([32]u8, &[_][32]u8{ repeat32(0xaa), repeat32(0xbb) });
    const l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{repeat32(0x45)});
    const filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ repeat20(0x03), repeat20(0x04) });

    return .{
        .public_inputs = .{
            .end_block_number = 14,
            .end_block_timestamp = 1763000210,
            .l2_l1_bridge_transaction_tree = repeat32(0xbc),
            .parent_l1_l2_bridge_rolling_hash = repeat32(0x02),
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
            .end_l1_l2_bridge_rolling_hash_message_number = 4,
            .dynamic_chain_config_hash = repeat32(0xc0),
            .parent_ftx_rolling_hash = repeat32(0x04),
            .parent_ftx_number = 15,
            .end_ftx_rolling_hash = repeat32(0x05),
            .end_processed_ftx_number = 18,
            .filtered_addresses_hash = repeat32(0x8f),
            .parent_data_rolling_hash = repeat32(0x47),
            .end_data_rolling_hash = repeat32(0x1f),
            .parent_block_hash = repeat32(0x0a),
            .end_block_hash = repeat32(0x0b),
            .start_offset = 4,
            .end_offset = 0x1ab5956f53caf2ea,
            .program_vks = program_vks,
        },
        .start_block_number = 10,
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

test "decodeInput: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
    corrupted[0] = 0x18;
    corrupted[1] = 0x01; // the output schema id, on input bytes
    try std.testing.expectError(error.MalformedFrame, rollup_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects a frame truncated below the 2-byte schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    try std.testing.expectError(error.MalformedFrame, rollup_ssz.decodeInput(alloc, golden_input[0..1]));
}

test "decodeInput: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    try std.testing.expectError(error.InvalidSsz, rollup_ssz.decodeInput(alloc, golden_input[0 .. 2 + 10]));
}

test "decodeInput: rejects a misaligned first offset (conflations)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
    // The conflations offset sits at absolute byte 2 (schema) + 48 (parent_data_rolling_hash(32) +
    // start_offset(8) + chain_id(8)) = 50; the canonical value is the 72-byte fixed head size.
    std.mem.writeInt(u32, corrupted[50..54], 73, .little);
    try std.testing.expectError(error.InvalidSsz, rollup_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects an out-of-order offset pair (l2_execution_proofs region ending before it starts)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
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

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
    // Collapse the l2_execution_proofs region to empty by setting the next field's offset
    // (opaque_prefix_bytes, absolute byte 2 + 60) equal to its own start offset (absolute byte
    // 2 + 56) — the region [off_proofs, off_prefix) becomes zero-length.
    const off_proofs = std.mem.readInt(u32, corrupted[2 + 56 ..][0..4], .little);
    std.mem.writeInt(u32, corrupted[2 + 60 ..][0..4], off_proofs, .little);

    const decoded = try rollup_ssz.decodeInput(alloc, corrupted);
    try std.testing.expectEqual(@as(usize, 0), decoded.l2_execution_proofs.len);
}
