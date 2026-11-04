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
const Start = da_chunks.Start;

const blobChunk = da_chunks.blobChunk;
const calldataChunk = da_chunks.calldataChunk;
const chunkOf = da_chunks.chunkOf;
const concat = da_chunks.concat;
const expectAccepted = da_chunks.expectAccepted;
const expectRejected = da_chunks.expectRejected;
const foldOnto = da_chunks.foldOnto;
const insideFirstChunk = da_chunks.insideFirstChunk;
const keccak = da_chunks.keccak;
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

// ---- layouts built at random -----------------------------------------------------------------

test "streams cut into blobs and calldata at random verify" {
    var prng = std.Random.DefaultPrng.init(4844);
    const random = prng.random();

    for (0..60) |_| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        const a = arena.allocator();

        const count = 1 + random.uintLessThan(usize, 5);
        const payloads = try a.alloc([]const u8, count);
        const segment_ends = try a.alloc(usize, count);
        var stream: std.ArrayList(u8) = .empty;
        for (payloads, segment_ends) |*payload, *end| {
            const bytes = try a.alloc(u8, 1 + random.uintLessThan(usize, 400));
            random.bytes(bytes);
            payload.* = bytes;
            try stream.appendSlice(a, try segment(a, bytes));
            end.* = stream.items.len;
        }

        // Calldata ends at a segment boundary; a blob ends anywhere and, being short, is followed
        // by calldata or by nothing. The first blob may start with bytes of the previous proof and
        // the last one end with bytes of the next.
        var chunks: std.ArrayList(Chunk) = .empty;
        var take: u64 = 0;
        var discard: u64 = 0;
        var position: usize = 0;
        var previous_was_blob = false;
        while (position < stream.items.len) {
            const as_blob = !previous_was_blob and random.boolean();
            if (as_blob) {
                const end = position + 1 + random.uintLessThan(usize, stream.items.len - position);
                var content: std.ArrayList(u8) = .empty;
                const prefix_len = if (chunks.items.len == 0 and random.boolean()) 1 + random.uintLessThan(usize, 40) else 0;
                try content.appendNTimes(a, 0xee, prefix_len);
                try content.appendSlice(a, stream.items[position..end]);
                const suffix_len = if (end == stream.items.len and random.boolean()) 1 + random.uintLessThan(usize, 40) else 0;
                try content.appendNTimes(a, 0xdd, suffix_len);
                if (prefix_len > 0) take = content.items.len - prefix_len;
                discard = suffix_len;
                try chunks.append(a, try blobChunk(a, content.items));
                position = end;
            } else {
                var candidates: std.ArrayList(usize) = .empty;
                for (segment_ends) |end| if (end > position) try candidates.append(a, end);
                const end = candidates.items[random.uintLessThan(usize, candidates.items.len)];
                try chunks.append(a, calldataChunk(stream.items[position..end]));
                position = end;
            }
            previous_was_blob = as_blob;
        }

        const start: Start = if (take > 0) insideFirstChunk(chunks.items, take) else .{};
        const folded_from = if (take > 0) before_first_chunk else parent;
        try expectAccepted(chunks.items, payloads, start, foldOnto(folded_from, chunks.items), discard);
    }
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

test "a blob followed by another blob has to hold a full payload" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const chunks = [_]Chunk{ try blobChunk(a, try segment(a, payload_1)), try blobChunk(a, try segment(a, payload_2)) };

    try expectRejected(error.PartialBlobBeforeBlob, &chunks, &.{ payload_1, payload_2 }, .{});
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
        const forged: []u8 = try a.dupe(u8, chunk.blob_bytes);
        forged[element * 32] |= 0x40;
        var with_forged = chunk;
        with_forged.blob_bytes = forged;
        try expectRejected(error.NonCanonicalBlob, &.{with_forged}, &.{payload_1}, .{});
    }
    // A missing terminal: an all-zero blob, or a last nonzero byte other than 0xff.
    var chunk = try blobChunk(a, own);
    var forged: []u8 = try a.alloc(u8, blob_payload.blob_len);
    @memset(forged, 0);
    chunk.blob_bytes = forged;
    try expectRejected(error.NonCanonicalBlob, &.{chunk}, &.{payload_1}, .{});
    forged = try a.dupe(u8, (try blobChunk(a, own)).blob_bytes);
    forged[blob_payload.blob_len - 1] = 1;
    chunk.blob_bytes = forged;
    try expectRejected(error.NonCanonicalBlob, &.{chunk}, &.{payload_1}, .{});
}

test "a blob has to be the one its commitment commits to" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const other = try blobChunk(a, try segment(a, payload_2));
    const other_commitment = other.kzg_commitment[0..48].*;
    const other_proof = other.kzg_proof[0..48].*;

    // The blob of the stream under a commitment and a proof of another blob, anchored by the
    // versioned hash of that commitment: every binding holds, and the blob is not the committed one.
    const own_blob = (try blobChunk(a, own)).blob_bytes[0..blob_payload.blob_len];
    try expectRejected(error.BlobKzgMismatch, &.{try chunkOf(a, own_blob, other_commitment, other_proof)}, &.{payload_1}, .{});

    // The commitment of the blob under the proof of another blob.
    const genuine = try blobChunk(a, own);
    try expectRejected(error.BlobKzgMismatch, &.{try chunkOf(a, own_blob, genuine.kzg_commitment[0..48].*, other_proof)}, &.{payload_1}, .{});

    // A proof that is no curve point.
    try expectRejected(error.BlobKzgMismatch, &.{try chunkOf(a, own_blob, genuine.kzg_commitment[0..48].*, @splat(0xff))}, &.{payload_1}, .{});
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

