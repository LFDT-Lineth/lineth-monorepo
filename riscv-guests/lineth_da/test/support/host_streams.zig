//! What the tests of the host layer share: conflations with synthetic or random frames, a stream
//! that runs them, the blobs it seals, where the cuts are, and the comparison of two runs.

const std = @import("std");

const da_chunks = @import("da_chunks");
const host = @import("host");
const support = @import("support");

const stream = host.stream;
const Allocator = std.mem.Allocator;
const SealedBlob = stream.SealedBlob;

const gpa = std.testing.allocator;

/// The data rolling hash a stream starts from.
pub const genesis: stream.Hash = @splat(0x3c);

pub const Conflation = struct {
    frame: []const u8,
    end_block: u64,
    /// Whether the stream is flushed after the conflation.
    flush: bool = false,
    /// The payload the frame compresses, for the frames of random payloads.
    payload: ?[]const u8 = null,
};

/// Where the segment of a conflation ends: in the blob `blob`, as its `end`th cut, followed by
/// `tail` bytes of payload.
pub const Location = struct {
    blob: usize,
    end: usize,
    tail: u32,
};

/// A frame whose segment, the frame behind its four-byte length prefix, is `segment_len` bytes.
pub fn frameOfSegment(allocator: Allocator, segment_len: usize, seed: u8) ![]u8 {
    const frame = try allocator.alloc(u8, segment_len - stream.length_prefix_len);
    for (frame, 0..) |*byte, i| byte.* = @truncate(i *% 131 +% seed *% 17 +% (i >> 9));
    return frame;
}

/// The stream the segments of `conflations` make, in order.
pub fn streamBytes(allocator: Allocator, conflations: []const Conflation) ![]u8 {
    var bytes: std.ArrayList(u8) = .empty;
    errdefer bytes.deinit(allocator);
    for (conflations) |conflation| {
        const segment = try support.da_stream.lengthPrefixed(allocator, conflation.frame);
        defer allocator.free(segment);
        try bytes.appendSlice(allocator, segment);
    }
    return bytes.toOwnedSlice(allocator);
}

/// Conflations of synthetic frames whose segments have the sizes `segment_sizes`, ending at blocks
/// 1, 2, …; release them with `freeSynthetic`.
pub fn syntheticConflations(allocator: Allocator, segment_sizes: []const usize) ![]Conflation {
    const list = try allocator.alloc(Conflation, segment_sizes.len);
    for (list, segment_sizes, 1..) |*conflation, size, number| {
        conflation.* = .{ .frame = try frameOfSegment(allocator, size, @intCast(number)), .end_block = number };
    }
    return list;
}

pub fn freeSynthetic(allocator: Allocator, conflations: []const Conflation) void {
    for (conflations) |conflation| allocator.free(conflation.frame);
    allocator.free(conflations);
}

/// Six conflations of random payloads, which `arena` holds along with their frames, flushed at
/// random and after the last.
pub fn randomConflations(arena: Allocator, random: std.Random) ![]Conflation {
    const conflations = try arena.alloc(Conflation, 6);
    for (conflations, 1..) |*conflation, number| {
        const payload = try randomPayload(arena, random);
        conflation.* = .{
            .frame = try da_chunks.packageFrame(arena, payload),
            .end_block = number,
            .flush = random.uintLessThan(u8, 6) == 0,
            .payload = payload,
        };
    }
    conflations[conflations.len - 1].flush = true;
    return conflations;
}

/// A stream of random conflations: where rollups may end, and the blobs the stream seals.
pub const Scenario = struct {
    conflations: []const Conflation,
    /// The conflations after which a rollup ends: every flush, the last conflation, and others at random.
    cuts: []const usize,
    /// The blobs of a stream that appends every conflation, flushing where they say.
    blobs: std.ArrayList(SealedBlob),

    /// The scenario of `seed`; `arena` holds the payloads and frames.
    pub fn init(arena: Allocator, seed: u64) !Scenario {
        var prng = std.Random.DefaultPrng.init(seed);
        const random = prng.random();
        const conflations = try randomConflations(arena, random);
        var cuts: std.ArrayList(usize) = .empty;
        for (conflations, 0..) |conflation, index| {
            if (conflation.flush or random.uintLessThan(u8, 3) == 0) try cuts.append(arena, index);
        }
        return .{ .conflations = conflations, .cuts = cuts.items, .blobs = try run(&.fresh(genesis), conflations) };
    }

    pub fn deinit(self: *Scenario) void {
        release(&self.blobs);
    }

    /// Where the segment of the conflation `cut` ends.
    pub fn locateCut(self: Scenario, cut: usize) Location {
        return locate(self.blobs.items, self.conflations[cut].end_block);
    }
};

