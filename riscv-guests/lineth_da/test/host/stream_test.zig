const std = @import("std");

const host = @import("host");
const host_streams = @import("host_streams");
const lineth_da = @import("lineth_da");
const support = @import("support");

const stream = host.stream;
const blob_kzg = lineth_da.encoding.blob_kzg;
const blob_payload = lineth_da.encoding.blob_payload;
const rolling_hash = lineth_da.encoding.rolling_hash;
const Allocator = std.mem.Allocator;
const Hash = stream.Hash;
const SealedBlob = stream.SealedBlob;

const gpa = std.testing.allocator;
const capacity = blob_payload.payload_capacity;
const genesis = host_streams.genesis;
const frameOfSegment = host_streams.frameOfSegment;
const segmentOf = support.da_stream.lengthPrefixed;
const locate = host_streams.locate;
const positionAfter = host_streams.positionAfter;

/// The blobs a stream sealed, collected over its calls. `allocator` is the stream's.
const Blobs = struct {
    allocator: Allocator,
    list: std.ArrayList(SealedBlob) = .empty,

    fn deinit(self: *Blobs) void {
        for (self.list.items) |*blob| self.allocator.free(blob.segmentEnds());
        self.list.deinit(gpa);
    }

    /// Takes the blobs of `sealed` and releases its slice.
    fn take(self: *Blobs, sealed: []SealedBlob) !void {
        defer self.allocator.free(sealed);
        try self.list.appendSlice(gpa, sealed);
    }

    fn append(self: *Blobs, s: *stream.Stream, frame: []const u8, end_block: u64) !void {
        try self.take(try s.append(frame, end_block));
    }

    fn flush(self: *Blobs, s: *stream.Stream) !void {
        try self.take(try s.flush());
    }
};

fn expectEnds(blob: *const SealedBlob, expected: []const stream.SegmentEnd) !void {
    try std.testing.expectEqualDeep(expected, blob.segmentEnds());
}

fn segmentEnd(conflation_end_block: u64, unused_tail_bytes: u32) stream.SegmentEnd {
    return .{ .conflation_end_block = conflation_end_block, .unused_tail_bytes = unused_tail_bytes };
}

fn keccak(preimage: []const u8) Hash {
    var out: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(preimage, &out, .{});
    return out;
}

/// Everything a blob has to be: the canonical packing of the next `payload_bytes` of `stream_bytes`,
/// anchored by the versioned hash of its commitment, which its proof opens, and folded onto `prev`.
fn expectBlob(blob: *const SealedBlob, prev: Hash, stream_bytes: []const u8) !void {
    const scratch = try gpa.create([blob_payload.stream_len]u8);
    defer gpa.destroy(scratch);
    const payload = try blob_payload.unpack(&blob.blob, scratch);
    try std.testing.expectEqual(@as(usize, blob.payload_bytes), payload.len);
    try std.testing.expectEqualSlices(u8, stream_bytes[0..payload.len], payload);

    var digest: Hash = undefined;
    std.crypto.hash.sha2.Sha256.hash(&blob.kzg_commitment, &digest, .{});
    digest[0] = 0x01;
    try std.testing.expectEqualSlices(u8, &digest, &blob.chunk_hash);
    try std.testing.expect(blob_kzg.verifyBatch(&.{blob.blob}, &.{blob.kzg_commitment}, &.{blob.kzg_proof}));

    try std.testing.expectEqualSlices(u8, &prev, &blob.prev_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &keccak(&(prev ++ blob.chunk_hash)), &blob.data_rolling_hash);
}

/// Every blob is the next one of `stream_bytes`, folded onto `prev`, and together they take all of it.
fn expectBlobs(blobs: []const SealedBlob, prev: Hash, stream_bytes: []const u8) !void {
    var rest = stream_bytes;
    var before = prev;
    for (blobs) |*blob| {
        try expectBlob(blob, before, rest);
        rest = rest[blob.payload_bytes..];
        before = blob.data_rolling_hash;
    }
    try std.testing.expectEqual(0, rest.len);
}

// ---- a stream from a fresh start --------------------------------------------------------------

