const std = @import("std");
const rollup_ssz = @import("rollup").rollup_ssz;
const rollup = @import("rollup").rollup;
const core_serde = @import("core_serde");
const support = @import("support");
const repeat32 = support.repeat32;
const repeat20 = support.repeat20;
const sampleInput = support.sampleInput;

const l2_execution_proof = core_serde.l2_execution_proof;
const rollup_proof = core_serde.rollup_proof;

// ── Echo semantics ────────────────────────────────────────────────────────────────────────────
// `sampleInput` has 2 l2_execution_proofs, both carrying the SAME program_vk (`SHARED_PROGRAM_VK`)
// — so the output's `program_vks` collapses to a single element, which is itself a useful
// assertion (dedup actually ran) distinct from just checking membership.

test "run: copied fields echo the mapped source (first/last l2_execution_proofs, input)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try sampleInput(alloc);
    const pi = try rollup.run(alloc, input);

    // last (proof1) sourced fields.
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_NUMBER, pi.end_block_number);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_TIMESTAMP, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_FTX_ROLLING_HASH, &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF1_END_PROCESSED_FTX_NUMBER, pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_BLOCK_HASH, &pi.end_block_hash);

    // End rolling-hash pair: proof1 is the last child with a non-zero message number.
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_L1L2_BRIDGE_ROLLING_HASH, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.end_l1_l2_bridge_rolling_hash_message_number);

    // first (proof0) sourced fields.
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_FTX_ROLLING_HASH, &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_FTX_NUMBER, pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_DYNAMIC_CHAIN_CONFIG_HASH, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_BLOCK_HASH, &pi.parent_block_hash);

    // Top-level input fields.
    try std.testing.expectEqualSlices(u8, &support.PARENT_DATA_ROLLING_HASH, &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(support.PARENT_DATA_TAIL_TAKE_BYTES, pi.parent_data_tail_take_bytes);

    // program_vks: both proofs share SHARED_PROGRAM_VK -> deduplicated to exactly one element.
    try std.testing.expectEqual(@as(usize, 1), pi.program_vks.len);
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &pi.program_vks[0]);

    // block_count: the proofs' counts add up.
    try std.testing.expectEqual(support.PROOF0_BLOCK_COUNT + support.PROOF1_BLOCK_COUNT, pi.block_count);
}

test "run: l2_l1_messages are every proof's messages in proof order" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const pi = try rollup.run(alloc, try sampleInput(alloc));
    try std.testing.expectEqualSlices([32]u8, &[_][32]u8{
        support.PROOF0_L2_L1_MESSAGE_0,
        support.PROOF0_L2_L1_MESSAGE_1,
        support.PROOF1_L2_L1_MESSAGE_0,
    }, pi.l2_l1_messages);
}

test "run: filtered_addresses are the distinct addresses of every proof, sorted ascending" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // proof0 lists 0x04, 0x03 and proof1 lists 0x03, 0x05.
    const pi = try rollup.run(alloc, try sampleInput(alloc));
    try std.testing.expectEqualSlices([20]u8, &[_][20]u8{ repeat20(0x03), repeat20(0x04), repeat20(0x05) }, pi.filtered_addresses);
}

// ── End rolling-hash pair: sourced from the last child proof with a non-zero message number ─────

test "run: a trailing child without anchoring keeps the earlier child's end rolling-hash pair" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var input = try sampleInput(alloc);
    var proofs = [_]l2_execution_proof.VerifiableL2ExecutionProof{
        input.l2_execution_proofs[0],
        input.l2_execution_proofs[1],
    };
    proofs[1].proof.public_inputs.end_l1_l2_bridge_rolling_hash = repeat32(0);
    proofs[1].proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number = 0;
    input.l2_execution_proofs = &proofs;

    const pi = try rollup.run(alloc, input);

    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "run: children without anchoring give a zero end rolling-hash pair" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var input = try sampleInput(alloc);
    var proofs = [_]l2_execution_proof.VerifiableL2ExecutionProof{
        input.l2_execution_proofs[0],
        input.l2_execution_proofs[1],
    };
    for (&proofs) |*p| {
        p.proof.public_inputs.end_l1_l2_bridge_rolling_hash = repeat32(0);
        p.proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number = 0;
    }
    input.l2_execution_proofs = &proofs;

    const pi = try rollup.run(alloc, input);

    try std.testing.expectEqualSlices(u8, &repeat32(0), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "run: output program_vks are distinct and sorted for unsorted duplicate inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var input = try sampleInput(alloc);
    var proofs = [_]l2_execution_proof.VerifiableL2ExecutionProof{
        input.l2_execution_proofs[0],
        input.l2_execution_proofs[1],
        input.l2_execution_proofs[0],
        input.l2_execution_proofs[1],
    };
    proofs[0].program_vk = repeat32(0xcc);
    proofs[1].program_vk = repeat32(0xaa);
    proofs[2].program_vk = repeat32(0xcc);
    proofs[3].program_vk = repeat32(0xbb);
    input.l2_execution_proofs = &proofs;

    const pi = try rollup.run(alloc, input);
    try std.testing.expectEqualSlices([32]u8, &[_][32]u8{
        repeat32(0xaa),
        repeat32(0xbb),
        repeat32(0xcc),
    }, pi.program_vks);
}

