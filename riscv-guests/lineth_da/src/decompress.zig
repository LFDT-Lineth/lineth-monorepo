//! Verification of a rollup proof's data-availability stream against the chunks that carry it.
//!
//! The stream is the segments `[4-byte big-endian nonzero length][zstd frame]`, one per
//! conflation, in order. Its chunks are blobs (bound by the versioned hash of their KZG commitment
//! and the EIP-4844 proof of the blob against it) and calldata submissions (bound by keccak256). A
//! blob contributes its unpacked payload, a calldata chunk its bytes; the concatenation is the
//! stream, and each segment's frame decompresses, with the decoder's dictionary, to the canonical
//! payload of its conflation.

const std = @import("std");

const accelerators = @import("zesu_accelerators");
const blob_kzg = @import("encoding/blob_kzg.zig");
const blob_payload = @import("encoding/blob_payload.zig");
const rolling_hash = @import("encoding/rolling_hash.zig");
const zstd = @import("zstd.zig");

const Allocator = std.mem.Allocator;

pub const Hash = [32]u8;

pub const Error = error{
    OutOfMemory,

    // Binding of the chunks to their anchored hashes.

    /// The list of chunks is empty.
    NoChunks,
    /// A chunk's fields contradict its kind: a blob with calldata, a calldata chunk with a blob,
    /// commitment or proof, a blob body other than 131072 bytes, or a commitment or proof other
    /// than 48 bytes.
    MalformedChunk,
    /// A blob breaks the canonical packing of a payload: an element of 2^254 or more, or no 0xff
    /// terminal.
    NonCanonicalBlob,
    /// The versioned hash of a blob's commitment, or the keccak256 of a calldata chunk's bytes,
    /// differs from the chunk's hash.
    ChunkHashMismatch,
    /// A blob's EIP-4844 proof does not open its commitment to the blob.
    BlobKzgMismatch,
    /// `boundary_prev_data_rolling_hash` is missing or opens a hash other than the parent hash.
    BoundaryPreimageMismatch,

    // Layout of the stream across the chunks.

    /// A blob followed by another blob holds less than a full payload.
    PartialBlobBeforeBlob,
    /// A positive tail count with a calldata first chunk, or at least as long as the first blob's
    /// payload.
    InvalidTailTake,
    /// Fewer than four bytes remain for a segment's length prefix.
    MissingSegmentLength,
    /// A segment's length is zero or reaches past the end of the stream.
    InvalidSegmentLength,
    /// A chunk contains no byte of the parsed segments.
    ChunkOwnsNoBytes,
    /// A calldata chunk ends inside a segment.
    CalldataEndsMidSegment,

    // Decompression of the segments.

    /// A segment's bytes are other than one complete zstd frame that decodes with the decoder's
    /// dictionary.
    InvalidFrame,
    /// A frame decompresses to bytes other than its conflation's canonical payload.
    PayloadMismatch,
};

/// One chunk the proof touches, as the witness carries it.
pub const Chunk = struct {
    /// The anchored hash: the blob's versioned hash, or keccak256 of the calldata submission.
    chunk_hash: Hash,
    is_calldata: bool,
    /// The 131072 physical bytes of a blob; empty for calldata.
    blob_bytes: []const u8 = &.{},
    /// The exact calldata submission; empty for a blob.
    calldata_bytes: []const u8 = &.{},
    /// The 48-byte compressed G1 commitment of a blob; empty for calldata.
    kzg_commitment: []const u8 = &.{},
    /// The 48-byte EIP-4844 blob proof; empty for calldata.
    kzg_proof: []const u8 = &.{},
};

pub const Verified = struct {
    end_data_rolling_hash: Hash,
    /// The bytes left in the stream after the last segment.
    final_data_tail_discard_bytes: u64,
};