/// Where the segment of the conflation ending at `end_block` ends in `blobs`.
pub fn locate(blobs: []const SealedBlob, end_block: u64) Location {
    for (blobs, 0..) |*blob, blob_index| {
        for (blob.segmentEnds(), 0..) |cut, end| {
            if (cut.conflation_end_block == end_block) return .{ .blob = blob_index, .end = end, .tail = cut.unused_tail_bytes };
        }
    }
    unreachable;
}

/// The position after a rollup that ended at `end` in `blobs`: inside the blob when its tail is
/// positive, else on a fresh blob after it.
pub fn positionAfter(blobs: []const SealedBlob, end: Location) stream.Position {
    const blob = &blobs[end.blob];
    if (end.tail == 0) return .fresh(blob.data_rolling_hash);
    return .{
        .data_rolling_hash = blob.data_rolling_hash,
        .parent_blob_data_rolling_hash = blob.prev_data_rolling_hash,
        .shared_blob_carry_over_tail_bytes = end.tail,
        .shared_blob = &blob.blob,
    };
}

/// The blobs of a stream opened at `position` that appends `conflations`, flushing after those that
/// say so and at the end. Release them with `release`.
pub fn run(position: *const stream.Position, conflations: []const Conflation) !std.ArrayList(SealedBlob) {
    var s = try stream.Stream.open(gpa, position);
    defer s.deinit();
    var blobs: std.ArrayList(SealedBlob) = .empty;
    errdefer release(&blobs);
    for (conflations) |conflation| {
        const sealed = try s.append(conflation.frame, conflation.end_block);
        defer gpa.free(sealed);
        try blobs.appendSlice(gpa, sealed);
        if (conflation.flush) try flushInto(&s, &blobs);
    }
    try flushInto(&s, &blobs);
    return blobs;
}

fn flushInto(s: *stream.Stream, blobs: *std.ArrayList(SealedBlob)) !void {
    const flushed = try s.flush();
    defer gpa.free(flushed);
    try blobs.appendSlice(gpa, flushed);
}

pub fn release(blobs: *std.ArrayList(SealedBlob)) void {
    for (blobs.items) |*blob| gpa.free(blob.segmentEnds());
    blobs.deinit(gpa);
}

/// Two blobs agree in their bytes, hashes, commitments, proofs and payload sizes.
pub fn expectSameBlobBytes(expected: *const SealedBlob, actual: *const SealedBlob) !void {
    try std.testing.expectEqualSlices(u8, &expected.blob, &actual.blob);
    try std.testing.expectEqualSlices(u8, &expected.chunk_hash, &actual.chunk_hash);
    try std.testing.expectEqualSlices(u8, &expected.kzg_commitment, &actual.kzg_commitment);
    try std.testing.expectEqualSlices(u8, &expected.kzg_proof, &actual.kzg_proof);
    try std.testing.expectEqualSlices(u8, &expected.prev_data_rolling_hash, &actual.prev_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &expected.data_rolling_hash, &actual.data_rolling_hash);
    try std.testing.expectEqual(expected.payload_bytes, actual.payload_bytes);
}

/// Two blobs agree in everything, the segment ends included.
pub fn expectSameBlob(expected: *const SealedBlob, actual: *const SealedBlob) !void {
    try expectSameBlobBytes(expected, actual);
    try std.testing.expectEqualDeep(expected.segmentEnds(), actual.segmentEnds());
}

fn randomPayload(arena: Allocator, random: std.Random) ![]u8 {
    const payload = try arena.alloc(u8, 1 + random.uintLessThan(usize, 150_000));
    switch (random.uintLessThan(u8, 4)) {
        0 => for (payload, 0..) |*byte, i| {
            byte.* = @truncate(i / (1 + random.uintLessThan(usize, 5)));
        },
        else => random.bytes(payload),
    }
    return payload;
}
