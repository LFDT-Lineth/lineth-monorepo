//! Chunks of a DA stream, built from payloads and verified under the package dictionary, for the
//! tests of `decompress`.

const std = @import("std");

const lineth_da = @import("lineth_da");
const support = @import("support");
const zstd_encoder = @import("zstd_encoder");

const decompress = lineth_da.decompress;
const blob_payload = lineth_da.encoding.blob_payload;
const zstd_decoder = lineth_da.zstd_decoder;
const da_stream = support.da_stream;

const Allocator = std.mem.Allocator;
pub const Chunk = decompress.Chunk;
pub const Hash = decompress.Hash;

const gpa = std.testing.allocator;
const dictionary = lineth_da.dictionary_v2025_04_21;

pub const parent: Hash = @splat(0x10);
pub const before_first_chunk: Hash = @splat(0x20);

pub fn keccak(bytes: []const u8) Hash {
    var hash: Hash = undefined;
    std.crypto.hash.sha3.Keccak256.hash(bytes, &hash, .{});
    return hash;
}

/// The encoder of the package, which digests the dictionary once for every test of the process.
var encoder: ?zstd_encoder.Encoder = null;

/// The frame the package's encoder produces.
pub fn packageFrame(a: Allocator, payload: []const u8) ![]u8 {
    if (encoder == null) encoder = try zstd_encoder.Encoder.init(std.heap.page_allocator, dictionary);
    return encoder.?.compress(a, payload);
}

/// One stream segment: the length prefix and the frame of `payload`.
pub fn segment(a: Allocator, payload: []const u8) ![]u8 {
    return da_stream.lengthPrefixed(a, try packageFrame(a, payload));
}

pub fn concat(a: Allocator, parts: []const []const u8) ![]u8 {
    return std.mem.concat(a, u8, parts);
}

/// The blob chunk packing `payload`, with the commitment and the blob proof of c-kzg-4844.
pub fn blobChunk(a: Allocator, payload: []const u8) !Chunk {
    const blob = try a.create([blob_payload.blob_len]u8);
    try blob_payload.pack(payload, blob);
    const opening = support.kzg.open(blob);
    return chunkOf(a, blob, opening.commitment, opening.proof);
}

/// A blob chunk anchored by the versioned hash of `commitment`, whatever the blob and the proof.
pub fn chunkOf(a: Allocator, blob: *const [blob_payload.blob_len]u8, commitment: [48]u8, proof: [48]u8) !Chunk {
    const commitment_bytes = try a.create([48]u8);
    commitment_bytes.* = commitment;
    const proof_bytes = try a.create([48]u8);
    proof_bytes.* = proof;
    return .{
        .chunk_hash = da_stream.versionedHash(commitment_bytes),
        .is_calldata = false,
        .blob_bytes = blob,
        .kzg_commitment = commitment_bytes,
        .kzg_proof = proof_bytes,
    };
}

pub fn calldataChunk(bytes: []const u8) Chunk {
    return .{ .chunk_hash = keccak(bytes), .is_calldata = true, .calldata_bytes = bytes };
}

/// The dataRollingHash after folding `chunks` onto `from`.
pub fn foldOnto(from: Hash, chunks: []const Chunk) Hash {
    var hash = from;
    for (chunks) |chunk| hash = keccak(&(hash ++ chunk.chunk_hash));
    return hash;
}

pub const Start = struct {
    parent: Hash = parent,
    take: u64 = 0,
    boundary: ?Hash = null,
};

/// The start of a range inside the first chunk, which the parent hash already folds.
pub fn insideFirstChunk(chunks: []const Chunk, take: u64) Start {
    return .{ .parent = foldOnto(before_first_chunk, chunks[0..1]), .take = take, .boundary = before_first_chunk };
}

/// Verifies the stream under the package dictionary.
pub fn verify(chunks: []const Chunk, payloads: []const []const u8, start: Start) !decompress.Verified {
    const decoder = try zstd_decoder.Decoder.init(gpa, dictionary);
    defer decoder.deinit();
    return decompress.verifyDaChunks(gpa, decoder, chunks, payloads, start.parent, start.take, start.boundary);
}

pub fn expectAccepted(chunks: []const Chunk, payloads: []const []const u8, start: Start, end_hash: Hash, discard: u64) !void {
    const outcome = try verify(chunks, payloads, start);
    try std.testing.expectEqualSlices(u8, &end_hash, &outcome.end_data_rolling_hash);
    try std.testing.expectEqual(discard, outcome.final_data_tail_discard_bytes);
}

pub fn expectRejected(expected: anyerror, chunks: []const Chunk, payloads: []const []const u8, start: Start) !void {
    try std.testing.expectError(expected, verify(chunks, payloads, start));
}