test "an append leaves a blob with room open, and a flush seals it with the end of the segment" {
    var s = try stream.Stream.open(gpa, &.fresh(genesis));
    defer s.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    const frame = try frameOfSegment(gpa, 1000, 1);
    defer gpa.free(frame);
    const bytes = try segmentOf(gpa, frame);
    defer gpa.free(bytes);

    try blobs.append(&s, frame, 1);
    try std.testing.expectEqual(0, blobs.list.items.len);
    try blobs.flush(&s);

    try std.testing.expectEqual(1, blobs.list.items.len);
    try expectBlobs(blobs.list.items, genesis, bytes);
    try std.testing.expectEqual(1000, blobs.list.items[0].payload_bytes);
    try expectEnds(&blobs.list.items[0], &.{segmentEnd(1, 0)});

    try blobs.flush(&s);
    try std.testing.expectEqual(1, blobs.list.items.len);
}

test "a full blob is sealed only when the next segment needs room" {
    var s = try stream.Stream.open(gpa, &.fresh(genesis));
    defer s.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    const full = try frameOfSegment(gpa, capacity, 1);
    defer gpa.free(full);
    const next = try frameOfSegment(gpa, 500, 2);
    defer gpa.free(next);

    try blobs.append(&s, full, 1);
    try std.testing.expectEqual(0, blobs.list.items.len);
    try blobs.append(&s, next, 2);

    try std.testing.expectEqual(1, blobs.list.items.len);
    try std.testing.expectEqual(capacity, blobs.list.items[0].payload_bytes);
    try expectEnds(&blobs.list.items[0], &.{segmentEnd(1, 0)});
    const first = try segmentOf(gpa, full);
    defer gpa.free(first);
    try expectBlob(&blobs.list.items[0], genesis, first);

    try blobs.flush(&s);
    try expectEnds(&blobs.list.items[1], &.{segmentEnd(2, 0)});
    try std.testing.expectEqual(500, blobs.list.items[1].payload_bytes);
}

test "a blob that holds several cuts reports all of them in order" {
    var s = try stream.Stream.open(gpa, &.fresh(genesis));
    defer s.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    const sizes = [_]usize{ 40_000, 50_000, 60_000 };
    var stream_bytes: std.ArrayList(u8) = .empty;
    defer stream_bytes.deinit(gpa);

    for (sizes, 1..) |size, number| {
        const frame = try frameOfSegment(gpa, size, @intCast(number));
        defer gpa.free(frame);
        const bytes = try segmentOf(gpa, frame);
        defer gpa.free(bytes);
        try stream_bytes.appendSlice(gpa, bytes);
        try blobs.append(&s, frame, @intCast(number));
    }

    try std.testing.expectEqual(1, blobs.list.items.len);
    try expectEnds(&blobs.list.items[0], &.{ segmentEnd(1, capacity - 40_000), segmentEnd(2, capacity - 90_000) });
    try blobs.flush(&s);
    try expectEnds(&blobs.list.items[1], &.{segmentEnd(3, 0)});
    try expectBlobs(blobs.list.items, genesis, stream_bytes.items);
}

test "a segment that spans several blobs leaves the blobs in between without ends" {
    var s = try stream.Stream.open(gpa, &.fresh(genesis));
    defer s.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    const small = try frameOfSegment(gpa, 10_000, 1);
    defer gpa.free(small);
    const huge = try frameOfSegment(gpa, 3 * capacity, 2);
    defer gpa.free(huge);
    const small_bytes = try segmentOf(gpa, small);
    defer gpa.free(small_bytes);
    const huge_bytes = try segmentOf(gpa, huge);
    defer gpa.free(huge_bytes);

    try blobs.append(&s, small, 1);
    try blobs.append(&s, huge, 2);

    try std.testing.expectEqual(3, blobs.list.items.len);
    try expectEnds(&blobs.list.items[0], &.{segmentEnd(1, capacity - 10_000)});
    try expectEnds(&blobs.list.items[1], &.{});
    try expectEnds(&blobs.list.items[2], &.{});
    for (blobs.list.items) |blob| try std.testing.expectEqual(capacity, blob.payload_bytes);
    try blobs.flush(&s);
    try expectEnds(&blobs.list.items[3], &.{segmentEnd(2, 0)});
    const all = try std.mem.concat(gpa, u8, &.{ small_bytes, huge_bytes });
    defer gpa.free(all);
    try expectBlobs(blobs.list.items, genesis, all);
}

