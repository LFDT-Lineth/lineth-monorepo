//! The public inputs of accepted rollup requests: what each field is derived from, over the DA
//! layouts a stream can take across blob and calldata chunks.

const std = @import("std");

const core_serde = @import("core_serde");
const rollup = @import("rollup");
const scenario = @import("scenario");

const rollup_proof = core_serde.rollup_proof;
const rollup_ssz = rollup.rollup_ssz;

const Allocator = std.mem.Allocator;
const Hash = scenario.Hash;
const Address = scenario.Address;

fn hashOf(byte: u8) Hash {
    return @splat(byte);
}

fn addressOf(byte: u8) Address {
    return @splat(byte);
}

fn run(a: Allocator, request: scenario.Request) !rollup_proof.RollupPublicInput {
    return rollup.rollup.run(a, request.input());
}

// ── The public inputs of one conflation ──────────────────────────────────────────────────────────

test "one conflation in one blob: the public inputs follow its proof and its stream" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const conflations = try scenario.conflations(a, .{}, &.{.{ .blocks = 2 }});
    const request = try scenario.build(a, conflations, .{});
    const chunk_hash = request.chunks[0].chunk_hash;
    const pi = try run(a, request);

    // Blocks 100 and 101, timestamps 12 seconds apart from 1_700_000_000.
    try std.testing.expectEqual(@as(u64, 101), pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 1_700_001_212), pi.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, 2), pi.block_count);
    try std.testing.expectEqualSlices(u8, &hashOf(0x01), &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &conflations[0].block_hashes[1], &pi.end_block_hash);

    // The proof's forced-transaction state and configuration.
    try std.testing.expectEqualSlices(u8, &hashOf(0xc0), &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 10), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(1), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 12), pi.end_processed_ftx_number);

    // The stream: one chunk folded onto the parent dataRollingHash, nothing left behind it.
    try std.testing.expectEqualSlices(u8, &hashOf(0x47), &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.parent_data_tail_take_bytes);
    try std.testing.expectEqualSlices(u8, &scenario.keccak(&(hashOf(0x47) ++ chunk_hash)), &pi.end_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.final_data_tail_discard_bytes);

    try std.testing.expectEqual(@as(usize, 0), pi.l2_l1_messages.len);
    try std.testing.expectEqual(@as(usize, 0), pi.filtered_addresses.len);
    try std.testing.expectEqual(@as(usize, 0), pi.l2_messaging_blocks_offsets.len);
    try std.testing.expectEqualSlices([32]u8, &.{hashOf(0xaa)}, pi.program_vks);
    try std.testing.expectEqualSlices(u8, &hashOf(0), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "the framed request yields a framed output that decodes to the public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const conflations = try scenario.conflations(a, .{}, &.{ .{ .blocks = 2 }, .{} });
    const request = try scenario.build(a, conflations, .{});

    const frame = try rollup_ssz.encodeInput(a, request.input());
    const pi = try rollup.rollup.run(a, try rollup_ssz.decodeInput(a, frame));
    const output = try rollup_proof.encodeOutput(a, pi);

    try std.testing.expectEqualDeep(pi, try rollup_proof.decodeOutput(a, output));
    try std.testing.expectEqualSlices(u8, &(try rollup_proof.hashPublicInputs(a, pi)), output[0..32]);
    try std.testing.expectEqual(request.end_data_rolling_hash, pi.end_data_rolling_hash);
}

// ── The public inputs of several conflations ─────────────────────────────────────────────────────

const three_conflations = [_]scenario.ConflationSpec{ .{ .blocks = 2 }, .{ .blocks = 1 }, .{ .blocks = 3 } };

const messages_0 = [_]Hash{ hashOf(0x08), hashOf(0x18) };
const messages_1 = [_]Hash{hashOf(0x28)};
const addresses_0 = [_]Address{ addressOf(0x04), addressOf(0x03) };
const addresses_1 = [_]Address{ addressOf(0x03), addressOf(0x05) };

fn threeProofs() [3]scenario.ProofSpec {
    return .{
        .{
            .program_vk = hashOf(0xcc),
            .l2_l1_messages = &messages_0,
            .filtered_addresses = &addresses_0,
            .bridge_rolling_hash = hashOf(0x03),
            .bridge_message_number = 4,
            .messaging_offsets = &.{ 1, 2 },
        },
        .{
            .program_vk = hashOf(0xaa),
            .l2_l1_messages = &messages_1,
            .filtered_addresses = &addresses_1,
            .messaging_offsets = &.{1},
        },
        .{
            .program_vk = hashOf(0xcc),
            .bridge_rolling_hash = hashOf(0x13),
            .bridge_message_number = 6,
            .messaging_offsets = &.{ 2, 3 },
        },
    };
}

fn threeConflations(a: Allocator, proofs: []const scenario.ProofSpec) !scenario.Request {
    return scenario.build(a, try scenario.conflations(a, .{}, &three_conflations), .{ .proofs = proofs });
}

test "the public inputs of three conflations combine their proofs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = threeProofs();
    const pi = try run(a, try threeConflations(a, &proofs));

    // Blocks 100 to 105.
    try std.testing.expectEqual(@as(u64, 105), pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 6), pi.block_count);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 10), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(3), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 16), pi.end_processed_ftx_number);

    // The L2 to L1 messages are the proofs' lists end to end.
    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0x08), hashOf(0x18), hashOf(0x28) }, pi.l2_l1_messages);
    // The addresses are the distinct ones, ascending.
    try std.testing.expectEqualSlices([20]u8, &.{ addressOf(0x03), addressOf(0x04), addressOf(0x05) }, pi.filtered_addresses);
    // The program VKs are the distinct ones, ascending.
    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0xaa), hashOf(0xcc) }, pi.program_vks);
    // The bridge pair is the last non-zero one.
    try std.testing.expectEqualSlices(u8, &hashOf(0x13), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 6), pi.end_l1_l2_bridge_rolling_hash_message_number);
    // The offsets count blocks from block 100: the second proof starts at block 102, the third at 103.
    try std.testing.expectEqualSlices(u16, &.{ 1, 2, 3, 5, 6 }, pi.l2_messaging_blocks_offsets);
}

