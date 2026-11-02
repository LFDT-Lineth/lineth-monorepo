//! Wire codec of the rollup guest's input: the `RollupProofPrivateInput` container behind the
//! 0x1001 schema id. The frame is the 2-byte big-endian schema id followed by the SSZ container
//! (SSZ itself is little-endian). The decoder accepts only the canonical encoding and the byte-list
//! fields alias the decoded buffer.
//!
//! The already-proven l2-execution proofs the input embeds are core_serde's contract, and so is the
//! guest's output (`core_serde.rollup_proof`).
//!
//! A list of variable-size elements (`conflations`, `chunks`, `l2_execution_proofs`, a conflation's
//! `block_rlps`) is an offset table over its region (`guest_common.ssz.decodeVariableList`); a list
//! of fixed-size elements (`boundary_prev_data_rolling_hash`) is the plain concatenation of its
//! elements.
//!
//! Fixed-head sizes (variable-size fields sit behind 4-byte offsets):
//!   RollupProofPrivateInput:  64 bytes
//!   ConflationWitness:         4 bytes
//!   ChunkWitness:             49 bytes

const std = @import("std");
const core_serde = @import("core_serde");
const lineth_da = @import("lineth_da");
const ssz = @import("guest_common").ssz;

const blob_kzg = lineth_da.encoding.blob_kzg;
const l2_execution_proof = core_serde.l2_execution_proof;
const rollup_proof = core_serde.rollup_proof;

pub const INPUT_SCHEMA_ID: u16 = 0x1001;
const SCHEMA_ID_SIZE: usize = 2;

// List bounds of the wire schema: encoders and decoders reject longer lists.
/// Conflations one rollup proof covers.
pub const MAX_CONFLATIONS_PER_ROLLUP: usize = 1 << 10;
/// Paired one to one with the conflations.
pub const MAX_L2_EXECUTION_PROOFS_PER_ROLLUP: usize = rollup_proof.MAX_L2_EXECUTION_PROOFS_PER_ROLLUP;
/// Chunks one rollup proof touches.
pub const MAX_CHUNKS_PER_ROLLUP: usize = 1 << 12;
/// Full block RLPs (one per block) of one conflation.
pub const MAX_BLOCK_RLPS_PER_CONFLATION: usize = 1 << 12;
/// 16 MiB: a full canonical block RLP including all transaction bodies.
pub const MAX_BYTES_PER_BLOCK_RLP: usize = 1 << 24;
/// 16 MiB: a single calldata submission.
pub const MAX_CALLDATA_BYTES_PER_CHUNK: usize = 1 << 24;
/// An EIP-4844 blob: 4096 field elements of 32 bytes.
pub const BLOB_BYTES_LENGTH = lineth_da.encoding.blob_payload.blob_len;
pub const BYTES_PER_COMMITMENT = blob_kzg.commitment_len;
pub const BYTES_PER_PROOF = blob_kzg.proof_len;

// ── Logical values ────────────────────────────────────────────────────────────

pub const ConflationWitness = struct {
    /// The canonical full block RLPs of the conflation, one per block.
    block_rlps: []const []const u8,
};

/// One touched chunk: its anchored hash, its declared kind and the bytes it is checked against. A
/// blob carries its physical bytes, commitment and proof; calldata its exact submission.
pub const ChunkWitness = lineth_da.decompress.Chunk;

pub const RollupProofPrivateInput = struct {
    parent_data_rolling_hash: [32]u8,
    parent_data_tail_take_bytes: u64,
    chain_id: u64,
    conflations: []const ConflationWitness,
    chunks: []const ChunkWitness,
    l2_execution_proofs: []const l2_execution_proof.VerifiableL2ExecutionProof,
    /// The wire list `boundary_prev_data_rolling_hash` holds at most one element: absent is the
    /// empty list, present the single hash.
    boundary_prev_data_rolling_hash: ?[32]u8,
};

const readU32 = ssz.readU32;
const writeU32 = ssz.writeU32;
const putHash = ssz.putHash;
const putU64 = ssz.putU64;
const putOffset = ssz.putOffset;
const getHash = ssz.getHash;
const getU64 = ssz.getU64;
const getOffset = ssz.getOffset;

