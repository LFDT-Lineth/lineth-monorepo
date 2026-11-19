const std = @import("std");

const da_chunks = @import("da_chunks");
const lineth_da = @import("lineth_da");
const support = @import("support");
const zstd_frames = @import("zstd_frames");

const blob_payload = lineth_da.encoding.blob_payload;
const da_stream = support.da_stream;

const Allocator = std.mem.Allocator;
const Chunk = da_chunks.Chunk;
const Hash = da_chunks.Hash;

const blobChunk = da_chunks.blobChunk;
const calldataChunk = da_chunks.calldataChunk;
const chunkOf = da_chunks.chunkOf;
const concat = da_chunks.concat;
const expectAccepted = da_chunks.expectAccepted;
const expectRejected = da_chunks.expectRejected;
const foldOnto = da_chunks.foldOnto;
const insideFirstChunk = da_chunks.insideFirstChunk;
const packageFrame = da_chunks.packageFrame;
const segment = da_chunks.segment;

const gpa = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;
const capacity = blob_payload.payload_capacity;

const payload_1 = "conflation 1";
const payload_2 = "conflation 2";

const parent = da_chunks.parent;
const before_first_chunk = da_chunks.before_first_chunk;
const wrong_hash: Hash = @splat(0x30);

const foreign_prefix = "segments owned by the previous proof";
const foreign_suffix = "segments owned by the next proof";

// Offsets into a segment, whose first four bytes are its length prefix.
const inside_length_prefix = 2;
const inside_body = 7;

// ---- accepted layouts ------------------------------------------------------------------------

test "a blob holding one segment" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const blob = try blobChunk(a, try segment(a, payload_1));

    try expectAccepted(&.{blob}, &.{payload_1}, .{}, foldOnto(parent, &.{blob}), 0);
}

test "a shared blob with a foreign prefix and suffix hands its suffix to the next proof" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const own = try segment(a, payload_1);
    const blob = try blobChunk(a, try concat(a, &.{ foreign_prefix, own, foreign_suffix }));
    const take = own.len + foreign_suffix.len;

    // The parent hash already folds the shared blob, so nothing is folded again.
    try expectAccepted(&.{blob}, &.{payload_1}, insideFirstChunk(&.{blob}, take), foldOnto(before_first_chunk, &.{blob}), foreign_suffix.len);
}

test "a segment crosses a full blob into the next blob" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);

    for ([_]usize{ inside_length_prefix, inside_body }) |split| {
        const filler = try a.alloc(u8, capacity - split);
        @memset(filler, 0);
        const first = try blobChunk(a, try concat(a, &.{ filler, own[0..split] }));
        const second = try blobChunk(a, own[split..]);
        const chunks = [_]Chunk{ first, second };

        try expectAccepted(&chunks, &.{payload_1}, insideFirstChunk(&chunks, split), foldOnto(before_first_chunk, &chunks), 0);
    }
}

test "a segment crosses a short blob into calldata" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);

    for ([_]usize{ inside_length_prefix, inside_body }) |split| {
        const chunks = [_]Chunk{ try blobChunk(a, own[0..split]), calldataChunk(own[split..]) };

        try expectAccepted(&chunks, &.{payload_1}, .{}, foldOnto(parent, &chunks), 0);
    }
}

test "calldata finishes the blob's segment and carries a second segment" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);
    const second = try segment(a, payload_2);

    const chunks = [_]Chunk{ try blobChunk(a, first[0..inside_body]), calldataChunk(try concat(a, &.{ first[inside_body..], second })) };

    try expectAccepted(&chunks, &.{ payload_1, payload_2 }, .{}, foldOnto(parent, &chunks), 0);
}

test "calldata holds a whole segment after a short blob" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const chunks = [_]Chunk{ try blobChunk(a, try segment(a, payload_1)), calldataChunk(try segment(a, payload_2)) };

    try expectAccepted(&chunks, &.{ payload_1, payload_2 }, .{}, foldOnto(parent, &chunks), 0);
}

test "a shared blob is followed by calldata that finishes its last segment" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);

    const chunks = [_]Chunk{
        try blobChunk(a, try concat(a, &.{ foreign_prefix, own[0..inside_length_prefix] })),
        calldataChunk(own[inside_length_prefix..]),
    };

    try expectAccepted(&chunks, &.{payload_1}, insideFirstChunk(&chunks, inside_length_prefix), foldOnto(before_first_chunk, &chunks), 0);
}

test "calldata is followed by a blob" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const chunks = [_]Chunk{ calldataChunk(try segment(a, payload_1)), try blobChunk(a, try segment(a, payload_2)) };

    try expectAccepted(&chunks, &.{ payload_1, payload_2 }, .{}, foldOnto(parent, &chunks), 0);
}

