//! The public inputs of accepted rollup requests: what each field is derived from, over the DA
//! layouts a stream can take across blob and calldata chunks. The requests are built from real
//! blocks of the EF zkevm fixtures and of Linea mainnet.

const std = @import("std");

const core_serde = @import("core_serde");
const rollup = @import("rollup");
const scenario = @import("scenario");

const rollup_proof = core_serde.rollup_proof;
const rollup_ssz = rollup.rollup_ssz;

const Allocator = std.mem.Allocator;
const Hash = scenario.Hash;
const Address = scenario.Address;
const Fact = scenario.Fact;

fn hashOf(byte: u8) Hash {
    return @splat(byte);
}

fn addressOf(byte: u8) Address {
    return @splat(byte);
}

fn run(a: Allocator, request: scenario.Request) !rollup_proof.RollupPublicInput {
    return rollup.rollup.run(a, request.input());
}

/// The public inputs the blocks of a request determine: the range they span and the hashes at its
/// ends.
fn expectSpanOf(pi: rollup_proof.RollupPublicInput, blocks: []const Fact) !void {
    const last = blocks[blocks.len - 1];
    try std.testing.expectEqual(last.number, pi.end_block_number);
    try std.testing.expectEqual(last.timestamp, pi.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, blocks.len), pi.block_count);
    try std.testing.expectEqualSlices(u8, &blocks[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &last.hash, &pi.end_block_hash);
}

// ── The public inputs of one conflation ──────────────────────────────────────────────────────────

test "one conflation in one blob: the public inputs follow its proof and its stream" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const chain = (try scenario.selection()).chain;
    const conflations = try scenario.chainConflations(a, &.{2});
    const request = try scenario.build(a, conflations, .{});
    const chunk_hash = request.chunks[0].chunk_hash;
    const pi = try run(a, request);

    try std.testing.expectEqual(chain[1].number, pi.end_block_number);
    try std.testing.expectEqual(chain[1].timestamp, pi.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, 2), pi.block_count);
    try std.testing.expectEqualSlices(u8, &chain[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &chain[1].hash, &pi.end_block_hash);

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

    try std.testing.expectEqual(@as(usize, 0), pi.l2_l1_messages.items.len);
    try std.testing.expectEqual(@as(usize, 0), pi.filtered_addresses.items.len);
    try std.testing.expectEqual(@as(usize, 0), pi.l2_messaging_blocks_offsets.items.len);
    try std.testing.expectEqualSlices([32]u8, &.{hashOf(0x2a)}, pi.program_vks.items);
    try std.testing.expectEqualSlices(u8, &hashOf(0), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.end_l1_l2_bridge_rolling_hash_message_number);
}

test "each accepted EF block is accepted as a conflation of its own" {
    const picks = (try scenario.selection()).picks;
    for (picks, 0..) |pick, index| {
        var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer arena.deinit();
        const a = arena.allocator();

        const conflation = try scenario.conflationFrom(a, scenario.ef_chain_id, &.{pick});
        const request = try scenario.build(a, &.{conflation}, .{});
        const pi = run(a, request) catch |err| {
            std.debug.print("EF block {d}: {s}\n", .{ index, @errorName(err) });
            return err;
        };
        try expectSpanOf(pi, &.{pick});
        try std.testing.expectEqualSlices(u8, &scenario.keccak(&(hashOf(0x47) ++ request.chunks[0].chunk_hash)), &pi.end_data_rolling_hash);
    }
}

test "an Osaka block of Linea mainnet is accepted as a conflation of its own" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const block = (try scenario.selection()).linea[0..1];
    const request = try scenario.build(a, &.{try scenario.conflationFrom(a, scenario.linea_chain_id, block)}, .{});
    try expectSpanOf(try run(a, request), block);
}

test "the framed request yields a framed output that decodes to the public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const conflations = try scenario.chainConflations(a, &.{ 2, 1 });
    const request = try scenario.build(a, conflations, .{});

    const frame = try rollup_ssz.encodeInput(a, request.input());
    const pi = try rollup.rollup.run(a, try rollup_ssz.decodeInput(a, frame));
    const output = try rollup_proof.encodeOutput(a, pi);

    try std.testing.expectEqualDeep(pi, try rollup_proof.decodeOutput(a, output));
    try std.testing.expectEqualSlices(u8, &(try rollup_proof.hashPublicInputs(a, pi)), output[0..32]);
    try std.testing.expectEqual(request.end_data_rolling_hash, pi.end_data_rolling_hash);
}

