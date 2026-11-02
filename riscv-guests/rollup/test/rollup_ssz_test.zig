//! Tests for the rollup guest's input codec: the byte layout of the frame, round-trips through
//! `encodeInput`/`decodeInput`, and rejection of every non-canonical encoding and of every list
//! or byte field beyond its bound.

const std = @import("std");
const rollup_ssz = @import("rollup").rollup_ssz;
const l2_execution_proof = @import("core_serde").l2_execution_proof;
const guest_errors = @import("rollup").guest_errors;
const ssz = @import("guest_common").ssz;
const support = @import("support");

const repeat32 = support.repeat32;

fn le32(value: u32) [4]u8 {
    var out: [4]u8 = undefined;
    std.mem.writeInt(u32, &out, value, .little);
    return out;
}

fn le64(value: u64) [8]u8 {
    var out: [8]u8 = undefined;
    std.mem.writeInt(u64, &out, value, .little);
    return out;
}

// Positions of the fixed head inside a frame: the 2-byte schema id, then the hash, the tail count,
// the chain id and the four list offsets.
const head_size = 64;
const conflations_offset_pos = 2 + 48;
const chunks_offset_pos = 2 + 52;
const proofs_offset_pos = 2 + 56;
const boundary_offset_pos = 2 + 60;
const chunk_head_size = 49;
const chunk_blob_offset_pos = 33;

fn expectDecodeError(want: anyerror, bytes: []const u8) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(want, rollup_ssz.decodeInput(arena.allocator(), bytes));
}

// A frame assembled from the bytes of its four list regions, behind the fixed head of the sample's
// hash, tail count and chain id.
fn frameOf(alloc: std.mem.Allocator, conflations: []const u8, chunks: []const u8, proofs: []const u8, boundary: []const u8) ![]u8 {
    const chunks_start = head_size + conflations.len;
    const proofs_start = chunks_start + chunks.len;
    const boundary_start = proofs_start + proofs.len;
    const out = try alloc.alloc(u8, 2 + boundary_start + boundary.len);
    std.mem.writeInt(u16, out[0..2], 0x1001, .big);
    @memcpy(out[2..34], &support.PARENT_DATA_ROLLING_HASH);
    std.mem.writeInt(u64, out[34..42], support.PARENT_DATA_TAIL_TAKE_BYTES, .little);
    std.mem.writeInt(u64, out[42..50], support.CHAIN_ID, .little);
    std.mem.writeInt(u32, out[conflations_offset_pos..][0..4], head_size, .little);
    std.mem.writeInt(u32, out[chunks_offset_pos..][0..4], @intCast(chunks_start), .little);
    std.mem.writeInt(u32, out[proofs_offset_pos..][0..4], @intCast(proofs_start), .little);
    std.mem.writeInt(u32, out[boundary_offset_pos..][0..4], @intCast(boundary_start), .little);
    @memcpy(out[2 + head_size ..][0..conflations.len], conflations);
    @memcpy(out[2 + chunks_start ..][0..chunks.len], chunks);
    @memcpy(out[2 + proofs_start ..][0..proofs.len], proofs);
    @memcpy(out[2 + boundary_start ..], boundary);
    return out;
}

// The offset table of `count` elements that are all empty.
fn emptyElements(alloc: std.mem.Allocator, count: usize) ![]u8 {
    const table = try alloc.alloc(u8, count * 4);
    for (0..count) |i| std.mem.writeInt(u32, table[i * 4 ..][0..4], @intCast(count * 4), .little);
    return table;
}

// A chunk assembled from its fields behind the fixed head of four offsets.
fn chunkOf(alloc: std.mem.Allocator, is_calldata: u8, blob: []const u8, calldata: []const u8, commitment: []const u8, proof: []const u8) ![]u8 {
    const calldata_start = chunk_head_size + blob.len;
    const commitment_start = calldata_start + calldata.len;
    const proof_start = commitment_start + commitment.len;
    const out = try alloc.alloc(u8, proof_start + proof.len);
    @memcpy(out[0..32], &support.CALLDATA_CHUNK_HASH);
    out[32] = is_calldata;
    std.mem.writeInt(u32, out[chunk_blob_offset_pos..][0..4], chunk_head_size, .little);
    std.mem.writeInt(u32, out[chunk_blob_offset_pos + 4 ..][0..4], @intCast(calldata_start), .little);
    std.mem.writeInt(u32, out[chunk_blob_offset_pos + 8 ..][0..4], @intCast(commitment_start), .little);
    std.mem.writeInt(u32, out[chunk_blob_offset_pos + 12 ..][0..4], @intCast(proof_start), .little);
    @memcpy(out[chunk_head_size..][0..blob.len], blob);
    @memcpy(out[calldata_start..][0..calldata.len], calldata);
    @memcpy(out[commitment_start..][0..commitment.len], commitment);
    @memcpy(out[proof_start..], proof);
    return out;
}