test "a segment spans three blobs" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    // Incompressible, so the segment fills two blobs and spills into a third.
    var prng = std.Random.DefaultPrng.init(4844);
    const large = try a.alloc(u8, 2 * capacity);
    prng.random().bytes(large);
    const own = try segment(a, large);
    const chunks = [_]Chunk{
        try blobChunk(a, own[0..capacity]),
        try blobChunk(a, own[capacity .. 2 * capacity]),
        try blobChunk(a, own[2 * capacity ..]),
    };

    try expectAccepted(&chunks, &.{large}, .{}, foldOnto(parent, &chunks), 0);
}

test "a calldata-only stream carries no blob" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const calldata = calldataChunk(try segment(a, payload_1));

    try expectAccepted(&.{calldata}, &.{payload_1}, .{}, foldOnto(parent, &.{calldata}), 0);
}

test "several small conflations share one blob" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const payloads = [_][]const u8{ "first", "second conflation", "third", "a fourth one" };
    var stream: std.ArrayList(u8) = .empty;
    for (payloads) |payload| try stream.appendSlice(a, try segment(a, payload));
    const blob = try blobChunk(a, stream.items);

    try expectAccepted(&.{blob}, &payloads, .{}, foldOnto(parent, &.{blob}), 0);
}

test "segments the proof does not cover stay in the terminal blob as unconsumed bytes" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const next = try segment(a, payload_2);
    const blob = try blobChunk(a, try concat(a, &.{ try segment(a, payload_1), next }));

    try expectAccepted(&.{blob}, &.{payload_1}, .{}, foldOnto(parent, &.{blob}), next.len);
}

test "a positive tail count is accepted up to one byte below a full payload" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const own = try segment(a, payload_1);
    const payload = try a.alloc(u8, capacity);
    @memset(payload, 0);
    payload[0] = 0xaa;
    @memcpy(payload[1..][0..own.len], own);
    const blob = try blobChunk(a, payload);
    const take = capacity - 1;

    try expectAccepted(&.{blob}, &.{payload_1}, insideFirstChunk(&.{blob}, take), foldOnto(before_first_chunk, &.{blob}), take - own.len);
}

// ---- rejected streams ------------------------------------------------------------------------

test "no chunk at all is rejected" {
    try expectRejected(error.NoChunks, &.{}, &.{payload_1}, .{});
}

test "calldata that ends inside a segment is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);

    try expectRejected(error.CalldataEndsMidSegment, &.{ calldataChunk(first[0..5]), calldataChunk(first[5..]) }, &.{payload_1}, .{});
    try expectRejected(error.CalldataEndsMidSegment, &.{ calldataChunk(first[0..5]), try blobChunk(a, first[5..]) }, &.{payload_1}, .{});
    try expectRejected(
        error.CalldataEndsMidSegment,
        &.{ try blobChunk(a, first[0..5]), calldataChunk(first[5..10]), calldataChunk(first[10..]) },
        &.{payload_1},
        .{},
    );
}

test "terminal calldata may not leave bytes after the last segment" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);
    const second = try segment(a, payload_2);

    try expectRejected(error.CalldataEndsMidSegment, &.{calldataChunk(try concat(a, &.{ first, "extra" }))}, &.{payload_1}, .{});
    try expectRejected(error.CalldataEndsMidSegment, &.{calldataChunk(try concat(a, &.{ first, second[0..5] }))}, &.{payload_1}, .{});
    try expectRejected(error.CalldataEndsMidSegment, &.{calldataChunk(try concat(a, &.{ first, second }))}, &.{payload_1}, .{});
}

test "a blob one byte short of a full payload before another blob is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const payload = try a.alloc(u8, capacity - 1);
    @memset(payload, 0);
    @memcpy(payload[0..own.len], own);

    try expectRejected(error.PartialBlobBeforeBlob, &.{ try blobChunk(a, payload), try blobChunk(a, "tail") }, &.{payload_1}, .{});
}

test "every chunk has to own a byte inside the segments" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);
    const second = try segment(a, payload_2);

    // A blob past the last segment.
    const padded = try a.alloc(u8, capacity);
    @memset(padded, 0);
    @memcpy(padded[0..first.len], first);
    try expectRejected(error.ChunkOwnsNoBytes, &.{ try blobChunk(a, padded), try blobChunk(a, second) }, &.{payload_1}, .{});
    // An empty blob between two calldata chunks.
    try expectRejected(
        error.ChunkOwnsNoBytes,
        &.{ calldataChunk(first), try blobChunk(a, ""), calldataChunk(second) },
        &.{ payload_1, payload_2 },
        .{},
    );
    // An empty calldata submission.
    try expectRejected(error.ChunkOwnsNoBytes, &.{ calldataChunk(first), calldataChunk("") }, &.{payload_1}, .{});
    // No conflation to cover the chunk.
    try expectRejected(error.ChunkOwnsNoBytes, &.{calldataChunk(first)}, &.{}, .{});
}

