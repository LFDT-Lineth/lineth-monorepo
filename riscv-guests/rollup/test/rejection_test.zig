//! The requests the rollup guest rejects: every rule of the statement broken in an otherwise valid
//! request, with the error it is rejected by and the exit-code category of that error; and the
//! requests at the edge of an output bound, which it accepts.

const std = @import("std");

const core_serde = @import("core_serde");
const rollup = @import("rollup");
const scenario = @import("scenario");

const guest_errors = rollup.guest_errors;
const l2_execution_proof = core_serde.l2_execution_proof;
const messaging_offsets = rollup.messaging_offsets;
const rollup_ssz = rollup.rollup_ssz;

const Address = scenario.Address;
const Allocator = std.mem.Allocator;
const Request = scenario.Request;

/// Runs `request` and expects it rejected by `expected`, which belongs to the category `code`.
fn expectRejected(expected: anyerror, code: guest_errors.ExitCode, a: Allocator, request: Request) !void {
    try std.testing.expectError(expected, rollup.rollup.run(a, request.input()));
    try std.testing.expectEqual(code, guest_errors.exitCode(expected));
}

/// A request of conflations of `blocks` blocks each, in one blob.
fn requestOf(a: Allocator, blocks: []const usize, options: scenario.Options) !Request {
    const specs = try a.alloc(scenario.ConflationSpec, blocks.len);
    for (specs, blocks) |*spec, count| spec.* = .{ .blocks = count };
    return scenario.build(a, try scenario.conflations(a, .{}, specs), options);
}

fn flip(bytes: []const u8, index: usize) void {
    @constCast(bytes)[index] ^= 0x01;
}

// ── The shape of the request ─────────────────────────────────────────────────────────────────────

test "a request without conflations or proofs is malformed" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.conflations = &.{};
    request.proofs = &.{};
    try expectRejected(error.InvalidRequest, guest_errors.CODE_MALFORMED_REQUEST, a, request);
}

test "conflations that do not pair with the proofs one to one are malformed" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs = request.proofs[0..1];
    try expectRejected(error.InvalidRequest, guest_errors.CODE_MALFORMED_REQUEST, a, request);
}

// ── The blocks of a conflation (steps 1 and 3) ───────────────────────────────────────────────────

