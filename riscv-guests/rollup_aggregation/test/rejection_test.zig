//! The requests the rollup-aggregation guest rejects: every rule of the statement broken in an
//! otherwise valid chain of rollup proofs that carry real devmode proofs, with the error it is
//! rejected by and the exit-code category of that error; and the requests at the edge of an output
//! bound, which it accepts.

const std = @import("std");
const core_serde = @import("core_serde");
const devmode_prover = @import("devmode_prover");
const rollup_aggregation = @import("rollup_aggregation");
const scenario = @import("scenario");

const Allocator = std.mem.Allocator;
const guest_errors = rollup_aggregation.guest_errors;
const rollup_proof = core_serde.rollup_proof;
const rollup_ssz = rollup_aggregation.rollup_aggregation_ssz;
const run = rollup_aggregation.rollup_aggregation.run;
const runFramed = rollup_aggregation.rollup_aggregation.runFramed;

/// Runs `proofs` as they are and expects them rejected by `expected`, which belongs to the category
/// `code`.
fn expectRejectedAsIs(expected: anyerror, code: guest_errors.ExitCode, a: Allocator, proofs: []const rollup_proof.VerifiableRollupProof) !void {
    try std.testing.expectError(expected, run(a, scenario.input(proofs)));
    try std.testing.expectEqual(code, guest_errors.exitCode(expected));
}

/// As `expectRejectedAsIs`, with the proofs valid for the public inputs the test left them with.
fn expectRejected(expected: anyerror, code: guest_errors.ExitCode, a: Allocator, proofs: []rollup_proof.VerifiableRollupProof) !void {
    try scenario.prove(a, proofs);
    try expectRejectedAsIs(expected, code, a, proofs);
}

/// Runs the framed request `frame` and expects it rejected by `expected`, which belongs to the category
/// `code`.
fn expectFramedRejected(expected: anyerror, code: guest_errors.ExitCode, a: Allocator, frame: []const u8) !void {
    try std.testing.expectError(expected, runFramed(a, frame));
    try std.testing.expectEqual(code, guest_errors.exitCode(expected));
}

/// The framed request of two valid proofs.
fn framedRequest(a: Allocator) ![]u8 {
    return rollup_ssz.encodeInput(a, try scenario.request(a, &.{ .{}, .{} }));
}

// ── The shape of the request ─────────────────────────────────────────────────────────────────────

test "a request without proofs is malformed" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    try expectRejectedAsIs(error.InvalidRequest, guest_errors.CODE_MALFORMED_REQUEST, a, &.{});
}

test "a frame of another schema id is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const frame = try framedRequest(a);
    frame[1] = @truncate(rollup_ssz.OUTPUT_SCHEMA_ID);
    try expectFramedRejected(error.MalformedFrame, guest_errors.CODE_MALFORMED_FRAME, a, frame);
}

test "a frame without a schema id is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    try expectFramedRejected(error.MalformedFrame, guest_errors.CODE_MALFORMED_FRAME, a, &.{});
}

test "a frame cut inside its body is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const frame = try framedRequest(a);
    try expectFramedRejected(error.InvalidSsz, guest_errors.CODE_MALFORMED_SSZ, a, frame[0 .. frame.len / 2]);
}

test "a frame whose container offset points past its end is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const frame = try framedRequest(a);
    @memset(frame[2..6], 0xff);
    try expectFramedRejected(error.InvalidSsz, guest_errors.CODE_MALFORMED_SSZ, a, frame);
}

// ── Recursion ────────────────────────────────────────────────────────────────────────────────────

test "a proof of another public input is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    try scenario.prove(a, proofs);
    _ = try run(a, scenario.input(proofs));

    proofs[0].proof.public_inputs.end_block_timestamp += 1;
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

test "a proof of another program VK is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    try scenario.prove(a, proofs);
    _ = try run(a, scenario.input(proofs));

    proofs[0].program_vk[0] ^= 0x01;
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

test "a proof image with a changed round commitment is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    try scenario.prove(a, proofs);
    const image = try a.dupe(u8, proofs[1].proof.proof.bytes);
    image[try devmode_prover.commitmentOffset(a, image)] ^= 0x01;
    proofs[1].proof.proof = .{ .bytes = image };
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

test "a truncated proof image is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    try scenario.prove(a, proofs);
    const image = proofs[0].proof.proof.bytes;
    proofs[0].proof.proof = .{ .bytes = image[0 .. image.len / 2] };
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

test "an empty proof image is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    try scenario.prove(a, proofs);
    proofs[0].proof.proof = .{ .bytes = &.{} };
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

test "a proof whose public input is beyond a bound of its wire schema is rejected by the recursive verifier" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{}});
    proofs[0].proof.public_inputs.l2_messaging_blocks_offsets = .{ .items = try a.alloc(u16, rollup_proof.MAX_MESSAGING_BLOCKS_OFFSETS + 1) };
    try expectRejectedAsIs(error.RecursiveProofRejected, guest_errors.CODE_RECURSION, a, proofs);
}