// ── ConflationWitness ─────────────────────────────────────────────────────────
// Fixed head: block_rlps offset(4).
const CONFLATION_FIXED_SIZE: usize = 4;

fn decodeConflationWitness(alloc: std.mem.Allocator, bytes: []const u8) !ConflationWitness {
    if (bytes.len < CONFLATION_FIXED_SIZE) return error.InvalidSsz;
    if (readU32(bytes, 0) != CONFLATION_FIXED_SIZE) return error.InvalidSsz;
    const block_rlps = try ssz.decodeVariableList(alloc, bytes[CONFLATION_FIXED_SIZE..], MAX_BLOCK_RLPS_PER_CONFLATION);
    for (block_rlps) |block_rlp| {
        if (block_rlp.len > MAX_BYTES_PER_BLOCK_RLP) return error.BoundsViolation;
    }
    return .{ .block_rlps = block_rlps };
}

fn encodeConflationWitness(alloc: std.mem.Allocator, v: ConflationWitness) ![]u8 {
    if (v.block_rlps.len > MAX_BLOCK_RLPS_PER_CONFLATION) return error.BoundsViolation;
    for (v.block_rlps) |block_rlp| {
        if (block_rlp.len > MAX_BYTES_PER_BLOCK_RLP) return error.BoundsViolation;
    }
    const block_rlps = try ssz.encodeVariableList(alloc, v.block_rlps);
    const out = try alloc.alloc(u8, CONFLATION_FIXED_SIZE + block_rlps.len);
    writeU32(out, 0, CONFLATION_FIXED_SIZE);
    @memcpy(out[CONFLATION_FIXED_SIZE..], block_rlps);
    return out;
}

// ── ChunkWitness ──────────────────────────────────────────────────────────────
// Fixed head: chunk_hash(32) + is_calldata(1) + four byte-list offsets(4 each) = 49.
const CHUNK_FIXED_SIZE: usize = 32 + 1 + 4 * 4;

fn decodeChunkWitness(bytes: []const u8) !ChunkWitness {
    if (bytes.len < CHUNK_FIXED_SIZE) return error.InvalidSsz;
    var pos: usize = 0;
    const chunk_hash = getHash(bytes, &pos);
    const is_calldata = switch (bytes[pos]) {
        0 => false,
        1 => true,
        else => return error.InvalidSsz,
    };
    pos += 1;
    const blob_start = getOffset(bytes, &pos);
    const calldata_start = getOffset(bytes, &pos);
    const commitment_start = getOffset(bytes, &pos);
    const proof_start = getOffset(bytes, &pos);
    std.debug.assert(pos == CHUNK_FIXED_SIZE);

    // The first tail starts right behind the fixed head and every tail ends where the next begins.
    if (blob_start != CHUNK_FIXED_SIZE or calldata_start < blob_start or commitment_start < calldata_start or
        proof_start < commitment_start or proof_start > bytes.len) return error.InvalidSsz;

    const blob_bytes = bytes[blob_start..calldata_start];
    const calldata_bytes = bytes[calldata_start..commitment_start];
    const kzg_commitment = bytes[commitment_start..proof_start];
    const kzg_proof = bytes[proof_start..];
    if (blob_bytes.len > BLOB_BYTES_LENGTH or calldata_bytes.len > MAX_CALLDATA_BYTES_PER_CHUNK or
        kzg_commitment.len > BYTES_PER_COMMITMENT or kzg_proof.len > BYTES_PER_PROOF) return error.BoundsViolation;

    return .{
        .chunk_hash = chunk_hash,
        .is_calldata = is_calldata,
        .blob_bytes = blob_bytes,
        .calldata_bytes = calldata_bytes,
        .kzg_commitment = kzg_commitment,
        .kzg_proof = kzg_proof,
    };
}