test "a conflation with more blocks than its proof's range is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[0].proof.public_inputs.end_block_number -= 1;
    try expectRejected(error.BlockCountMismatch, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a conflation with fewer blocks than its proof's range is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.public_inputs.end_block_number += 1;
    try expectRejected(error.BlockCountMismatch, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a conflation without blocks is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.conflations[0].block_rlps = &.{};
    try expectRejected(error.BlockCountMismatch, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a conflation without blocks is rejected even when its proof's range is empty" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.conflations[0].block_rlps = &.{};
    request.proofs[0].proof.public_inputs.end_block_number = request.proofs[0].proof.start_block_number - 1;
    try expectRejected(error.BlockCountMismatch, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "senders that do not hash to the proof's txFromsHash are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs[1].proof.public_inputs.tx_froms_hash[5] ^= 0x01;
    try expectRejected(error.SendersHashMismatch, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a block RLP that is not canonical is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{2}, .{});
    // The last byte of a block is its empty withdrawals list; 0xc1 announces a byte that is missing.
    const block_rlp = try a.dupe(u8, request.conflations[0].block_rlps[1]);
    block_rlp[block_rlp.len - 1] = 0xc1;
    const block_rlps = try a.dupe([]const u8, request.conflations[0].block_rlps);
    block_rlps[1] = block_rlp;
    request.conflations[0].block_rlps = block_rlps;
    try expectRejected(error.MalformedBlock, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a chain ID of zero is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.chain_id = 0;
    try expectRejected(error.SenderRecoveryFailed, guest_errors.CODE_BLOCK_BINDING, a, request);
}

test "a chain ID other than the one the legacy transactions were signed under is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // An EIP-155 signature commits to the chain ID.
    var request = try scenario.build(a, try scenario.conflations(a, .{ .kinds = &.{.legacy} }, &.{.{ .blocks = 2 }}), .{});
    request.chain_id += 1;
    try expectRejected(error.SenderRecoveryFailed, guest_errors.CODE_BLOCK_BINDING, a, request);
}

// ── The chunks and their blobs (step 2) ──────────────────────────────────────────────────────────

test "a request without chunks is malformed" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.chunks = &.{};
    try expectRejected(error.NoChunks, guest_errors.CODE_MALFORMED_REQUEST, a, request);
}

test "a blob that does not hash to its chunk hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.chunks[0].chunk_hash[0] ^= 0x01;
    try expectRejected(error.ChunkHashMismatch, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "calldata that does not hash to its chunk hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{ .chunks = &.{.{ .kind = .calldata }} });
    flip(request.chunks[0].calldata_bytes, 0);
    try expectRejected(error.ChunkHashMismatch, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "a calldata chunk that carries a commitment is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{ .chunks = &.{.{ .kind = .calldata }} });
    request.chunks[0].kzg_commitment = &(@as([48]u8, @splat(0)));
    try expectRejected(error.MalformedChunk, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "a blob with an element of 2^254 or more is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{});
    @constCast(request.chunks[0].blob_bytes)[0] |= 0x40;
    try expectRejected(error.NonCanonicalBlob, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "a range inside a blob without the opening of its parent hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{ .chunks = &.{.{ .prefix = .{ .bytes = 1000 } }} });
    request.boundary_prev_data_rolling_hash = null;
    try expectRejected(error.BoundaryPreimageMismatch, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "a range inside a blob with another opening of its parent hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{ .chunks = &.{.{ .prefix = .{ .bytes = 1000 } }} });
    request.boundary_prev_data_rolling_hash.?[0] ^= 0x01;
    try expectRejected(error.BoundaryPreimageMismatch, guest_errors.CODE_CHUNK_BINDING, a, request);
}

test "a blob proof that does not open the commitment is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{});
    flip(request.chunks[0].kzg_proof, 47);
    try expectRejected(error.BlobKzgMismatch, guest_errors.CODE_BLOB_KZG, a, request);
}

test "a blob that is not the one its commitment commits to is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{});
    // The low bit of the first element changes the blob and keeps it a canonical packing.
    flip(request.chunks[0].blob_bytes, 31);
    try expectRejected(error.BlobKzgMismatch, guest_errors.CODE_BLOB_KZG, a, request);
}

// ── The layout of the stream (step 2) ────────────────────────────────────────────────────────────

test "a blob that is not full before another blob is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{ .chunks = &.{ .{ .own = .{ .bytes = 20 } }, .{} } });
    try expectRejected(error.PartialBlobBeforeBlob, guest_errors.CODE_STREAM_LAYOUT, a, request);
}

test "a calldata chunk that ends inside a segment is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 1, 1 }, .{ .chunks = &.{ .{ .kind = .calldata, .own = .{ .bytes = 20 } }, .{ .kind = .calldata } } });
    try expectRejected(error.CalldataEndsMidSegment, guest_errors.CODE_STREAM_LAYOUT, a, request);
}

test "a stream with fewer segments than conflations is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 1, 1 }, .{ .chunks = &.{.{ .own = .{ .through_segment = 1 } }} });
    try expectRejected(error.MissingSegmentLength, guest_errors.CODE_STREAM_LAYOUT, a, request);
}

test "a tail count that reaches the whole payload of the first blob is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{1}, .{});
    request.parent_data_tail_take_bytes = scenario.blob_capacity;
    try expectRejected(error.InvalidTailTake, guest_errors.CODE_STREAM_LAYOUT, a, request);
}

// ── The decompression of the segments (step 2) ───────────────────────────────────────────────────