// A frame whose only list is the chunks `chunk_regions`.
fn frameOfChunks(alloc: std.mem.Allocator, chunk_regions: []const []const u8) ![]u8 {
    return frameOf(alloc, &.{}, try ssz.encodeVariableList(alloc, chunk_regions), &.{}, &.{});
}

// A frame whose only list is the one conflation of `block_rlps_region`.
fn frameOfBlockRlps(alloc: std.mem.Allocator, block_rlps_region: []const u8) ![]u8 {
    const head = le32(4);
    const conflation = try std.mem.concat(alloc, u8, &.{ &head, block_rlps_region });
    return frameOf(alloc, try ssz.encodeVariableList(alloc, &.{conflation}), &.{}, &.{}, &.{});
}

// ── Layout ────────────────────────────────────────────────────────────────────

// The input with no conflation, chunk or proof and no boundary hash: the schema id and the fixed head.
const empty_input: rollup_ssz.RollupProofPrivateInput = .{
    .parent_data_rolling_hash = support.PARENT_DATA_ROLLING_HASH,
    .parent_data_tail_take_bytes = support.PARENT_DATA_TAIL_TAKE_BYTES,
    .chain_id = support.CHAIN_ID,
    .conflations = &.{},
    .chunks = &.{},
    .l2_execution_proofs = &.{},
    .boundary_prev_data_rolling_hash = null,
};

const empty_frame = [_]u8{ 0x10, 0x01 } ++ support.PARENT_DATA_ROLLING_HASH ++ le64(support.PARENT_DATA_TAIL_TAKE_BYTES) ++
    le64(support.CHAIN_ID) ++ le32(head_size) ++ le32(head_size) ++ le32(head_size) ++ le32(head_size);

test "encodeInput: an input without lists is the schema id and the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectEqualSlices(u8, &empty_frame, try rollup_ssz.encodeInput(arena.allocator(), empty_input));
}

test "decodeInput: the fixed head without lists decodes to an input without lists" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectEqualDeep(empty_input, try rollup_ssz.decodeInput(arena.allocator(), &empty_frame));
}

// One conflation of two blocks, one calldata chunk, no proof, and a boundary hash.
const layout_block_rlp_0 = [_]u8{ 0xaa, 0xbb };
const layout_block_rlp_1 = [_]u8{0xcc};
const layout_calldata = [_]u8{ 0xde, 0xad };

const layout_input: rollup_ssz.RollupProofPrivateInput = .{
    .parent_data_rolling_hash = support.PARENT_DATA_ROLLING_HASH,
    .parent_data_tail_take_bytes = support.PARENT_DATA_TAIL_TAKE_BYTES,
    .chain_id = support.CHAIN_ID,
    .conflations = &.{.{ .block_rlps = &.{ &layout_block_rlp_0, &layout_block_rlp_1 } }},
    .chunks = &.{.{ .chunk_hash = support.CALLDATA_CHUNK_HASH, .is_calldata = true, .calldata_bytes = &layout_calldata }},
    .l2_execution_proofs = &.{},
    .boundary_prev_data_rolling_hash = support.BOUNDARY_PREV_DATA_ROLLING_HASH,
};

// Conflation: the block_rlps offset(4), then the offset table of the two blocks (8, 10) and their
// bytes. Chunk: the hash, the flag and the four byte-list offsets (49, 49, 51, 51), then the
// calldata. Each list is behind its own offset table.
const conflation_region = le32(4) ++ le32(8) ++ le32(10) ++ layout_block_rlp_0 ++ layout_block_rlp_1;
const chunk_region = support.CALLDATA_CHUNK_HASH ++ [_]u8{1} ++ le32(49) ++ le32(49) ++ le32(51) ++ le32(51) ++ layout_calldata;
const conflations_list = le32(4) ++ conflation_region;
const chunks_list = le32(4) ++ chunk_region;
const layout_frame = [_]u8{ 0x10, 0x01 } ++ support.PARENT_DATA_ROLLING_HASH ++ le64(support.PARENT_DATA_TAIL_TAKE_BYTES) ++
    le64(support.CHAIN_ID) ++
    le32(head_size) ++ le32(head_size + conflations_list.len) ++
    le32(head_size + conflations_list.len + chunks_list.len) ++ le32(head_size + conflations_list.len + chunks_list.len) ++
    conflations_list ++ chunks_list ++ support.BOUNDARY_PREV_DATA_ROLLING_HASH;

