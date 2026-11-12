//! Tests for the rollup guest's input codec: the schema-id framing, the round trip of the input
//! with each optional part present and absent, and the exit codes of the decode errors. The SSZ
//! rules are the generic codec's and are tested with it.

const std = @import("std");
const rollup_ssz = @import("rollup").rollup_ssz;
const guest_errors = @import("rollup").guest_errors;
const support = @import("support");

fn expectDecodeError(want: anyerror, bytes: []const u8) !void {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(want, rollup_ssz.decodeInput(arena.allocator(), bytes));
}

test "decodeInput: rejects a frame under another schema id or without one" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var frame = try rollup_ssz.encodeInput(alloc, try support.sampleInput(alloc));
    frame[1] = 0x05; // the rollup output's schema id
    try expectDecodeError(error.MalformedFrame, frame);
    frame[1] = 0x06; // the aggregation input's schema id
    try expectDecodeError(error.MalformedFrame, frame);
    try expectDecodeError(error.MalformedFrame, &.{});
    try expectDecodeError(error.MalformedFrame, frame[0..1]);
}

test "encodeInput/decodeInput: round-trips every field of a readable sample input" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const original = try support.sampleInput(alloc);
    const encoded = try rollup_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x04 }, encoded[0..2]);
    try std.testing.expectEqualDeep(original, try rollup_ssz.decodeInput(alloc, encoded));
}

test "encodeInput/decodeInput: an absent boundary hash round-trips as absent" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var original = try support.sampleInput(alloc);
    original.boundary_prev_data_rolling_hash = .{ .items = &.{} };
    const decoded = try rollup_ssz.decodeInput(alloc, try rollup_ssz.encodeInput(alloc, original));
    try std.testing.expectEqual(@as(usize, 0), decoded.boundary_prev_data_rolling_hash.items.len);
    try std.testing.expectEqualDeep(original, decoded);
}

test "encodeInput/decodeInput: an input without conflations, chunks and proofs round-trips" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var original = try support.sampleInput(alloc);
    original.conflations = .{ .items = &.{} };
    original.chunks = .{ .items = &.{} };
    original.l2_execution_proofs = .{ .items = &.{} };
    try std.testing.expectEqualDeep(original, try rollup_ssz.decodeInput(alloc, try rollup_ssz.encodeInput(alloc, original)));
}

test "decodeInput: the sample's blob and calldata chunks keep their fields" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const decoded = try rollup_ssz.decodeInput(alloc, try rollup_ssz.encodeInput(alloc, try support.sampleInput(alloc)));
    const blob_chunk = decoded.chunks.items[0];
    try std.testing.expect(!blob_chunk.is_calldata.value);
    try std.testing.expectEqualSlices(u8, &support.BLOB_CHUNK_HASH, &blob_chunk.chunk_hash);
    try std.testing.expectEqualSlices(u8, try support.sampleBlob(alloc), blob_chunk.blob_bytes.bytes);
    try std.testing.expectEqual(@as(usize, 0), blob_chunk.calldata_bytes.bytes.len);
    try std.testing.expectEqualSlices(u8, &support.BLOB_KZG_COMMITMENT, blob_chunk.kzg_commitment.bytes);
    try std.testing.expectEqualSlices(u8, &support.BLOB_KZG_PROOF, blob_chunk.kzg_proof.bytes);

    const calldata_chunk = decoded.chunks.items[1];
    try std.testing.expect(calldata_chunk.is_calldata.value);
    try std.testing.expectEqualSlices(u8, &support.CALLDATA_BYTES, calldata_chunk.calldata_bytes.bytes);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.blob_bytes.bytes.len);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.kzg_commitment.bytes.len);
    try std.testing.expectEqual(@as(usize, 0), calldata_chunk.kzg_proof.bytes.len);
}

fn exitCodeOfDecoding(bytes: []const u8) !guest_errors.ExitCode {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    if (rollup_ssz.decodeInput(arena.allocator(), bytes)) |_| return error.DecodedMalformedInput else |err| return guest_errors.exitCode(err);
}

test "decoding failures exit with the code of their category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const frame = try rollup_ssz.encodeInput(alloc, try support.sampleInput(alloc));
    const wrong_schema = try alloc.dupe(u8, frame);
    wrong_schema[1] = 0x05;
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_FRAME, try exitCodeOfDecoding(wrong_schema));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_SSZ, try exitCodeOfDecoding(frame[0 .. frame.len - 1]));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_SSZ, try exitCodeOfDecoding(try std.mem.concat(alloc, u8, &.{ frame, &.{0} })));
}