test "an empty frame and a frame too long for its length prefix are rejected, and the stream goes on" {
    var s = try stream.Stream.open(gpa, &.fresh(genesis));
    defer s.deinit();

    try std.testing.expectError(error.EmptyFrame, s.append(&.{}, 1));
    // The length is checked before a byte is read.
    const too_long = @as([*]const u8, @ptrFromInt(@alignOf(usize)))[0 .. stream.max_frame_len + 1];
    try std.testing.expectError(error.FrameTooLong, s.append(too_long, 1));

    const blobs = try s.flush();
    try std.testing.expectEqual(0, blobs.len);
    const frame = try frameOfSegment(gpa, 100, 1);
    defer gpa.free(frame);
    stream.freeSealed(gpa, try s.append(frame, 1));
    const flushed = try s.flush();
    defer stream.freeSealed(gpa, flushed);
    try std.testing.expectEqual(1, flushed.len);
}

// ---- resuming inside a shared blob ------------------------------------------------------------

/// The blobs of a stream that appends the conflations A to E and flushes. The segments are cut so
/// that blob 0 holds A and the start of B, blob 1 the end of B, C and the start of D, and blob 2 the
/// end of D and E.
const sizes_a_to_e = [_]usize{ 50_000, 100_000, 40_000, 80_000, 30_000 };

test "resuming inside a shared blob reproduces the uninterrupted run byte for byte" {
    const list = try host_streams.syntheticConflations(gpa, &sizes_a_to_e);
    defer host_streams.freeSynthetic(gpa, list);
    var uninterrupted = try host_streams.run(&.fresh(genesis), list);
    defer host_streams.release(&uninterrupted);
    try std.testing.expectEqual(3, uninterrupted.items.len);
    const all = try host_streams.streamBytes(gpa, list);
    defer gpa.free(all);
    try expectBlobs(uninterrupted.items, genesis, all);

    // The rollup ended with C, inside blob 1.
    const position = positionAfter(uninterrupted.items, locate(uninterrupted.items, 3));
    try std.testing.expect(position.shared_blob_carry_over_tail_bytes > 0);
    var resumed = try stream.Stream.open(gpa, &position);
    defer resumed.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    try blobs.append(&resumed, list[3].frame, list[3].end_block);
    try blobs.append(&resumed, list[4].frame, list[4].end_block);
    try blobs.flush(&resumed);

    try std.testing.expectEqual(2, blobs.list.items.len);
    for (blobs.list.items, uninterrupted.items[1..]) |*actual, *expected| try host_streams.expectSameBlobBytes(expected, actual);
    // The shared blob reports the cuts of the segments appended after the resume: none, D spans into blob 2.
    try expectEnds(&blobs.list.items[0], &.{});
    try std.testing.expectEqualDeep(uninterrupted.items[2].segmentEnds(), blobs.list.items[1].segmentEnds());
}

test "bytes that differ from the shared blob fail with a mismatch and leave the stream unchanged" {
    const list = try host_streams.syntheticConflations(gpa, &sizes_a_to_e);
    defer host_streams.freeSynthetic(gpa, list);
    var uninterrupted = try host_streams.run(&.fresh(genesis), list);
    defer host_streams.release(&uninterrupted);
    const position = positionAfter(uninterrupted.items, locate(uninterrupted.items, 3));
    var resumed = try stream.Stream.open(gpa, &position);
    defer resumed.deinit();

    // Another length, so another prefix, and a byte changed in the part the shared blob holds.
    const longer = try frameOfSegment(gpa, 80_001, 4);
    defer gpa.free(longer);
    try std.testing.expectError(error.SeededBlobMismatch, resumed.append(longer, 4));
    const last_shared_byte = try gpa.dupe(u8, list[3].frame);
    defer gpa.free(last_shared_byte);
    last_shared_byte[position.shared_blob_carry_over_tail_bytes - 5] ^= 1;
    try std.testing.expectError(error.SeededBlobMismatch, resumed.append(last_shared_byte, 4));

    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    try blobs.append(&resumed, list[3].frame, list[3].end_block);
    try blobs.append(&resumed, list[4].frame, list[4].end_block);
    try blobs.flush(&resumed);
    for (blobs.list.items, uninterrupted.items[1..]) |*actual, *expected| try host_streams.expectSameBlobBytes(expected, actual);
}

