const std = @import("std");

const lineth_da = @import("lineth_da");
const reference_vectors = @import("reference_vectors");

const blob_kzg = lineth_da.encoding.blob_kzg;

const gpa = std.testing.allocator;
const io = std.testing.io;

const Case = reference_vectors.Case;
const BatchCase = reference_vectors.BatchCase;

fn loadSuite(suite: []const u8) !std.ArrayList(Case) {
    return reference_vectors.loadSuite(gpa, io, suite);
}

fn freeCases(cases: *std.ArrayList(Case)) void {
    reference_vectors.freeCases(gpa, cases);
}

fn expectCase(name: []const u8, ok: bool) !void {
    if (!ok) std.debug.print("reference vector {s} fails\n", .{name});
    try std.testing.expect(ok);
}

/// The entries as one contiguous array; null when an entry has the wrong size.
fn contiguous(comptime len: usize, entries: []const []u8) !?[][len]u8 {
    const out = try gpa.alloc([len]u8, entries.len);
    errdefer gpa.free(out);
    for (entries, out) |entry, *slot| {
        if (entry.len != len) {
            gpa.free(out);
            return null;
        }
        slot.* = entry[0..len].*;
    }
    return out;
}

fn verifyBatchCase(case: BatchCase) !bool {
    const blobs = try contiguous(blob_kzg.blob_len, case.blobs) orelse return false;
    defer gpa.free(blobs);
    const commitments = try contiguous(blob_kzg.commitment_len, case.commitments) orelse return false;
    defer gpa.free(commitments);
    const proofs = try contiguous(blob_kzg.proof_len, case.proofs) orelse return false;
    defer gpa.free(proofs);
    return blob_kzg.verifyBatch(blobs, commitments, proofs);
}

test "blob proof batches verify exactly as the reference vectors say" {
    var cases = try reference_vectors.loadBatchSuite(gpa, io);
    defer reference_vectors.freeBatchCases(gpa, &cases);

    var accepted: usize = 0;
    var rejected: usize = 0;
    for (cases.items) |case| {
        const verified = try verifyBatchCase(case);
        // The reference answers `true` for a verifying batch, `false` for one that does not verify
        // and null for malformed input; only a verifying batch is accepted.
        try expectCase(case.name, verified == std.mem.eql(u8, case.output, "true"));
        if (verified) accepted += 1 else rejected += 1;
    }
    try std.testing.expectEqual(7, accepted);
    try std.testing.expectEqual(17, rejected);
}

test "blob proofs verify exactly as the reference vectors say, each as a batch of one" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);

    var accepted: usize = 0;
    var rejected: usize = 0;
    for (cases.items) |case| {
        const verified = if (case.blobArray()) |blob|
            if (case.commitmentArray()) |commitment|
                if (case.proofArray()) |proof| blob_kzg.verifyBatch(&.{blob.*}, &.{commitment.*}, &.{proof.*}) else false
            else
                false
        else
            false;
        try expectCase(case.name, verified == std.mem.eql(u8, case.output, "true"));
        if (verified) accepted += 1 else rejected += 1;
    }
    try std.testing.expectEqual(9, accepted);
    try std.testing.expectEqual(20, rejected);
}

test "the versioned hash of a commitment is 0x01 followed by its sha256 without the first byte" {
    // The compressed point at infinity: sha256 5f0657f3…4014.
    var infinity: [48]u8 = @splat(0);
    infinity[0] = 0xc0;
    const expected: [32]u8 = .{
        0x01, 0x06, 0x57, 0xf3, 0x75, 0x54, 0xc7, 0x81, 0x40, 0x2a, 0x22, 0x91, 0x7d, 0xee, 0x2f, 0x75,
        0xde, 0xf7, 0xab, 0x96, 0x6d, 0x7b, 0x77, 0x09, 0x05, 0x39, 0x8e, 0xba, 0x3c, 0x44, 0x40, 0x14,
    };
    try std.testing.expectEqualSlices(u8, &expected, &blob_kzg.versionedHash(&infinity));
}
