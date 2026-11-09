//! The rollup guest statement over the Amsterdam blocks of the Ethereum Foundation's zkevm fixtures.
//! The consecutive valid blocks of each fixture are grouped into conflations whose hashes, parent
//! hashes, numbers, timestamps and senders are the fixture's own, published in calldata and blob
//! chunks, alone and sharing a blob with the neighbouring proofs. Every request is accepted with
//! the public inputs the fixture determines, and a tampered copy of it is rejected by the error of
//! the rule it breaks.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const ef_fixtures = @import("ef_fixtures");
const rollup = @import("rollup");
const scenario = @import("scenario");

const rollup_ssz = rollup.rollup_ssz;
const ByteList = @import("guest_common").ssz.ByteList(rollup_ssz.MAX_BYTES_PER_BLOCK_RLP);

const Allocator = std.mem.Allocator;
const Fact = ef_blocks.Fact;

const gpa = std.testing.allocator;
const io = std.testing.io;

/// One request in this many publishes its stream in a blob, whose commitment and proof cost far more
/// than the rest of the request.
const blob_period = 16;

/// How the stream of the `index`-th request is cut into chunks.
fn chunksOf(index: usize, conflations: usize) []const scenario.ChunkSpec {
    return switch (index % blob_period) {
        0 => &.{.{}},
        // A segment that starts in a blob and ends in calldata.
        blob_period / 2 - 1 => &.{ .{ .own = .{ .bytes = 8 } }, .{ .kind = .calldata } },
        // A blob shared with the previous and the next proof.
        blob_period / 2 => &.{.{ .prefix = .{ .bytes = 37 }, .suffix = 91 }},
        // Calldata cut at a segment boundary, when there is one.
        3 => if (conflations > 1)
            &.{ .{ .kind = .calldata, .own = .{ .through_segment = 1 } }, .{ .kind = .calldata } }
        else
            &.{.{ .kind = .calldata }},
        else => &.{.{ .kind = .calldata }},
    };
}

const Tally = struct {
    requests: usize = 0,
    blocks: usize = 0,
    blobs: usize = 0,
    shared_blobs: usize = 0,
    calldata_chunks: usize = 0,
    multi_conflation: usize = 0,
    tampered: usize = 0,
};

