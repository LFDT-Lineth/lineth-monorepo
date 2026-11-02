//! The rollup guest statement over the Amsterdam blocks of the Ethereum Foundation's zkevm fixtures.
//! The consecutive valid blocks of each fixture are grouped into conflations whose hashes, parent
//! hashes, numbers, timestamps and senders are the fixture's own, published in calldata and blob
//! chunks, alone and sharing a blob with the neighbouring proofs. Every request is accepted with
//! the public inputs the fixture determines, and a tampered copy of it is rejected by the error of
//! the rule it breaks.

const std = @import("std");

const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");
const rollup = @import("rollup");
const scenario = @import("scenario");

const truncation = lineth_da.encoding.truncation;
const rollup_ssz = rollup.rollup_ssz;

const Allocator = std.mem.Allocator;
const Hash = scenario.Hash;
const Address = scenario.Address;

const gpa = std.testing.allocator;
const io = std.testing.io;

/// One request in this many publishes its stream in a blob, whose commitment and proof cost far more
/// than the rest of the request.
const blob_period = 16;

const Header = struct {
    hash: []const u8,
    parentHash: []const u8,
    timestamp: []const u8,
    number: []const u8,
};

const Transaction = struct {
    sender: []const u8,
};

const Block = struct {
    rlp: ?[]const u8 = null,
    blockHeader: ?Header = null,
    transactions: ?[]const Transaction = null,
    expectException: ?[]const u8 = null,
};

const Case = struct {
    config: struct { chainid: []const u8 } = .{ .chainid = "0x01" },
    blocks: []const Block = &.{},
};

/// A block the fixture accepts, as the fixture lists it.
const Fact = struct {
    rlp: []const u8,
    hash: Hash,
    parent_hash: Hash,
    number: u64,
    timestamp: u64,
    senders: []const Address,
};

fn decodeHex(comptime len: usize, text: []const u8) ![len]u8 {
    var bytes: [len]u8 = undefined;
    _ = try std.fmt.hexToBytes(&bytes, if (std.mem.startsWith(u8, text, "0x")) text[2..] else text);
    return bytes;
}

fn factOf(a: Allocator, block: Block) !Fact {
    const header = block.blockHeader.?;
    const text = block.rlp.?;
    const rlp = try a.alloc(u8, (text.len - 2) / 2);
    _ = try std.fmt.hexToBytes(rlp, text[2..]);
    const transactions = block.transactions orelse &.{};
    const senders = try a.alloc(Address, transactions.len);
    for (transactions, senders) |transaction, *sender| sender.* = try decodeHex(20, transaction.sender);
    return .{
        .rlp = rlp,
        .hash = try decodeHex(32, header.hash),
        .parent_hash = try decodeHex(32, header.parentHash),
        .number = try std.fmt.parseInt(u64, header.number, 0),
        .timestamp = try std.fmt.parseInt(u64, header.timestamp, 0),
        .senders = senders,
    };
}