test "run: rejects more distinct filtered addresses than the output bound" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var input = try sampleInput(alloc);
    const max = rollup_proof.MAX_FILTERED_ADDRESSES;
    const addresses = try alloc.alloc([20]u8, max + 1);
    for (addresses, 0..) |*address, i| {
        address.* = repeat20(0);
        std.mem.writeInt(u32, address[0..4], @intCast(i), .big);
    }
    var proofs = [_]l2_execution_proof.VerifiableL2ExecutionProof{
        input.l2_execution_proofs[0],
        input.l2_execution_proofs[1],
    };
    proofs[0].proof.public_inputs.filtered_addresses = addresses[0..max];
    proofs[1].proof.public_inputs.filtered_addresses = addresses[max..];
    input.l2_execution_proofs = &proofs;

    try std.testing.expectError(error.BoundsViolation, rollup.run(alloc, input));
}

test "run: sentinel fields equal their pinned constants exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const pi = try rollup.run(alloc, try sampleInput(alloc));

    try std.testing.expectEqualSlices(u8, &rollup.END_DATA_ROLLING_HASH, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(rollup.FINAL_DATA_TAIL_DISCARD_BYTES, pi.final_data_tail_discard_bytes);
    try std.testing.expectEqualSlices(u16, &[_]u16{rollup.L2_MESSAGING_BLOCKS_OFFSET}, pi.l2_messaging_blocks_offsets);

    // Exact pinned values, transcribed independently of `rollup.zig`'s own constants — catches a wrong
    // constant definition (e.g. hashing the wrong tag, or a wrong algorithm), not just a copy/paste
    // of the same bug into both the constant and this assertion.
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x1f, 0xe6, 0x17, 0xc1, 0x0b, 0x3b, 0xfc, 0x97, 0xfd, 0x6d, 0x00, 0x90, 0xc6, 0x08, 0xdf, 0x47,
        0xde, 0x54, 0x4a, 0x4b, 0x7d, 0x4f, 0x63, 0x79, 0x30, 0x0b, 0xd9, 0x61, 0x67, 0xda, 0x2a, 0xda,
    }, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0xe1d65aa159295514), pi.final_data_tail_discard_bytes);
    try std.testing.expectEqual(@as(u16, 0x73bd), pi.l2_messaging_blocks_offsets[0]);
}

// ── Malformed input: empty proofs list ───────────────────────────────────────────────────────────

test "run: rejects an empty l2_execution_proofs list" {
    const input = rollup_ssz.RollupProofPrivateInput{
        .parent_data_rolling_hash = repeat32(0),
        .parent_data_tail_take_bytes = 0,
        .chain_id = 0,
        .conflations = &.{},
        .chunks = &.{},
        .l2_execution_proofs = &.{},
        .boundary_prev_data_rolling_hash = null,
    };
    try std.testing.expectError(error.EmptyProofs, rollup.run(std.testing.allocator, input));
}

// ── Guest pipeline: framed input to framed output ──────────────────────────────────────────────

test "the framed sample input yields a framed output that decodes to the mapped public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const frame = try rollup_ssz.encodeInput(alloc, try sampleInput(alloc));
    const pi = try rollup.run(alloc, try rollup_ssz.decodeInput(alloc, frame));
    const output = try rollup_proof.encodeOutput(alloc, pi);

    try std.testing.expectEqualDeep(pi, try rollup_proof.decodeOutput(alloc, output));
    try std.testing.expectEqualSlices(u8, &(try rollup_proof.hashPublicInputs(alloc, pi)), output[0..32]);
}