test "a positive tail count needs a first blob with a longer payload" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);

    try expectRejected(error.InvalidTailTake, &.{calldataChunk(own)}, &.{payload_1}, .{ .take = 3 });
    const blob = try blobChunk(a, own);
    try expectRejected(error.InvalidTailTake, &.{blob}, &.{payload_1}, insideFirstChunk(&.{blob}, own.len));
    try expectRejected(error.InvalidTailTake, &.{blob}, &.{payload_1}, insideFirstChunk(&.{blob}, own.len + 1));

    const full = try a.alloc(u8, capacity);
    @memset(full, 0);
    const full_blob = try blobChunk(a, full);
    try expectRejected(error.InvalidTailTake, &.{full_blob}, &.{payload_1}, insideFirstChunk(&.{full_blob}, capacity));
}

test "a blob has to be the canonical packing of its payload" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);

    // An element of 2^254 or more.
    for ([_]usize{ 0, 1000, blob_payload.field_elements - 1 }) |element| {
        const chunk = try blobChunk(a, own);
        const forged: []u8 = try a.dupe(u8, chunk.blob_bytes.bytes);
        forged[element * 32] |= 0x40;
        var with_forged = chunk;
        with_forged.blob_bytes = .{ .bytes = forged };
        try expectRejected(error.NonCanonicalBlob, &.{with_forged}, &.{payload_1}, .{});
    }
    // A missing terminal: an all-zero blob, or a last nonzero byte other than 0xff.
    var chunk = try blobChunk(a, own);
    var forged: []u8 = try a.alloc(u8, blob_payload.blob_len);
    @memset(forged, 0);
    chunk.blob_bytes = .{ .bytes = forged };
    try expectRejected(error.NonCanonicalBlob, &.{chunk}, &.{payload_1}, .{});
    forged = try a.dupe(u8, (try blobChunk(a, own)).blob_bytes.bytes);
    forged[blob_payload.blob_len - 1] = 1;
    chunk.blob_bytes = .{ .bytes = forged };
    try expectRejected(error.NonCanonicalBlob, &.{chunk}, &.{payload_1}, .{});
}

test "a blob has to be the one its commitment commits to" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const other = try blobChunk(a, try segment(a, payload_2));
    const other_commitment = other.kzg_commitment.bytes[0..48].*;
    const other_proof = other.kzg_proof.bytes[0..48].*;

    // The blob of the stream under a commitment and a proof of another blob, anchored by the
    // versioned hash of that commitment: every binding holds, and the blob is not the committed one.
    const own_blob = (try blobChunk(a, own)).blob_bytes.bytes[0..blob_payload.blob_len];
    try expectRejected(error.BlobKzgMismatch, &.{try chunkOf(a, own_blob, other_commitment, other_proof)}, &.{payload_1}, .{});
}

test "the proof of every blob is verified" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    // Incompressible, so the segment fills two blobs and spills into a third.
    var prng = std.Random.DefaultPrng.init(4844);
    const large = try a.alloc(u8, 2 * capacity);
    prng.random().bytes(large);
    const own = try segment(a, large);
    const chunks = [_]Chunk{
        try blobChunk(a, own[0..capacity]),
        try blobChunk(a, own[capacity .. 2 * capacity]),
        try blobChunk(a, own[2 * capacity ..]),
    };

    for (0..chunks.len) |wrong| {
        var forged = chunks;
        forged[wrong].kzg_proof = chunks[(wrong + 1) % chunks.len].kzg_proof;
        try expectRejected(error.BlobKzgMismatch, &forged, &.{large}, .{});
    }
}

test "a chunk's bytes have to match its anchored hash" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);
    const second = try segment(a, payload_2);

    var blob = try blobChunk(a, first);
    blob.chunk_hash = wrong_hash;
    try expectRejected(error.ChunkHashMismatch, &.{blob}, &.{payload_1}, .{});

    blob = try blobChunk(a, first);
    blob.kzg_commitment = (try blobChunk(a, second)).kzg_commitment;
    try expectRejected(error.ChunkHashMismatch, &.{blob}, &.{payload_1}, .{});

    var calldata = calldataChunk(first);
    calldata.chunk_hash = wrong_hash;
    try expectRejected(error.ChunkHashMismatch, &.{calldata}, &.{payload_1}, .{});

    calldata = calldataChunk(first);
    calldata.calldata_bytes = .{ .bytes = second };
    try expectRejected(error.ChunkHashMismatch, &.{calldata}, &.{payload_1}, .{});
}