fn encodeChunkWitness(alloc: std.mem.Allocator, v: ChunkWitness) ![]u8 {
    if (v.blob_bytes.len > BLOB_BYTES_LENGTH or v.calldata_bytes.len > MAX_CALLDATA_BYTES_PER_CHUNK or
        v.kzg_commitment.len > BYTES_PER_COMMITMENT or v.kzg_proof.len > BYTES_PER_PROOF) return error.BoundsViolation;

    const calldata_start = CHUNK_FIXED_SIZE + v.blob_bytes.len;
    const commitment_start = calldata_start + v.calldata_bytes.len;
    const proof_start = commitment_start + v.kzg_commitment.len;
    const out = try alloc.alloc(u8, proof_start + v.kzg_proof.len);

    var pos: usize = 0;
    putHash(out, &pos, v.chunk_hash);
    out[pos] = @intFromBool(v.is_calldata);
    pos += 1;
    putOffset(out, &pos, CHUNK_FIXED_SIZE);
    putOffset(out, &pos, calldata_start);
    putOffset(out, &pos, commitment_start);
    putOffset(out, &pos, proof_start);
    std.debug.assert(pos == CHUNK_FIXED_SIZE);
    @memcpy(out[CHUNK_FIXED_SIZE..][0..v.blob_bytes.len], v.blob_bytes);
    @memcpy(out[calldata_start..][0..v.calldata_bytes.len], v.calldata_bytes);
    @memcpy(out[commitment_start..][0..v.kzg_commitment.len], v.kzg_commitment);
    @memcpy(out[proof_start..], v.kzg_proof);
    return out;
}

// ── RollupProofPrivateInput ───────────────────────────────────────────────────
// Fixed head: parent_data_rolling_hash(32) + parent_data_tail_take_bytes(8) + chain_id(8) + four
// list offsets(4 each) = 64.
const INPUT_FIXED_SIZE: usize = 32 + 8 + 8 + 4 * 4;

/// Decode the rollup guest input: the 0x1001 schema id followed by the SSZ container. A missing or
/// different schema id is `error.MalformedFrame`; a container that is not the canonical encoding is
/// `error.InvalidSsz`; a list or byte field beyond its bound is `error.BoundsViolation`.
pub fn decodeInput(alloc: std.mem.Allocator, data: []const u8) !RollupProofPrivateInput {
    if (data.len < SCHEMA_ID_SIZE) return error.MalformedFrame;
    if (std.mem.readInt(u16, data[0..SCHEMA_ID_SIZE], .big) != INPUT_SCHEMA_ID) return error.MalformedFrame;

    const body = data[SCHEMA_ID_SIZE..];
    if (body.len < INPUT_FIXED_SIZE) return error.InvalidSsz;

    var pos: usize = 0;
    const parent_data_rolling_hash = getHash(body, &pos);
    const parent_data_tail_take_bytes = getU64(body, &pos);
    const chain_id = getU64(body, &pos);
    const conflations_start = getOffset(body, &pos);
    const chunks_start = getOffset(body, &pos);
    const proofs_start = getOffset(body, &pos);
    const boundary_start = getOffset(body, &pos);
    std.debug.assert(pos == INPUT_FIXED_SIZE);

    // The first tail starts right behind the fixed head and every tail ends where the next begins.
    if (conflations_start != INPUT_FIXED_SIZE or chunks_start < conflations_start or proofs_start < chunks_start or
        boundary_start < proofs_start or boundary_start > body.len) return error.InvalidSsz;

    const conflation_regions = try ssz.decodeVariableList(alloc, body[conflations_start..chunks_start], MAX_CONFLATIONS_PER_ROLLUP);
    const conflations = try alloc.alloc(ConflationWitness, conflation_regions.len);
    for (conflations, conflation_regions) |*conflation, region| conflation.* = try decodeConflationWitness(alloc, region);

    const chunk_regions = try ssz.decodeVariableList(alloc, body[chunks_start..proofs_start], MAX_CHUNKS_PER_ROLLUP);
    const chunks = try alloc.alloc(ChunkWitness, chunk_regions.len);
    for (chunks, chunk_regions) |*chunk, region| chunk.* = try decodeChunkWitness(region);

    const proof_regions = try ssz.decodeVariableList(alloc, body[proofs_start..boundary_start], MAX_L2_EXECUTION_PROOFS_PER_ROLLUP);
    const l2_execution_proofs = try alloc.alloc(l2_execution_proof.VerifiableL2ExecutionProof, proof_regions.len);
    for (l2_execution_proofs, proof_regions) |*proof, region| proof.* = try l2_execution_proof.decodeVerifiableProof(alloc, region);

    const boundary = try ssz.decodeFixedList([32]u8, body[boundary_start..], 1);

    return .{
        .parent_data_rolling_hash = parent_data_rolling_hash,
        .parent_data_tail_take_bytes = parent_data_tail_take_bytes,
        .chain_id = chain_id,
        .conflations = conflations,
        .chunks = chunks,
        .l2_execution_proofs = l2_execution_proofs,
        .boundary_prev_data_rolling_hash = if (boundary.len == 1) boundary[0] else null,
    };
}