// ── The public inputs of several conflations ─────────────────────────────────────────────────────

const three_conflations = [_]usize{ 2, 1, 3 };

const messages_0 = [_]Hash{ hashOf(0x08), hashOf(0x18) };
const messages_1 = [_]Hash{hashOf(0x28)};
const addresses_0 = [_]Address{ addressOf(0x04), addressOf(0x03) };
const addresses_1 = [_]Address{ addressOf(0x03), addressOf(0x05) };

fn threeProofs() [3]scenario.ProofSpec {
    return .{
        .{
            .program_vk = hashOf(0x2c),
            .l2_l1_messages = &messages_0,
            .filtered_addresses = &addresses_0,
            .bridge_rolling_hash = hashOf(0x03),
            .bridge_message_number = 4,
            .messaging_offsets = &.{ 1, 2 },
        },
        .{
            .program_vk = hashOf(0x2a),
            .l2_l1_messages = &messages_1,
            .filtered_addresses = &addresses_1,
            .messaging_offsets = &.{1},
        },
        .{
            .program_vk = hashOf(0x2c),
            .bridge_rolling_hash = hashOf(0x13),
            .bridge_message_number = 6,
            .messaging_offsets = &.{ 2, 3 },
        },
    };
}

fn threeRequest(a: Allocator, conflations: []const scenario.Conflation, proofs: []const scenario.ProofSpec) !scenario.Request {
    return scenario.build(a, conflations, .{ .proofs = proofs });
}

fn threeConflations(a: Allocator, proofs: []const scenario.ProofSpec) !scenario.Request {
    return threeRequest(a, try scenario.chainConflations(a, &three_conflations), proofs);
}

test "the public inputs of three conflations combine their proofs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const chain = (try scenario.selection()).chain;
    const proofs = threeProofs();
    const conflations = try scenario.chainConflations(a, &three_conflations);
    const pi = try run(a, try threeRequest(a, conflations, &proofs));

    try std.testing.expectEqual(chain[5].number, pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 6), pi.block_count);
    // The chain ends with the last conflation's last block and descends from the first conflation's parent.
    try std.testing.expectEqual(chain[5].timestamp, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &chain[5].hash, &pi.end_block_hash);
    try std.testing.expectEqualSlices(u8, &chain[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(0), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 10), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &scenario.ftxHash(3), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 16), pi.end_processed_ftx_number);

    // The L2 to L1 messages are the proofs' lists end to end.
    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0x08), hashOf(0x18), hashOf(0x28) }, pi.l2_l1_messages.items);
    // The addresses are the distinct ones, ascending.
    try std.testing.expectEqualSlices([20]u8, &.{ addressOf(0x03), addressOf(0x04), addressOf(0x05) }, pi.filtered_addresses.items);
    // The program VKs are the distinct ones, ascending.
    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0x2a), hashOf(0x2c) }, pi.program_vks.items);
    // The bridge pair is the last non-zero one.
    try std.testing.expectEqualSlices(u8, &hashOf(0x13), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 6), pi.end_l1_l2_bridge_rolling_hash_message_number);
    // The offsets count blocks from the first block: the second proof starts at the third block, the
    // third at the fourth.
    const expected_offsets = [_]u16{ 1, 2, 3, 5, 6 };
    try std.testing.expectEqualSlices(u8, std.mem.sliceAsBytes(&expected_offsets), std.mem.sliceAsBytes(pi.l2_messaging_blocks_offsets.items));
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

    const vks = [_]scenario.ProofSpec{ .{ .program_vk = hashOf(0x2c) }, .{ .program_vk = hashOf(0x2a) }, .{ .program_vk = hashOf(0x2c) }, .{ .program_vk = hashOf(0x2b) } };
    const request = try scenario.build(a, try scenario.chainConflations(a, &.{ 1, 1, 1, 1 }), .{ .proofs = &vks });
    const pi = try run(a, request);

    try std.testing.expectEqualSlices([32]u8, &.{ hashOf(0x2a), hashOf(0x2b), hashOf(0x2c) }, pi.program_vks.items);
}