// ── Tiling ───────────────────────────────────────────────────────────────────────────────────────

test "proofs that leave a gap between their block ranges are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.start_block_number += 1;
    try expectRejected(error.TilingBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "proofs whose block ranges overlap are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.start_block_number -= 1;
    try expectRejected(error.TilingBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

// ── Continuity ───────────────────────────────────────────────────────────────────────────────────

test "a proof that does not start from the data rolling hash its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_data_rolling_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "a proof that does not start from the shared-blob tail its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_shared_blob_carry_over_tail_bytes += 1;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "a proof that does not start from the block hash its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_block_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "proofs with different chain configurations are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.dynamic_chain_config_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "a proof that does not start from the forced-transaction rolling hash its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_ftx_rolling_hash[0] ^= 0x01;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

test "a proof that does not start from the processed forced-transaction number its predecessor ends with is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[1].proof.public_inputs.parent_ftx_number += 1;
    try expectRejected(error.ContinuityBroken, guest_errors.CODE_TILING_CONTINUITY, a, proofs);
}

// ── The messaging-block offsets ──────────────────────────────────────────────────────────────────

test "a proof whose block count differs from its range is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ .{}, .{} });
    proofs[0].proof.public_inputs.block_count += 1;
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, proofs);
}

test "an offset of zero is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{ .offsets = &.{0} }});
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, proofs);
}

test "an offset beyond the proof's blocks is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const blocks = 3;
    const proofs = try scenario.build(a, &.{.{ .block_count = blocks, .offsets = &.{ 1, blocks + 1 } }});
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, proofs);
}

test "offsets that do not increase are rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{.{ .offsets = &.{ 2, 2 } }});
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, proofs);
}

const max_offset = std.math.maxInt(u16);
const near_limit: scenario.Spec = .{ .block_count = max_offset - 4 };

test "an offset that rebases to 16 bits is accepted" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const pi = try run(a, try scenario.request(a, &.{ near_limit, .{ .offsets = &.{max_offset - near_limit.block_count} } }));
    try std.testing.expectEqualDeep(&[_]u16{max_offset}, pi.l2_messaging_blocks_offsets.items);
}

test "an offset that rebases beyond 16 bits is rejected" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const proofs = try scenario.build(a, &.{ near_limit, .{ .offsets = &.{max_offset - near_limit.block_count + 1} } });
    try expectRejected(error.InvalidMessagingOffsets, guest_errors.CODE_MESSAGING_OFFSETS, a, proofs);
}

// ── Bounds and resources ─────────────────────────────────────────────────────────────────────────

const max_program_ids = rollup_proof.MAX_PROGRAM_VKS;

/// `count` distinct program VKs in ascending order.
fn distinctVks(a: Allocator, count: usize) ![]const [32]u8 {
    const vks = try a.alloc([32]u8, count);
    for (vks, 0..) |*vk, i| {
        vk.* = @splat(0);
        std.mem.writeInt(u32, vk[0..4], @intCast(i), .big);
    }
    return vks;
}

test "more distinct program IDs than the output holds are rejected by the output encoder" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const request = try scenario.request(a, &.{.{ .program_vks = try distinctVks(a, max_program_ids) }});
    try expectFramedRejected(error.OutputEncodeFailed, guest_errors.CODE_OUTPUT_ENCODE_FAILED, a, try rollup_ssz.encodeInput(a, request));
}

test "as many distinct program IDs as the output holds are accepted" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const bubbled = try distinctVks(a, max_program_ids - 1);
    const request = try scenario.request(a, &.{.{ .program_vks = bubbled }});
    const output = try runFramed(a, try rollup_ssz.encodeInput(a, request));
    try std.testing.expectEqual(max_program_ids, (try rollup_ssz.decodeOutput(a, output)).program_ids.items.len);
}

test "an allocator that refuses any one request ends the run in the out-of-memory category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const a = arena.allocator();

    const frame = try framedRequest(a);
    const expected = try runFramed(a, frame);

    var fail_index: usize = 0;
    while (true) : (fail_index += 1) {
        var run_arena = std.heap.ArenaAllocator.init(std.testing.allocator);
        defer run_arena.deinit();
        var failing = std.testing.FailingAllocator.init(run_arena.allocator(), .{ .fail_index = fail_index });
        const output = runFramed(failing.allocator(), frame) catch |err| {
            try std.testing.expectEqual(guest_errors.CODE_OUT_OF_MEMORY, guest_errors.exitCode(err));
            continue;
        };
        try std.testing.expectEqualSlices(u8, expected, output);
        break;
    }
    try std.testing.expect(fail_index > 0);
}
