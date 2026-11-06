//! The verification of DA streams over the conflations of the selected real blocks of the EF zkevm
//! fixtures.

const std = @import("std");

const da_chunks = @import("da_chunks");
const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");

const blob_payload = lineth_da.encoding.blob_payload;

const Chunk = da_chunks.Chunk;
const Kind = ef_conflations.Kind;

const blobChunk = da_chunks.blobChunk;
const calldataChunk = da_chunks.calldataChunk;
const expectAccepted = da_chunks.expectAccepted;
const expectRejected = da_chunks.expectRejected;
const foldOnto = da_chunks.foldOnto;
const parent = da_chunks.parent;
const segment = da_chunks.segment;

const gpa = std.testing.allocator;

test "conflations of real blocks travel in a blob and calldata" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);

    const payloads = [_][]const u8{
        try selection.conflationPayload(a, &.{ .legacy, .fee_market, .access_list }),
        try selection.conflationPayload(a, &.{.set_code}),
        try selection.conflationPayload(a, &.{ .many_transactions, .empty_block }),
    };
    var stream: std.ArrayList(u8) = .empty;
    for (payloads) |payload| try stream.appendSlice(a, try segment(a, payload));

    // The blob ends inside the second segment; calldata finishes it and carries the third.
    const first_end = (try segment(a, payloads[0])).len;
    const split = first_end + 9;
    const chunks = [_]Chunk{ try blobChunk(a, stream.items[0..split]), calldataChunk(stream.items[split..]) };

    try expectAccepted(&chunks, &payloads, .{}, foldOnto(parent, &chunks), 0);
}

test "large conflations of real blocks span two blobs" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);

    const payloads = [_][]const u8{
        try selection.conflationPayload(a, &.{.large_lists}),
        try selection.conflationPayload(a, std.enums.values(Kind)),
    };
    var stream: std.ArrayList(u8) = .empty;
    for (payloads) |payload| try stream.appendSlice(a, try segment(a, payload));

    // The first blob is full and ends inside the second segment.
    const capacity = blob_payload.payload_capacity;
    try std.testing.expect(stream.items.len > capacity);
    const chunks = [_]Chunk{ try blobChunk(a, stream.items[0..capacity]), try blobChunk(a, stream.items[capacity..]) };

    try expectAccepted(&chunks, &payloads, .{}, foldOnto(parent, &chunks), 0);
}

test "a conflation whose blocks differ from the published ones is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selection = try ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);

    // The same blocks in another order: a payload of the same length with other bytes.
    const published = try selection.conflationPayload(a, &.{ .legacy, .fee_market });
    const claimed = try selection.conflationPayload(a, &.{ .fee_market, .legacy });
    const calldata = calldataChunk(try segment(a, published));

    try expectRejected(error.PayloadMismatch, &.{calldata}, &.{claimed}, .{});
}