test "encodeInput: lays the conflations, chunks and boundary hash out behind their offsets" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectEqualSlices(u8, &layout_frame, try rollup_ssz.encodeInput(arena.allocator(), layout_input));
}

test "decodeInput: reads the conflations, chunks and boundary hash from behind their offsets" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectEqualDeep(layout_input, try rollup_ssz.decodeInput(arena.allocator(), &layout_frame));
}

test "decodeInput: the byte-list fields alias the frame" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    var frame = layout_frame;
    const decoded = try rollup_ssz.decodeInput(arena.allocator(), &frame);
    const frame_start = @intFromPtr(&frame);
    const frame_end = frame_start + frame.len;
    for ([_][]const u8{ decoded.conflations[0].block_rlps[0], decoded.chunks[0].calldata_bytes }) |bytes| {
        try std.testing.expect(@intFromPtr(bytes.ptr) >= frame_start and @intFromPtr(bytes.ptr) + bytes.len <= frame_end);
    }
}

// ── Round-trips ───────────────────────────────────────────────────────────────

test "encodeInput/decodeInput: round-trips every field of a readable sample input" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const original = try support.sampleInput(alloc);
    const encoded = try rollup_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x10, 0x01 }, encoded[0..2]);
    try std.testing.expectEqualDeep(original, try rollup_ssz.decodeInput(alloc, encoded));
}

test "decodeInput: the sample's blob chunk keeps its 131072 bytes, commitment and proof" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const decoded = try rollup_ssz.decodeInput(alloc, try rollup_ssz.encodeInput(alloc, try support.sampleInput(alloc)));
    const blob_chunk = decoded.chunks[0];
    try std.testing.expect(!blob_chunk.is_calldata);
    try std.testing.expectEqualSlices(u8, &support.BLOB_CHUNK_HASH, &blob_chunk.chunk_hash);
    try std.testing.expectEqualSlices(u8, try support.sampleBlob(alloc), blob_chunk.blob_bytes);
    try std.testing.expectEqual(@as(usize, 0), blob_chunk.calldata_bytes.len);
    try std.testing.expectEqualSlices(u8, &support.BLOB_KZG_COMMITMENT, blob_chunk.kzg_commitment);
    try std.testing.expectEqualSlices(u8, &support.BLOB_KZG_PROOF, blob_chunk.kzg_proof);

    const calldata_chunk = decoded.chunks[1];
    try std.testing.expect(calldata_chunk.is_calldata);
    try std.testing.expectEqualSlices(u8, &support.CALLDATA_BYTES, calldata_chunk.calldata_bytes);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.blob_bytes.len);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.kzg_commitment.len);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.kzg_proof.len);
}

test "encodeInput/decodeInput: an absent boundary hash round-trips as absent" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var original = try support.sampleInput(alloc);
    original.boundary_prev_data_rolling_hash = null;
    const decoded = try rollup_ssz.decodeInput(alloc, try rollup_ssz.encodeInput(alloc, original));
    try std.testing.expectEqual(@as(?[32]u8, null), decoded.boundary_prev_data_rolling_hash);
}

// ── Malformed frames ──────────────────────────────────────────────────────────

test "decodeInput: rejects a frame under another schema id or without one" {
    var frame = empty_frame;
    frame[0] = 0x18; // the output schema id
    try expectDecodeError(error.MalformedFrame, &frame);
    frame[0] = 0x10;
    frame[1] = 0x02;
    try expectDecodeError(error.MalformedFrame, &frame);
    try expectDecodeError(error.MalformedFrame, &.{});
    try expectDecodeError(error.MalformedFrame, empty_frame[0..1]);
}

test "decodeInput: rejects every body shorter than the fixed head" {
    for (2..2 + head_size) |len| try expectDecodeError(error.InvalidSsz, empty_frame[0..len]);
}