test "blob proofs are checked after the chunk binding and before the layout" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const first = try segment(a, payload_1);
    const second = try segment(a, payload_2);

    // A short blob before a blob, the first of which carries the proof of the second.
    var short = try blobChunk(a, first);
    const next = try blobChunk(a, second);
    short.kzg_proof = next.kzg_proof;
    try expectRejected(error.BlobKzgMismatch, &.{ short, next }, &.{ payload_1, payload_2 }, .{});

    // The same, with a chunk of the second blob that is no canonical packing: the binding comes first.
    var forged = next;
    const bytes = try a.dupe(u8, next.blob_bytes);
    bytes[0] |= 0x40;
    forged.blob_bytes = bytes;
    try expectRejected(error.NonCanonicalBlob, &.{ short, forged }, &.{ payload_1, payload_2 }, .{});
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
    calldata.calldata_bytes = second;
    try expectRejected(error.ChunkHashMismatch, &.{calldata}, &.{payload_1}, .{});
}

test "a chunk carries the fields of its own kind only" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const own = try segment(a, payload_1);
    const blob = try blobChunk(a, own);

    var carrying_calldata = blob;
    carrying_calldata.calldata_bytes = own;
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
    short_body.blob_bytes = blob.blob_bytes[0 .. blob.blob_bytes.len - 1];
    try expectRejected(error.MalformedChunk, &.{short_body}, &.{payload_1}, .{});

    var long_body = blob;
    long_body.blob_bytes = try concat(a, &.{ blob.blob_bytes, &.{0} });
    try expectRejected(error.MalformedChunk, &.{long_body}, &.{payload_1}, .{});

    var no_commitment = blob;
    no_commitment.kzg_commitment = &.{};
    try expectRejected(error.MalformedChunk, &.{no_commitment}, &.{payload_1}, .{});

    var no_proof = blob;
    no_proof.kzg_proof = &.{};
    try expectRejected(error.MalformedChunk, &.{no_proof}, &.{payload_1}, .{});

    var short_commitment = blob;
    short_commitment.kzg_commitment = blob.kzg_commitment[0..47];
    try expectRejected(error.MalformedChunk, &.{short_commitment}, &.{payload_1}, .{});

    var long_proof = blob;
    long_proof.kzg_proof = try concat(a, &.{ blob.kzg_proof, &.{0} });
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

test "each segment is one complete frame of the DA form" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const frame = try packageFrame(a, payload_1);
    const other_frame = try packageFrame(a, payload_2);
    const other_dictionary = try zstd_frames.dictionaryWithId(a, dictionary, 0x1234_5678);

    const rejected_frames = [_][]const u8{
        "not a zstd frame",
        try concat(a, &.{ frame, "trailing" }),
        try concat(a, &.{ frame, other_frame }),
        frame[0 .. frame.len - 1],
        try zstd_frames.compress(a, payload_1, .{ .dictionary = dictionary, .content_size = false }),
        try packageFrame(a, payload_1 ++ "!"),
        try packageFrame(a, payload_1[0 .. payload_1.len - 1]),
        try zstd_frames.compress(a, payload_1, .{ .dictionary = dictionary, .dictionary_id = false }),
        try zstd_frames.compress(a, payload_1, .{ .dictionary = other_dictionary }),
        try zstd_frames.compress(a, payload_1, .{ .dictionary = dictionary, .checksum = true }),
    };
    for (rejected_frames) |rejected| {
        try expectRejected(error.InvalidFrame, &.{calldataChunk(try da_stream.lengthPrefixed(a, rejected))}, &.{payload_1}, .{});
    }
}

test "a frame has to decompress to its conflation's canonical payload" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const calldata = calldataChunk(try segment(a, "conflation X"));

    try expectRejected(error.PayloadMismatch, &.{calldata}, &.{payload_1}, .{});
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

test "a boundary hash is not consulted when the range starts at a chunk boundary" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    const blob = try blobChunk(a, try segment(a, payload_1));

    try expectAccepted(&.{blob}, &.{payload_1}, .{ .boundary = wrong_hash }, foldOnto(parent, &.{blob}), 0);
}

test "chunk binding is checked before the layout" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();

    // Both a short blob before a blob and a wrong hash: the hash is reported.
    var first = try blobChunk(a, try segment(a, payload_1));
    first.chunk_hash = wrong_hash;
    const chunks = [_]Chunk{ first, try blobChunk(a, try segment(a, payload_2)) };

    try expectRejected(error.ChunkHashMismatch, &chunks, &.{ payload_1, payload_2 }, .{});
}
