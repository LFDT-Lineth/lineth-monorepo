//! The rollup guest statement over the curated blocks: each accepted EF block alone, the EF chain of
//! ten consecutive blocks, the Linea mainnet window of 150 and three generated blocks of random calldata. Each run of consecutive blocks is
//! grouped into conflations whose hashes, parent hashes, numbers, timestamps and senders are the
//! fixture's own, published in calldata and blob chunks, alone and sharing a blob with the
//! neighbouring proofs. Every request is accepted with the public inputs the fixture determines, and
//! a tampered copy of it is rejected by the error of each rule it breaks.

const std = @import("std");

const rollup = @import("rollup");
const scenario = @import("scenario");

const rollup_ssz = rollup.rollup_ssz;
const ByteList = @import("guest_common").ssz.ByteList(rollup_ssz.MAX_BYTES_PER_BLOCK_RLP);

const Allocator = std.mem.Allocator;
const Fact = scenario.Fact;

/// Consecutive blocks of one chain.
const Run = struct {
    label: []const u8,
    chain_id: u64,
    blocks: []const Fact,
    layouts: []const Layout,
};

/// How the stream of a request is cut into chunks.
const Layout = enum {
    /// The whole stream in one blob.
    one_blob,
    /// A segment that starts in a blob and ends in calldata.
    blob_into_calldata,
    /// A blob shared with the previous and the next proof.
    shared_blob,
    /// Calldata cut at a segment boundary, when there is one.
    calldata_cut,
    calldata,
    /// Two full blobs and the rest of the stream in a third.
    full_blobs,
    /// A range that starts inside the first of two blobs.
    range_in_first_blob,

    fn chunks(self: Layout, conflations: usize) []const scenario.ChunkSpec {
        return switch (self) {
            .one_blob => &.{.{}},
            .blob_into_calldata => &.{ .{ .own = .{ .bytes = 8 } }, .{ .kind = .calldata } },
            .shared_blob => &.{.{ .prefix = .{ .bytes = 37 }, .suffix = 91 }},
            .calldata_cut => if (conflations > 1)
                &.{ .{ .kind = .calldata, .own = .{ .through_segment = 1 } }, .{ .kind = .calldata } }
            else
                &.{.{ .kind = .calldata }},
            .calldata => &.{.{ .kind = .calldata }},
            .full_blobs => &.{ .{ .own = .{ .bytes = scenario.blob_capacity } }, .{ .own = .{ .bytes = scenario.blob_capacity } }, .{} },
            .range_in_first_blob => &.{ .{ .own = .{ .bytes = 30_000 }, .prefix = .full }, .{ .suffix = 70 } },
        };
    }
};

const short_layouts = [_]Layout{ .one_blob, .blob_into_calldata, .shared_blob, .calldata_cut, .calldata };
const long_layouts = [_]Layout{ .calldata_cut, .range_in_first_blob };
const incompressible_layouts = [_]Layout{.full_blobs};

const Tally = struct {
    requests: usize = 0,
    blocks: usize = 0,
    blobs: usize = 0,
    shared_blobs: usize = 0,
    calldata_chunks: usize = 0,
    multi_conflation: usize = 0,
    tampered: usize = 0,
};

/// The rules a tampered copy of a request breaks, each with the error the guest rejects it by.
const Rule = enum {
    senders_hash,
    end_block_hash,
    parent_block_hash,
    chunk_hash,
    malformed_block,
    chunk_content,

    fn error_of(self: Rule, request: scenario.Request) anyerror {
        return switch (self) {
            .senders_hash => error.SendersHashMismatch,
            .end_block_hash => error.EndBlockHashMismatch,
            .parent_block_hash => error.ParentHashMismatch,
            .chunk_hash => error.ChunkHashMismatch,
            .malformed_block => error.MalformedBlock,
            .chunk_content => if (request.chunks[0].is_calldata.value) error.ChunkHashMismatch else error.BlobKzgMismatch,
        };
    }
};

/// Builds the request of `run` in `layout`, checks it is accepted with the public inputs the
/// fixture determines, and checks every tampered copy of it is rejected.
fn checkRequest(a: Allocator, scratch: Allocator, index: usize, run: Run, layout: Layout, tally: *Tally) !void {
    // Conflations of one to three groups of blocks, in a rotation that depends on the request.
    const group = std.math.divCeil(usize, run.blocks.len, 10) catch unreachable;
    var conflations: std.ArrayList(scenario.Conflation) = .empty;
    var start: usize = 0;
    var step = index % 3;
    while (start < run.blocks.len) : (step += 1) {
        const end = @min(start + group * (1 + step % 3), run.blocks.len);
        try conflations.append(a, try scenario.conflationOf(a, scratch, run.chain_id, run.blocks[start..end]));
        start = end;
    }

    var request = try scenario.build(a, conflations.items, .{ .chunks = layout.chunks(conflations.items.len) });
    const pi = try rollup.rollup.run(a, request.input());

    const last = run.blocks[run.blocks.len - 1];
    try std.testing.expectEqual(last.number, pi.end_block_number);
    try std.testing.expectEqual(last.timestamp, pi.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, run.blocks.len), pi.block_count);
    try std.testing.expectEqualSlices(u8, &last.hash, &pi.end_block_hash);
    try std.testing.expectEqualSlices(u8, &run.blocks[0].parent_hash, &pi.parent_block_hash);
    try std.testing.expectEqual(request.parent_data_tail_take_bytes, pi.parent_data_tail_take_bytes);
    try std.testing.expectEqualSlices(u8, &request.end_data_rolling_hash, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(request.final_data_tail_discard_bytes, pi.final_data_tail_discard_bytes);

    // The request survives the wire: the decoded frame gives the same public inputs.
    const frame = try rollup_ssz.encodeInput(a, request.input());
    try std.testing.expectEqualDeep(pi, try rollup.rollup.run(a, try rollup_ssz.decodeInput(a, frame)));

    tally.requests += 1;
    tally.blocks += run.blocks.len;
    if (conflations.items.len > 1) tally.multi_conflation += 1;
    for (request.chunks) |chunk| {
        if (chunk.is_calldata.value) tally.calldata_chunks += 1 else tally.blobs += 1;
    }
    if (request.parent_data_tail_take_bytes > 0) tally.shared_blobs += 1;

    for (std.enums.values(Rule)) |rule| {
        try expectRejected(a, &request, rule);
        tally.tampered += 1;
    }
    // Every rule left the request as it was.
    try std.testing.expectEqualDeep(pi, try rollup.rollup.run(a, request.input()));
}

