//! The guest's `decompress.verifyDaChunks` accepts the blobs of the host layer: the rollups cut from
//! a stream of random conflations, and the real conflations the compressor and a stream make.

const std = @import("std");

const da_chunks = @import("da_chunks");
const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const host = @import("host");
const lineth_da = @import("lineth_da");
const host_streams = @import("host_streams");

const Kind = ef_conflations.Kind;
const Location = host_streams.Location;
const SealedBlob = host.stream.SealedBlob;

const gpa = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;

/// The start of a rollup: its first blob and conflation, and what the guest is told about its parent.
const Rollup = struct {
    first_blob: usize,
    first_conflation: usize,
    start: da_chunks.Start,

    fn genesis() Rollup {
        return .{ .first_blob = 0, .first_conflation = 0, .start = .{ .parent = host_streams.genesis } };
    }

    /// The rollup after one that ended at `end`, the cut of the conflation `cut`: inside the blob of
    /// `end` when its tail is positive, else on the next blob.
    fn after(blobs: []const SealedBlob, end: Location, cut: usize) Rollup {
        const first_blob = if (end.tail > 0) end.blob else end.blob + 1;
        return .{
            .first_blob = first_blob,
            .first_conflation = cut + 1,
            .start = .{
                .parent = blobs[end.blob].data_rolling_hash,
                .take = end.tail,
                .parent_blob = if (end.tail > 0) blobs[first_blob].prev_data_rolling_hash else null,
            },
        };
    }
};

fn checkSeed(seed: u64) !void {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const a = arena_state.allocator();
    var scenario = try host_streams.Scenario.init(a, seed);
    defer scenario.deinit();
    const blobs = scenario.blobs.items;

    var rollup: Rollup = .genesis();
    for (scenario.cuts) |cut| {
        const end = scenario.locateCut(cut);
        const chunks = try a.alloc(da_chunks.Chunk, end.blob + 1 - rollup.first_blob);
        for (chunks, blobs[rollup.first_blob .. end.blob + 1]) |*chunk, *blob| {
            chunk.* = try da_chunks.chunkOf(a, &blob.blob, blob.kzg_commitment, blob.kzg_proof);
        }
        const payloads = try a.alloc([]const u8, cut + 1 - rollup.first_conflation);
        for (payloads, scenario.conflations[rollup.first_conflation .. cut + 1]) |*payload, conflation| payload.* = conflation.payload.?;

        try da_chunks.expectAccepted(chunks, payloads, rollup.start, blobs[end.blob].data_rolling_hash, end.tail);
        rollup = .after(blobs, end, cut);
    }
}

test "the guest accepts every rollup cut from a stream of random conflations" {
    for (1..9) |seed| try checkSeed(seed);
}

const conflations = [_][]const Kind{
    &.{ .legacy, .fee_market, .access_list },
    &.{.set_code},
    &.{ .many_transactions, .empty_block },
    &.{.large_lists},
    &.{ .calldata_heavy, .large_block },
};

test "the guest accepts the blobs a stream seals of the frames the compressor makes of real conflations" {
    var arena_state = std.heap.ArenaAllocator.init(gpa);
    defer arena_state.deinit();
    const a = arena_state.allocator();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
    const compressor = try host.compressor.Compressor.init(gpa, dictionary, selection.chain_id);
    defer compressor.deinit();
    var s = try host.stream.Stream.open(gpa, &.fresh(host_streams.genesis));
    defer s.deinit();

    var sealed: std.ArrayList(SealedBlob) = .empty;
    defer {
        for (sealed.items) |*blob| gpa.free(blob.segmentEnds());
        sealed.deinit(gpa);
    }
    const payloads = try a.alloc([]const u8, conflations.len);
    for (conflations, payloads, 1..) |kinds, *payload, number| {
        const frame = try compressor.compress(try selection.conflationRlps(a, kinds), null);
        defer gpa.free(frame);
        payload.* = try selection.conflationPayload(a, kinds);

        const blobs = try s.append(frame, number);
        defer gpa.free(blobs);
        try sealed.appendSlice(gpa, blobs);
    }
    const flushed = try s.flush();
    defer gpa.free(flushed);
    try sealed.appendSlice(gpa, flushed);

    const chunks = try a.alloc(da_chunks.Chunk, sealed.items.len);
    for (chunks, sealed.items) |*chunk, *blob| chunk.* = try da_chunks.chunkOf(a, &blob.blob, blob.kzg_commitment, blob.kzg_proof);
    const last = &sealed.items[sealed.items.len - 1];
    try da_chunks.expectAccepted(chunks, payloads, .{ .parent = host_streams.genesis }, last.data_rolling_hash, 0);
}