test "trailing proofs without a bridge anchor keep the earlier proof's pair" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var proofs = threeProofs();
    proofs[2].bridge_rolling_hash = hashOf(0);
    proofs[2].bridge_message_number = 0;
    const pi = try run(a, try threeConflations(a, &proofs));

    try std.testing.expectEqualSlices(u8, &hashOf(0x03), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "a hash without a message number is not a bridge anchor" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var proofs = threeProofs();
    proofs[2].bridge_rolling_hash = hashOf(0x13);
    proofs[2].bridge_message_number = 0;
    const pi = try run(a, try threeConflations(a, &proofs));

    try std.testing.expectEqualSlices(u8, &hashOf(0x03), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "a message number without a hash is a bridge anchor" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var proofs = threeProofs();
    proofs[2].bridge_rolling_hash = hashOf(0);
    proofs[2].bridge_message_number = 6;
    const pi = try run(a, try threeConflations(a, &proofs));

    try std.testing.expectEqualSlices(u8, &hashOf(0), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 6), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "proofs without a bridge anchor give a zero pair" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var proofs = threeProofs();
    for (&proofs) |*proof| {
        proof.bridge_rolling_hash = hashOf(0);
        proof.bridge_message_number = 0;
    }
    const pi = try run(a, try threeConflations(a, &proofs));

    try std.testing.expectEqualSlices(u8, &hashOf(0), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "the program VKs of many proofs are distinct and ascending whatever their order" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const specs = [_]scenario.ConflationSpec{ .{}, .{}, .{}, .{} };
    const vks = [_]scenario.ProofSpec{ .{ .program_vk = hashOf(0xcc) }, .{ .program_vk = hashOf(0xaa) }, .{ .program_vk = hashOf(0xcc) }, .{ .program_vk = hashOf(0xbb) } };
    const request = try scenario.build(a, try scenario.conflations(a, .{}, &specs), .{ .proofs = &vks });
    const pi = try run(a, request);

    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0xaa), hashOf(0xbb), hashOf(0xcc) }, pi.program_vks);
}

// ── The streams chunks can carry ─────────────────────────────────────────────────────────────────

const Expected = struct {
    /// The bytes of the first blob the range takes; null for a positive count that the layout determines.
    take: ?u64 = 0,
    discard: u64 = 0,
};

/// Builds the request of `specs` cut into `chunks` and checks the stream part of its public inputs.
fn expectStream(specs: []const scenario.ConflationSpec, chunks: []const scenario.ChunkSpec, expected: Expected) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try scenario.build(a, try scenario.conflations(a, .{}, specs), .{ .chunks = chunks });
    const pi = try run(a, request);

    if (expected.take) |take| {
        try std.testing.expectEqual(take, pi.parent_data_tail_take_bytes);
    } else {
        try std.testing.expect(pi.parent_data_tail_take_bytes > 0);
    }
    try std.testing.expectEqual(request.parent_data_tail_take_bytes, pi.parent_data_tail_take_bytes);
    try std.testing.expectEqual(expected.discard, pi.final_data_tail_discard_bytes);
    try std.testing.expectEqualSlices(u8, &request.parent_data_rolling_hash, &pi.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &request.end_data_rolling_hash, &pi.end_data_rolling_hash);
}

test "the stream of two conflations in one calldata chunk" {
    try expectStream(&.{ .{ .blocks = 2 }, .{} }, &.{.{ .kind = .calldata }}, .{});
}

test "the stream of two conflations in two calldata chunks split at a segment boundary" {
    try expectStream(&.{ .{ .blocks = 2 }, .{} }, &.{ .{ .kind = .calldata, .own = .{ .through_segment = 1 } }, .{ .kind = .calldata } }, .{});
}

test "a segment that continues from a blob into calldata" {
    try expectStream(&.{ .{ .blocks = 2 }, .{} }, &.{ .{ .own = .{ .bytes = 20 } }, .{ .kind = .calldata } }, .{});
}

test "a stream across two full blobs" {
    // The frame of 140000 random bytes does not fit one blob, whose payload is 130047 bytes.
    try expectStream(&.{.{ .bulk_bytes = 140_000 }}, &.{ .{ .own = .{ .bytes = scenario.blob_capacity } }, .{} }, .{});
}

test "a range inside one shared blob takes the bytes behind its neighbour and discards those before the next" {
    // The blob holds 1000 bytes of the previous proof's stream, the segments, then 500 bytes of the next proof's.
    try expectStream(&.{.{}}, &.{.{ .prefix = .{ .bytes = 1000 }, .suffix = 500 }}, .{ .take = null, .discard = 500 });
}

test "a range starting inside the first of two blobs" {
    // Only the last 20000 bytes of the first full blob belong to the range.
    try expectStream(&.{.{ .bulk_bytes = 140_000 }}, &.{ .{ .own = .{ .bytes = 20_000 }, .prefix = .full }, .{ .suffix = 70 } }, .{ .take = 20_000, .discard = 70 });
}
