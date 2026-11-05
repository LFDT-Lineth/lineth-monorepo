const std = @import("std");

const lineth_da = @import("lineth_da");
const reference_vectors = @import("reference_vectors");

const blob_kzg = lineth_da.encoding.blob_kzg;

const gpa = std.testing.allocator;
const io = std.testing.io;

const Case = reference_vectors.Case;

fn loadSuite(suite: []const u8) !std.ArrayList(Case) {
    return reference_vectors.loadSuite(gpa, io, suite);
}

fn freeCases(cases: *std.ArrayList(Case)) void {
    reference_vectors.freeCases(gpa, cases);
}

fn expectCase(case: Case, ok: bool) !void {
    if (!ok) std.debug.print("reference vector {s} fails\n", .{case.name});
    try std.testing.expect(ok);
}

test "blob proofs verify exactly as the reference vectors say" {
    var cases = try loadSuite("verify_blob_kzg_proof");
    defer freeCases(&cases);

    var accepted: usize = 0;
    var rejected: usize = 0;
    for (cases.items) |case| {
        const verified = if (case.blobArray()) |blob|
            if (case.commitmentArray()) |commitment|
                if (case.proofArray()) |proof| blob_kzg.verify(blob, commitment, proof) else false
            else
                false
        else
            false;
        // The reference answers `true` for a verifying proof, `false` for one that does not verify
        // and null for malformed input; only a verifying proof is accepted.
        try expectCase(case, verified == std.mem.eql(u8, case.output, "true"));
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