fn sampleFrame(alloc: std.mem.Allocator) ![]u8 {
    return rollup_ssz.encodeInput(alloc, try support.sampleInput(alloc));
}

fn withOffset(alloc: std.mem.Allocator, pos: usize, value: u32) ![]u8 {
    const frame = try sampleFrame(alloc);
    std.mem.writeInt(u32, frame[pos..][0..4], value, .little);
    return frame;
}

test "decodeInput: rejects bytes between the fixed head and the first list" {
    var frame = empty_frame ++ [_]u8{0} ** 4;
    for ([_]usize{ conflations_offset_pos, chunks_offset_pos, proofs_offset_pos, boundary_offset_pos }) |pos| {
        std.mem.writeInt(u32, frame[pos..][0..4], head_size + 4, .little);
    }
    try expectDecodeError(error.InvalidSsz, &frame);
}

test "decodeInput: rejects a first list offset that is not the fixed-head size" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, conflations_offset_pos, head_size + 4));
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, conflations_offset_pos, head_size - 4));
}

test "decodeInput: rejects list offsets that run backwards" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const frame = try sampleFrame(alloc);
    const chunks_start = std.mem.readInt(u32, frame[chunks_offset_pos..][0..4], .little);
    const proofs_start = std.mem.readInt(u32, frame[proofs_offset_pos..][0..4], .little);

    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, chunks_offset_pos, head_size - 1));
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, proofs_offset_pos, chunks_start - 1));
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, boundary_offset_pos, proofs_start - 1));
}

test "decodeInput: rejects a list offset past the end of the frame" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const body_len: u32 = @intCast((try sampleFrame(alloc)).len - 2);
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, boundary_offset_pos, body_len + 1));
    try expectDecodeError(error.InvalidSsz, try withOffset(alloc, boundary_offset_pos, 0xFFFFFFFF));
}

test "decodeInput: rejects a boundary region that is not one whole hash or none" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, &.{}, &.{}, &.{}, &[_]u8{0x39}));
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, &.{}, &.{}, &.{}, &([_]u8{0x39} ** 31)));
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, &.{}, &.{}, &.{}, &([_]u8{0x39} ** 33)));
    try expectDecodeError(error.BoundsViolation, try frameOf(alloc, &.{}, &.{}, &.{}, &([_]u8{0x39} ** 64)));
}

test "decodeInput: rejects a conflation shorter than its fixed head or with another block-list offset" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const too_short = try ssz.encodeVariableList(alloc, &.{&[_]u8{ 4, 0, 0 }});
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, too_short, &.{}, &.{}, &.{}));

    const other_head = comptime le32(8) ++ le32(4);
    const other_offset = try ssz.encodeVariableList(alloc, &.{&other_head});
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, other_offset, &.{}, &.{}, &.{}));
}

test "decodeInput: rejects a conflation whose block list is not a canonical offset table" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const zero_first = le32(0);
    const misaligned_first = le32(6);
    const backwards = comptime le32(8) ++ le32(4);
    try expectDecodeError(error.InvalidSsz, try frameOfBlockRlps(alloc, &zero_first));
    try expectDecodeError(error.InvalidSsz, try frameOfBlockRlps(alloc, &misaligned_first));
    try expectDecodeError(error.InvalidSsz, try frameOfBlockRlps(alloc, &backwards));
}

test "decodeInput: rejects a chunk with a non-canonical flag" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{try chunkOf(alloc, 2, &.{}, &.{}, &.{}, &.{})}));
    // The canonical flags decode.
    var decoded = try rollup_ssz.decodeInput(alloc, try frameOfChunks(alloc, &.{try chunkOf(alloc, 0, &.{}, &.{}, &.{}, &.{})}));
    try std.testing.expect(!decoded.chunks[0].is_calldata);
    decoded = try rollup_ssz.decodeInput(alloc, try frameOfChunks(alloc, &.{try chunkOf(alloc, 1, &.{}, &.{}, &.{}, &.{})}));
    try std.testing.expect(decoded.chunks[0].is_calldata);
}

test "decodeInput: rejects a chunk shorter than its fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const chunk = try chunkOf(alloc, 1, &.{}, &.{}, &.{}, &.{});
    for (0..chunk_head_size) |len| try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{chunk[0..len]}));
}