test "a frame that decompresses to another payload is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 1, 1 }, .{ .segment_faults = &.{ .none, .other_payload } });
    try expectRejected(error.PayloadMismatch, guest_errors.CODE_DECOMPRESSION, a, request);
}

test "a segment that is not a zstd frame is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{1}, .{ .segment_faults = &.{.not_a_frame} });
    try expectRejected(error.InvalidFrame, guest_errors.CODE_DECOMPRESSION, a, request);
}

// ── The proofs as a chain ────────────────────────────────────────────────────────────────────────

test "proofs that leave a gap between their ranges are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.start_block_number += 1;
    request.proofs[1].proof.public_inputs.end_block_number += 1;
    try expectRejected(error.TilingBroken, guest_errors.CODE_TILING_CONTINUITY, a, request);
}

test "a proof whose end block hash is not the hash of its last block is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[0].proof.public_inputs.end_block_hash[0] ^= 0x01;
    try expectRejected(error.EndBlockHashMismatch, guest_errors.CODE_BLOCK_HASH_CHAIN, a, request);
}

test "a first block that does not descend from the first proof's parent block hash is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[0].proof.public_inputs.parent_block_hash[0] ^= 0x01;
    try expectRejected(error.ParentHashMismatch, guest_errors.CODE_BLOCK_HASH_CHAIN, a, request);
}

test "a block that does not descend from the block before it is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // Every proof, hash and segment agrees with the blocks, whose second one has another parent.
    const specs = [_]scenario.ConflationSpec{ .{ .blocks = 3, .detached_block = 1 }, .{} };
    const request = try scenario.build(a, try scenario.conflations(a, .{}, &specs), .{});
    try expectRejected(error.ParentHashMismatch, guest_errors.CODE_BLOCK_HASH_CHAIN, a, request);
}

test "a proof whose public input is beyond a bound of its wire schema is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const offsets = try a.alloc(u16, l2_execution_proof.MAX_MESSAGING_BLOCKS + 1);
    @memset(offsets, 1);
    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs[0].proof.public_inputs.l2_messaging_blocks_offsets = offsets;
    try expectRejected(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, request);
}

test "a proof that does not start from the block hash its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.public_inputs.parent_block_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, request);
}

test "proofs with different chain configurations are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.public_inputs.dynamic_chain_config_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, request);
}

test "a proof that does not start from the forced-transaction rolling hash its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.public_inputs.parent_ftx_rolling_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, request);
}

test "a proof that does not start from the processed forced-transaction number its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[1].proof.public_inputs.parent_ftx_number += 1;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, request);
}

// ── The messaging-block offsets ──────────────────────────────────────────────────────────────────

test "a proof whose block count differs from its range is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    var request = try requestOf(a, &.{ 2, 1 }, .{});
    request.proofs[0].proof.public_inputs.block_count = 3;
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, request);
}

test "offsets that do not increase are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 3, 1 }, .{ .proofs = &.{.{ .messaging_offsets = &.{ 2, 2 } }} });
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, request);
}

test "an offset of zero is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 3, 1 }, .{ .proofs = &.{.{ .messaging_offsets = &.{ 0, 1 } }} });
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, request);
}

test "an offset beyond the proof's blocks is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try requestOf(a, &.{ 3, 1 }, .{ .proofs = &.{.{ .messaging_offsets = &.{ 1, 4 } }} });
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, request);
}

test "an offset rebases to 65535 but not to 65536" {
    var pi = std.mem.zeroes(l2_execution_proof.L2ExecutionProofPublicInput);
    pi.end_block_number = 65_536;
    pi.block_count = 2;
    pi.l2_messaging_blocks_offsets = &.{1};
    // The proof starts at block 65535, which is 65534 blocks after the first block of the rollup proof.
    var proof: l2_execution_proof.L2ExecutionProof = .{ .public_inputs = pi, .start_block_number = 65_535, .proof = &.{} };

    var out: [2]u16 = undefined;
    try messaging_offsets.rebase(proof, 1, out[0..1]);
    try std.testing.expectEqual(@as(u16, 65_535), out[0]);

    proof.public_inputs.l2_messaging_blocks_offsets = &.{ 1, 2 };
    try std.testing.expectError(error.InvalidMessagingOffsets, messaging_offsets.rebase(proof, 1, &out));
}