test "a flush is rejected until the shared blob is reproduced" {
    const list = try host_streams.syntheticConflations(gpa, &.{ 2_000, 3_000 });
    defer host_streams.freeSynthetic(gpa, list);
    var uninterrupted = try host_streams.run(&.fresh(genesis), list);
    defer host_streams.release(&uninterrupted);
    const shared = &uninterrupted.items[0];
    try std.testing.expectEqual(5_000, shared.payload_bytes);

    var resumed = try stream.Stream.open(gpa, &positionAfter(uninterrupted.items, locate(uninterrupted.items, 1)));
    defer resumed.deinit();
    try std.testing.expectError(error.FlushWhileReproducing, resumed.flush());

    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    try blobs.append(&resumed, list[1].frame, list[1].end_block);
    try std.testing.expectEqual(0, blobs.list.items.len);
    try blobs.flush(&resumed);
    try std.testing.expectEqual(1, blobs.list.items.len);
    try host_streams.expectSameBlobBytes(shared, &blobs.list.items[0]);
    try expectEnds(&blobs.list.items[0], &.{segmentEnd(2, 0)});
}

test "a short shared blob is sealed short when the next segment needs room" {
    const list = try host_streams.syntheticConflations(gpa, &.{ 2_000, 3_000, 6_000 });
    defer host_streams.freeSynthetic(gpa, list);
    var uninterrupted = try host_streams.run(&.fresh(genesis), list[0..2]);
    defer host_streams.release(&uninterrupted);
    const shared = &uninterrupted.items[0];

    var resumed = try stream.Stream.open(gpa, &positionAfter(uninterrupted.items, locate(uninterrupted.items, 1)));
    defer resumed.deinit();
    var blobs: Blobs = .{ .allocator = gpa };
    defer blobs.deinit();
    try blobs.append(&resumed, list[1].frame, list[1].end_block);
    try std.testing.expectEqual(0, blobs.list.items.len);
    try blobs.append(&resumed, list[2].frame, list[2].end_block);

    try std.testing.expectEqual(1, blobs.list.items.len);
    try host_streams.expectSameBlobBytes(shared, &blobs.list.items[0]);
    try expectEnds(&blobs.list.items[0], &.{segmentEnd(2, 0)});
    try blobs.flush(&resumed);
    try std.testing.expectEqualSlices(u8, &shared.data_rolling_hash, &blobs.list.items[1].prev_data_rolling_hash);
    try std.testing.expectEqual(6_000, blobs.list.items[1].payload_bytes);
}

test "a stream opens only at a position that its shared blob explains" {
    const list = try host_streams.syntheticConflations(gpa, &.{ 2_000, 3_000 });
    defer host_streams.freeSynthetic(gpa, list);
    var uninterrupted = try host_streams.run(&.fresh(genesis), list);
    defer host_streams.release(&uninterrupted);
    const shared = &uninterrupted.items[0];
    const good = positionAfter(uninterrupted.items, locate(uninterrupted.items, 1));

    var other_parent = good;
    other_parent.parent_blob_data_rolling_hash = @splat(0);
    try std.testing.expectError(error.SharedBlobHashMismatch, stream.Stream.open(gpa, &other_parent));
    var other_hash = good;
    other_hash.data_rolling_hash = @splat(0);
    try std.testing.expectError(error.SharedBlobHashMismatch, stream.Stream.open(gpa, &other_hash));

    var no_blob = good;
    no_blob.shared_blob = null;
    try std.testing.expectError(error.MissingSharedBlob, stream.Stream.open(gpa, &no_blob));

    var whole_payload = good;
    whole_payload.shared_blob_carry_over_tail_bytes = shared.payload_bytes;
    try std.testing.expectError(error.TailTooLong, stream.Stream.open(gpa, &whole_payload));
    var all_but_one = good;
    all_but_one.shared_blob_carry_over_tail_bytes = shared.payload_bytes - 1;
    var edge = try stream.Stream.open(gpa, &all_but_one);
    edge.deinit();

    const all_ones: [stream.blob_len]u8 = @splat(0xff);
    const all_zeros: [stream.blob_len]u8 = @splat(0);
    for ([_]*const [stream.blob_len]u8{ &all_ones, &all_zeros }) |bytes| {
        var broken = good;
        broken.shared_blob = bytes;
        try std.testing.expectError(error.NonCanonicalSharedBlob, stream.Stream.open(gpa, &broken));
    }
}