/// Builds the request of the `index`-th run of consecutive blocks, checks it is accepted with the
/// public inputs the fixture determines, and checks a tampered copy of it is rejected.
fn checkRun(a: Allocator, scratch: Allocator, index: usize, chain_id: u64, run: []const Fact, tally: *Tally) !void {
    // Conflations of one to three blocks, in a rotation that depends on the run.
    var conflations: std.ArrayList(scenario.Conflation) = .empty;
    var start: usize = 0;
    var step = index % 3;
    while (start < run.len) : (step += 1) {
        const end = @min(start + 1 + step % 3, run.len);
        try conflations.append(a, try scenario.conflationOf(a, scratch, chain_id, run[start..end]));
        start = end;
    }

    const chunk_specs = chunksOf(index, conflations.items.len);
    var request = try scenario.build(a, conflations.items, .{ .chain_id = chain_id, .chunks = chunk_specs });
    const pi = try rollup.rollup.run(a, request.input());

    const last = run[run.len - 1];
    try std.testing.expectEqual(last.number, pi.end_block_number);
    try std.testing.expectEqual(last.timestamp, pi.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, run.len), pi.block_count);
    try std.testing.expectEqualSlices(u8, &last.hash, &pi.end_block_hash);
    try std.testing.expectEqualSlices(u8, &run[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqual(request.parent_data_tail_take_bytes, pi.parent_data_tail_take_bytes);
    try std.testing.expectEqualSlices(u8, &request.end_data_rolling_hash, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(request.final_data_tail_discard_bytes, pi.final_data_tail_discard_bytes);

    // The request survives the wire: the decoded frame gives the same public inputs.
    if (index % 32 == 0) {
        const frame = try rollup_ssz.encodeInput(a, request.input());
        try std.testing.expectEqualDeep(pi, try rollup.rollup.run(a, try rollup_ssz.decodeInput(a, frame)));
    }

    tally.requests += 1;
    tally.blocks += run.len;
    if (conflations.items.len > 1) tally.multi_conflation += 1;
    for (request.chunks) |chunk| {
        if (chunk.is_calldata.value) tally.calldata_chunks += 1 else tally.blobs += 1;
    }
    if (request.parent_data_tail_take_bytes > 0) tally.shared_blobs += 1;

    try tamper(a, index, &request);
    tally.tampered += 1;
}

/// Breaks one rule of the accepted request, in a rotation, and expects the guest's error for it.
fn tamper(a: Allocator, index: usize, request: *scenario.Request) !void {
    const proofs = request.proofs;
    const expected: anyerror = switch (index % 6) {
        0 => blk: {
            proofs[0].proof.public_inputs.tx_froms_hash[0] ^= 0x01;
            break :blk error.SendersHashMismatch;
        },
        1 => blk: {
            proofs[0].proof.public_inputs.end_block_hash[0] ^= 0x01;
            break :blk error.EndBlockHashMismatch;
        },
        2 => blk: {
            proofs[0].proof.public_inputs.parent_block_hash[0] ^= 0x01;
            break :blk error.ParentHashMismatch;
        },
        3 => blk: {
            request.chunks[0].chunk_hash[0] ^= 0x01;
            break :blk error.ChunkHashMismatch;
        },
        4 => blk: {
            // The first byte of a block RLP is the prefix of its list.
            const block_rlps = try a.dupe(ByteList, request.conflations[0].block_rlps.items);
            const block_rlp = try a.dupe(u8, block_rlps[0].bytes);
            block_rlp[0] ^= 0x01;
            block_rlps[0] = .{ .bytes = block_rlp };
            request.conflations[0].block_rlps = .{ .items = block_rlps };
            break :blk error.MalformedBlock;
        },
        else => blk: {
            const first = request.chunks[0];
            if (first.is_calldata.value) {
                @constCast(first.calldata_bytes.bytes)[0] ^= 0x01;
                break :blk error.ChunkHashMismatch;
            }
            @constCast(first.kzg_proof.bytes)[0] ^= 0x01;
            break :blk error.BlobKzgMismatch;
        },
    };
    try std.testing.expectError(expected, rollup.rollup.run(a, request.input()));
}

test "requests built from consecutive blocks of the fixtures are accepted and their tampered copies rejected" {
    var corpus = try ef_blocks.Corpus.open(gpa, io, ef_fixtures.amsterdam_blocks);
    defer corpus.deinit();

    var tally: Tally = .{};
    var failures: usize = 0;
    var index: usize = 0;
    while (try corpus.next()) |case| {
        var request_arena = std.heap.ArenaAllocator.init(gpa);
        defer request_arena.deinit();
        const a = request_arena.allocator();
        var scratch_arena = std.heap.ArenaAllocator.init(gpa);
        defer scratch_arena.deinit();

        var run: std.ArrayList(Fact) = .empty;
        for (case.blocks, 0..) |block, position| {
            // The fixture's rejected blocks and the blocks with a blob transaction end a run.
            const usable: ?Fact = if (block) |fact| (if (fact.blob_transaction) null else fact) else null;
            if (usable) |fact| {
                // A run ends where the next block does not continue it.
                if (run.items.len > 0) {
                    const previous = run.items[run.items.len - 1];
                    if (!std.mem.eql(u8, &previous.hash, &fact.parent_hash) or previous.number + 1 != fact.number) {
                        try finishRun(a, scratch_arena.allocator(), &index, case.chain_id, run.items, &tally, &failures, case.path, case.name);
                        run.clearRetainingCapacity();
                    }
                }
                try run.append(a, fact);
            }
            if (usable == null or position + 1 == case.blocks.len) {
                try finishRun(a, scratch_arena.allocator(), &index, case.chain_id, run.items, &tally, &failures, case.path, case.name);
                run.clearRetainingCapacity();
            }
        }
    }
    std.debug.print(
        "{d} requests over {d} blocks ({d} blobs, {d} of them shared, {d} calldata chunks, {d} of several conflations); {d} tampered copies rejected\n",
        .{ tally.requests, tally.blocks, tally.blobs, tally.shared_blobs, tally.calldata_chunks, tally.multi_conflation, tally.tampered },
    );
    try std.testing.expect(tally.requests > 1000);
    try std.testing.expect(tally.shared_blobs > 10);
    try std.testing.expect(tally.multi_conflation > 100);
    try std.testing.expectEqual(0, failures);
}

/// Checks the run, if it holds a block, counting and reporting a failure instead of ending the walk.
fn finishRun(
    a: Allocator,
    scratch: Allocator,
    index: *usize,
    chain_id: u64,
    run: []const Fact,
    tally: *Tally,
    failures: *usize,
    path: []const u8,
    name: []const u8,
) !void {
    if (run.len == 0) return;
    defer index.* += 1;
    checkRun(a, scratch, index.*, chain_id, run, tally) catch |err| {
        failures.* += 1;
        if (failures.* <= 20) std.debug.print("{s}: {s}: {s}\n", .{ path, name, @errorName(err) });
    };
}