/// The conflation of `blocks`: what the fixture lists, and the canonical payload the truncation of
/// the package derives from the blocks' RLPs.
fn conflationOf(a: Allocator, scratch: Allocator, chain_id: u64, blocks: []const Fact) !scenario.Conflation {
    const rlps = try a.alloc([]const u8, blocks.len);
    const hashes = try a.alloc(Hash, blocks.len);
    const truncated = try a.alloc(truncation.TruncatedBlock, blocks.len);
    var senders: std.ArrayList(Address) = .empty;
    for (blocks, rlps, hashes, truncated) |block, *rlp, *hash, *slot| {
        rlp.* = block.rlp;
        hash.* = block.hash;
        slot.* = try truncation.truncateBlockRlp(a, scratch, block.rlp, chain_id);
        try senders.appendSlice(a, block.senders);
    }
    return .{
        .block_rlps = rlps,
        .payload = try truncation.encodeBlocks(a, scratch, truncated),
        .first_number = blocks[0].number,
        .parent_hash = blocks[0].parent_hash,
        .end_hash = blocks[blocks.len - 1].hash,
        .end_timestamp = blocks[blocks.len - 1].timestamp,
        .block_hashes = hashes,
        .senders = try senders.toOwnedSlice(a),
    };
}

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
        try conflations.append(a, try conflationOf(a, scratch, chain_id, run[start..end]));
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
    for (request.chunks) |chunk| {
        if (chunk.is_calldata) tally.calldata_chunks += 1 else tally.blobs += 1;
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
            const block_rlp = try a.dupe(u8, request.conflations[0].block_rlps[0]);
            block_rlp[0] ^= 0x01;
            const block_rlps = try a.dupe([]const u8, request.conflations[0].block_rlps);
            block_rlps[0] = block_rlp;
            request.conflations[0].block_rlps = block_rlps;
            break :blk error.MalformedBlock;
        },
        else => blk: {
            const first = request.chunks[0];
            if (first.is_calldata) {
                @constCast(first.calldata_bytes)[0] ^= 0x01;
                break :blk error.ChunkHashMismatch;
            }
            @constCast(first.kzg_proof)[0] ^= 0x01;
            break :blk error.BlobKzgMismatch;
        },
    };
    try std.testing.expectError(expected, rollup.rollup.run(a, request.input()));
}

test "requests built from consecutive blocks of the fixtures are accepted and their tampered copies rejected" {
    var dir = try std.Io.Dir.openDirAbsolute(io, ef_fixtures.amsterdam_blocks, .{ .iterate = true });
    defer dir.close(io);
    var walker = try dir.walk(gpa);
    defer walker.deinit();

    var tally: Tally = .{};
    var failures: usize = 0;
    var index: usize = 0;
    while (try walker.next(io)) |entry| {
        if (entry.kind != .file or !std.mem.endsWith(u8, entry.basename, ".json")) continue;

        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        const text = try dir.readFileAlloc(io, entry.path, arena.allocator(), .limited(1 << 30));
        const cases = try std.json.parseFromSliceLeaky(std.json.ArrayHashMap(Case), arena.allocator(), text, .{ .ignore_unknown_fields = true });

        for (cases.map.keys(), cases.map.values()) |name, case| {
            const chain_id = try std.fmt.parseInt(u64, case.config.chainid, 0);
            var request_arena = std.heap.ArenaAllocator.init(gpa);
            defer request_arena.deinit();
            const a = request_arena.allocator();
            var scratch_arena = std.heap.ArenaAllocator.init(gpa);
            defer scratch_arena.deinit();

            var run: std.ArrayList(Fact) = .empty;
            for (case.blocks, 0..) |block, position| {
                const accepted = block.expectException == null and block.rlp != null and block.blockHeader != null;
                if (accepted) {
                    const fact = try factOf(a, block);
                    // A run ends where the next block does not continue it.
                    if (run.items.len > 0) {
                        const previous = run.items[run.items.len - 1];
                        if (!std.mem.eql(u8, &previous.hash, &fact.parent_hash) or previous.number + 1 != fact.number) {
                            try finishRun(a, scratch_arena.allocator(), &index, chain_id, run.items, &tally, &failures, entry.path, name);
                            run.clearRetainingCapacity();
                        }
                    }
                    try run.append(a, fact);
                }
                if (!accepted or position + 1 == case.blocks.len) {
                    try finishRun(a, scratch_arena.allocator(), &index, chain_id, run.items, &tally, &failures, entry.path, name);
                    run.clearRetainingCapacity();
                }
            }
        }
    }
    std.debug.print(
        "{d} requests over {d} blocks ({d} blobs, {d} of them shared, {d} calldata chunks); {d} tampered copies rejected\n",
        .{ tally.requests, tally.blocks, tally.blobs, tally.shared_blobs, tally.calldata_chunks, tally.tampered },
    );
    try std.testing.expect(tally.requests > 1000);
    try std.testing.expect(tally.shared_blobs > 10);
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