test "decodeInput: rejects a chunk whose byte-list offsets are not the canonical sequence" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const chunk = try chunkOf(alloc, 0, &[_]u8{ 1, 2, 3 }, &[_]u8{ 4, 5 }, &[_]u8{6}, &[_]u8{7});
    const first_offset_pos = chunk_blob_offset_pos;
    const calldata_offset_pos = chunk_blob_offset_pos + 4;
    const commitment_offset_pos = chunk_blob_offset_pos + 8;
    const proof_offset_pos = chunk_blob_offset_pos + 12;

    // The first offset is not the fixed-head size, an offset runs backwards, or one is past the end.
    const first_wrong = try alloc.dupe(u8, chunk);
    std.mem.writeInt(u32, first_wrong[first_offset_pos..][0..4], chunk_head_size + 1, .little);
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{first_wrong}));

    const calldata_backwards = try alloc.dupe(u8, chunk);
    std.mem.writeInt(u32, calldata_backwards[calldata_offset_pos..][0..4], chunk_head_size - 1, .little);
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{calldata_backwards}));

    const commitment_backwards = try alloc.dupe(u8, chunk);
    std.mem.writeInt(u32, commitment_backwards[commitment_offset_pos..][0..4], chunk_head_size + 2, .little);
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{commitment_backwards}));

    const proof_backwards = try alloc.dupe(u8, chunk);
    std.mem.writeInt(u32, proof_backwards[proof_offset_pos..][0..4], chunk_head_size + 3 + 2 - 1, .little);
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{proof_backwards}));

    const proof_past_end = try alloc.dupe(u8, chunk);
    std.mem.writeInt(u32, proof_past_end[proof_offset_pos..][0..4], @intCast(chunk.len + 1), .little);
    try expectDecodeError(error.InvalidSsz, try frameOfChunks(alloc, &.{proof_past_end}));
}

test "decodeInput: rejects a malformed l2-execution proof" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const proofs = try ssz.encodeVariableList(alloc, &.{&([_]u8{0} ** 10)});
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, &.{}, &.{}, proofs, &.{}));
}

test "decodeInput: rejects a proof whose own encoding is not canonical" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const proof = try l2_execution_proof.encodeVerifiableProof(alloc, try support.sampleProof0(alloc));
    std.mem.writeInt(u32, proof[0..4], 40, .little);
    const proofs = try ssz.encodeVariableList(alloc, &.{proof});
    try expectDecodeError(error.InvalidSsz, try frameOf(alloc, &.{}, &.{}, proofs, &.{}));
}

// ── Bounds ────────────────────────────────────────────────────────────────────

test "the bounds are those of the wire schema" {
    try std.testing.expectEqual(@as(usize, 1024), rollup_ssz.MAX_CONFLATIONS_PER_ROLLUP);
    try std.testing.expectEqual(@as(usize, 1024), rollup_ssz.MAX_L2_EXECUTION_PROOFS_PER_ROLLUP);
    try std.testing.expectEqual(@as(usize, 4096), rollup_ssz.MAX_CHUNKS_PER_ROLLUP);
    try std.testing.expectEqual(@as(usize, 4096), rollup_ssz.MAX_BLOCK_RLPS_PER_CONFLATION);
    try std.testing.expectEqual(@as(usize, 16 * 1024 * 1024), rollup_ssz.MAX_BYTES_PER_BLOCK_RLP);
    try std.testing.expectEqual(@as(usize, 16 * 1024 * 1024), rollup_ssz.MAX_CALLDATA_BYTES_PER_CHUNK);
    try std.testing.expectEqual(@as(usize, 131072), rollup_ssz.BLOB_BYTES_LENGTH);
    try std.testing.expectEqual(@as(usize, 48), rollup_ssz.BYTES_PER_COMMITMENT);
    try std.testing.expectEqual(@as(usize, 48), rollup_ssz.BYTES_PER_PROOF);
}

test "decodeInput: accepts the conflations at their bound and rejects one more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const max = rollup_ssz.MAX_CONFLATIONS_PER_ROLLUP;

    const conflation = le32(4);
    const at_bound = try alloc.alloc([]const u8, max);
    @memset(at_bound, &conflation);
    const decoded = try rollup_ssz.decodeInput(alloc, try frameOf(alloc, try ssz.encodeVariableList(alloc, at_bound), &.{}, &.{}, &.{}));
    try std.testing.expectEqual(max, decoded.conflations.len);

    try expectDecodeError(error.BoundsViolation, try frameOf(alloc, try emptyElements(alloc, max + 1), &.{}, &.{}, &.{}));
}

