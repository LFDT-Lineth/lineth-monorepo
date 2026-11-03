//! The verification of DA streams over the conflations of real blocks of the EF zkevm fixtures.

const std = @import("std");

const da_chunks = @import("da_chunks");
const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");

const Chunk = da_chunks.Chunk;

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
    var blocks = try ef_conflations.Blocks.open(gpa, std.testing.io, ef_fixtures.amsterdam_blocks);
    defer blocks.deinit();

    const payloads = [_][]const u8{
        try blocks.conflationPayload(a, 3),
        try blocks.conflationPayload(a, 1),
        try blocks.conflationPayload(a, 2),
    };
    var stream: std.ArrayList(u8) = .empty;
    for (payloads) |payload| try stream.appendSlice(a, try segment(a, payload));

    // The blob ends inside the second segment; calldata finishes it and carries the third.
    const first_end = (try segment(a, payloads[0])).len;
    const split = first_end + 9;
    const chunks = [_]Chunk{ try blobChunk(a, stream.items[0..split]), calldataChunk(stream.items[split..]) };

    try expectAccepted(&chunks, &payloads, .{}, foldOnto(parent, &chunks), 0);
}

test "a conflation whose blocks differ from the published ones is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    var blocks = try ef_conflations.Blocks.open(gpa, std.testing.io, ef_fixtures.amsterdam_blocks);
    defer blocks.deinit();

    const published = try blocks.conflationPayload(a, 2);
    const claimed = try blocks.conflationPayload(a, 2);
    const calldata = calldataChunk(try segment(a, published));

    try expectRejected(error.PayloadMismatch, &.{calldata}, &.{claimed}, .{});
}