test "a position without a tail starts on a fresh blob and ignores the rest of the position" {
    var position = stream.Position.fresh(genesis);
    position.parent_blob_data_rolling_hash = @splat(0x77);
    var s = try stream.Stream.open(gpa, &position);
    defer s.deinit();
    const frame = try frameOfSegment(gpa, 100, 1);
    defer gpa.free(frame);
    stream.freeSealed(gpa, try s.append(frame, 1));
    const flushed = try s.flush();
    defer stream.freeSealed(gpa, flushed);
    try std.testing.expectEqualSlices(u8, &genesis, &flushed[0].prev_data_rolling_hash);
}

// ---- refused allocations ----------------------------------------------------------------------

/// A script on a stream opened inside a short shared blob: reproducing it, a segment that seals it
/// and a blob, a short one and a flush.
const Script = struct {
    position: stream.Position,
    frames: []const []const u8,
    ends: []const u64,
    expected: []const SealedBlob,
};

/// The blobs of `script`, where a step whose allocation `failing` refuses is repeated with the
/// allocator working again.
fn runScript(failing: *std.testing.FailingAllocator, script: *const Script) !Blobs {
    const allocator = failing.allocator();
    var s = stream.Stream.open(allocator, &script.position) catch |err| blk: {
        try std.testing.expectEqual(error.OutOfMemory, err);
        failing.fail_index = std.math.maxInt(usize);
        break :blk try stream.Stream.open(allocator, &script.position);
    };
    defer s.deinit();
    var blobs: Blobs = .{ .allocator = allocator };
    errdefer blobs.deinit();
    for (script.frames, script.ends) |frame, end_block| {
        const sealed = s.append(frame, end_block) catch |err| blk: {
            try std.testing.expectEqual(error.OutOfMemory, err);
            failing.fail_index = std.math.maxInt(usize);
            break :blk try s.append(frame, end_block);
        };
        try blobs.take(sealed);
    }
    const flushed = s.flush() catch |err| blk: {
        try std.testing.expectEqual(error.OutOfMemory, err);
        failing.fail_index = std.math.maxInt(usize);
        break :blk try s.flush();
    };
    try blobs.take(flushed);
    return blobs;
}

test "an allocation that fails leaves the stream as it was and leaks nothing" {
    const list = try host_streams.syntheticConflations(gpa, &.{ 2_000, 3_000, 140_000, 2_000 });
    defer host_streams.freeSynthetic(gpa, list);
    var prefix = try host_streams.run(&.fresh(genesis), list[0..2]);
    defer host_streams.release(&prefix);
    var script: Script = .{
        .position = positionAfter(prefix.items, locate(prefix.items, 1)),
        .frames = &.{ list[1].frame, list[2].frame, list[3].frame },
        .ends = &.{ 2, 3, 4 },
        .expected = &.{},
    };

    var reference_failing = std.testing.FailingAllocator.init(gpa, .{});
    var reference = try runScript(&reference_failing, &script);
    defer reference.deinit();
    try std.testing.expectEqual(3, reference.list.items.len);
    script.expected = reference.list.items;
    const needed_allocations = reference_failing.alloc_index;

    for (0..needed_allocations) |fail_index| {
        var failing = std.testing.FailingAllocator.init(gpa, .{ .fail_index = fail_index });
        var blobs = try runScript(&failing, &script);
        try std.testing.expect(failing.has_induced_failure);
        try std.testing.expectEqual(script.expected.len, blobs.list.items.len);
        for (script.expected, blobs.list.items) |*expected, *actual| {
            try host_streams.expectSameBlob(expected, actual);
        }
        blobs.deinit();
        try std.testing.expectEqual(failing.allocated_bytes, failing.freed_bytes);
    }
}