test "decodeInput: accepts the chunks at their bound and rejects one more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const max = rollup_ssz.MAX_CHUNKS_PER_ROLLUP;

    const chunk = try chunkOf(alloc, 1, &.{}, &.{}, &.{}, &.{});
    const at_bound = try alloc.alloc([]const u8, max);
    @memset(at_bound, chunk);
    const decoded = try rollup_ssz.decodeInput(alloc, try frameOfChunks(alloc, at_bound));
    try std.testing.expectEqual(max, decoded.chunks.len);

    try expectDecodeError(error.BoundsViolation, try frameOf(alloc, &.{}, try emptyElements(alloc, max + 1), &.{}, &.{}));
}

test "decodeInput: accepts the l2-execution proofs at their bound and rejects one more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const max = rollup_ssz.MAX_L2_EXECUTION_PROOFS_PER_ROLLUP;

    const proof = try l2_execution_proof.encodeVerifiableProof(alloc, try support.sampleProof0(alloc));
    const at_bound = try alloc.alloc([]const u8, max);
    @memset(at_bound, proof);
    const decoded = try rollup_ssz.decodeInput(alloc, try frameOf(alloc, &.{}, &.{}, try ssz.encodeVariableList(alloc, at_bound), &.{}));
    try std.testing.expectEqual(max, decoded.l2_execution_proofs.len);

    try expectDecodeError(error.BoundsViolation, try frameOf(alloc, &.{}, &.{}, try emptyElements(alloc, max + 1), &.{}));
}

test "decodeInput: accepts the blocks of a conflation at their bound and rejects one more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const max = rollup_ssz.MAX_BLOCK_RLPS_PER_CONFLATION;

    const decoded = try rollup_ssz.decodeInput(alloc, try frameOfBlockRlps(alloc, try emptyElements(alloc, max)));
    try std.testing.expectEqual(max, decoded.conflations[0].block_rlps.len);

    try expectDecodeError(error.BoundsViolation, try frameOfBlockRlps(alloc, try emptyElements(alloc, max + 1)));
}

test "decodeInput: accepts a block RLP at its bound and rejects one byte more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const max = rollup_ssz.MAX_BYTES_PER_BLOCK_RLP;
    const block_rlp = try alloc.alloc(u8, max + 1);
    @memset(block_rlp, 0xbb);

    const at_bound = try ssz.encodeVariableList(alloc, &.{block_rlp[0..max]});
    const decoded = try rollup_ssz.decodeInput(alloc, try frameOfBlockRlps(alloc, at_bound));
    try std.testing.expectEqual(max, decoded.conflations[0].block_rlps[0].len);

    const over_bound = try ssz.encodeVariableList(alloc, &.{block_rlp});
    try expectDecodeError(error.BoundsViolation, try frameOfBlockRlps(alloc, over_bound));
}

test "decodeInput: accepts each byte list of a chunk at its bound and rejects one byte more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();
    const filler = try alloc.alloc(u8, rollup_ssz.MAX_CALLDATA_BYTES_PER_CHUNK + 1);
    @memset(filler, 0x5a);

    const blob_max = rollup_ssz.BLOB_BYTES_LENGTH;
    const calldata_max = rollup_ssz.MAX_CALLDATA_BYTES_PER_CHUNK;
    const commitment_max = rollup_ssz.BYTES_PER_COMMITMENT;
    const proof_max = rollup_ssz.BYTES_PER_PROOF;

    const at_bound = try rollup_ssz.decodeInput(alloc, try frameOfChunks(alloc, &.{
        try chunkOf(alloc, 0, filler[0..blob_max], filler[0..calldata_max], filler[0..commitment_max], filler[0..proof_max]),
    }));
    try std.testing.expectEqual(blob_max, at_bound.chunks[0].blob_bytes.len);
    try std.testing.expectEqual(calldata_max, at_bound.chunks[0].calldata_bytes.len);
    try std.testing.expectEqual(commitment_max, at_bound.chunks[0].kzg_commitment.len);
    try std.testing.expectEqual(proof_max, at_bound.chunks[0].kzg_proof.len);

    const over_blob = try chunkOf(alloc, 0, filler[0 .. blob_max + 1], &.{}, &.{}, &.{});
    try expectDecodeError(error.BoundsViolation, try frameOfChunks(alloc, &.{over_blob}));
    const over_calldata = try chunkOf(alloc, 1, &.{}, filler[0 .. calldata_max + 1], &.{}, &.{});
    try expectDecodeError(error.BoundsViolation, try frameOfChunks(alloc, &.{over_calldata}));
    const over_commitment = try chunkOf(alloc, 0, &.{}, &.{}, filler[0 .. commitment_max + 1], &.{});
    try expectDecodeError(error.BoundsViolation, try frameOfChunks(alloc, &.{over_commitment}));
    const over_proof = try chunkOf(alloc, 0, &.{}, &.{}, &.{}, filler[0 .. proof_max + 1]);
    try expectDecodeError(error.BoundsViolation, try frameOfChunks(alloc, &.{over_proof}));
}