test "a chunk carries the fields of its own kind only" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const blob = try blobChunk(a, own);

    var carrying_calldata = blob;
    carrying_calldata.calldata_bytes = .{ .bytes = own };
    try expectRejected(error.MalformedChunk, &.{carrying_calldata}, &.{payload_1}, .{});

    var carrying_blob = calldataChunk(own);
    carrying_blob.blob_bytes = blob.blob_bytes;
    try expectRejected(error.MalformedChunk, &.{carrying_blob}, &.{payload_1}, .{});

    var carrying_commitment = calldataChunk(own);
    carrying_commitment.kzg_commitment = blob.kzg_commitment;
    try expectRejected(error.MalformedChunk, &.{carrying_commitment}, &.{payload_1}, .{});

    var carrying_proof = calldataChunk(own);
    carrying_proof.kzg_proof = blob.kzg_proof;
    try expectRejected(error.MalformedChunk, &.{carrying_proof}, &.{payload_1}, .{});
}

test "a blob has a 131072-byte body, a 48-byte commitment and a 48-byte proof" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const blob = try blobChunk(a, try segment(a, payload_1));

    var short_body = blob;
    short_body.blob_bytes = .{ .bytes = blob.blob_bytes.bytes[0 .. blob.blob_bytes.bytes.len - 1] };
    try expectRejected(error.MalformedChunk, &.{short_body}, &.{payload_1}, .{});

    var long_body = blob;
    long_body.blob_bytes = .{ .bytes = try concat(a, &.{ blob.blob_bytes.bytes, &.{0} }) };
    try expectRejected(error.MalformedChunk, &.{long_body}, &.{payload_1}, .{});

    var no_commitment = blob;
    no_commitment.kzg_commitment = .{ .bytes = &.{} };
    try expectRejected(error.MalformedChunk, &.{no_commitment}, &.{payload_1}, .{});

    var no_proof = blob;
    no_proof.kzg_proof = .{ .bytes = &.{} };
    try expectRejected(error.MalformedChunk, &.{no_proof}, &.{payload_1}, .{});

    var short_commitment = blob;
    short_commitment.kzg_commitment = .{ .bytes = blob.kzg_commitment.bytes[0..47] };
    try expectRejected(error.MalformedChunk, &.{short_commitment}, &.{payload_1}, .{});

    var long_proof = blob;
    long_proof.kzg_proof = .{ .bytes = try concat(a, &.{ blob.kzg_proof.bytes, &.{0} }) };
    try expectRejected(error.MalformedChunk, &.{long_proof}, &.{payload_1}, .{});
}

test "each segment starts with a nonzero length that stays inside the stream" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const frame = try packageFrame(a, payload_1);

    try expectRejected(error.MissingSegmentLength, &.{calldataChunk(&.{ 0x00, 0x01 })}, &.{payload_1}, .{});
    try expectRejected(error.InvalidSegmentLength, &.{calldataChunk(try da_stream.lengthPrefixed(a, &.{}))}, &.{payload_1}, .{});
    const too_long = try da_stream.lengthPrefixed(a, frame);
    std.mem.writeInt(u32, too_long[0..4], @intCast(frame.len + 1), .big);
    try expectRejected(error.InvalidSegmentLength, &.{calldataChunk(too_long)}, &.{payload_1}, .{});
    const huge = try da_stream.lengthPrefixed(a, frame);
    std.mem.writeInt(u32, huge[0..4], std.math.maxInt(u32), .big);
    try expectRejected(error.InvalidSegmentLength, &.{calldataChunk(huge)}, &.{payload_1}, .{});
}

test "a conflation without a segment in the stream is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const calldata = calldataChunk(try segment(a, payload_1));

    try expectRejected(error.MissingSegmentLength, &.{calldata}, &.{ payload_1, payload_2 }, .{});
}

test "a segment whose frame is outside the DA form is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const frame = try zstd_frames.compress(a, payload_1, .{ .dictionary = dictionary, .content_size = false });

    try expectRejected(error.InvalidFrame, &.{calldataChunk(try da_stream.lengthPrefixed(a, frame))}, &.{payload_1}, .{});
}

test "the segments of a stream are matched to the conflations in order" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const calldata = calldataChunk(try concat(a, &.{ try segment(a, payload_1), try segment(a, payload_2) }));

    try expectRejected(error.PayloadMismatch, &.{calldata}, &.{ payload_2, payload_1 }, .{});
}

test "a range starting inside a chunk has to open its parent hash" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const blob = try blobChunk(a, try concat(a, &.{ foreign_prefix, own }));

    var start = insideFirstChunk(&.{blob}, own.len);
    start.boundary = null;
    try expectRejected(error.BoundaryPreimageMismatch, &.{blob}, &.{payload_1}, start);
    start.boundary = wrong_hash;
    try expectRejected(error.BoundaryPreimageMismatch, &.{blob}, &.{payload_1}, start);
}