/// Verifies the DA stream carried by `chunks`: one segment per entry of `canonical_payloads`.
///
/// - Each chunk is bound to its `chunk_hash`: a blob has no calldata, a 48-byte commitment and
///   proof and the canonical packing of its payload, and the versioned hash of its commitment is
///   the `chunk_hash`; a calldata chunk has no blob, commitment or proof, and its keccak256 is the
///   `chunk_hash`. Once every chunk is bound, each blob's EIP-4844 proof opens its commitment to
///   the blob.
/// - Layout: a blob followed by another blob holds a full payload; a positive
///   `parent_data_tail_take_bytes` requires a first blob with a longer payload, and the stream
///   starts at the last `parent_data_tail_take_bytes` bytes of it; every chunk owns at least one
///   byte inside the parsed segments; every calldata chunk ends at a segment boundary.
/// - Every segment's frame decompresses exactly to its conflation's canonical payload.
/// - The dataRollingHash folds the chunks onto `parent_data_rolling_hash`; a positive tail count
///   starts the range inside the first chunk, which the parent hash includes, so
///   `boundary_prev_data_rolling_hash` has to open it.
///
/// `allocator` serves the working buffers, released before returning.
pub fn verifyDaChunks(
    allocator: Allocator,
    decoder: zstd.Decoder,
    chunks: []const Chunk,
    canonical_payloads: []const []const u8,
    parent_data_rolling_hash: Hash,
    parent_data_tail_take_bytes: u64,
    boundary_prev_data_rolling_hash: ?Hash,
) Error!Verified {
    if (chunks.len == 0) return error.NoChunks;

    var capacity: usize = 0;
    for (chunks) |chunk| {
        capacity += if (chunk.is_calldata) chunk.calldata_bytes.len else blob_payload.payload_capacity;
    }
    const joined = try allocator.alloc(u8, capacity);
    defer allocator.free(joined);
    const payload_lens = try allocator.alloc(usize, chunks.len);
    defer allocator.free(payload_lens);
    const unpacked = try allocator.create([blob_payload.stream_len]u8);
    defer allocator.destroy(unpacked);

    var joined_len: usize = 0;
    for (chunks, payload_lens) |chunk, *payload_len| {
        const payload = try bindChunk(chunk, unpacked);
        @memcpy(joined[joined_len..][0..payload.len], payload);
        joined_len += payload.len;
        payload_len.* = payload.len;
    }

    for (chunks) |chunk| {
        if (chunk.is_calldata) continue;
        const blob = chunk.blob_bytes[0..blob_kzg.blob_len];
        if (!blob_kzg.verify(blob, chunk.kzg_commitment[0..blob_kzg.commitment_len], chunk.kzg_proof[0..blob_kzg.proof_len])) {
            return error.BlobKzgMismatch;
        }
    }

    for (chunks[0 .. chunks.len - 1], chunks[1..], payload_lens[0 .. chunks.len - 1]) |chunk, following, payload_len| {
        if (!chunk.is_calldata and !following.is_calldata and payload_len != blob_payload.payload_capacity) {
            return error.PartialBlobBeforeBlob;
        }
    }

    var stream: []const u8 = joined[0..joined_len];
    var take: usize = 0;
    if (parent_data_tail_take_bytes > 0) {
        if (chunks[0].is_calldata or parent_data_tail_take_bytes >= payload_lens[0]) return error.InvalidTailTake;
        take = @intCast(parent_data_tail_take_bytes);
        stream = stream[payload_lens[0] - take ..];
        payload_lens[0] = take;
    }

    const segment_ends = try allocator.alloc(usize, canonical_payloads.len);
    defer allocator.free(segment_ends);
    const consumed = try parseSegments(allocator, decoder, stream, canonical_payloads, segment_ends);

    var chunk_start: usize = 0;
    for (chunks, payload_lens) |chunk, payload_len| {
        const chunk_end = chunk_start + payload_len;
        if (chunk_start >= @min(chunk_end, consumed)) return error.ChunkOwnsNoBytes;
        if (chunk.is_calldata and std.mem.indexOfScalar(usize, segment_ends, chunk_end) == null) {
            return error.CalldataEndsMidSegment;
        }
        chunk_start = chunk_end;
    }

    const chunk_hashes = try allocator.alloc(Hash, chunks.len);
    defer allocator.free(chunk_hashes);
    for (chunks, chunk_hashes) |chunk, *chunk_hash| chunk_hash.* = chunk.chunk_hash;
    return .{
        .end_data_rolling_hash = try rolling_hash.fold(parent_data_rolling_hash, chunk_hashes, take, boundary_prev_data_rolling_hash),
        .final_data_tail_discard_bytes = stream.len - consumed,
    };
}

/// Binds `chunk` to its hash and returns what it contributes to the stream. A blob's payload is
/// written into `unpacked`.
fn bindChunk(chunk: Chunk, unpacked: *[blob_payload.stream_len]u8) Error![]const u8 {
    if (chunk.is_calldata) {
        if (chunk.blob_bytes.len != 0 or chunk.kzg_commitment.len != 0 or chunk.kzg_proof.len != 0) return error.MalformedChunk;
        var hash: Hash = undefined;
        accelerators.keccak256(chunk.calldata_bytes, &hash);
        if (!std.mem.eql(u8, &hash, &chunk.chunk_hash)) return error.ChunkHashMismatch;
        return chunk.calldata_bytes;
    }

    if (chunk.calldata_bytes.len != 0 or
        chunk.kzg_commitment.len != blob_kzg.commitment_len or
        chunk.kzg_proof.len != blob_kzg.proof_len or
        chunk.blob_bytes.len != blob_kzg.blob_len) return error.MalformedChunk;
    const payload = blob_payload.unpack(chunk.blob_bytes[0..blob_kzg.blob_len], unpacked) catch return error.NonCanonicalBlob;
    if (!std.mem.eql(u8, &blob_kzg.versionedHash(chunk.kzg_commitment[0..blob_kzg.commitment_len]), &chunk.chunk_hash)) return error.ChunkHashMismatch;
    return payload;
}

/// Parses one segment per canonical payload from the front of `stream`, writing each segment's end
/// offset to `segment_ends`, and returns the offset where the last one ends.
fn parseSegments(
    allocator: Allocator,
    decoder: zstd.Decoder,
    stream: []const u8,
    canonical_payloads: []const []const u8,
    segment_ends: []usize,
) Error!usize {
    var longest: usize = 0;
    for (canonical_payloads) |canonical| longest = @max(longest, canonical.len);
    const decompressed = try allocator.alloc(u8, longest);
    defer allocator.free(decompressed);

    var cursor: usize = 0;
    for (canonical_payloads, segment_ends) |canonical, *segment_end| {
        if (stream.len - cursor < 4) return error.MissingSegmentLength;
        const length = std.mem.readInt(u32, stream[cursor..][0..4], .big);
        if (length == 0 or length > stream.len - cursor - 4) return error.InvalidSegmentLength;
        const frame = stream[cursor + 4 ..][0..length];
        const bytes = decoder.decompress(frame, decompressed[0..canonical.len]) catch |err| return switch (err) {
            error.OutOfMemory => error.OutOfMemory,
            error.OutputTooLarge => error.PayloadMismatch,
            else => error.InvalidFrame,
        };
        if (!std.mem.eql(u8, bytes, canonical)) return error.PayloadMismatch;
        cursor += 4 + length;
        segment_end.* = cursor;
    }
    return cursor;
}