/// Encode the rollup guest input: inverse of `decodeInput`. `alloc` is an arena that owns the
/// scratch buffers of the regions.
pub fn encodeInput(alloc: std.mem.Allocator, v: RollupProofPrivateInput) ![]u8 {
    if (v.conflations.len > MAX_CONFLATIONS_PER_ROLLUP or
        v.chunks.len > MAX_CHUNKS_PER_ROLLUP or
        v.l2_execution_proofs.len > MAX_L2_EXECUTION_PROOFS_PER_ROLLUP) return error.BoundsViolation;

    const conflation_regions = try alloc.alloc([]const u8, v.conflations.len);
    for (conflation_regions, v.conflations) |*region, conflation| region.* = try encodeConflationWitness(alloc, conflation);
    const conflations = try ssz.encodeVariableList(alloc, conflation_regions);

    const chunk_regions = try alloc.alloc([]const u8, v.chunks.len);
    for (chunk_regions, v.chunks) |*region, chunk| region.* = try encodeChunkWitness(alloc, chunk);
    const chunks = try ssz.encodeVariableList(alloc, chunk_regions);

    const proof_regions = try alloc.alloc([]const u8, v.l2_execution_proofs.len);
    for (proof_regions, v.l2_execution_proofs) |*region, proof| region.* = try l2_execution_proof.encodeVerifiableProof(alloc, proof);
    const proofs = try ssz.encodeVariableList(alloc, proof_regions);

    const boundary_len: usize = if (v.boundary_prev_data_rolling_hash == null) 0 else 32;

    const chunks_start = INPUT_FIXED_SIZE + conflations.len;
    const proofs_start = chunks_start + chunks.len;
    const boundary_start = proofs_start + proofs.len;
    const body_len = boundary_start + boundary_len;
    // Every offset is a u32.
    if (body_len > std.math.maxInt(u32)) return error.InvalidSsz;

    const out = try alloc.alloc(u8, SCHEMA_ID_SIZE + body_len);
    std.mem.writeInt(u16, out[0..SCHEMA_ID_SIZE], INPUT_SCHEMA_ID, .big);
    const body = out[SCHEMA_ID_SIZE..];

    var pos: usize = 0;
    putHash(body, &pos, v.parent_data_rolling_hash);
    putU64(body, &pos, v.parent_data_tail_take_bytes);
    putU64(body, &pos, v.chain_id);
    putOffset(body, &pos, INPUT_FIXED_SIZE);
    putOffset(body, &pos, chunks_start);
    putOffset(body, &pos, proofs_start);
    putOffset(body, &pos, boundary_start);
    std.debug.assert(pos == INPUT_FIXED_SIZE);
    @memcpy(body[INPUT_FIXED_SIZE..][0..conflations.len], conflations);
    @memcpy(body[chunks_start..][0..chunks.len], chunks);
    @memcpy(body[proofs_start..][0..proofs.len], proofs);
    if (v.boundary_prev_data_rolling_hash) |hash| @memcpy(body[boundary_start..][0..32], &hash);
    return out;
}