// ── The streams chunks can carry ─────────────────────────────────────────────────────────────────

const Expected = struct {
    /// The bytes of the first blob the range takes; null for a positive count that the layout determines.
    take: ?u64 = 0,
    discard: u64 = 0,
};

const Accepted = struct {
    request: scenario.Request,
    pi: rollup_proof.RollupPublicInput,
};

/// Builds the request of `conflations` cut into `chunks` and checks the stream part of its public inputs.
fn expectStreamOf(a: Allocator, conflations: []const scenario.Conflation, options: scenario.Options, expected: Expected) !Accepted {
    const request = try scenario.build(a, conflations, options);
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
    return .{ .request = request, .pi = pi };
}

/// The same for consecutive conflations of the chain, `counts[i]` blocks in the `i`-th.
fn expectStream(counts: []const usize, chunks: []const scenario.ChunkSpec, expected: Expected) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    _ = try expectStreamOf(a, try scenario.chainConflations(a, counts), .{ .chunks = chunks }, expected);
}

test "the stream of two conflations in one calldata chunk" {
    try expectStream(&.{ 2, 1 }, &.{.{ .kind = .calldata }}, .{});
}

test "the stream of two conflations in two calldata chunks split at a segment boundary" {
    try expectStream(&.{ 2, 1 }, &.{ .{ .kind = .calldata, .own = .{ .through_segment = 1 } }, .{ .kind = .calldata } }, .{});
}

test "a segment that continues from a blob into calldata" {
    try expectStream(&.{ 2, 1 }, &.{ .{ .own = .{ .bytes = 20 } }, .{ .kind = .calldata } }, .{});
}

test "a range inside one shared blob takes the bytes behind its neighbour and discards those before the next" {
    // The blob holds 1000 bytes of the previous proof's stream, the segments, then 500 bytes of the next proof's.
    try expectStream(&.{1}, &.{.{ .prefix = .{ .bytes = 1000 }, .suffix = 500 }}, .{ .take = null, .discard = 500 });
}

// ── The Linea window ─────────────────────────────────────────────────────────────────────────────

const linea_conflations = [_]usize{ 40, 60, 50 };

/// The Linea window as three conflations published as `chunks`: accepted, with the span of all its
/// blocks and the stream part of the public inputs `expected` says.
fn expectLineaStream(options: scenario.Options, expected: Expected) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const conflations = try scenario.lineaConflations(a, &linea_conflations);
    const accepted = try expectStreamOf(a, conflations, options, expected);
    try expectSpanOf(accepted.pi, (try scenario.selection()).linea);
}

test "three conflations of the Linea window in calldata cut at their segment boundaries" {
    try expectLineaStream(.{ .chunks = &.{
        .{ .kind = .calldata, .own = .{ .through_segment = 1 } },
        .{ .kind = .calldata, .own = .{ .through_segment = 2 } },
        .{ .kind = .calldata },
    } }, .{});
}

test "a range starting inside the first of two blobs" {
    // Only the last 30000 bytes of the first full blob belong to the range; the second blob holds
    // the rest of the stream and 70 bytes of the next proof's behind it.
    try expectLineaStream(.{ .chunks = &.{ .{ .own = .{ .bytes = 30_000 }, .prefix = .full }, .{ .suffix = 70 } } }, .{ .take = 30_000, .discard = 70 });
}

test "incompressible blocks fill two blobs and run into a third" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const conflations = try scenario.incompressibleConflations(a);
    const chunks = [_]scenario.ChunkSpec{ .{ .own = .{ .bytes = scenario.blob_capacity } }, .{ .own = .{ .bytes = scenario.blob_capacity } }, .{} };
    const accepted = try expectStreamOf(a, conflations, .{ .chunks = &chunks }, .{});
    try expectSpanOf(accepted.pi, (try scenario.selection()).incompressible);
    var stream: usize = 0;
    for (accepted.request.segments) |segment| stream += segment.len;
    try std.testing.expect(stream > 2 * scenario.blob_capacity);
}