/// Breaks `rule` in the accepted request, expects the guest's error for it, and mends the request.
fn expectRejected(a: Allocator, request: *scenario.Request, rule: Rule) !void {
    const proofs = try a.dupe(@typeInfo(@TypeOf(request.proofs)).pointer.child, request.proofs);
    const chunks = try a.dupe(@typeInfo(@TypeOf(request.chunks)).pointer.child, request.chunks);
    const conflations = try a.dupe(@typeInfo(@TypeOf(request.conflations)).pointer.child, request.conflations);
    const expected = rule.error_of(request.*);

    switch (rule) {
        .senders_hash => request.proofs[0].proof.public_inputs.tx_froms_hash[0] ^= 0x01,
        .end_block_hash => request.proofs[0].proof.public_inputs.end_block_hash[0] ^= 0x01,
        .parent_block_hash => request.proofs[0].proof.public_inputs.parent_block_hash[0] ^= 0x01,
        .chunk_hash => request.chunks[0].chunk_hash[0] ^= 0x01,
        .malformed_block => {
            // The first byte of a block RLP is the prefix of its list.
            const block_rlps = try a.dupe(ByteList, request.conflations[0].block_rlps.items);
            const block_rlp = try a.dupe(u8, block_rlps[0].bytes);
            block_rlp[0] ^= 0x01;
            block_rlps[0] = .{ .bytes = block_rlp };
            request.conflations[0].block_rlps = .{ .items = block_rlps };
        },
        .chunk_content => flipChunkContent(request.chunks[0]),
    }
    try scenario.prove(a, request.proofs);
    try std.testing.expectError(expected, rollup.rollup.run(a, request.input()));

    @memcpy(request.proofs, proofs);
    @memcpy(request.chunks, chunks);
    @memcpy(request.conflations, conflations);
    if (rule == .chunk_content) flipChunkContent(request.chunks[0]);
}

/// Changes the calldata of a chunk, or the proof of its blob.
fn flipChunkContent(chunk: rollup_ssz.ChunkWitness) void {
    const bytes = if (chunk.is_calldata.value) chunk.calldata_bytes.bytes else chunk.kzg_proof.bytes;
    @constCast(bytes)[0] ^= 0x01;
}

test "requests built from the curated blocks are accepted and their tampered copies rejected" {
    const selection = try scenario.selection();
    const gpa = std.testing.allocator;

    var runs: std.ArrayList(Run) = .empty;
    defer runs.deinit(gpa);
    for (selection.picks, 0..) |*pick, index| {
        try runs.append(gpa, .{ .label = ef_labels[index], .chain_id = scenario.ef_chain_id, .blocks = @as(*const [1]Fact, pick), .layouts = &short_layouts });
    }
    try runs.append(gpa, .{ .label = "EF chain", .chain_id = scenario.ef_chain_id, .blocks = selection.chain, .layouts = &short_layouts });
    try runs.append(gpa, .{ .label = "Linea window", .chain_id = scenario.linea_chain_id, .blocks = selection.linea, .layouts = &long_layouts });
    try runs.append(gpa, .{ .label = "incompressible blocks", .chain_id = scenario.incompressible_chain_id, .blocks = selection.incompressible, .layouts = &incompressible_layouts });

    var tally: Tally = .{};
    var index: usize = 0;
    for (runs.items) |run| {
        for (run.layouts) |layout| {
            var request_arena = std.heap.ArenaAllocator.init(gpa);
            defer request_arena.deinit();
            var scratch_arena = std.heap.ArenaAllocator.init(gpa);
            defer scratch_arena.deinit();

            checkRequest(request_arena.allocator(), scratch_arena.allocator(), index, run, layout, &tally) catch |err| {
                std.debug.print("{s}, {s}: {s}\n", .{ run.label, @tagName(layout), @errorName(err) });
                return err;
            };
            index += 1;
        }
    }
    std.debug.print(
        "{d} requests over {d} blocks ({d} blobs, {d} of them shared, {d} calldata chunks, {d} of several conflations); {d} tampered copies rejected\n",
        .{ tally.requests, tally.blocks, tally.blobs, tally.shared_blobs, tally.calldata_chunks, tally.multi_conflation, tally.tampered },
    );
    try std.testing.expectEqual(tally.requests * std.enums.values(Rule).len, tally.tampered);
    try std.testing.expect(tally.shared_blobs > 0);
    try std.testing.expect(tally.multi_conflation > 0);
}

/// The names of the accepted EF blocks, in the order of the selection.
const ef_labels = [_][]const u8{
    "legacy with a contract creation",
    "fee-market",
    "set-code",
    "legacy and access-list",
    "legacy and fee-market",
    "legacy and set-code",
    "access-list and set-code",
    "fee-market and set-code",
    "1020 legacy transactions",
    "200 deposits",
};