test "encodeInput: rejects lists and byte fields beyond their bound" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var too_many_conflations = empty_input;
    too_many_conflations.conflations = try alloc.alloc(rollup_ssz.ConflationWitness, rollup_ssz.MAX_CONFLATIONS_PER_ROLLUP + 1);
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, too_many_conflations));

    var too_many_chunks = empty_input;
    too_many_chunks.chunks = try alloc.alloc(rollup_ssz.ChunkWitness, rollup_ssz.MAX_CHUNKS_PER_ROLLUP + 1);
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, too_many_chunks));

    var too_many_proofs = empty_input;
    too_many_proofs.l2_execution_proofs = try alloc.alloc(l2_execution_proof.VerifiableL2ExecutionProof, rollup_ssz.MAX_L2_EXECUTION_PROOFS_PER_ROLLUP + 1);
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, too_many_proofs));

    var too_many_blocks = empty_input;
    too_many_blocks.conflations = &.{.{ .block_rlps = try alloc.alloc([]const u8, rollup_ssz.MAX_BLOCK_RLPS_PER_CONFLATION + 1) }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, too_many_blocks));

    const oversized_bytes = try alloc.alloc(u8, rollup_ssz.MAX_BYTES_PER_BLOCK_RLP + 1);
    var oversized_block = empty_input;
    oversized_block.conflations = &.{.{ .block_rlps = &.{oversized_bytes} }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, oversized_block));

    const hash = repeat32(0);
    var oversized_blob = empty_input;
    oversized_blob.chunks = &.{.{ .chunk_hash = hash, .is_calldata = false, .blob_bytes = oversized_bytes[0 .. rollup_ssz.BLOB_BYTES_LENGTH + 1] }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, oversized_blob));

    var oversized_calldata = empty_input;
    oversized_calldata.chunks = &.{.{ .chunk_hash = hash, .is_calldata = true, .calldata_bytes = oversized_bytes }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, oversized_calldata));

    var oversized_commitment = empty_input;
    oversized_commitment.chunks = &.{.{ .chunk_hash = hash, .is_calldata = false, .kzg_commitment = oversized_bytes[0 .. rollup_ssz.BYTES_PER_COMMITMENT + 1] }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, oversized_commitment));

    var oversized_proof = empty_input;
    oversized_proof.chunks = &.{.{ .chunk_hash = hash, .is_calldata = false, .kzg_proof = oversized_bytes[0 .. rollup_ssz.BYTES_PER_PROOF + 1] }};
    try std.testing.expectError(error.BoundsViolation, rollup_ssz.encodeInput(alloc, oversized_proof));
}

// ── Exit codes ────────────────────────────────────────────────────────────────

fn exitCodeOfDecoding(bytes: []const u8) !guest_errors.ExitCode {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    if (rollup_ssz.decodeInput(arena.allocator(), bytes)) |_| return error.DecodedMalformedInput else |err| return guest_errors.exitCode(err);
}

test "decoding failures exit with the code of their category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var wrong_schema = empty_frame;
    wrong_schema[0] = 0x18;
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_FRAME, try exitCodeOfDecoding(&wrong_schema));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_SSZ, try exitCodeOfDecoding(empty_frame[0 .. empty_frame.len - 1]));
    try std.testing.expectEqual(guest_errors.CODE_BOUNDS_VIOLATION, try exitCodeOfDecoding(try frameOf(alloc, &.{}, &.{}, &.{}, &([_]u8{0x39} ** 64))));
}