// ── Bounds and resources ─────────────────────────────────────────────────────────────────────────

const max_filtered_addresses = core_serde.rollup_proof.MAX_FILTERED_ADDRESSES;

/// `count` distinct addresses in ascending order.
fn ascendingAddresses(a: Allocator, count: usize) ![]Address {
    const addresses = try a.alloc(Address, count);
    for (addresses, 0..) |*address, i| {
        address.* = @splat(0);
        std.mem.writeInt(u32, address[0..4], @intCast(i), .big);
    }
    return addresses;
}

test "more distinct filtered addresses than the output holds are rejected by the output encoder" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const max = max_filtered_addresses;
    const addresses = try ascendingAddresses(a, max + 1);
    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs[0].proof.public_inputs.filtered_addresses = addresses[0..max];
    request.proofs[1].proof.public_inputs.filtered_addresses = addresses[max..];

    const frame = try rollup_ssz.encodeInput(a, request.input());
    try std.testing.expectError(error.OutputEncodeFailed, rollup.rollup.runFramed(a, frame));
    try std.testing.expectEqual(guest_errors.CODE_OUTPUT_ENCODE_FAILED, guest_errors.exitCode(error.OutputEncodeFailed));
}

test "the same maximal list of filtered addresses in two proofs is accepted as one list" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const max = max_filtered_addresses;
    const addresses = try ascendingAddresses(a, max);
    const descending = try a.alloc(Address, max);
    for (descending, 0..) |*address, i| address.* = addresses[max - 1 - i];
    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs[0].proof.public_inputs.filtered_addresses = descending;
    request.proofs[1].proof.public_inputs.filtered_addresses = addresses;

    const pi = try rollup.rollup.run(a, request.input());
    try std.testing.expectEqualSlices(Address, addresses, pi.filtered_addresses);
}

test "a maximal list of distinct filtered addresses split between two proofs is accepted" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const max = max_filtered_addresses;
    const addresses = try ascendingAddresses(a, max);
    var request = try requestOf(a, &.{ 1, 1 }, .{});
    request.proofs[0].proof.public_inputs.filtered_addresses = addresses[max / 2 ..];
    request.proofs[1].proof.public_inputs.filtered_addresses = addresses[0 .. max / 2];

    const pi = try rollup.rollup.run(a, request.input());
    try std.testing.expectEqualSlices(Address, addresses, pi.filtered_addresses);
}

test "an allocator that refuses any one request ends the run in the out-of-memory category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    // One blob and one calldata chunk: the run decodes, binds, verifies and decompresses both and
    // encodes the output.
    const request = try requestOf(a, &.{ 2, 1 }, .{ .chunks = &.{ .{ .own = .{ .bytes = 20 } }, .{ .kind = .calldata } } });
    const frame = try rollup_ssz.encodeInput(a, request.input());
    const expected = try core_serde.rollup_proof.encodeOutput(a, try rollup.rollup.run(a, request.input()));

    var fail_index: usize = 0;
    while (true) : (fail_index += 1) {
        var run_arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer run_arena.deinit();
        var failing = std.testing.FailingAllocator.init(run_arena.allocator(), .{ .fail_index = fail_index });
        const output = rollup.rollup.runFramed(failing.allocator(), frame) catch |err| {
            try std.testing.expectEqual(guest_errors.CODE_OUT_OF_MEMORY, guest_errors.exitCode(err));
            continue;
        };
        try std.testing.expectEqualSlices(u8, expected, output);
        break;
    }
    try std.testing.expect(fail_index > 0);
}
